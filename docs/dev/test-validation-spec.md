# zhi-file-service-go 测试与验证规范文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的统一测试与验证规范。

它解决的问题：

1. 四个服务分别应该测什么，哪些测试必须做
2. 什么场景可以只跑单测，什么场景必须跑集成、契约、E2E 和压测
3. PostgreSQL / Redis / MinIO 这类真实依赖如何在本地和 CI 中稳定验证
4. 压测结果如何和 Prometheus / Grafana 观测体系联动，形成可复核证据

配套文档：

- [service-layout-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/service-layout-spec.md)
- [code-style-guide.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/code-style-guide.md)
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md)
- [access-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/access-service-implementation-spec.md)
- [admin-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-service-implementation-spec.md)
- [job-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/job-service-implementation-spec.md)
- [storage-abstraction-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/storage-abstraction-spec.md)
- [slo-observability-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/slo-observability-spec.md)

## 2. 核心结论

### 2.1 统一采用五层验证模型

本项目的验证层次固定为：

1. 单元测试
2. 集成测试
3. 契约测试
4. E2E 测试
5. 压测 / 稳定性验证

不接受只有单测或只有手工联调就合并的提交习惯。

### 2.2 规则逻辑优先单测，外部依赖优先真实集成

以下内容优先单测：

- 状态机
- 领域规则
- use case 编排分支
- 错误码映射

以下内容必须优先真实依赖集成验证：

- PostgreSQL repository
- 事务边界
- Redis 分布式锁
- MinIO / S3 适配器
- presign / multipart 行为

### 2.3 OpenAPI 是契约测试基准

所有 HTTP API 的 request / response / status / error code 都必须以 `api/openapi/*.yaml` 为准。

任何接口实现如果与 OpenAPI 不一致，视为缺陷，不视为“实现细节差异”。

### 2.4 热路径改动必须带压测证据

以下改动默认视为热路径改动：

- `upload-service` create session / complete upload
- `access-service` get file / resolve download / issue ticket
- `admin-service` 大列表查询、批量治理接口
- `job-service` cleanup / repair / reconcile 批处理
- PostgreSQL 索引、SQL、连接池、缓存、对象存储访问策略

这类变更在合并前必须补充压测结果与 Grafana 观测截图或指标摘要。

### 2.5 压测必须和观测联动

压测不是只看压测工具的 RPS。

压测期间必须同时观察：

- HTTP 吞吐
- p95 / p99 延迟
- 5xx 比例
- PostgreSQL 连接数与慢 SQL
- Redis 延迟与锁冲突
- MinIO / S3 请求延迟与错误率
- job backlog、锁等待、重试数

这些数据统一以 Prometheus 指标为准，通过 Grafana 面板聚合展示。

## 3. 测试层次定义

## 3.1 单元测试

用途：

- 验证纯领域规则和 use case 分支
- 快速定位状态机或参数校验错误
- 为重构提供最小回归网

范围：

- `internal/services/<service>/domain`
- `internal/services/<service>/app`
- 少量纯函数型 `pkg/`

要求：

- 紧贴代码放 `_test.go`
- 对规则密集逻辑优先使用表驱动
- 断言必须具体，不接受只断言 `err == nil`
- 单测不得依赖真实外部网络服务

典型对象：

- upload session 状态推进
- content hash 校验分支
- access ticket 签发与验签
- tenant policy 判断
- job retry / backoff 策略

## 3.2 集成测试

用途：

- 验证真实基础设施行为
- 验证 repository SQL、事务、一致性和 provider 行为
- 避免 mock 掩盖真实数据库和对象存储问题

范围：

- PostgreSQL repository
- `sqlc + pgx` 查询与事务
- Redis 分布式锁
- 对象存储 multipart / presign / finalize
- outbox 发布与幂等消费

要求：

- 统一放 `test/integration`
- 必须使用真实 PostgreSQL / Redis / MinIO
- 每个测试运行必须自带数据准备与清理
- 禁止依赖共享的人工维护测试环境

## 3.3 契约测试

用途：

- 验证实现与 OpenAPI、错误码注册表一致
- 防止 handler 演进后 response body、status code、字段名偷偷漂移

范围：

- `upload-service`
- `access-service`
- `admin-service`

要求：

- 统一放 `test/contract`
- 以 `api/openapi/*.yaml` 作为请求与响应断言基准
- 必须覆盖成功路径和主要失败路径
- 错误码必须对齐 [error-code-registry.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/error-code-registry.md)

推荐覆盖：

- 参数校验错误
- 资源不存在
- 权限不足
- 幂等重复请求
- 上传状态冲突

## 3.4 E2E 测试

用途：

- 验证跨服务联动、真实 HTTP、真实依赖与关键业务闭环
- 防止单服务局部正确但系统级链路断裂

