# zhi-file-service-go 代码风格文档

## 1. 目标

这份文档不是为了统一空格，而是为了统一：

- 代码结构
- 命名习惯
- 错误处理
- 接口粒度
- 事务边界
- 日志与可观测性
- 测试方式

目标是让不同人写出来的 Go 代码在同一个项目里仍然像同一套系统。

## 2. 总体原则

### 2.1 清晰优先于炫技

优先选择：

- 简单
- 可读
- 可调试
- 可测试

不要为了“更抽象”或“更优雅”牺牲理解成本。

### 2.2 显式优先于隐式

优先选择：

- 明确参数
- 明确返回值
- 明确依赖
- 明确事务边界

不要依赖隐藏上下文、全局状态或魔法行为。

### 2.3 约束优先于便利

项目允许写得慢一点，但不允许边界混乱。

如果一个实现破坏了：

- 服务边界
- 依赖方向
- 状态机一致性

那它就不是合格代码。

## 3. 格式化与工具

### 3.1 强制工具

统一使用：

- `gofmt`
- `goimports`

可选但推荐：

- `golangci-lint`

### 3.2 格式准则

- 不手调对齐
- 不为了视觉整齐破坏 `gofmt`
- import 顺序交给工具

### 3.3 行长

不追求硬性 80 列，但一行明显过长时应主动拆分，优先保证可读性。

### 3.4 CI 基线

默认最小检查集：

- `gofmt -w`
- `goimports -w`
- `go test ./...`

建议在 CI 中增加：

- `golangci-lint run`
- `staticcheck ./...`

本项目不接受“本地能跑，格式没过，lint 先跳过”的提交习惯。

## 4. 命名规范

## 4.1 包名

规则：

- 全小写
- 简短
- 不带下划线
- 不带无意义后缀

正确示例：

- `upload`
- `access`
- `storage`
- `postgres`

错误示例：

- `uploadservice`
- `commonutil`
- `file_access`

## 4.2 类型名

规则：

- 使用清晰业务名词
- 避免 package stutter

例如在 `upload` 包里：

- 用 `Session`
- 不用 `UploadSessionEntity`

在 `storage` 包里：

- 用 `ObjectRef`
- 不用 `StorageObjectRefModel`

## 4.3 方法名

规则：

- 用动词开头
- 体现行为而不是技术细节

正确示例：

- `CreateSession`
- `CompleteUpload`
- `IssueAccessTicket`
- `ResolveObjectURL`

避免：

- `DoUpload`
- `HandleData`
- `ProcessInfo`

## 4.4 变量名

规则：

- 短作用域用短名
- 长作用域用全名

正确示例：

- `ctx`
- `tx`
- `fileID`
- `uploadSession`

避免：

- `fs`
- `tmp1`
- `dataObj`

## 5. 包结构落地模板

### 5.1 标准目录模板

每个业务服务（upload / access / admin / job）的内部结构必须遵循以下模板：

```
internal/services/<service>/
├── domain/                     # 领域层：纯业务规则，无外部依赖
│   ├── session.go              # 领域对象（聚合根 / 实体）
│   ├── session_status.go       # 值对象 / 枚举
│   ├── session_mode.go         # 值对象 / 枚举
│   ├── dedup.go                # 领域概念
│   └── errors.go               # 领域错误定义
│
├── ports/                      # 接口层：消费方定义的抽象契约
│   ├── session_repository.go   # 持久化接口
│   ├── storage_ports.go        # 存储接口（按能力拆分）
│   ├── outbox_publisher.go     # 事件发布接口
│   └── tenant_policy_reader.go # 外部策略读取接口
│
├── app/                        # 应用层：use case 编排
│   ├── commands/               # 写操作（产生状态变更）
│   │   ├── create_upload_session.go
│   │   ├── complete_upload_session.go
│   │   └── abort_upload_session.go
│   ├── queries/                # 读操作（只读，不变更状态）
│   │   ├── get_upload_session.go
│   │   └── list_uploaded_parts.go
│   └── view/                   # app 层输出视图模型（不等于 HTTP DTO）
│       └── upload_session.go
│
├── infra/                      # 基础设施层：接口的具体实现
│   ├── postgres/               # Postgres 适配器
│   │   └── repositories.go
│   ├── storage/                # 对象存储适配器
│   │   └── adapter.go
│   └── token/                  # Token 实现（如 HMAC issuer）
│       └── issuer.go
│
├── transport/
│   └── http/                   # HTTP 传输层
│       ├── handler.go          # 路由注册 + use case 调用
│       └── auth_dev.go         # 开发环境 auth 旁路
│
└── runtime/
    └── runtime.go              # 唯一 wiring 层，组装所有依赖
```

