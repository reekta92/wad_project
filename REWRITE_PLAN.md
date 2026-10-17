# Exam Management System Rewrite Plan

Status: planning only. No rewrite implementation, deployment, database reset, repository publication, or assessment upload has been performed.

This plan follows `MD_exam_management_system_REPORT.docx` and the assessment requirements supplied in the conversation. The report was checked before this plan was written. Its original contents are preserved in `MD_exam_management_system_REPORT.original.docx`.

## 1. Decisions and scope

- Build the replacement alongside `legacy/`: Go with Fiber, PostgreSQL, and React with Vite and Tailwind CSS. Go serves the production React build.
- Start with reproducible demo data, as requested. Do not migrate, reset, or otherwise modify the existing SQL Server database.
- Preserve `legacy/` as a reference and rollback option. Preserve scheduling behavior except for the explicitly documented corrections below.
- Use self-hosted Keycloak for local accounts, OpenID Connect, OAuth2 Authorization Code with PKCE, and signed JWT access tokens. Prefer passkeys with user verification; support password plus TOTP. Require the configured strong-authentication assurance level for administrators.
- Retain OAuth2 and JWT because the assessment explicitly requires them. Passkeys/MFA strengthen authentication; they do not replace OAuth2 authorization. No custom OAuth2 server, password grant, or implicit flow will be built.
- Use one application and one PostgreSQL instance, with separate application and Keycloak databases and owners. Keycloak is the additional service justified by the stronger authentication requirement.
- Complete the report's scheduling, approval, exports, backup, format negotiation, and SonarQube commitments as well as the assessment minimum. Do not reduce the system to three unrelated CRUD demos.

Only the report, its original backup, this plan, and `docs/architecture.dot` are deliverables of the current planning task. Paths and commands later in this document describe future implementation work.

## 2. Requirement audit performed before planning

**Verdict on the original report: it did not fully satisfy the supplied requirements.** It described most backend capabilities, but several UI criteria were missing or vague. Its authentication flow, role mapping, and database permissions needed correction. A specification cannot establish that software works, a collection runs, a repository is hosted, or Lab 0 was uploaded.

In the tables, **specified** means the original report explicitly required the behavior, not that an implementation passed. **Partial** means an important detail was missing. **Unverified** means delivery or runtime evidence was unavailable. Section references refer to the original report.

### Backend and delivery requirements

| ID | Supplied requirement | Original report assessment | Planned proof |
| --- | --- | --- | --- |
| B01 | Upload the Lab 0 project description. | Partial / unverified: section 1 contains a description; no upload receipt or submission URL. | Submit the revised report through the assessment system and record the receipt in M7. |
| B02 | Include at least three entities. | Specified: section 1.3 names five core entities. | M1 schema and M3 API cover eight independently managed business entities, with owned exam-room relationships. |
| B03 | Each entity has create, retrieve, update, delete, and list methods. | Specified in section 1.3, but no concrete route inventory or execution evidence. | Five operations per business resource in section 7; contract and integration checks in M3/M6. |
| B04 | Return at least one resource combining at least two entities. | Specified: dashboard, schedule, and duty lists in sections 1.2–1.3. | Joined schedule/dashboard/duty responses checked against seeded database records. |
| B05 | Implement pagination and filtering. | Specified for all list endpoints in section 1.3. | Database-side pagination, allowlisted filters, bounds and stable-order checks. |
| B06 | Relate the three or more entities. | Specified through foreign keys and joined resources in section 1.3. | Migration constraints and relationship tests in M1/M3. |
| B07 | Use a database. | Specified: PostgreSQL in sections 1.3–1.4. | Clean database migration and persistence demonstration. |
| B08 | Include at least guest, member, and administrator roles. | Partial: guest behavior is implicit; Student and Invigilator are named, but member is not mapped explicitly. | Exact role/profile matrix in section 5 and positive/negative authorization tests. |
| B09 | Implement authorization using OAuth2 and JWT. | Partial: section 1.3 says OAuth2/JWT, but no grant, issuer, validation, or token lifecycle is defined. | Keycloak Authorization Code + PKCE, signed access JWT validation, role/ownership enforcement. |
| B10 | Use local authentication, an external provider, or 2FA. | Specified: local hashed-password authentication in section 1.3; implementation unverified. | Keycloak local accounts and passkey/MFA login demonstration in M2. |
| B11 | Prepare client-server infrastructure locally or in the cloud. | Architecture specified in section 1.4; runnable replacement infrastructure unverified. | Local Go/React/PostgreSQL/Keycloak demonstration and production-build smoke check. |
| B12 | Prepare a development environment for quick startup. | Partial: containers and seeds mentioned in section 1.3; no replacement quick-start commands. | Fresh-checkout setup with pinned versions, migrations, seeds, and documented commands. |
| B13 | Design and implement a REST API. | Design intent specified in section 1.3; implemented REST coverage unverified. | OpenAPI resource paths, HTTP semantics, status codes, and executable contract checks. |
| B14 | API calls return accurately constructed responses in an appropriate format. | Formats specified in section 1.3; status/error contracts and executable evidence absent. | Browser/Postman calls verify content types, bodies, headers, and errors. |
| B15 | Implement all API methods. | Unverified: the report lists intentions, not a completed implementation. | Every OpenAPI operation has a handler, Postman request, and suitable test. |
| B16 | Prepare request collections for all API methods. | Specified in section 1.3; no collection found among inspected project files. | Committed Postman collection/environment covering every application operation and required auth flow. |
| B17 | All API methods work correctly. | Unverified; cannot be established by prose. | Passing endpoint, permission, invariant, and failure-path checks in M6. |
| B18 | Return correct data. | Unverified; no expected-versus-actual response evidence. | Assertions against known demo fixtures, relationships, filters, and ownership boundaries. |
| B19 | Implement authentication and authorization. | Specified in section 1.3; legacy login does not establish an authenticated identity. | Password/passkey verification by provider plus API-side role and ownership checks. |
| B20 | Store the project in a hosted repository. | Specified in section 1.3; this directory is not currently a Git repository and no hosted URL was supplied. | Hosted repository URL, assessment commit, and clean-clone verification in M7. |

