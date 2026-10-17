INSERT INTO departments (id, name) VALUES (1, 'Computer Engineering'), (2, 'Electrical Engineering');
INSERT INTO courses (id, code, name, kind, enrollment, term, department_id) VALUES
    (1, 'WAD301', 'Web Application Development', 'mandatory', 132, 5, 1),
    (2, 'DB301', 'Database Systems', 'mandatory', 60, 5, 1),
    (3, 'EE201', 'Electrical Circuits', 'mandatory', 80, 3, 2);
INSERT INTO classrooms (id, name, capacity, type, floor) VALUES
    (1, 'Lecture Hall', 150, 'lecture hall', 0),
    (2, 'A102', 100, 'classroom', 1),
    (3, 'A103', 80, 'classroom', 1);
INSERT INTO sessions (id, date, start_time, end_time, description) VALUES
    (1, '2026-11-02', '09:00', '10:00', 'Morning'),
    (2, '2026-11-02', '11:00', '12:00', 'Late morning'),
    (3, '2026-11-02', '13:00', '14:00', 'Afternoon');
INSERT INTO personnel (id, title, first_name, last_name, department_id) VALUES
    (1, 'Dr.', 'Ada', 'Demir', 1),
    (2, 'Dr.', 'Deniz', 'Yılmaz', 1),
    (3, 'Dr.', 'Ece', 'Kaya', 2);
INSERT INTO exams (id, course_id, session_id) VALUES (1, 1, 1), (2, 2, 3), (3, 3, 1);
INSERT INTO exam_rooms (id, exam_id, classroom_id) VALUES (1, 1, 1), (2, 2, 1), (3, 3, 2);
INSERT INTO invigilator_assignments (id, personnel_id) VALUES (1, 1), (2, 2), (3, 3);
UPDATE schedule_state SET published = true, updated_at = now() WHERE id = 1;
INSERT INTO audit_logs (action, entity, new_value)
    VALUES ('demo_seed', 'schedule', '{"source":"demo-v1","exams":3}'::jsonb);

SELECT setval(pg_get_serial_sequence('departments', 'id'), (SELECT max(id) FROM departments));
SELECT setval(pg_get_serial_sequence('courses', 'id'), (SELECT max(id) FROM courses));
SELECT setval(pg_get_serial_sequence('classrooms', 'id'), (SELECT max(id) FROM classrooms));
SELECT setval(pg_get_serial_sequence('sessions', 'id'), (SELECT max(id) FROM sessions));
SELECT setval(pg_get_serial_sequence('personnel', 'id'), (SELECT max(id) FROM personnel));
SELECT setval(pg_get_serial_sequence('exams', 'id'), (SELECT max(id) FROM exams));
SELECT setval(pg_get_serial_sequence('exam_rooms', 'id'), (SELECT max(id) FROM exam_rooms));
