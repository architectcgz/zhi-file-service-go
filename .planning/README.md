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
- 当前活跃：
  - 当前无活跃模块；上一轮收尾项已随 `449f316`、`8db1b16`、`4d0f5cc` 落到 `main`

当前代码事实：

- 四个服务当前都已有 `cmd/*` 启动入口；`admin-service`、`job-service` 均通过 `Build(...) + RegisterRuntime(...)` 接入 runtime 生命周期与 readiness
- `admin-service` 已切到 JWKS 管理员鉴权，runtime 使用 `ADMIN_AUTH_JWKS` 与 `ADMIN_AUTH_ALLOWED_ISSUERS` 构建生产认证链路，不再把 dev resolver 当主路径
- `job-service` runtime 已注册 `expire_upload_sessions`、`repair_stuck_completing`、`process_outbox_events`、`finalize_file_delete`、`cleanup_multipart`、`cleanup_orphan_blobs`、`reconcile_tenant_usage` 全量任务
- `test/contract/`、`test/e2e/`、`deployments/helm/`、`deployments/kustomize/`、`test/performance/` 均为正式资产，不再是占位目录
- `Makefile` 与 `scripts/test/{contract,e2e,performance}.sh` 已提供统一验证入口，`delivery-validation` 不再缺统一测试收口

建议执行顺序：

1. 先把 `admin-service` planning 转入归档视角，保留 JWKS/issuer allowlist 已完成事实
2. 再把 `job-service` planning 转入归档视角，移除 Phase 6 未接线缺口表述
3. 最后把 `delivery-validation` planning 转入归档视角，以统一测试入口作为后续维护基线
