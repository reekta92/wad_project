-- Invigilators can read their duties without receiving access to other personnel or schedules.
CREATE VIEW own_duties WITH (security_barrier = true) AS
SELECT a.id, a.personnel_id, e.id AS exam_id, c.id AS course_id, c.code AS course_code,
       c.name AS course_name, d.name AS department, s.id AS session_id, s.date,
       s.start_time, s.end_time, r.id AS classroom_id, r.name AS classroom
FROM invigilator_assignments a
JOIN exam_rooms allocation ON allocation.id = a.id
JOIN exams e ON e.id = allocation.exam_id
JOIN courses c ON c.id = e.course_id
JOIN departments d ON d.id = c.department_id
JOIN sessions s ON s.id = e.session_id
JOIN classrooms r ON r.id = allocation.classroom_id
WHERE a.personnel_id = nullif(current_setting('app.personnel_id', true), '')::bigint;

CREATE VIEW own_personnel WITH (security_barrier = true) AS
SELECT p.id, p.id AS personnel_id, p.title, p.first_name, p.last_name, p.department_id,
       d.name AS department
FROM personnel p JOIN departments d ON d.id = p.department_id
WHERE p.id = nullif(current_setting('app.personnel_id', true), '')::bigint;

GRANT SELECT ON own_duties, own_personnel TO exam_invigilator;
