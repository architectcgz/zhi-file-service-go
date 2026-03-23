# zhi-file-service-go Outbox 事件契约文档

## 1. 目标

这份文档定义 `infra.outbox_events` 的事件契约。

它解决的问题：

1. `event_type` 应该怎么命名
2. 哪些服务会产出哪些事件
3. `payload` 里必须带什么字段
4. `job-service` 应该如何消费而不返工

配套文档：

- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md)
- [admin-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-service-implementation-spec.md)
- [job-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/job-service-implementation-spec.md)

## 2. 核心结论

### 2.1 Outbox 是系统内部异步一致性契约

`infra.outbox_events` 不是“可有可无的日志表”，而是：

- 上传 complete 后补偿
- 文件删除后物理清理
- 后台修复与清理任务

的统一事件桥梁。

### 2.2 交付语义固定为 At-Least-Once

消费者必须默认事件可能重复投递。

因此：

- 消费方必须幂等
- 不能把“只会消费一次”当成前提

### 2.3 `event_type` 命名统一为小写点分版本

统一格式：

```text
<domain>.<resource>.<action>.v1
```

例如：

- `upload.session.completed.v1`
- `file.asset.delete_requested.v1`

## 3. Envelope 规则

表结构见 [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md) 中 `infra.outbox_events`。

固定含义：

- `service_name`: 事件生产者，例如 `upload-service`
- `aggregate_type`: 聚合类型，例如 `upload_session`
- `aggregate_id`: 聚合 ID
- `event_type`: 稳定事件类型
- `payload`: 事件业务体

消费者幂等键默认使用：

- `event_id`

## 4. Payload 通用字段

建议所有 payload 至少包含：

| 字段 | 说明 |
|------|------|
| `occurredAt` | 业务事件发生时间 |
| `requestId` | 关联请求 ID |
| `tenantId` | 所属租户 |
| `producer` | 生产服务 |

禁止把整行数据库对象无差别塞进 payload。

## 5. 事件类型注册表

## 5.1 `upload.session.completed.v1`

生产者：

- `upload-service`

触发时机：

- upload complete 阶段 C 事务提交时

建议 payload：

```json
{
  "occurredAt": "2026-03-21T00:00:00Z",
  "requestId": "01HQ...",
  "tenantId": "blog",
  "producer": "upload-service",
  "uploadSessionId": "01HQ...",
  "fileId": "01HR...",
  "blobObjectId": "01HS...",
  "hashAlgorithm": "SHA256",
  "hashValue": "abc...",
  "sizeBytes": 12345
}
```

用途：

- 后台修复判定
- 事件审计
- 可选异步索引或通知

## 5.2 `upload.session.failed.v1`

生产者：

- `upload-service`

触发时机：

- complete 或 finalize 路径进入失败终态

建议 payload：

```json
{
  "occurredAt": "2026-03-21T00:00:00Z",
  "requestId": "01HQ...",
  "tenantId": "blog",
  "producer": "upload-service",
  "uploadSessionId": "01HQ...",
  "failureCode": "UPLOAD_HASH_MISMATCH",
  "failureMessage": "declared hash does not match verified hash"
}
```

用途：

- `job-service` 修复或人工排查

## 5.3 `file.asset.delete_requested.v1`

生产者：

- `admin-service`

触发时机：

- 管理员删除文件的逻辑删除事务提交时

建议 payload：

```json
{
  "occurredAt": "2026-03-21T00:00:00Z",
  "requestId": "01HQ...",
  "tenantId": "blog",
  "producer": "admin-service",
  "fileId": "01HR...",
  "blobObjectId": "01HS...",
  "deletedBy": "admin-123",
  "reason": "manual cleanup"
}
```

用途：

- `job-service` 执行物理清理

## 5.4 `blob.delete_requested.v1`

生产者：

- `admin-service`
- `job-service`

触发时机：

- 某个 `blob_object` 已确认 `reference_count = 0`

建议 payload：

```json
{
  "occurredAt": "2026-03-21T00:00:00Z",
  "requestId": "01HQ...",
  "tenantId": "blog",
  "producer": "job-service",
  "blobObjectId": "01HS...",
  "storageProvider": "MINIO",
  "bucketName": "private-bucket",
  "objectKey": "tenant/blog/..."
}
```

用途：

- 对象存储物理删除

## 5.5 `tenant.policy.updated.v1`

生产者：

- `admin-service`

触发时机：

- tenant policy 事务提交时

建议 payload：

```json
{
  "occurredAt": "2026-03-21T00:00:00Z",
  "requestId": "01HQ...",
  "tenantId": "blog",
  "producer": "admin-service",
  "updatedBy": "admin-123"
}
```

用途：

- 后台缓存失效
- 策略刷新

## 6. 消费规则

`job-service` 消费 outbox 时必须遵守：

1. 领取事件使用 `FOR UPDATE SKIP LOCKED`
2. 同类调度先拿分布式锁
3. 成功后标记 `PUBLISHED`
4. 失败后增加 `retry_count`，并写 `next_attempt_at`

禁止：

- 消费成功但不更新状态
- 失败后无限热重试
- 依据 payload 盲删对象而不复核当前数据库状态

## 7. 幂等规则

固定要求：

- 同一个 `event_id` 被重复消费不得产生额外坏结果
- 物理删除对象前必须复核 `reference_count`
- repair 任务必须复核 session 当前状态

这意味着：

- outbox 事件是“触发器”
- 数据库当前状态才是最终事实源

## 8. 演进规则

新增事件时必须：

1. 在本文件注册 `event_type`
2. 说明生产者
3. 说明 payload 字段
4. 说明消费者和幂等要求

不允许：

- 使用未注册事件类型
- 直接改变既有 `event_type` 语义
- 在不升版本的情况下删除既有 payload 关键字段

## 9. 最终结论

Outbox 事件契约必须被当成服务间内部协议来管理。

如果不先固定 `event_type` 和 payload，后面 upload/admin/job 三个服务一定会各写各的补偿逻辑，最后重新返工。
