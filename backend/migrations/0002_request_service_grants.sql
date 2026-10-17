-- Pending request writes use the invigilator pool and retain row/column restrictions.
GRANT SELECT (id, active, personnel_id) ON app_users TO exam_invigilator;
GRANT SELECT (id, date) ON sessions TO exam_invigilator;
GRANT INSERT (actor_user_id, action, entity, entity_id, old_value, new_value) ON audit_logs TO exam_invigilator;
GRANT USAGE ON SEQUENCE audit_logs_id_seq TO exam_invigilator;