### UI requirements

| ID | Supplied requirement | Original report assessment | Planned proof |
| --- | --- | --- | --- |
| U01 | Responsive design with at least two breakpoints. | Partial: alignment across resolutions is mentioned, but two breakpoints are not. | Explicit 640px and 1024px layouts; checks around both boundaries. |
| U02 | Responsive images using a max-width rule. | Missing: images are mentioned without a responsive-image rule. | A rendered image uses `max-width: 100%; height: auto`; computed-style and overflow check. |
| U03 | Different Header, Content, and Footer styles, with several elements each. | Partial: the three regions are named, but different styles and multiple elements are not explicit. | Distinct semantic regions and visual evidence described in section 9. |
| U04 | Data entry with several input types. | Specified: date pickers and combo boxes in section 1.3. | Real CRUD/request forms using text, number, date, time, checkbox, and select inputs. |
| U05 | Use animations. | Specified: CSS animations in section 1.3. | Visible short transitions and a reduced-motion check. |
| U06 | Responsive menu differing on large and small screens. | Partial: navigation is mentioned without mobile/desktop behavior. | Expandable mobile menu; persistent desktop navigation; keyboard checks. |
| U07 | Vector icons. | Specified: SVG graphics in section 1.3. | Rendered SVG icons with appropriate accessible names or decorative hiding. |
| U08 | Non-standard font. | Specified in section 1.3. | Loaded custom/Google font with a usable fallback. |
| U09 | Informational modal window. | Specified: warnings in section 1.3. | Conflict-details modal with title, focus management, dismissal, and focus return. |
| U10 | Keep elements aligned when resolution changes. | Specified in section 1.3; visual correctness unverified. | Grid/flex layout and screenshots at mobile, tablet, and desktop widths. |
| U11 | Keep items visible and accessible. | Partial: alignment is mentioned, but keyboard access, focus, contrast, and unobstructed controls are not defined. | Keyboard, zoom, contrast, menu/dialog, and overflow checks. |
| U12 | Clear and consistent forms. | Partial: input variety is specified, but consistent labels, validation, and layout are not. | Shared form conventions and successful/error-state review. |

### Changes made to the report after the audit

The revised report closes specification gaps; it does not claim implementation or submission is complete.

1. Authentication now uses Keycloak with OIDC, OAuth2 Authorization Code + PKCE (S256), JWT validation, passkeys/password-plus-TOTP, and required administrator MFA. Token issuance has moved out of Go. The deployment figure has been regenerated from `docs/architecture.dot`.
2. Guest, member, administrator, Student, and Invigilator are mapped explicitly. Restricted invigilator writes resolve the original contradiction between submitting requests and using a strictly read-only database user. Runtime administrators no longer need database-owner privileges.
3. Two breakpoints, responsive images/menu, distinct multi-element page regions, accessibility, and consistent forms are explicit.
4. The business resource inventory, HTTP semantics, collection evidence, and delivery evidence are explicit. Append-only audit records and provider-owned security records are not presented as unrestricted CRUD resources.
5. Cross-table uniqueness is no longer incorrectly promised as a simple PostgreSQL `UNIQUE` constraint over joined tables. The revised design retains normalization and serializes invariant-affecting writes in Go transactions. Table-local uniqueness remains declarative.
6. Cohort limits explicitly use department plus term, matching the legacy procedure. Minimum room count takes priority over floor preference. Atomic bulk behavior and the extra legacy invigilation limits are explicit.

Unresolved evidence remains B01 and B11–B20 where execution, hosting, or submission is required. All implementation acceptance remains future work.

## 3. Current system: behavior, readers, writers, and risks

### Traced behavior

