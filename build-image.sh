#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Building service-b image (service-b:local)..."
docker build -t service-b:local "${SCRIPT_DIR}/service-b"

echo "==> Building service-a image (service-a:local)..."
docker build -t service-a:local "${SCRIPT_DIR}/service-a"

echo "==> Docker images built successfully."
