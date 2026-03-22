# zhi-file-service-go 服务布局设计文档

## 1. 目标

本设计文档定义 `zhi-file-service-go` 的 monorepo 目录结构、包边界、依赖方向和测试布局。

它解决的问题不是“目录怎么好看”，而是以下四个工程约束：

1. 多个服务在同一仓库中如何长期演进而不相互污染
2. 哪些代码属于服务私有实现，哪些可以成为共享模块
3. 如何避免 Go 项目常见的“大而全 pkg”或“无边界 internal”问题
4. 后续真正开始写代码时，如何让目录结构直接反映架构边界

## 2. 核心结论

### 2.1 推荐采用“服务优先”的 monorepo 布局

不要用一套全局的：

- `internal/domain`
- `internal/app`
- `internal/infra`
- `internal/transport`

去承载所有服务。

原因：

- 这会让四个服务的领域代码重新耦合回一个大应用
- 文件会越写越胖，边界很快重新模糊

推荐改为：

- `cmd/<service>` 放服务入口
- `internal/services/<service>` 放服务私有代码
- `internal/platform` 放跨服务运行时基础设施
- `pkg/` 只保留极少数稳定共享包

### 2.2 `pkg/` 必须克制

`pkg/` 不是“任何公共代码都能扔进去”的地方。

只有满足以下条件的代码才可以进入 `pkg/`：

- 被两个及以上服务使用
- 语义稳定
- 不属于某一个具体业务服务
- 不依赖某个服务内部的领域对象

### 2.3 业务领域应当归属到服务

例如：

- upload session 状态机属于 `upload-service`
- 文件访问鉴权属于 `access-service`
- 租户治理属于 `admin-service`

即使多个服务都依赖某一类业务概念，也不代表它应该被抽到全局 `pkg/`。

## 3. 推荐目录结构

```text
zhi-file-service-go/
  Makefile
  .env.example
  .planning/
    platform/
      task_plan.md
      findings.md
      progress.md

  cmd/
    upload-service/
      main.go
    access-service/
      main.go
    admin-service/
      main.go
    job-service/
      main.go

  internal/
    platform/
      bootstrap/
      config/
      httpserver/
      middleware/
      observability/
      persistence/
      redis/
      testkit/

    services/
      upload/
        domain/
        app/
        ports/
        transport/http/
        infra/
      access/
        domain/
        app/
        ports/
        transport/http/
        infra/
      admin/
        domain/
        app/
        ports/
        transport/http/
        infra/
      job/
        app/
        ports/
        infra/

  pkg/
    storage/
    contracts/
    ids/
    clock/
    xerrors/
    client/

  api/
    openapi/

  migrations/
    tenant/
    file/
    upload/
    audit/
    infra/

  bootstrap/
    seed/
      dev/
      test/

  scripts/
    bootstrap/
    dev/
    test/
    tools/

  test/
    integration/
    contract/
    e2e/
    performance/
    fixtures/

  deployments/
    helm/
    kustomize/

  docs/
```

## 4. 顶层目录职责

## 4.1 `cmd/`

用途：

- 每个服务一个可执行入口

要求：

- `main.go` 尽量薄
- 只做启动配置、依赖装配、server 启停
- 不包含业务逻辑

示例：

```text
cmd/upload-service/main.go
cmd/access-service/main.go
```

## 4.2 `internal/platform/`

用途：

- 放跨服务共享的运行时基础设施

适合放这里的内容：

- 配置加载
- logger 初始化
- tracing / metrics 注册
- PostgreSQL 连接池
- Redis 客户端
- HTTP server 框架封装
- 统一 middleware
- 测试脚手架

不适合放这里的内容：

- 任何上传、访问、租户、管理员相关业务规则

## 4.3 `internal/services/`

用途：

- 每个服务自己的全部业务代码

这是整仓最重要的边界。

服务私有代码必须收敛在自己的目录下，避免跨服务直接引用内部实现。

## 4.4 `pkg/`

用途：

- 放非常克制、稳定、跨服务共享的公共包

推荐只保留：

- `pkg/storage`
- `pkg/contracts`
- `pkg/ids`
- `pkg/clock`
- `pkg/xerrors`
- `pkg/client`

## 4.5 `api/`

用途：

- OpenAPI 文档与生成产物

建议：

- 每个服务一个 OpenAPI 文件
- 如需生成 client，输出到 `pkg/client`

## 4.6 `migrations/`

用途：

- PostgreSQL schema migration

推荐按业务 schema 分目录，而不是按服务分目录。

原因：

- 表的归属是业务域，不是 HTTP 服务
- job-service 和 upload-service 可能共享 `upload` schema

## 4.7 `test/`

用途：

- 跨包、跨服务、跨进程测试

放这里的内容：

