# zhi-file-service-go 配置注册表规范文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的统一配置注册表规范。

它解决的问题：

1. 当前文档里出现的 `upload.max_inline_size`、`UPLOAD_MAX_INLINE_SIZE` 这类命名如何统一
2. 外部注入、内部结构体、默认值、Secret 属性如何形成单一事实源
3. `Makefile`、`.env.example`、Kubernetes、Go 配置加载如何避免各写各的
4. 新增配置时到底需要更新哪些地方

配套文档：

- [deployment-runtime-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/deployment-runtime-spec.md)
- [development-workflow-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/development-workflow-spec.md)
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md)
- [access-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/access-service-implementation-spec.md)
- [admin-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-service-implementation-spec.md)
- [job-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/job-service-implementation-spec.md)
- [storage-abstraction-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/storage-abstraction-spec.md)

## 2. 核心结论

### 2.1 外部契约统一是环境变量

对运行时、容器、`.env.local`、Kubernetes、CI 来说，正式配置契约统一为环境变量。

例如：

- `UPLOAD_MAX_INLINE_SIZE`
- `ACCESS_TICKET_TTL`
- `JOB_LOCK_TTL`

不要把点号键名直接当成运行时注入契约。

### 2.2 点号键名只作为逻辑命名，不作为部署接口

文档中出现的：

- `upload.max_inline_size`
- `access.ticket_ttl`
- `job.lock_ttl`

只用于表达逻辑归属和配置结构体层级。

映射规则固定为：

- 逻辑键：`upload.max_inline_size`
- 环境变量：`UPLOAD_MAX_INLINE_SIZE`
- Go 结构体字段：`Upload.MaxInlineSize`

### 2.3 必须维护一份配置注册表

从现在开始，新增配置不能只在代码里偷偷加。

每个配置项至少要有以下信息：

- 逻辑键
- 环境变量名
- 所属服务
- 类型
- 默认值
- 是否必填
- 是否 Secret
- 用途说明

### 2.4 `doctor`、`.env.example`、K8s 都必须从同一事实源派生

要求：

- `make doctor` 的检查清单来自注册表
- `.env.example` 的示例项来自注册表
- Helm / Kustomize 注入项来自注册表

这样才能避免本地能跑、CI 缺变量、线上名称不一致这类问题。

## 3. 命名与映射规则

## 3.1 逻辑键命名

逻辑键统一采用：

- 小写
- 点号分层

例如：

- `app.env`
- `db.dsn`
- `upload.session_ttl`
- `access.private_presign_ttl`

## 3.2 环境变量命名

环境变量统一采用：

- 全大写
- 下划线分隔

映射规则：

- 点号转下划线
- 保留语义前缀

例如：

| 逻辑键 | 环境变量 |
|------|------|
| `app.env` | `APP_ENV` |
| `http.port` | `HTTP_PORT` |
| `db.dsn` | `DB_DSN` |
| `upload.max_inline_size` | `UPLOAD_MAX_INLINE_SIZE` |
| `job.lock_ttl` | `JOB_LOCK_TTL` |

## 3.3 结构体命名

Go 配置结构体建议按逻辑键分组：

```go
type Config struct {
    App     AppConfig
    HTTP    HTTPConfig
    DB      DBConfig
    Upload  UploadConfig
    Access  AccessConfig
    Admin   AdminConfig
    Job     JobConfig
    Storage StorageConfig
}
```

不要把几十个配置平铺成巨型结构体。

## 4. 注册表字段定义

每个配置项建议至少记录以下列：

| 字段 | 说明 |
|------|------|
| `logicalKey` | 逻辑键 |
| `envVar` | 环境变量名 |
| `serviceScope` | `shared` 或具体服务 |
| `type` | string/int/bool/duration/url/json 等 |
| `required` | 是否必填 |
| `secret` | 是否属于敏感信息 |
| `default` | 默认值 |
| `example` | 示例值 |
| `description` | 用途说明 |

