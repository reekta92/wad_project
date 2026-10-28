# Test the API with Postman

The v3 YAML [collection](../../postman/collections/exams) covers all 59 operations in
[the OpenAPI contract](../openapi.yaml). Reuse it rather than generating another
collection. Postman CLI 1.76.0 is configured through
[`../../.postman/resources.yaml`](../../.postman/resources.yaml) for the
[existing workspace](https://go.postman.co/workspace/9c10d52b-1741-4ec2-87e3-ccd3a436c22b/overview).
The workspace binding does not upload files automatically. Local Mode uses v3
YAML collections and environments; JSON files are no longer supported there.
Open the repository in Postman Local Mode to use the configured YAML resources.

The [v2.1 JSON collection](exams.postman_collection.json) remains the compatibility
input for the existing Newman browser tests. When changing requests, edit that
file and regenerate the v3 collection from the repository root:

```sh
postman collection migrate docs/postman/exams.postman_collection.json \
  --output postman/collections/exams
postman collection lint postman/collections/exams
```

Keep the [YAML environment](../../postman/environments/local.environment.yaml)
and [JSON compatibility environment](local.postman_environment.json) aligned;
leave exported access tokens blank.

## Local API and public checks

Start the API and its dependencies using the [local setup](../../README.md#run-locally).
Postman targets `http://localhost:8080`; the frontend is not required for public
checks. Open this repository in Postman Local Mode, then select
**Faculty Exams — local**. For cloud workspaces, the JSON compatibility files
can still be imported. A web client
needs the Postman Desktop Agent to reach localhost.

For a read-only command-line run, execute this from the repository root:

```sh
postman collection run postman/collections/exams \
  -e postman/environments/local.environment.yaml \
  -i "Process liveness" \
  -i "Dependency readiness" \
  -i "Published schedule" \
  -i "Filter and paginate schedule" \
  -i "Reject invalid page size" \
  -i "Reject unsupported representation" \
  -i "Unknown API does not return the SPA" \
  -i "Browser sign-in configuration" \
  -i "Reject submitted roles without verified identity" \
  -i "Response representations" \
  --timeout-request 10000 --no-report-events
```

These requests check health, public schedule filtering/pagination, response
formats, validation errors, authentication configuration, and rejection of a
forged identity. An unpublished schedule may legitimately be empty. Dependency
readiness requires both PostgreSQL and Keycloak. `--no-report-events` keeps run
results local; it does not require workspace sign-in.

Do not run the entire collection against a database you want to preserve.
Its administrator workflows clear and rebuild the schedule.

## Authenticated checks

Postman account sign-in authenticates the CLI to the workspace. It does not
authenticate requests to this application's API.

1. In the collection, open **Verified current profile**, then
   **Authorization** and **Get New Access Token**. The request already configures
   Authorization Code with PKCE S256, client `exam-postman`, scope `openid`,
   callback `https://oauth.pstmn.io/v1/callback`, and authorization/token endpoints
   under `{{issuer}}` (`http://localhost:8081/realms/exams`). Use browser sign-in;
   this is a public client with no client secret or password grant.
2. Sign in with the appropriate demo account and its authenticator code. Read
   credentials locally from the private `.local/demo-accounts.json`; do not copy
   passwords or TOTP secrets into collection variables. Choose **Use Token** and
   send **Verified current profile**.
3. For folder requests, put the resulting access token in a private, local
   `access_token` environment value. Leave the shared/exported value blank and
   disable token sharing/synchronization. Tokens expire after five minutes;
   obtain a fresh token when required. Do not commit environment exports or
   include tokens in command arguments or saved run reports.
4. With `invigilator.demo`, run **Own reports (invigilator)** for private duties
   and workload. With `admin.demo` and verified MFA, send **GET /dashboard**,
   **GET /reports/workload**, **GET /reports/availability**, and **GET /audit-logs**
   individually inside **Workflow operations (administrator)** for read-only
   checks. Run the audit list before its detail request; detail requires a
   saved `audit_id` and an existing audit entry.

## Tests that change data

Use a separately prepared, disposable demo database. Changing only `base_url`
does not isolate data; confirm that the target API uses the disposable database.

- **Business CRUD (administrator)** must run all three phases in order. It creates,
  reads/replaces/lists, then deletes related fixtures. A failed run may leave
  fixtures behind. Writes can invalidate timetable publication.
- **Workflow operations (administrator)** needs `decision_request_id` set to an
  existing pending request that does not conflict with duties. It decides that
  request, clears every exam/allocation/duty, plans courses, assigns staff, and
  publishes. Run invigilator checks before this folder changes duties.
- **Downloads (administrator)** includes a database backup containing private
  application data. Keep downloads private; it is not part of the read-only
  quick check above.

The existing browser suite obtains real PKCE/MFA tokens and runs the collection
with the correct roles and decision fixture. See
[verification instructions](../verification.md#reproduce-focused-checks) to run
that suite against isolated services. No additional test runner is needed.

## Workspace synchronization

Local requests work without publishing. To inspect the connected workspace and
preview a create/update-only upload:

```sh
postman login
postman workspace get 9c10d52b-1741-4ec2-87e3-ccd3a436c22b --elements --json
postman workspace diff --push-strategy default
```

After reviewing the diff and approving publication, use
`postman workspace push --push-strategy default`. Keep the existing workspace
binding; do not create a duplicate workspace or use `force-sync`, which can
delete unrelated workspace entities. Keep access tokens and credentials out of
all published files.

The v3 migration preserves all 68 saved requests, authentication, payloads,
scripts, and ordering within folders. The existing cloud resource IDs remain
bound to the YAML paths. Specification content comparison is not supported by
CLI 1.76.0.

For a local inventory check that needs no running API:

```sh
python3 -m unittest discover -s scripts -p 'test_contract.py'
postman workspace lint --meta
postman collection lint postman/collections/exams
postman environment lint postman/environments/local.environment.yaml
```

The inventory check verifies that every OpenAPI operation has a saved request;
it does not prove live API behavior.
