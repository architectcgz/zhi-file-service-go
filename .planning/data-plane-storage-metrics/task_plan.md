# data-plane-storage-metrics task plan

## Goal

补齐当前 Grafana 热路径 dashboard 里仍会显示空值的 storage-path 细粒度指标，并保持最小改动范围。

## Inputs

- `internal/services/upload/app/commands/complete_upload_session.go`
- `internal/services/access/app/queries/resolve_download.go`
- `internal/services/access/app/queries/redirect_by_ticket.go`
- `internal/services/*/runtime/*`
- `docs/ops/slo-observability-spec.md`
- `test/performance/grafana-*.json`
- `test/performance/README.md`

## Scope

- `upload_dedup_hit_total`
- `upload_dedup_miss_total`
- `access_storage_presign_duration_seconds`

## Phases

### Phase 1 (`completed`)

- 确认 upload dedup 决策点与 access private presign 调用点
- 确认适合的 metrics 抽象层级，避免把 transport 层 recorder 硬塞到 app 层

### Phase 2 (`completed`)

- 为 upload complete 路径补 dedup hit/miss 指标
- 为 access private download / ticket redirect 路径补 presign duration 指标
- 完成 runtime wiring

### Phase 3 (`in_progress`)

- 补 app/runtime 级测试
- 对齐 README 与 observability 文档
- 做 review、验证、提交整理
- 合并前完成最终人工 review 收敛

## Deliverables

- storage-path foundation metrics
- 对应测试
- dashboard/README 口径对齐

## Exit Criteria

- `/metrics` 可暴露 `upload_dedup_hit_total`
- `/metrics` 可暴露 `upload_dedup_miss_total`
- `/metrics` 可暴露 `access_storage_presign_duration_seconds`
- 相关测试通过
