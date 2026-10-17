# Exam Management System

Class project for WAD.

Rewrite of the legacy faculty exam scheduler using Go/Fiber, PostgreSQL, React/Vite/Tailwind, and Keycloak.

Local planning and report documents are kept in the ignored `development/` directory. Runtime database and identity-provider configuration is versioned under `infrastructure/`.

The original SQL Server application remains unchanged under the local `legacy/` directory. It is excluded from this replacement repository because it contains demo credentials and destructive setup scripts. The replacement does not connect to or modify its database.

Repository: https://github.com/reekta92/wad_project

This managed workspace makes `.git/` read-only. Its local Git metadata is stored in `.git-store/`; use `git --git-dir=.git-store --work-tree=.` here. A normal clone from GitHub uses standard Git commands.

## Run locally

Prerequisites: Go 1.25 or newer, Node.js 22.12 or newer (CI uses Go 1.27.1 and Node 24.21.0), Python 3, Docker, and Docker Compose. Ports 5433, 8080, 8081, and 5173 must be available.

```sh
python3 scripts/setup.py
docker compose up -d --wait --wait-timeout 240
python3 scripts/run.py migrate
python3 scripts/run.py seed
python3 scripts/run.py server
```

In another terminal:

```sh
cd frontend
npm ci
cd ..
python3 scripts/run.py web
```

Open http://localhost:5173. Vite proxies API requests to Go. The demo schedule contains three exams on 2 November 2026; course-name/code search, date filtering, pagination, list view, and calendar view query the PostgreSQL-backed API.

Setup generates distinct local secrets in a private, ignored `.env`. Existing configuration is never overwritten. Migrations are transactional and checksum-checked. Demo seeding runs once, preserves later edits on repeat runs, and refuses to populate an existing non-demo database. None of these commands uses the legacy SQL Server database.

Keycloak is at http://localhost:8081. Its provider administrator is `local-admin`; its generated password is in `.env`. The application realm/client exists with Authorization Code + PKCE, but application login, account provisioning, MFA, and protected CRUD workflows are still being implemented. There is no role-selector login or privileged API bypass.

The development Compose file binds database/provider ports to loopback and uses development HTTP. It is not a production deployment configuration. Go requires API TLS and an HTTPS issuer when `APP_ENV=production`; configure `TLS_CERT`, `TLS_KEY`, `OIDC_ISSUER`, and the appropriate database connection URL before production use.

Changing database passwords in `.env` does not change passwords inside an existing PostgreSQL volume. Keep existing credentials or rotate roles deliberately; do not delete the database volume to repair configuration.

## API and tests

Currently implemented:

- `GET /api/v1/health/live`: process liveness.
- `GET /api/v1/health/ready`: PostgreSQL and issuer-discovery readiness, returning 503 when a dependency is unavailable.
- `GET /api/v1/schedule`: public, sanitized, published schedule joining courses, departments, sessions, and classrooms. Lists use stable ordering and database-side filters/pagination. Private personnel and unavailability records are not exposed.

The implemented contract is in [docs/openapi.yaml](docs/openapi.yaml). Import [docs/postman/exams.postman_collection.json](docs/postman/exams.postman_collection.json) and its environment into Postman. No authentication is needed for these public methods; protected methods will be documented as they are implemented.

```sh
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/run.py test
node frontend/src/api.test.js
```

Database integration tests require a fresh, isolated database with the Compose-created roles:

```sh
RUN_DATABASE_TESTS=1 python3 scripts/run.py test
```

That integration test edits and unpublishes its demo fixture; never point it at a database you use for demonstrations or real work. CI starts its own isolated PostgreSQL and Keycloak services, verifies migrations/seeds, grants, publication filtering, pagination, and readiness, and builds both applications.

To serve the production frontend build from Go:

```sh
cd frontend
npm ci
npm run build
cd ..
python3 scripts/run.py server
```

Go serves `frontend/dist` at http://localhost:8080 in local development. Unknown `/api` paths remain JSON 404 responses rather than falling through to the SPA. External fonts have a system-font fallback when offline.