### 5.2 几个常见问题的答案

**`app` 里面按什么拆？**
按 CQRS 语义拆成 `commands/`（写）和 `queries/`（读），每个文件对应一个 use case。

**`domain` 按实体还是按聚合？**
按聚合根组织文件。一个聚合根一个主文件（如 `session.go`），值对象、枚举、错误各自独立文件。不要把所有 domain 对象塞进一个文件。

**`infra` 怎么分？**
按存储技术分子目录：`postgres/`、`redis/`、`storage/`。同一技术的多个 repository 可以合并到一个文件（如 `postgres/repositories.go`），也可以按聚合根拆分。

**`view/` 和 HTTP DTO 的区别？**
`view/` 是 app 层的输出模型，结构面向业务语义，不带 `json` tag。HTTP DTO（含 `json` tag）定义在 `transport/http/` 内部，由 transport 层负责转换。

### 5.3 一个文件一个主要关注点

不要把以下内容混在同一个文件：

- handler
- domain model
- repository SQL
- config

不要把以下内容混在同一个文件：

- handler
- domain model
- repository SQL
- config

### 5.2 文件命名

建议按用例或对象命名：

- `create_session.go`
- `complete_upload.go`
- `session_repository.go`
- `handler.go`

不要使用：

- `misc.go`
- `util.go`
- `helper.go`

除非它真的是极小且边界明确的辅助文件。

### 5.3 生成代码隔离

如果项目使用：

- `sqlc`
- OpenAPI codegen
- protobuf / grpc

生成代码必须与手写代码分开：

- 生成代码放 `internal/platform/generated/...` 或明确的 `gen/` 目录
- 不在生成代码里手改业务逻辑
- 对生成代码的封装写在 adapter 层

这样可以避免升级工具时把业务改动覆盖掉。

## 6. 包边界

### 6.1 `domain`

只能放：

- 领域对象
- 领域规则
- 状态机
- 纯函数规则

不能放：

- SQL
- HTTP DTO
- Redis key 组装
- S3 SDK 调用

### 6.2 `app`

负责：

- use case 编排
- 事务边界
- 调用顺序

不能放：

- HTTP 解析
- SQL 细节
- provider-specific SDK

### 6.3 `transport/http`

负责：

- request 解析
- response 输出
- 参数校验
- 错误码映射

不能放：

- 业务规则
- repository 调用
- 事务控制

### 6.4 `infra`

负责：

- 适配器实现
- Postgres / Redis / S3 接入

不能放：

- 跨服务共享的业务规则

### 6.5 依赖方向（强制）

各层之间的 import 只允许单向流动，不允许反向依赖：

```
┌─────────────────────────────────────────────────────┐
│                    transport/http                    │
│         （解析请求、输出响应、参数校验）              │
└────────────────────┬────────────────────────────────┘
                     │ import
                     ▼
┌─────────────────────────────────────────────────────┐
│                      app/                           │
│     （use case 编排、事务边界、调用顺序）             │
└──────────┬──────────────────────┬───────────────────┘
           │ import               │ import
           ▼                     ▼
┌──────────────────┐   ┌─────────────────────────────┐
│     domain/      │   │          ports/             │
│  （领域对象、     │   │  （repository / storage /   │
│   状态机、规则）  │   │   publisher 接口定义）       │
└──────────────────┘   └──────────────┬──────────────┘
                                      │ 实现（implement）
                                      ▼
                       ┌─────────────────────────────┐
                       │           infra/            │
                       │  （Postgres / Redis / S3    │
                       │   的具体适配器实现）          │
                       └─────────────────────────────┘

组装层（runtime/）：唯一允许 import 所有层的地方，负责依赖注入与 wiring。
```

**禁止的 import 方向：**

| 禁止 | 原因 |
|------|------|
| `domain` → `infra` / `transport` / `app` | 领域对象不能依赖外部技术细节 |
| `app` → `infra` | app 只能通过 ports 接口依赖，不能直接 import 实现 |
| `app` → `transport` | 业务逻辑不能依赖 HTTP 层 |
| `infra` → `app` / `transport` | 基础设施层不能感知上层业务 |
| `ports` → `infra` / `app` | 接口定义层保持纯粹，不能依赖实现 |

