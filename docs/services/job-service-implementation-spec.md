# zhi-file-service-go job-service 实现细节文档

## 1. 目标

这份文档定义 `job-service` 的实现细节。

它的目标是提前固定后台任务的执行模型，避免后续把补偿、清理、修复逻辑零散塞回在线服务。

配套文档：

- [architecture-upgrade-design.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/architecture-upgrade-design.md)
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md)
- [admin-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-service-implementation-spec.md)
- [outbox-event-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/outbox-event-spec.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)

## 2. 服务职责

明确职责：

- 过期 upload session 清理
- stuck `COMPLETING` / `FAILED` 会话修复
- multipart 残留清理
- orphan blob 清理
- 文件物理删除
- tenant usage 对账
- outbox 驱动的补偿任务

明确不负责：

- 对外 north-south API
- 热路径同步业务决策
- 管理后台页面查询

## 3. 服务内包结构

推荐结构：

```text
internal/services/job/
  app/
    scheduler/
    jobs/
      expire_upload_sessions.go
      repair_stuck_completing.go
      finalize_file_delete.go
      cleanup_orphan_blobs.go
      reconcile_tenant_usage.go
      cleanup_multipart.go
  ports/
    upload_session_repository.go
    file_cleanup_repository.go
    blob_repository.go
    tenant_usage_repository.go
    outbox_reader.go
    distributed_locker.go
    storage_ports.go
  infra/
    postgres/
    storage/
    runner/
```

`job-service` 第一阶段不需要单独 `domain/`，以任务编排和幂等流程为主。

## 4. 执行模型

## 4.1 调度与执行分离

推荐结构：

- `scheduler`: 定时扫描、发起任务
- `worker`: 并发执行具体任务

职责分离后：

- 调度频率更容易控制
- 执行并发更容易限流

## 4.2 避免多副本重复执行

在 Kubernetes 多副本下，必须保证同类任务不会被所有副本同时重复调度。

这是硬约束：

- 多实例清理任务必须先拿分布式锁
- 没有分布式锁，不允许上线多实例 cleanup / reconcile / repair 任务

推荐两层保护：

1. 调度层使用分布式锁
2. 任务领取使用 `FOR UPDATE SKIP LOCKED`

这里要明确：

- `FOR UPDATE SKIP LOCKED` 只能解决“多个 worker 如何分片领取同一批任务”
- 它不能替代“哪一个实例有资格启动本轮清理调度”

推荐分布式锁实现：

- Redis 租约锁
- 或基于共享 PostgreSQL 的 advisory lock

无论底层用哪种实现，都必须满足：

- 带 TTL 或租约过期
- 支持 owner 标识
- 支持续租
- 进程崩溃后最终可释放
- 同一个 `job_name` 在同一时间只能有一个 scheduler leader

禁止仅靠“副本数暂时设为 1”。

## 4.3 任务必须幂等

任何任务都必须满足：

- 重跑不产生额外坏结果
- 单次失败可安全重试
- 成功与失败都有明确状态

## 5. 任务目录

## 5.1 `ExpireUploadSessionsJob`

职责：

- 扫描超过 TTL 且未完成的 session
- 将其推进到 `EXPIRED`
- 为 `DIRECT` 模式补充 multipart abort 清理信号

约束：

- 不直接扫全表
- 按分页或批量窗口执行

## 5.2 `RepairStuckCompletingJob`

职责：

- 扫描长时间停留在 `COMPLETING` 的 session
- 判断对象是否已实际完成
- 能补完成的补完成
- 不能补完成的标记 `FAILED` 并记录原因

这是 upload complete 两阶段/三阶段实现的必备配套任务，不能省略。

## 5.3 `FinalizeFileDeleteJob`

职责：

- 处理后台删除文件后的 outbox 事件
- 若 blob 引用计数为 0，则删除对象存储对象
- 记录物理删除结果

约束：

- 只处理 `deleted_at + job.file_delete_retention <= now` 的记录
- 必须再次确认 refcount 为 0
- 不允许凭 outbox 事件盲删对象

