package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A personnel scope selects the restricted invigilator pool and transaction-local RLS identity.
type Requests struct {
	Pool        *pgxpool.Pool
	PersonnelID *int64
}

type RequestInput struct {
	PersonnelID int64  `json:"personnel_id"`
	Date        string `json:"date"`
	SessionID   *int64 `json:"session_id"`
	Type        string `json:"type"`
	Reason      string `json:"reason"`
}

func (r Requests) List(ctx context.Context, filter ResourceFilter) (json.RawMessage, error) {
	return readWithPersonnel(ctx, r.Pool, r.PersonnelID, func(tx pgx.Tx) (json.RawMessage, error) {
		return ListResources(ctx, tx, "unavailability-requests", filter)
	})
}

func (r Requests) Get(ctx context.Context, id int64) (json.RawMessage, error) {
	return readWithPersonnel(ctx, r.Pool, r.PersonnelID, func(tx pgx.Tx) (json.RawMessage, error) { return GetResource(ctx, tx, "unavailability-requests", id) })
}

func (r Requests) mutate(ctx context.Context, actorID int64, apply func(pgx.Tx) (change, error)) error {
	if r.PersonnelID == nil {
		return (Scheduler{Pool: r.Pool}).mutate(ctx, actorID, apply)
	}
	tx, err := beginWrite(ctx, r.Pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := setPersonnel(ctx, tx, r.PersonnelID); err != nil {
		return err
	}
	if err := activeActor(ctx, tx, actorID, r.PersonnelID); err != nil {
		return err
	}
	value, err := apply(tx)
	if err != nil {
		return err
	}
	// This path changes only pending requests, which cannot affect schedule eligibility.
	if err := appendAudit(ctx, tx, actorID, value); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func requestDate(ctx context.Context, tx pgx.Tx, value RequestInput) (string, error) {
	if value.SessionID == nil {
		return value.Date, nil
	}
	var date string
	err := tx.QueryRow(ctx, `SELECT date::text FROM sessions WHERE id = $1`, *value.SessionID).Scan(&date)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", conflict("missing_session", "The selected session no longer exists.")
	}
	return date, err
}

func pendingRequest(ctx context.Context, tx pgx.Tx, id, owner int64) (json.RawMessage, error) {
	body, err := document(ctx, tx, "unavailability_requests", id)
	if err != nil {
		return nil, err
	}
	var previous struct {
		Status      string `json:"status"`
		PersonnelID int64  `json:"personnel_id"`
	}
	if err := json.Unmarshal(body, &previous); err != nil {
		return nil, err
	}
	if previous.Status != "pending" {
		return nil, conflict("request_decided", "Only a pending request can be edited or withdrawn.")
	}
	if owner != 0 && previous.PersonnelID != owner {
		return nil, conflict("request_owner", "The request owner cannot be changed.")
	}
	return body, nil
}

func (r Requests) Create(ctx context.Context, actorID int64, input RequestInput) (int64, error) {
	var id int64
	err := r.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "create", Entity: "unavailability_requests"}
		date, err := requestDate(ctx, tx, input)
		if err != nil {
			return value, err
		}
		err = tx.QueryRow(ctx, `INSERT INTO unavailability_requests (personnel_id, date, session_id, type, reason) VALUES ($1, $2, $3, $4, $5) RETURNING id`, input.PersonnelID, date, input.SessionID, input.Type, input.Reason).Scan(&id)
		if err != nil {
			return value, err
		}
		value.ID = id
		value.New, err = document(ctx, tx, value.Entity, id)
		return value, err
	})
	return id, err
}

func (r Requests) Update(ctx context.Context, actorID, id int64, input RequestInput) error {
	return r.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "update", Entity: "unavailability_requests", ID: id}
		var err error
		value.Old, err = pendingRequest(ctx, tx, id, input.PersonnelID)
		if err != nil {
			return value, err
		}
		date, err := requestDate(ctx, tx, input)
		if err != nil {
			return value, err
		}
		if _, err := tx.Exec(ctx, `UPDATE unavailability_requests SET date = $2, session_id = $3, type = $4, reason = $5 WHERE id = $1`, id, date, input.SessionID, input.Type, input.Reason); err != nil {
			return value, err
		}
		value.New, err = document(ctx, tx, value.Entity, id)
		return value, err
	})
}

func (r Requests) Delete(ctx context.Context, actorID, id int64) error {
	return r.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "delete", Entity: "unavailability_requests", ID: id}
		var err error
		value.Old, err = pendingRequest(ctx, tx, id, 0)
		if err != nil {
			return value, err
		}
		_, err = tx.Exec(ctx, `DELETE FROM unavailability_requests WHERE id = $1`, id)
		return value, err
	})
}
