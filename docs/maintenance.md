# Maintenance and acceptance traceability

The product is complete. This document supports bug fixes, refinements, and handover. It maps the [PRD](prd.md) to existing checks; it does **not** claim those checks were rerun merely by documenting them. Commit-specific passing evidence and its limitations remain in [verification.md](verification.md).

## Requirement-to-evidence map

Paths below identify implementation and executable checks; integration/browser checks require isolated services.

| Requirement | Owning implementation | Existing proof |
| --- | --- | --- |
| F01 public schedule | [schedule.go](../backend/internal/store/schedule.go), [main.jsx](../frontend/src/main.jsx) | [store integration](../backend/internal/store/integration_test.go), [API tests](../backend/internal/api/api_test.go), [design journey](../frontend/tests/design.spec.js). |
| F02 identity and MFA | [verifier.go](../backend/internal/auth/verifier.go), [API auth](../backend/internal/api/auth.go), [browser adapter](../frontend/src/auth.js) | [JWT trust tests](../backend/internal/auth/verifier_test.go), [login journeys](../frontend/tests/login.spec.js), [authentication checks](../frontend/tests/authentication.js). |
| F03 business CRUD | [resources API](../backend/internal/api/resources.go), [resource store](../backend/internal/store/resources.go), [forms](../frontend/src/management.jsx) | [resource API tests](../backend/internal/api/resources_test.go), [browser CRUD](../frontend/tests/business.js), [Postman collection](postman/exams.postman_collection.json). |
| F04 manual scheduling | [scheduler.go](../backend/internal/store/scheduler.go), [rules.go](../backend/internal/store/rules.go) | [rule tests](../backend/internal/store/rules_test.go), [scheduler integration](../backend/internal/store/scheduler_integration_test.go). |
| F05 automatic planning | [automatic.go](../backend/internal/store/automatic.go) | [automatic tests](../backend/internal/store/automatic_test.go), [workflow integration](../backend/internal/store/workflows_integration_test.go). |
| F06 staffing | [rules.go](../backend/internal/store/rules.go), [automatic.go](../backend/internal/store/automatic.go) | [staffing/ranking tests](../backend/internal/store/rules_test.go), [workflow integration](../backend/internal/store/workflows_integration_test.go). |
| F07 publication and clear | [write.go](../backend/internal/store/write.go), [automatic.go](../backend/internal/store/automatic.go), [workflow UI](../frontend/src/workflows.jsx) | [workflow integration](../backend/internal/store/workflows_integration_test.go), [browser workflows](../frontend/tests/workflows.js). |
| F08 own pending requests | [requests.go](../backend/internal/store/requests.go), [RLS migration](../backend/migrations/0001_initial.sql) | [request integration](../backend/internal/store/requests_integration_test.go), [ownership/pool isolation](../backend/internal/store/scheduler_integration_test.go), [browser CRUD](../frontend/tests/business.js). |
| F09 decisions | [scheduler.go](../backend/internal/store/scheduler.go) | [workflow integration](../backend/internal/store/workflows_integration_test.go), [decision conflict journey](../frontend/tests/workflows.js). |
| F10 dashboard/reports | [reporting.go](../backend/internal/store/reporting.go) | [reports and owner scoping](../backend/internal/store/workflows_integration_test.go), [browser reports](../frontend/tests/workflows.js). |
| F11 audit | [write.go](../backend/internal/store/write.go), [resources.go](../backend/internal/store/resources.go) | [audit failure rollback](../backend/internal/store/scheduler_integration_test.go), [audit browser checks](../frontend/tests/workflows.js). |
| F12 XLSX | [schedule export](../backend/internal/store/schedule.go), [xlsx](../backend/internal/xlsx/schedule.go) | [workbook unit tests](../backend/internal/xlsx/schedule_test.go), [export integration](../backend/internal/store/export_integration_test.go), [downloads journey](../frontend/tests/downloads.js). |
| F13 backup/recovery | [archive.go](../backend/internal/backup/archive.go), [download routes](../backend/internal/api/downloads.go) | [archive tests](../backend/internal/backup/archive_test.go), [downloads/restore journey](../frontend/tests/downloads.js), [CI restore verifier](../scripts/verify_download.py). |
| Q01 security/privacy | [auth](../backend/internal/auth/), [migrations](../backend/migrations/), [provisioning](../backend/internal/provision/) | JWT/ownership tests above, [provision tests](../backend/internal/provision/provision_test.go), and real identity journeys. |
| Q02 concurrency/integrity | [transaction owner](../backend/internal/store/write.go), [migration runner](../backend/internal/store/migrate.go) | [actual lock contention and rollback](../backend/internal/store/scheduler_integration_test.go), [lifecycle integration](../backend/internal/store/integration_test.go). |
| Q03 accessible responsive UI | [shared CSS](../frontend/src/style.css), [forms](../frontend/src/management.jsx), [workflows](../frontend/src/workflows.jsx) | [UI requirement matrix](ui-evidence.md), [design tests](../frontend/tests/design.spec.js), [provider theme checks](../frontend/tests/provider-theme.js). |
| Q04 formats/contract | [representation.go](../backend/internal/api/representation.go), [OpenAPI](openapi.yaml) | [representation tests](../backend/internal/api/representation_test.go), [operation inventory test](../scripts/test_contract.py), [collection runner](../frontend/tests/collection.js). |
| Q05 operations | [config](../backend/internal/config/config.go), [scripts](../scripts/), [server](../backend/cmd/server/main.go) | [config tests](../backend/internal/config/config_test.go), Python script tests, [HTTPS journey](../frontend/tests/production.spec.js). |
| Q06 maintainability | Versioned sources/contracts/migrations and [CI workflow](../.github/workflows/ci.yml). | Existing formatting/vet/lint/unit/build/quality gates described in [verification.md](verification.md). |

