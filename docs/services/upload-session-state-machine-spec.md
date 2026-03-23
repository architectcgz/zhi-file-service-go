# zhi-file-service-go UploadSession 状态机设计文档

## 1. 目标

本设计文档定义 `zhi-file-service-go` 中 `UploadSession` 的 canonical 状态机。

它解决三个问题：

1. 所有上传协议收口到同一状态模型
2. `complete / abort / expire / retry` 的边界行为可验证
3. Go 版实现时，状态迁移、表结构和接口行为保持一致

这份文档是数据模型文档的直接补充，和 [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md) 配套使用。

## 2. 适用范围

`UploadSession` 统一覆盖以下上传模式：

- `INLINE`
- `DIRECT`
- `PRESIGNED_SINGLE`

统一覆盖以下新 API 入口：

- `POST /api/v1/upload-sessions`
- `PUT /api/v1/upload-sessions/{uploadSessionId}/content`
- `POST /api/v1/upload-sessions/{uploadSessionId}/parts/presign`
- `POST /api/v1/upload-sessions/{uploadSessionId}/complete`
- `POST /api/v1/upload-sessions/{uploadSessionId}/abort`

设计原则是：

- 外部 north-south 入口统一收敛到 `upload-sessions` 资源路径
- 内部状态机必须统一

## 3. UploadSession 的职责边界

`UploadSession` 表示“一次上传过程”，不是“文件本身”。

它负责：

- 记录上传发起者、租户、目标访问级别
- 记录本次上传使用的模式和对象存储上下文
- 追踪当前上传是否可继续、是否可完成、是否已终止
- 为幂等、续传、超时回收提供唯一事实来源

它不负责：

- 作为最终文件事实表
- 直接承载文件访问权限判断
- 代替 `file_assets` 或 `blob_objects`

## 4. 模式定义

### 4.1 `INLINE`

含义：

- 文件内容经过 `upload-service` 进程，由服务端同步写对象存储

特点：

- 没有 presigned URL
- 通常不会有 multipart part list
- 更适合低并发和后台场景

### 4.2 `DIRECT`

含义：

- 服务端创建 multipart upload，并签发 part 级 presigned URL
- 上游客户端将分片直接发送给对象存储，服务只观察 authoritative part 结果

特点：

- 有 `provider_upload_id`
- 有 `object_key`
- 有 `total_parts`
- 支持 progress / complete / abort / resume

### 4.3 `PRESIGNED_SINGLE`

含义：

- 服务端签发单对象 `PUT` URL，客户端直接上传完整文件对象

特点：

- 不需要 multipart 上下文
- 需要单对象 metadata 校验
- `complete` 时从对象存储读取对象元信息并落库

## 5. 状态定义

推荐状态枚举：

- `INITIATED`
- `UPLOADING`
- `COMPLETING`
- `COMPLETED`
- `ABORTED`
- `EXPIRED`
- `FAILED`

### 5.1 `INITIATED`

含义：

- 会话已创建
- 可以继续上传
- 但实际数据可能尚未开始写入对象存储

典型场景：

- `INLINE` 创建后尚未开始写对象
- `PRESIGNED_SINGLE` 已生成对象 key，但客户端尚未上传
- `DIRECT` 已创建 multipart upload，但尚未观察到任何已上传分片

### 5.2 `UPLOADING`

含义：

- 会话处于活跃上传中
- 对象存储已开始接收内容或已存在 multipart context

典型场景：

- `DIRECT` 创建 multipart upload 后
- `INLINE` 服务端已开始流式写入对象
- `PRESIGNED_SINGLE` 客户端已经开始或已完成对象写入，但业务还未 `complete`

### 5.3 `COMPLETING`

含义：

- 有一个请求已经获得 complete 所有权
- 其他并发 complete 请求必须等待或返回“稍后重试”

这个状态存在的核心价值：

- 避免并发 `complete` 导致双写元数据
- 把“完成上传”建模成一个显式的临界区

### 5.4 `COMPLETED`

含义：

- 已成功生成 `file_assets`
- 已绑定最终 `file_id`
- 是成功终态

性质：

- 幂等终态
- 所有后续 complete 请求都应返回同一个 `file_id`

### 5.5 `ABORTED`

含义：

- 上传被主动中止

性质：

- 终态
- 不允许再继续上传或 complete

### 5.6 `EXPIRED`

含义：

- 会话超过 TTL 且未完成

性质：

