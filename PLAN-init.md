# PLAN: go-retry-services

Two Go services (A calls B) with retry + exponential backoff, deployed to Kind, with build/start/test/stop/bootstrap scripts.

Go 1.27. Two separate modules (`service-a/`, `service-b/`), each independently buildable/versioned. Service A reached via `kubectl port-forward` in `manual-test.sh`. Service B's failure behavior is deterministic (counter-based, not random) so retry/backoff is demonstrably exercised on every run.

### Phase 1: Repo scaffold + Service B (callee)
- Goal: Working, independently testable Service B with a deterministic flaky `/work` endpoint.
- Tasks:
  - [x] `service-b/go.mod` (Go 1.27), own module
  - [x] `service-b/cmd/service-b/main.go`: HTTP server, `GET /healthz`, `POST /work`
  - [x] Deterministic failure: in-memory request counter; fails (503) on the first N requests per "sequence" (e.g. configurable `FAIL_COUNT`, default 2 — meaning request #1 and #2 fail, #3 succeeds), then resets via `POST /reset` so `manual-test.sh` runs are repeatable without redeploying
  - [x] Structured logging (slog): request number, outcome, latency
  - [x] Dockerfile for service-b (multi-stage, distroless/alpine final stage)
  - [x] Unit test: counter logic — fails exactly `FAIL_COUNT` times then succeeds; `/reset` returns counter to 0
- Done when: `go build ./...` (in `service-b/`) succeeds; `go test ./...` passes; `docker build` succeeds locally.

### Phase 2: Service A (caller) with retry + exponential backoff
- Goal: Service A calls Service B with a documented retry/backoff policy, independently unit-testable against a mock B, own module.
- Tasks:
  - [x] `service-a/go.mod` (Go 1.27), own module
  - [x] `service-a/cmd/service-a/main.go`: HTTP server, `GET /healthz`, `POST /trigger` that calls `SERVICE_B_URL` + `/work`
  - [x] `service-a/internal/retry`: exponential backoff with jitter, max attempts, max elapsed time — env-configurable (`RETRY_MAX_ATTEMPTS`, `RETRY_BASE_DELAY`, `RETRY_MAX_DELAY`)
  - [x] Retry only on retryable conditions (5xx, network error, timeout) — not on 4xx
  - [x] Context-based per-attempt timeout + overall deadline
  - [x] `/trigger` response includes attempt count and final outcome
  - [x] Unit tests (table-driven, `httptest.Server` mock of B): succeeds on Nth attempt, exhausts attempts, respects context cancellation, does not retry on 4xx
  - [x] Dockerfile for service-a (same pattern as B)
- Done when: `go build ./...` (in `service-a/`) succeeds; `go test ./...` passes including retry edge cases; `docker build` succeeds for both images.

### Phase 3: Kubernetes manifests (Kind-targeted)
- Goal: Both services deployable to a Kind cluster and reachable from each other via in-cluster DNS.
- Tasks:
  - [x] `k8s/namespace.yaml`
  - [x] `k8s/service-b-deployment.yaml` + `k8s/service-b-service.yaml` (ClusterIP)
  - [x] `k8s/service-a-deployment.yaml` + `k8s/service-a-service.yaml` (ClusterIP — A only reached via port-forward, never NodePort/Ingress)
  - [x] Service A's `SERVICE_B_URL` env set to `http://service-b.retry-demo.svc.cluster.local:8080`
  - [x] `FAIL_COUNT` env on B's deployment
  - [x] Readiness/liveness probes on `/healthz` for both
  - [x] Resource requests/limits (small — this is a demo)
  - [x] `kind-config.yaml` (single control-plane node)
- Done when: manifests validate with `kubectl apply --dry-run=client -f k8s/` with no errors.

### Phase 4: Scripts — build-image.sh, start.sh, stop.sh
- Goal: One-command local lifecycle management across both modules.
- Tasks:
  - [x] `./build-image.sh`: `docker build` in `service-a/` and `service-b/`, tag `:local`
  - [x] `./start.sh`: create Kind cluster if absent (`kind get clusters | grep -q retry-demo || kind create cluster --name retry-demo --config kind-config.yaml`), `kind load docker-image` both images into it, `kubectl apply -f k8s/`, wait for rollout (`kubectl rollout status`)
  - [x] `./stop.sh`: `kind delete cluster --name retry-demo` (support `--keep-cluster` to instead `kubectl delete -f k8s/`)
  - [x] All scripts: `set -euo pipefail`, idempotent, clear echo of each step
- Done when: `./build-image.sh && ./start.sh` brings up a healthy cluster (`kubectl get pods -n retry-demo` all Running/Ready); `./stop.sh` tears down cleanly.

### Phase 5: manual-test.sh + bootstrap.sh
- Goal: One command deterministically proves the retry/backoff behavior end-to-end; one command runs everything from zero.
- Tasks:
  - [x] `./manual-test.sh`: `kubectl port-forward` service-a in background (trap to kill on exit), `curl -X POST localhost:<port>/trigger`, print response (attempt count, outcome, latency) — with `FAIL_COUNT` fixed, output is reproducible every run
  - [x] Optional `--reset` flag: hits B's `/reset` via a second port-forward before triggering, for repeat runs without redeploying
  - [x] `./bootstrap.sh`: calls `build-image.sh` → `start.sh` → `manual-test.sh` in sequence, exits non-zero on first failure
- Done when: `./bootstrap.sh` run from a clean checkout (no cluster, no images) ends with a passing, reproducible manual test and exit code 0.
