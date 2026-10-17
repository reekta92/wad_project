package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/reekta92/wad_project/backend/migrations"
)

func Migrate(ctx context.Context, databaseURL string) error {
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to migration database: %w", err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(92831401)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	files, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		return err
	}
	for _, name := range files {
		body, err := migrations.Files.ReadFile(name)
		if err != nil {
			return err
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(body))
		var previous string
		err = tx.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE name = $1`, name).Scan(&previous)
		if err == nil {
			if previous != checksum {
				return fmt.Errorf("applied migration %s changed; add a new migration instead", name)
			}
			continue
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`, name, checksum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
