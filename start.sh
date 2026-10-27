#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="retry-demo"

echo "==> Checking if Kind cluster '${CLUSTER_NAME}' exists..."
if ! kind get clusters | grep -qx "${CLUSTER_NAME}"; then
  echo "==> Creating Kind cluster '${CLUSTER_NAME}'..."
  kind create cluster --name "${CLUSTER_NAME}" --config "${SCRIPT_DIR}/kind-config.yaml"
else
  echo "==> Kind cluster '${CLUSTER_NAME}' already exists."
fi

echo "==> Loading docker images into Kind cluster..."
kind load docker-image service-a:local --name "${CLUSTER_NAME}"
kind load docker-image service-b:local --name "${CLUSTER_NAME}"

echo "==> Applying Kubernetes manifests..."
kubectl apply -f "${SCRIPT_DIR}/k8s/"

echo "==> Waiting for service-b rollout..."
kubectl rollout status deployment/service-b -n retry-demo --timeout=60s

echo "==> Waiting for service-a rollout..."
kubectl rollout status deployment/service-a -n retry-demo --timeout=60s

echo "==> Allowing cluster networking and proxy tables to settle..."
sleep 2

echo "==> Cluster and services are ready."