**验证方法：** 如果你在 `domain/` 里 import 了 `github.com/jackc/pgx` 或 `net/http`，依赖方向就已经被破坏了。

## 7. 接口规范

### 7.1 接口应由消费方定义

默认规则：

- 在 `ports/` 里定义接口
- 接口属于用它的服务，不属于提供者

### 7.2 接口必须小

推荐一个接口只承载一组高度相关行为。

正确示例：

```go
type SessionRepository interface {
    FindByID(ctx context.Context, id string) (*Session, error)
    Save(ctx context.Context, session *Session) error
    MarkCompleting(ctx context.Context, id string) (bool, error)
}
```

不推荐：

```go
type Repository interface {
    SaveAny(...)
    FindAny(...)
    UpdateAny(...)
    DeleteAny(...)
}
```

### 7.3 不要接口先行

如果只有一个实现、没有消费方抽象需求，就先写具体类型。

## 8. 函数规范

### 8.1 参数顺序

统一顺序：

1. `ctx context.Context`
2. 核心业务参数
3. 可选配置参数

### 8.2 返回值

规则：

- 优先 `(T, error)`
- 需要布尔语义时 `(bool, error)`
- 不返回无意义多元组

### 8.3 函数长度

经验规则：

- 20 到 40 行通常最容易维护
- 明显超过 60 行要考虑拆分

但不要为了“短”把一个连续业务流程拆得支离破碎。

### 8.4 构造函数与依赖注入

推荐模式：

- 通过 `NewXxx(...)` 显式注入依赖
- 构造时校验关键依赖不为 `nil`
- 依赖字段尽量使用最小接口

例如：

```go
func NewService(repo SessionRepository, storage Storage, clock Clock, logger *slog.Logger) *Service
```

避免：

- 在方法内部临时创建数据库连接
- 在业务对象里直接 `s3.NewFromConfig(...)`
- 依赖通过全局变量注入

## 9. 错误处理规范

### 9.0 错误体系（强制）

本项目使用统一的 `pkg/xerrors` 错误体系，**禁止**在各层自行发明错误类型。

```go
// xerrors.Error 是项目唯一的结构化错误类型
type Error struct {
    Code    Code           // 机器可读错误码，如 "UPLOAD_SESSION_NOT_FOUND"
    Message string         // 人类可读描述
    Details map[string]any // 结构化上下文（resourceId、field 等）
    Err     error          // 可选：包裹的原始错误
}
```

**领域错误定义在 `domain/errors.go`：**

```go
// ✅ 正确：通过 xerrors.New 定义，携带业务上下文
func ErrUploadSessionNotFound(id string) error {
    return xerrors.New(CodeUploadSessionNotFound, "upload session not found", xerrors.Details{
        "resourceType": "uploadSession",
        "resourceId":   id,
    })
}
```

**transport 层统一映射到 HTTP 状态码：**

```go
// ✅ 正确：用 xerrors.StatusOf / xerrors.CodeOf 完成映射，不手写 switch
status := xerrors.StatusOf(err)
code   := xerrors.CodeOf(err)
```

**禁止的错误写法：**

```go
// ❌ 裸字符串错误（无业务语义）
errors.New("something wrong")
fmt.Errorf("failed")

// ❌ 自定义错误结构（绕过 xerrors 体系）
type MyError struct { Msg string }

// ❌ sentinel 错误用于跨层判断
var ErrNotFound = errors.New("not found")
if err == ErrNotFound { ... }  // 应改用 xerrors.CodeOf
```

**`error.Is / error.As` 使用规范：**

- 跨层判断错误类型，优先使用 `xerrors.CodeOf(err)` 比对错误码
- `errors.Is` 用于判断哨兵错误（仅限标准库，如 `sql.ErrNoRows`）
- `errors.As` 用于提取 `*xerrors.Error` 中的 Details
- 不要用 `errors.Is` 比较自定义业务错误，改用错误码1 永远返回 error，不静默吞错

除非明确是后台兜底清理，否则不要忽略错误。

### 9.2 包装错误

统一使用：

```go
fmt.Errorf("complete upload session %s: %w", sessionID, err)
```

要求：

- 外层加业务上下文
- 保留底层错误链

### 9.3 错误分类

建议使用三层分类：

