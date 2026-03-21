# zhi-file-service-go 架构升级设计文档

## 1. 背景

现有 `file-service` 已经承载了以下核心能力：

- 多租户隔离
- 普通上传、直传、分片上传、上传会话
- 文件访问鉴权与跳转
- 文件去重与引用计数
- 双 bucket 策略
- 管理员 API 与租户管理

从当前代码和压测结果看，系统的主要问题不是功能缺失，而是以下三类约束开始变得明显：

1. 热路径职责过多，上传、访问、管理能力耦合在同一服务内
2. 大文件与混合流量场景下，multipart 链路更容易成为性能和稳定性瓶颈
3. 现有 Java 服务虽然已经做了分层，但在面向高吞吐、云原生弹性和后续多团队协作时，服务边界仍然偏粗

本设计不是单纯的语言迁移，而是借 Go 重写完成一次架构升级。

## 2. 目标

### 2.1 核心目标

- 在保持现有外部 API 兼容的前提下，重构为更清晰的多服务架构
- 提升上传和访问链路的吞吐能力，优先优化数据面
- 提升系统可维护性，使领域边界、代码边界、部署边界一致
- 提升云原生适配能力，支持独立扩缩容、故障隔离和阶段化落地

### 2.2 设计目标排序

1. 性能与吞吐
2. 可维护性
3. 云原生伸缩与稳定性
4. 全量兼容现有功能和接口语义

### 2.3 非目标

- 第一阶段不追求把所有能力拆成细粒度微服务
- 第一阶段不把对象存储能力做成独立 RPC 存储服务
- 第一阶段不强制引入 Kafka 这类重型基础设施

## 3. 设计原则

### 3.1 数据面与控制面分离

高频、高吞吐、低延迟的上传与访问链路属于数据面，应独立部署和扩容。租户管理、文件治理、后台查询属于控制面，应和数据面隔离。

### 3.2 热路径少跳数

上传和访问链路必须尽量减少跨服务同步调用。Go 重写后不应为了“微服务化”把热路径拆成多跳 RPC。

### 3.3 共享模块优先于过早拆服务

对象存储适配、元数据访问、租户策略、鉴权规则更适合作为共享模块存在，而不是第一天就拆成独立网络服务。

### 3.4 兼容优先，内部可重构

外部 API 路径、请求头、错误码语义尽量保持兼容。内部模型、表结构和服务拆分可以按 Go 版目标重新设计，不以旧实现为边界。

### 3.5 云原生友好

所有服务默认面向容器化和 Kubernetes 部署设计，具备：

- 无状态副本
- readiness / liveness
- metrics / trace / structured log
- 水平扩缩容
- 配置外置化

## 4. 推荐架构

推荐采用“单仓库、多服务、共享底层模块”的设计。

### 4.1 服务划分

#### A. zhi-file-upload-service

职责：

- 普通上传
- 图片上传
- 直传初始化
- 上传会话管理
- multipart init / part / progress / complete / abort
- 秒传与去重判定
- 文件落库与引用计数更新

说明：

- 这是写入数据面服务
- 负责最重的对象写入与元数据写入链路
- 后续如需要，可进一步拆出专门的 `upload-session` 子服务，但第一阶段不建议

#### B. zhi-file-access-service

职责：

- 文件访问鉴权
- 公有文件直出
- 私有文件预签名 URL 签发
- 访问票据校验
- 302 跳转
- 可选的防盗链、短时票据、下载限流

说明：

- 这是读取数据面服务
- 本质上是现有 `file-gateway-service` 的升级版 Go 实现
- 访问链路必须保持轻量，避免与上传链路耦合

#### C. zhi-file-admin-service

职责：

- 管理员 API
- 文件管理与查询
- 租户管理
- 租户配额调整
- 租户状态变更
- 审计记录

说明：

- 这是控制面服务
- 第一阶段将 `tenant-service` 合并进 `admin-service`
- 原因是租户管理流量低、强治理属性明显，单独拆分收益低于复杂度成本

#### D. zhi-file-job-service

职责：

- 过期 upload session 清理
- 孤儿对象清理
- 引用计数修复
- 配额对账
- 异步补偿
- 可选的数据修复任务

说明：

- 离线任务和在线请求必须分离
- 避免后台任务影响上传和访问流量

### 4.2 为什么不推荐第一阶段拆出的服务

#### 不建议单独拆 `storage-service`

对象存储适配属于基础设施能力。若做成网络服务，会导致上传、访问链路都多一跳，热路径性能会下降，故障面也会扩大。

#### 不建议单独拆 `metadata-service`

PostgreSQL 访问是强事务边界的一部分。若第一阶段强行通过 RPC 包装元数据访问，会增加分布式事务问题。

