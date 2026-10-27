#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="retry-demo"
KEEP_CLUSTER=false

for arg in "$@"; do
  if [ "$arg" = "--keep-cluster" ]; then
    KEEP_CLUSTER=true
  fi
done

if [ "$KEEP_CLUSTER" = true ]; then
  echo "==> Deleting Kubernetes manifests from '${CLUSTER_NAME}'..."
  if kind get clusters | grep -qx "${CLUSTER_NAME}"; then
    kubectl delete -f "${SCRIPT_DIR}/k8s/" --ignore-not-found=true
  else
    echo "==> Cluster '${CLUSTER_NAME}' does not exist, nothing to delete."
  fi
else
  echo "==> Deleting Kind cluster '${CLUSTER_NAME}'..."
  if kind get clusters | grep -qx "${CLUSTER_NAME}"; then
    kind delete cluster --name "${CLUSTER_NAME}"
  else
    echo "==> Cluster '${CLUSTER_NAME}' does not exist."
  fi
fi

echo "==> Stop operation completed."