1. canonical sentinel error
2. 领域错误
3. transport 映射错误

### 9.4 不要重复记录同一个错误

规则：

- 谁决定吞掉错误，谁记录
- 谁只是向上返回，谁不要顺手再打日志

避免出现同一个错误在 4 层各打一遍。

### 9.5 错误定义位置

建议：

- 领域错误定义在 `domain/errors.go`
- use case 级错误定义在对应 `app` 包
- transport 层只做映射，不重新发明错误类型

命名建议：

- `ErrSessionNotFound`
- `ErrUploadAlreadyCompleted`
- `ErrTenantQuotaExceeded`

不要使用：

- `errors.New("something wrong")`
- `fmt.Errorf("failed")`

这种没有业务语义的裸错误作为边界错误。

## 10. context 规范

### 10.1 `context.Context` 必须向下传递

规则：

- 所有跨边界调用都要带 `ctx`
- repository / storage / redis / http client 都必须接收 `ctx`

### 10.2 不把业务参数塞进 `ctx`

允许通过 `context.Value` 传递的字段（仅限基础设施关注点）：

- `request_id`
- `trace_id`

**禁止**通过 `context.Value` 传递以下业务参数：

- `tenant_id`
- `user_id` / `subject_id`
- request DTO
- 任何业务实体

原因：`context.Value` 是弱类型、不可见依赖，业务参数必须显式传递，否则调用链追踪和测试都会变得困难。

```go
// ❌ 禁止：把 tenant_id 藏进 ctx
ctx = context.WithValue(ctx, "tenant_id", tenantID)
tenantID := ctx.Value("tenant_id").(string) // 隐式依赖，类型不安全

// ✅ 正确：显式传递业务参数
func (h *CreateUploadSessionHandler) Handle(ctx context.Context, cmd CreateUploadSessionCommand) (view.UploadSession, error)
// cmd 中携带 TenantID，清晰可见
```

### 10.3 超时控制

由调用边界决定：

- handler 设置 request timeout
- app 层不随意重新套 timeout
- infra 层尊重上游 `ctx`

## 11. 日志规范

### 11.1 统一结构化日志

日志必须是 key-value 风格。

至少包含：

- `service`
- `request_id`
- `tenant_id`
- `user_id`
- `upload_session_id`
- `file_id`
- `error`

### 11.2 日志级别

- `Debug`: 本地调试或细粒度链路信息
- `Info`: 正常关键业务事件
- `Warn`: 可恢复异常
- `Error`: 请求失败或后台任务失败

### 11.3 不记录大对象内容

禁止直接输出：

- 文件二进制
- 大块 JSON payload
- 全量 SQL 结果

### 11.4 字段名统一

日志和 trace 字段名尽量统一，避免同一含义出现多套命名：

- 用 `tenant_id`，不要一会儿 `tenantId` 一会儿 `tid`
- 用 `file_id`，不要一会儿 `fid` 一会儿 `asset_id`
- 用 `request_id`，不要再造 `traceId` 的业务别名

字段统一后，Grafana / Loki / Tempo 检索才不会失控。

### 11.5 可观测性闭环（tracing + metrics + logging）

三个信号必须协同，才能在生产环境有效排查问题：

**Tracing（OpenTelemetry）：**

- 所有入站 HTTP 请求必须创建 span，使用 `otelhttp` middleware 自动注入
- 关键业务操作（create session、complete upload、dedup 查询）需手动创建子 span
- `trace_id` 必须注入到日志字段，实现 log ↔ trace 关联

```go
// ✅ span 命名规范："service.operation"
ctx, span := tracer.Start(ctx, "upload.complete_session")
defer span.End()
```

**Metrics（Prometheus）：**

- 每个服务必须暴露以下基础指标：
  - `http_requests_total`（按 method、path、status 分组）
  - `http_request_duration_seconds`（latency histogram）
  - `upload_sessions_created_total`
  - `upload_sessions_completed_total`
  - `upload_sessions_failed_total`
- 后台 job 必须暴露：`job_executions_total`、`job_duration_seconds`

**三者字段对齐：**

| 概念 | 日志字段 | Trace attribute | Metrics label |
|------|----------|-----------------|---------------|
| 租户 | `tenant_id` | `tenant.id` | `tenant_id` |
| 请求 | `request_id` | `http.request_id` | — |
| 操作结果 | `error` | `otel.status_code` | `status` |

