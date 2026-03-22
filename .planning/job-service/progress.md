# job-service progress

## Status

- `in_progress`

## Notes

- 已开始落 Phase 1：scheduler / worker 模型与 distributed locker 抽象
- 当前批次只固化调度锁、续租、释放和执行边界，不提前写具体 cleanup / reconcile SQL