## Choosing the smallest sufficient regression proof

| Change | Focused checks |
| --- | --- |
| Documentation only | Resolve local links/images, parse diagram JSON and OpenAPI, and check statements against owning code. Run the inventory test if touching contract documentation. No live data mutation is needed. |
| Rule or automatic selection | Relevant store unit test plus isolated database test for rollback/locking if the transaction path changes. |
| API input/permission/representation | Nearest API/auth test, contract inventory, and affected real identity/collection scenario when authorization behavior changes. |
| UI form/filter/focus behavior | Frontend unit/lint checks plus the relevant browser journey for behavior that needs a browser. |
| Migration, seed, provisioning, or restore | Isolated PostgreSQL/Keycloak lifecycle tests; verify repetition, grants, preserved data, and failure rollback. |
| Download or deployment | Workbook/archive tests plus affected isolated download/restore or trusted HTTPS journey. |

Basic local commands are:

```sh
python3 -m unittest discover -s scripts -p 'test_*.py'
(cd backend && go vet ./... && go test -race ./...)
(cd frontend && npm ci && npm run lint && npm test && npm run build)
```

For database/browser acceptance, prepare **disposable** services according to the README, then run `RUN_DATABASE_TESTS=1 python3 scripts/run.py test` and `npm run test:browser` from `frontend/`. The suites change fixtures, account state, and timetables; never point them at demonstration or real working data. Existing CI additionally runs the collection and HTTPS demonstration.

## Refinement review checklist

- Trace the affected behavior through callers, API permissions, transaction ownership, SQL/view constraints, and UI recovery before changing it.
- Preserve public sanitization, active-identity/MFA/ownership checks, final-state validation, shared-lock order, publication invalidation, and atomic audit.
- Update the affected contract, examples, requirements/rules, and diagram alongside the implementation. Keep unrelated Postman workspace changes and private local files out of the patch.
- Add one regression check for the defect or refined behavior, run the nearest affected gates, and record what actually passed and against which revision/environment.
- Review irreversible deletion/schema/identity effects and backup/compatibility before deployment. A passing unit test is not a restore or concurrency proof.

## Recorded limits and open verification boundaries

Existing records describe a completed implementation with unit, database, browser, collection, quality, and isolated HTTPS evidence. Evidence artifacts have limited retention; inspect the linked run/commit rather than treating historical results as a current branch guarantee.

Full WCAG conformance, manual screen-reader use, Firefox/Safari, physical passkey/mobile-device behavior, production capacity/SLA, public hosting, full provider disaster recovery, and stakeholder/assessment acceptance are not established. Existing Sonar coverage measures unit tests only. The [verification record](verification.md) also records unresolved development-tool dependency advisories; production-only and full dependency audits are different claims.

These are evidence and operating boundaries, not an unfinished feature backlog. No new product roadmap or delivery estimate is inferred from them.
