# Exam Management System

Class project for WAD.

Rewrite of the legacy faculty exam scheduler using Go/Fiber, PostgreSQL, React/Vite/Tailwind, and Keycloak.

Local planning and report documents are kept in the ignored `development/` directory. Runtime database and identity-provider configuration is versioned under `infrastructure/`.

The original SQL Server application remains unchanged under the local `legacy/` directory. It is excluded from this replacement repository because it contains demo credentials and destructive setup scripts. The replacement does not connect to or modify its database.

Repository: https://github.com/reekta92/wad_project

This managed workspace makes `.git/` read-only. Its local Git metadata is stored in `.git-store/`; use `git --git-dir=.git-store --work-tree=.` here. A normal clone from GitHub uses standard Git commands.

## Run locally

Prerequisites: Go 1.25 or newer, Node.js 22.12 or newer (CI uses Go 1.27.1 and Node 24.21.0), Python 3, Docker, Docker Compose, and a PostgreSQL 18 client (`pg_dump` on the API host PATH; CI uses `postgresql-client-18`). Ports 5433, 8080, 8081, and 5173 must be available.

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

Targeted UI requirement evidence and accessibility limits are mapped in [docs/ui-evidence.md](docs/ui-evidence.md).

The frontend uses a Google Workspace inspired layout with a desktop sidebar, a compact mobile menu, rounded borderless surfaces, large controls, and Roboto typography. Light and dark themes pair Google-style neutral backgrounds with One Dark blue, green, and purple accents. Choose **Device theme**, **Light**, or **Dark** in the header; the browser remembers the preference independently of authentication. Keyboard focus remains visible, and reduced-motion and high-contrast preferences are respected.

Setup generates distinct local secrets in a private, ignored `.env`. Existing configuration is never overwritten. Migrations are transactional and checksum-checked. Demo seeding runs once, preserves later edits on repeat runs, and refuses to populate an existing non-demo database. None of these commands uses the legacy SQL Server database.

Use **Sign in** in the header at http://localhost:5173. Keycloak verifies the account with Authorization Code + PKCE S256; Go verifies JWT signatures, issuer, audience, token lifetime, role, MFA, and the active local identity link. There is no role-selector login.

Provisioning creates three fictional accounts with distinct generated credentials:

| Username | Access |
| --- | --- |
| `student.demo` | Member profile and published schedule |
| `invigilator.demo` | Member with the invigilator permission, linked to personnel record 1 |
| `admin.demo` | Administrator; requires a verified passkey or password plus TOTP |
| Guest | Public schedule without an account |

Open the private `.local/demo-accounts.json` locally to find each password and `totp_secret`. Add the secret to an authenticator app using TOTP, SHA-1, 6 digits, and a 30-second period. All demo password logins require the authenticator code; codes cannot be reused in the same period. After signing in, choose **Register a passkey** to enroll a supported device with user verification. Sign out and sign in again to use that passkey. If the provider offers your password first, choose **Try another way**, then your passkey.

The credential directory/file use modes 0700/0600 and are Git-ignored. Provisioning can upgrade the existing development realm without deleting it. Repeat runs preserve accounts, edited passwords, roles, local active status, and personnel links; conflicting identities/mappings cause an error instead of being overwritten. Keep the credential file if you keep the database volume. Replacing it will not reset existing account passwords. This demo command refuses production and non-local identity realms.

Keycloak administration is at http://localhost:8081. Its provider operator is `local-admin`; its generated password is in `.env`. This operator is separate from `admin.demo`. Application registration and password-reset email are disabled; a faculty operator must verify identity before resetting credentials or MFA in Keycloak.

After administrator sign-in, the sidebar provides **Departments**, **Courses**, **Classrooms**, **Sessions**, **Personnel**, **Requests**, **Exams**, and **Assignments**. Each area has filters, pagination, creation, details, full replacement edits, and confirmed deletion. Linked-record selectors load options in pages and preserve the current selection. Exams allocate rooms automatically; assignment creation selects an allocation from the owning exam. Invigilators see **My duties**, **My workload**, and **My requests** for their own records; students retain public schedule/profile access. Request ownership and decision fields cannot be changed through these forms.

The administrator **Dashboard** shows publication state, master/schedule counts, and the first five unplanned courses, pending requests, and unstaffed rooms. To build a timetable:

1. Create the courses, sessions, rooms, and personnel, then review pending **Requests**. Administrators can approve or reject a pending request once. Approval checks existing duties; a conflict leaves the request pending. Reviewed requests cannot be edited or withdrawn.
2. Use **Plan unplanned courses** for all terms, autumn (odd terms), or spring (even terms). It preserves existing exams and processes course IDs in order, choosing the earliest valid session and allocating rooms through the same rules as manual scheduling. This deterministic strategy does not search every ordering: a failed course rolls back the whole batch, and another manual ordering may succeed.
3. Use **Staff selected rooms** for all unstaffed rooms or one allocation. Selection prefers eligible personnel from the course department, then the lowest workload, then personnel ID. Existing duties remain. Failed batches commit no partial assignments.
4. Use **Publish timetable** to expose the schedule publicly. Publication requires at least one exam, valid allocations, and an invigilator for every allocated room. **Unpublish timetable** hides it immediately. Schedule-affecting writes, including linked master edits, invalidate publication. Valid request decisions preserve the timetable; approval cannot commit a duty conflict. Repeating a planning/staffing action with no work preserves publication.

