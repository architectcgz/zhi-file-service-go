# zhi-file-service-go CODEX Prompt

这份文件是本仓库的约束性提示词。

任何 AI 代理、自动化代码助手、脚手架工具、协作者在本仓库内工作时，都必须先遵守这里的规则，再开始写代码、改结构、补文档或生成脚本。

目标只有一个：**防止 `zhi-file-service-go` 偏离已经确定的架构升级方向。**

## 1. 项目定位

你正在参与的是：

- `file-service` 的 Go 重写版
- 一次借重写完成的架构升级
- 不是旧系统表结构迁移项目
- 不是 Java 代码逐文件翻译项目

项目当前共识：

- 旧系统仍在测试阶段
- 可以完全重写 canonical schema
- 需要覆盖租户管理、管理员 API、上传与访问全链路
- 后续所有服务与调用方统一切到新 API

优先级排序固定为：

1. 性能与吞吐
2. 可维护性
3. 云原生伸缩与稳定性
4. 新 API 契约统一与 OpenAPI 化

如果某个实现同时满足“性能”和“API 清晰度”，优先选择两者都更好的方案。
不要为了照顾历史路径，把新的 canonical API 重新做成延续旧泥球的外壳。

## 2. 开工前必读

开始任何设计、编码、脚本生成、目录调整之前，至少先读取以下文档：

1. `docs/README.md`
2. `docs/architecture-upgrade-design.md`
3. `docs/architecture-style-decision.md`
4. `docs/api-design-spec.md`
5. `docs/openapi-contract-spec.md`
6. `docs/data-plane-auth-context-spec.md`
7. `docs/error-code-registry.md`
8. `docs/configuration-registry-spec.md`
9. `docs/data-model-spec.md`
10. `docs/data-protection-recovery-spec.md`
11. `docs/service-layout-spec.md`
12. `docs/storage-abstraction-spec.md`
13. `docs/upload-session-state-machine-spec.md`
14. `docs/code-style-guide.md`
15. `docs/migration-bootstrap-spec.md`
16. `docs/test-validation-spec.md`
17. `docs/deployment-runtime-spec.md`
18. `docs/development-workflow-spec.md`

实施计划文件统一放在仓库根目录 `.planning/`：

- 每个模块一个目录
- 每个目录固定使用 `task_plan.md`、`findings.md`、`progress.md`
- 不要把实施计划重新写回 `docs/`、仓库根目录或任意临时文件

如果改动落在具体服务内，还必须额外读取对应文档：

- `upload-service`: `docs/upload-service-implementation-spec.md`、`docs/upload-integrity-hash-spec.md`、`docs/outbox-event-spec.md`
- `access-service`: `docs/access-service-implementation-spec.md`
- `admin-service`: `docs/admin-service-implementation-spec.md`、`docs/admin-auth-spec.md`、`docs/outbox-event-spec.md`
- `job-service`: `docs/job-service-implementation-spec.md`、`docs/outbox-event-spec.md`

如果你的改动会影响：

- 服务拆分
- 表结构
- API 设计契约
- OpenAPI 正式契约
- 存储抽象
- 配置注册表
- 数据保护与恢复策略
- 代码风格约束
- 本地开发启动方式
- Makefile / scripts / CI 命令面

那么你必须同步更新相应文档，不能只改代码不改设计。

## 3. 不可偏离的架构结论

本项目固定采用：

- `Clean-ish + DDD-lite + CQRS-lite`

含义是：

- 保持清晰依赖方向
- 只在真正需要的地方做领域建模
- 只做轻量读写分离

不是以下任何一种：

- 教科书式重型 `Clean Architecture`
- 重型 DDD
- 重型 CQRS
- Event Sourcing
- 为抽象而抽象

如果你发现某个实现开始引入大量：

- command bus / query bus
- event sourcing
- 全局 generic repository
- 到处都是 interface + factory + specification
- “所有东西都拆成微服务”

那么你正在把项目带偏，必须停止并回到现有设计。

## 4. 固定服务边界

第一阶段服务边界已经确定，不要随意重划：

- `upload-service`
- `access-service`
- `admin-service`
- `job-service`

职责原则：

- `upload-service` 负责上传写路径、上传会话、分片、去重、complete/abort
- `access-service` 负责访问鉴权、票据、预签名、302 跳转、读路径优化
- `admin-service` 负责管理员 API、租户管理、治理能力、审计入口
- `job-service` 负责清理、修复、补偿、对账、后台任务

当前阶段明确不做：

- 独立 `storage-service`
- 独立 `metadata-service`
- 独立 `tenant-service`

原因很简单：

- 热路径不能多一跳
- 强事务边界不能过早 RPC 化
- 控制面低频能力拆太细没有收益

## 5. 固定数据架构

数据层当前共识不可随意改动：

- 使用一个 PostgreSQL 集群
- 一个业务数据库
- 多个逻辑 schema 分域

推荐 schema：

- `tenant`
- `file`
- `upload`
- `audit`
- `infra`

不要在第一阶段把数据库拆成：

- 每个服务一个独立数据库
- 上传、文件、租户各自远程调用
- 分布式事务 + 补偿驱动的核心链路

### 5.1 核心模型不可混淆

必须严格区分：

- `blob_objects`: 物理对象
- `file_assets`: 业务可见文件
- `upload_sessions`: 上传状态机
- `tenant_policies`: 配额与规则
- `tenant_usage`: 聚合统计

不要把这些概念重新揉成一张“大表”或一个“大对象”。

### 5.2 上传状态机是核心约束

`UploadSession` 不是普通 DTO，而是核心业务状态机。

任何上传相关实现都必须尊重：