| Concern | Current path and evidence | Rewrite owner |
| --- | --- | --- |
| Login and role checks | `legacy/web_app/views/login.ejs` submits role/personnel ID; `legacy/web_app/app.js:136` stores them directly in the session. `requireAdmin`, `requireGozetmen`, and `requireOgrenci` trust that session. | Keycloak verifies identity; Go validates access tokens and authorizes each operation. |
| Page reads | `/admin`, `/gozetmen`, and `/ogrenci` query SQL Server views/tables and render EJS. The admin page combines schedule, courses, sessions, rooms, staff, logs, and leave records. | React consumes documented Go resource/projection endpoints. |
| Scheduling writes | Form and AJAX handlers in `app.js` call `executeProcedure`/`runQuery`; `sp_SinavPlanla` in `legacy/final_db_setup.sql:365` performs scheduling and room allocation. | One scheduling service shared by every HTTP write path. |
| Invigilation | `sp_GozetmenAta`, `sp_GozetmenAtaManuel`, and `fn_GozetmenMusaitlikDurumu` check availability and assign staff. | The same transaction-owning service checks eligibility and workload. |
| Bulk writes | `sp_OtomatikPlanla` at line 706 and `sp_OtomatikGozetmenAta` at line 770 catch individual failures and keep prior successes. | Explicit atomic bulk transaction, as required by the revised report. |
| Leave records | Admin routes insert/delete `Personel_Durum`; no invigilator request/approval workflow or status column was found. | Owned request lifecycle and administrator decisions. |
| Reporting/export | `vw_SinavProgrami`, `vw_GozetmenGorevYuku`, and `/export-excel` join related data; Excel has calendar and list sheets. | SQL reporting views plus shared filtered queries and XLSX generation. |
| Audit | SQL triggers and `logToDatabase` write logs, sometimes separately from the business transaction. | Audit old/new values and authenticated actor in the business transaction. |
| Backup | `/admin/yedekle` invokes a SQL Server backup and shells out to Docker copy commands. | Admin-only, bounded `pg_dump` invocation without a shell. |
| Startup | `legacy/dev_setup.js` installs dependencies and executes the SQL Server setup file. | Documented non-destructive startup, versioned migrations, deterministic seeds. |

The current mutable data shape is: departments, courses, classrooms, sessions, personnel, personnel-status records, exams, room allocations, and invigilator assignments, plus audit logs. API/form handlers and stored procedures write it; EJS pages, AJAX endpoints, exports, and reporting views read it. One-off scripts also change schema/data and are not a migration framework.

### Findings that affect the rewrite

- Legacy authentication is a role selector, not password verification. It must not be copied or exposed as a real login system. Hardcoded credentials/session secrets must not become replacement configuration.
- Legacy JSON errors often return HTTP 200, and authorization middleware redirects API callers to HTML. Replace both with documented JSON errors and correct HTTP status codes.
- The schema has no authenticated-user mapping, request approval status, or publication state. Those are additions needed by the report, not already working legacy behavior.
- The legacy room allocator prefers a floor before minimizing room count. The report requires the reverse priority.
- The legacy availability function additionally caps duties at four per day and rejects a fourth consecutive duty. Automatic selection prefers the course department before the shared pool. Preserve these rules explicitly.
- Schedule-affecting edits to enrollment, room capacity/activity, session times, staff availability, or assignments can invalidate existing exams. They must pass the same rules as initial scheduling.
- `legacy/final_db_setup.sql:1` drops an existing database. `legacy/web_app/test_plan.js:19` deletes the schedule. Do not run these scripts against an existing database. The legacy `npm test` is a placeholder, and several diagnostic scripts print errors without failing reliably.

Inspection was static: the DOCX text and embedded figure were reviewed, routes and SQL rules were read, and callers were traced. A supplementary Graphify AST index reported 35 dangling-endpoint edges and 3 collapsed endpoint pairs; conclusions above were checked directly against source. The database and legacy scripts were not executed.

## 4. Target structure and request ownership

```text
WAD/
  legacy/                         # preserved reference; excluded from new startup
  backend/
    cmd/server/                   # configuration, pools, routes, static serving
    internal/auth/                # OIDC/JWT checks, role and ownership decisions
    internal/http/                # decoding, validation, handlers, representations
    internal/scheduling/          # shared invariants and transaction orchestration
    internal/store/               # parameterized PostgreSQL access and reporting
    migrations/                   # schema and grants; separate migration owner
    seeds/                        # deterministic demo fixtures
  frontend/src/                   # pages, shared form/layout conventions, API calls
  docs/
    architecture.dot
    openapi.yaml
    postman/
  compose.yaml                    # PostgreSQL and Keycloak for local development
  .env.example                    # placeholders; no usable passwords or private keys
  README.md                       # quick start, testing, production demonstration
  REWRITE_PLAN.md
```

This is a responsibility map, not a requirement to create empty packages or an interface for each file. Split only implemented responsibilities. Use `pgx` for PostgreSQL and explicit SQL; do not introduce a second ORM, generic CRUD framework, queue, or plugin system.

Request path: React or Postman sends an HTTP request; auth middleware validates identity and permissions; the handler validates input; the appropriate service owns the transaction and invariants; parameterized SQL persists changes and audit records; the handler encodes the documented response. Handlers never duplicate scheduling rules.

In development, Go and Vite run locally; Compose runs PostgreSQL and Keycloak. Vite proxies `/api` to Go. In the production demonstration, Go serves the built SPA and `/api/v1`; SPA fallback must never turn unknown API paths into HTML. Go and Keycloak both use HTTPS in production. The application and identity databases stay private to the host/service network.

## 5. Authentication and authorization contract

