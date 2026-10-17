package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduleFilter struct {
	Page, PageSize          int
	DateFrom, DateTo, Query string
	CourseID, DepartmentID  int64
	Term                    int
}

func Schedule(ctx context.Context, pool *pgxpool.Pool, f ScheduleFilter) (json.RawMessage, error) {
	const query = `WITH matching AS MATERIALIZED (
        SELECT * FROM published_schedule
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
        'page', $9::integer, 'page_size', $7::integer, 'total', (SELECT count(*) FROM matching))`
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
