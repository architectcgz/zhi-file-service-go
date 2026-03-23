# Performance Test Assets

这一目录承载 `upload-service`、`access-service` 与 `admin-service` 的压测与 Grafana 观测资产。

这里同时提供两类入口：

- 可立即执行的 Go benchmark
- 面向已接线 north-south HTTP 运行时的 k6 / Prometheus / Grafana 资产

说明：

- `upload-service`、`access-service` 仍通过统一脚本入口接进 `make test-performance`
- `admin-service` 当前以独立 k6 脚本交付，暂未并入 `make test-performance`

## 1. 统一执行入口

在仓库根目录执行：

```bash
make test-performance
```

```bash
make test-e2e
```

默认 `make test-performance` 等价于：

```bash
PERF_MODE=bench PERF_TARGET=all scripts/test/performance.sh
```

可选环境变量：

- `PERF_MODE=bench|k6`
- `PERF_TARGET=upload|access|all`
- `PERF_K6_SUITE=hotpath|full-api`
- `PERF_K6_WARMUP=true|false`
- `PERF_K6_WARMUP_DURATION`
- `PERF_K6_WARMUP_VUS`
- `GO_BIN=/path/to/go`
- `K6_BIN=/path/to/k6`

说明：

- `PERF_K6_SUITE` 默认为 `hotpath`
- 当 `PERF_MODE=k6` 且 `PERF_K6_SUITE=full-api` 时，`scripts/test/performance.sh` 默认会先执行一次 warm-up，再执行正式统计 run
- warm-up 默认使用 `PERF_K6_WARMUP_DURATION=2s`、`PERF_K6_WARMUP_VUS=1`
- 如需关闭 warm-up，可显式传 `PERF_K6_WARMUP=false`

`k6` 模式下的 `BEARER_TOKEN` 必须满足当前数据面鉴权契约：

- `aud` 覆盖 `zhi-file-data-plane`
- `iss` 命中服务配置的 `*_AUTH_ALLOWED_ISSUERS`，或该白名单为空
- token 已配置进对应服务的 `*_AUTH_JWKS` 可验证范围
- upload 场景至少带 `file:write`，access 场景至少带 `file:read`

示例：

```bash
PERF_MODE=bench PERF_TARGET=upload make test-performance
```

```bash
PERF_MODE=k6 PERF_TARGET=access \
BASE_URL=http://127.0.0.1:8081 \
BEARER_TOKEN=<valid-data-plane-jwt> \
FILE_ID=file-1 \
make test-performance
```

```bash
PERF_MODE=k6 \
PERF_TARGET=upload \
PERF_K6_SUITE=full-api \
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=<valid-data-plane-jwt> \
make test-performance
```

## 2. 全量接口 k6 脚本

脚本文件：

- `test/performance/upload-all-apis.js`
- `test/performance/access-all-apis.js`
- `test/performance/admin-api-full-coverage.js`
- `test/performance/admin-governance-workload.js`

目标：

- `upload-all-apis.js` 覆盖 `upload-service.yaml` 中全部 7 个业务接口
- `access-all-apis.js` 覆盖 `access-service.yaml` 中全部 4 个业务接口
- `admin-api-full-coverage.js` 覆盖 `admin-service.yaml` 中全部 11 个业务接口，并走真实文件删除 happy path
- `admin-governance-workload.js` 提供更保守的治理压测入口，默认不删除共享文件

最小运行方式：

```bash
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=<valid-data-plane-jwt> \
STORAGE_ENDPOINT_REWRITE_TO=http://127.0.0.1:19000 \
k6 run test/performance/upload-all-apis.js
```

```bash
BASE_URL=http://127.0.0.1:8081 \
UPLOAD_BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=<valid-data-plane-jwt> \
k6 run test/performance/access-all-apis.js
```

```bash
BASE_URL=http://127.0.0.1:8082 \
UPLOAD_BASE_URL=http://127.0.0.1:8080 \
ADMIN_BEARER_TOKEN=<valid-admin-jwt> \
DATA_BEARER_TOKEN=<valid-data-plane-jwt> \
k6 run test/performance/admin-api-full-coverage.js
```

通过统一入口运行 full-api 并自动 warm-up：

```bash
PERF_MODE=k6 \
PERF_TARGET=upload \
PERF_K6_SUITE=full-api \
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=<valid-data-plane-jwt> \
scripts/test/performance.sh
```

```bash
PERF_MODE=k6 \
PERF_TARGET=access \
PERF_K6_SUITE=full-api \
BASE_URL=http://127.0.0.1:8081 \
UPLOAD_BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=<valid-data-plane-jwt> \
scripts/test/performance.sh
```

说明：

