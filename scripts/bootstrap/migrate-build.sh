#!/usr/bin/env bash
# 构建统一 migration 执行视图：复用 Go runner 扫描分域目录、校验并生成扁平目录
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
MIGRATIONS_ROOT="${MIGRATIONS_ROOT:-${ROOT_DIR}/migrations}"
MIGRATE_BUILD_DIR="${MIGRATE_BUILD_DIR:-${ROOT_DIR}/.build/migrations/all}"
GO_BIN="${GO_BIN:-go}"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
  echo "[ERROR] 未找到 go 命令: ${GO_BIN}" >&2
  exit 1
fi

cd "${ROOT_DIR}"
"${GO_BIN}" run ./cmd/migrate-runner \
  -source "${MIGRATIONS_ROOT}" \
  -output "${MIGRATE_BUILD_DIR}"