## 5.4 `CleanupOrphanBlobsJob`

职责：

- 扫描长时间无引用的 blob
- 检查是否确实没有 `file_asset` 关联
- 执行物理删除或标记待人工处理

## 5.5 `ReconcileTenantUsageJob`

职责：

- 定期对账 `tenant_usage`
- 修复极端故障下的聚合漂移

约束：

- 低频执行
- 不进入高频实时请求链路

## 5.6 `CleanupMultipartJob`

职责：

- 清理 provider 侧已中止、已过期或长期未完成的 multipart 上下文

## 6. 任务实现细节

## 6.1 批处理策略

默认每个 job 都要支持：

- `batch_size`
- `max_concurrency`
- `scan_interval`
- `retry_backoff`
- `lock_ttl`
- `lock_renew_interval`

不要把这些值写死在代码里。

## 6.2 出错处理

任务执行失败后必须明确记录：

- job name
- resource id
- retry count
- error
- next retry time

不允许只打一行日志然后靠人工猜。

## 6.3 Outbox 消费

凡是由在线事务发出的后台补偿事件，统一走 `infra.outbox_events`。

处理原则：

- 读取未消费事件
- 领取后执行
- 成功标记已消费
- 失败记录重试信息

第一阶段不引入 Kafka，不等于允许每个在线服务各写一套“自己扫表补偿”逻辑。

## 6.4 分布式锁接口约束

建议抽象：

```go
type DistributedLocker interface {
    Acquire(ctx context.Context, key string, ttl time.Duration) (LockHandle, bool, error)
}

type LockHandle interface {
    Renew(ctx context.Context, ttl time.Duration) error
    Release(ctx context.Context) error
}
```

使用原则：

- key 必须稳定，例如 `job:cleanup_orphan_blobs`
- scheduler 启动前先 `Acquire`
- 长任务执行期间定期 `Renew`
- 任务退出时显式 `Release`

禁止：

- 用内存锁代替分布式锁
- 只在应用启动时抢一次锁后长期不续租
- 锁过期后仍继续执行无边界清理

## 7. 与其他服务的边界

`job-service` 可以访问：

- PostgreSQL
- 对象存储
- 可选 Redis

但默认不通过同步 RPC 依赖在线服务。

原因：

- 后台修复要独立于在线流量存活
- 在线服务故障时，后台补偿仍需继续推进

## 8. 配置项

建议至少提供：

- `job.scheduler_enabled`
- `job.default_batch_size`
- `job.default_max_concurrency`
- `job.lock_backend`
- `job.lock_ttl`
- `job.lock_renew_interval`
- `job.expire_upload_sessions.interval`
- `job.repair_stuck_completing.interval`
- `job.finalize_file_delete.interval`
- `job.file_delete_retention`
- `job.cleanup_orphan_blobs.interval`
- `job.reconcile_tenant_usage.interval`

## 9. 可观测性

关键指标：

- `job_run_total`
- `job_run_failed_total`
- `job_duration_seconds`
- `job_items_processed_total`
- `job_retry_total`

关键日志字段：

- `job_name`
- `resource_id`
- `retry_count`
- `batch_size`
- `worker_id`

关键 trace span：

- `job.expire_upload_sessions`
- `job.repair_stuck_completing`
- `job.finalize_file_delete`
- `job.cleanup_orphan_blobs`

## 10. 测试要求

必须覆盖：

- 分布式锁 acquire / renew / release
- 锁过期后其他实例接管
- 重复调度不重复处理
- `FOR UPDATE SKIP LOCKED` 领取行为
- stuck completing 修复
- file delete 物理清理幂等
- usage 对账修复

## 11. Code Review 检查项

看到以下实现应直接拦截：

- 在线请求里直接做后台清理逻辑
- 多实例 cleanup 任务没有分布式锁
- 把 `FOR UPDATE SKIP LOCKED` 当成分布式锁替代品
- 任务失败只记日志不留重试状态
- 文件物理删除不复核 refcount
