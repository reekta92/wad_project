# API integration guide

The implemented interface is [OpenAPI 3.1](openapi.yaml); exact fields, schemas, resource filters, media types, and status codes live there. [Postman instructions](postman/README.md) and the [executable collection](postman/exams.postman_collection.json) provide reusable requests. This guide explains caller decisions without duplicating the contract.

Local base URL: `http://localhost:8080/api/v1`. Vite development clients use `/api/v1` through the configured proxy; compiled delivery uses the application's own origin. Use HTTPS for a production deployment.

## Operation and authorization map

| Group | Operations | Access |
| --- | --- | --- |
| Public (4) | `GET /health/live`, `/health/ready`, `/auth/config`, `/schedule` | No token; schedule is sanitized and publication-gated. |
| Profile (1) | `GET /me` | Verified member/administrator; active local account. |
| Business CRUD (40) | For each of `/departments`, `/courses`, `/classrooms`, `/sessions`, `/personnel`, `/unavailability-requests`, `/exams`, `/invigilator-assignments`: POST collection, GET collection, GET item, PUT item, DELETE item. | Administrator with MFA; linked invigilators can use the request resource only, within own pending-write scope. |
| Administrator workflows (10) | `GET /dashboard`, `/reports/workload`, `/reports/availability`, `/audit-logs`, `/audit-logs/{id}`; `POST /unavailability-requests/{id}/decision`, `/schedule/automatic-planning`, `/invigilator-assignments/automatic`; `PUT /schedule/publication`; `DELETE /schedule`. | Administrator with MFA. |
| Personal reports (2) | `GET /me/duties`, `/me/workload` | Linked invigilator permission. Administrator role alone does not confer this permission. |
| Downloads (2) | `GET /schedule/export`; `POST /backups` | Export: authenticated member/administrator. Backup: administrator with MFA. |

Total: **59 application operations**. Keycloak protocol endpoints are not part of this inventory.

## Authentication and data ownership

Obtain access tokens through Keycloak Authorization Code + PKCE, using the configured `exam-web` or `exam-postman` client. Send `Authorization: Bearer <access-token>` as a header; never put tokens in URLs. The API does not accept caller-selected roles or a password-grant shortcut. The active local identity and administrator strong-authentication checks still apply to validly signed tokens.

Invigilator request writes are scoped to the verified personnel identity; supplying another personnel ID cannot grant access. Request owner, status, `decided_by`, and `decided_at` are not general replacement fields. Use the decision workflow to approve/reject. There is no public CRUD API for users, room allocations, or audit mutation.

## Request and response conventions

Send UTF-8 `application/json` for structured write bodies and use **PUT as full replacement**, not a partial patch. Unknown/duplicate fields, invalid values, and unsupported filters are rejected. Successful creates return 201 with `Location`; deletes return an empty 204. Authenticated responses are `no-store`.

Ordinary resource/workflow responses support JSON, XML, and text; collection reads additionally support Atom. Health and browser sign-in configuration are JSON-only; downloads are XLSX or `application/octet-stream`. Errors always use JSON. See [representation rules](representations.md) for quality values, `q=0`, type preservation, and 406 behavior before writes. `Vary: Accept` identifies negotiated responses.

Paginated collections return `data`, `page`, `page_size`, and `total`. Defaults are page 1 and size 20; maximum size is 100. Filters are server-side, query keys cannot repeat, and empty/unknown filter values are rejected. Follow pages using the same filters. Stable ordering is not a snapshot across separate page requests if data changes between requests.

The public schedule accepts `q`, `date_from`, `date_to`, `department_id`, `course_id`, `term`, `page`, and `page_size`. Dates use `YYYY-MM-DD`, term is 1–8, and search is bounded to 100 UTF-8 bytes. Resource/report-specific filters are listed in OpenAPI. Exports accept schedule filters **without** `page` or `page_size` and include all matching rows.

## Examples

These are illustrative local requests, not results of a new live API run. Dates and IDs depend on the database.

```sh
curl --fail-with-body --get 'http://localhost:8080/api/v1/schedule' \
  --data-urlencode 'q=mathematics' \
  --data-urlencode 'date_from=2026-11-01' \
  --data-urlencode 'page_size=20' \
  -H 'Accept: application/json'
```

An empty or unpublished result is a valid page:

```json
{"data":[],"page":1,"page_size":20,"total":0}
```

Use the private Postman environment for authenticated examples. A publication request body is `{"published":true}`; an automatic planning body is `{"season":"autumn"}`; automatic staffing uses `{"scope":"all"}` or `{"scope":"one","allocation_id":123}`; a request decision uses `{"approved":true}`. These actions change shared data; execute destructive collection workflows only against disposable services.

## Error handling and retry

```json
{"error":{"code":"validation","message":"Check the highlighted fields.","request_id":"example-id","fields":{"term":"Choose a term from 1 to 8."}}}
```

`fields` is present for field-validation errors; the request ID correlates the response. Do not assume every code is a domain identifier: standard HTTP failures may use their HTTP reason phrase.

| Status | Caller action |
| --- | --- |
| 400 | Correct body/filter/field values; resending the same request will not fix it. |
| 401 | Sign in or refresh through the provider; the token is absent or invalid. |
| 403 | Check application role, administrator MFA, active link, and invigilator mapping. |
| 404 | The item is absent or concealed by ownership, or the API route is unknown. |
| 406 / 415 | Choose an available response format / send JSON request content. |
| 409 | Resolve duplicate/reference/domain conflicts; refresh affected data before another attempt. |
| 413 | For export, narrow filters; input bodies also have a 1 MiB limit. No export is silently truncated. |
| 429 | Backup already running; respect `Retry-After: 5`. |
| 500 / 503 | Preserve the request ID, inspect service/dependency health, and use bounded retries for reads. |

Writes are transactional, but the API has no idempotency-key facility. If a write response is lost, read current state/audit before retrying; even a successful repeated action may produce another audit entry. Never blindly retry schedule clearing or request decisions. Health readiness is a dependency check, not an authorization or full backup-client check.

## Contract maintenance

Treat OpenAPI as the HTTP contract and the checked-in JSON collection as executable evidence. The [inventory check](../scripts/test_contract.py) compares operations; it does not generate schemas or prove every response. For an interface refinement, update implementation, OpenAPI, affected collection assertions/examples, and callers together. Follow [Postman workspace guidance](postman/README.md) before synchronization; this documentation task does not publish or change workspace access.