- 终态
- 由请求路径上的惰性过期判定与 `job-service` 后台清理共同收敛到该状态

### 5.7 `FAILED`

含义：

- 上传流程在内部发生不可恢复错误

典型场景：

- complete 过程中对象存储合并失败且无法确定最终状态
- 元数据写入失败并进入人工修复路径

性质：

- 终态
- 是否允许新建续传会话由应用层另行判断

## 6. 状态机总览

```text
                +------------+
                | INITIATED  |
                +------------+
                 |   |    |
         start   |   |    | abort
        upload   |   |    v
                 v   | +---------+
            +-----------+ ABORTED|
            | UPLOADING | +---------+
            +-----------+
                 |   |
          complete   | expire
      acquire lock   v
                 +-----------+
                 |COMPLETING |
                 +-----------+
                  |   |   |
      success     |   |   | fail
                  v   |   v
            +-----------+ +--------+
            | COMPLETED | | FAILED |
            +-----------+ +--------+
                       ^
                       |
                   instant upload

INITIATED / UPLOADING / COMPLETING
  --expire--> EXPIRED
```

## 7. 事件与命令

定义以下 canonical 命令：

- `CreateSession`
- `IssueSingleUploadUrl`
- `IssuePartUploadUrls`
- `UploadInlineBytes`
- `ObserveUploadedParts`
- `GetProgress`
- `CompleteSingleUpload`
- `CompleteMultipartUpload`
- `AbortSession`
- `ExpireSession`

定义以下内部事件：

- `MultipartUploadCreated`
- `PartObserved`
- `ObjectObserved`
- `CompletionLockAcquired`
- `BlobMaterialized`
- `FileAssetCreated`
- `SessionCompleted`
- `SessionExpired`
- `SessionAborted`
- `SessionFailed`

## 8. 状态转移表

### 8.1 创建会话

| 当前状态 | 命令 | 条件 | 下一状态 | 说明 |
|------|------|------|------|------|
| 不存在 | `CreateSession` | `INLINE` | `INITIATED` | 尚未写入对象 |
| 不存在 | `CreateSession` | `DIRECT` | `INITIATED` | 已创建 multipart upload 并持久化 `provider_upload_id` |
| 不存在 | `CreateSession` | `PRESIGNED_SINGLE` | `INITIATED` | 已生成 object key |
| 不存在 | `CreateSession` | 命中秒传 | `COMPLETED` | 直接复用现有 blob 并创建 file asset |

### 8.2 开始上传 / 继续上传

| 当前状态 | 命令 | 条件 | 下一状态 | 说明 |
|------|------|------|------|------|
| `INITIATED` | `UploadInlineBytes` | `INLINE` | `UPLOADING` | 服务端开始接收内容 |
| `INITIATED` | `ObserveUploadedParts` | `DIRECT` 且已观察到至少一个 authoritative part | `UPLOADING` | multipart 上下文已被客户端实际使用 |
| `INITIATED` | `IssueSingleUploadUrl` | `PRESIGNED_SINGLE` | `INITIATED` | 签发 URL 不改变状态 |
| `INITIATED` | `IssuePartUploadUrls` | `DIRECT` | `INITIATED` | 签发 URL 不改变状态 |
| `UPLOADING` | `ObserveUploadedParts` | `DIRECT` | `UPLOADING` | 刷新已上传分片观察值 |
| `UPLOADING` | `UploadInlineBytes` | `INLINE` | `UPLOADING` | 幂等留在原状态 |

### 8.3 完成上传

| 当前状态 | 命令 | 条件 | 下一状态 | 说明 |
|------|------|------|------|------|
| `INITIATED` | `CompleteSingleUpload` | 对象已存在且 metadata 合法 | `COMPLETING` | 获取 complete 所有权 |
| `UPLOADING` | `CompleteSingleUpload` | 对象已存在且 metadata 合法 | `COMPLETING` | 获取 complete 所有权 |
| `INITIATED` | `CompleteMultipartUpload` | parts 齐全 | `COMPLETING` | 获取 complete 所有权 |
| `UPLOADING` | `CompleteMultipartUpload` | parts 齐全 | `COMPLETING` | 获取 complete 所有权 |
| `COMPLETING` | `CompleteSingleUpload` | 其他请求并发进入 | `COMPLETING` | 等待结果 |
| `COMPLETING` | `CompleteMultipartUpload` | 其他请求并发进入 | `COMPLETING` | 等待结果 |
| `COMPLETING` | 内部完成成功 | blob/file 均已落库 | `COMPLETED` | 成功终态 |
| `COMPLETING` | 内部完成失败 | 不可恢复错误 | `FAILED` | 失败终态 |

