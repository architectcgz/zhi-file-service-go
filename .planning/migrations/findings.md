# migrations findings

## Confirmed Constraints

- 当前项目不做旧系统迁移兼容
- canonical schema 可以按 Go 重写版重新定义
- migration 必须独立于服务启动执行
- 数据库事实与对象存储事实都要考虑恢复与清理
