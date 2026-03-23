# access-service task plan

## Goal

补齐 access-service 剩余收尾工作，重点是高频读路径的性能验证、观测资产与交付标准，而不是重复实现已完成的主链路。

## Inputs

- `docs/access-service-implementation-spec.md`
- `docs/slo-observability-spec.md`
- `docs/test-validation-spec.md`
- `docs/data-plane-auth-context-spec.md`
- `docs/storage-abstraction-spec.md`
- `api/openapi/access-service.yaml`

## Phases

### Phase 1 (`completed`)

- 固化读取模型与鉴权边界
- 固化 public/private 访问语义

### Phase 2 (`completed`)

- 实现 `GetFile`
- 落文件状态与租户范围校验

### Phase 3 (`completed`)

- 实现 `CreateAccessTicket`
- 实现 `download -> 302`
- 实现 `redirectByAccessTicket`

### Phase 4 (`completed`)

- 接入 public URL 与 private presign GET
- 可选短缓存，但不改变事实源

### Phase 5 (`completed`)

- 为 `GetFile` / `ResolveDownload` / `redirectByAccessTicket` 补 benchmark 或可复用 hotpath 基准
- 增加 access-service 的 k6 场景、Prometheus 抓取样例和 Grafana dashboard
- 明确 access 热路径指标命名，和 `docs/slo-observability-spec.md` 对齐
- 验证性能资产能直接接入现有 `test/performance/` 目录，而不复制 upload 方案

## Deliverables

- `test/performance/` 下 access-service 对应资产
- 必要的 access-service 性能测试或 benchmark
- 与文档一致的指标与观测说明

## Exit Criteria

- access-service 高频读路径有可复用的 benchmark 或压测脚本
- Grafana / Prometheus 资产能够覆盖核心读链路
- contract / e2e 相关工作已移交到 `delivery-validation`，本模块可关闭