### 8.4 中止与过期

| 当前状态 | 命令 | 条件 | 下一状态 | 说明 |
|------|------|------|------|------|
| `INITIATED` | `AbortSession` | 手动中止 | `ABORTED` | 终态 |
| `UPLOADING` | `AbortSession` | 手动中止 | `ABORTED` | 终态 |
| `INITIATED` | `ExpireSession` | TTL 超时 | `EXPIRED` | 终态 |
| `UPLOADING` | `ExpireSession` | TTL 超时 | `EXPIRED` | 终态 |
| `COMPLETING` | `ExpireSession` | 仍未完成且 TTL 超时 | `EXPIRED` | 极端情况，由后台兜底 |

### 8.5 终态行为

| 当前状态 | 命令 | 结果 |
|------|------|------|
| `COMPLETED` | `Complete*` | 返回已有 `file_id` |
| `COMPLETED` | `AbortSession` | 返回错误，不允许 |
| `ABORTED` | `Upload*` / `Complete*` | 返回错误 |
| `EXPIRED` | `Upload*` / `Complete*` | 返回错误 |
| `FAILED` | `Upload*` / `Complete*` | 返回错误或要求新建会话 |

## 9. 各模式下的状态约束

### 9.1 `INLINE`

要求：

- 不应有 `provider_upload_id`
- `total_parts` 固定为 `1`
- 不允许调用 `IssuePartUploadUrls`
- 完成方式通常由服务端上传后内部直接转为 `COMPLETING`

推荐状态路径：

`INITIATED -> UPLOADING -> COMPLETING -> COMPLETED`

### 9.2 `DIRECT`

要求：

- 必须有 `provider_upload_id`
- 必须有 `object_key`
- `total_parts >= 1`
- 对象存储 authoritative list parts 是权威来源
- `upload_session_parts` 只保存观察值、审计信息和幂等辅助数据

推荐状态路径：

`INITIATED -> UPLOADING -> COMPLETING -> COMPLETED`

用于支持先建会话、先签发 presign、再在观察到分片后进入 `UPLOADING`。

### 9.3 `PRESIGNED_SINGLE`

要求：

- 必须有 `object_key`
- 不应有 `provider_upload_id`
- `total_parts = 1`
- complete 时必须读取对象 metadata 校验大小和类型

推荐状态路径：

`INITIATED -> COMPLETING -> COMPLETED`

也允许：

`INITIATED -> UPLOADING -> COMPLETING -> COMPLETED`

当服务检测到对象已上传但还未 complete 时，可将状态标为 `UPLOADING`。

## 10. 并发与幂等语义

### 10.1 创建会话幂等

若请求携带相同：

- `tenant_id`
- `owner_id`
- `upload_mode`
- `target_access_level`
- `expected_size`
- `file_hash`

且存在活跃会话，系统可复用现有会话，而不是盲目新建。

活跃会话集合建议定义为：

- `INITIATED`
- `UPLOADING`
- `COMPLETING`

不包括：

- `COMPLETED`
- `ABORTED`
- `EXPIRED`
- `FAILED`

### 10.2 complete 幂等

`complete` 必须满足：

- 同一 `upload_session_id` 最终只能生成一个 `file_id`
- 如果会话已 `COMPLETED`，重复 complete 返回同一个 `file_id`
- 如果其他请求已经把状态推进到 `COMPLETING`，后来的请求只能等待结果，不得再次执行副作用

### 10.3 abort 幂等

`AbortSession` 必须满足：

- `ABORTED` 再次 abort 返回成功或幂等空结果
- `EXPIRED` 再次 abort 返回幂等空结果
- `FAILED` 再次 abort 返回幂等空结果
- `COMPLETED` 不允许 abort

### 10.4 part 上传幂等

对 `DIRECT`：

- 相同 `part_number` 可重复上传
- 权威结果以对象存储中最后可见的已上传 part 为准
- `upload_session_parts` 可由 complete 前同步，也可由 progress / reconcile 时回填

## 11. 完成阶段的副作用顺序

`COMPLETING` 阶段推荐执行顺序：