不要日志用 `tenantId`、trace 用 `tenant`、metrics 用 `tid`，三套命名让排查变成猜谜。

## 12. HTTP 规范

### 12.1 handler 要薄

handler 只做：

1. parse
2. validate
3. call app
4. map response

### 12.2 DTO 与领域对象分离

不要把 HTTP request struct 直接传给 domain 或 repository。

### 12.3 DTO / Model 转换策略

转换必须发生在明确的层边界，不允许跨层携带结构体。

**HTTP Request → domain：**

- 必须在 `transport/http/` 层完成，转换为 domain 参数或 app command
- 不允许把带 `json` tag 的 struct 传入 `app` 或 `domain`

```go
// ✅ 正确：transport 层完成转换
func (h *Handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
    var req createSessionRequest // HTTP DTO，有 json tag
    json.NewDecoder(r.Body).Decode(&req)

    cmd := commands.CreateUploadSessionCommand{ // app command，无 json tag
        FileName:    req.FileName,
        ContentType: req.ContentType,
        SizeBytes:   req.SizeBytes,
    }
    result, err := h.options.CreateUploadSession.Handle(ctx, cmd)
}
```

**domain → HTTP Response：**

- 必须在 `transport/http/` 层完成，将 domain 对象或 `view.*` 映射为 response struct
- `domain` 对象不能有 `json` tag
- `view.*` 是 app 层输出视图，可有少量 tag，但不面向 HTTP 细节

```go
// ❌ 禁止：domain 对象直接带 json tag
type Session struct {
    ID     string `json:"id"`   // 错误！domain 不应感知序列化
    Status string `json:"status"`
}

// ✅ 正确：transport 层定义 response struct
type sessionResponse struct {
    SessionID string `json:"sessionId"`
    Status    string `json:"status"`
}
```

**repository 返回类型：**

- repository 必须返回 domain object（如 `*domain.Session`），不返回数据库行 struct
- 数据库行到 domain object 的映射在 `infra/postgres/` 内部完成

```go
// ❌ 禁止：repository 返回 DB struct
func (r *SessionRepo) GetByID(ctx context.Context, id string) (*dbRow, error)

// ✅ 正确：repository 返回 domain object
func (r *SessionRepo) GetByID(ctx context.Context, tenantID, id string) (*domain.Session, error)
```

### 12.3 错误映射集中

建议统一在 transport 层维护：

- 领域错误 -> HTTP status
- 领域错误 -> `errorCode`

不要每个 handler 手写一套 `switch err`。

## 13. 数据库规范

### 13.1 SQL 优先显式

推荐：

- `sqlc + pgx`

原则：

- SQL 写清楚
- 索引命中路径清楚
- 锁语义清楚

### 13.2 repository 返回类型约定（强制）

repository 必须返回 **domain object**，不允许返回数据库行 struct 或 persistence model。

- 数据库行 → domain object 的映射在 `infra/postgres/` 内部完成
- app 层看到的永远是 `*domain.Session`，而不是 `*dbSession`

```go
// ❌ 禁止：暴露 DB struct
func (r *Repo) GetByID(ctx context.Context, id string) (*postgresRow, error)

// ✅ 正确：返回 domain object
func (r *Repo) GetByID(ctx context.Context, tenantID, id string) (*domain.Session, error)
```

### 13.3 repository 不拼业务

repository 负责：

- SQL 读写
- 数据映射（DB row → domain object）

repository 不负责：

- 状态机推进决策
- 鉴权
- 配额规则

### 13.4 事务边界在 app 层

规则：

- `app` 决定事务开始和结束
- repository 不偷偷开自己的业务事务

### 13.5 时间与删除语义

统一约束：

- 时间字段统一使用 `time.Time`
- 数据库存储统一使用 `timestamptz`
- 不使用字符串时间

删除语义默认分两类：

- 业务软删除：显式 `deleted_at`
- 物理清理：由后台任务执行

不要把

统一约束：

- 时间字段统一使用 `time.Time`
- 数据库存储统一使用 `timestamptz`
- 不使用字符串时间

删除语义默认分两类：

- 业务软删除：显式 `deleted_at`
- 物理清理：由后台任务执行

不要把“用户不可见”和“数据库已删除”混成一个概念。

## 14. 并发规范

### 14.1 不轻易开 goroutine

规则：

- 有明确收益再并发
- 并发必须有取消、超时、错误收敛