## 4.1 推荐落位

当前阶段这份文档就是配置注册表规范源。

后续实现阶段建议再补充机器可读源，例如：

- `config/registry.yaml`
- `config/registry.json`

但在补出机器可读源之前，仍以本规范为准。

## 5. 通用配置基线

## 5.1 应用级

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `app.env` | `APP_ENV` | string | 是 | 否 | `dev` | 环境名 |
| `app.service_name` | `APP_SERVICE_NAME` | string | 是 | 否 | 无 | 服务名 |
| `app.log_level` | `APP_LOG_LEVEL` | string | 否 | 否 | `info` | 日志级别 |
| `app.shutdown_timeout` | `APP_SHUTDOWN_TIMEOUT` | duration | 否 | 否 | `15s` | 优雅停机超时 |

## 5.2 HTTP

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `http.port` | `HTTP_PORT` | int | 否 | 否 | `8080` | 监听端口 |
| `http.read_timeout` | `HTTP_READ_TIMEOUT` | duration | 否 | 否 | `15s` | 读超时 |
| `http.write_timeout` | `HTTP_WRITE_TIMEOUT` | duration | 否 | 否 | `30s` | 写超时 |
| `http.idle_timeout` | `HTTP_IDLE_TIMEOUT` | duration | 否 | 否 | `60s` | keep-alive 超时 |

## 5.3 数据库

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `db.dsn` | `DB_DSN` | string | 是 | 是 | 无 | PostgreSQL DSN |
| `db.max_open_conns` | `DB_MAX_OPEN_CONNS` | int | 否 | 否 | `50` | 连接池上限 |
| `db.max_idle_conns` | `DB_MAX_IDLE_CONNS` | int | 否 | 否 | `10` | 空闲连接 |
| `db.conn_max_lifetime` | `DB_CONN_MAX_LIFETIME` | duration | 否 | 否 | `30m` | 连接最大生命周期 |

## 5.4 Redis

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `redis.addr` | `REDIS_ADDR` | string | 视服务而定 | 否 | 无 | Redis 地址 |
| `redis.password` | `REDIS_PASSWORD` | string | 否 | 是 | 无 | Redis 密码 |
| `redis.db` | `REDIS_DB` | int | 否 | 否 | `0` | DB index |

## 5.5 存储

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `storage.endpoint` | `STORAGE_ENDPOINT` | string | 是 | 否 | 无 | S3/MinIO endpoint |
| `storage.region` | `STORAGE_REGION` | string | 否 | 否 | 无 | region |
| `storage.access_key` | `STORAGE_ACCESS_KEY` | string | 是 | 是 | 无 | access key |
| `storage.secret_key` | `STORAGE_SECRET_KEY` | string | 是 | 是 | 无 | secret key |
| `storage.public_bucket` | `STORAGE_PUBLIC_BUCKET` | string | 是 | 否 | 无 | public bucket |
| `storage.private_bucket` | `STORAGE_PRIVATE_BUCKET` | string | 是 | 否 | 无 | private bucket |
| `storage.public_base_url` | `STORAGE_PUBLIC_BASE_URL` | string | 否 | 否 | 无 | public URL 基础前缀 |
| `storage.force_path_style` | `STORAGE_FORCE_PATH_STYLE` | bool | 否 | 否 | `true` | S3 path style 开关 |

## 5.6 可观测性

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `otel.endpoint` | `OTEL_ENDPOINT` | string | 否 | 否 | 无 | OTLP 上报地址 |
| `otel.service_version` | `OTEL_SERVICE_VERSION` | string | 否 | 否 | 无 | 服务版本 |
| `metrics.enabled` | `METRICS_ENABLED` | bool | 否 | 否 | `true` | metrics 开关 |

## 6. 服务私有配置