- 集成测试
- 契约测试
- E2E 测试
- 压测脚本
- 公共 fixture

## 4.8 `bootstrap/`

用途：

- 放 seed 数据和环境初始化所需静态样本

放这里的内容：

- `seed/dev`
- `seed/test`

不要放：

- 运行时配置 Secret
- 需要人工维护的大量脏数据

## 4.9 `scripts/`

用途：

- 放 `Makefile` 背后的底层执行脚本

放这里的内容：

- bootstrap 脚本
- 本地运行脚本
- 测试编排脚本
- 工具校验脚本

原则：

- `Makefile` 是统一入口
- `scripts/` 承载实现细节
- 不把业务逻辑写进脚本

## 4.10 `.planning/`

用途：

- 放模块级实施计划、review 发现和阶段进度

固定约束：

- 只允许按模块建目录，例如 `.planning/platform/`、`.planning/upload-service/`
- 每个目录固定使用 `task_plan.md`、`findings.md`、`progress.md`
- 不把实施计划散落到 `docs/`、仓库根目录或临时草稿文件

## 5. 服务内部结构模板

以 `upload-service` 为例：

```text
internal/services/upload/
  domain/
    upload_session.go
    upload_mode.go
    upload_status.go
    rules.go
  app/
    create_session.go
    complete_upload.go
    abort_upload.go
    get_progress.go
  ports/
    session_repository.go
    blob_repository.go
    tenant_policy_reader.go
    storage.go
    outbox.go
  transport/http/
    handler.go
    request.go
    response.go
    routes.go
  infra/
    postgres/
      session_repository.go
      blob_repository.go
    redis/
      dedup_claim_store.go
    outbox/
      publisher.go
```

### 5.1 `domain/`

职责：

- 核心领域模型
- 状态机
- 不变量校验
- 纯业务规则

要求：

- 不依赖 HTTP、SQL、Redis、S3 SDK
- 可以被 `app/` 直接调用

### 5.2 `app/`

职责：

- 组合 use case
- 定义事务边界
- 组织端口调用顺序

要求：

- 只依赖本服务的 `domain/`、`ports/` 和极少量 `pkg/`
- 不直接依赖具体实现

### 5.3 `ports/`

职责：

- 定义本服务需要的输入输出边界

包括：

- repository interfaces
- storage-facing interfaces
- ticket/policy interfaces
- publisher interfaces

要求：

- 接口归属于服务，而不是全局仓库
- 如果某个接口只服务 `upload-service`，就不该放到 `pkg/`

### 5.4 `transport/http/`

职责：

- HTTP handlers
- DTO
- 参数校验
- 错误映射
- 路由注册

要求：

- 不直接操作数据库
- 不直接拼业务状态机
- 只调用 `app/`

### 5.5 `infra/`

职责：

- 本服务自己的适配器实现

例如：

- PostgreSQL repository
- Redis cache / lock
- outbox publisher
- 后台 worker

要求：

- 具体实现只服务于本服务
- 如果未来另一个服务也需要相同能力，先评估是否应抽成 `internal/platform` 或 `pkg/`

## 6. 四个服务的边界建议

## 6.1 `upload-service`

服务目录：

```text
internal/services/upload/
```

拥有的业务：

- upload session
- multipart 协议
- single upload complete
- 秒传
- dedup claim
- 文件创建与删除的写路径

## 6.2 `access-service`

服务目录：

```text
internal/services/access/
```

拥有的业务：

- 文件访问鉴权
- access ticket
- public / private URL 解析
- access level 切换

## 6.3 `admin-service`

服务目录：

```text
internal/services/admin/
```

拥有的业务：

- tenant 管理
- 文件后台查询
- 管理员审计
- 配额调整

## 6.4 `job-service`

服务目录：

```text
internal/services/job/
```

拥有的业务：

- 过期 session 清理
- orphan object cleanup
- 对账任务
- outbox 投递

说明：

- `job-service` 通常不需要完整 `transport/http`
- 以 worker / scheduler 为主

## 7. `internal/platform` 与 `pkg` 的边界

### 7.1 放 `internal/platform` 的条件

满足以下任意条件，优先放 `internal/platform`：

- 只在本仓库内使用
- 是运行时基础设施
- 包含对框架、配置、连接池的封装
- 未来不打算暴露给外部引用

例如：

- DB 初始化
- HTTP server 启动器
- 统一中间件
- 测试容器工具

### 7.2 放 `pkg` 的条件

只有当以下条件同时满足，才放 `pkg`：

- 多个服务会直接 import
- 语义非常稳定
- 不属于某一服务专属业务
- 对外暴露没有歧义

例如：

- `pkg/storage`
- `pkg/ids`
- `pkg/clock`

### 7.3 不该放 `pkg` 的内容

以下内容默认不要放进 `pkg`：