- `upload-all-apis.js` 的 `DIRECT` 流量必须用合法 multipart part 大小执行。默认第 1 个分片为 `5 MiB`，第 2 个分片为 `512 KiB`，以满足 S3 / MinIO 对“非最后一个分片至少 5 MiB”的约束。
- upload 全量脚本推荐放在与服务相同的 Docker 网络里执行；否则需要通过 `STORAGE_ENDPOINT_REWRITE_FROM/TO` 把服务返回的对象存储地址改写成压测执行器可达地址。脚本内建兼容 `http://host.docker.internal:19000` 和 `http://zhi-file-perf-minio:9000` 两类常见返回地址。
- full-api 入口的 warm-up 会用同一份脚本先跑一次短时低并发预热，因此不会把冷启动抖动混进正式阈值判断。

## 3. admin-service 独立治理 k6 脚本

脚本文件：

- `test/performance/admin-governance-workload.js`

目标：

- 覆盖 `api/openapi/admin-service.yaml` 中全部 11 个业务接口
- 在 `setup()` 中创建本次压测唯一 tenant，并预热 tenant patch / policy patch / audit 数据
- 在 `teardown()` 中将该 tenant 标记为 `DELETED`，尽量减少共享环境残留
- 文件相关接口默认走安全探测模式，不删除共享文件；如需验证真实删除路径，可显式传入隔离文件 ID

覆盖接口：

- `GET /api/admin/v1/tenants`
- `POST /api/admin/v1/tenants`
- `GET /api/admin/v1/tenants/{tenantId}`
- `PATCH /api/admin/v1/tenants/{tenantId}`
- `GET /api/admin/v1/tenants/{tenantId}/policy`
- `PATCH /api/admin/v1/tenants/{tenantId}/policy`
- `GET /api/admin/v1/tenants/{tenantId}/usage`
- `GET /api/admin/v1/files`
- `GET /api/admin/v1/files/{fileId}`
- `DELETE /api/admin/v1/files/{fileId}`
- `GET /api/admin/v1/audit-logs`

最小运行方式：

```bash
ADMIN_BASE_URL=http://127.0.0.1:8082 \
ADMIN_BEARER_TOKEN=<valid-admin-jwt> \
k6 run test/performance/admin-governance-workload.js
```

推荐的 token 约束：

- `aud` 覆盖 `zhi-file-admin`
- `iss` 命中 `ADMIN_AUTH_ALLOWED_ISSUERS`，或服务端未配置白名单
- 角色至少包含 `admin.super`
- `tenant_scopes` 建议为 `["*"]`，否则脚本无法为本次运行创建唯一 tenant

环境变量约定：

- `ADMIN_BASE_URL`
  admin-service 根地址，默认 `http://127.0.0.1:8082`
- `ADMIN_BEARER_TOKEN`
  必填，管理面正式 JWT
- `ADMIN_TENANT_PREFIX`
  自动创建 tenant 的前缀，默认 `k6admin`
- `ADMIN_TENANT_EMAIL_DOMAIN`
  自动创建 tenant 联系邮箱域名，默认 `perf.local`
- `ADMIN_LIST_LIMIT`
  列表查询 limit，默认 `20`
- `ADMIN_AUDIT_ACTION`
  audit 场景默认过滤动作，默认 `PATCH_TENANT_POLICY`
- `ADMIN_ACTOR_ID`
  可选，按管理员主体过滤 audit 日志
- `ADMIN_FILE_MODE=missing|existing`
  默认 `missing`
- `ADMIN_FILE_ID`
  当 `ADMIN_FILE_MODE=existing` 时必填，指向你自行准备的隔离文件
- `ADMIN_DELETE_REASON`
  删除文件场景的审计 reason
- `ADMIN_RETIRE_REASON`
  teardown 将 tenant 标记为 `DELETED` 时使用的 reason
- `ADMIN_SLEEP_SECONDS`
  场景间 sleep，默认 `1`
- `STORAGE_ENDPOINT_REWRITE_FROM`
  仅 `upload-all-apis.js` 使用；宿主机执行时可补充对象存储地址改写前缀，支持逗号分隔多个值；脚本内建兼容 `http://host.docker.internal:19000` 和 `http://zhi-file-perf-minio:9000`，显式置空可关闭 rewrite
- `STORAGE_ENDPOINT_REWRITE_TO`
  仅 `upload-all-apis.js` 使用；对象存储地址改写后的宿主机可达地址，默认 `http://127.0.0.1:19000`
- `UPLOAD_DIRECT_PART_ONE_BYTES`
  upload 全量脚本里 DIRECT multipart 第 1 个分片大小，默认 `5242880`
- `UPLOAD_DIRECT_PART_TWO_BYTES`
  upload 全量脚本里 DIRECT multipart 第 2 个分片大小，默认 `524288`

默认文件模式：

- `ADMIN_FILE_MODE=missing`
  脚本会对本次运行生成一个不存在的 `fileId`，`GET/DELETE /files/{fileId}` 预期返回 `404 FILE_NOT_FOUND`
  这个模式的目的不是验证物理删除成功，而是安全覆盖文件详情/删除接口本身，避免污染共享数据