**Clear schedule…** requires confirmation and permanently removes all exams, allocations, and duties, then unpublishes. Courses, rooms, sessions, personnel, requests, and audit history remain. Ordinary startup preserves edits and publication state.

Administrators also have filtered, paginated **Workload**, **Availability**, and **Audit log** reports. Workload includes zero-duty personnel; date filters change duty counts without hiding those personnel. Availability evaluates one additional duty using the scheduling service's leave, double-booking, daily, and consecutive-session limits. Audit details show the actor, UTC timestamp, and before/after values; its date filters use UTC days. Invigilator duties and workload are fixed to the verified personnel identity, including unpublished assignments. Exam/session dates and times use Europe/Istanbul. Authenticated users can export the filtered schedule as a two-sheet XLSX workbook; administrators also have private database backups. See the download/restore instructions below.

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

All eight business resources implement `POST /resource`, `GET /resource/{id}`, `PUT /resource/{id}`, `DELETE /resource/{id}`, and `GET /resource`: `/departments`, `/courses`, `/classrooms`, `/sessions`, `/personnel`, `/unavailability-requests`, `/exams`, and `/invigilator-assignments`. These 40 methods use authenticated JSON, strict field/filter validation, SQL filtering/pagination, 201 plus `Location` on creation, and empty 204 deletion responses. JSON, XML, and plain-text responses are supported, with Atom for collection reads. Unsupported representations return 406 before writes; unsupported request types return 415. Administrator MFA is required for business administration. Invigilator request operations use the restricted database pool, transaction-local ownership policies, pending-only writes, and transactional audit entries. Cross-owner records are concealed. All error responses remain JSON, including errors of non-JSON requests. [Representation rules and XML/Atom encodings](docs/representations.md) document quality values, exclusions, and type preservation.

There are 12 workflow methods plus two download methods (59 application operations in total): administrator `GET /dashboard`, `GET /reports/workload`, `GET /reports/availability`, `GET /audit-logs`, `GET /audit-logs/{id}`, `POST /unavailability-requests/{id}/decision`, `POST /schedule/automatic-planning`, `POST /invigilator-assignments/automatic`, `PUT /schedule/publication`, and `DELETE /schedule`; linked invigilators have `GET /me/duties` and `GET /me/workload`. All require verified authentication, use JSON and no-store responses, and enforce their role before accessing data. `GET /schedule/export` requires an authenticated member or administrator; `POST /backups` requires administrator MFA. Audit access is read-only. Bulk writes, decisions, and publication reuse the scheduling service's lock, validation, and transaction-owned audit.

The implemented contract is in [docs/openapi.yaml](docs/openapi.yaml). Import [docs/postman/exams.postman_collection.json](docs/postman/exams.postman_collection.json) and its environment into Postman. The profile and business resources require Authorization Code + PKCE sign-in; health checks, browser authentication configuration, and the published schedule are public.

The scheduling service under `backend/internal/store/` enforces cohort spacing, room capacity/reservations, staff availability and duty limits. Its writes acquire one transaction-scoped advisory lock before rule reads, validate the final state, and commit audit records and publication invalidation atomically. Every business CRUD write uses the appropriate transaction-owning service; pending member requests share the lock and commit their audit atomically without gaining schedule administration rights.

The Postman **Business CRUD (administrator)** folder creates related test fixtures, retrieves/replaces/lists them, and deletes them in dependency order. Run all three phases against a disposable demo database with an administrator token in the private environment's `access_token` value. Only generated test IDs are stored as collection variables; exported files contain no tokens or account credentials. The **Workflow operations (administrator)** folder contains individual dashboard/report, decision, planning/staffing, publication, and clear requests. Run these deliberately against a disposable database: clear removes the schedule, a decision needs a pending `decision_request_id`, and audit detail needs the `audit_id` saved by the list request. The **Own reports (invigilator)** folder requires an invigilator token in place of the administrator token. The request inventory is checked against OpenAPI in CI.

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

