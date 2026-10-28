# Faculty Exams

A faculty exam management application for scheduling exams, allocating rooms, assigning invigilators, and publishing timetables. Administrators manage scheduling and staffing; invigilators view their duties and submit availability requests; students browse the published schedule.

Built with a Go/Fiber API, React frontend, PostgreSQL, and Keycloak authentication. It also supports automatic planning, workload reports, schedule exports, and database backups.

## Run locally

Requires Python 3, Go 1.25+, Node.js 22.12+, npm, and Docker with Compose. Database backups also require a PostgreSQL 18-compatible `pg_dump` on the API server's `PATH`.

From the repository root:

```sh
python3 scripts/setup.py
docker compose up -d --wait
python3 scripts/run.py migrate
python3 scripts/run.py seed
python3 scripts/run.py provision
npm --prefix frontend ci
```

Start the API and frontend in separate terminals:

```sh
python3 scripts/run.py server
```

```sh
python3 scripts/run.py web
```

Open **http://localhost:5173**. The API listens on `127.0.0.1:8080`; Keycloak runs at **http://localhost:8081**. API, database, and Keycloak ports can be changed in `.env`.

Demo login credentials and authenticator secrets are generated in `.local/demo-accounts.json`. Add the account's `totp_secret` to an authenticator app to complete sign-in. Keep this file and `.env` private. The Compose setup uses Keycloak development mode and is intended for local use.

## Project layout

- `backend/` — API, business rules, SQL migrations, and database access.
- `frontend/` — React interface and development server.
- `infrastructure/` — database initialization and Keycloak configuration.
- `scripts/` — local setup and commands that load `.env`.
