#!/usr/bin/env bash
# 检查必填环境变量是否已设置
# Secret 类型只检查变量是否存在，不打印值
set -euo pipefail

ERRORS=0

check_required() {
  local var_name="$1"
  local is_secret="${2:-false}"

  if [ -z "${!var_name:-}" ]; then
    echo "[MISSING] ${var_name}  (REQUIRED)"
    ERRORS=$((ERRORS + 1))
  else
    if [ "$is_secret" = "true" ]; then
      echo "[OK]      ${var_name}  (SECRET, value hidden)"
    else
      echo "[OK]      ${var_name}=${!var_name}"
    fi
  fi
}

check_optional() {
  local var_name="$1"
  local default="${2:-}"

  if [ -z "${!var_name:-}" ]; then
    echo "[DEFAULT] ${var_name}  (will use default: ${default})"
  else
    echo "[OK]      ${var_name}=${!var_name}"
  fi
}

echo "=== zhi-file-service-go 环境变量检查 ==="
echo ""

echo "--- 应用级 ---"
check_required APP_ENV
check_required APP_SERVICE_NAME
check_optional APP_LOG_LEVEL "info"
check_optional APP_SHUTDOWN_TIMEOUT "15s"

echo ""
echo "--- 数据库 ---"
check_required DB_DSN true
check_optional DB_MAX_OPEN_CONNS "50"

echo ""
echo "--- 对象存储 ---"
check_required STORAGE_ENDPOINT
check_required STORAGE_ACCESS_KEY true
check_required STORAGE_SECRET_KEY true
check_required STORAGE_PUBLIC_BUCKET
check_required STORAGE_PRIVATE_BUCKET

echo ""
echo "--- 服务专属（按 APP_SERVICE_NAME 检查）---"
case "${APP_SERVICE_NAME:-}" in
  upload-service)
    check_optional UPLOAD_MAX_INLINE_SIZE "10485760"
    check_optional UPLOAD_SESSION_TTL "24h"
    check_required REDIS_ADDR
    ;;
  access-service)
    check_required ACCESS_TICKET_SIGNING_KEY true
    check_optional ACCESS_TICKET_TTL "5m"
    ;;
  admin-service)
    check_required ADMIN_AUTH_JWKS true
    check_optional ADMIN_DELETE_REQUIRES_REASON "true"
    ;;
  job-service)
    check_required REDIS_ADDR
    check_optional JOB_LOCK_TTL "30s"
    ;;
  *)
    echo "[WARN] APP_SERVICE_NAME 未设置或未知，跳过服务专属检查"
    ;;
esac

echo ""
if [ "$ERRORS" -gt 0 ]; then
  echo ">>> doctor 检查失败：${ERRORS} 个必填变量缺失，请检查 .env.example 并配置后重试"
  exit 1
else
  echo ">>> doctor 检查通过"
fi
