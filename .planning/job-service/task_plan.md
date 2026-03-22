# job-service task plan

## Goal

落后台调度、分布式锁、清理、补偿、修复与对账任务，确保多实例下不会重复调度或误删对象。

## Inputs

- `docs/job-service-implementation-spec.md`
- `docs/outbox-event-spec.md`
- `docs/data-protection-recovery-spec.md`
- `docs/deployment-runtime-spec.md`

## Phases

### Phase 1

- 落 scheduler / worker 模型
- 落 distributed locker 抽象

### Phase 2

- 实现 expire session / repair completing / finalize delete / cleanup orphan / reconcile usage / cleanup multipart

### Phase 3

- 接入 outbox、失败重试、幂等和多实例领取控制

### Phase 4

- 接入指标、日志、trace、任务健康信息

### Phase 5

- 补分布式锁、接管、幂等、repair/reconcile 测试

## Deliverables

- `internal/services/job`
- 对应 scheduler / runner / infra / tests

## Exit Criteria

- 多实例 cleanup / repair / reconcile 全部依赖分布式锁
- outbox 驱动任务与主事务边界一致
- 关键任务失败、重试、接管行为可验证
