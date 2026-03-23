# job-service findings

## Confirmed Constraints

- 多实例调度必须先拿分布式锁
- `FOR UPDATE SKIP LOCKED` 不能替代调度层锁
- 物理删除前必须再次确认 refcount 和逻辑删除状态
- 不允许靠单副本假设上线 cleanup / reconcile / repair

## Implementation Findings

- `scheduler`、`jobs`、`outbox consumer` 和 observability 接缝已落地，应用层测试也已覆盖关键边界
- `infra/postgres`、`infra/runner`、`infra/storage` 仍为空目录，分布式锁和仓储还没有真实实现
- `cmd/job-service/main.go` 当前仅调用 `bootstrap.Run(...)`，没有 runtime ready check，也没有把 scheduler 真正挂进进程生命周期
- `job-service` 第一阶段不要求完整 north-south 业务 API，但至少需要健康检查、调度生命周期管理和多实例锁实现
