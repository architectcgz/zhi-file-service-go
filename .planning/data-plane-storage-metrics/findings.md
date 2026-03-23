# data-plane-storage-metrics findings

## Findings

### 2026-03-23

- `test/performance/grafana-upload-hotpath-dashboard.json` 仍在查询 `upload_dedup_hit_total` 与 `upload_dedup_miss_total`
- `test/performance/grafana-access-hotpath-dashboard.json` 仍在查询 `access_storage_presign_duration_seconds`
- upload dedup 决策点位于 `internal/services/upload/app/commands/complete_upload_session.go` 的 `LookupByHash(...)` 之后
- access private object presign 路径位于 `resolve_download.go` 与 `redirect_by_ticket.go` 的 `PresignGetObject(...)` 调用处
- 这些指标都更靠近 app/storage 路径，继续塞在 transport handler 层会让语义失真，因此下一模块应下沉到 app 级可观测性抽象
- 当前实现采用 app 级 metrics interface + runtime 注入的方式，不把 app 层直接耦合到 Prometheus，也避免为了细粒度指标继续污染 transport handler
- `upload_dedup_hit_total` / `upload_dedup_miss_total` 当前仅在 canonical hash 存在且实际执行 dedup lookup 后记录，避免把未参与 dedup 的请求误记成 miss
- `access_storage_presign_duration_seconds` 当前仅覆盖 private presign 分支，public URL 分支不记录
