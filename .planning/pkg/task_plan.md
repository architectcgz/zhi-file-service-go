# pkg task plan

## Goal

仅实现稳定、跨服务复用、与具体业务服务解耦的共享包，避免把 `pkg/` 做成无边界杂物堆。

## Inputs

- `CODEX.md`
- `docs/service-layout-spec.md`
- `docs/storage-abstraction-spec.md`
- `docs/error-code-registry.md`
- `docs/openapi-contract-spec.md`

## Phases

### Phase 1

- 明确允许进入 `pkg/` 的能力范围
- 明确禁止进入 `pkg/` 的业务对象

### Phase 2

- 落 `pkg/ids`
- 落 `pkg/clock`
- 落 `pkg/xerrors`

### Phase 3

- 落 `pkg/storage`
- 落必要的 contracts / client 基础包

### Phase 4

- 验证 upload/access/admin/job 的复用方式
- 清理任何业务服务专属抽象

## Deliverables

- 稳定共享类型、接口、错误模型
- 不依赖服务内部 domain 的共享包

## Exit Criteria

- 每个共享包至少被两个服务真实依赖
- `pkg/` 中不出现 upload/access/admin/job 领域对象
