#!/usr/bin/env bash
# 写入 seed 数据
# 用法：seed.sh [dev|test]
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
ENV="${1:-dev}"
DB_DSN="${DB_DSN:-postgres://zhi:zhi@localhost:5432/zhi_file_service?sslmode=disable}"
SEED_ROOT="${SEED_ROOT:-${ROOT_DIR}/bootstrap/seed}"
PSQL_BIN="${PSQL_BIN:-psql}"

if [ "${ENV}" = "prod" ]; then
  echo "[ERROR] 禁止在 prod 环境执行 seed 脚本" >&2
  exit 1
fi

if [ "${ENV}" != "dev" ] && [ "${ENV}" != "test" ]; then
  echo "[ERROR] 仅支持 dev/test seed，当前: ${ENV}" >&2
  exit 1
fi

if ! command -v "${PSQL_BIN}" >/dev/null 2>&1; then
  echo "[ERROR] 未找到 psql 命令: ${PSQL_BIN}" >&2
  exit 1
fi

SEED_DIR="${SEED_ROOT}/${ENV}"
if [ ! -d "${SEED_DIR}" ]; then
  echo "[ERROR] seed 目录不存在: ${SEED_DIR}" >&2
  exit 1
fi

shopt -s nullglob
sql_files=("${SEED_DIR}"/*.sql)
if [ ${#sql_files[@]} -eq 0 ]; then
  echo "[WARN] ${SEED_DIR} 下没有 .sql 文件，跳过 seed"
  exit 0
fi

mapfile -t sql_files < <(printf '%s\n' "${sql_files[@]}" | sort)

echo ">>> 写入 ${ENV} seed 数据..."
for sql_file in "${sql_files[@]}"; do
  echo "  -> $(basename "${sql_file}")"
  "${PSQL_BIN}" "${DB_DSN}" -v ON_ERROR_STOP=1 -f "${sql_file}"
done
echo ">>> seed 完成"
