# Verification and build demonstration

CI uses fresh runner workspaces, application/identity databases, and Docker volumes. It never connects to the legacy SQL Server database or the developer's active PostgreSQL database. Ordinary startup preserves data; the integration and browser suites require disposable demo data.

## Evidence

[CI run 39113349449](https://github.com/reekta92/wad_project/actions/runs/39113349449) at `42f4248` passed all five jobs:

| Job | What it proves |
| --- | --- |
| backend | Formatting, module consistency, vet, race/unit/database tests, safe migrations/seeds, actual database grants and scheduling concurrency/rollback, identity provisioning and safe repetition. |
| frontend | JSX/accessibility lint, native JavaScript tests, and the production frontend build. |
| browser | Eleven Chromium journeys with actual Keycloak PKCE/password/TOTP, virtual passkey enrollment/login, role/owner checks, refresh/logout, CRUD/workflows, formats, responsive UI, XLSX, and HTTP backup/restore. Newman actually executes all 59 OpenAPI operations with 186 assertions. The restored application's 14 tables/sequences match, and the source digest stays unchanged. |
| quality | A real private, disposable SonarQube server/scanner, actual Go/JavaScript unit-coverage imports, and a saved analysis report. Findings are reviewed separately from the first-analysis gate. |
| production-demo | A compiled Go executable serves the frontend/API with `APP_ENV=production`. Chromium and the API client trust a short-lived CI CA and verify both application and Keycloak HTTPS certificates. Real administrator/student MFA, CRUD, JSON/XML/text/Atom, publication, XLSX, and logout work. |

The workflow is [.github/workflows/ci.yml](../.github/workflows/ci.yml). Each evidence artifact belongs to its workflow commit; inspect the run's commit before comparing results.

## Reproduce focused checks

```sh
python3 -m unittest discover -s scripts -p 'test_*.py'
(cd backend && go vet ./... && go test -race ./...)
(cd frontend && npm ci && npm run lint && npm test && npm run build)
```

For real database/browser/collection checks, prepare **isolated** services using the README setup commands, then:

```sh
RUN_DATABASE_TESTS=1 python3 scripts/run.py test
cd frontend
npx --no-install playwright install chromium
npm run test:browser
```

Browser tests obtain actual tokens through PKCE and MFA; they do not use a password grant or authentication bypass. The collection runs last because its ordered administrator workflow clears, replans, staffs, and publishes the disposable timetable. Its decision request needs a pending fixture. Authentication credentials, tokens, full Newman summaries, screenshots, traces, and video of authenticated pages are never saved as artifacts.

The managed workspace cannot access Docker or download all verification packages. Local unit/race/vet/build checks are separate from the hosted live-infrastructure results.

## SonarQube

The CI-only [runner](../scripts/sonar_ci.py) uses SonarQube Community Build **26.10.0.133078** and Scanner CLI **8.1.0.6389**, pinned by Docker digest. It binds a private server to loopback, immediately replaces the default administrator password, creates a private project and one analysis token, waits for analysis, exports safe evidence, and removes the server in `finally`. Credentials never enter command arguments or artifacts.

A fresh server has no previous new-code baseline: its default gate can report success with no applicable conditions. The runner additionally rejects any overall bug, vulnerability, or security hotspot. Code-smell findings remain visible for review. Imported coverage measures unit tests only; live database/browser coverage is not included, and no coverage threshold is invented.

The reviewed follow-up analysis in [run 39113821965](https://github.com/reekta92/wad_project/actions/runs/39113821965) at `3a3f705` reports **zero bugs, zero vulnerabilities, zero security hotspots, 35.0% unit-test coverage, and 1.0% duplicated lines**. The coverage excludes live integration/browser execution.

Four `javascript:S6845` findings concern the labeled, keyboard-focusable calendar/report/audit scroll regions. `tabIndex="0"` lets keyboard users reach and scroll overflowing content; each region has an accessible name. Removing it or assigning a false widget role would impair accessibility. These findings remain visible and are documented as reviewed false positives. The corresponding ESLint accessibility rule allows named regions. Real keyboard/browser checks remain required.

## Verification dependencies

ESLint 9.39.5, jsx-a11y 6.10.2, and Newman 6.2.3 are pinned with the lockfile. Same-major overrides patch Handlebars, Lodash, flatted, qs, Underscore, jose, and node-forge without forcing incompatible Faker/CSV/UUID APIs into Newman. The production-only npm audit must pass. The full audit remains visible as an artifact, including development findings.

The full audit on 18 October 2026 reports **10 affected dependency nodes: six high, four moderate, zero critical**, originating from four advisories:

| Dependency | Advisory and remaining limit |
| --- | --- |
| `@faker-js/faker` | [GHSA-qxc2-j82w-r537](https://github.com/advisories/GHSA-qxc2-j82w-r537): untrusted `helpers.fake` templates can execute code. Postman's current collection package still pins Faker 5; the patched Faker 10 API is incompatible. These tests use checked-in scripts and fixed fixtures, not untrusted Faker templates. |
| `csv-parse` | [GHSA-8cw4-87c7-c6xx](https://github.com/advisories/GHSA-8cw4-87c7-c6xx): vulnerable column parsing. Newman pins version 4; patched version 7 changes its CommonJS interface. Tests supply no CSV iteration files. |
| `uuid` | [GHSA-w5hq-g745-h8pq](https://github.com/advisories/GHSA-w5hq-g745-h8pq): missing caller-buffer bounds checks in v3/v5/v6. A compatible patch is unavailable for the pinned legacy majors. These collection scripts provide no UUID output buffers. |
| `node-forge` | [GHSA-86w9-cpqp-85rv](https://github.com/advisories/GHSA-86w9-cpqp-85rv): RSA PKCS#1 v1.5 verification accepts extra nested algorithm fields. The latest 1.4.0 is still affected, with no published patch. Newman calls the loopback HTTP API; production HTTPS/JWT verification uses the browser and Go, not Forge. |

These are unresolved **test-tool** vulnerabilities, not a clean full audit. Newman stays a development dependency and is absent from the browser production bundle and Go executable. Run only the trusted collection against disposable services; do not use this runner for untrusted collections, templates, CSV inputs, or production credentials. Revisit these pins when Postman adopts patched compatible dependencies. The 10-node count includes packages affected transitively, not ten independent advisories.

## Production build and artifacts

`production-demo` builds Go and React from the checkout, then uses `compose.https-demo.yaml` with a separate Compose project and database port. Keycloak runs its production HTTPS command. Only bootstrap migration/seed/provision commands use development mode; the tested Go process uses production mode. Both Playwright certificate-validation flags remain enabled. No public deployment is claimed.

The HTTPS preparation script refuses ordinary workspace execution. Reproduce the demonstration through the `production-demo` Actions job rather than rewriting an existing local realm. To run the compiled executable locally, use the README build commands and `python3 scripts/run.py serve`; production requires your own certificates, HTTPS issuer, and appropriate database configuration.

| Artifact | Contents | Retention |
| --- | --- | --- |
| `collection-evidence` | Operation inventory and request/assertion counts; no tokens. | 30 days |
| `dependency-audit` | Full npm audit, including unresolved development dependencies. | 30 days |
| `sonar-analysis` | Commit, versions, gate, measures, issues, and hotspots; no credentials. | 30 days |
| `production-https-evidence` | Commit, local origins, TLS-validation setting, production mode, and verified checks. | 30 days |
| `public-layout-previews` | Fixed guest-fixture phone/desktop images only. | 7 days |

These checks do not establish full WCAG conformance, manual screen-reader support, Firefox/Safari behavior, physical passkey/device behavior, a public deployment, or assessment submission. Lab 0 and final handoff exports remain local under ignored `development/`, as requested.