## 6.1 `upload-service`

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `upload.max_inline_size` | `UPLOAD_MAX_INLINE_SIZE` | int64 | 否 | 否 | `10485760` | 代理上传大小阈值 |
| `upload.session_ttl` | `UPLOAD_SESSION_TTL` | duration | 否 | 否 | `24h` | 会话存活时间 |
| `upload.complete_timeout` | `UPLOAD_COMPLETE_TIMEOUT` | duration | 否 | 否 | `30s` | complete 内部超时 |
| `upload.presign_ttl` | `UPLOAD_PRESIGN_TTL` | duration | 否 | 否 | `15m` | presign 过期时间 |
| `upload.allowed_modes` | `UPLOAD_ALLOWED_MODES` | csv | 否 | 否 | `INLINE,PRESIGNED_SINGLE,DIRECT` | 允许上传模式 |

## 6.2 `access-service`

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `access.ticket_signing_key` | `ACCESS_TICKET_SIGNING_KEY` | string | 是 | 是 | 无 | 票据签名密钥 |
| `access.ticket_ttl` | `ACCESS_TICKET_TTL` | duration | 否 | 否 | `5m` | 票据 TTL |
| `access.download_redirect_ttl` | `ACCESS_DOWNLOAD_REDIRECT_TTL` | duration | 否 | 否 | `2m` | 跳转地址 TTL |
| `access.public_url_enabled` | `ACCESS_PUBLIC_URL_ENABLED` | bool | 否 | 否 | `true` | public URL 开关 |
| `access.private_presign_ttl` | `ACCESS_PRIVATE_PRESIGN_TTL` | duration | 否 | 否 | `2m` | private presign TTL |

## 6.3 `admin-service`

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `admin.auth.jwks` | `ADMIN_AUTH_JWKS` | json/url | 是 | 是 | 无 | 管理面认证密钥或 JWKS 地址 |
| `admin.auth.allowed_issuers` | `ADMIN_AUTH_ALLOWED_ISSUERS` | csv | 否 | 否 | 空 | 管理面允许的 `iss` 白名单，空表示只校验 claim 存在 |
| `admin.delete_requires_reason` | `ADMIN_DELETE_REQUIRES_REASON` | bool | 否 | 否 | `true` | 删除是否强制 reason |
| `admin.list_default_limit` | `ADMIN_LIST_DEFAULT_LIMIT` | int | 否 | 否 | `50` | 列表默认大小 |
| `admin.list_max_limit` | `ADMIN_LIST_MAX_LIMIT` | int | 否 | 否 | `200` | 列表最大大小 |

说明：

- 管理面审计是强约束，不提供 `admin.audit_enabled` 这类关闭开关

## 6.4 `job-service`

| 逻辑键 | 环境变量 | 类型 | 必填 | Secret | 默认值 | 说明 |
|------|------|------|------|------|------|------|
| `job.scheduler_enabled` | `JOB_SCHEDULER_ENABLED` | bool | 否 | 否 | `true` | 是否启用调度器主循环 |
| `job.default_batch_size` | `JOB_DEFAULT_BATCH_SIZE` | int | 否 | 否 | `100` | 默认批处理大小 |
| `job.default_max_concurrency` | `JOB_DEFAULT_MAX_CONCURRENCY` | int | 否 | 否 | `4` | 默认并发数 |
| `job.lock_backend` | `JOB_LOCK_BACKEND` | string | 否 | 否 | `redis` | 分布式锁后端 |
| `job.lock_ttl` | `JOB_LOCK_TTL` | duration | 否 | 否 | `30s` | 锁 TTL |
| `job.lock_renew_interval` | `JOB_LOCK_RENEW_INTERVAL` | duration | 否 | 否 | `10s` | 续租间隔 |
| `job.expire_upload_sessions.interval` | `JOB_EXPIRE_UPLOAD_SESSIONS_INTERVAL` | duration | 否 | 否 | `5m` | 过期会话扫描周期 |
| `job.repair_stuck_completing.interval` | `JOB_REPAIR_STUCK_COMPLETING_INTERVAL` | duration | 否 | 否 | `2m` | stuck completing 修复周期 |
| `job.process_outbox_events.interval` | `JOB_PROCESS_OUTBOX_EVENTS_INTERVAL` | duration | 否 | 否 | `15s` | outbox 事件消费周期 |
| `job.finalize_file_delete.interval` | `JOB_FINALIZE_FILE_DELETE_INTERVAL` | duration | 否 | 否 | `1m` | 文件物理删除周期 |
| `job.cleanup_multipart.interval` | `JOB_CLEANUP_MULTIPART_INTERVAL` | duration | 否 | 否 | `10m` | provider multipart 残留清理周期 |
| `job.file_delete_retention` | `JOB_FILE_DELETE_RETENTION` | duration | 否 | 否 | `168h` | 文件逻辑删除后的最小保留窗口 |
| `job.cleanup_orphan_blobs.interval` | `JOB_CLEANUP_ORPHAN_BLOBS_INTERVAL` | duration | 否 | 否 | `10m` | 孤儿对象清理周期 |
| `job.reconcile_tenant_usage.interval` | `JOB_RECONCILE_TENANT_USAGE_INTERVAL` | duration | 否 | 否 | `30m` | usage 对账周期 |

