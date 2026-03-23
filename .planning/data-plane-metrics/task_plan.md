# data-plane-metrics task plan

## Goal

为 `upload-service` 与 `access-service` 补齐当前压测与 Grafana 面板已经依赖、但运行时尚未稳定产出的数据面 Prometheus 指标。

## Inputs

- `internal/platform/observability/*`
- `internal/platform/httpserver/*`
- `internal/services/upload/transport/http/*`
- `internal/services/access/transport/http/*`
- `docs/ops/slo-observability-spec.md`
- `test/performance/grafana-*.json`
- `test/performance/README.md`

## Phases

### Phase 1 (`completed`)

- 固化本模块边界，只做数据面 metrics foundation，不扩散到完整 observability 大重构
- 确认 dashboard 已使用且当前缺失的指标集合

### Phase 2 (`completed`)

- 在平台层补 north-south HTTP Prometheus 指标
- 在 upload/access transport 层补关键业务计数器与时延指标
- 完成 runtime wiring

### Phase 3 (`in_progress`)

- 补 handler / httpserver / metrics 回归测试
- 同步必要文档并复核 dashboard 依赖口径
- 合并前做 review 收敛与最终提交整理

## Deliverables

- 数据面 HTTP Prometheus 指标
- upload/access 关键业务指标
- 对应测试与文档同步

## Exit Criteria

- `/metrics` 可暴露 `http_requests_total`、`http_request_duration_seconds`、`http_response_size_bytes`
- `/metrics` 可暴露 Grafana 热路径已查询的 upload/access 关键业务指标
- 相关测试通过
