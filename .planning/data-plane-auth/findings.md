# data-plane-auth findings

## Findings

### 2026-03-23

- `docs/api/data-plane-auth-context-spec.md` 已把数据面 north-south API 统一到 Bearer Token，并推荐固定 audience 为 `zhi-file-data-plane`
- `internal/services/access/runtime/runtime.go` 与 `internal/services/upload/runtime/runtime.go` 当前都仍使用 `NewDevelopmentAuthResolver(...)`
- `admin-service` 已有一套可复用的 JWKS/JWT 校验实现，但其 audience 固定为 `zhi-file-admin`，不能直接原样复用到数据面
- `upload` 与 `access` 的 `domain.AuthContext` 字段结构一致，差异主要在所需 scope 与后续业务授权校验；因此适合抽共享“认证基础设施”，不适合抽共享“业务鉴权层”
- `test/performance/README.md` 当前仍写着“upload north-south HTTP runtime 未完整接线”，需要和实际 runtime 现状一起复核
