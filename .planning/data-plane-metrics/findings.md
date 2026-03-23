# data-plane-metrics findings

## Findings

### 2026-03-23

- 当前平台层只有 Prometheus registry 与 `/metrics` 暴露，没有通用 north-south HTTP 指标中间件
- `test/performance/grafana-upload-hotpath-dashboard.json` 与 `test/performance/grafana-access-hotpath-dashboard.json` 已在查询 `http_request_duration_seconds`、`upload_session_complete_total`、`download_redirect_total`、`access_ticket_verify_failed_total`
- `upload/access` 当前代码中尚未看到对应业务指标接线，因此这些面板现状仍可能为空
- `job-service` 已有独立 observability 记录器，可作为业务指标抽象方式参考，但不适合直接拿来复用到数据面 HTTP handler
- `http.Request.Pattern` 在当前 Go 版本下可在外层 middleware 于 `next.ServeHTTP` 返回后读取到真实匹配结果，适合用来归一化 north-south route label，避免把顶层 `"/"` 挂到所有业务请求上
- 当前模块已补齐平台 HTTP 指标，以及 upload/access 的 foundation 业务指标；`upload_dedup_hit_total`、`upload_dedup_miss_total`、`access_storage_presign_duration_seconds` 仍留给后续更细粒度 observability 模块