- `UploadSession`
- `TenantPolicy` 领域规则
- `FileAccess` 业务判断
- repository 接口
- service-specific DTO

因为这些都是业务边界的一部分，不是基础库。

## 8. 依赖方向

## 8.1 允许的依赖

### `cmd/*`

允许依赖：

- `internal/platform/*`
- `internal/services/<service>/*`
- `pkg/*`

### `internal/services/<service>/transport`

允许依赖：

- `internal/services/<service>/app`
- `internal/services/<service>/domain`
- `pkg/contracts`
- `pkg/xerrors`

### `internal/services/<service>/app`

允许依赖：

- `internal/services/<service>/domain`
- `internal/services/<service>/ports`
- `pkg/*`

### `internal/services/<service>/infra`

允许依赖：

- `internal/services/<service>/ports`
- `internal/services/<service>/domain`
- `internal/platform/*`
- `pkg/*`

### `internal/services/<service>/domain`

允许依赖：

- 标准库
- 极少量无业务语义的 `pkg/*`

## 8.2 禁止的依赖

严禁：

- `upload-service` 直接 import `internal/services/access/*`
- `access-service` 直接 import `internal/services/admin/*`
- `domain/` import `infra/`
- `domain/` import `transport/`
- `pkg/` import `internal/services/*`

规则总结：

- 服务之间只能通过 API、client、消息或共享稳定契约交互
- 不能直接穿透对方内部实现

## 9. 跨服务共享的正确方式

如果两个服务都需要同一能力，按以下顺序判断：

1. 它是基础设施抽象吗
是则考虑 `pkg/` 或 `internal/platform`

2. 它是业务契约吗
是则放 `pkg/contracts` 或 `api/openapi`

3. 它是某个服务的内部业务逻辑吗
是则不要共享实现，应该通过 API 或重新抽象端口

## 10. 测试布局

## 10.1 单元测试

规则：

- 紧贴代码放 `_test.go`
- 优先测试 `domain/` 和 `app/`

示例：

```text
internal/services/upload/domain/upload_session_test.go
internal/services/upload/app/complete_upload_test.go
```

## 10.2 集成测试

规则：

- 需要真实 PostgreSQL / Redis / MinIO 的测试，统一放到 `test/integration`

示例：

```text
test/integration/upload_service_complete_test.go
test/integration/access_service_presign_test.go
```

## 10.3 契约测试

规则：

- 基于 OpenAPI 的 HTTP 契约测试放 `test/contract`

示例：

```text
test/contract/upload_create_session_contract_test.go
test/contract/admin_delete_file_contract_test.go
```

## 10.4 E2E 测试

规则：

- 跨服务、带真实 HTTP 的测试放 `test/e2e`

## 10.5 压测

规则：

- 压测脚本和场景定义放 `test/performance`

## 10.6 fixture

规则：

- 通用文件、SQL、JSON 样本放 `test/fixtures`

## 11. 配置与装配建议

### 11.1 每个服务单独配置

建议：

- `config/upload-service.yaml`
- `config/access-service.yaml`
- `config/admin-service.yaml`
- `config/job-service.yaml`

### 11.2 装配入口统一

每个 `cmd/<service>/main.go` 只做：

1. 加载配置
2. 初始化 platform 组件
3. 初始化 service adapters
4. 构建 app use cases
5. 启动 server 或 worker

不要在 `main.go` 里写任何业务逻辑。

## 12. 代码生成与契约管理

### 12.1 OpenAPI

建议：

- 每个服务一个 openapi 文件
- 生成代码放 `pkg/client/<service>`

### 12.2 SQL

建议：

- `sqlc` 输入放在服务自己的 `infra/postgres/sql/`
- 生成代码放回服务自己的 `infra/postgres/gen/`

不要把所有 SQL 全放到全局目录里。

## 13. 第一阶段最小骨架

如果现在就开始建仓，第一批目录至少要有：

```text
cmd/
  upload-service/
  access-service/
  admin-service/
  job-service/
.planning/
  platform/
    task_plan.md
    findings.md
    progress.md
internal/
  platform/
  services/
    upload/
    access/
    admin/
    job/
pkg/
  storage/
  ids/
  xerrors/
api/
  openapi/
migrations/
bootstrap/
scripts/
docs/
test/
  integration/
  contract/
  e2e/
  performance/
  fixtures/
```

这样已经足够开始第一批实现。

## 14. 最终建议

这份布局设计的核心不是目录树本身，而是以下三条规则：

1. 服务私有业务代码必须放在 `internal/services/<service>`
2. `pkg/` 必须克制，只保留稳定共享包
3. 服务之间禁止直接 import 对方内部实现

如果这三条守住了，monorepo 才会真正服务于架构，而不是把架构边界重新写散。
