# zhi-file-service-go SLO 与可观测性规范文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的 SLO、指标、日志、trace、仪表盘和告警规范。

它解决的问题：

1. 四个服务上线后用什么指标判断是否健康
2. 哪些链路必须打点和 trace
3. Grafana / Prometheus / Loki / Tempo 里应该看什么
4. 告警怎么避免过多噪音，又能覆盖真正故障

配套文档：

- [architecture-upgrade-design.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/architecture-upgrade-design.md)
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md)
- [access-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/access-service-implementation-spec.md)
- [admin-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-service-implementation-spec.md)
- [job-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/job-service-implementation-spec.md)
- [code-style-guide.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/code-style-guide.md)

## 2. 核心结论

### 2.1 默认观测栈

推荐默认栈：

- Metrics: Prometheus
- Dashboard: Grafana
- Logs: Loki
- Trace: Tempo
- Instrumentation: OpenTelemetry

### 2.2 优先覆盖四条主线

优先级从高到低：

1. upload complete 一致性
2. access 下载授权延迟与错误
3. admin 删除文件与策略修改
4. job 修复与清理积压

### 2.3 先有少而准的告警

第一阶段告警目标不是“全都报警”，而是：

- 线上故障能及时发现
- 值班看得懂
- 告警能直接定位到服务和链路

## 3. SLO 定义

## 3.1 `access-service`

这是最接近用户读路径的服务，SLO 最高。

建议：

- 可用性 SLO：`99.95%`
- `GET /api/v1/files/{fileId}` p95 延迟：`< 80ms`
- `GET /api/v1/files/{fileId}/download` p95 延迟：`< 120ms`
- 5xx 错误率：`< 0.1%`

### 3.2 `upload-service`

上传链路允许更高延迟，但不能牺牲一致性。

建议：

- 可用性 SLO：`99.9%`
- `POST /api/v1/upload-sessions` p95 延迟：`< 150ms`
- `POST /api/v1/upload-sessions/{id}/complete` p95 延迟：`< 2s`
- complete 成功率：`>= 99.9%`

### 3.3 `admin-service`

控制面不是最高 QPS，但要稳定。

建议：

- 可用性 SLO：`99.9%`
- 管理查询接口 p95 延迟：`< 300ms`
- destructive API 成功率：`>= 99.9%`

### 3.4 `job-service`

`job-service` 更适合定义处理时延目标，而不是用户响应时间。

建议：

- outbox 积压延迟：`p95 < 2m`
- stuck `COMPLETING` 修复延迟：`p95 < 10m`
- 文件物理删除最终完成延迟：`p95 < 30m`

## 4. Metrics 规范

## 4.1 通用标签

所有业务指标统一标签集合，尽量克制：

- `service`
- `operation`
- `status`
- `error_code`

不要默认打高基数字段：

- `file_id`
- `upload_session_id`
- `tenant_id`
- `user_id`

这些字段应该放日志和 trace，不应当做 Prometheus label。

## 4.2 HTTP 指标

所有 north-south API 必须暴露：

- `http_requests_total`
- `http_request_duration_seconds`
- `http_response_size_bytes`

最小标签：

- `service`
- `method`
- `route`
- `status_code`

## 4.3 upload-service 关键指标

必须暴露：

- `upload_session_create_total`
- `upload_session_complete_total`
- `upload_session_complete_failed_total`
- `upload_session_abort_total`
- `upload_session_active_total`
- `upload_complete_duration_seconds`
- `upload_storage_io_duration_seconds`
- `upload_dedup_hit_total`
- `upload_dedup_miss_total`

### 4.4 access-service 关键指标

必须暴露：

- `file_get_total`
- `access_ticket_issue_total`
- `access_ticket_verify_failed_total`
- `download_redirect_total`
- `download_redirect_failed_total`
- `access_storage_presign_duration_seconds`

## 4.5 admin-service 关键指标

必须暴露：

- `admin_tenant_create_total`
- `admin_tenant_patch_total`
- `admin_policy_patch_total`
- `admin_file_delete_total`
- `admin_audit_query_total`

## 4.6 job-service 关键指标

必须暴露：

- `job_run_total`
- `job_run_failed_total`
- `job_duration_seconds`
- `job_items_processed_total`
- `job_retry_total`
- `job_lock_acquire_failed_total`
- `outbox_pending_total`
- `outbox_oldest_pending_age_seconds`

