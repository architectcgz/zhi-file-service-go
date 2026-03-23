# zhi-file-service-go 架构风格决策文档

## 1. 决策结论

`zhi-file-service-go` 采用：

- `Clean Architecture` 的依赖方向
- `DDD-lite` 的领域建模
- `CQRS-lite` 的读写分离

更准确地说，项目采用：

**`Clean-ish + DDD-lite + CQRS-lite`**

这不是术语堆叠，而是对项目复杂度和团队成本的平衡。

## 2. 为什么这样选

这个项目同时具备三类特征：

1. 有明确的业务边界
   - `upload`
   - `access`
   - `admin`
   - `job`

2. 有强状态机和一致性要求
   - `UploadSession`
   - `BlobObject`
   - `FileAsset`
   - `TenantPolicy / TenantUsage`

3. 有明显的读写差异
   - 上传写路径强调状态转换、事务和幂等
   - 访问读路径强调轻量、快速、少跳数

这种系统如果完全不用领域建模，很快会退化成“大 handler + 大 service + 大 dao”；但如果上重型 DDD / 重型 CQRS，又会引入大量工程噪音。

## 3. 我们要的不是“纯”架构

### 3.1 不是教科书式 `Clean Architecture`

我们要的是：

- `domain -> app -> ports -> infra/transport` 的依赖方向

而不是：

- 为了分层而分层
- 每层都定义大量空洞接口
- 所有对象都强行包成 use case / presenter / interactor

### 3.2 不是重型 DDD

我们要的是：

- 用领域模型承载真正的业务规则和状态机

而不是：

- 到处制造 aggregate / factory / specification / domain service 名词
- 为每个小字段创建 value object
- 用复杂术语掩盖简单业务

### 3.3 不是重型 CQRS

我们要的是：

- 代码层和 SQL 层的读写分离

而不是：

- command bus / query bus 先行
- 读库写库物理分离先行
- event sourcing 先行

## 4. 具体采用方式

## 4.1 Clean-ish

每个服务内部保持以下方向：

```text
transport/http -> app -> domain
                   |
                 ports
                   |
                 infra
```

关键约束：

- `domain` 不依赖 HTTP、DB、Redis、S3 SDK
- `app` 只编排 use case 和事务
- `infra` 实现端口
- `transport` 只负责协议适配

## 4.2 DDD-lite

真正需要领域建模的对象包括：

- `UploadSession`
- `FileAsset`
- `BlobObject`
- `TenantPolicy`
- `TenantUsage`

这些对象可以承载：

- 状态机
- 不变量
- 幂等规则
- 访问/配额规则

不需要领域建模的内容不要硬建模，例如：

- 纯列表查询 DTO
- 纯 OpenAPI request/response
- 简单配置对象

## 4.3 CQRS-lite

读写分离体现在三个层面：

1. use case 分离
   - `CreateUploadSession`
   - `CompleteUpload`
   - `AbortUpload`
   - `GetUploadProgress`
   - `GetFileAccess`
   - `ListTenantFiles`

2. SQL 分离
   - 命令侧 SQL 关注事务、锁、状态推进
   - 查询侧 SQL 关注投影、分页、索引命中

3. 模型分离
   - 写模型强调正确性
   - 读模型强调查询效率

但第一阶段不做：

- 物理读写分库
- 独立读模型存储
- 异步投影系统

## 5. 明确不采用的做法

### 5.1 不采用 Event Sourcing

原因：

- 复杂度远高于收益
- 项目当前核心不是审计重放，而是上传、访问和一致性

### 5.2 不采用全局 CommandBus / QueryBus

原因：

- 会隐藏调用链
- 会让简单用例变复杂
- 对当前规模没有必要

### 5.3 不采用全局 Generic Repository

原因：

- 领域模型差异太大
- SQL 语义不同
- 容易把复杂查询和锁语义抹平

### 5.4 不采用“所有东西都抽接口”

原因：

- Go 接口应该服务于消费方，而不是面向抽象表演
- 过度接口化会增加跳转和认知成本

## 6. 设计准则

## 6.1 什么时候建领域对象

满足以下任一条件就应考虑领域对象：

- 有明确状态机
- 有不变量
- 有多个 use case 共享规则
- 需要在事务内统一决策

否则优先保持简单 DTO。

## 6.2 什么时候读写分离

满足以下任一条件就应做 CQRS-lite：

- 查询和写入关注点明显不同
- 查询需要额外冗余字段或投影
- 写路径需要锁和事务，而读路径不需要

`file-service` 明显满足这些条件。

## 6.3 什么时候抽接口

只有在以下情况才抽接口：

- 调用方需要依赖抽象，而不是实现
- 需要替换 provider，例如 `storage`
- 需要在单元测试中隔离外部依赖

不满足这三条，就不要为了“好像更解耦”而抽接口。

## 7. 在本项目里的落地映射

### 7.1 upload-service

采用：

- `Clean-ish`
- `DDD-lite`
- `CQRS-lite`

因为它拥有最重的状态机和一致性逻辑。

### 7.2 access-service

采用：

- `Clean-ish`
- 轻量 `DDD-lite`
- 轻量 `CQRS-lite`

因为访问链路更偏读路径，领域逻辑比上传薄。

### 7.3 admin-service

采用：

- `Clean-ish`
- 轻量 `DDD-lite`

读写可以分 use case，但不需要过度强调 CQRS。

### 7.4 job-service

采用：

- `Clean-ish`
- 任务导向的 use case 编排

它更像后台 worker，不需要完整 HTTP 分层。

## 8. 对代码评审的直接约束

后续 code review 默认按以下问题判断实现是否偏离架构风格：

1. 是否把业务规则塞进 handler 或 repository
2. 是否让领域对象依赖 HTTP/DB/S3 SDK
3. 是否为了抽象而制造无意义接口
4. 是否把复杂写入和复杂查询混在一个 use case
5. 是否把不同服务的领域代码重新耦合到一起

## 9. 最终建议

这套风格的核心不是“信某个流派”，而是三条非常实际的工程规则：

1. 用 `Clean Architecture` 守住依赖方向
2. 用 `DDD-lite` 承载真正有业务价值的模型
3. 用 `CQRS-lite` 把读写关注点分开，但不引入重型基础设施

这就是当前项目最稳、最省成本、也最能长期演进的选择。
