package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (d *Databases) Dashboard(ctx context.Context) (json.RawMessage, error) {
	var body json.RawMessage
	err := d.Administrator.QueryRow(ctx, `SELECT jsonb_build_object(
        'publication', (SELECT to_jsonb(t) FROM schedule_state t WHERE id = 1),
        'totals', jsonb_build_object('departments', (SELECT count(*) FROM departments),
            'courses', (SELECT count(*) FROM courses), 'sessions', (SELECT count(*) FROM sessions),
            'classrooms', (SELECT count(*) FROM classrooms), 'personnel', (SELECT count(*) FROM personnel),
            'exams', (SELECT count(*) FROM exams),
            'unplanned_courses', (SELECT count(*) FROM courses c WHERE NOT EXISTS (SELECT 1 FROM exams e WHERE e.course_id = c.id)),
            'pending_requests', (SELECT count(*) FROM unavailability_requests WHERE status = 'pending'),
            'unstaffed_rooms', (SELECT count(*) FROM exam_rooms r WHERE NOT EXISTS (SELECT 1 FROM invigilator_assignments a WHERE a.id = r.id))),
        'previews', jsonb_build_object(
            'unplanned_courses', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM
                (SELECT c.id, c.code, c.name FROM courses c WHERE NOT EXISTS (SELECT 1 FROM exams e WHERE e.course_id = c.id) ORDER BY c.id LIMIT 5) t),
            'pending_requests', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM
                (SELECT r.id, r.date, r.type, r.personnel_id, p.first_name || ' ' || p.last_name AS personnel FROM unavailability_requests r
                 JOIN personnel p ON p.id = r.personnel_id WHERE r.status = 'pending' ORDER BY r.id LIMIT 5) t),
            'unstaffed_rooms', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM
                (SELECT r.id, r.exam_id, c.code AS course_code, room.name AS classroom FROM exam_rooms r
                 JOIN exams e ON e.id = r.exam_id JOIN courses c ON c.id = e.course_id JOIN classrooms room ON room.id = r.classroom_id
                 WHERE NOT EXISTS (SELECT 1 FROM invigilator_assignments a WHERE a.id = r.id) ORDER BY r.id LIMIT 5) t)))`).Scan(&body)
	return body, err
}

func (d *Databases) Workload(ctx context.Context, f ResourceFilter) (json.RawMessage, error) {
	const query = `SELECT p.id, p.id AS personnel_id, p.title, p.first_name, p.last_name, p.department_id, d.name AS department,
        (SELECT count(*) FROM invigilator_assignments a JOIN exam_rooms r ON r.id = a.id
         JOIN exams e ON e.id = r.exam_id JOIN sessions s ON s.id = e.session_id
         WHERE a.personnel_id = p.id AND ($3::date IS NULL OR s.date >= $3) AND ($4::date IS NULL OR s.date <= $4)) AS duties
        FROM personnel p JOIN departments d ON d.id = p.department_id
        WHERE ($1::bigint = 0 OR p.department_id = $1) AND ($2::bigint = 0 OR p.id = $2)`
	return pageQuery(ctx, d.Administrator, query, []any{f.DepartmentID, f.PersonnelID, nullableDate(f.DateFrom), nullableDate(f.DateTo)}, f, "id")
}

func (d *Databases) OwnDuties(ctx context.Context, personnelID int64, f ResourceFilter) (json.RawMessage, error) {
	return readWithPersonnel(ctx, d.Invigilator, &personnelID, func(tx pgx.Tx) (json.RawMessage, error) {
		const query = `SELECT * FROM own_duties WHERE ($1::date IS NULL OR date >= $1) AND ($2::date IS NULL OR date <= $2)
            AND ($3::bigint = 0 OR course_id = $3) AND ($4 = '' OR strpos(lower(course_code || ' ' || course_name), lower($4)) > 0)`
		return pageQuery(ctx, tx, query, []any{nullableDate(f.DateFrom), nullableDate(f.DateTo), f.CourseID, f.Query}, f, "date, start_time, id")
	})
}

func (d *Databases) OwnWorkload(ctx context.Context, personnelID int64, f ResourceFilter) (json.RawMessage, error) {
	return readWithPersonnel(ctx, d.Invigilator, &personnelID, func(tx pgx.Tx) (json.RawMessage, error) {
		var body json.RawMessage
		err := tx.QueryRow(ctx, `SELECT to_jsonb(p) || jsonb_build_object('duties',
            (SELECT count(*) FROM own_duties WHERE ($1::date IS NULL OR date >= $1) AND ($2::date IS NULL OR date <= $2))) FROM own_personnel p`, nullableDate(f.DateFrom), nullableDate(f.DateTo)).Scan(&body)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return body, err
	})
}

type availabilityRow struct {
	PersonnelID  int64  `json:"personnel_id"`
	Personnel    string `json:"personnel"`
	DepartmentID int64  `json:"department_id"`
	Department   string `json:"department"`
	SessionID    int64  `json:"session_id"`
	Date         string `json:"date"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Available    bool   `json:"available"`
	ReasonCode   string `json:"reason_code"`
	Reason       string `json:"reason"`
}

func (d *Databases) Availability(ctx context.Context, f ResourceFilter) (json.RawMessage, error) {
	tx, err := d.Administrator.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	state, err := loadSchedule(ctx, tx)
	if err != nil {
		return nil, err
	}
	const query = `SELECT p.id AS personnel_id, p.first_name || ' ' || p.last_name AS personnel,
        p.department_id, d.name AS department, s.id AS session_id, s.date, s.start_time, s.end_time
        FROM personnel p JOIN departments d ON d.id = p.department_id CROSS JOIN sessions s
        WHERE ($1::bigint = 0 OR p.department_id = $1) AND ($2::bigint = 0 OR p.id = $2) AND ($3::bigint = 0 OR s.id = $3)
        AND ($4::date IS NULL OR s.date >= $4) AND ($5::date IS NULL OR s.date <= $5)`
	body, err := pageQuery(ctx, tx, query, []any{f.DepartmentID, f.PersonnelID, f.SessionID, nullableDate(f.DateFrom), nullableDate(f.DateTo)}, f, "personnel_id, date, start_time, session_id")
	if err != nil {
		return nil, err
	}
	var result struct {
		Data     []availabilityRow `json:"data"`
		Page     int               `json:"page"`
		PageSize int               `json:"page_size"`
		Total    int64             `json:"total"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	for index := range result.Data {
		row := &result.Data[index]
		err := state.personAvailable(row.PersonnelID, row.SessionID, 0)
		row.Available, row.Reason = err == nil, "Available for an additional duty."
		if err != nil {
			var rule *RuleError
			if !errors.As(err, &rule) {
				return nil, err
			}
			row.ReasonCode, row.Reason = rule.Code, rule.Message
		}
	}
	return json.Marshal(result)
}
