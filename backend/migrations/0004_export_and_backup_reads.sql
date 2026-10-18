CREATE VIEW current_schedule AS
SELECT e.id, c.id AS course_id, c.code AS course_code, c.name AS course_name,
       c.term, d.id AS department_id, d.name AS department,
       s.id AS session_id, s.date, s.start_time, s.end_time,
       COALESCE(jsonb_agg(jsonb_build_object('id', r.id, 'name', r.name, 'floor', r.floor)
           ORDER BY r.id) FILTER (WHERE r.id IS NOT NULL), '[]'::jsonb) AS rooms
FROM exams e
JOIN courses c ON c.id = e.course_id
JOIN departments d ON d.id = c.department_id
JOIN sessions s ON s.id = e.session_id
LEFT JOIN exam_rooms a ON a.exam_id = e.id
LEFT JOIN classrooms r ON r.id = a.classroom_id
GROUP BY e.id, c.id, d.id, s.id;

CREATE OR REPLACE VIEW published_schedule AS
SELECT * FROM current_schedule
WHERE EXISTS (SELECT 1 FROM schedule_state WHERE id = 1 AND published);
GRANT SELECT ON current_schedule TO exam_admin;

-- A complete application dump includes migration/seed markers; credentials live elsewhere.
CREATE TABLE IF NOT EXISTS seed_runs (name text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now());
GRANT SELECT ON schema_migrations, seed_runs TO exam_admin;
