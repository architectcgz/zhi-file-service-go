# zhi-file-service-go Planning

计划统一放在仓库根目录 `.planning/` 下。

规则固定为：

- 每个模块一个目录
- 每个目录内固定三份文件：
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
- 不再在 `docs/` 下维护实施计划

当前模块分两类：

- 已完成归档：
  - `platform/`
  - `pkg/`
  - `migrations/`
  - `upload-service/`
  - `access-service/`
- 当前活跃：
  - `admin-service/`
  - `job-service/`
  - `delivery-validation/`

当前代码事实：

- 四个服务当前都已有 `cmd/*` 启动入口，主仓库 `go test ./...` 可通过
- `upload-service`、`access-service` 都已有性能资产，`test/performance/` 不再只有 upload 一项
- `admin-service` 已接通 Postgres / HTTP / runtime，但鉴权仍是 dev resolver，生产 JWKS 仍待补
- `job-service` 已接通 runner / Redis 分布式锁 / runtime，但 scheduler 目前只注册核心维护任务，outbox 与 multipart 清理还未接进运行时
- `deployments/`、`test/contract/`、`test/e2e/` 已不再是占位目录
- 当前剩余交付收尾主要集中在 `admin-service`、`job-service` 与 `delivery-validation`

建议执行顺序：

1. `admin-service` 生产鉴权收口
2. `job-service` 调度任务全量接线与对应 e2e 输入
3. `delivery-validation` 性能入口与统一验证收口
