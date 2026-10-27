#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=========================================================="
echo " Starting bootstrap: build -> start -> manual-test"
echo "=========================================================="

echo "Step 1/3: Building images..."
"${SCRIPT_DIR}/build-image.sh"

echo "Step 2/3: Starting cluster and deploying services..."
"${SCRIPT_DIR}/start.sh"

echo "Step 3/3: Running manual test..."
"${SCRIPT_DIR}/manual-test.sh"

echo "=========================================================="
echo " Bootstrap finished successfully."
echo "=========================================================="