### 14.2 推荐的并发模式

**优先使用 `errgroup` 做 fan-out / 错误收敛：**

```go
// ✅ 正确：errgroup 自动处理取消和错误收敛
g, ctx := errgroup.WithContext(ctx)

g.Go(func() error {
    return doTaskA(ctx)
})
g.Go(func() error {
    return doTaskB(ctx)
})

if err := g.Wait(); err != nil {
    return err
}
```

**需要限制并发数时使用 worker pool：**

```go
// ✅ 正确：通过 semaphore 控制并发上限，防止资源耗尽
sem := make(chan struct{}, maxWorkers)
for _, item := range items {
    sem <- struct{}{}
    g.Go(func() error {
        defer func() { <-sem }()
        return process(ctx, item)
    })
}
```

**禁止的并发写法：**

```go
// ❌ 裸 goroutine，无错误收敛
go func() { doSomething() }()

// ❌ channel 无 close，导致 goroutine 泄漏
ch := make(chan Result)
go func() { ch <- compute() }()
// 如果调用方提前返回，goroutine 永远阻塞

// ❌ 忽略 goroutine 中的 panic
go func() {
    result := riskyOperation() // panic 会静默崩溃整个服务
}()
```

### 14.3 热路径优先简单同步模型

例如上传 complete，不要为了“更快”先写复杂并发逻辑。

### 14.3 共享状态必须显式保护

如果确实需要共享状态：

- `sync.Mutex`
- `sync.RWMutex`
- channel
- 原子操作

必须明确说明理由。

## 15. 注释规范

### 15.1 注释解释“为什么”，不是解释“是什么”

不要写：

```go
// Set the status to completed.
session.Status = StatusCompleted
```

应该写：

```go
// COMPLETING 到 COMPLETED 的推进必须和 file_id 落库在同一事务内完成，
// 否则重复 complete 会丢失幂等事实。
```

### 15.2 导出符号写 doc comment

所有导出类型、函数、接口都应有标准注释。

## 16. 测试规范

### 16.1 单测优先测规则

优先级：

1. `domain`
2. `app`
3. `transport`
4. `infra`

### 16.2 测试命名

推荐：

- `TestCreateSession_ReusesActiveSession`
- `TestCompleteUpload_ReturnsExistingFileIDWhenCompleted`

### 16.3 表驱动测试

对规则密集逻辑优先使用表驱动测试。

### 16.4 测试断言

断言要具体，不要只断言 `err == nil`。

### 16.5 集成测试

需要集成测试的对象：

- repository
- 事务型 use case
- 对象存储适配器

建议：

- 优先测真实 Postgres 行为
- 明确准备测试数据
- 明确清理策略

不要把大量 repository 行为都用 mock 假装验证通过。

### 16.6 Mock 策略

| 测试目标 | 是否 Mock | 说明 |
|----------|-----------|------|
| `domain` 单测 | ❌ 不 mock | 纯函数，直接测 |
| `app` 单测（依赖外部） | ✅ Mock ports 接口 | 用 mock 隔离 DB / 存储 |
| `infra/postgres` 集成测试 | ❌ 不 mock | 必须连真实 DB，测 SQL 行为 |
| `transport/http` 单测 | ✅ Mock use case | 只测解析 / 路由 / 错误映射 |
| `infra/storage` 集成测试 | ❌ 不 mock | 连真实 MinIO / S3 或 testcontainer |

**Mock 实现方式：**

- 优先手写 mock struct（实现 port 接口），不依赖 mockgen
- mock 结构体放在 `ports/` 或 `testkit/` 下，不散落各测试文件

## 17. 禁止事项

以下内容在 code review 中默认视为坏味道：

- 巨型 `common` / `utils` 包
- 全局单例到处传
- handler 直接写 SQL
- repository 直接做业务决策
- 领域对象 import S3 / Redis / HTTP 包
- 没有上下文信息的裸错误
- 同一个错误多层重复打日志
- 为了抽象而抽象的接口
- 滥用 `panic`
- 滥用 `init()`
- package 级可变全局状态
- 没有 `ctx` 的外部 IO 调用

## 18. 最终建议

这份代码风格的核心只有四条：

1. 边界清楚
2. 依赖显式
3. 错误可追踪
4. 代码能被别人快速改动

如果一段代码形式上符合 Go 语法，但破坏了这四条，它仍然是不合格代码。
