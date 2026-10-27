#!/usr/bin/env bash
set -euo pipefail

PORT_A=18080
PORT_B=18081
PID_A=""
PID_B=""
RESET_REQUESTED=false

for arg in "$@"; do
  if [ "$arg" = "--reset" ]; then
    RESET_REQUESTED=true
  fi
done

cleanup() {
  if [ -n "${PID_A}" ]; then
    kill "${PID_A}" 2>/dev/null || true
  fi
  if [ -n "${PID_B}" ]; then
    kill "${PID_B}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

wait_for_port() {
  local port="$1"
  local max_wait=30
  local elapsed=0
  while ! curl -s -f "http://127.0.0.1:${port}/healthz" > /dev/null 2>&1; do
    sleep 0.5
    elapsed=$((elapsed + 1))
    if [ "${elapsed}" -ge "${max_wait}" ]; then
      echo "ERROR: Timed out waiting for port-forward on port ${port}." >&2
      return 1
    fi
  done
}

if [ "${RESET_REQUESTED}" = true ]; then
  echo "==> Setting up port-forward to service-b (port ${PORT_B})..."
  kubectl port-forward svc/service-b "${PORT_B}:8080" -n retry-demo > /dev/null 2>&1 &
  PID_B=$!
  wait_for_port "${PORT_B}"

  echo "==> Resetting service-b failure sequence..."
  RESET_RES=$(curl -s -X POST "http://127.0.0.1:${PORT_B}/reset")
  echo "Reset response: ${RESET_RES}"
fi

echo "==> Setting up port-forward to service-a (port ${PORT_A})..."
kubectl port-forward svc/service-a "${PORT_A}:8080" -n retry-demo > /dev/null 2>&1 &
PID_A=$!
wait_for_port "${PORT_A}"

echo "==> Triggering service-a (/trigger) calling service-b (/work)..."
START_MS=$(date +%s%3N)
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "http://127.0.0.1:${PORT_A}/trigger")
END_MS=$(date +%s%3N)
CLIENT_LATENCY=$((END_MS - START_MS))

HTTP_BODY=$(echo "${RESPONSE}" | head -n -1)
HTTP_STATUS=$(echo "${RESPONSE}" | tail -n 1)

echo "==> Result:"
echo "HTTP Status Code : ${HTTP_STATUS}"
echo "Client Latency   : ${CLIENT_LATENCY} ms"
echo "Response Body    :"
if command -v jq >/dev/null 2>&1; then
  echo "${HTTP_BODY}" | jq . || echo "${HTTP_BODY}"
else
  echo "${HTTP_BODY}"
fi

if [ "${HTTP_STATUS}" -ne 200 ]; then
  echo "ERROR: Manual test failed with status ${HTTP_STATUS}" >&2
  exit 1
fi

echo "==> Manual test succeeded."
