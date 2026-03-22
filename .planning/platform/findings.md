# platform findings

## Confirmed Constraints

- 平台层只放跨服务运行时基础设施
- readiness / liveness / metrics 是必备能力
- 服务启动不得隐式执行 migration、seed、bucket 初始化
- 多服务必须复用统一 bootstrap，而不是各自拼装
