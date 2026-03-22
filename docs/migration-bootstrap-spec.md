# zhi-file-service-go 初始化建表与环境 Bootstrap 规范文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的数据库初始化建表、schema 演进和环境 bootstrap 规范。

前提约束：

- 当前 `file-service` 仍处于测试阶段
- 不考虑从旧 Java 版平滑迁移历史数据
- 可以直接以 Go 重写版的数据模型作为 canonical schema

这份文档解决的问题：

1. 新库应该如何初始化，不同 schema 的建表顺序是什么
2. 后续 schema 变更如何通过 migration 落地，而不是靠手工 SQL
3. 开发、测试、CI、发布前的 bootstrap 顺序如何统一
4. seed 数据应该放哪里，哪些环境允许自动 seed

配套文档：

- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)
- [service-layout-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/service-layout-spec.md)
- [deployment-runtime-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/deployment-runtime-spec.md)
- [architecture-upgrade-design.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/architecture-upgrade-design.md)

## 2. 核心结论

### 2.1 这里只有“初始化”和“演进”，没有“旧系统迁移”

本项目当前阶段不写：

- 从旧表导入数据的迁移脚本
- 双写 / 双读兼容逻辑
- 老 API 到新 API 的过渡 schema

本项目要写的是：

- 新 PostgreSQL 数据库的初始化建表
- 后续 schema 演进 migration
- 本地 / CI / 测试环境 bootstrap

### 2.2 migration 必须独立于服务启动

禁止把 schema migration 作为服务启动的一部分自动执行。

原因：

- 多副本部署时会产生竞争
- 失败回滚边界不清楚
- 会把“建表问题”变成“服务不可用问题”

正确做法：

- 本地开发通过显式命令执行 migration
- CI 通过独立步骤执行 migration
- 生产环境通过独立 Job 或发布流水线执行 migration

### 2.3 统一使用 `golang-migrate`

数据库 schema 演进统一使用：

- `golang-migrate`

不接受以下方式作为主路径：

- 手工在数据库里执行 SQL
- 服务启动时偷偷建表
- 多套不一致的迁移工具并存

### 2.4 migration 以 schema 分域编写，以全局版本排序执行

为了同时满足：

- 与 [service-layout-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/service-layout-spec.md) 一致的分域管理
- `golang-migrate` 需要稳定线性顺序的执行要求

本项目采用：

- `migrations/<schema>/` 作为作者视角的分域目录
- 每个 migration 文件必须使用全局唯一版本号
- 正式执行入口必须先把多目录 migration 汇总成线性执行视图，再交给 `golang-migrate`

这样即使文件分布在不同 schema 目录，仍能按全局版本号明确执行顺序。

### 2.5 bootstrap 必须可重复、可自动化、可审计

环境引导流程必须满足：

- 同一套命令可反复执行
- 成功或失败边界明确
- 哪一步创建了什么资源可追踪

“先手工点一点，再补几条 SQL” 不属于可接受方案。

## 3. 目录与命名规范

## 3.1 migration 目录

推荐结构：

```text
migrations/
  tenant/
    000001_create_tenant_schema.up.sql
    000001_create_tenant_schema.down.sql
    000002_create_tenants.up.sql
    000002_create_tenants.down.sql
  file/
    000100_create_file_schema.up.sql
    000100_create_file_schema.down.sql
    000101_create_blob_objects.up.sql
    000101_create_blob_objects.down.sql
  upload/
  audit/
  infra/
```

规则：

- 版本号必须全局唯一
- 文件名必须表达 schema 和动作
- `up/down` 成对存在
- 单个 migration 只做一个清晰变更，不写巨型全量 SQL

## 3.1.1 执行视图

因为 `golang-migrate` 需要单线性 migration source，本项目要求保留一个统一执行视图。

推荐方式：

- 作者在 `migrations/<schema>/` 下维护 SQL
- `migrate runner` 扫描全部 schema 目录
- 校验版本号全局唯一、`up/down` 成对存在
- 按版本号排序后生成一个扁平执行目录，例如 `.build/migrations/all/`
- 再由 `golang-migrate` 对该扁平目录执行 `up/down/status`

规则：

- `.build/` 产物不提交 Git
- 本地、CI、发布流水线都必须复用同一个 runner
- 不允许有人绕过 runner，手工指定某个 schema 子目录直接执行

## 3.2 seed 目录

推荐结构：

```text
bootstrap/
  seed/
    dev/
    test/
```

