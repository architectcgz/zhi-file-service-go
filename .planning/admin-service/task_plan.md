# admin-service task plan

## Goal

把 admin-service 从“规则与用例已存在”推进到“真正可启动、可对外提供管理面 API、可通过 readiness”，并完成生产鉴权链路收口。

## Inputs

- `docs/services/admin-service-implementation-spec.md`
- `docs/api/admin-auth-spec.md`
- `docs/ops/deployment-runtime-spec.md`
- `docs/dev/test-validation-spec.md`
- `docs/api/error-code-registry.md`
- `api/openapi/admin-service.yaml`

## Phases

### Phase 1 (`completed`)

- 固化 `AdminContext`
- 固化角色矩阵、tenant scope、destructive reason 规则

### Phase 2 (`completed`)

- 实现 tenant / policy / usage 相关接口

### Phase 3 (`completed`)

- 实现后台文件查询、删除与审计查询

### Phase 4 (`completed`)

- 实现 `infra/postgres` 仓储、分页/筛选 SQL、`TxManager`、审计与 outbox 落库
- 补 `infra/storage` 中后台文件治理所需的最小对象存储访问能力
- 让命令查询从当前 stub/port 走到真实基础设施

### Phase 5 (`completed`)

- 实现 `transport/http` 和管理面鉴权适配，完整对齐 `api/openapi/admin-service.yaml`
- 构建 runtime，修改 `cmd/admin-service` 通过 `bootstrap.New(...) + RegisterRuntime(...)` 启动
- 补 service-level 测试：handler、runtime ready、事务边界、审计一致性

### Phase 6 (`completed`)

- 增加 admin-service contract test 输入，供 `delivery-validation` 直接复用
- 跑目标范围 `go test` / `-race`，确认服务可运行且 readiness 为绿

### Phase 7 (`completed`)

- runtime 改为通过 `NewJWKSAuthResolverWithIssuers(...)` 装配生产鉴权，读取 `ADMIN_AUTH_JWKS` 与 `ADMIN_AUTH_ALLOWED_ISSUERS`
- 对齐 `docs/api/admin-auth-spec.md` 中的密钥轮换、issuer allowlist 与失败路径
- 补生产鉴权回归测试，覆盖 inline/URL JWKS、未知 key、audience 不匹配与 issuer 拒绝场景

## Deliverables

- `internal/services/admin`
- `cmd/admin-service`
- 对应 transport / infra / runtime / tests

## Exit Criteria

- `api/openapi/admin-service.yaml` 全量路径闭环
- `/ready` 在 runtime 注册后可通过，未注册时不再是假健康
- 权限矩阵、审计事务、删除语义无分叉
- 配置键与注册表完全一致，生产 JWKS 鉴权链路已闭环
