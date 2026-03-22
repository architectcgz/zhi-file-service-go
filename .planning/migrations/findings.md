# migrations findings

## Confirmed Constraints

- 当前项目不做旧系统迁移兼容
- canonical schema 可以按 Go 重写版重新定义
- migration 必须独立于服务启动执行
- 数据库事实与对象存储事实都要考虑恢复与清理

## Implementation Findings

- 分目录 migration 不能直接交给 `golang-migrate`，必须先汇总成线性执行视图
- `migrate-build.sh` 应复用 Go runner，避免 Bash 和 Go 维护两套排序/校验规则
- seed 目录需要迁回 `bootstrap/seed/{dev,test}`，不能继续依赖旧的 `test/fixtures/seed-*.sql` 形式