#### 不建议单独拆 `tenant-service`

租户策略属于低频控制面。独立拆分会引入更多同步依赖，但对性能提升没有直接收益。

## 5. 共享模块设计

四个服务共享同一个 monorepo 中的基础模块。

### 5.1 推荐目录结构

```text
zhi-file-service-go/
  cmd/
    upload-service/
    access-service/
    admin-service/
    job-service/
  internal/
    platform/
      bootstrap/
      config/
      httpserver/
      middleware/
      observability/
      persistence/
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
  deployments/
    helm/
    kustomize/
  docs/
```

### 5.2 模块边界

#### `internal/platform`

跨服务但不对外暴露的运行时基础设施：

- 配置加载
- HTTP server 启停
- PostgreSQL / Redis 连接管理
- 统一 middleware
- metrics / tracing / logging
- 测试夹具与 testkit

要求：

- 不承载具体业务领域规则
- 允许被所有服务内部代码引用

#### `internal/services/<service>/domain`

核心领域对象：

- Tenant
- TenantQuota
- FileAsset
- StorageObject
- UploadSession
- MultipartTask
- AccessTicket

要求：

- 只包含领域模型、领域规则、状态机
- 不依赖 HTTP、DB、S3 SDK

#### `internal/services/<service>/app`

应用服务：

- UploadApp
- AccessApp
- TenantAdminApp
- FileAdminApp
- CleanupApp

职责：

- 编排领域对象与端口调用
- 定义事务边界
- 暴露清晰 use case

#### `internal/services/<service>/ports`

服务自己的输入 / 输出端口：

- repository ports
- storage-facing ports
- ticket / auth / policy ports

要求：

- 只定义接口，不写实现
- 归属于服务，不归属于全局 `pkg`

#### `internal/services/<service>/transport`

协议适配层：

- HTTP handlers
- request / response DTO
- 参数校验
- 错误映射

#### `internal/services/<service>/infra`

本服务私有的适配器实现：

- PostgreSQL repository
- Redis adapter
- outbox publisher
- background worker

#### `pkg/storage`

对象存储抽象：

- MinIO / S3 适配
- put / head / delete
- multipart create / upload part / list parts / complete / abort
- presign put / get

说明：

- 这是少数允许放进 `pkg` 的共享能力
- 因为它是稳定、跨服务、偏基础设施的抽象

#### `pkg/contracts`

放跨服务稳定契约：

- OpenAPI 生成类型
- 共享枚举
- 内部 client 约定结构

#### `pkg/ids` / `pkg/clock` / `pkg/xerrors`

放足够小且稳定的通用组件：

- ID 生成
- 时钟抽象
- canonical error helpers

## 6. 数据模型设计

Go 版应尽量兼容现有核心数据语义，但允许做结构升级。

### 6.1 核心表

建议保留以下核心概念：

- `tenants`
- `tenant_usage`
- `storage_objects`
- `file_assets` 或 `file_records`
- `upload_sessions`
- `upload_session_parts`
- `admin_audit_logs`

### 6.2 关键建模原则

#### StorageObject 与 FileAsset 分离

- `storage_objects` 表示物理对象
- `file_assets` 表示业务侧文件引用
- 同租户内可基于哈希复用同一物理对象

#### UploadSession 独立建模

所有上传方式都收口为统一 `upload_session` 状态机：

- INITIATED
- UPLOADING
- COMPLETING
- COMPLETED
- ABORTED
- EXPIRED
- FAILED

这样普通上传、单对象直传、multipart 直传、服务端中转上传都能在同一抽象下被治理。

#### 租户配额单独维护

租户配置和租户使用量不能混在同一对象内：

- `tenants`: 配置、状态、限制
- `tenant_usage`: 已用大小、已用文件数、更新时间

#### 审计日志单独建表

管理员 API、租户变更、人工删除文件等操作建议独立审计，避免在主表中混杂行为日志。

## 7. API 兼容策略

本项目目标是全量兼容，不是重新发明一套外部协议。

### 7.1 兼容范围

以下内容默认保持兼容：

- API 路径
- `X-App-Id` 请求头语义
- 主要请求 / 响应结构
- 主要错误码语义
- 管理员 API Key 认证模式
- 文件访问跳转语义

### 7.2 兼容原则

#### 原路径兼容

保留现有主要入口：

- `/api/v1/upload/*`
- `/api/v1/multipart/*`
- `/api/v1/direct-upload/*`
- `/api/v1/upload-sessions*`
- `/api/v1/files/*`
- `/api/v1/admin/*`

#### 内部统一，外部兼容

外部可以保留多套 legacy API，但内部统一映射到同一套 UploadSession / FileAccess / TenantPolicy 模型。

