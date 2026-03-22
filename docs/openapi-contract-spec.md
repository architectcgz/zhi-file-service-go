# zhi-file-service-go OpenAPI 正式契约规范文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的 OpenAPI 正式契约规范。

它解决的问题：

1. 当前 `api/openapi/*.yaml` 如何从“骨架”升级为正式外部契约
2. 哪些字段、响应、错误、示例必须写进 OpenAPI，而不是只停留在设计文档里
3. API 变更时，OpenAPI、实现、测试、SDK 如何保持同一事实源
4. 下载跳转、分页、幂等头、错误码这类容易漂移的细节如何固定

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [error-code-registry.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/error-code-registry.md)
- [data-plane-auth-context-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-plane-auth-context-spec.md)
- [admin-auth-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-auth-spec.md)
- [test-validation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/test-validation-spec.md)

## 2. 核心结论

### 2.1 OpenAPI 不是附属文档，而是 north-south API 的正式合同

对外 HTTP API 的最终契约以 OpenAPI 为准。

这意味着：

- 设计文档负责解释方向和原则
- OpenAPI 负责定义字段级、状态码级、header 级合同
- 实现、契约测试、SDK、网关配置都要跟着 OpenAPI 走

如果实现与 OpenAPI 不一致，应视为实现缺陷，而不是文档滞后。

### 2.2 三份服务契约都必须从“骨架”升级成“正式契约”

第一阶段正式契约仍按服务拆分：

- `api/openapi/upload-service.yaml`
- `api/openapi/access-service.yaml`
- `api/openapi/admin-service.yaml`

从现在开始，这三份文件不再只允许写“骨架”。

它们必须逐步具备：

- 完整 path / operation
- 完整 request / response schema
- 关键成功与失败示例
- endpoint 级错误码映射
- 分页、幂等、鉴权、跳转等细节语义

### 2.3 文档完成标准必须显式化

一个 API operation 只有在同时满足以下条件时，才算“OpenAPI 已完成”：

1. `operationId`、`summary`、`tags` 齐全
2. 请求参数和 body 明确
3. 成功响应结构完整
4. 主要失败响应完整
5. 错误码与 [error-code-registry.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/error-code-registry.md) 对齐
6. 至少一个成功示例
7. 至少一个关键失败示例
8. 已被契约测试覆盖

### 2.4 OpenAPI 必须可直接支撑自动化

正式契约的目标不是“给人看看”，而是要能直接服务于：

- contract test
- SDK codegen
- 网关校验
- 前后端联调
- 文档站渲染

任何无法支撑这些用途的 OpenAPI，都只能算未完成。

## 3. 适用范围

适用于：

- `/api/v1/*`
- `/api/admin/v1/*`

不适用于：

- `job-service` 内部 worker 交互
- 内部 outbox payload
- 服务内 Go 接口

## 4. 文件组织规则

## 4.1 当前拆分方式

继续保留每个 north-south 服务一份独立 OpenAPI 文件：

- `upload-service.yaml`
- `access-service.yaml`
- `admin-service.yaml`

原因：

- 服务边界清晰
- review 范围可控
- 与部署单元一致

## 4.2 公共组件策略

第一阶段允许两种方式：

1. 每个 YAML 自包含
2. 维护共享组件源，发布时 bundle 成单文件

但对外提供给代码生成、文档站和契约测试的产物必须是稳定可解析的最终文件。

禁止只保留一堆无法独立消费的半成品片段。

## 4.3 产物原则

推荐区分：

- `source spec`
- `bundled spec`

要求：

- source 可读、便于维护
- bundled 可直接供工具消费
- 两者来源关系必须明确

## 5. Operation 级必填项

每个 operation 至少必须具备：

- `tags`
- `summary`
- `description`
- `operationId`
- `security`
- path / query / header / cookie 参数定义
- requestBody
- responses

### 5.1 `operationId` 规范

要求：

- 全仓唯一
- 使用稳定动词 + 资源命名
- 不因 handler 名调整而变化

推荐示例：

- `createUploadSession`
- `completeUploadSession`
- `createAccessTicket`
- `patchTenantPolicy`

### 5.2 `description` 规范

`summary` 只写一句话。

`description` 必须补足以下易漂移语义：

- 路由用途
- 适用场景
- 特殊行为
- 幂等要求
- 跳转语义
- 对调用方的重要限制

## 6. 参数与请求体规范

## 6.1 参数命名

统一采用驼峰：

- `uploadSessionId`
- `fileId`
- `tenantId`
- `nextCursor`

不要在同一套 API 中混用：

- `file_id`
- `fileId`

## 6.2 Header 约束

当前明确纳入契约的公共 header：

- `Authorization`
- `X-Request-Id`
- `Idempotency-Key`

要求：

- 若某接口支持幂等，则在 OpenAPI 中显式声明 `Idempotency-Key`
- 若 `X-Request-Id` 由网关透传，也应在文档站或公共 header 说明中出现

## 6.3 Content-Type 约束

必须显式声明 content type，不依赖默认猜测。

重点场景：

- `application/json`
- `application/octet-stream`

上传相关接口尤其要避免“ body 是二进制但契约没写 content type ”。

## 6.4 请求体示例

每个有 request body 的写接口至少提供：

- 一个成功示例
- 一个典型错误输入示例

示例应覆盖真实字段，而不是空壳对象。

## 7. 响应规范

## 7.1 JSON 成功响应

统一结构：

```json
{
  "requestId": "01HQ...",
  "data": {}
}
```

列表型统一结构：

