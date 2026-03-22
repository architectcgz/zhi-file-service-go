# Performance Test Assets

这一目录承载 `upload-service` Phase 5 的压测与 Grafana 观测资产。

当前代码库还没有完整的 north-south 上传 HTTP 运行时接线，因此这里同时提供两类入口：

- 可立即执行的 Go benchmark
- 面向后续 HTTP 运行时的 k6 / Prometheus / Grafana 资产

## 1. 立即可跑的 benchmark

在仓库根目录执行：

```bash
go test -bench 'Benchmark(CreateUploadSessionInline|CompleteUploadSessionPresignedSingle)' -benchmem ./internal/services/upload/app/commands
```

建议重点记录：

- `ns/op`
- `B/op`
- `allocs/op`

这组 benchmark 直接压 `upload-service` 应用层热点：

- create session
- complete presigned single

## 2. k6 热路径脚本

脚本文件：

- `test/performance/upload-session-hotpath.js`

示例：

```bash
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=dev-token \
k6 run test/performance/upload-session-hotpath.js
```

可选变量：

- `UPLOAD_SESSION_ID`
  传入后启用 complete 场景；未传时只跑 create session 压测。
- `TENANT_FILE_HASH`
  默认使用脚本内置 SHA256。

## 3. Prometheus

抓取配置模板：

- `test/performance/prometheus.yml`

默认抓取：

- `http://127.0.0.1:8080/metrics`

## 4. Grafana

导入 dashboard：

- `test/performance/grafana-upload-hotpath-dashboard.json`

推荐绑定 Prometheus 数据源后，重点看：

- Go runtime / RSS / goroutines
- `http_request_duration_seconds`
- `upload_session_complete_total`
- `upload_complete_duration_seconds`

说明：

- `upload_*` 指标面板已经按规范预留
- 如果当前运行时尚未接入对应业务指标，面板会显示空值，这是预期行为
- dashboard 的价值是先把查询口径固定下来，避免后续压测时再反复改图
