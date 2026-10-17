package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("resource not found")
var ErrInactiveActor = errors.New("the acting account is not active")

// Scheduler owns administrator domain writes; callers supply the verified local user ID.
type Scheduler struct{ Pool *pgxpool.Pool }

type change struct {
	Action, Entity string
	ID             int64
	Old, New       json.RawMessage
	Invalidate     bool
}

func beginWrite(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	// The lock statement must finish before any rule reads obtain their snapshot.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, ScheduleLock); err != nil {
		tx.Rollback(context.WithoutCancel(ctx))
		return nil, err
	}
	return tx, nil
}

func loadSchedule(ctx context.Context, tx pgx.Tx) (scheduleState, error) {
	const query = `SELECT jsonb_build_object(
        'courses', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM courses t),
        'sessions', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM sessions t),
        'rooms', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM classrooms t),
        'personnel', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM personnel t),
        'exams', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM exams t),
        'allocations', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM exam_rooms t),
        'assignments', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM invigilator_assignments t),
        'requests', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM unavailability_requests t))`
	var state scheduleState
	var body []byte
	if err := tx.QueryRow(ctx, query).Scan(&body); err != nil {
		return state, err
	}
	return state, json.Unmarshal(body, &state)
}

func (s Scheduler) mutate(ctx context.Context, actorID int64, apply func(pgx.Tx) (change, error)) error {
	tx, err := beginWrite(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := activeActor(ctx, tx, actorID, nil); err != nil {
		return err
	}
	value, err := apply(tx)
	if err != nil {
		return err
	}
	state, err := loadSchedule(ctx, tx)
	if err != nil {
		return err
	}
	if err := state.validate(value.Action == "publish"); err != nil {
		return err
	}
	if value.Invalidate {
		if _, err := tx.Exec(ctx, `UPDATE schedule_state SET published = false, updated_at = now() WHERE id = 1`); err != nil {
			return err
		}
	}
	if err := appendAudit(ctx, tx, actorID, value); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func activeActor(ctx context.Context, tx pgx.Tx, actorID int64, personnelID *int64) error {
	var active bool
	err := tx.QueryRow(ctx, `SELECT coalesce(active AND ($2::bigint IS NULL OR personnel_id = $2), false) FROM app_users WHERE id = $1`, actorID, personnelID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
		return ErrInactiveActor
	}
	return err
}

func appendAudit(ctx context.Context, tx pgx.Tx, actorID int64, value change) error {
	var entityID any
	if value.ID > 0 {
		entityID = value.ID
	}
	_, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, action, entity, entity_id, old_value, new_value) VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb)`, actorID, value.Action, value.Entity, entityID, value.Old, value.New)
	return err
}

func document(ctx context.Context, tx pgx.Tx, entity string, id int64) (json.RawMessage, error) {
	var query string
	switch entity {
	case "exams":
		query = `SELECT to_jsonb(e) || jsonb_build_object(
            'rooms', (SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY r.id), '[]'::jsonb) FROM exam_rooms r WHERE r.exam_id = e.id),
            'assignments', (SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY a.id), '[]'::jsonb) FROM invigilator_assignments a JOIN exam_rooms r ON r.id = a.id WHERE r.exam_id = e.id))
            FROM exams e WHERE e.id = $1`
	case "departments", "courses", "sessions", "classrooms", "personnel", "unavailability_requests", "invigilator_assignments", "schedule_state":
		query = `SELECT to_jsonb(t) FROM ` + entity + ` t WHERE id = $1`
	default:
		return nil, fmt.Errorf("unknown internal audit entity")
	}
	var body json.RawMessage
	err := tx.QueryRow(ctx, query, id).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return body, err
}

func (s Scheduler) update(ctx context.Context, actorID int64, entity string, id int64, query string, args ...any) error {
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "update", Entity: entity, ID: id, Invalidate: true}
		var err error
		value.Old, err = document(ctx, tx, entity, id)
		if err != nil {
			return value, err
		}
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return value, err
		}
		value.New, err = document(ctx, tx, entity, id)
		return value, err
	})
}
