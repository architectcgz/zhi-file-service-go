#!/usr/bin/env bash
# 写入 seed 数据
# 用法：seed.sh [dev|test]
set -euo pipefail

ENV="${1:-dev}"
DB_DSN="${DB_DSN:-postgres://zhi:zhi@localhost:5432/zhi_file_service?sslmode=disable}"
FIXTURES_DIR="$(cd "$(dirname "$0")/../.." && pwd)/test/fixtures"

if [ "$ENV" = "prod" ]; then
  echo "[ERROR] 禁止在 prod 环境执行 seed 脚本"
  exit 1
fi

SEED_FILE="${FIXTURES_DIR}/seed-${ENV}.sql"

if [ ! -f "$SEED_FILE" ]; then
  echo "[WARN] seed 文件不存在，跳过：${SEED_FILE}"
  exit 0
fi

echo ">>> 写入 ${ENV} seed 数据..."
psql "$DB_DSN" -f "$SEED_FILE"
echo ">>> seed 完成"
