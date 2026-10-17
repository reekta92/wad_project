package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type ResourceFilter struct {
	Page, PageSize, Term, MinCapacity                      int
	DepartmentID, CourseID, SessionID, PersonnelID, ExamID int64
	ActorID                                                int64
	Name, Query, DateFrom, DateTo, Kind, Status, Type      string
	Entity, Action                                         string
	Active                                                 *bool
	Floor                                                  *int
}

// Both pools and identity-scoped transactions execute the same parameterized reads.
type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func resourceQuery(resource string, f ResourceFilter) (string, []any, error) {
	switch resource {
	case "departments":
		return `SELECT d.* FROM departments d WHERE ($1 = '' OR strpos(lower(d.name), lower($1)) > 0)`, []any{f.Name}, nil
	case "courses":
		return `SELECT c.*, d.name AS department FROM courses c JOIN departments d ON d.id = c.department_id
            WHERE ($1::bigint = 0 OR c.department_id = $1) AND ($2::integer = 0 OR c.term = $2)
            AND ($3 = '' OR c.kind = $3) AND ($4 = '' OR strpos(lower(c.code || ' ' || c.name), lower($4)) > 0)`, []any{f.DepartmentID, f.Term, f.Kind, f.Query}, nil
	case "sessions":
		return `SELECT s.* FROM sessions s WHERE ($1::date IS NULL OR s.date >= $1) AND ($2::date IS NULL OR s.date <= $2)`, []any{nullableDate(f.DateFrom), nullableDate(f.DateTo)}, nil
	case "classrooms":
		return `SELECT r.* FROM classrooms r WHERE ($1::boolean IS NULL OR r.active = $1) AND ($2::integer IS NULL OR r.floor = $2) AND r.capacity >= $3`, []any{f.Active, f.Floor, f.MinCapacity}, nil
	case "personnel":
		return `SELECT p.*, d.name AS department FROM personnel p JOIN departments d ON d.id = p.department_id
            WHERE ($1::bigint = 0 OR p.department_id = $1) AND ($2 = '' OR strpos(lower(p.first_name || ' ' || p.last_name), lower($2)) > 0)`, []any{f.DepartmentID, f.Query}, nil
	case "unavailability-requests":
		return `SELECT r.* FROM unavailability_requests r WHERE ($1::bigint = 0 OR r.personnel_id = $1)
            AND ($2::date IS NULL OR r.date >= $2) AND ($3::date IS NULL OR r.date <= $3)
            AND ($4 = '' OR r.status = $4) AND ($5 = '' OR r.type = $5)`, []any{f.PersonnelID, nullableDate(f.DateFrom), nullableDate(f.DateTo), f.Status, f.Type}, nil
	case "exams":
		return `SELECT e.*, c.code AS course_code, c.name AS course_name, c.department_id, c.term,
            s.date, s.start_time, s.end_time,
            (SELECT coalesce(jsonb_agg(jsonb_build_object('id', a.id, 'classroom_id', r.id, 'name', r.name, 'capacity', r.capacity, 'floor', r.floor) ORDER BY a.id), '[]'::jsonb)
             FROM exam_rooms a JOIN classrooms r ON r.id = a.classroom_id WHERE a.exam_id = e.id) AS rooms
            FROM exams e JOIN courses c ON c.id = e.course_id JOIN sessions s ON s.id = e.session_id
            WHERE ($1::bigint = 0 OR e.course_id = $1) AND ($2::bigint = 0 OR c.department_id = $2)
            AND ($3::integer = 0 OR c.term = $3) AND ($4::bigint = 0 OR e.session_id = $4)
            AND ($5::date IS NULL OR s.date >= $5) AND ($6::date IS NULL OR s.date <= $6)`, []any{f.CourseID, f.DepartmentID, f.Term, f.SessionID, nullableDate(f.DateFrom), nullableDate(f.DateTo)}, nil
	case "invigilator-assignments":
		return `SELECT a.*, r.exam_id, r.classroom_id, c.name AS classroom, e.session_id,
            co.code AS course_code, p.first_name || ' ' || p.last_name AS personnel,
            s.date, s.start_time, s.end_time FROM invigilator_assignments a
            JOIN exam_rooms r ON r.id = a.id JOIN classrooms c ON c.id = r.classroom_id
            JOIN exams e ON e.id = r.exam_id JOIN courses co ON co.id = e.course_id
            JOIN personnel p ON p.id = a.personnel_id JOIN sessions s ON s.id = e.session_id
            WHERE ($1::bigint = 0 OR a.personnel_id = $1) AND ($2::bigint = 0 OR r.exam_id = $2) AND ($3::bigint = 0 OR e.session_id = $3)`, []any{f.PersonnelID, f.ExamID, f.SessionID}, nil
	case "audit-logs":
		return `SELECT a.* FROM audit_logs a WHERE ($1::bigint = 0 OR a.actor_user_id = $1)
            AND ($2 = '' OR a.entity = $2) AND ($3 = '' OR a.action = $3)
            AND ($4::date IS NULL OR a.created_at >= ($4::date::timestamp AT TIME ZONE 'UTC'))
            AND ($5::date IS NULL OR a.created_at < (($5::date + 1)::timestamp AT TIME ZONE 'UTC'))`, []any{f.ActorID, f.Entity, f.Action, nullableDate(f.DateFrom), nullableDate(f.DateTo)}, nil
	case "schedule/publication":
		return `SELECT * FROM schedule_state WHERE id = 1`, nil, nil
	}
	return "", nil, fmt.Errorf("unknown internal resource")
}

