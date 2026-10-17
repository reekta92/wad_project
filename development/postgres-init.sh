#!/bin/sh
set -eu

psql --username "$POSTGRES_USER" --dbname postgres --set=ON_ERROR_STOP=1 \
  --set=owner_password="$EXAM_OWNER_PASSWORD" \
  --set=admin_password="$EXAM_ADMIN_PASSWORD" \
  --set=invigilator_password="$EXAM_INVIGILATOR_PASSWORD" \
  --set=reader_password="$EXAM_READER_PASSWORD" \
  --set=identity_password="$KEYCLOAK_DB_PASSWORD" <<'SQL'
CREATE ROLE exam_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'owner_password';
CREATE ROLE exam_admin LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'admin_password';
CREATE ROLE exam_invigilator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'invigilator_password';
CREATE ROLE exam_reader LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'reader_password';
CREATE ROLE keycloak_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'identity_password';
CREATE DATABASE exams OWNER exam_owner;
CREATE DATABASE keycloak OWNER keycloak_owner;
REVOKE CONNECT ON DATABASE exams FROM PUBLIC;
GRANT CONNECT ON DATABASE exams TO exam_owner, exam_admin, exam_invigilator, exam_reader;
REVOKE CONNECT ON DATABASE keycloak FROM PUBLIC;
GRANT CONNECT ON DATABASE keycloak TO keycloak_owner;
SQL
