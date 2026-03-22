# Performance Test Assets

这一目录承载 `upload-service` 与 `access-service` Phase 5 的压测与 Grafana 观测资产。

当前代码库还没有完整的 north-south 上传 HTTP 运行时接线，因此这里同时提供两类入口：

- 可立即执行的 Go benchmark
- 面向后续 HTTP 运行时的 k6 / Prometheus / Grafana 资产

## 1. 立即可跑的 benchmark

在仓库根目录执行：

```bash
go test -bench 'Benchmark(CreateUploadSessionInline|CompleteUploadSessionPresignedSingle)' -benchmem ./internal/services/upload/app/commands
```

```bash
go test -bench 'Benchmark(GetFilePublic|ResolveDownloadPrivate|RedirectByAccessTicketPrivate)' -benchmem ./internal/services/access/app/queries
```

建议重点记录：

- `ns/op`
- `B/op`
- `allocs/op`

这组 benchmark 直接压 `upload-service` 应用层热点：

- create session
- complete presigned single

这组 benchmark 直接压 `access-service` 高频读路径热点：

- get file metadata + public URL 分支
- resolve download + private presign 分支
- redirect by access ticket + verify 分支

## 2. k6 热路径脚本

脚本文件：

- `test/performance/upload-session-hotpath.js`
- `test/performance/access-read-hotpath.js`

示例：

```bash
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=dev-token \
k6 run test/performance/upload-session-hotpath.js
```

```bash
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=dev-token \
FILE_ID=file-1 \
ACCESS_TICKET=at_xxx \
k6 run test/performance/access-read-hotpath.js
```

可选变量：

- `UPLOAD_SESSION_ID`
  传入后启用 complete 场景；未传时只跑 create session 压测。
- `TENANT_FILE_HASH`
  默认使用脚本内置 SHA256。
- `FILE_ID`
  access 脚本要访问的文件 ID。
- `ACCESS_TICKET`
  传入后启用 ticket redirect 场景；未传时只跑 get file + resolve download。
- `DISPOSITION`
  access 下载场景的 `disposition`，默认 `attachment`。

说明：

- access 脚本对 `/download` 与 `/access-tickets/{ticket}/redirect` 使用 `redirects: 0`，只测 `access-service` 自己返回 `302` 的 hop，不把对象存储最终下载链路算进去。

## 3. Prometheus

抓取配置模板：

- `test/performance/prometheus.yml`

默认抓取：

- `http://127.0.0.1:8080/metrics`
- `http://127.0.0.1:8081/metrics`

说明：

- 示例里默认把 `upload-service` 放在 `8080`，把 `access-service` 放在 `8081`。
- 如果你只单独启动一个服务，或改了 `HTTP_PORT`，按实际端口调整 `prometheus.yml` 即可。

## 4. Grafana

导入 dashboard：

- `test/performance/grafana-upload-hotpath-dashboard.json`
- `test/performance/grafana-access-hotpath-dashboard.json`

推荐绑定 Prometheus 数据源后，重点看：

- Go runtime / RSS / goroutines
- `http_request_duration_seconds`
- `upload_session_complete_total`
- `upload_complete_duration_seconds`
- `file_get_total`
- `download_redirect_total`
- `access_ticket_verify_failed_total`
- `access_storage_presign_duration_seconds`

说明：

- `upload_*` 与 `access_*` 指标面板已经按规范预留
- dashboard 里的 HTTP 与业务指标查询口径已经固定，但如果当前运行时还没把对应 metrics 接到 `/metrics`，面板会显示空值，这是预期行为
- dashboard 的价值是先把查询口径固定下来，避免后续压测时再反复改图