#### 错误码兼容

保持当前双层错误表达：

- HTTP status
- `errorCode`

这样可以降低客户端改造成本。

## 8. 关键链路设计

### 8.1 上传链路

#### 小文件代理上传

1. 请求进入 `upload-service`
2. 校验身份、租户、配额、文件类型
3. 计算哈希并执行秒传 / 去重判定
4. 未命中则上传对象存储
5. 写入 `storage_object` 和 `file_asset`
6. 更新 `tenant_usage`
7. 返回 fileId 和访问信息

适用：

- 后台管理
- 低并发
- 必须同步处理服务端逻辑

#### 大文件直传 / multipart

1. 客户端请求 `upload-session`
2. 服务创建 `upload_session`
3. 服务初始化 multipart upload
4. 服务签发 part URLs 或返回上传上下文
5. 客户端直传对象存储
6. 客户端调用 `complete`
7. 服务校验 authoritative parts
8. 完成 multipart upload
9. 落元数据并更新配额

设计重点：

- 热数据尽量不穿过 Go 服务进程
- 服务负责控制面和元数据，不负责搬运大流量二进制

### 8.2 访问链路

1. 请求进入 `access-service`
2. 校验用户、租户、文件归属、访问权限
3. PUBLIC 文件直接返回公共对象地址或 302
4. PRIVATE 文件生成短时 presigned URL 或 access ticket
5. 返回 302 跳转

设计重点：

- access-service 默认无状态
- 不读取大对象内容
- 重点优化鉴权、票据和跳转

### 8.3 管理员链路

1. 请求进入 `admin-service`
2. 校验 `X-Admin-Api-Key`
3. 查询或修改租户 / 文件 / 配额
4. 写入 audit log

设计重点：

- 强调权限与可审计性
- 和数据面彻底隔离

## 9. 存储与事务设计

### 9.1 事务边界

第一阶段仍以 PostgreSQL 本地事务为主，不引入分布式事务框架。

事务原则：

- 元数据写入在单库事务内完成
- 对象存储操作与数据库写入之间采用“最终一致 + 补偿”思路
- complete / abort 这类操作必须具备幂等性

### 9.2 幂等性

必须重点处理以下幂等问题：

- 重复创建 upload session
- 重复 complete
- 重复 abort
- 重复删除文件
- 秒传竞争
- 同哈希并发上传竞争

建议手段：

- 唯一索引
- 幂等键
- 行级锁或 advisory lock
- 明确状态机转移条件

### 9.3 异步一致性

建议第一阶段采用“事务表 + 后台 job 补偿”的轻量模式：

- 在主事务中记录需要补偿或异步处理的事件
- `job-service` 负责扫描并处理

这样能先控制复杂度，后续如果事件量增大，再引入 NATS JetStream 或 Kafka。

## 10. 性能设计

### 10.1 上传性能

- 默认优先直传，不让服务中转大文件字节流
- multipart part size 可配置
- 上传 complete 阶段只做必要元数据落库
- 去重判定使用哈希索引与最小查询路径
- 大文件链路避免长事务

### 10.2 访问性能

- access-service 无状态化
- 权限判断尽量本地完成，避免多跳 RPC
- 公有文件优先走 CDN / 公共 bucket
- 私有文件使用短 TTL 预签名 URL，减小服务压力

### 10.3 数据库性能

- 以 PostgreSQL 为主库
- 高热点查询建立组合索引
- tenant_usage 更新避免全表扫描
- 列表查询使用分页与条件索引

### 10.4 Redis 使用建议

Redis 建议只承担以下职责：

- access ticket / 短期会话缓存
- 幂等锁
- 热点元数据短缓存

不要让 Redis 成为文件元数据的唯一来源。

## 11. 云原生部署设计

### 11.1 部署形态

推荐 Kubernetes 部署：

- `upload-service` 独立 HPA
- `access-service` 独立 HPA
- `admin-service` 低副本固定部署
- `job-service` 单副本或 leader election

### 11.2 伸缩策略

#### upload-service

根据以下指标扩缩容：

- CPU
- 请求耗时
- in-flight requests
- multipart complete 延迟

#### access-service

根据以下指标扩缩容：

- RPS
- p95 latency
- 302 / 403 / 5xx 比例

### 11.3 稳定性要求

每个服务都应具备：

- readiness probe
- liveness probe
- graceful shutdown
- config hot reload 或滚动更新
- connection pool 上限
- 请求超时和下游超时控制

## 12. 可观测性设计

### 12.1 Metrics

每个服务必须输出：

- HTTP RPS
- HTTP latency p50 / p95 / p99
- error rate
- object storage request latency
- PostgreSQL query latency
- Redis latency
- upload session state counts
- multipart init / complete success and failure rate

