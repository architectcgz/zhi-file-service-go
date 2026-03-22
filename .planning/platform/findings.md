# platform findings

## Confirmed Constraints

- 平台层只放跨服务运行时基础设施
- readiness / liveness / metrics 是必备能力
- 服务启动不得隐式执行 migration、seed、bucket 初始化
- 多服务必须复用统一 bootstrap，而不是各自拼装

## Implementation Findings

- `upload-service` 和 `job-service` 将 Redis 视为必需依赖；其他服务允许无 Redis 启动
- `/live`、`/ready`、`/metrics` 统一走主监听端口，不拆单独管理端口
- request trace 需要在平台层统一植入，否则后续业务服务很容易出现日志有 `request_id` 但没有 `trace_id` / `span_id` 的漂移
- graceful shutdown 至少要覆盖 HTTP server、数据库连接池、Redis client 和 tracer provider flush
- 在业务 runtime 尚未接入前，服务不得通过 readiness，避免出现“探针绿、业务流量全 404”的假健康
- 对象存储在 startup 和 readiness 都要做真实 `HeadBucket` 校验，不能只验证 endpoint/bucket 配置字符串
- `UPLOAD_ALLOWED_MODES` 默认值与多份设计文档统一为 `INLINE,PRESIGNED_SINGLE,DIRECT`
- `bootstrap.NewWithOptions` / `bootstrap.RunWithOptions` 已作为后续服务模块注册 handler / runtime ready check 的正式入口