## 7. Secret 规则

以下配置必须视为 Secret：

- `DB_DSN`
- `REDIS_PASSWORD`
- `STORAGE_ACCESS_KEY`
- `STORAGE_SECRET_KEY`
- `ACCESS_TICKET_SIGNING_KEY`
- `ADMIN_AUTH_JWKS` 或其等价敏感配置

要求：

- 不写入 Git
- 不写入 `.env.example`
- 不写入普通 ConfigMap
- `doctor` 只检查“是否存在”，不打印值

## 8. 默认值与必填规则

## 8.1 默认值原则

默认值只适用于：

- 本地开发
- 非敏感阈值
- 可安全回退的行为

不应给以下项提供误导性默认值：

- 数据库地址
- 存储凭证
- 签名密钥

## 8.2 必填项原则

以下情况默认必填：

- 无默认值会导致服务错误连接或安全风险
- 不同环境必须显式指定
- 属于认证、存储、主依赖连接配置

## 9. 注册表与工程命令联动

## 9.1 `make doctor`

`doctor` 至少应校验：

- 所有必填非 Secret 配置存在
- Secret 型配置存在但不打印值
- 服务级关键配置没有明显冲突

## 9.2 `.env.example`

`.env.example` 只从注册表导出：

- 非敏感项
- 示例值
- 注释说明

## 9.3 K8s 注入

Kubernetes 中：

- 非敏感配置进 ConfigMap
- Secret 配置进 Secret
- 环境变量名必须与本注册表完全一致

## 10. 新增配置流程

新增一个配置项时必须同时更新：

1. 本注册表
2. 对应服务实现文档
3. `.env.example`
4. `doctor` 检查逻辑
5. 部署模板或运行时注入配置

禁止：

- 只在代码里加字段
- 只在 Helm values 里加键
- 同一含义出现两套变量名

## 11. Code Review 拦截项

看到以下情况应直接拦截：

- 文档写 `upload.max_inline_size`，运行时却注入另一套名字
- `.env.example`、K8s、代码结构体名称三套不一致
- Secret 被放进普通 ConfigMap
- 必填配置没有进入 `doctor`
- 一个配置没有注册表记录就直接进代码

## 12. 最终建议

这份文档的核心只有五条：

1. 外部配置契约统一是环境变量
2. 点号键名只用于逻辑命名和结构体映射
3. 所有配置都必须进入注册表
4. `doctor`、`.env.example`、K8s 注入都应从同一事实源派生
5. 新增配置不能只改代码不改文档

如果这层不先统一，后面实现阶段 `.env`、Makefile、Kubernetes 和 Go 配置加载很快就会各写各的。