```json
{
  "requestId": "01HQ...",
  "data": [],
  "page": {
    "nextCursor": "01HR..."
  }
}
```

OpenAPI 中必须明确：

- `requestId` 始终存在
- `page` 仅在列表接口出现
- `nextCursor` 为空时的语义

## 7.2 错误响应

统一结构：

```json
{
  "requestId": "01HQ...",
  "error": {
    "code": "UPLOAD_SESSION_NOT_FOUND",
    "message": "upload session not found",
    "details": {}
  }
}
```

要求：

- `error.code` 必须来自注册表
- `message` 必须保留
- `details` 必须允许结构化扩展

## 7.3 302 跳转响应

对以下接口：

- `GET /api/v1/files/{fileId}/download`
- `GET /api/v1/access-tickets/{ticket}/redirect`

OpenAPI 必须明确：

- 返回 `302`
- `Location` header 必填
- 响应体默认为空
- 跳转地址的时效语义

不要只写“下载文件”，不写它到底是：

- 返回 JSON
- 还是直接 302

## 7.4 空响应与异步语义

若某接口未来返回：

- `202 Accepted`
- `204 No Content`

必须在 OpenAPI 里明确写出，不允许靠实现自行发挥。

## 8. 错误码映射规则

## 8.1 每个 operation 都要列出主要失败状态

最低要求：

- `400`
- `401`
- `403`
- `404`
- `409`
- `429`
- `500`
- `503`

并不是每个接口都必须全部出现，但必须显式判断哪些适用，哪些不适用。

## 8.2 Endpoint 级错误码说明

除了复用统一 `ErrorResponse`，还必须在 operation 描述或补充表中写清：

- 该接口可能返回哪些 `error.code`
- 每个错误码对应什么触发条件

例如：

- `completeUploadSession` 可返回 `UPLOAD_PARTS_MISSING`
- `patchTenant` 可返回 `TENANT_STATUS_INVALID`
- `createAccessTicket` 可返回 `DOWNLOAD_NOT_ALLOWED`

## 8.3 注册表联动

新增错误码时必须同时改：

1. OpenAPI
2. [error-code-registry.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/error-code-registry.md)
3. 契约测试

## 9. 分页、幂等与版本规则

## 9.1 分页

列表查询统一使用 cursor 分页。

OpenAPI 必须明确：

- `cursor`
- `limit`
- `nextCursor`

并定义：

- `cursor` 是不透明游标
- 客户端不得自行解析
- `nextCursor` 缺失或为空表示翻页结束

## 9.2 幂等

以下写操作默认支持幂等：

- 创建上传会话
- complete upload
- abort upload
- 创建租户
- 更新租户
- 更新租户策略
- 删除文件

OpenAPI 必须明确：

- 是否接受 `Idempotency-Key`
- 自然幂等还是显式幂等
- 重试时成功响应是否复用首次结果

## 9.3 版本

当前 north-south API 固定：

- `/api/v1`
- `/api/admin/v1`

OpenAPI 禁止混入未批准的：

- `/v2`
- `/legacy`
- `/internal`

## 10. 示例与文档站要求

## 10.1 示例要求

每个 operation 至少提供：

- 一个 happy path example
- 一个典型错误 example

高风险接口建议额外提供：

- 幂等重试 example
- 状态冲突 example
- 302 跳转 example

## 10.2 示例内容要求

示例必须尽量真实：

- 使用类似 ULID 的 ID
- 使用真实字段名
- 时间用 RFC3339
- 错误示例使用真实 `error.code`

不要用：

- `"string"`
- `"foo"`
- 空对象

来伪装完整示例。

## 10.3 curl 示例

正式对外文档站建议补充 curl 示例，但 curl 示例可以放在渲染层或派生文档中，不强制写进 OpenAPI YAML 本体。

OpenAPI 本体至少先保证：

- request example
- response example

## 11. 变更治理

## 11.1 API 变更流程

任何 API 变更至少同时满足：

1. 修改设计文档或明确说明无需修改
2. 修改 OpenAPI
3. 修改实现
4. 修改契约测试
5. 更新错误码或示例

不接受“代码先改，OpenAPI 以后再补”。

## 11.2 Breaking Change 规则

同一大版本内禁止：

- 删除字段
- 重命名字段
- 修改字段语义
- 修改既有错误码语义
- 把 `302` 改成 `200 JSON`
- 把 `200 JSON` 改成 `204`

如果必须发生，必须通过版本升级处理。

## 11.3 完成定义

一个服务的 OpenAPI 只有在同时满足以下条件时才算“正式契约已完成”：

1. 全部公开接口已在 YAML 中
2. 关键字段和 header 已完整
3. 示例齐全
4. 契约测试已接入
5. 文档站可直接渲染

## 12. Code Review 拦截项

看到以下情况应直接拦截：

- OpenAPI 里只有 path，没有关键示例
- 实现返回了新字段，但 YAML 未更新
- 错误码注册表已变更，但 OpenAPI 未同步
- 302 下载接口没有写 `Location`
- 写接口支持幂等，但 OpenAPI 没写 `Idempotency-Key`
- 继续把 YAML 当“骨架”而不是正式合同维护

## 13. 最终建议

这份文档的核心只有五条：

1. OpenAPI 是正式外部合同，不是附属说明
2. 三份服务 YAML 必须从骨架升级成可消费契约
3. 错误码、分页、幂等、302 语义都必须写进 OpenAPI
4. 示例必须真实，不能用空壳占位
5. API 变更必须同步改 OpenAPI、实现和契约测试

如果这层不先补齐，后续 SDK、联调、网关配置和 contract test 一定会反复返工。