func ListResources(ctx context.Context, db rowQuerier, resource string, f ResourceFilter) (json.RawMessage, error) {
	query, args, err := resourceQuery(resource, f)
	if err != nil {
		return nil, err
	}
	return pageQuery(ctx, db, query, args, f, "id")
}

// order is an internal SQL expression; every caller supplies a fixed literal.
func pageQuery(ctx context.Context, db rowQuerier, query string, args []any, f ResourceFilter, order string) (json.RawMessage, error) {
	n := len(args)
	query = fmt.Sprintf(`WITH matching AS MATERIALIZED (%s)
        SELECT jsonb_build_object('data', coalesce((SELECT jsonb_agg(to_jsonb(p) ORDER BY %s)
        FROM (SELECT * FROM matching ORDER BY %s LIMIT $%d OFFSET $%d) p), '[]'::jsonb),
        'total', (SELECT count(*) FROM matching), 'page', $%d::integer, 'page_size', $%d::integer)`, query, order, order, n+1, n+2, n+3, n+1)
	args = append(args, f.PageSize, int64(f.Page-1)*int64(f.PageSize), f.Page)
	var body json.RawMessage
	err := db.QueryRow(ctx, query, args...).Scan(&body)
	return body, err
}

func GetResource(ctx context.Context, db rowQuerier, resource string, id int64) (json.RawMessage, error) {
	query, args, err := resourceQuery(resource, ResourceFilter{})
	if err != nil {
		return nil, err
	}
	query = fmt.Sprintf(`SELECT to_jsonb(t) FROM (%s) t WHERE t.id = $%d`, query, len(args)+1)
	var body json.RawMessage
	err = db.QueryRow(ctx, query, append(args, id)...).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return body, err
}

func (s Scheduler) create(ctx context.Context, actorID int64, entity, query string, args ...any) (int64, error) {
	var id int64
	err := s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "create", Entity: entity, Invalidate: true}
		if err := tx.QueryRow(ctx, query, args...).Scan(&id); err != nil {
			return value, err
		}
		value.ID = id
		var err error
		value.New, err = document(ctx, tx, entity, id)
		return value, err
	})
	return id, err
}

func (s Scheduler) CreateDepartment(ctx context.Context, actorID int64, name string) (int64, error) {
	return s.create(ctx, actorID, "departments", `INSERT INTO departments (name) VALUES ($1) RETURNING id`, name)
}

func (s Scheduler) UpdateDepartment(ctx context.Context, actorID, id int64, name string) error {
	return s.update(ctx, actorID, "departments", id, `UPDATE departments SET name = $2 WHERE id = $1`, id, name)
}

func (s Scheduler) CreateCourse(ctx context.Context, actorID int64, v CourseInput) (int64, error) {
	return s.create(ctx, actorID, "courses", `INSERT INTO courses (code, name, kind, enrollment, term, department_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, v.Code, v.Name, v.Kind, v.Enrollment, v.Term, v.DepartmentID)
}

func (s Scheduler) CreateSession(ctx context.Context, actorID int64, v SessionInput) (int64, error) {
	return s.create(ctx, actorID, "sessions", `INSERT INTO sessions (date, start_time, end_time, description) VALUES ($1, $2, $3, $4) RETURNING id`, v.Date, v.StartTime, v.EndTime, v.Description)
}

func (s Scheduler) CreateClassroom(ctx context.Context, actorID int64, v ClassroomInput) (int64, error) {
	return s.create(ctx, actorID, "classrooms", `INSERT INTO classrooms (name, capacity, type, floor, active) VALUES ($1, $2, $3, $4, $5) RETURNING id`, v.Name, v.Capacity, v.Type, v.Floor, v.Active)
}

func (s Scheduler) CreatePersonnel(ctx context.Context, actorID int64, v PersonnelInput) (int64, error) {
	return s.create(ctx, actorID, "personnel", `INSERT INTO personnel (title, first_name, last_name, department_id) VALUES ($1, $2, $3, $4) RETURNING id`, v.Title, v.FirstName, v.LastName, v.DepartmentID)
}

func (s Scheduler) DeleteMaster(ctx context.Context, actorID int64, resource string, id int64) error {
	switch resource {
	case "departments", "courses", "classrooms", "sessions", "personnel":
	default:
		return fmt.Errorf("unknown internal master resource")
	}
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "delete", Entity: resource, ID: id, Invalidate: true}
		var err error
		value.Old, err = document(ctx, tx, resource, id)
		if err != nil {
			return value, err
		}
		_, err = tx.Exec(ctx, `DELETE FROM `+resource+` WHERE id = $1`, id)
		return value, err
	})
}
