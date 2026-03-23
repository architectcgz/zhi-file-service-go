# zhi-file-service-go 开发启动与命令约定文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的本地开发启动方式、`Makefile` 目标设计、脚本边界和工程命令约定。

它解决的问题：

1. 新成员如何在本地稳定拉起开发环境
2. `Makefile`、`scripts/`、服务入口各自负责什么，不负责什么
3. migration、seed、bucket 初始化、服务启动应该怎么分层执行
4. 后续 CI、联调、压测如何复用同一套命令面，而不是各写各的

配套文档：

- [service-layout-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/service-layout-spec.md)
- [migration-bootstrap-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/migration-bootstrap-spec.md)
- [deployment-runtime-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/deployment-runtime-spec.md)
- [test-validation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/test-validation-spec.md)
- [code-style-guide.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/code-style-guide.md)
- [configuration-registry-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/configuration-registry-spec.md)

## 2. 核心结论

### 2.1 `Makefile` 是统一入口，`scripts/` 是实现细节

仓库对开发者、CI、自动化工具暴露的正式命令入口应统一为：

- `make <target>`

`scripts/` 的职责是承载底层实现细节，而不是让每个人记住不同脚本路径和参数。

### 2.2 一个命令只做一类事

命令职责固定分为五类：

1. 依赖环境
2. 数据库与对象存储 bootstrap
3. 服务运行
4. 测试验证
5. 工具检查

不要写一个“万能 bootstrap 脚本”把：

- 拉依赖
- 建库
- migration
- seed
- 启动四个服务
- 跑测试

全部塞在一起。

### 2.3 服务启动必须保持纯净

`cmd/<service>/main.go` 只能负责进程启动，不负责：

- migration
- seed
- bucket 创建
- 自动补环境
- 自动修 schema

这些动作必须通过显式命令完成。

### 2.4 本地依赖容器化，服务进程本机运行

第一阶段本地开发模式固定为：

- PostgreSQL / Redis / MinIO 通过容器拉起
- 四个 Go 服务默认在本机进程中运行

原因：

- 更方便调试和断点
- 更适合 `go test`、pprof、race、trace
- 与生产的“无状态服务 + 外部依赖”模式保持一致

### 2.5 CI 必须复用相同命令面

CI 不应自己拼接一套平行命令。

本地和 CI 的差异可以体现在：

- 环境变量
- 目标参数
- 执行范围

但正式入口统一为，例如：

- `make migrate-up`
- `make test-integration`
- `make openapi-validate`

## 3. 命令面目录布局

第一阶段命令面目录固定为以下布局：

```text
Makefile
.env.example
scripts/
  bootstrap/
    deps-up.sh
    deps-down.sh
    db-create.sh
    bucket-init.sh
    migrate-build.sh
    migrate-up.sh
    seed-dev.sh
  dev/
    run-upload.sh
    run-access.sh
    run-admin.sh
    run-job.sh
    run-all.sh
  test/
    test-unit.sh
    test-integration.sh
    test-contract.sh
    test-e2e.sh
    test-performance.sh
  tools/
    openapi-validate.sh
    lint.sh
    fmt.sh
```

说明：

- `Makefile` 提供稳定入口
- `scripts/bootstrap` 负责环境引导
- `scripts/dev` 负责本地服务运行
- `scripts/test` 负责测试套件装配
- `scripts/tools` 负责静态检查和契约检查

## 4. `Makefile` 目标设计

## 4.1 目标分组

目标分组固定为：

- `deps-*`
- `db-*`
- `bucket-*`
- `migrate-*`
- `seed-*`
- `bootstrap-*`
- `run-*`
- `test-*`
- `lint-*`
- `fmt-*`
- `openapi-*`
- `doctor`

这样目标名一眼就能看出职责，不需要打开脚本猜。

## 4.2 依赖环境目标

第一阶段正式目标固定为：

- `make deps-up`
- `make deps-down`
- `make deps-reset`
- `make deps-logs`

职责：

- 拉起 / 停止 PostgreSQL、Redis、MinIO
- 不负责 migration
- 不负责 seed
- 不负责启动 Go 服务

