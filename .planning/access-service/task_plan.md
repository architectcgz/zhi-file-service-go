# access-service task plan

## Goal

打通 access 读路径，覆盖文件最小视图、access ticket、download redirect、公私有访问分流和匿名落点控制。

## Inputs

- `docs/access-service-implementation-spec.md`
- `docs/data-plane-auth-context-spec.md`
- `docs/storage-abstraction-spec.md`
- `api/openapi/access-service.yaml`

## Phases

### Phase 1

- 固化读取模型与鉴权边界
- 固化 public/private 访问语义

### Phase 2

- 实现 `GetFile`
- 落文件状态与租户范围校验

### Phase 3

- 实现 `CreateAccessTicket`
- 实现 `download -> 302`
- 实现 `redirectByAccessTicket`

### Phase 4

- 接入 public URL 与 private presign GET
- 可选短缓存，但不改变事实源

### Phase 5

- 补 ticket、redirect、匿名例外、公私有路径测试
- 补高频读路径压测与观测

## Deliverables

- `internal/services/access`
- 对应 transport / infra / tests

## Exit Criteria

- `api/openapi/access-service.yaml` 四条核心路径闭环
- ticket redirect 成为唯一 north-south 匿名入口
- 无跨服务同步 RPC、无文件字节流代理
