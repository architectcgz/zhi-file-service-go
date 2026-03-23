# delivery-validation findings

## Confirmed Constraints

- OpenAPI 已经是正式契约，contract test 不能只停留在 YAML 语法校验
- e2e 必须覆盖跨服务闭环，而不是单服务 handler happy path
- `deployments/` 需要和运行时、配置注册表、探针、扩缩容策略一致
- performance 资产按服务拆分，但目录统一放在 `test/performance`

## Implementation Findings

- `Makefile` 已声明 `test-contract`、`test-e2e`，但当前 `scripts/test/contract.sh` 只做 OpenAPI YAML 校验
- `test/contract/` 与 `test/e2e/` 目前只有 `.gitkeep`
- `deployments/helm`、`deployments/kustomize` 目前也只有占位目录
- `test/performance/` 目前只有 upload-service 的 benchmark / k6 / Prometheus / Grafana 资产