## 4.3 数据库与对象存储目标

第一阶段正式目标固定为：

- `make db-create`
- `make db-drop`
- `make bucket-init`

职责：

- 创建开发数据库
- 清理并重建本地数据库
- 初始化 MinIO bucket 和基础前缀

禁止：

- 在 `run-*` 目标中隐式执行这些动作

## 4.4 migration 目标

第一阶段正式目标固定为：

- `make migrate-build`
- `make migrate-up`
- `make migrate-down`
- `make migrate-status`
- `make migrate-reset`

职责：

- 构建统一 migration 执行视图
- 执行 schema 变更
- 查看当前版本状态

约束：

- 必须遵循 [migration-bootstrap-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/migration-bootstrap-spec.md)
- `migrate-reset` 仅允许本地开发环境使用
- 生产环境不提供一键 reset 入口

## 4.5 seed 目标

第一阶段正式目标固定为：

- `make seed-dev`
- `make seed-test`

职责：

- 装入开发或测试所需最小样本数据

约束：

- `seed-dev` 必须幂等
- `seed-test` 只服务于自动化环境，不写 demo 垃圾数据
- 不提供 `seed-prod`

## 4.6 本地 bootstrap 目标

第一阶段正式目标固定为：

- `make bootstrap-local`

其内部顺序固定为：

1. `deps-up`
2. `db-create`
3. `migrate-build`
4. `migrate-up`
5. `seed-dev`
6. `bucket-init`

`bootstrap-local` 的职责到这里结束，不继续启动服务。

### 原因

- bootstrap 关注的是“环境可用”
- 运行服务关注的是“进程生命周期”

这两个责任不应混在一起。

## 4.7 服务运行目标

第一阶段正式目标固定为：

- `make run-upload`
- `make run-access`
- `make run-admin`
- `make run-job`

可选保留：

- `make run-all`

规则：

- `run-*` 默认只启动对应服务
- 不执行 migration
- 不执行 seed
- 不隐式创建 bucket

`run-all` 只允许作为本地开发 convenience wrapper。

它本质上应当只是：

- 同时拉起四个本地进程
- 聚合日志输出
- 支持快速停止

不能把它做成生产部署思路的替代品。

## 4.8 测试目标

第一阶段正式目标固定为：

- `make test-unit`
- `make test-integration`
- `make test-contract`
- `make test-e2e`
- `make test-performance`
- `make test-all`

要求：

- 与 [test-validation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/test-validation-spec.md) 的层次完全对齐
- `test-all` 可以组合多类测试，但不替代分层目标

## 4.9 工具目标

第一阶段正式目标固定为：

- `make fmt`
- `make lint`
- `make openapi-validate`
- `make doctor`

职责：

- `fmt`: 执行 `gofmt` / `goimports`
- `lint`: 执行 lint 和静态检查
- `openapi-validate`: 校验 `api/openapi/*.yaml`
- `doctor`: 检查本地依赖、关键环境变量、端口占用、工具链可用性

## 5. 环境变量与配置约定

## 5.1 本地环境文件

固定保留：

- `.env.example`
- `.env.local`

规则：

- `.env.example` 提供非敏感示例
- `.env.local` 仅用于开发者本地，不提交 Git
- 真正的 Secret 不写入 `.env.example`
- `.env.example` 与 `make doctor` 检查项必须从配置注册表派生

## 5.2 配置注入优先级

本地开发也必须遵循与运行时一致的覆盖顺序：

1. 默认值
2. `.env.local`
3. shell 显式导出
4. `make` 目标参数覆盖

不要让脚本在多个文件里偷偷覆盖同名配置，导致启动行为不可预测。

## 5.3 服务级配置隔离

即使本地开发，也必须按服务划分配置前缀，例如：

- `UPLOAD_*`
- `ACCESS_*`
- `ADMIN_*`
- `JOB_*`

避免把四个服务所有参数都摊平在一个没有前缀的大配置文件中。

## 6. 本地开发推荐流程

## 6.1 首次启动

推荐固定流程：