规则：

- `dev` 放本地演示和联调用基础数据
- `test` 放自动化验证所需最小样本
- 不建立 `prod` 样例数据目录

## 3.3 本地引导脚本

推荐保留显式脚本入口，例如：

```text
scripts/
  bootstrap/
    local-up.sh
    migrate-up.sh
    seed-dev.sh
```

目标不是脚本形式本身，而是保证：

- 新成员能在最少人工步骤下拉起环境
- CI 能复用同一套引导逻辑

## 4. migration 编写原则

## 4.1 先建 schema，再建表，再建索引和约束

推荐顺序：

1. 创建 schema
2. 创建核心表
3. 创建外键与约束
4. 创建索引
5. `COMMENT ON`

这样能让失败点更明确，也更方便 review。

## 4.2 使用显式名称

必须显式写出：

- constraint name
- index name
- foreign key name

不要依赖 PostgreSQL 自动生成名字，否则后续排障和增量变更成本会变高。

## 4.3 避免把大改动塞进单个 migration

单个 migration 不应同时做：

- 多张表创建
- 大量索引
- 数据回填
- 约束重建

应拆成多个小步，保持每步可 review、可定位、可回退。

## 4.4 兼容性变更采用 expand-contract

后续进入持续迭代后，涉及破坏性变更时必须遵循：

1. 先扩展 schema
2. 再升级应用读写逻辑
3. 最后清理旧字段 / 旧索引 / 旧约束

禁止一步到位删列、改类型、改语义，迫使旧版本实例立即失效。

## 4.5 大表索引变更需要单独评审

对未来已成长为大表的对象，例如：

- `file.file_assets`
- `file.blob_objects`
- `upload.upload_sessions`
- `infra.outbox_events`

新增或调整索引时需要单独评审：

- 是否要 `CONCURRENTLY`
- 是否需要禁用事务执行
- 是否需要在低峰发布

第一阶段初始化建表可以先用普通索引；进入生产数据量阶段后必须重新评估。

## 5. 建表与执行顺序

## 5.1 初始化顺序

新环境 bootstrap 建议固定为：

1. 创建数据库
2. 创建 schema
3. 执行 `tenant` migration
4. 执行 `file` migration
5. 执行 `upload` migration
6. 执行 `audit` migration
7. 执行 `infra` migration
8. 执行 dev/test seed
9. 初始化 MinIO bucket 与必要前缀
10. 启动服务

这个顺序的原因：

- `tenant` 是租户主数据起点
- `file` 和 `upload` 依赖租户语义
- `audit`、`infra` 更偏横向支撑能力

## 5.2 schema 级依赖原则

依赖方向建议固定为：

- `tenant` 可被其他 schema 引用
- `file` 可依赖 `tenant`
- `upload` 可依赖 `tenant`，必要时引用 `file`
- `audit` 不反向依赖业务热路径表
- `infra` 作为基础设施支撑层，不承载业务强耦合外键

这样能降低后续 schema 演进时的连锁复杂度。

## 5.3 禁止应用启动自动补资源

生产环境禁止在服务启动时自动：

- 建表
- 改表
- 插入 seed
- 创建 bucket

开发环境如需自动创建 bucket，只能放在显式 bootstrap 命令里，不能藏进服务主流程。

## 6. `golang-migrate` 使用约束

## 6.1 版本号策略

推荐使用 6 位或更长的全局递增版本号。

例如：

- `000001`
- `000002`
- `000100`

要求：

- 版本号在整个仓库范围内唯一
- 不因 schema 目录不同而重复

## 6.2 执行入口

统一只保留一套正式入口，例如：

- `make migrate-up`
- `make migrate-down`
- `make migrate-status`
- `make migrate-build`

其底层可以调用：

- `golang-migrate` CLI
- `golang-migrate` library 封装的 bootstrap 程序

但仓库对外暴露的入口必须统一，避免团队成员各自运行不同命令。

执行入口必须包含以下步骤：

1. 扫描 `migrations/*`
2. 校验版本号、文件对称性和命名
3. 生成统一扁平执行目录
4. 再调用 `golang-migrate`

## 6.3 migration 状态表

数据库中必须保留 `golang-migrate` 的版本状态记录。

任何环境发现 migration 状态异常时，先修复 migration 状态，再继续发布，不允许跳过。

## 6.4 回滚策略

要求：

- 每个 migration 都要提供 `down`
- 但生产环境默认优先“向前修复”，不是盲目执行全量回滚

