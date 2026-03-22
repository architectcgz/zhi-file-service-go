# zhi-file-service-go Planning

计划统一放在仓库根目录 `.planning/` 下。

规则固定为：

- 每个模块一个目录
- 每个目录内固定三份文件：
  - `task_plan.md`
  - `findings.md`
  - `progress.md`
- 不再在 `docs/` 下维护实施计划

当前模块：

- `platform/`
- `pkg/`
- `migrations/`
- `upload-service/`
- `access-service/`
- `admin-service/`
- `job-service/`

执行顺序：

1. `platform` -> `pkg` -> `migrations`
2. `upload-service` || `access-service`
3. `admin-service` || `job-service`
