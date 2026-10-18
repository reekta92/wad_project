package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reekta92/wad_project/backend/internal/config"
)

type Databases struct {
	Reader, Administrator, Invigilator *pgxpool.Pool
}

func Open(ctx context.Context, cfg config.Config) (*Databases, error) {
	d := &Databases{}
	var err error
	for _, entry := range []struct {
		target **pgxpool.Pool
		url    string
	}{{&d.Reader, cfg.ReaderURL}, {&d.Administrator, cfg.AdminURL}, {&d.Invigilator, cfg.InvigilatorURL}} {
		*entry.target, err = pgxpool.New(ctx, entry.url)
		if err != nil {
			d.Close()
			return nil, errors.New("invalid application database configuration")
		}
	}
	return d, nil
}

func (d *Databases) Close() {
	for _, pool := range []*pgxpool.Pool{d.Reader, d.Administrator, d.Invigilator} {
		if pool != nil {
			pool.Close()
		}
	}
}

func (d *Databases) Ready(ctx context.Context) error {
	for _, pool := range []*pgxpool.Pool{d.Reader, d.Administrator, d.Invigilator} {
		if pool == nil {
			return errors.New("database pool is not configured")
		}
		if err := pool.Ping(ctx); err != nil {
			return err
		}
	}
	if _, err := d.Reader.Exec(ctx, `SELECT 1 FROM published_schedule LIMIT 0`); err != nil {
		return err
	}
	if _, err := d.Administrator.Exec(ctx, `SELECT 1 FROM current_schedule LIMIT 0`); err != nil {
		return err
	}
	_, err := d.Invigilator.Exec(ctx, `SELECT 1 FROM own_duties LIMIT 0`)
	return err
}

type User struct {
	ID          int64
	PersonnelID *int64
}

func (d *Databases) User(ctx context.Context, issuer, subject string) (User, error) {
	var user User
	err := d.Administrator.QueryRow(ctx, `SELECT id, personnel_id FROM app_users WHERE issuer = $1 AND subject = $2 AND active`, issuer, subject).Scan(&user.ID, &user.PersonnelID)
	return user, err
}

func setPersonnel(ctx context.Context, tx pgx.Tx, personnelID *int64) error {
	if personnelID == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `SELECT set_config('app.personnel_id', $1, true)`, strconv.FormatInt(*personnelID, 10))
	return err
}

func readWithPersonnel(ctx context.Context, pool *pgxpool.Pool, personnelID *int64, read func(pgx.Tx) (json.RawMessage, error)) (json.RawMessage, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := setPersonnel(ctx, tx, personnelID); err != nil {
		return nil, err
	}
	return read(tx)
}
