# zhi-file-service-go upload-service 实现细节文档

## 1. 目标

这份文档定义 `upload-service` 的实现细节，目标不是重复 API 文档，而是提前固定以下内容：

1. 服务内模块怎么拆
2. 上传核心链路怎么落到代码
3. 事务边界和对象存储调用如何分离
4. 哪些实现方式明确禁止，避免后期返工

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [data-plane-auth-context-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api/data-plane-auth-context-spec.md)
- [upload-integrity-hash-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-integrity-hash-spec.md)
- [outbox-event-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/outbox-event-spec.md)
- [upload-session-state-machine-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-session-state-machine-spec.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)
- [storage-abstraction-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/storage-abstraction-spec.md)

## 2. 服务职责

`upload-service` 只负责上传写路径。

明确职责：

- 创建上传会话
- 代理上传内容
- 生成单文件或 multipart presign
- 查询上传进度
- complete / abort
- 去重判定
- 物理对象与逻辑文件落库
- 更新引用计数与租户使用量

明确不负责：

- 文件下载跳转
- 访问票据签发
- 后台租户治理
- 大规模清理和修复任务

## 3. 服务内包结构

推荐结构：

```text
internal/services/upload/
  domain/
    session.go
    session_status.go
    session_mode.go
    dedup.go
    errors.go
  app/
    commands/
      create_session.go
      upload_inline_content.go
      presign_parts.go
      complete_session.go
      abort_session.go
    queries/
      get_session.go
      list_parts.go
    tx/
      manager.go
  ports/
    session_repository.go
    dedup_repository.go
    blob_repository.go
    file_repository.go
    tenant_policy_reader.go
    tenant_usage_repository.go
    outbox_publisher.go
    storage_ports.go
  transport/http/
    handler.go
    request.go
    response.go
    mapper.go
  infra/
    postgres/
    storage/
    outbox/
```

关键约束：

- `domain` 只放状态机、不变量、幂等规则
- `app` 负责编排流程和事务
- `ports` 只定义消费方需要的最小接口
- `infra/postgres` 与 `infra/storage` 不相互耦合业务规则

## 4. 核心领域对象

`upload-service` 内部以以下对象为核心：

- `Session`
- `SessionMode`
- `SessionStatus`
- `DedupDecision`
- `CompletionResult`

必须坚持的事实模型：

- `UploadSession` 是上传过程，不是最终文件
- `BlobObject` 和 `FileAsset` 不是同一个概念
- `CompleteUpload` 是显式临界区，不是普通 update

## 5. 端口定义原则

`upload-service` 需要的端口应收敛为以下几类：

- `SessionRepository`
- `BlobRepository`
- `FileRepository`
- `DedupRepository`
- `TenantPolicyReader`
- `TenantUsageRepository`
- `TxManager`
- `OutboxPublisher`
- `BucketResolver`
- `MultipartManager`
- `PresignManager`
- `ObjectReader`
- `InlineObjectWriter`

禁止：

- 直接在 use case 中 import S3 SDK
- 直接在 handler 里开事务
- 用一个大而全的 `UploadRepository` 包住所有 SQL

## 6. 核心用例实现

## 6.1 `CreateUploadSession`

输入：

- 文件名
- content type
- size
- content hash
- access level
- upload mode
- total parts

流程：

1. 校验租户状态与策略
2. 校验 `contentHash` 规则；`DIRECT` 与 `PRESIGNED_SINGLE` 默认要求提供，且当前仅支持 `SHA256`
3. 生成 `uploadSessionId`
4. 规划 bucket 与 object key
5. 根据模式决定是否立即创建 provider 侧上下文
6. 落 `upload.upload_sessions`
7. 返回上传会话及必要的 presign 信息

模式差异：

- `INLINE`: 只创建 session，不创建 presign
- `PRESIGNED_SINGLE`: 创建对象 key，并生成单对象 PUT URL
- `DIRECT`: 创建 multipart upload，持久化 `provider_upload_id`

## 6.2 `UploadInlineContent`

仅用于受控代理上传。

流程：

1. 读取 session 并校验状态必须可上传
2. 流式写入对象存储
3. 更新 session 为 `UPLOADING`
4. 记录对象大小、etag 或 checksum 观察值

约束：

- 不把整文件读入内存
- 不在 handler 里直接写对象存储
- 大文件不走这条默认路径

## 6.3 `PresignMultipartParts`

流程：

1. 读取 session
2. 校验模式必须是 `DIRECT`
3. 校验状态不能是终态
4. 按 part number 批量签发 presign
5. 返回 URL 与附加 header

约束：

- part presign 只读 session，不开启业务事务
- presign TTL 必须配置化
- 不在此阶段写入 `upload_session_parts`

## 6.4 `ListUploadedParts`

流程：

1. 读取 session
2. 调对象存储 authoritative list parts
3. 与本地 `upload_session_parts` 对齐或补记观察值
4. 返回升序 part 列表

约束：

- 权威来源是对象存储，不是本地缓存
- 本地表用于幂等和审计，不替代 provider 真相

## 6.5 `CompleteUploadSession`

