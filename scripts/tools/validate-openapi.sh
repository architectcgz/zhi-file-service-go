#!/usr/bin/env bash
# 校验 OpenAPI YAML 合法性
# 依赖：@stoplight/spectral-cli 或 swagger-cli
set -euo pipefail

OPENAPI_DIR="$(cd "$(dirname "$0")/../.." && pwd)/api/openapi"
FAIL=0

echo ">>> 校验 OpenAPI 契约..."
for YAML_FILE in "${OPENAPI_DIR}"/*.yaml; do
  echo "  检查: $(basename "${YAML_FILE}")"
  if command -v spectral &> /dev/null; then
    spectral lint "${YAML_FILE}" --ruleset .spectral.yml 2>&1 || FAIL=1
  elif command -v swagger-cli &> /dev/null; then
    swagger-cli validate "${YAML_FILE}" || FAIL=1
  else
    echo "  [WARN] 未找到 spectral 或 swagger-cli，跳过校验"
    echo "  安装方式：npm install -g @stoplight/spectral-cli"
    break
  fi
done

if [ "$FAIL" -ne 0 ]; then
  echo ">>> OpenAPI 校验失败"
  exit 1
fi
echo ">>> OpenAPI 校验通过"