### Login and token lifecycle

1. Register an exact-redirect-URI public SPA client and the `exam-api` audience in Keycloak. Disable public registration, implicit flow, and password grant. No client secret belongs in React.
2. Use a maintained OIDC/Keycloak browser adapter with Authorization Code + PKCE S256, `state`, and `nonce`. Keycloak owns password hashing, credential verification, MFA enrollment, recovery, sessions, and signing keys.
3. Prefer passkeys with required user verification. Provide password plus TOTP as the alternative strong-login path. Administrators enroll before accessing privileged functionality; Go checks a configured assurance claim in addition to role membership. Prove this with a real token from the configured provider.
4. Keep access and refresh tokens in memory, never URLs or browser persistent storage. Use short-lived access tokens, initially five minutes, and provider-supported refresh-token rotation and bounded sessions. Expiry requires refresh or login.
5. Go verifies the signature and an allowlisted algorithm, trusted issuer, API audience, expiry/not-before, and trusted role claims using cached discovery/JWKS. Reject ID tokens used as access tokens, unknown/untrusted issuers, malformed tokens, and missing API audience. Handle key rotation without accepting an unverified token.
6. Map `(issuer, subject)` to a local active user and, for invigilators, one personnel record. Never let request bodies choose identity or elevate a role. Disabled local users cannot operate even with an otherwise valid JWT.
7. Logout clears browser state and ends/revokes the provider session/refresh capability. An already issued access token may remain valid until its short expiry unless the local account is disabled; document and test this limit. JWTs are not encrypted storage for sensitive data.
8. Use exact CORS origins where needed, restrictive CSP, safe text rendering, HTTPS, secure provider cookies, CSRF protection on provider forms, login throttling, audited MFA resets, and administrator-verified recovery. Do not bypass MFA to make the demo convenient.

### Roles and resource ownership

| Access category | Identity/profile | Allowed behavior |
| --- | --- | --- |
| `guest` | No authenticated identity. | Landing page and sanitized published schedule with date/course filtering; no private personnel, requests, logs, or writes. |
| `member` | Authenticated Student profile. | Published schedule and Excel download; own basic profile; no administration or invigilator requests. |
| `member` + `invigilator` | Authenticated member linked to a personnel record. | Member reads, own duties/workload, and CRUD/list for their own pending unavailability requests. No decisions on requests. |
| `administrator` | Authenticated administrator with required strong-login assurance. | Business CRUD, scheduling, request decisions, reporting, publication, exports, audit reads, and backups. |

Guests are an explicit authorization category, not accounts with a shared password. Administrator claims and the invigilator permission are provisioned through Keycloak administration, never through a public role field. Provider account management uses Keycloak's existing administrative interface; do not build a second identity-management UI.

Use fixed database roles selected from validated authorization context: a permitted-view reader, a narrowly scoped invigilator writer, and an application administrator. Runtime roles cannot run schema migrations or access the Keycloak database. Member request writes require column grants and PostgreSQL row policies or equivalent database restrictions on owner and pending status. Set any database identity context transaction-locally from the validated local user; pooled connections must not retain another user's context. Audit tables permit append/read according to role, never user-facing update/delete.

## 6. Data model and invariants

### Data shape

| Entity | Main fields and relationships |
| --- | --- |
| Department | ID, name; referenced by courses and personnel. |
| Course | ID, unique code, name, kind, positive enrollment, term 1–8, department FK. |
| Classroom | ID, name, positive capacity, type, floor, active flag. |
| Session | ID, date, start time, end time, description; start must precede end. |
| Personnel | ID, title, first/last name, department FK; optional local-user link is unique. |
| Unavailability request | ID, owner/personnel FK, date, optional session FK, type/reason, status `pending/approved/rejected`, decision actor/time. |
| Exam | ID, course FK with UNIQUE, session FK. |
| Exam-room allocation (owned relationship) | ID, exam FK, classroom FK; unique exam/classroom pair. Created/replaced/deleted atomically through the owning exam, not managed as an independent CRUD resource. |
| Invigilator assignment | Exam-room allocation FK as primary key, personnel FK; at most one invigilator per allocation. |

Supporting records are local identity links, append-only audit entries, and one schedule-state row containing publication state. Keycloak owns passwords and provider security records in its separate database. Reporting views join exams, courses/departments, sessions, classrooms, and assignments; workload views include personnel with zero duties.

Keep the application schema normalized. A PostgreSQL `UNIQUE` constraint cannot directly constrain joined `classroom_id + exam.session_id` or `personnel_id + exam.session_id`; do not claim otherwise or silently duplicate session keys. Use local PK/FK/UNIQUE/CHECK constraints and the shared transaction boundary below for cross-table rules.

### Transaction boundary

Every invariant-affecting write begins a `READ COMMITTED` transaction and acquires one fixed transaction-scoped PostgreSQL advisory lock with `pg_advisory_xact_lock` in a separate statement **before** reading rule inputs. This includes CRUD on linked courses, sessions, rooms, personnel, requests, exams, owned allocations, assignments, and publication. All write paths use this order, including bulk operations and request approval. This deliberately serializes writes at faculty-demo scale, avoids check-then-insert races, and releases the lock automatically on commit/rollback. Readers do not acquire it; invigilators need no permission to update publication state merely to acquire the lock.

