# job-service task plan

## Goal

把 job-service 从“调度与任务编排骨架”推进到“多实例可运行的后台服务”，确保锁、runner、补偿和清理任务真正能执行，并完成 Phase 6 runtime 收口。

## Inputs

- `docs/services/job-service-implementation-spec.md`
- `docs/api/outbox-event-spec.md`
- `docs/ops/data-protection-recovery-spec.md`
- `docs/ops/deployment-runtime-spec.md`
- `docs/dev/test-validation-spec.md`

## Phases

### Phase 1 (`completed`)

- 落 scheduler / worker 模型
- 落 distributed locker 抽象

### Phase 2 (`completed`)

- 实现 expire session / repair completing / finalize delete / cleanup orphan / reconcile usage / cleanup multipart

### Phase 3 (`completed`)

- 实现 `infra/postgres` 仓储与分布式锁
- 实现 `infra/storage` / `infra/runner`，把 jobs 所需依赖接到真实资源
- 明确 outbox 读取、ack、重试与任务 side effect 的基础设施边界

### Phase 4 (`completed`)

- 构建 runtime，接入 scheduler 生命周期、ready check、graceful shutdown
- 修改 `cmd/job-service` 为显式 runtime 注册，而不是空跑 `bootstrap.Run(...)`
- 按设计决定是否只暴露 probe/metrics，还是增加最小管理 HTTP handler

### Phase 5 (`completed`)

- 补真实 locker、接管、幂等、repair/reconcile、cleanup multipart 的集成测试
- 跑 `go test` / `-race`，验证多实例竞争下不会重复调度或误删对象

### Phase 6 (`completed`)

- 把 `process_outbox_events`、`cleanup_multipart` 与其余维护任务接进 runtime scheduler，并由 `buildScheduledJobs(...)` 统一编目
- 为验证链路补齐 job 相关闭环：逻辑删除后物理清理、upload fail 后 outbox 消费与 multipart cleanup
- 对齐运行与配置说明，补齐 interval / lock / scheduler 相关配置键

## Deliverables

- `internal/services/job`
- `cmd/job-service`
- 对应 scheduler / runner / infra / runtime / tests

## Exit Criteria

- 多实例 cleanup / repair / reconcile 全部依赖分布式锁
- outbox 驱动任务与主事务边界一致
- scheduler 已真正接入进程生命周期，`/ready` 能反映 runtime 状态
- 关键任务失败、重试、接管行为可验证
- `process_outbox_events`、`cleanup_multipart` 等关键任务已注册进 runtime，本模块可从活跃列表移除
