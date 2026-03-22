# job-service findings

## Confirmed Constraints

- 多实例调度必须先拿分布式锁
- `FOR UPDATE SKIP LOCKED` 不能替代调度层锁
- 物理删除前必须再次确认 refcount 和逻辑删除状态
- 不允许靠单副本假设上线 cleanup / reconcile / repair
