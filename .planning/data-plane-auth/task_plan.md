# data-plane-auth task plan

## Goal

把 `upload-service` 与 `access-service` 从开发态 Bearer 假鉴权收口到正式的数据面 Bearer/JWKS 校验链路，并抽出一层共享的数据面认证基础设施，避免两套实现继续漂移。

## Inputs

- `docs/api/data-plane-auth-context-spec.md`
- `docs/ops/configuration-registry-spec.md`
- `docs/services/upload-service-implementation-spec.md`
- `docs/services/access-service-implementation-spec.md`
- `docs/ops/deployment-runtime-spec.md`
- `docs/dev/test-validation-spec.md`
- `internal/services/admin/transport/http/auth_jwks.go`

## Phases

### Phase 1 (`completed`)

- 固化共享认证基础设施边界，明确哪些逻辑进入 `internal/platform`，哪些仍留在 `upload/access` 领域内
- 确认数据面 audience、issuer allowlist、claim 映射和错误语义与文档一致
- 建立共享 worktree，并补主工作树 planning 记录

### Phase 2 (`completed`)

- 抽取共享数据面 JWKS/Bearer 认证实现
- 为 `upload-service` / `access-service` 补正式配置键与 runtime wiring
- 保留 `auth_dev.go` 仅作开发辅助，不再作为 runtime 主路径

### Phase 3 (`completed`)

- 补 upload/access 的鉴权与 runtime 回归测试
- 校验 `go test ./...`、关键 runtime 测试和契约不回退

### Phase 4 (`completed`)

- 同步文档：数据面鉴权、配置注册表、实现细节、性能说明
- 复核 `.planning`、收口剩余风险，必要时转后续任务

## Deliverables

- `internal/platform` 下共享数据面认证基础设施
- `upload-service` / `access-service` 正式 Bearer/JWKS runtime 接线
- 对应测试与文档更新

## Exit Criteria

- `upload-service` 与 `access-service` runtime 不再默认使用 dev auth resolver
- 数据面 Bearer/JWKS 鉴权满足 `iss` / `aud` / `scope` / `tenant_id` / `sub` 基本约束
- 配置键、运行时行为和文档描述一致
- `go test ./...` 通过，且无新增活跃规划外缺口
