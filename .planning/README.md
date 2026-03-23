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
  - `admin-service/`
  - `job-service/`
  - `delivery-validation/`
  - `data-plane-auth/`
  - `deployment-auth-guard/`
  - `data-plane-metrics/`
- 当前活跃：
  - `data-plane-storage-metrics/`

当前代码事实：

- 四个服务当前都已有 `cmd/*` 启动入口；`admin-service`、`job-service` 均通过 `Build(...) + RegisterRuntime(...)` 接入 runtime 生命周期与 readiness
- `admin-service` 已切到 JWKS 管理员鉴权，runtime 使用 `ADMIN_AUTH_JWKS` 与 `ADMIN_AUTH_ALLOWED_ISSUERS` 构建生产认证链路，不再把 dev resolver 当主路径
- `job-service` runtime 已注册 `expire_upload_sessions`、`repair_stuck_completing`、`process_outbox_events`、`finalize_file_delete`、`cleanup_multipart`、`cleanup_orphan_blobs`、`reconcile_tenant_usage` 全量任务
- `test/contract/`、`test/e2e/`、`deployments/helm/`、`deployments/kustomize/`、`test/performance/` 均为正式资产，不再是占位目录
- `Makefile` 与 `scripts/test/{contract,e2e,performance}.sh` 已提供统一验证入口，`delivery-validation` 不再缺统一测试收口

后续维护说明：

1. 当前活跃模块为 `data-plane-storage-metrics/`
2. 后续若出现新任务，再新增对应 `planning` 目录并按三件套（`task_plan.md`、`findings.md`、`progress.md`）维护
3. 主工作树若存在本地 `.planning/` 脏改动，不要直接合并 `planning` 分支，先完成人工对比与清理后再合并
