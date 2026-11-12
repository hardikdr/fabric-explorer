# Development

The application is a standalone Go module. It uses Kubernetes dynamic clients without importing the Fabric or SONiC Operator Go modules, so dependency versions do not conflict.

- internal/model: allowlisted Kubernetes DTOs and correlated inventory.
- internal/correlate: pure association, reference resolution and diagnostics.
- internal/source: existing four-GVR reader, synthetic fixture files and bounded polling.
- internal/httpapi: read-only API, security headers and static assets.
- internal/observation: versioned future observation model and parser/link utilities; no runtime equipment adapter.
- web/src: inventory, logical topology, inspector, themes and future observation views.
- testdata/kubernetes: synthetic schema-shaped fixtures using example names and documentation addresses.
- web/tests/fixtures: synthetic inventory contract for component tests.
- web/tests/e2e: fixture-only browser workflow.

Build assets with npm --prefix web run build, then build the Go binary. The default fixture mode needs no credentials. Frontend unit tests mock the graph shell for shared-selection tests; browser tests exercise actual React Flow.

Update the frontend contract fixture only from a running fixture-mode server. Never capture an operational environment into test data:

~~~sh
curl --fail --silent http://127.0.0.1:8099/api/v1/inventory -o web/tests/fixtures/inventory.json
~~~

The optional IronCore in a Box helper requires explicit kubeconfig/context and checks existing CRDs. It never installs anything or changes the selected global context. Future telemetry should implement the versioned observation model with independently timed data; existing Kubernetes readiness must retain its current semantics.
