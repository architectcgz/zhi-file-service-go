# admin-service findings

## Confirmed Constraints

- destructive operation 必须带 `reason`
- 审计与业务变更必须同事务
- 删除文件只做逻辑删除，物理删除交给 `job-service`
- 管理面使用独立身份体系，不复用数据面 token

## Implementation Findings

- `domain`、`app/commands`、`app/queries` 已落地，并已有针对 tenant scope、cursor、destructive reason、delete idempotency 的测试
- `infra/postgres`、`infra/storage`、`transport/http` 目前仍为空目录，没有真正的仓储、HTTP handler 和 runtime 装配
- `cmd/admin-service/main.go` 当前仅调用 `bootstrap.Run(...)`，没有注册 runtime，按平台层设计 `readiness` 不会通过
- OpenAPI 已是正式契约，后续实现必须直接以 `api/openapi/admin-service.yaml` 为 north-south 合同，而不是再发明一套接口
