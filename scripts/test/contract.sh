#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

echo ">>> 运行 OpenAPI 契约检查..."
"${ROOT_DIR}/scripts/tools/validate-openapi.sh"

echo ">>> 运行 HTTP 契约测试..."
cd "${ROOT_DIR}"
go test ./test/contract/...
