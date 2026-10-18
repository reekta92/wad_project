package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reekta92/wad_project/backend/internal/config"
	"github.com/reekta92/wad_project/backend/migrations"
	"github.com/reekta92/wad_project/backend/seeds"
)

type schedulerFixture struct {
	ctx       context.Context
	owner     *pgxpool.Pool
	scheduler Scheduler
	actor     int64
}

func isolatedScheduler(t *testing.T) schedulerFixture {
	t.Helper()
	if os.Getenv("RUN_DATABASE_TESTS") != "1" {
		t.Skip("requires isolated PostgreSQL; CI runs this test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	t.Cleanup(cancel)
	ownerURL, err := config.MigrationURL()
	if err != nil {
		t.Fatal(err)
	}
	ownerConfig, err := pgxpool.ParseConfig(ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := pgxpool.NewWithConfig(ctx, ownerConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bootstrap.Close)
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	schema := "scheduler_test_" + hex.EncodeToString(nonce[:])
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := bootstrap.Exec(ctx, `CREATE SCHEMA `+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := bootstrap.Exec(cleanup, `DROP SCHEMA `+quoted+` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	ownerConfig.ConnConfig.RuntimeParams["search_path"] = schema
	owner, err := pgxpool.NewWithConfig(ctx, ownerConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(owner.Close)
	tx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(92831401)`); err != nil {
		t.Fatal(err)
	}
	files, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE schema_migrations (name text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		body, err := migrations.Files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
	}
	// Production migrations target public; these grants mirror them in this disposable schema.
	if _, err := tx.Exec(ctx, `GRANT USAGE ON SCHEMA `+quoted+` TO exam_admin, exam_invigilator, exam_reader; GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA `+quoted+` TO exam_admin; GRANT USAGE ON SEQUENCE `+quoted+`.unavailability_requests_id_seq TO exam_invigilator`); err != nil {
		t.Fatal(err)
	}
	body, err := seeds.Files.ReadFile("demo.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(body)); err != nil {
		t.Fatal(err)
	}
	var actor int64
	if err := tx.QueryRow(ctx, `INSERT INTO app_users (issuer, subject) VALUES ('https://identity.example/realms/exams', 'scheduler-test') RETURNING id`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	adminURL, err := config.DatabaseURL("exam_admin", "EXAM_ADMIN_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	adminConfig, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	adminConfig.ConnConfig.RuntimeParams["search_path"] = schema
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	return schedulerFixture{ctx: ctx, owner: owner, scheduler: Scheduler{Pool: admin}, actor: actor}
}

func TestInvigilatorDatabaseOwnershipAndPoolIsolation(t *testing.T) {
	f := isolatedScheduler(t)
	f.exec(t, `INSERT INTO unavailability_requests (personnel_id, date, type, reason) VALUES (1, '2026-11-03', 'leave', 'Own pending'), (2, '2026-11-03', 'leave', 'Another person')`)
	invigilatorURL, err := config.DatabaseURL("exam_invigilator", "EXAM_INVIGILATOR_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := pgxpool.ParseConfig(invigilatorURL)
	if err != nil {
		t.Fatal(err)
	}
	settings.MaxConns = 1
	settings.ConnConfig.RuntimeParams["search_path"] = f.owner.Config().ConnConfig.RuntimeParams["search_path"]
	pool, err := pgxpool.NewWithConfig(f.ctx, settings)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := beginWrite(f.ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err := tx.Exec(f.ctx, `SELECT set_config('app.personnel_id', '1', true)`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(f.ctx, `SELECT count(*) FROM unavailability_requests`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("another owner's request was visible: count=%d %v", count, err)
	}
	if result, err := tx.Exec(f.ctx, `UPDATE unavailability_requests SET reason = 'Changed another person' WHERE personnel_id = 2`); err != nil || result.RowsAffected() != 0 {
		t.Fatalf("cross-owner update succeeded: %v", err)
	}
	for _, query := range []string{
		`INSERT INTO unavailability_requests (personnel_id, date, type, reason) VALUES (2, '2026-11-04', 'leave', 'Impersonation')`,
		`UPDATE unavailability_requests SET status = 'approved', decided_by = 1, decided_at = now() WHERE personnel_id = 1`,
		`UPDATE unavailability_requests SET personnel_id = 2 WHERE personnel_id = 1`,
	} {
		savepoint, err := tx.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, denied := savepoint.Exec(f.ctx, query)
		if err := savepoint.Rollback(f.ctx); err != nil {
			t.Fatal(err)
		}
		var failure *pgconn.PgError
		if !errors.As(denied, &failure) || failure.Code != "42501" {
			t.Fatalf("ownership or decision write was not denied: %v", denied)
		}
	}
	if result, err := tx.Exec(f.ctx, `UPDATE unavailability_requests SET reason = 'Own allowed edit' WHERE personnel_id = 1`); err != nil || result.RowsAffected() != 1 {
		t.Fatalf("own pending edit failed: %v", err)
	}
	if err := tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(f.ctx, `SELECT count(*) FROM unavailability_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("pooled connection retained the previous user's identity: count=%d %v", count, err)
	}
	f.exec(t, `UPDATE unavailability_requests SET status = 'approved', decided_by = $1, decided_at = now() WHERE personnel_id = 1`, f.actor)
	tx, err = beginWrite(f.ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err := tx.Exec(f.ctx, `SELECT set_config('app.personnel_id', '1', true)`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`UPDATE unavailability_requests SET reason = 'Changed decided request'`, `DELETE FROM unavailability_requests`} {
		if result, err := tx.Exec(f.ctx, query); err != nil || result.RowsAffected() != 0 {
			t.Fatalf("decided request changed: %v", err)
		}
	}
}

func (f schedulerFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.owner.Exec(f.ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f schedulerFixture) fingerprint(t *testing.T) string {
	t.Helper()
	var value string
	const query = `SELECT jsonb_build_object(
        'courses', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM courses t),
        'sessions', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM sessions t),
        'rooms', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM classrooms t),
        'personnel', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM personnel t),
        'exams', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM exams t),
        'allocations', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM exam_rooms t),
        'assignments', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM invigilator_assignments t),
        'requests', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM unavailability_requests t),
        'audit', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM audit_logs t),
        'publication', (SELECT to_jsonb(t) FROM schedule_state t WHERE id = 1))::text`
	if err := f.owner.QueryRow(f.ctx, query).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func (f schedulerFixture) expectRollback(t *testing.T, code string, operation func() error) {
	t.Helper()
	before := f.fingerprint(t)
	err := operation()
	if err == nil || (code != "" && ruleCode(err) != code) {
		t.Fatalf("wanted conflict %q, got %v", code, err)
	}
	if after := f.fingerprint(t); after != before {
		t.Fatalf("failed write changed domain data, audit, or publication")
	}
}

func TestSchedulerTransactionsAndFinalState(t *testing.T) {
	f := isolatedScheduler(t)
	course := CourseInput{Code: "WAD301", Name: "Web Application Development", Kind: "mandatory", Enrollment: 151, Term: 5, DepartmentID: 1}
	f.expectRollback(t, "room_capacity", func() error { return f.scheduler.UpdateCourse(f.ctx, f.actor, 1, course) })
	f.expectRollback(t, "session_overlap", func() error {
		return f.scheduler.UpdateSession(f.ctx, f.actor, 1, SessionInput{Date: "2026-11-02", StartTime: "09:00", EndTime: "11:30"})
	})
	f.expectRollback(t, "room_inactive", func() error {
		return f.scheduler.UpdateClassroom(f.ctx, f.actor, 1, ClassroomInput{Name: "Lecture Hall", Capacity: 150, Type: "lecture hall", Active: false})
	})
	f.expectRollback(t, "cohort_gap", func() error { return f.scheduler.UpdateExam(f.ctx, f.actor, 2, 2, 2) })
	// Session 1 requires replacing the old room and cascading its duty before final validation fails.
	f.expectRollback(t, "cohort_gap", func() error { return f.scheduler.UpdateExam(f.ctx, f.actor, 2, 2, 1) })
	f.expectRollback(t, "personnel_conflict", func() error { return f.scheduler.UpdateAssignment(f.ctx, f.actor, 3, 1) })
	f.expectRollback(t, "", func() error { _, err := f.scheduler.CreateExam(f.ctx, f.actor, 1, 2); return err })
	f.exec(t, `INSERT INTO unavailability_requests (personnel_id, date, type, reason) VALUES (1, '2026-11-02', 'leave', 'Fictional request')`)
	f.expectRollback(t, "personnel_unavailable", func() error { return f.scheduler.DecideRequest(f.ctx, f.actor, 1, true) })
	if err := f.scheduler.DecideRequest(f.ctx, f.actor, 1, false); err != nil {
		t.Fatal(err)
	}
	f.expectRollback(t, "request_decided", func() error { return f.scheduler.DecideRequest(f.ctx, f.actor, 1, true) })
	// Replacing an unchanged exam preserves its existing allocation identity and duty.
	if err := f.scheduler.UpdateExam(f.ctx, f.actor, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	var preserved bool
	if err := f.owner.QueryRow(f.ctx, `SELECT EXISTS (SELECT 1 FROM exam_rooms r JOIN invigilator_assignments a ON a.id = r.id WHERE r.id = 1 AND r.exam_id = 1 AND a.personnel_id = 1)`).Scan(&preserved); err != nil || !preserved {
		t.Fatalf("unchanged duty lost: %v", err)
	}
	course.Enrollment, course.Name = 132, "Updated course name"
	if err := f.scheduler.UpdateCourse(f.ctx, f.actor, 1, course); err != nil {
		t.Fatal(err)
	}
	var published bool
	var oldName, newName string
	var actor int64
	if err := f.owner.QueryRow(f.ctx, `SELECT published FROM schedule_state WHERE id = 1`).Scan(&published); err != nil || published {
		t.Fatalf("write did not unpublish: %v", err)
	}
	if err := f.owner.QueryRow(f.ctx, `SELECT old_value->>'name', new_value->>'name', actor_user_id FROM audit_logs WHERE entity = 'courses' ORDER BY id DESC LIMIT 1`).Scan(&oldName, &newName, &actor); err != nil || oldName != "Web Application Development" || newName != course.Name || actor != f.actor {
		t.Fatalf("inaccurate audit: %q %q %d %v", oldName, newName, actor, err)
	}
	if err := f.scheduler.SetPublication(f.ctx, f.actor, true); err != nil {
		t.Fatal(err)
	}
	if err := f.scheduler.DeleteAssignment(f.ctx, f.actor, 1); err != nil {
		t.Fatal(err)
	}
	f.expectRollback(t, "unstaffed_room", func() error { return f.scheduler.SetPublication(f.ctx, f.actor, true) })
	if err := f.scheduler.CreateAssignment(f.ctx, f.actor, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := f.scheduler.SetPublication(f.ctx, f.actor, true); err != nil {
		t.Fatal(err)
	}
	if err := f.scheduler.DeleteExam(f.ctx, f.actor, 3); err != nil {
		t.Fatal(err)
	}
	var cascade bool
	if err := f.owner.QueryRow(f.ctx, `SELECT NOT EXISTS (SELECT 1 FROM exam_rooms WHERE exam_id = 3) AND NOT EXISTS (SELECT 1 FROM invigilator_assignments WHERE id = 3) AND EXISTS (SELECT 1 FROM personnel WHERE id = 3)`).Scan(&cascade); err != nil || !cascade {
		t.Fatalf("wrong deletion scope: %v", err)
	}
}

func TestSchedulerAuditFailureRollsBackEverything(t *testing.T) {
	f := isolatedScheduler(t)
	f.exec(t, `REVOKE INSERT ON audit_logs FROM exam_admin`)
	f.expectRollback(t, "", func() error {
		return f.scheduler.UpdatePersonnel(f.ctx, f.actor, 1, PersonnelInput{Title: "Prof.", FirstName: "Ada", LastName: "Demir", DepartmentID: 1})
	})
	f.exec(t, `GRANT INSERT ON audit_logs TO exam_admin`)
	f.exec(t, `UPDATE app_users SET active = false WHERE id = $1`, f.actor)
	f.expectRollback(t, "", func() error {
		err := f.scheduler.DeleteExam(f.ctx, f.actor, 1)
		if !errors.Is(err, ErrInactiveActor) {
			t.Fatalf("disabled actor accepted: %v", err)
		}
		return err
	})
}

func TestSchedulerBatchFailureRollsBackEarlierWrites(t *testing.T) {
	f := isolatedScheduler(t)
	emptyScheduleForRace(t, f)
	f.expectRollback(t, "room_capacity", func() error {
		return f.scheduler.mutate(f.ctx, f.actor, func(tx pgx.Tx) (change, error) {
			value := change{Action: "create_batch", Entity: "exams", Invalidate: true}
			for _, courseID := range []int64{4, 5} {
				var id int64
				if err := tx.QueryRow(f.ctx, `INSERT INTO exams (course_id, session_id) VALUES ($1, 2) RETURNING id`, courseID).Scan(&id); err != nil {
					return value, err
				}
				if err := allocateRooms(f.ctx, tx, id, courseID, 2); err != nil {
					return value, err
				}
			}
			return value, nil
		})
	})
}

func waitForLockWaiters(t *testing.T, f schedulerFixture, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := f.owner.QueryRow(f.ctx, `SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND objid = $1 AND NOT granted`, ScheduleLock).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting >= count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("writers did not contend on the shared transaction lock")
}

func emptyScheduleForRace(t *testing.T, f schedulerFixture) {
	t.Helper()
	f.exec(t, `DELETE FROM exams; DELETE FROM classrooms WHERE id <> 1;
        INSERT INTO courses (id, code, name, kind, enrollment, term, department_id) VALUES
        (4, 'RACE4', 'Concurrent A', 'mandatory', 140, 2, 1),
        (5, 'RACE5', 'Concurrent B', 'mandatory', 140, 2, 2)`)
}

func TestSchedulerConcurrentCreatesSerializeRuleReads(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		name := "room conflict"
		if duplicate {
			name = "duplicate course"
		}
		t.Run(name, func(t *testing.T) {
			f := isolatedScheduler(t)
			emptyScheduleForRace(t, f)
			holder, err := beginWrite(f.ctx, f.owner)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(f.ctx)
			results := make(chan error, 2)
			for _, courseID := range []int64{4, 5} {
				if duplicate {
					courseID = 4
				}
				go func(id int64) { _, err := f.scheduler.CreateExam(f.ctx, f.actor, id, 2); results <- err }(courseID)
			}
			waitForLockWaiters(t, f, 2)
			if err := holder.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			successes, conflicts := 0, 0
			for range 2 {
				err := <-results
				if err == nil {
					successes++
					continue
				}
				var databaseError *pgconn.PgError
				if (!duplicate && ruleCode(err) == "room_capacity") || (duplicate && errors.As(err, &databaseError) && databaseError.Code == "23505") {
					conflicts++
				} else {
					t.Fatalf("unexpected concurrent failure: %v", err)
				}
			}
			var exams, rooms, audit int
			if err := f.owner.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM exams), (SELECT count(*) FROM exam_rooms), (SELECT count(*) FROM audit_logs WHERE action = 'create')`).Scan(&exams, &rooms, &audit); err != nil {
				t.Fatal(err)
			}
			if successes != 1 || conflicts != 1 || exams != 1 || rooms != 1 || audit != 1 {
				t.Fatalf("non-atomic concurrent writes: successes=%d conflicts=%d exams=%d rooms=%d audit=%d", successes, conflicts, exams, rooms, audit)
			}
		})
	}
}

func TestSchedulerReadsMasterEditAfterAcquiringLock(t *testing.T) {
	f := isolatedScheduler(t)
	emptyScheduleForRace(t, f)
	holder, err := beginWrite(f.ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(f.ctx)
	if _, err := holder.Exec(f.ctx, `UPDATE classrooms SET capacity = 10 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := f.scheduler.CreateExam(f.ctx, f.actor, 4, 2); result <- err }()
	waitForLockWaiters(t, f, 1)
	if err := holder.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; ruleCode(err) != "room_capacity" {
		t.Fatalf("writer did not see committed master edit: %v", err)
	}
	var count int
	if err := f.owner.QueryRow(f.ctx, `SELECT count(*) FROM exams`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed candidate persisted: %d %v", count, err)
	}
}
