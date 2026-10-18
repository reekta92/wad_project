package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduleFilter struct {
	Page, PageSize          int
	DateFrom, DateTo, Query string
	CourseID, DepartmentID  int64
	Term                    int
}

func Schedule(ctx context.Context, pool *pgxpool.Pool, f ScheduleFilter) (json.RawMessage, error) {
	return schedule(ctx, pool, f, false)
}

func schedule(ctx context.Context, pool *pgxpool.Pool, f ScheduleFilter, current bool) (json.RawMessage, error) {
	view := "published_schedule"
	if current {
		view = "current_schedule"
	}
	query := fmt.Sprintf(`WITH matching AS MATERIALIZED (
        SELECT * FROM %s
        WHERE ($1::date IS NULL OR date >= $1)
          AND ($2::date IS NULL OR date <= $2)
          AND ($3::bigint = 0 OR course_id = $3)
          AND ($4::bigint = 0 OR department_id = $4)
          AND ($5::integer = 0 OR term = $5)
          AND ($6 = '' OR strpos(lower(course_code || ' ' || course_name), lower($6)) > 0)
    )
    SELECT jsonb_build_object(
        'data', COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.date, p.start_time, p.id)
            FROM (SELECT * FROM matching ORDER BY date, start_time, id LIMIT $7 OFFSET $8) p), '[]'::jsonb),
        'page', $9::integer, 'page_size', $7::integer, 'total', (SELECT count(*) FROM matching))`, view)
	var result json.RawMessage
	err := pool.QueryRow(ctx, query, nullableDate(f.DateFrom), nullableDate(f.DateTo), f.CourseID, f.DepartmentID, f.Term, f.Query, f.PageSize, int64(f.Page-1)*int64(f.PageSize), f.Page).Scan(&result)
	return result, err
}

func nullableDate(value string) any {
	if value == "" {
		return nil
	}
	return value
}

const ExportLimit = 10000

var ErrExportLimit = errors.New("export exceeds its resource limit")

type ScheduleExam struct {
	ID           int64  `json:"id"`
	CourseID     int64  `json:"course_id"`
	CourseCode   string `json:"course_code"`
	CourseName   string `json:"course_name"`
	Term         int    `json:"term"`
	DepartmentID int64  `json:"department_id"`
	Department   string `json:"department"`
	SessionID    int64  `json:"session_id"`
	Date         string `json:"date"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Rooms        []struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Floor int    `json:"floor"`
	} `json:"rooms"`
}

func ExportSchedule(ctx context.Context, pool *pgxpool.Pool, f ScheduleFilter, current bool) ([]ScheduleExam, error) {
	f.Page, f.PageSize = 1, ExportLimit+1
	body, err := schedule(ctx, pool, f, current)
	if err != nil {
		return nil, err
	}
	if len(body) > 32*1024*1024 {
		return nil, ErrExportLimit
	}
	var page struct {
		Data  []ScheduleExam `json:"data"`
		Total int            `json:"total"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}
	if page.Total > ExportLimit {
		return nil, ErrExportLimit
	}
	return page.Data, nil
}