Within the transaction, validate final proposed state, apply changes, append the audit entry, and unpublish if published schedule content changed. Commit once. On validation, database, audit, or allocation failure, roll back everything. Unexpected errors are not swallowed. Automatic planning checks known candidate conflicts before writing; if candidate evaluation requires writes, isolate it with a savepoint. Never continue using a PostgreSQL transaction after an unhandled statement error. No other application/script may write domain tables outside this path.

### Required rules and focused proofs

| Rule | Enforcement and proof |
| --- | --- |
| One exam per course. | `UNIQUE(course_id)` plus conflict response; duplicate and concurrent-create tests. |
| Same department and term cannot share a session. | Joined cohort check while locked; different-department control case. |
| At most two daily exams per department-and-term cohort; at least one free session between them. | Order the day's non-overlapping sessions by start time and ID; test second adjacent exam, third daily exam, and valid separated sessions. |
| A classroom cannot host two exams in a session. | Check allocations joined to exams under the shared lock; concurrent allocation test. |
| Rooms cover enrollment, minimizing count before same-floor preference. | Consider only active, free rooms. Sort by capacity descending, then ID; the shortest capacity prefix defines minimum count `k`. If any single floor's largest `k` rooms cover enrollment, use such a floor with stable tie-breaking; otherwise use the global prefix. A smaller set cannot exceed the largest `k-1` capacities. Test capacity shortage and a floor-first counterexample. |
| No invigilator double-booking or approved-unavailability conflict. | Check session/date conflicts under the same lock. Pending/rejected requests do not block duties; approval rechecks and rejects a conflict. |
| Four duties per day maximum; never four consecutive sessions. | Shared eligibility check used by manual and automatic assignment; boundary cases include inserting into the middle of a consecutive run. |
| Balanced, deterministic automatic staffing. | Among eligible staff, choose least total duties in the course department first, then the shared pool; break ties by personnel ID. This is the retained legacy policy, not a claim of globally optimal fairness. |
| Valid edits cannot invalidate existing schedules. | Revalidate affected exams/assignments when enrollment, capacity, active state, dates/times, departments/terms, or approved leave changes. Reject conflicting final state with 409; no partial edits. |
| Requests follow an authorized lifecycle. | Members create pending requests for themselves, edit/delete only their own pending records; administrators decide pending requests. Session-specific requests derive their date from that session. |
| Bulk scheduling/assignment is atomic. | Use one outer transaction and the same rule functions. If any targeted course/room remains unresolved, return 409 with reasons and roll back all new work from that action. Existing schedule stays intact. |
| Publication is explicit. | Guest/member public-schedule reads require published state. Successful schedule-affecting edits atomically unpublish; an administrator republishes after validation. Do not add a separate revision/snapshot subsystem. |
| Audit describes committed changes. | Old/new values, validated actor, UTC timestamp, and operation are written in the same transaction. Rollback leaves neither partial business data nor a success audit entry. |

Sessions use faculty-local dates/times, with Europe/Istanbul for demo presentation and UTC audit timestamps. Prevent overlapping sessions for the same date so distinct session IDs cannot disguise double-booking. Deleting referenced departments/courses/sessions/rooms/personnel returns 409; deleting an exam intentionally removes only its allocations and assignments in the same audited transaction.

Automatic planning is deterministic and tries courses and sessions in documented order; it is not an optimal timetabling solver. Failure identifies unresolved targets and leaves no partial batch. Do not silently claim infeasibility merely because a heuristic could not find a complete schedule.

## 7. Complete REST API surface

Base path: `/api/v1`. Every route below must appear in OpenAPI and Postman. IDs and filters are validated; no client controls a SQL identifier or sort expression.

### Five methods for every mutable business entity

For **each** row, implement all five operations: `POST /resource` (create), `GET /resource/{id}` (retrieve), `PUT /resource/{id}` (update), `DELETE /resource/{id}` (delete), and `GET /resource` (list). That is 40 business-resource operations across eight independently managed entities. Full replacement semantics for PUT and immutable/server-owned fields are defined in OpenAPI. Exam-room allocations are owned relationships returned and changed through the exam resource, not an additional independently mutable entity.

| Resource | List filters | Permissions and invariant owner |
| --- | --- | --- |
| `/departments` | `name` | Administrator; referenced deletion is restricted. |
| `/courses` | `department_id`, `term`, `kind`, `q` | Administrator; enrollment/cohort edits revalidate exams. |
| `/sessions` | `date_from`, `date_to` | Administrator; date/time edits revalidate ordering and duties. |
| `/classrooms` | `active`, `floor`, `min_capacity` | Administrator; capacity/activity edits revalidate allocations. |
| `/personnel` | `department_id`, `q` | Administrator; identity linkage is protected from member edits. |
| `/unavailability-requests` | `personnel_id`, `date_from`, `date_to`, `status`, `type` | Administrator, or invigilator owner subject to the pending-only rules. Status/decision fields cannot be set through member CRUD. |
| `/exams` | `course_id`, `department_id`, `term`, `session_id`, `date_from`, `date_to` | Administrator; create/update invokes scheduling and allocation. |
| `/invigilator-assignments` | `personnel_id`, `exam_id`, `session_id` | Administrator; create/update checks the same staffing rules as automatic assignment. |

