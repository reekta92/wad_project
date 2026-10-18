package store

import "testing"

func TestExportReusesPublicationAndFiltersWithoutPageTruncation(t *testing.T) {
	f := isolatedScheduler(t)
	filter := ScheduleFilter{Page: 99, PageSize: 1, DepartmentID: 1}
	rows, err := ExportSchedule(f.ctx, f.scheduler.Pool, filter, true)
	if err != nil || len(rows) != 2 || rows[0].CourseCode != "WAD301" || rows[1].CourseCode != "DB301" {
		t.Fatalf("filtered export: %+v %v", rows, err)
	}
	if err := f.scheduler.SetPublication(f.ctx, f.actor, false); err != nil {
		t.Fatal(err)
	}
	public, err := ExportSchedule(f.ctx, f.scheduler.Pool, filter, false)
	if err != nil || len(public) != 0 {
		t.Fatalf("unpublished data exposed: %v %v", public, err)
	}
	current, err := ExportSchedule(f.ctx, f.scheduler.Pool, filter, true)
	if err != nil || len(current) != 2 {
		t.Fatal("administrator lost current schedule")
	}
	f.exec(t, `INSERT INTO departments (name) VALUES ('Export limit department'); INSERT INTO courses (code,name,kind,enrollment,term,department_id) SELECT 'LIMIT' || i, 'Limit fixture', 'elective',1,1,(SELECT max(id) FROM departments) FROM generate_series(1,10001) i; INSERT INTO exams (course_id,session_id) SELECT id,1 FROM courses WHERE code LIKE 'LIMIT%'`)
	if _, err := ExportSchedule(f.ctx, f.scheduler.Pool, ScheduleFilter{}, true); err != ErrExportLimit {
		t.Fatalf("unbounded export accepted: %v", err)
	}
}