1. `make doctor`
2. `make bootstrap-local`
3. `make run-upload`
4. `make run-access`
5. `make run-admin`
6. `make run-job`

## 6.2 常规开发

如果只是改单个服务代码，推荐流程：

1. 保持依赖环境存活
2. 如无 schema 变化，不重复执行 bootstrap
3. 只重启受影响服务
4. 跑受影响范围测试

## 6.3 表结构变更

若改动涉及 migration，推荐流程：

1. 更新 migration 文件
2. `make migrate-build`
3. `make migrate-up`
4. 跑 integration / contract / e2e
5. 必要时 `make migrate-status` 复核版本状态

## 6.4 全量重建

若本地环境已污染，推荐：

1. `make deps-reset`
2. `make bootstrap-local`

不要通过手工删一半容器、手工补一半数据的方式修环境。

## 7. 脚本编写规范

## 7.1 Shell 脚本必须简单直接

脚本优先用于：

- 封装命令
- 编排固定顺序
- 输出可读日志

脚本不应用于承载：

- 业务规则
- 隐式环境修复逻辑
- 难以追踪的大量条件分支

## 7.2 脚本风格

推荐 shell 脚本默认使用：

```bash
set -euo pipefail
```

要求：

- 失败立即退出
- 日志输出清楚显示当前步骤
- 关键路径参数显式校验

## 7.3 幂等性

以下脚本必须尽量幂等：

- `deps-up`
- `db-create`
- `bucket-init`
- `seed-dev`
- `migrate-build`

这意味着重复执行不应轻易把环境搞坏。

## 7.4 危险操作保护

以下动作必须显式区分环境：

- `db-drop`
- `migrate-reset`
- `deps-reset`

要求：

- 默认只允许本地环境
- 必要时增加确认开关或环境校验
- 禁止在非本地环境无保护执行

## 8. `run-all` 与进程管理约束

如果后续需要 `make run-all`，推荐实现为本地开发专用聚合器。

可选方式：

- 多终端手工启动
- `Procfile`
- 轻量进程管理器

无论采用哪一种，必须满足：

- 四个服务仍是四个独立进程
- 可以单独重启某个服务
- 日志能区分服务来源
- 停止时能优雅退出

不要为了本地方便，把四个服务重新拼回单进程大应用。

## 9. CI 映射规则

CI 正式复用以下目标：

- `make fmt`
- `make lint`
- `make openapi-validate`
- `make migrate-up`
- `make test-unit`
- `make test-integration`
- `make test-contract`
- `make test-e2e`

规则：

- CI 可以不跑 `run-*`
- CI 可以跳过 `seed-dev`，改用 `seed-test`
- CI 仍应通过显式命令完成 bootstrap，而不是自定义隐藏逻辑

## 10. 未来落地顺序建议

真正开始实现命令面时，建议按以下顺序落地：

1. `.env.example`
2. `Makefile`
3. `scripts/bootstrap/*`
4. `scripts/tools/*`
5. `scripts/test/*`
6. `scripts/dev/run-*.sh`
7. 可选 `run-all`

原因：

- 先把环境引导和验证链路定住
- 再补服务运行便利性

## 11. Code Review 拦截项

看到以下实现应直接拦截：

- 在 `main.go` 中自动执行 migration 或 seed
- `run-*` 目标隐式重建数据库或创建 bucket
- 一个脚本同时做 bootstrap、启动服务、跑测试三件事
- CI 使用一套本地不存在的隐藏命令
- 把 demo Secret、真实账号或生产地址写进 `.env.example`
- 为了本地开发方便把四个服务重新合并成单进程

## 12. 最终建议

这份文档的核心只有五条：

1. `Makefile` 是统一入口，`scripts/` 是实现细节
2. migration / seed / bucket 初始化必须显式执行，不能混进服务启动
3. 本地依赖容器化，服务默认本机运行
4. 本地、CI、自动化工具必须复用同一套命令面
5. 便利性不能破坏已经定下的服务边界和运行时边界

如果这套命令约定不先写死，后面脚本和启动方式很容易各写各的，最后又会把环境治理、服务启动和数据初始化重新搅在一起。