范围：

- 上传后立即访问下载
- 管理员删除文件后访问链路失效
- job-service 清理后对象状态和元数据一致
- outbox 驱动的异步补偿闭环

要求：

- 统一放 `test/e2e`
- 至少启动涉及链路的真实服务进程
- 不允许把 E2E 退化成单个 handler 的黑盒测试

## 3.5 压测与稳定性验证

用途：

- 验证吞吐、延迟、资源消耗和扩缩容边界
- 验证热点 SQL、缓存、锁竞争和对象存储访问策略
- 验证长时间运行下的稳定性，而不是只看短时峰值

范围：

- `test/performance`
- 压测脚本、数据准备脚本、结果摘要模板

要求：

- HTTP 压测优先使用 `k6`
- 必须输出压测配置、并发、时长、样本量
- 必须同步记录 Grafana 仪表盘结果
- 关键结果要能复现，不接受口头结论

## 4. 测试目录与命名约定

推荐结构：

```text
internal/
  platform/
    testkit/
test/
  integration/
  contract/
  e2e/
  performance/
  fixtures/
```

约定：

- 单测仍紧贴业务代码
- `test/integration` 放真实依赖集成验证
- `test/contract` 放 OpenAPI 契约校验
- `test/e2e` 放跨服务闭环验证
- `test/performance` 放 k6 脚本与压测场景
- `internal/platform/testkit` 放容器启动、测试配置、fixture 装载等通用脚手架

推荐命名：

- `upload_complete_integration_test.go`
- `access_download_contract_test.go`
- `full_upload_then_download_e2e_test.go`
- `access_resolve_download.js`

## 5. 测试环境策略

## 5.1 本地与 CI 统一依赖模型

集成 / 契约 / E2E / 压测环境统一依赖：

- PostgreSQL
- Redis
- MinIO

本地和 CI 都必须尽量使用同一套依赖组合，避免“本地 sqlite、CI postgres、线上 pg”的偏差。

### 推荐方式

优先顺序：

1. `testcontainers-go`
2. `docker compose`

规则：

- 单个测试套件可独立启动依赖
- CI 不依赖共享长存活环境
- 所有环境参数显式注入，不依赖个人本地默认配置

## 5.2 数据隔离策略

每次测试运行必须有隔离边界。

至少做到：

- 独立数据库名或独立 schema
- 独立 Redis key 前缀
- 独立 MinIO bucket 或 object key 前缀
- 独立 tenant fixture

禁止：

- 多个测试任务共享一个固定 bucket 前缀
- 多个并发任务复用同一批 tenant 和 file_id
- 用“手工清库”维持测试可用

## 5.3 fixture 规则

`test/fixtures` 只放稳定公共样本：

- 小文件 / 大文件样本
- 多 MIME 类型样本
- OpenAPI response golden
- SQL 初始化片段

禁止把大量一次性、场景耦合严重的数据堆进公共 fixture。

## 5.4 时间与随机性控制

涉及以下场景时必须可控：

- token 过期
- session 超时
- cleanup TTL
- retry backoff
- time-based ULID 排序

要求：

- 通过 `pkg/clock` 或等价抽象注入时间
- 随机 ID 生成可在测试中替换为可预测实现

## 6. 各服务必测场景

## 6.1 `upload-service`

必须覆盖：

- create session 成功创建
- 已有活动 session 复用
- content hash 缺失或格式非法
- content hash 算法不支持
- 单文件大小、MIME、扩展名策略拦截
- multipart 分片签名
- complete 并发幂等
- dedup 命中 / 未命中
- finalize 成功但 DB 持久化失败后的补偿
- session 终态后重复 complete / abort

## 6.2 `access-service`

必须覆盖：

- public / private 两条访问路径
- 文件状态与访问级别校验
- ticket 签发、验签、过期、篡改
- 除 ticket redirect 外，数据面接口必须认证
- `PUBLIC` 文件匿名访问仅出现在最终 public URL 或 redirect 落点
- 下载 disposition 处理
- presign GET 集成测试
- 热路径读取不依赖跨服务 RPC

## 6.3 `admin-service`

必须覆盖：

- tenant 创建事务完整性
- policy 更新与审计日志同事务写入
- 删除文件幂等
- 删除文件后 usage / refcount 变更
- 管理后台分页、筛选、排序 SQL
- destructive operation 必填 `reason`

## 6.4 `job-service`

必须覆盖：

- 分布式锁 acquire / renew / release
- 锁过期后其他实例接管
- `FOR UPDATE SKIP LOCKED` 领取行为
- 重复调度不重复处理
- stuck completing 修复
- 物理删除幂等
- usage 对账修复

特别强调：