Browser tests change a demo account's active status temporarily, enroll a test passkey, and create/edit/delete business fixtures; use a disposable development database. Administrator browser journeys exercise all eight resource forms, details, edits, deletion confirmations, linked-record selectors, validation recovery, and narrow/tablet/desktop widths. Direct API checks cover all response representations, filtered XLSX values, private backup downloads with isolated restore proof, all 40 resource methods, missing/forbidden roles, list metadata, references, duplicates, and publication invalidation. Workflow journeys verify automatic planning/staffing, clear cancellation and confirmation, publication visibility, request decision conflict recovery/immutability, filtered workload/availability, audit details, personal duties/workload, report pagination, and responsive layouts. Invigilator journeys create/edit/withdraw only their own requests. They require Docker Compose for the operator-side disabled-account check. Authentication traces, videos, and screenshots are disabled so account secrets are not retained as test artifacts. Separate public layout checks use fixed schedule fixtures to verify themes, contrast, breakpoint boundaries, a 200% desktop zoom equivalent (720px CSS layout viewport), long errors, phone/tablet/desktop widths, keyboard navigation, filtering, paging, calendar scrolling, and the information dialog. Only those guest fixture pages produce layout previews; CI keeps them for seven days.

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

## Downloads and restore

Members use **Download XLSX** beside the published schedule controls. Administrators use **Export current XLSX** there or in Dashboard **Downloads**. Schedule search/date/department/course/term filters apply across all pages; `page` and `page_size` are rejected by the export API. Members see only published exams, while administrators export current exams even when unpublished. Calendar and List sheets share one database query snapshot, use Europe/Istanbul dates/times, and contain literal text cells so names beginning with `=`, `+`, `-`, or `@` cannot become formulas. IDs are text cells to preserve all digits. An empty result produces a valid workbook with headers.

Exports are limited to 10,000 exams, 32 MiB of source JSON, and 32,767 UTF-16 code units per spreadsheet cell. Narrow filters after a 413 response; exports are never silently truncated. Downloads use the in-memory bearer token in the request header, never a URL token or persisted credential.

Administrators select **Download database backup**, review the private-data notice, and confirm. The API runs a host `pg_dump` client against its configured application database, without a shell, Docker socket, caller paths, or command arguments. Install PostgreSQL client 18 from your distribution or the [official PostgreSQL repositories](https://www.postgresql.org/download/linux/); set PATH to include its binary directory when multiple versions are installed. A client older than the PostgreSQL 18 server cannot dump it. The local `development/start.py` launcher checks that `pg_dump` exists; an incompatible client or failed dump returns a sanitized 503 without sending a partial file.

The downloaded `application.dump` is a PostgreSQL custom archive. It includes application tables, sequences, views, policies, object grants, local identity links, audit history, and migration/seed markers. It contains private personnel/request data. Store it privately. It does **not** include cluster roles/passwords, `.env`, demo credential files, or the separate Keycloak database and signing/authenticator state. Keep the existing PostgreSQL roles and private configuration; back up Keycloak separately using provider/operator procedures before relying on application recovery.

Only one backup/download may be active at a time (429 with `Retry-After: 5` otherwise). Dump execution is limited to 45 seconds and 256 MiB. The server creates private temporary credentials/archive files, waits for a successful completed archive before starting the response, and removes temporary files on success, failure, or stream closure/disconnect. It uses the existing administrator database role with explicit read grants for migration/seed metadata and RLS policy access to all application requests; no schema or provider access is added to that role.

Restore into a **new** database first. These operator commands use the existing local Compose roles and never overwrite `exams`:

```sh
docker compose exec -T postgres createdb -U postgres --template=template0 --owner=exam_owner exams_restore_review
docker compose exec -T postgres psql -U postgres -d postgres -v ON_ERROR_STOP=1 -c 'REVOKE CONNECT ON DATABASE exams_restore_review FROM PUBLIC; GRANT CONNECT ON DATABASE exams_restore_review TO exam_owner, exam_admin, exam_invigilator, exam_reader;'
docker compose exec -T postgres pg_restore -U exam_owner --dbname=exams_restore_review --single-transaction --exit-on-error --no-owner < /private/path/application.dump
docker compose exec -T postgres psql -U exam_owner -d exams_restore_review -v ON_ERROR_STOP=1 -c 'SELECT count(*) AS exams FROM exams; SELECT count(*) AS duties FROM invigilator_assignments; SELECT e.id, c.code, s.date FROM exams e JOIN courses c ON c.id = e.course_id JOIN sessions s ON s.id = e.session_id ORDER BY e.id;'
```

The create command fails if `exams_restore_review` already exists. Review restored records, sequences, grants, migration state, and the matching Keycloak identity state before deliberately switching runtime database URLs. Normal startup never restores or deletes a database. The browser CI job downloads the actual HTTP archive, restores it into a random newly created database, compares all 14 application tables and sequences, verifies that the source digest is unchanged, and removes only that disposable clone. `scripts/verify_download.py` restricts this automated restore proof to CI.

Postman's **Downloads (administrator)** folder contains both methods; use **Send and Download** and a verified administrator token. The export request also works with a member token for published data. **Response representations** demonstrates XML/text/Atom and a 406 response on public schedule reads. No secrets or backup files are stored in the collection.
