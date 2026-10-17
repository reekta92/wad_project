package store

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reekta92/wad_project/backend/internal/config"
)

func TestAutomaticScheduleStaffingPublicationAndClear(t *testing.T) {
	f := isolatedScheduler(t)
	if err := f.scheduler.Clear(f.ctx, f.actor); err != nil {
		t.Fatal(err)
	}
	plan, err := f.scheduler.Plan(f.ctx, f.actor, "all")
	if err != nil || plan.Count != 3 {
		t.Fatalf("planning: %+v %v", plan, err)
	}
	var session int64
	if err := f.owner.QueryRow(f.ctx, `SELECT session_id FROM exams WHERE course_id = 2`).Scan(&session); err != nil || session != 3 {
		t.Fatalf("planning did not skip adjacent conflicts: %d %v", session, err)
	}
	f.expectRollback(t, "unstaffed_room", func() error { return f.scheduler.SetPublication(f.ctx, f.actor, true) })
	staffed, err := f.scheduler.Staff(f.ctx, f.actor, "all", 0)
	if err != nil || staffed.Count != 3 {
		t.Fatalf("staffing: %+v %v", staffed, err)
	}
	if err := f.scheduler.SetPublication(f.ctx, f.actor, true); err != nil {
		t.Fatal(err)
	}
	if noop, err := f.scheduler.Plan(f.ctx, f.actor, "all"); err != nil || noop.Count != 0 {
		t.Fatalf("planning was not repeatable: %+v %v", noop, err)
	}
	if noop, err := f.scheduler.Staff(f.ctx, f.actor, "all", 0); err != nil || noop.Count != 0 {
		t.Fatalf("staffing was not repeatable: %+v %v", noop, err)
	}
	var published bool
	if err := f.owner.QueryRow(f.ctx, `SELECT published FROM schedule_state`).Scan(&published); err != nil || !published {
		t.Fatalf("no-op unpublished the schedule: %v", err)
	}
	if err := f.scheduler.Clear(f.ctx, f.actor); err != nil {
		t.Fatal(err)
	}
	var exams, allocations, assignments, courses, auditBefore, auditAfter int
	if err := f.owner.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM exams), (SELECT count(*) FROM exam_rooms), (SELECT count(*) FROM invigilator_assignments), (SELECT count(*) FROM courses),
        (SELECT jsonb_array_length(old_value->'exams') FROM audit_logs WHERE action = 'clear' ORDER BY id DESC LIMIT 1),
        (SELECT jsonb_array_length(new_value->'exams') FROM audit_logs WHERE action = 'clear' ORDER BY id DESC LIMIT 1)`).Scan(&exams, &allocations, &assignments, &courses, &auditBefore, &auditAfter); err != nil {
		t.Fatal(err)
	}
	if exams != 0 || allocations != 0 || assignments != 0 || courses != 3 || auditBefore != 3 || auditAfter != 0 {
		t.Fatal("clear scope or audit was incorrect")
	}
}

func TestAutomaticBatchesRollBackEarlierTargetsAndAuditFailures(t *testing.T) {
	t.Run("planning", func(t *testing.T) {
		f := isolatedScheduler(t)
		emptyScheduleForRace(t, f)
		f.exec(t, `UPDATE courses SET enrollment = 1000 WHERE id = 5`)
		f.expectRollback(t, "planning_unresolved", func() error { _, err := f.scheduler.Plan(f.ctx, f.actor, "spring"); return err })
	})
	t.Run("staffing", func(t *testing.T) {
		f := isolatedScheduler(t)
		f.exec(t, `DELETE FROM invigilator_assignments; DELETE FROM personnel WHERE id <> 1`)
		f.expectRollback(t, "personnel_unavailable", func() error { _, err := f.scheduler.Staff(f.ctx, f.actor, "all", 0); return err })
		if result, err := f.scheduler.Staff(f.ctx, f.actor, "one", 1); err != nil || result.Count != 1 {
			t.Fatalf("single staffing: %+v %v", result, err)
		}
		f.expectRollback(t, "allocation_staffed", func() error { _, err := f.scheduler.Staff(f.ctx, f.actor, "one", 1); return err })
		if _, err := f.scheduler.Staff(f.ctx, f.actor, "one", 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("missing allocation: %v", err)
		}
	})
	t.Run("audit", func(t *testing.T) {
		f := isolatedScheduler(t)
		f.exec(t, `REVOKE INSERT ON audit_logs FROM exam_admin`)
		f.expectRollback(t, "", func() error { return f.scheduler.Clear(f.ctx, f.actor) })
	})
}

func TestReportsIncludeZeroDutiesAndScopeMemberViews(t *testing.T) {
	f := isolatedScheduler(t)
	f.exec(t, `INSERT INTO personnel (id, title, first_name, last_name, department_id) VALUES (4, 'Dr.', 'Zero', 'Duties', 1)`)
	url, err := config.DatabaseURL("exam_invigilator", "EXAM_INVIGILATOR_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := pgxpool.ParseConfig(url)
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
	d := Databases{Administrator: f.scheduler.Pool, Invigilator: pool}
	body, err := d.Workload(f.ctx, ResourceFilter{Page: 1, PageSize: 100, DateFrom: "2026-11-03"})
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Data []struct {
			Duties      int
			PersonnelID int64 `json:"personnel_id"`
		}
		Total int
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || len(page.Data) != 4 {
		t.Fatalf("zero-duty personnel omitted: %s", body)
	}
	for _, row := range page.Data {
		if row.Duties != 0 {
			t.Fatalf("date filter lost: %s", body)
		}
	}
	body, err = d.OwnDuties(f.ctx, 1, ResourceFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Data[0].PersonnelID != 1 {
		t.Fatalf("cross-owner duties: %s", body)
	}
	body, err = d.OwnWorkload(f.ctx, 4, ResourceFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var own struct {
		Duties      int
		PersonnelID int64 `json:"personnel_id"`
	}
	if err := json.Unmarshal(body, &own); err != nil || own.Duties != 0 || own.PersonnelID != 4 {
		t.Fatalf("own empty workload: %s %v", body, err)
	}
	var count int
	if err := pool.QueryRow(f.ctx, `SELECT count(*) FROM own_duties`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("identity leaked through pooled member views: %d %v", count, err)
	}
	for _, tc := range []struct {
		person, session int64
		available       bool
		code            string
	}{{1, 1, false, "personnel_conflict"}, {1, 2, true, ""}} {
		body, err := d.Availability(f.ctx, ResourceFilter{Page: 1, PageSize: 20, PersonnelID: tc.person, SessionID: tc.session})
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Data  []availabilityRow
			Total int
		}
		if err := json.Unmarshal(body, &result); err != nil || result.Total != 1 || len(result.Data) != 1 || result.Data[0].Available != tc.available || result.Data[0].ReasonCode != tc.code {
			t.Fatalf("incorrect availability: %s %v", body, err)
		}
	}
	f.exec(t, `INSERT INTO courses (code,name,kind,enrollment,term,department_id) SELECT 'EXTRA' || n, 'Extra course', 'elective', 10, 1, 1 FROM generate_series(1,7) n`)
	body, err = d.Dashboard(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	var dashboard struct {
		Totals struct {
			Unplanned int `json:"unplanned_courses"`
		}
		Previews struct {
			Unplanned []any `json:"unplanned_courses"`
		}
	}
	if err := json.Unmarshal(body, &dashboard); err != nil || dashboard.Totals.Unplanned != 7 || len(dashboard.Previews.Unplanned) != 5 {
		t.Fatalf("unbounded or inaccurate dashboard: %s %v", body, err)
	}
}