- 多实例 cleanup / reconcile / repair 必须验证分布式锁语义
- 仅验证 `SKIP LOCKED` 不算完成

## 6.5 共享模块

必须覆盖：

- storage abstraction provider 行为一致性
- outbox payload 编码与消费幂等
- 错误码到 HTTP status 的映射
- tracing / metrics 中间件基础行为

## 7. 变更类型与准入门禁

## 7.1 最小门禁

任何可合并变更至少满足：

1. `gofmt` / `goimports` 通过
2. 受影响包的单测通过
3. 若改动 repository / SQL / storage / lock，则集成测试通过
4. 若改动 HTTP API，则契约测试通过
5. 若改动跨服务链路，则 E2E 通过
6. 若改动热路径性能敏感逻辑，则压测结果更新

## 7.2 按改动类型划分

### 仅领域规则或纯函数变更

至少需要：

- 单元测试

### SQL / repository / transaction / storage 变更

至少需要：

- 单元测试
- 集成测试

### OpenAPI / handler / DTO / error code 变更

至少需要：

- 单元测试
- 契约测试
- 必要时补一条 E2E

### 跨服务联动、鉴权、状态机、删除补偿变更

至少需要：

- 单元测试
- 集成测试
- E2E 测试

### 热路径性能、缓存、锁、索引、对象存储策略变更

至少需要：

- 单元测试
- 集成测试
- 压测 / benchmark
- Grafana 指标证据

## 8. 压测规范

## 8.1 默认压测对象

第一阶段必须具备以下压测场景：

1. `upload-service` create session
2. `upload-service` complete upload
3. `access-service` get file metadata
4. `access-service` resolve download / issue ticket
5. `admin-service` list files / list tenants
6. `job-service` cleanup 扫描与锁竞争

## 8.2 场景类型

每个关键场景至少定义以下四类压测模型：

- `smoke`: 低并发快速校验
- `steady`: 稳态吞吐验证
- `spike`: 突发流量验证
- `soak`: 长稳运行验证

必要时增加：

- `contention`: 锁竞争
- `degraded-dependency`: 下游延迟放大

## 8.3 压测输出要求

每次压测至少输出：

- 代码版本 / commit
- 测试场景名
- 持续时间
- 虚拟用户数或并发数
- 成功率
- p50 / p95 / p99
- 吞吐
- 错误类型分布
- 压测期间资源曲线摘要

## 8.4 Grafana 观测要求

压测时至少打开以下 Grafana 面板：

- 服务总览
- HTTP 延迟与状态码分布
- PostgreSQL 连接池、TPS、慢 SQL
- Redis 命中率、延迟、错误率
- 对象存储请求延迟与失败率
- job lock 冲突、backlog、重试

建议把压测报告中的关键图表截图沉淀到变更记录或 PR 描述。

## 8.5 压测停止条件

出现以下情况应提前终止并判定为失败：

- 5xx 比例持续高于约定阈值
- p99 延迟明显突破 SLO 预算
- PostgreSQL 连接池耗尽
- Redis 锁续租失败持续升高
- MinIO / S3 错误率持续异常
- 大量 goroutine / 内存持续增长且不回落

## 9. CI 验证矩阵

推荐采用三层流水线：

### PR 快速校验

目标：

- 快速发现格式、编译、单测和轻量契约问题

建议内容：

- `gofmt` / `goimports`
- `go test ./...`
- 变更相关的轻量 contract test

### Merge 前完整校验

目标：

- 在进入 `main` 前阻断基础设施和系统链路回归

建议内容：

- 全量单元测试
- 全量集成测试
- 关键 E2E
- OpenAPI 校验

### Nightly / Pre-release 校验

目标：

- 运行耗时更长但更贴近线上风险的检查

建议内容：

- soak 压测
- contention 压测
- 大数据量列表查询验证
- job-service 长时间清理与修复任务

## 10. Code Review 拦截项

看到以下情况应直接拦截：

- 只补 handler happy path，没有补状态机或失败路径测试
- 用 mock 覆盖 PostgreSQL / Redis / MinIO 行为，替代真实集成验证
- OpenAPI 已修改但没有契约测试或实现同步
- 热路径 SQL 改动没有压测或指标证据
- 多实例 job 改动没有验证分布式锁接管语义
- 测试依赖共享环境和固定脏数据

## 11. 最终建议

这份文档的核心只有五条：

1. 规则逻辑优先单测，外部依赖优先真实集成
2. API 变更必须受 OpenAPI 契约约束
3. 跨服务链路必须有 E2E 闭环
4. 热路径变更必须带压测与 Grafana 证据
5. 分布式锁、多实例清理和一致性补偿必须按真实并发场景验证

如果没有这套验证规范，后续代码即使功能“看起来能用”，也很难保证吞吐、稳定性和多实例行为真的正确。
