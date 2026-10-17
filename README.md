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
python3 scripts/run.py provision
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

Use **Sign in** in the account panel at http://localhost:5173. Keycloak verifies the account with Authorization Code + PKCE S256; Go verifies JWT signatures, issuer, audience, token lifetime, role, MFA, and the active local identity link. There is no role-selector login.

Provisioning creates three fictional accounts with distinct generated credentials:

| Username | Access |
| --- | --- |
| `student.demo` | Member profile and published schedule |
| `invigilator.demo` | Member with the invigilator permission, linked to personnel record 1 |
| `admin.demo` | Administrator; requires a verified passkey or password plus TOTP |
| Guest | Public schedule without an account |

Open the private `.local/demo-accounts.json` locally to find each password and `totp_secret`. Add the secret to an authenticator app using TOTP, SHA-1, 6 digits, and a 30-second period. All demo password logins require the authenticator code; codes cannot be reused in the same period. After signing in, choose **Register a passkey** to enroll a supported device with user verification. Sign out and sign in again to use that passkey. If the provider offers your password first, choose **Try another way**, then your passkey.

The credential directory/file use modes 0700/0600 and are Git-ignored. Provisioning can upgrade the existing development realm without deleting it. Repeat runs preserve accounts, edited passwords, roles, local active status, and personnel links; conflicting identities/mappings cause an error instead of being overwritten. Keep the credential file if you keep the database volume. Replacing it will not reset existing account passwords. This demo command refuses production and non-local identity realms.

Keycloak administration is at http://localhost:8081. Its provider operator is `local-admin`; its generated password is in `.env`. This operator is separate from `admin.demo`. Application registration and password-reset email are disabled; a faculty operator must verify identity before resetting credentials or MFA in Keycloak. Business CRUD workflows follow authentication in the implementation sequence and remain unfinished.

Access and refresh tokens remain in browser memory. The adapter keeps only temporary login state/PKCE data in browser storage. Refresh tokens rotate; logout clears browser state and ends the provider session. An already-issued access JWT can remain usable until its five-minute expiry (plus the API's 30-second clock allowance); disabling the local account blocks it immediately. Reloading the page requires selecting Sign in again because tokens are not persisted.

The development Compose file binds database/provider ports to loopback and uses development HTTP. It is not a production deployment configuration. Go requires API TLS and an HTTPS issuer when `APP_ENV=production`; configure `TLS_CERT`, `TLS_KEY`, `OIDC_ISSUER`, and the appropriate database connection URL before production use.

Changing database passwords in `.env` does not change passwords inside an existing PostgreSQL volume. Keep existing credentials or rotate roles deliberately; do not delete the database volume to repair configuration.

## API and tests

Currently implemented:

- `GET /api/v1/health/live`: process liveness.
- `GET /api/v1/health/ready`: PostgreSQL and issuer-discovery readiness, returning 503 when a dependency is unavailable.
- `GET /api/v1/schedule`: public, sanitized, published schedule joining courses, departments, sessions, and classrooms. Lists use stable ordering and database-side filters/pagination. Private personnel and unavailability records are not exposed.
- `GET /api/v1/auth/config`: public browser sign-in configuration.
- `GET /api/v1/me`: verified local profile; requires a valid access token, an active linked account, and verified MFA for administrators. Responses use `Cache-Control: no-store`.

The implemented contract is in [docs/openapi.yaml](docs/openapi.yaml). Import [docs/postman/exams.postman_collection.json](docs/postman/exams.postman_collection.json) and its environment into Postman. The profile requires Authorization Code + PKCE sign-in; the other listed methods are public.

The scheduling service under `backend/internal/store/` enforces cohort spacing, room capacity/reservations, staff availability and duty limits. Its writes acquire one transaction-scoped advisory lock before rule reads, validate the final state, and commit audit records and publication invalidation atomically. Business CRUD API routes are the next implementation step.

```sh
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/run.py test
node frontend/src/api.test.js
```

After building the frontend and running migrate, seed, and provision against isolated local services, browser tests start Go and verify real Keycloak password/TOTP login, administrator MFA, user-verified passkey enrollment/login with a Chromium virtual authenticator, token refresh/revocation, and local account disabling:

```sh
cd frontend
npm ci
npm run build
npx --no-install playwright install chromium
npm run test:browser
```

Browser tests change a demo account's active status temporarily and enroll a test passkey; use a disposable development database. They require Docker Compose for the operator-side disabled-account check. Traces, videos, and screenshots are disabled so authentication secrets are not retained as test artifacts.

Database integration tests require a fresh, isolated database with the Compose-created roles:

```sh
RUN_DATABASE_TESTS=1 python3 scripts/run.py test
```

The lifecycle integration test edits and unpublishes its demo fixture; never point the suite at a database you use for demonstrations or real work. Scheduling integration tests create disposable schemas and verify final-state validation, rollback after cascading changes or audit failure, and actual advisory-lock contention between competing writers. CI starts its own isolated PostgreSQL and Keycloak services, verifies migrations/seeds, grants, publication filtering, pagination, and readiness, and builds both applications.

To serve the production frontend build from Go:

```sh
cd frontend
npm ci
npm run build
cd ..
python3 scripts/run.py server
```

Go serves `frontend/dist` at http://localhost:8080 in local development. Unknown `/api` paths remain JSON 404 responses rather than falling through to the SPA. External fonts have a system-font fallback when offline.