Assignment endpoints do not bypass the scheduling service. Exam create/update allocates or replaces the complete room selection atomically; deletion removes the owned allocations. This avoids standalone allocation create/delete methods that could never succeed while maintaining minimum room count and sufficient capacity. A mutation that leaves too few rooms, a duplicate reservation, or an invalid duty returns 409.

### Combined reads and report-specific operations

| Method and path | Behavior and access |
| --- | --- |
| `GET /schedule` | Guest/member published schedule joining courses, departments, sessions, classrooms, and permitted assignment fields. Supports date/course/department/term filters and pagination; omits private personnel/request/audit information. |
| `GET /dashboard` | Administrator totals and bounded previews from several entities. Preview links lead to separately paginated resources; never return every table unbounded. |
| `GET /me` | Authenticated local profile, trusted permissions, and personnel linkage. |
| `GET /me/duties` | Invigilator's own joined course/session/room duties, with date/course filters and pagination. |
| `GET /me/workload` | Invigilator's own duty totals, optionally filtered by date range. |
| `GET /reports/workload` | Administrator workload list, filterable by department/personnel/date and paginated. |
| `GET /reports/availability` | Administrator availability rows, filterable by session/personnel/department and paginated. |
| `GET /audit-logs` and `GET /audit-logs/{id}` | Administrator read-only audit access; list supports actor/entity/action/date filters and pagination. |
| `POST /unavailability-requests/{id}/decision` | Administrator approves or rejects a pending request with a documented decision body; rechecks conflicts in the transaction. |
| `POST /schedule/automatic-planning` | Administrator atomically schedules all currently unplanned courses, optionally filtered to autumn/spring term. Response reports completed targets or conflict reasons. |
| `POST /invigilator-assignments/automatic` | Administrator assigns one targeted allocation, or atomically fills all unstaffed allocations using an explicit scope field. |
| `DELETE /schedule` | Administrator clears exams/allocations/assignments atomically; preserves master data and audit history. Requires the UI confirmation flow. |
| `PUT /schedule/publication` | Administrator publishes or unpublishes with an explicit boolean; publishing validates the current schedule. |
| `GET /schedule/export` | Authenticated members get published schedule; administrators can export current schedule. XLSX contains calendar and list sheets, uses documented filters, and uses the same authorization/query policy as schedule reads. |
| `POST /backups` | Administrator with strong authentication downloads a completed application-database `pg_dump` archive. No caller-supplied command/path. |
| `GET /health/live` and `GET /health/ready` | Minimal operational status without secrets; readiness verifies required configuration/dependencies. |

Keycloak's discovery, authorization, token, JWKS, and logout endpoints remain provider-owned protocol endpoints. Document their configured URLs and interactive Postman PKCE flow; do not duplicate them under the business API.

### Pagination, responses, and representation rules

- Every list endpoint accepts `page` (integer, minimum 1) and `page_size` (1–100, default 20). Apply filtering and authorization in SQL before counting/pagination. Use deterministic ordering with a unique ID tie-breaker. Out-of-range pages return an empty list and accurate metadata; malformed parameters return 400.
- JSON list shape is `{ "data": [], "page": 1, "page_size": 20, "total": 0 }`. Detail/create/update responses contain the documented resource. Validate required fields, ranges, foreign keys, body size, and server-owned fields; reject forbidden ownership/status changes.
- Creates return 201 and `Location`; reads/updates return 200; deletes return 204 without a body. Return 400 for invalid input, 401 for absent/invalid authentication, 403 for forbidden operations, 404 for missing or concealed unowned resources, 409 for business/reference conflicts, 406 for unsupported response formats, and 415 for unsupported request types. Unexpected failures return sanitized 500 errors with a traceable request ID.
- Ordinary resource responses support JSON, XML, and plain text; collection reads additionally support Atom feeds. Honor `Accept` quality values, default absent/`*/*` to JSON, set `Content-Type` and `Vary: Accept`, and document 406 cases. XML/text must preserve the resource fields; Atom requires valid feed metadata/entries. OAuth/OIDC, XLSX, and backup endpoints retain their appropriate protocol/binary formats.
- Use one documented structured error model, including a stable code, message, optional field errors, and request ID. Document error media types and test representative negotiation failures. Never leak SQL, stack traces, tokens, password hashes, or credentials.
- Export is intentionally not page-truncated: apply its filters and permission policy, produce both sheets consistently, and bound execution/resources. Complete `pg_dump` successfully before starting a backup download; use a process argument vector, a timeout, protected temporary files/credentials, and cleanup on both success and failure. Never mount the Docker socket into the API.

## 8. Development, compatibility, and rollback

The replacement uses a new PostgreSQL database and separate local ports. Old Node/EJS readers/writers continue to use SQL Server; new React/Go readers/writers use PostgreSQL. There is no shared mutable database, mixed-schema interval, live-data import, or dual writing. Demo records are fixtures, not a synchronization mechanism.

