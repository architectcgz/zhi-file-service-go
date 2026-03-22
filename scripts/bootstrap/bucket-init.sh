#!/usr/bin/env bash
# 初始化对象存储 bucket
# 依赖：mc（MinIO Client）或 awscli
set -euo pipefail

STORAGE_ENDPOINT="${STORAGE_ENDPOINT:-http://localhost:9000}"
STORAGE_ACCESS_KEY="${STORAGE_ACCESS_KEY:-minioadmin}"
STORAGE_SECRET_KEY="${STORAGE_SECRET_KEY:-minioadmin}"
STORAGE_PUBLIC_BUCKET="${STORAGE_PUBLIC_BUCKET:-zhi-files-public}"
STORAGE_PRIVATE_BUCKET="${STORAGE_PRIVATE_BUCKET:-zhi-files-private}"

echo ">>> 初始化对象存储 bucket..."
echo "  endpoint: ${STORAGE_ENDPOINT}"
echo "  public bucket: ${STORAGE_PUBLIC_BUCKET}"
echo "  private bucket: ${STORAGE_PRIVATE_BUCKET}"

# 使用 mc 创建 bucket
mc alias set zhi-local "$STORAGE_ENDPOINT" "$STORAGE_ACCESS_KEY" "$STORAGE_SECRET_KEY"

for BUCKET in "$STORAGE_PUBLIC_BUCKET" "$STORAGE_PRIVATE_BUCKET"; do
  if mc ls "zhi-local/${BUCKET}" > /dev/null 2>&1; then
    echo "  [SKIP] bucket 已存在: ${BUCKET}"
  else
    mc mb "zhi-local/${BUCKET}"
    echo "  [OK]   bucket 已创建: ${BUCKET}"
  fi
done

# public bucket 设置匿名只读访问
mc anonymous set download "zhi-local/${STORAGE_PUBLIC_BUCKET}"
echo "  [OK]   public bucket 已设置匿名只读访问"

echo ">>> bucket 初始化完成"