- `ADMIN_FILE_MODE=existing`
  用你显式提供的 `ADMIN_FILE_ID` 执行真实 file get/delete
  推荐只对隔离环境或一次性 fixture 使用，因为 delete 属于 destructive operation

示例：真实 delete 验证

```bash
ADMIN_BASE_URL=http://127.0.0.1:8082 \
ADMIN_BEARER_TOKEN=<valid-admin-jwt> \
ADMIN_FILE_MODE=existing \
ADMIN_FILE_ID=01JPACWQ6A9Y0MST6FZFSGC4Y1 \
k6 run test/performance/admin-governance-workload.js
```

## 4. 立即可跑的 benchmark

脚本在 `bench` 模式下会执行两组 Go benchmark：

```bash
go test -run '^$' -bench 'Benchmark(CreateUploadSessionInline|CompleteUploadSessionPresignedSingle)' -benchmem ./internal/services/upload/app/commands
```

```bash
go test -run '^$' -bench 'Benchmark(GetFilePublic|ResolveDownloadPrivate|RedirectByAccessTicketPrivate)' -benchmem ./internal/services/access/app/queries
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

## 5. k6 热路径脚本

脚本文件：

- `test/performance/upload-session-hotpath.js`
- `test/performance/access-read-hotpath.js`

说明：`BEARER_TOKEN` 需要是可被服务端 JWKS 验签的正式 JWT，且 `aud` 必须包含 `zhi-file-data-plane`。

示例：

```bash
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=<valid-data-plane-jwt> \
k6 run test/performance/upload-session-hotpath.js
```

```bash
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=<valid-data-plane-jwt> \
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

也可以直接通过统一入口触发：

```bash
PERF_MODE=k6 PERF_TARGET=upload \
BASE_URL=http://127.0.0.1:8080 \
BEARER_TOKEN=<valid-data-plane-jwt> \
make test-performance
```

注意：

- `admin-governance-workload.js` 当前不接入 `make test-performance`，请直接使用 `k6 run ...`

## 6. Prometheus

抓取配置模板：

- `test/performance/prometheus.yml`

默认抓取：

- `http://127.0.0.1:8080/metrics`
- `http://127.0.0.1:8081/metrics`
- `http://127.0.0.1:8082/metrics`

说明：

- 示例里默认把 `upload-service` 放在 `8080`，把 `access-service` 放在 `8081`。
- 示例里默认把 `admin-service` 放在 `8082`。
- 如果你只单独启动一个服务，或改了 `HTTP_PORT`，按实际端口调整 `prometheus.yml` 即可。

建议使用 `test/performance/prometheus.yml` 作为 dashboard 复现时的最小抓取模板。

## 7. Grafana

导入 dashboard：

- `test/performance/grafana-upload-hotpath-dashboard.json`
- `test/performance/grafana-access-hotpath-dashboard.json`
- `test/performance/grafana-admin-api-dashboard.json`
- `test/performance/grafana-admin-governance-dashboard.json`

推荐绑定 Prometheus 数据源后，重点看：

- Go runtime / RSS / goroutines
- `http_request_duration_seconds`
- `upload_session_complete_total`
- `upload_complete_duration_seconds`
- `file_get_total`
- `download_redirect_total`
- `access_ticket_verify_failed_total`
- `access_storage_presign_duration_seconds`
- `http_requests_total{service="admin-service"}`
- `http_request_duration_seconds{service="admin-service"}`
- `http_response_size_bytes{service="admin-service"}`

复现步骤建议固定为：

1. 启动待测服务并暴露 `/metrics`
2. 用 `prometheus.yml` 启动 Prometheus
3. 导入对应 dashboard JSON
4. 执行 `make test-performance`，或直接运行对应 `k6 run test/performance/*.js`

说明：

- `http_requests_total`、`http_request_duration_seconds`、`http_response_size_bytes` 已在数据面服务运行时接通
- `upload_session_create_total`、`upload_session_complete_total`、`upload_session_complete_failed_total`、`upload_session_abort_total`、`upload_complete_duration_seconds` 已接通
- `upload_dedup_hit_total`、`upload_dedup_miss_total` 已接通
- `file_get_total`、`access_ticket_issue_total`、`download_redirect_total`、`download_redirect_failed_total`、`access_ticket_verify_failed_total` 已接通
- `access_storage_presign_duration_seconds` 已接通
- admin dashboard 当前以通用 HTTP 指标为主，按 route/method/status_code 聚合治理接口流量、延迟与错误；`admin_tenant_create_total` 等业务指标仍以规范文档为准，待服务端进一步接通后可补充业务面板
- dashboard 的价值是先把查询口径固定下来，避免后续压测时再反复改图