The future README must give exact supported Go/Node/Compose versions and commands. The intended sequence is:

```text
Copy .env.example to an ignored local environment file and supply secrets.
docker compose up -d
Run the documented backend migration command using the migration owner.
Run the idempotent demo-data/account setup command against the demo environment.
Start Go from backend/ with go run ./cmd/server.
Install locked frontend dependencies with npm ci; start Vite with npm run dev.
```

M0 must choose and implement the exact migration/seed command; placeholders above are not claimed to work today. Keycloak configuration must be reproducible, with separate public/browser and service-network URLs tested for issuer consistency. Local passkeys require a supported secure context such as localhost; production requires HTTPS and a stable relying-party hostname.

Seeds create stable department/course/session/room/personnel fixtures and separate student, invigilator, and administrator demo accounts. Read initial credentials from ignored local configuration; do not commit reusable passwords, private signing keys, exported provider secrets, or real personnel data. Rerunning seeds must not duplicate records, reset existing passwords, silently overwrite edited data, or clear schedules. Setup errors fail visibly.

Forward path: create the new database, migrate, seed, implement, verify, and demonstrate from a clean checkout. Only then select the replacement for normal demonstration/use. No legacy removal is required to finish the rewrite.

Rollback: stop the replacement and use the unchanged legacy source/SQL Server environment locally, if needed for comparison. This preserves existing data but does not make the legacy authentication safe for public exposure. Back up PostgreSQL before future destructive schema changes; verify restore into an isolated database. Never delete a volume/database or run destructive down/reset scripts as part of ordinary startup or rollback.

## 9. UI delivery map

| Screen/area | Required behavior |
| --- | --- |
| Shared layout | Header: brand/logo, navigation, identity/login control. Content: heading, filters/forms, results. Footer: project information, contact/help, repository/documentation links. Distinct styles for all three regions. |
| Landing and schedule | Guest-accessible landing page and published list/calendar views; date/course filters; explicit empty/unpublished/error states; pagination; responsive image. |
| Authentication | Keycloak login/enrollment screens; clear login/logout state; no role selector that authenticates a user. Explain recovery without exposing implementation secrets. |
| Administrator dashboard | Totals/previews, master-data CRUD screens, exams/rooms/assignments, automatic planning/staffing, schedule clearing/publication, availability/workload, audit reads, exports, and backup. |
| Invigilator area | Own duties/workload, create/edit/withdraw pending requests, request status/history. No access to other members' requests. |
| Student area | Published schedule and Excel download. |
| Forms | Consistent field labels, help text, required indicators, field/group errors, submit/cancel placement, and loading/disabled states. Use the actual input types listed in the report. Preserve input after server validation errors. |
| Modal | Accessible conflict-information modal and destructive-action confirmation; visible title, initial focus, focus containment/return, Escape dismissal where safe. A native dialog is sufficient if it meets these behaviors. |

Use mobile-first layouts with breakpoints at 640px and 1024px. Below 640px, provide an expandable menu and stacked forms/cards; at medium widths use suitable multi-column space; at 1024px and above show persistent navigation and wider dashboard/calendar layouts. Use CSS grid/flex rather than fixed coordinates. Keep wide tables/calendars in labeled scroll containers and retain a usable list view; never hide required actions to make a screenshot fit.

Use a real visible image with `max-width: 100%; height: auto`, SVG icons, and a non-standard font with fallback. Prefer existing/native UI facilities over a new component system. Animate menu/dialog/loading transitions briefly and respect `prefers-reduced-motion`. Ensure labels, focus visibility, readable contrast, approximately 44px primary touch targets, 200% zoom usability, and no overlapping menu/modal controls.

Check 375px, 639px, 640px, 768px, 1023px, 1024px, and 1440px widths. Exercise keyboard-only navigation, screen-reader labels for key forms/dialogs, long names/errors, empty lists, permission failures, and repeated submit prevention. Save representative mobile/tablet/desktop evidence; do not treat screenshots alone as accessibility proof.

## 10. Implementation milestones and exit conditions

Do milestones in order. Each ends with a working, verifiable increment. Do not create later scaffolding before it has a consumer.

