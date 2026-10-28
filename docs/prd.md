# Product requirements — Faculty Exams

**Status:** retrospective specification of the complete product. See the [documentation baseline](README.md).

## Problem and outcome

A faculty needs one place to prepare exam timetables, allocate suitable rooms, assign invigilators, review staff unavailability, and publish a schedule students can trust. These tasks share constraints: changing a session, room, course, or duty can invalidate other work. The delivered web application replaces the legacy SQL Server application with a Go/PostgreSQL/React/Keycloak system whose scheduling changes are validated together.

The product provides a public timetable and authenticated workspaces for administration and personal duties. The replacement operates on its own databases; it does not read, migrate, or modify the local legacy database.

## Users and permissions

| User | Need and delivered access |
| --- | --- |
| Guest | Find published exams without signing in; search/filter and switch between list and calendar views. |
| Member/student | Guest access plus verified profile, account/passkey controls, and published XLSX export. No student-record table or enrollment login workflow is required. |
| Linked invigilator | Member access plus own duties/workload and own unavailability requests. This is a member with the `invigilator` permission and a personnel link. |
| Application administrator | Maintain reference data, exams and duties; decide requests; plan/staff/publish/clear; inspect reports/audit; export current data and download a private backup. Strong authentication is mandatory. |
| Identity/operator administrator | Manage verified identities and account/MFA recovery in Keycloak, and operate infrastructure. This is separate from the application administrator role. |

## Delivered functional requirements

All rows describe implemented capabilities. The [acceptance matrix](maintenance.md) points to their code and tests.

| ID | Requirement / user story | Acceptance criteria |
| --- | --- | --- |
| F01 | As a guest, find the published timetable. | Search course code/name, filter dates, and page results in stable order. List/calendar show the current results page. Unpublished exams and private staff/request data are absent. Schedule times use Europe/Istanbul. |
| F02 | As a user, sign in with verified identity. | Authorization Code + PKCE establishes identity. API checks token trust, role, active local mapping, and administrator MFA; submitted client roles cannot grant access. |
| F03 | As an administrator, maintain the business records. | Create, retrieve, fully replace, delete, and filter/page departments, courses, classrooms, sessions, personnel, requests, exams, and assignments. Invalid fields, duplicates, references, and scheduling conflicts produce actionable errors. |
| F04 | As an administrator, schedule a course manually. | A course has at most one exam. Session/cohort rules hold, sufficient active rooms are allocated automatically, and a conflict commits no change. |
| F05 | As an administrator, plan remaining courses automatically. | All/autumn/spring scope preserves existing exams, tries course IDs and chronological sessions deterministically, and rolls back the entire batch if a course cannot be placed. |
| F06 | As an administrator, staff allocated rooms. | Manual and automatic staffing enforce availability and duty limits. Automatic staffing handles all unstaffed allocations or one allocation, preferring department, then workload, then ID. A failed batch commits nothing. |
| F07 | As an administrator, control public visibility. | Publish only a nonempty, valid, fully staffed timetable; unpublish immediately. Schedule-affecting writes invalidate publication atomically. Confirmed clear removes exams/allocations/duties while retaining master data, requests, and audit history. |
| F08 | As an invigilator, request unavailability. | Create/read/update/withdraw only own requests; edits/withdrawal require pending status. Owner/status/decision metadata cannot be changed through a replacement form. Other owners' records are concealed. |
| F09 | As an administrator, decide pending requests. | Approve/reject once. Approval conflicting with an existing duty fails and leaves the request pending. Reviewed requests remain immutable. Valid decisions preserve publication. |
| F10 | As a user, understand scheduling work. | Administrator dashboard shows publication, counts, and bounded issue previews. Workload retains zero-duty people; availability checks an additional duty. Invigilator reports remain fixed to the verified personnel identity, including unpublished duties. |
| F11 | As an administrator, inspect change history. | Filter/page read-only audit records and inspect actor, UTC timestamp, and before/after values. Domain writes and their audit commit or roll back together. |
| F12 | As an authenticated user, export the schedule. | A two-sheet XLSX includes every matching exam across pages; members receive published data and administrators current data. Empty results are valid, text cannot become formulas, and excess size returns an error without truncation. |
| F13 | As an administrator, obtain an application backup. | MFA-protected, confirmed download produces a bounded custom PostgreSQL archive containing private application data. Concurrent backup attempts receive 429. Recovery is first reviewed in a separate database. |

## Quality requirements

| ID | Requirement | Delivered criterion and limit |
| --- | --- | --- |
| Q01 | Security and privacy | Server authorization, signed identity, administrator MFA, restricted database roles, request row ownership, no-store authenticated responses, private credentials/backups, and memory-only browser tokens. See [trust boundaries](architecture.md). |
| Q02 | Data integrity and concurrency | Scheduling writes acquire the shared transaction advisory lock before rule reads; final-state checks, publication changes, and audit commit together. SQL constraints supplement service rules. |
| Q03 | Usability and accessibility | Responsive navigation/forms/reports, clear errors, confirmations, keyboard focus/dialog handling, device/light/dark themes, reduced motion, and labeled overflow regions. [Targeted UI evidence](ui-evidence.md) does not establish full WCAG conformance. |
| Q04 | Interoperability | Versioned REST interface; strict JSON requests; negotiated JSON/XML/text and Atom collections; XLSX schedule output and PostgreSQL custom backup. |
| Q05 | Operability and recovery | Separate live/ready endpoints, request IDs, safe repeated setup/provisioning, checksum-checked transactional migrations, non-destructive startup, bounded downloads, and documented restore. |
| Q06 | Maintainability | Domain rules live in the store layer; contracts, migrations, tests, and deployment definitions are versioned. CI records backend/frontend/browser/quality/HTTPS evidence by commit. |

There is no measured production latency, throughput, uptime, or disaster-recovery SLA. Export/backup limits are implemented resource bounds, not capacity benchmarks.

## Product boundaries

The delivered scope is a faculty timetable and staffing system. It has no individual student registration/grades, seating chart, notifications, self-registration, email password reset, academic-year archive, or global optimization solver. Courses' unique exam constraint applies to the current database; clearing/reusing a schedule is supported, separate historical timetables are not modeled. These boundaries describe the current product and do not create a roadmap.

Keycloak stores credentials and authenticators; the application stores identity links and personnel. The development Compose setup and trusted HTTPS CI demonstration do not amount to a public production deployment. Stakeholder acceptance and assessment submission are not inferred from automated tests.

## Completion and maintenance

The core features above are complete. Remaining changes should correct defects or refine delivered behavior while preserving the [business invariants](workflows.md) and [acceptance checks](maintenance.md). New product capabilities require a separately agreed scope; they are not prerequisites for documenting or maintaining this baseline.