这是整个服务最关键的用例。

必须固定为三阶段：

### 阶段 A：获取 complete 所有权

事务内执行：

1. `SELECT ... FOR UPDATE` 锁定 session
2. 校验状态允许 complete
3. 若已 `COMPLETED`，直接返回既有 `file_id`
4. 将状态推进到 `COMPLETING`
5. 写入 `completion_token` / `completion_started_at`
6. 提交事务

目的：

- 让并发 complete 只有一个获胜者
- 把外部 I/O 从数据库锁区间里剥离出去

### 阶段 B：对象事实固化

事务外执行：

1. 对 `DIRECT` 先调用 provider `ListUploadedParts` 读取 authoritative part 列表，并与本地观察值对齐或补记
2. 对 `DIRECT` 基于 authoritative part 列表调用 provider complete multipart
3. 调 `HeadObject` 获取最终对象 metadata
4. 若请求回传 `contentHash`，校验算法当前仅支持 `SHA256`，并与 create 阶段声明值一致
5. 计算或确认 checksum / size / content type
6. 执行 dedup 判定

目的：

- 避免把对象存储网络调用放进数据库事务
- 固定以 provider authoritative parts / object facts 作为 complete 依据

### 阶段 C：元数据提交

新事务内执行：

1. 再次锁定 session 并确认仍归当前 `completion_token`
2. upsert `file.blob_objects`
3. 创建 `file.file_assets`
4. 更新 blob 引用计数
5. 更新 `tenant.tenant_usage`
6. 标记 session 为 `COMPLETED`
7. 写入 outbox event
8. 提交事务

失败处理：

- 如果阶段 B 失败，session 进入 `FAILED` 或保留 `COMPLETING` 并由 `job-service` 修复
- 如果阶段 C 失败，但对象已完成，必须记录修复标记，交给后台补偿

禁止做法：

- 一个长事务同时包数据库和对象存储 complete
- complete 完成后不记录修复线索
- 在 session 已 `COMPLETED` 时重复创建新 file

## 6.6 `AbortUploadSession`

流程：

1. 锁定 session
2. 若已终态，返回当前结果
3. 标记 `ABORTED`
4. 对 `DIRECT` 在事务外调用 multipart abort
5. 记录 outbox / audit 事实

约束：

- abort 必须幂等
- 对象存储 abort 失败不能回滚业务终态，只能进入清理任务

## 7. Repository 与 SQL 约束

建议拆分为：

- `session_queries.sql`
- `session_commands.sql`
- `blob_commands.sql`
- `file_commands.sql`
- `tenant_usage_commands.sql`
- `dedup_queries.sql`

SQL 约束：

- 命令侧 SQL 与查询侧 SQL 分开
- 任何 `FOR UPDATE` 必须写清目的
- `complete` 链路的 SQL 不允许隐藏在通用 helper 里

## 8. 与其他服务的关系

`upload-service` 第一阶段不依赖其他业务服务的同步 RPC。

只允许依赖：

- PostgreSQL
- 对象存储
- 可选 Redis
- outbox

不允许：

- complete 时同步调用 `access-service`
- 创建 session 时同步调用 `admin-service`

租户策略读取直接走本库表或共享数据库 schema，不引入跨服务网络跳数。

## 9. 配置项

建议至少提供：

- `upload.max_inline_size`
- `upload.session_ttl`
- `upload.complete_timeout`
- `upload.presign_ttl`
- `upload.allowed_modes`
- `upload.auth.jwks`
- `upload.auth.allowed_issuers`
- `storage.public_bucket`
- `storage.private_bucket`

配置原则：

- 所有 TTL 和阈值外置
- 不把 bucket 名、固定大小阈值写死在代码里
- runtime 默认使用正式 JWKS resolver；`auth_dev.go` 仅用于开发辅助与测试注入

## 10. 可观测性

关键指标：

- `upload_session_create_total`
- `upload_session_complete_total`
- `upload_session_complete_failed_total`
- `upload_session_abort_total`
- `upload_complete_duration_seconds`
- `upload_storage_io_duration_seconds`

关键日志字段：

- `upload_session_id`
- `tenant_id`
- `file_id`
- `upload_mode`
- `session_status`
- `completion_token`

关键 trace span：

- `upload.create_session`
- `upload.presign_parts`
- `upload.complete.acquire_lock`
- `upload.complete.storage_finalize`
- `upload.complete.persist_metadata`

## 11. 测试要求

必须覆盖：

- Session 状态机单测
- complete 并发幂等测试
- multipart provider 行为集成测试
- dedup 命中 / 未命中路径
- 阶段 B 成功但阶段 C 失败的补偿场景

测试优先级：

1. `domain`
2. `app complete`
3. `infra postgres`
4. `infra storage`
5. HTTP 契约

## 12. Code Review 检查项

看到以下实现应直接拦截：

- handler 直接操作对象存储
- repository 决定 session 状态推进
- complete 用一个事务包住 DB + S3
- 用全局锁代替 session 行锁
- 通过访问服务回查文件是否存在
- 大文件默认走服务端代理上传
