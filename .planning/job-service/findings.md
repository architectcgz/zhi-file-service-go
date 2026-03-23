# job-service findings

## Confirmed Constraints

- 多实例调度必须先拿分布式锁
- `FOR UPDATE SKIP LOCKED` 不能替代调度层锁
- 物理删除前必须再次确认 refcount 和逻辑删除状态
- 不允许靠单副本假设上线 cleanup / reconcile / repair

## Implementation Findings

- `scheduler`、`jobs`、`outbox consumer`、observability、`infra/postgres`、`infra/runner`、`infra/storage` 均已落地，应用层与运行时测试覆盖关键边界
- `cmd/job-service/main.go` 当前通过 `bootstrap.New(...) -> jobruntime.Build(...) -> app.RegisterRuntime(...)` 启动，scheduler 已挂进进程生命周期并受 readiness/start/stop 管理
- `runtime/catalog.go` 已注册 `process_outbox_events`、`cleanup_multipart` 在内的七类周期任务，不再只停留在库级实现
- `runtime_integration_test.go` 已覆盖 admin delete -> outbox -> finalize physical delete，以及 upload fail -> outbox -> cleanup multipart 的系统级闭环
- `job-service` 当前仍以后台 runtime 为主，不要求完整 north-south 业务 API；健康检查、调度生命周期管理和多实例锁实现已具备