原因：

- 数据库回滚往往比应用回滚风险更高
- 某些 DDL 即使有 `down`，也不代表线上安全可逆

## 7. Seed 策略

## 7.1 生产环境不写演示数据

生产环境只允许：

- schema 初始化
- 必要系统级配置初始化

不允许自动写入：

- demo tenant
- demo file
- 测试管理员

## 7.2 开发环境 seed

`dev` seed 建议只提供最小可联调数据：

- 一个默认 tenant，例如 `demo`
- 对应 policy
- 少量基础 usage 记录

不要在 seed 中写入大量示例文件元数据，否则会污染本地问题定位。

## 7.3 测试环境 seed

自动化测试优先在测试代码里就地造数。

`bootstrap/seed/test` 只保留：

- 通用基础 tenant
- 必要固定枚举
- 极少量不可替代的公共样本

禁止把一整套庞大测试数据固化成共享 seed。

## 7.4 seed 必须幂等

允许重复执行的 seed 必须具备幂等性，例如：

- `INSERT ... ON CONFLICT DO UPDATE`
- 显式清理后再写入

不能依赖“只跑一次”的侥幸前提。

## 8. 本地开发 Bootstrap 流程

## 8.1 推荐顺序

本地开发环境推荐统一为以下步骤：

1. 拉起 PostgreSQL / Redis / MinIO
2. 创建数据库
3. 执行 migration
4. 执行 `dev` seed
5. 创建 bucket
6. 启动 `upload-service`
7. 启动 `access-service`
8. 启动 `admin-service`
9. 启动 `job-service`

## 8.2 启动校验

服务启动前至少验证：

- 数据库 schema 已完整存在
- bucket 已创建
- 关键 Secret / Config 已注入
- OpenAPI 与当前实现版本一致

## 8.3 本地重建

必须支持一键重建开发环境：

- 清理容器和数据卷
- 重新建库
- 重新执行 migration
- 重新 seed

否则本地环境很快会演变为不可复制的“个人手工环境”。

## 9. CI 与发布阶段 Bootstrap

## 9.1 CI

CI 中推荐顺序：

1. 启动 PostgreSQL / Redis / MinIO
2. 执行 migration
3. 执行测试专用 seed
4. 跑 integration / contract / e2e

CI 不应直接复用开发者本地 dump 数据。

## 9.2 发布前

发布前至少检查：

1. migration 已在目标环境执行成功
2. schema version 与应用版本要求一致
3. bucket 与对象存储权限正常
4. 关键索引已存在
5. rollback / forward-fix 预案已明确

## 9.3 发布顺序

推荐固定为：

1. 执行 migration
2. 校验 migration 状态
3. 再滚动发布各服务

不要反过来先发服务，再让新代码赌目标表已经存在。

## 10. 失败处理与排障原则

## 10.1 migration 失败

发生 migration 失败时：

1. 先确认失败版本号
2. 确认数据库当前实际对象状态
3. 评估执行 `down` 还是向前修复
4. 修复后再恢复发布流程

禁止直接手工补几条 SQL 然后假装 migration 已成功。

## 10.2 seed 失败

若 seed 失败：

- 优先检查幂等冲突和依赖顺序
- 不允许跳过错误继续启动服务

## 10.3 bucket 初始化失败

若对象存储 bucket 未创建成功：

- 开发 / CI 环境应直接失败
- 生产环境应阻断发布，不允许靠服务运行时兜底创建

## 11. Code Review 拦截项

看到以下实现应直接拦截：

- 服务启动时自动执行 schema migration
- migration 文件没有全局唯一版本号
- 同一个 migration 同时混入建表、回填、索引重建和 seed
- 生产环境 seed 写入 demo 数据
- 依赖人工手工执行 SQL 才能完成发布
- 大表索引变更没有说明执行策略

## 12. 最终建议

这份文档的核心只有五条：

1. 当前阶段不做旧系统迁移，只做新库初始化和增量演进
2. migration 必须独立于服务启动，并统一使用 `golang-migrate`
3. 目录按 schema 分域，但执行顺序必须由全局版本号固定
4. dev/test seed 可以有，prod seed 必须极度克制
5. bootstrap 必须能在本地、CI、发布流程中重复执行并稳定复现

如果不把这套 bootstrap 规则先写死，后面真正开始落表、接 CI、接 K8s 和发布时，返工会非常集中。
