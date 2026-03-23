#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
GO_BIN="${GO_BIN:-go}"

cd "${ROOT_DIR}"
exec "${GO_BIN}" test -count=1 -timeout=300s ./test/e2e/...