- init
- 上传中
- completing
- completed
- aborted / failed / expired

状态推进、一致性、幂等判断不能散落在 handler、repository、脚本里。

## 6. 固定目录与依赖方向

monorepo 采用“服务优先”布局：

```text
cmd/<service>
internal/platform
internal/services/<service>
pkg/
api/
migrations/
docs/
```

禁止把所有业务重新堆回这种全局布局：

- `internal/domain`
- `internal/app`
- `internal/infra`
- `internal/transport`

原因：

- 这会把多服务重新写回一个大应用
- 会让边界在几周内再次失效

### 6.1 分层规则

依赖方向固定为：

```text
transport/http -> app -> domain
                   |
                 ports
                   |
                 infra
```

必须遵守：

- `domain` 不依赖 HTTP、SQL、Redis、S3 SDK
- `app` 负责 use case 编排和事务边界
- `transport/http` 负责协议适配、参数校验、错误映射
- `infra` 负责 Postgres / Redis / S3 等适配实现

## 7. 共享模块规则

`pkg/` 必须非常克制。

只有满足以下条件才允许进入 `pkg/`：

- 被两个及以上服务复用
- 语义稳定
- 不属于单一业务服务
- 不依赖某个服务内部领域模型

适合的典型内容：

- `storage`
- `ids`
- `clock`
- `xerrors`
- 稳定 `contracts`
- 跨服务 client

不允许把以下内容丢进 `pkg/`：

- 上传状态机
- 文件访问鉴权规则
- 租户治理逻辑
- “先丢进去以后再整理”的通用工具垃圾场

## 8. 编码规则

实现代码时必须遵守 `docs/code-style-guide.md`，至少包含以下硬约束：

- handler 必须薄
- 事务边界在 `app` 层
- repository 不做业务决策
- 领域对象不 import S3 / Redis / HTTP 包
- 所有跨边界调用都显式传 `ctx`
- 不把 `tenant_id` / `user_id` 等业务参数塞进 `ctx`
- 使用结构化日志
- 错误必须保留上下文
- 不重复打印同一个错误
- 不滥用 goroutine
- 不引入巨型 `common` / `utils` 包

推荐技术取向：

- `sqlc + pgx`
- `gofmt`
- `goimports`
- `golangci-lint`
- `staticcheck`

## 9. 观测性与运行约束

所有服务默认面向云原生部署设计。

必须具备：

- 无状态副本
- `GET /ready`
- `GET /live`
- `GET /metrics`
- trace
- structured log
- 配置外置化

日志字段尽量统一：

- `service`
- `request_id`
- `tenant_id`
- `user_id`
- `upload_session_id`
- `file_id`
- `error`

不要为了快速落地牺牲可观测性，否则后续 Grafana / Loki / Tempo 会迅速失控。

对 `job-service` 额外强制要求：

- 多实例 cleanup / repair / reconcile 任务必须使用分布式锁
- `FOR UPDATE SKIP LOCKED` 只能用于任务领取，不能替代分布式锁

## 10. API 契约约束

本项目对外目标是定义并收敛到一套新的 canonical API。

这意味着：

- 新 API 才是外部唯一契约
- 不再以旧 `file-service` 路径作为设计边界
- 后续完整字段级文档以 OpenAPI 为准
- 内部表结构、领域模型、服务边界都可以围绕新 API 重构

必须遵守：

- 数据面路径统一在 `/api/v1`
- 控制面路径统一在 `/api/admin/v1`
- 上传入口统一围绕 `upload-sessions`
- 除 `GET /api/v1/access-tickets/{ticket}/redirect` 外，数据面 north-south API 统一要求 Bearer Token
- `PUBLIC` 文件的匿名访问只发生在最终 public URL 或 ticket redirect 落点，不发生在 `/api/v1/files/*`
- 不长期维护 old/new 两套 north-south API
- 不新增未经批准的 legacy alias 路径

## 11. 禁止事项

以下做法默认视为偏航：

- 为了微服务而微服务
- 为了 DDD 而 DDD
- 为了 CQRS 而 CQRS
- 为了抽象而抽象
- 为了照顾历史路径而保留旧泥球结构
- 在热路径引入额外同步 RPC
- 在 repository 里推进状态机
- 在 handler 里拼业务
- 在 domain 里操作数据库或 SDK
- 在 `pkg/` 里堆业务逻辑
- 使用全局 generic repository 抹平业务差异
- 没有文档更新的架构调整

## 12. 发生冲突时怎么做

如果你收到的新需求与现有设计冲突：

1. 不要直接按需求硬改代码
2. 先明确指出冲突点
3. 说明为什么它会让项目偏离当前架构
4. 给出最小偏移方案
5. 如确实要改方向，先改文档再改代码

禁止“先写出来再说”。

## 13. 每次交付前自检

在声称“完成”之前，至少自检以下问题：

1. 改动属于哪个服务边界，是否放对目录
2. 是否破坏了 `Clean-ish + DDD-lite + CQRS-lite`
3. 是否把业务规则塞进了错误层次
4. 是否破坏了 `blob_objects / file_assets / upload_sessions` 的分离
5. 是否引入了不必要的网络跳数
6. 是否保持了热路径简单、少跳数、易扩缩容
7. 是否同步更新了受影响文档
8. 是否符合 `docs/code-style-guide.md`

## 14. 一句话原则

如果一个实现让系统变得：

- 更快
- 更清楚
- 更稳
- 更容易被多人长期维护

它大概率是对的。

如果一个实现让系统变得：

- 更绕
- 更重
- 更隐式
- 更依赖“只有作者自己懂”

它大概率就是错的。