### 12.2 Logging

日志必须是 structured logging，最少包含：

- request_id
- trace_id
- tenant_id
- user_id
- file_id
- upload_session_id
- service
- error_code

### 12.3 Tracing

重点覆盖：

- create upload session
- multipart init
- multipart complete
- file access authorize
- admin mutation

## 13. 安全设计

### 13.1 身份与鉴权

- 用户请求保留现有身份头或 JWT 解析逻辑
- 管理员链路单独使用 API Key 或内部 SSO
- 服务间调用使用 mTLS 或内部签名

### 13.2 数据安全

- PRIVATE bucket 不允许匿名访问
- 预签名 URL TTL 默认短时
- 敏感管理接口必须审计
- 对象删除与租户冻结操作必须支持二次确认或审计追踪

## 14. 实施顺序

### 14.1 总体策略

采用“先打稳核心模型，再实现服务”的顺序，而不是先堆接口。

### 14.2 推荐阶段

#### Phase 0: 契约与模型冻结

- 冻结外部 API 契约
- 固化数据模型
- 固化 UploadSession 状态机
- 补齐 golden tests

#### Phase 1: 先实现 access-service

原因：

- 访问链路职责最薄
- 更容易先打通 Go 服务基础设施基线
- 能最早验证鉴权、日志、metrics、配置、部署模板

#### Phase 2: 实现 upload-service 的 upload-session 与 direct-upload 能力

优先实现：

- `/api/v1/upload-sessions*`
- `/api/v1/direct-upload/*`

原因：

- 这两类路径最符合新架构方向
- 状态机和对象存储抽象最容易先沉淀为 canonical 模型

#### Phase 3: 实现 proxy upload 与 multipart 代理上传

补齐：

- `/api/v1/upload/*`
- `/api/v1/multipart/*`

#### Phase 4: 实现 admin-service 与 job-service

最后补齐：

- tenant admin
- file admin
- cleanup jobs

### 14.3 开发要求

- 所有阶段都以契约测试为入口
- 状态机测试优先于接口测试
- 先保证幂等性与一致性，再追求额外优化

## 15. 技术选型建议

### 15.1 Go 技术栈

- HTTP 框架: `gin` 或 `chi`
- ORM / SQL: `sqlc` + `pgx`，不建议重 ORM 优先
- 配置: `viper` 或显式配置加载
- 日志: `zap`
- metrics: `prometheus`
- trace: `OpenTelemetry`
- 对象存储: `minio-go` 或 AWS SDK for Go v2
- 迁移: `golang-migrate`

推荐：

- API 层用 `chi`
- 数据访问用 `pgx + sqlc`
- 对象存储优先 AWS SDK for Go v2

理由：

- 性能更稳
- 代码生成明确
- 比重 ORM 更利于维护复杂 SQL 和索引

## 16. 风险评估

| 级别 | 风险 | 说明 | 缓解措施 |
|------|------|------|----------|
| 高 | API 兼容性回归 | Go 重写后细节偏差会直接影响客户端 | 先做契约冻结和 golden tests |
| 高 | multipart 状态机不完整 | complete / abort / resume 容易出一致性问题 | 明确状态机与幂等规则 |
| 高 | 过度拆服务导致热路径变慢 | 服务边界过细会让上传和访问多跳 | 第一阶段控制为 4 个服务 |
| 中 | 契约兼容遗漏 | Go 重写后请求、响应、错误细节可能偏离旧客户端预期 | 用兼容矩阵和 golden tests 约束 |
| 中 | 管理后台与租户治理规则遗漏 | 控制面流量低但规则多 | 单独补兼容矩阵与管理用例 |
| 中 | 云原生配置复杂度 | 多服务后配置项和部署模板增多 | 建立统一部署基线与模板 |

## 17. 最终建议

推荐采用以下最终方案：

- 仓库形态：单仓库 monorepo
- 架构形态：共享底层模块 + 4 个核心服务
- 第一阶段重点：先迁访问链路，再迁 upload-sessions / direct-upload
- 技术路线：兼容现有 API，内部统一为 UploadSession / FileAccess / TenantPolicy 三大核心模型

这是一个兼顾性能、可维护性和云原生稳定性的最小可行升级方案。

它避免了两种常见错误：

1. 只换语言，不换架构
2. 一上来过度微服务化，导致热路径变慢、系统更脆

## 18. 后续建议文档

建议下一批文档按以下顺序补齐：

1. API 兼容矩阵
2. 核心数据表设计
3. UploadSession 状态机设计
4. multipart 一致性与幂等设计
5. 对象存储抽象设计
