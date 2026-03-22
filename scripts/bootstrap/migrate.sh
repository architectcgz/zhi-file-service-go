#!/usr/bin/env bash
# 执行数据库 migration
# 用法：migrate.sh [up|down|status]
# 依赖：golang-migrate CLI（make deps 安装）
# migration 文件位于 migrations/<schema>/ 按全局版本号线性排序执行
set -euo pipefail

COMMAND="${1:-up}"
DB_DSN="${DB_DSN:-postgres://zhi:zhi@localhost:5432/zhi_file_service?sslmode=disable}"

# 汇总 migrations 目录下所有子目录的文件，按文件名全局版本号排序
MIGRATION_DIR="$(cd "$(dirname "$0")/../.." && pwd)/migrations"

case "$COMMAND" in
  up)
    echo ">>> 执行 migration up..."
    migrate -path "$MIGRATION_DIR" -database "$DB_DSN" up
    ;;
  down)
    echo ">>> 回滚最近一次 migration..."
    migrate -path "$MIGRATION_DIR" -database "$DB_DSN" down 1
    ;;
  status)
    echo ">>> 查看 migration 状态..."
    migrate -path "$MIGRATION_DIR" -database "$DB_DSN" version
    ;;
  *)
    echo "用法: migrate.sh [up|down|status]"
    exit 1
    ;;
esac
