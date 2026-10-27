# k8s-go-retry-services

Deterministic retry and exponential backoff demo with two independent Go microservices on a local Kind Kubernetes cluster.

## Architecture

- `service-a` (caller): Exposes `POST /trigger` and `GET /healthz`. Calls `service-b` `/work`. Retries on 5xx status codes, timeouts, and network connection errors using exponential backoff with full jitter. Does not retry on 4xx client errors.
- `service-b` (callee): Exposes `POST /work`, `POST /reset`, and `GET /healthz`. Uses an in-memory counter to deterministically fail the first `FAIL_COUNT` requests (default: 2) with HTTP 503 before succeeding with HTTP 200.
- `k8s/`: Deployments and ClusterIP Services in `retry-demo` namespace. Inter-service routing uses Kubernetes DNS (`http://service-b.retry-demo.svc.cluster.local:8080`).

## Development & Test Commands

- Build Docker images: `./build-image.sh`
- Deploy to Kind: `./start.sh`
- Run deterministic verification: `./manual-test.sh` (or `./manual-test.sh --reset` for repeat runs)
- Tear down cluster: `./stop.sh` (or `./stop.sh --keep-cluster` to retain the cluster)
- End-to-end bootstrap from scratch: `./bootstrap.sh`
- Unit tests:
  - `(cd service-a && go test -v ./...)`
  - `(cd service-b && go test -v ./...)`

## Conventions & Rules

- Independent modules: `service-a` and `service-b` maintain separate `go.mod` files; do not combine them into a single module.
- Strict typing: Closed variant sets must use typed `enum` constants (e.g. `Outcome`), never raw string literals.
- Fast unit testing: Use `httptest.Server` and pass custom no-op sleepers (`retry.Sleeper`) to avoid sleeping in test loops.
- Structured logging: Use standard library `log/slog` for structured JSON logs.
- Portable paths: Never hardcode user- or machine-specific filesystem paths.
- Scripting: Shell scripts must use `set -euo pipefail` and remain idempotent. Do not inline multiline scripts in YAML manifests.

## Cautions

- Network settling: In Kind, newly created ClusterIP services require a short settling time (~1-2 seconds) after pod rollout for `kube-proxy` iptables / EndpointSlice routing to fully propagate.
- Image pull policy: Manifests must specify `imagePullPolicy: IfNotPresent` because images are loaded locally into Kind via `kind load docker-image`.
- Test reproducibility: `manual-test.sh` expects the counter sequence to start at 0. Always pass `--reset` or redeploy when executing repeat test passes against an active cluster.
