# delivery-validation findings

## Confirmed Constraints

- OpenAPI 已经是正式契约，contract test 不能只停留在 YAML 语法校验
- e2e 必须覆盖跨服务闭环，而不是单服务 handler happy path
- `deployments/` 需要和运行时、配置注册表、探针、扩缩容策略一致
- performance 资产按服务拆分，但目录统一放在 `test/performance`

## Implementation Findings

- `Makefile` 已声明并接通 `test-contract`、`test-e2e`、`test-performance`；`scripts/test/contract.sh`、`scripts/test/e2e.sh`、`scripts/test/performance.sh` 都是实际可执行入口
- `test/contract/` 已包含 upload/access/admin 契约测试，`test/e2e/` 已包含跨服务 HTTP 闭环测试
- `deployments/helm/*` 与 `deployments/kustomize/{base,overlays}` 已是正式部署资产，而非占位目录
- `test/performance/README.md` 已统一说明 benchmark、k6、Prometheus、Grafana；性能资产已覆盖 upload-service 与 access-service
