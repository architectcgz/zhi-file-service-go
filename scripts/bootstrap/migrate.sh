#!/usr/bin/env bash
# 执行数据库 migration（按 schema 目录维护，按全局版本扁平执行）
# 用法：migrate.sh [build|up|down|status|reset] [down_steps]
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
MIGRATIONS_ROOT="${MIGRATIONS_ROOT:-${ROOT_DIR}/migrations}"
MIGRATE_BUILD_DIR="${MIGRATE_BUILD_DIR:-${ROOT_DIR}/.build/migrations/all}"
MIGRATE_BUILD_SCRIPT="${MIGRATE_BUILD_SCRIPT:-${ROOT_DIR}/scripts/bootstrap/migrate-build.sh}"
MIGRATE_BIN="${MIGRATE_BIN:-migrate}"
DB_DSN="${DB_DSN:-postgres://zhi:zhi@localhost:5432/zhi_file_service?sslmode=disable}"

COMMAND="${1:-up}"
DOWN_STEPS="${2:-1}"
APP_ENV="${APP_ENV:-local}"

ensure_tool() {
  if ! command -v "${MIGRATE_BIN}" >/dev/null 2>&1; then
    echo "[ERROR] 未找到 migrate CLI: ${MIGRATE_BIN}" >&2
    exit 1
  fi
}

run_build() {
  MIGRATIONS_ROOT="${MIGRATIONS_ROOT}" MIGRATE_BUILD_DIR="${MIGRATE_BUILD_DIR}" "${MIGRATE_BUILD_SCRIPT}"
}

run_migrate() {
  ensure_tool
  "${MIGRATE_BIN}" -path "${MIGRATE_BUILD_DIR}" -database "${DB_DSN}" "$@"
}

case "${COMMAND}" in
  build)
    run_build
    ;;
  up)
    run_build
    echo ">>> 执行 migration up..."
    run_migrate up
    ;;
  down)
    run_build
    echo ">>> 回滚最近 ${DOWN_STEPS} 次 migration..."
    run_migrate down "${DOWN_STEPS}"
    ;;
  status)
    run_build
    echo ">>> 查看 migration 状态..."
    run_migrate version
    ;;
  reset)
    if [ "${APP_ENV}" != "local" ] && [ "${APP_ENV}" != "dev" ] && [ "${APP_ENV}" != "test" ]; then
      echo "[ERROR] migrate reset 仅允许 local/dev/test 环境执行，当前 APP_ENV=${APP_ENV}" >&2
      exit 1
    fi
    run_build
    echo ">>> 执行 migration reset（down all + up）..."
    run_migrate down -all
    run_migrate up
    ;;
  *)
    echo "用法: migrate.sh [build|up|down|status|reset] [down_steps]" >&2
    exit 1
    ;;
esac
