# k8s-go-retry-services

Deterministic retry and exponential backoff demo: two independent Go microservices on a local Kind Kubernetes cluster.

## Architecture

| Service | Endpoints | Behavior |
|---|---|---|
| `service-a` (caller) | `POST /trigger`, `GET /healthz` | Calls `service-b` `/work`. Retries on 5xx, timeouts and network errors using exponential backoff with full jitter. Does not retry 4xx. |
| `service-b` (callee) | `POST /work`, `POST /reset`, `GET /healthz` | In-memory counter: fails the first `FAIL_COUNT` requests (default 2) with 503, then returns 200. |

`k8s/` holds the Deployments and ClusterIP Services in the `retry-demo` namespace. Service-to-service calls use Kubernetes DNS: `http://service-b.retry-demo.svc.cluster.local:8080`.

## Prerequisites

- Go
- Docker
- Kind
- kubectl
- curl (optional: jq for pretty output)

## Quick start

```sh
./bootstrap.sh      # build, create cluster, deploy, verify
```

Or step by step:

```sh
./build-image.sh    # build Docker images
./start.sh          # create Kind cluster and deploy
./manual-test.sh    # trigger service-a and verify the retry flow
./stop.sh           # tear down (use --keep-cluster to retain the cluster)
```

Expected result: `service-a` retries twice against `service-b` (two 503s) and succeeds on the third attempt, returning HTTP 200.

`manual-test.sh` expects the failure counter to start at 0. For repeat runs on a live cluster:

```sh
./manual-test.sh --reset
```

## Unit tests

```sh
(cd service-a && go test -v ./...)
(cd service-b && go test -v ./...)
```

Tests use `httptest.Server` and a no-op `retry.Sleeper`, so no real sleeping occurs.

## Notes

- `service-a` and `service-b` are separate Go modules (separate `go.mod`).
- Logs are structured JSON via `log/slog`.
- Manifests use `imagePullPolicy: IfNotPresent` because images are loaded into Kind with `kind load docker-image`.
- After a rollout, allow ~1-2 seconds for `kube-proxy` to propagate new ClusterIP routing.
