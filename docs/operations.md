# Operations and recovery

These procedures describe the delivered local application and its deployment constraints. The [root README](../README.md) remains the exact quick start and detailed manual restore reference. Commands below run from the repository root unless stated otherwise.

## Prerequisites and startup

Install Go ≥1.25, Node ≥22.12, Python 3, Docker with Compose, and a PostgreSQL 18 client with `pg_dump` on the API host's PATH. Current CI pins are recorded in [ci.yml](../.github/workflows/ci.yml). Default ports are PostgreSQL 5433, Keycloak 8081, Go 8080, and Vite 5173.

```sh
python3 scripts/setup.py
docker compose up -d --wait --wait-timeout 240
python3 scripts/run.py migrate
python3 scripts/run.py seed
python3 scripts/run.py provision
python3 scripts/run.py server
```

In a second terminal, install frontend dependencies with `npm ci` in `frontend/`, then run `python3 scripts/run.py web` from the repository root. Open `http://localhost:5173`. The public timetable requires no account; private generated demo credentials are in `.local/demo-accounts.json`. Do not paste that file into reports or commit it.

Setup preserves an existing `.env`. Migrations are transactional and checksum-checked. Demo seed is once-only, preserves later edits, and refuses a non-demo database. Provisioning is limited to the local development identity realm and preserves existing accounts/passwords/roles/mappings; conflicting links fail. It is not a production identity provisioning procedure. See the README for TOTP and passkey setup.

Stopping and starting services must preserve the PostgreSQL volume. Changing a password in `.env` does not rotate the password already stored inside the volume; repair the configuration or deliberately rotate roles. Never delete the volume as a configuration repair.

## Configuration ownership

| Setting / artifact | Purpose |
| --- | --- |
| `.env` generated from [.env.example](../.env.example) | Private local database/provider secrets and port settings; ignored by Git. |
| `DATABASE_READER_URL`, `DATABASE_ADMIN_URL`, `DATABASE_INVIGILATOR_URL` | Explicit runtime database URLs for the three restricted roles. Local fallbacks use the respective `EXAM_*_PASSWORD` and `POSTGRES_PORT`. |
| `DATABASE_OWNER_URL` / `EXAM_OWNER_PASSWORD` | Migration/seed owner connection, separate from runtime roles. |
| `OIDC_ISSUER` | Exact Keycloak realm issuer; defaults to `http://localhost:8081/realms/exams`. HTTPS required outside loopback. |
| `API_ADDRESS` | Listener, default `127.0.0.1:8080`; select an appropriate bind address for the actual deployment topology. |
| `STATIC_DIR` | Built React assets; default is relative to the backend working directory. The launch wrapper runs the server from `backend/`. |
| `APP_ENV=production`, `TLS_CERT`, `TLS_KEY` | Production mode requires both API TLS files and an HTTPS issuer. |
| `compose.yaml` / `infrastructure/` | Development PostgreSQL/Keycloak definitions, initial database roles, realm, and provider theme. |

Explicit database URLs are the path for non-local database configuration; the generated local fallback uses loopback and disables database TLS. Production database transport, certificates, secret storage, provider configuration, and network exposure remain deployment responsibilities.

## Build and delivery

```sh
cd frontend
npm ci
npm run build
cd ..
mkdir -p .local/bin
(cd backend && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o ../.local/bin/exam-server ./cmd/server)
python3 scripts/run.py serve
```

This serves the static frontend and API at `http://localhost:8080` in development. Runtime still needs PostgreSQL, Keycloak, and the host backup client. Migrations and identity setup are explicit steps before serving, not automatic server startup work.

The existing production demonstration uses [compose.https-demo.yaml](../compose.https-demo.yaml) in isolated CI with trusted temporary TLS certificates and `APP_ENV=production`. It proves the compiled arrangement under that environment; it is not a public deployment template or a hosted service. Its preparation script intentionally refuses ordinary workspace execution. Reproduce it through the existing CI job, not by rewriting a working local realm.

## Health and diagnosis

`GET /api/v1/health/live` reports the running process. `GET /api/v1/health/ready` checks all three database pools, required views, and exact issuer discovery; it returns 503 if a dependency is unavailable. Readiness does not test SMTP, backup tools, public certificate renewal, or every business operation.

| Symptom | Focused check / response |
| --- | --- |
| Empty public schedule | Administrator dashboard publication state, exam count, active filters; unpublishing intentionally returns zero public rows. |
| Login/profile rejected | Exact issuer/client/audience, local active link, personnel mapping, and administrator MFA. Reload requires selecting Sign in again because tokens are not persisted. |
| Provider login unavailable | Keycloak health/realm and issuer discovery; public schedule can still be read if the application database is available. |
| Database authentication fails after config edit | Compare intended role credentials with existing-volume state; editing `.env` did not rotate the role. |
| Scheduling/approval returns 409 | Read the domain error and current schedule/requests; resolve the conflict and retry deliberately. A planning failure can depend on ordering. |
| Export returns 413 | Narrow filters; all matching rows, not just the visible page, count toward bounds. |
| Backup returns 429 | Wait at least the supplied five-second retry interval. The single slot lasts through stream closure. |
| Backup returns 503 | Verify compatible host `pg_dump`, application admin connection, database reachability, temporary storage, and size/time bounds. No partial archive should be used. |

API errors include a request ID and sanitized message. Inspect service logs without retaining credentials, bearer headers, request reasons, private backups, or authenticated screenshots as general evidence.

## Backup and restore boundaries

Administrators download `application.dump` after the UI private-data confirmation. The API runs custom-format `pg_dump` against the application database, allows one active backup/download, and limits dump execution to 45 seconds and output to 256 MiB. Temporary credential/archive files are private and removed on failure or stream closure. The archive contains private personnel/request/audit/identity-link data and must be stored privately.

| Included | Must be preserved/backed up separately |
| --- | --- |
| Application tables, sequences, views, row policies, grants, audit history, local identity links, migration/seed markers. | PostgreSQL cluster roles/passwords, `.env`, demo credential file, Keycloak database/signing keys/authenticator state, TLS keys/certificates, and deployment configuration. |

For recovery:

1. Retain the source database and private configuration. Create a **new**, separately named database with the expected owner/roles and restricted connect grants, using the [manual restore commands](../README.md#downloads-and-restore).
2. Restore the custom archive with `pg_restore --single-transaction --exit-on-error --no-owner`. Do not restore over the working `exams` database.
3. Verify all application tables/sequences, constraints/views/grants, migration checksums, publication, reports, and compatible identity-provider links. Application recovery alone cannot reconstruct Keycloak users/MFA.
4. Only after review, deliberately switch the runtime database URLs and verify readiness and role-specific behavior. Preserve the previous database/configuration for rollback.

No automated production backup schedule, retention policy, restore-point objective, or recovery-time objective is delivered. Establish those for an actual hosting environment before relying on recovery guarantees. CI's restore comparison proves an isolated application clone, not full provider/infrastructure disaster recovery.

## Maintenance and rollback

Before a refinement, preserve current data/configuration, review schema compatibility, and run the [focused acceptance checks](maintenance.md). Add migrations instead of editing applied files. There is no down-migration command; rollback of a schema-changing release requires a reviewed compatible deployment or restoring a verified separate clone. Ordinary startup never resets, restores, clears, or republishes the schedule.