| Milestone | Work and affected surfaces | Exit condition |
| --- | --- | --- |
| **M0 — Contract and runnable foundation** | Read this plan/revised report; choose compatible supported versions; define OpenAPI operations/roles/error/representation contracts; initialize the intended repository; add Go/Fiber and Vite/Tailwind entry points, environment example, Compose PostgreSQL/Keycloak, migration/seed commands, and quick-start README. Do not publish to an unspecified remote. | Fresh local setup starts all components without touching SQL Server; Go health and a React API read work; exact setup commands documented. |
| **M1 — Schema and transaction core** | Implement migrations, fixed runtime/migration database roles, local identity links, eight business entities and owned room relationships, audit/publication state, reporting views, deterministic fixtures, and the shared transaction lock. | Clean migration succeeds; constraints and grants work; idempotent reseed preserves edits; one scheduled exam can commit or roll back with its audit. |
| **M2 — Real identity and permissions** | Provision Keycloak configuration/clients/roles/demo accounts; integrate frontend login and backend JWT/JWKS checks; map personnel/ownership; configure passkeys/TOTP and administrator assurance. | Student, invigilator, administrator, and guest follow the matrix; admin login without required assurance fails; forged/expired/wrong-audience tokens and cross-owner access fail. |
| **M3 — CRUD API with first usable screens** | Implement all 40 resource operations, shared validation/errors/negotiation, pagination/filtering, entity forms/list/detail pages, and matching Postman requests. All domain writes use the transaction-owning service. | Each entity completes create/detail/update/list/delete through the API; relationships persist; filters/paging and forbidden operations behave correctly; core forms consume the API. |
| **M4 — Scheduling and report workflows** | Complete room allocation, automatic scheduling/staffing, request decisions, shared rule enforcement on edits, publication, joined dashboard/duties/workload/availability, transactional audit, and their frontend actions/collection requests. | Invariant tests and concurrency tests pass; a failed bulk action leaves no new partial data; public schedule stays hidden until publication; member ownership is demonstrated. |
| **M5 — Exports, backup, and responsive UI** | Add two-sheet XLSX exports, safe application-database backup, remaining screens, both breakpoints/menu behavior, images/font/SVG/animations, modal/accessibility behavior, and representation coverage. | Export values match filtered database data; backup restores to an isolated database; UI requirement evidence U01–U12 is complete. |
| **M6 — Verification and production-build demonstration** | Finish contract/collection coverage, realistic DB tests, browser journeys, lint/build checks, SonarQube analysis, and single-host Go-served production build. Fix failures rather than documenting broken required methods as done. | Every OpenAPI operation has an executable request and correct handler; focused suites/collection/build pass; role boundaries and deployed-format responses verified. |
| **M7 — Assessment evidence and handoff** | Review report/README against delivered behavior, commit source/tests/contracts/collection, use the designated hosted repository, record URLs/commit/test results, and submit Lab 0 through the designated assessment channel. | B01–B20 and U01–U12 each have actual evidence. Repository and upload claims are backed by URLs/receipts. No secrets or real data are committed. |

Repository destination and the Lab 0 submission channel are not supplied. They are delivery details to obtain when M7 is reached, not reasons to block this local plan. Creating documentation does not authorize sending messages or submitting to an unknown destination.

## 11. Focused verification and final acceptance

- **Database/service tests:** Use an isolated PostgreSQL test database, not SQL Server or a developer's active data. Cover each scheduling rule in section 6, rollback including audit failure, request approval conflicts, referenced deletes, publication invalidation, pooled identity isolation, and two simultaneous writers attempting the same room/person. Verify persisted rows, not only HTTP success.
- **API contract tests:** Exercise every operation's valid path and relevant validation/permission/not-found/conflict paths. Assert response status/type/body, `Location`, empty 204 responses, list bounds/order/total, filters, owner scoping, JSON/XML/text, and Atom collection validity. Include empty data and non-ASCII course/personnel names.
- **Authentication tests:** Include real Keycloak login/PKCE and administrator MFA assurance, with isolated signing-key unit fixtures for malformed/signature/issuer/audience/expiry cases. Test refresh/logout behavior and the documented remaining access-token lifetime. Never use a production-role bypass or a fake role selector for end-to-end acceptance.
- **Collection coverage:** Compare the Postman method/path inventory with OpenAPI. Every application operation needs a request and meaningful response assertions; include documented provider authentication and environment variables without embedded tokens/secrets. Use interactive PKCE for obtaining a demo token, then a collection runner for API checks. Do not enable password grant solely for automation.
- **Browser journeys:** Guest views/filters published schedule; student downloads schedule; invigilator submits/edits a request; administrator decides it, schedules/assigns/publishes, edits a session with audit evidence, exports, and backs up. Check unauthorized attempts directly against the API as well as hidden/disabled UI controls.
- **Build/quality proof:** Backend tests and appropriate Go checks, frontend lint/tests/production build, collection run, and the promised SonarQube analysis. Keep artifacts/results tied to the assessment commit. Do not invent a coverage percentage as a substitute for testing invariants.
- **Restore proof:** Restore the downloaded application backup into a new isolated PostgreSQL database and compare representative counts/relationships. Document separate Keycloak configuration/identity backup responsibilities; the application download must not expose provider credentials.

The rewrite is complete only when it starts from documented commands, implements every planned operation, preserves its invariants under concurrent requests, enforces real authentication/authorization, meets all UI criteria, and has reproducible assessment evidence. A report or plan alone satisfies none of the runtime checks.

## 12. Explicit omissions and limits

- No live SQL Server import, dual writing, legacy deletion, or database reset: demo-data startup was selected. Revisit only if real-data migration becomes a separate requirement.
- No custom identity provider, extra social providers, public registration, bespoke user-management UI, optimal constraint solver, background jobs, microservices, or speculative extension points.
- Serialized business writes are deliberate for the faculty-scale system. Revisit locking granularity only if measured contention requires it; any change must retain concurrency tests and invariants.
- Automatic planning may fail to find a complete arrangement even when a different search order could succeed; failure is explicit and atomic. No global optimality claim is made.
- Existing runtime correctness, hosted repository status, and assessment-upload status remain unverified. They require implementation and external evidence, not further changes to this plan.
