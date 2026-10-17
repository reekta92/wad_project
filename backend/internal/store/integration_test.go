package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reekta92/wad_project/backend/internal/config"
)

func TestDatabaseLifecycle(t *testing.T) {
	if os.Getenv("RUN_DATABASE_TESTS") != "1" {
		t.Skip("requires an isolated PostgreSQL database; CI runs this test")
	}
	ownerURL, err := config.MigrationURL()
	if err != nil {
		t.Fatal(err)
	}
	readerURL, err := config.DatabaseURL("exam_reader", "EXAM_READER_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for range 2 {
		if err := Migrate(ctx, ownerURL); err != nil {
			t.Fatal(err)
		}
	}
	created, err := Seed(ctx, ownerURL)
	if err != nil || !created {
		t.Fatalf("initial seed: created=%v error=%v", created, err)
	}
	owner, err := pgx.Connect(ctx, ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close(ctx)
	if _, err = owner.Exec(ctx, `UPDATE departments SET name = 'Edited demo department' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	created, err = Seed(ctx, ownerURL)
	if err != nil || created {
		t.Fatalf("reseed must preserve edits: created=%v error=%v", created, err)
	}
	var name string
	if err = owner.QueryRow(ctx, `SELECT name FROM departments WHERE id = 1`).Scan(&name); err != nil || name != "Edited demo department" {
		t.Fatal("seed overwrote an edit")
	}
	pool, err := pgxpool.New(ctx, readerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, tc := range []struct {
		filter       ScheduleFilter
		count, total int
	}{
		{ScheduleFilter{Page: 1, PageSize: 1}, 1, 3},
		{ScheduleFilter{Page: 100, PageSize: 1}, 0, 3},
		{ScheduleFilter{Page: 1, PageSize: 20, DepartmentID: 1, Term: 5}, 2, 2},
		{ScheduleFilter{Page: 1, PageSize: 20, Query: "Web"}, 1, 1},
		{ScheduleFilter{Page: 1, PageSize: 20, DateFrom: "2026-11-03"}, 0, 0},
	} {
		body, err := Schedule(ctx, pool, tc.filter)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Data  []map[string]any `json:"data"`
			Total int              `json:"total"`
		}
		if err = json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Data) != tc.count || result.Total != tc.total {
			t.Fatalf("incorrect filtered page: %s", body)
		}
		for _, row := range result.Data {
			if _, exists := row["rooms"]; !exists {
				t.Fatal("joined room data missing")
			}
		}
	}
	if _, err := pool.Exec(ctx, `SELECT * FROM personnel`); err == nil {
		t.Fatal("guest role can read private personnel")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO exams (course_id, session_id) VALUES (1, 1)`); err == nil {
		t.Fatal("guest role can write exams")
	}
	if _, err := owner.Exec(ctx, `INSERT INTO exams (course_id, session_id) VALUES (1, 2)`); err == nil {
		t.Fatal("duplicate exam bypassed uniqueness")
	}
	if _, err := owner.Exec(ctx, `UPDATE schedule_state SET published = false WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	body, err := Schedule(ctx, pool, ScheduleFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	var unpublished struct {
		Data  []any `json:"data"`
		Total int   `json:"total"`
	}
	if err = json.Unmarshal(body, &unpublished); err != nil {
		t.Fatal(err)
	}
	if unpublished.Total != 0 || len(unpublished.Data) != 0 {
		t.Fatal("unpublished exams leaked to guest")
	}
}
