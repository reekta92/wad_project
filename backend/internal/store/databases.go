package store

import (
	"context"
	"errors"

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
	_, err := d.Reader.Exec(ctx, `SELECT 1 FROM published_schedule LIMIT 0`)
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
