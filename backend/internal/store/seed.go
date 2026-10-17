package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/reekta92/wad_project/backend/seeds"
)

// ScheduleLock is shared by every domain writer, including demo initialization.
const ScheduleLock int64 = 92831402

func Seed(ctx context.Context, databaseURL string) (bool, error) {
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, ScheduleLock); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS seed_runs (name text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return false, err
	}
	var done bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM seed_runs WHERE name = 'demo-v1')`).Scan(&done); err != nil {
		return false, err
	}
	if done {
		return false, tx.Commit(ctx)
	}
	var populated bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM departments) OR EXISTS (SELECT 1 FROM exams)`).Scan(&populated); err != nil {
		return false, err
	}
	if populated {
		return false, fmt.Errorf("demo seed requires an empty application database; existing data was preserved")
	}
	body, err := seeds.Files.ReadFile("demo.sql")
	if err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, string(body)); err != nil {
		return false, err
	}
	state, err := loadSchedule(ctx, tx)
	if err != nil {
		return false, err
	}
	if err := state.validate(true); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO seed_runs (name) VALUES ('demo-v1')`); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