## 5. Logging 规范

## 5.1 统一结构化日志

日志必须是 JSON 或等价结构化格式。

最小公共字段：

- `ts`
- `level`
- `service`
- `request_id`
- `trace_id`
- `span_id`
- `message`

## 5.2 业务字段

按场景追加：

- `tenant_id`
- `user_id`
- `admin_id`
- `upload_session_id`
- `file_id`
- `job_name`
- `event_id`
- `error_code`

## 5.3 记录原则

默认规则：

- 成功路径少量关键事件打 `INFO`
- 重试、降级、补偿打 `WARN`
- 请求失败、任务失败打 `ERROR`

禁止：

- 打完整 token
- 打二进制文件内容
- 打大块 payload

## 6. Trace 规范

## 6.1 通用要求

所有 north-south 请求必须开启 trace。

span 命名统一：

- `<service>.<operation>`

例如：

- `upload.create_session`
- `upload.complete.acquire_lock`
- `access.resolve_download`
- `admin.delete_file`
- `job.cleanup_orphan_blobs`

## 6.2 upload-service 关键 span

必须有：

- `upload.create_session`
- `upload.presign_parts`
- `upload.complete.acquire_lock`
- `upload.complete.storage_finalize`
- `upload.complete.persist_metadata`

## 6.3 access-service 关键 span

必须有：

- `access.get_file`
- `access.issue_ticket`
- `access.resolve_download`
- `access.redirect_by_ticket`

## 6.4 admin-service 关键 span

必须有：

- `admin.create_tenant`
- `admin.patch_tenant`
- `admin.patch_policy`
- `admin.delete_file`

## 6.5 job-service 关键 span

必须有：

- `job.expire_upload_sessions`
- `job.repair_stuck_completing`
- `job.process_outbox_events`
- `job.finalize_file_delete`
- `job.cleanup_multipart`
- `job.cleanup_orphan_blobs`
- `job.reconcile_tenant_usage`

## 7. Grafana 仪表盘建议

至少维护 5 个 dashboard：

### 7.1 `Overview`

展示：

- 四个服务 QPS
- 5xx 错误率
- p95 延迟
- Pod 重启数
- CPU / Memory

### 7.2 `Upload Pipeline`

展示：

- create / complete 总量
- complete 失败率
- active session 数
- dedup hit rate
- 存储 I/O 耗时

### 7.3 `Access Pipeline`

展示：

- 文件详情读取 QPS
- 下载跳转 QPS
- presign 失败率
- 401 / 403 / 404 分布

### 7.4 `Admin Control Plane`

展示：

- tenant / policy 写操作总量
- 文件删除总量
- 审计查询量
- 后台查询 p95

### 7.5 `Async Jobs`

展示：

- outbox pending 数
- oldest pending age
- job duration
- job failed total
- lock acquire failed total

## 8. 告警规范

## 8.1 P1 告警

触发条件建议：

- `access-service` 5xx 比例连续 5 分钟 > `2%`
- `upload-service` complete 失败率连续 10 分钟 > `5%`
- `outbox_oldest_pending_age_seconds > 900`
- 任一服务全部实例不可用

## 8.2 P2 告警

触发条件建议：

- `access-service` p95 延迟连续 10 分钟 > `300ms`
- `upload-service` complete p95 连续 10 分钟 > `5s`
- `job_lock_acquire_failed_total` 异常升高
- `stuck completing` 数量持续增长

## 8.3 P3 告警

触发条件建议：

- 单服务重启频繁
- admin 查询延迟异常
- 某类 error code 持续升高

## 9. 采样策略

建议：

- metrics 全量
- error log 全量
- trace 默认采样 `5%`
- error request trace 全量保留

upload complete、admin delete、job repair 这三类关键链路建议提升采样比率。

## 10. 上线前最低检查项

服务上线前至少确认：

1. `/metrics` 可抓取
2. `/ready` 正常
3. `/live` 正常
4. 关键 span 已打出
5. 关键 error code 可在日志检索
6. Grafana 基础 dashboard 可用
6. 至少一条 P1 告警已演练

## 11. 最终结论

可观测性不是上线后补的附属品，而是文件服务在高并发上传、下载跳转、后台修复场景下的基本能力。

如果没有这套 SLO 与观测规范，后续压测、扩容、线上排障和稳定性治理都会重新返工。
