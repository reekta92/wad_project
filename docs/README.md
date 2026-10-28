# Faculty Exams — software documentation

**Product status:** complete; maintenance consists of refinements and bug fixes.  
**Documentation baseline:** source revision `a2e8826`, inspected 28 October 2026.  
**Audience:** project assessors, developers, and operators.

This pack describes the delivered system. Requirements and acceptance criteria record existing behavior; they are not a proposed implementation backlog. Completion does not imply public deployment, formal stakeholder sign-off, or verification beyond the evidence linked below.

## Reading guide

| Document | Purpose |
| --- | --- |
| [Product requirements (PRD)](prd.md) | Problem, users, delivered scope, functional and quality requirements, acceptance criteria, and boundaries. |
| [Architecture and security](architecture.md) | System context, components, trust boundaries, authentication sequence, and implementation decisions. |
| [Data model and dictionary](data-model.md) | Entity relationships, all 14 application tables, constraints, views, ownership, and deletion behavior. |
| [Workflows and business rules](workflows.md) | Scheduling, room allocation, staffing, publication, requests, and transactional guarantees. |
| [API integration guide](api-guide.md) | Interface map, authorization, pagination, formats, examples, errors, and retry considerations. |
| [Operations and recovery](operations.md) | Local startup, configuration, build/deployment boundaries, health checks, backup, restore, and troubleshooting. |
| [Maintenance and acceptance traceability](maintenance.md) | Requirement-to-test map, focused verification, regression checklist, and known evidence limits. |
| [Diagram gallery and editing](diagrams/README.md) | Five Mermaid diagrams embedded in their owning documents. |

The [root README](../README.md) is the executable quick start. The existing [OpenAPI contract](openapi.yaml), [representation specification](representations.md), [Postman guide](postman/README.md), [verification record](verification.md), and [UI evidence](ui-evidence.md) remain part of the pack.

## Sources of truth

| Subject | Authoritative source |
| --- | --- |
| Runtime behavior | [Go API](../backend/internal/api/), [domain/store code](../backend/internal/store/), and [React application](../frontend/src/). |
| Persisted schema and privileges | [Ordered SQL migrations](../backend/migrations/); role/database bootstrap is in [postgres-init.sh](../infrastructure/postgres-init.sh). |
| HTTP interface | [openapi.yaml](openapi.yaml), checked against the implemented routes and executable collection. |
| Reproducible request examples | [Checked-in Postman collection](postman/exams.postman_collection.json); see the guide before running mutating requests. |
| Verification claims | Commit-specific runs in [verification.md](verification.md); test presence alone is not a passing result. |

The SQL migrations are the database schema deliverable; the OpenAPI document supplies request/response schemas. This pack explains them without maintaining a second executable schema. Diagrams summarize the implementation and must be updated when their owning behavior changes. Package and contract version fields describe their respective artifacts, not an independently declared product release.

## Keeping the pack current

For a bug fix or refinement, update only the affected requirements, rules, schema notes, interface descriptions, and diagrams. Add a focused regression check and record the revision/run when claiming new verification. Add a new SQL migration for a schema change; never edit a migration already applied. Do not copy credentials, identity-provider state, private backups, or local assessment files into these documents.