1. 校验当前会话仍属于调用者且未过期
2. CAS 将状态从 `INITIATED/UPLOADING` 推进到 `COMPLETING`
3. 从对象存储读取 authoritative 对象或 part 列表
4. 校验 `expected_size`、parts、etag
5. 执行对象存储 complete 或确认单对象存在
6. 事务内写 `blob_objects`
7. 事务内写 `file_assets`
8. 事务内更新 `tenant_usage`
9. 事务内把 `upload_sessions` 标为 `COMPLETED` 并写入 `file_id`
10. 写 `outbox_events`

任何顺序偏差都会导致状态和事实表不一致。

## 12. 过期规则

### 12.1 过期判定

会话同时满足以下条件时可视为过期：

- `now > expires_at`
- 当前状态仍不是终态

### 12.2 过期推进

必须同时具备两层推进：

1. 惰性推进
在读取或操作时发现会话已过期，当前请求必须按 `EXPIRED` 语义拒绝继续上传或 complete

2. 后台推进
`job-service` 定时扫描“未终态且超过 `expires_at`”的会话，推进状态并执行清理副作用

### 12.3 过期副作用

对 `DIRECT`：

- 调用对象存储 `abort multipart upload`

对 `PRESIGNED_SINGLE`：

- 删除已上传但未完成的对象

对 `INLINE`：

- 若对象已落地但文件未完成，按内部实现选择删除对象或标记待 GC

## 13. 失败规则

建议把失败分为两类：

### 13.1 可重试失败

例如：

- complete 时网络闪断
- 等待完成超时

处理建议：

- 不立即推进到 `FAILED`
- 先保留在 `COMPLETING` 或返回“稍后重试”

### 13.2 不可恢复失败

例如：

- 元数据损坏
- 对象缺失且无法恢复
- 事务关键步骤失败后需要人工介入

处理建议：

- 推进到 `FAILED`
- 写入 `failure_code` 和 `failure_message`

## 14. 允许与禁止的转移

### 14.1 允许

- `INITIATED -> UPLOADING`
- `INITIATED -> COMPLETING`
- `INITIATED -> ABORTED`
- `INITIATED -> EXPIRED`
- `UPLOADING -> COMPLETING`
- `UPLOADING -> ABORTED`
- `UPLOADING -> EXPIRED`
- `COMPLETING -> COMPLETED`
- `COMPLETING -> FAILED`
- `COMPLETING -> EXPIRED`

### 14.2 禁止

- `COMPLETED -> ABORTED`
- `COMPLETED -> EXPIRED`
- `ABORTED -> UPLOADING`
- `EXPIRED -> UPLOADING`
- `FAILED -> COMPLETING`

## 15. 数据库写约束建议

在 [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md) 基础上，`upload.upload_sessions` 额外建议：

- `COMPLETED` 时 `file_id` 必须非空
- `DIRECT` 时 `provider_upload_id` 必须非空
- `PRESIGNED_SINGLE` 时 `provider_upload_id` 必须为空
- `DIRECT` 时 `total_parts >= 1`
- 终态写入必须带 `updated_at`

如果使用 SQL `CHECK` 难以表达全部模式约束，至少要在应用层强校验。

## 16. 可观测性要求

每次状态迁移都应输出：

- `upload_session_id`
- `tenant_id`
- `owner_id`
- `upload_mode`
- `from_status`
- `to_status`
- `reason`
- `request_id`

建议指标：

- `upload_session_transitions_total{from,to,mode}`
- `upload_session_terminal_total{status,mode}`
- `upload_session_completing_duration_seconds`
- `upload_session_expired_total{mode}`
- `upload_session_failed_total{mode,reason}`

## 17. 测试要求

状态机测试至少覆盖：

1. `DIRECT` 正常 complete
2. `PRESIGNED_SINGLE` 正常 complete
3. `INLINE` 正常 complete
4. 重复 complete 返回同一 `file_id`
5. 并发 complete 只有一个请求获得所有权
6. 过期 session 无法继续上传
7. `COMPLETED` session 无法 abort
8. `ABORTED` / `EXPIRED` / `FAILED` 不能恢复上传
9. 命中秒传时直接创建 `COMPLETED` 会话
10. 同哈希活跃会话复用规则正确

## 18. 最终建议

这份状态机的核心不是“状态名有哪些”，而是三条硬规则：

1. `UploadSession` 只表示上传过程，不表示文件事实
2. `COMPLETING` 是并发 complete 的唯一临界区
3. `COMPLETED` 必须绑定唯一 `file_id`，且对后续重复请求幂等

如果这三条守住了，Go 版的 upload-service 才会在多模式上传、并发 complete、异常恢复这几个高风险点上稳定下来。
