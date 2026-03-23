# admin-service findings

## Confirmed Constraints

- destructive operation 必须带 `reason`
- 审计与业务变更必须同事务
- 删除文件只做逻辑删除，物理删除交给 `job-service`
- 管理面使用独立身份体系，不复用数据面 token

## Implementation Findings

- `domain`、`app/commands`、`app/queries`、`infra/postgres`、`transport/http` 与 runtime 已落地，并已有针对 tenant scope、cursor、destructive reason、delete idempotency 的测试
- `cmd/admin-service/main.go` 当前通过 `bootstrap.New(...) -> adminruntime.Build(...) -> app.RegisterRuntime(...)` 启动，`/ready` 由 runtime 表检查驱动
- runtime 通过 `NewJWKSAuthResolverWithIssuers(...)` 装配 JWKS 鉴权，并读取 `ADMIN_AUTH_JWKS`、`ADMIN_AUTH_ALLOWED_ISSUERS`
- `auth_jwks_test.go` 与 `runtime_auth_test.go` 已覆盖 inline/remote JWKS、密钥轮换、audience 校验、issuer allowlist 与无效配置失败路径
- OpenAPI 已是正式契约，当前实现直接以 `api/openapi/admin-service.yaml` 为 north-south 合同
