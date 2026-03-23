# zhi-file-service-go 新 API 设计文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的新 API 设计基线。

前提已经明确：

- 不再为旧 `file-service` 维护一份兼容矩阵
- 不再以 legacy 路径作为 canonical 外部契约
- 后续客户端、网关、内部调用方统一切到新 API

这份文档解决的问题是：

1. Go 重写版对外到底提供哪一套 API
2. `upload-service`、`access-service`、`admin-service` 的 API 如何统一风格
3. 路径、版本、鉴权、错误、分页、幂等如何形成统一规范
4. 后续 OpenAPI 文档该如何拆分和落地

## 2. 核心结论

### 2.1 新 API 是唯一外部契约

本项目采用一套新的 canonical API。

含义：

- 不再把旧路径当作必须保留的前提
- 不做 old/new 双轨长期并存
- 不为历史接口形态保留额外设计包袱

如果后续确实存在极少量过渡接口，也只能作为临时适配层存在，不能反向决定 canonical API 设计。

### 2.2 对外分为数据面与控制面两组前缀

推荐前缀：

- 数据面：`/api/v1`
- 控制面：`/api/admin/v1`

原因：

- 上传、访问属于数据面
- 租户治理、后台查询、审计属于控制面
- 路由、鉴权、限流、网关策略天然可以分离

### 2.3 统一采用资源化 + 会话化设计

新 API 不再延续“一个能力一组历史路径”的做法，而采用：

- 文件资源：`files`
- 上传会话：`upload-sessions`
- 访问票据：`access-tickets`
- 租户资源：`tenants`

其中上传必须以 `UploadSession` 为核心，而不是散落在：

- `upload`
- `direct-upload`
- `multipart`

多套并列入口中。

## 3. 设计原则

### 3.1 上传统一收敛到 `upload-sessions`

所有上传模式都以创建上传会话开始。

上传模式只是会话参数不同，而不是路径体系不同。

例如：

- 小文件代理上传
- 单对象 presigned 上传
- multipart presigned 上传

都应通过同一个 `create upload session` 入口创建。

### 3.2 文件访问与文件管理分离

`access-service` 提供：

- 文件访问授权
- 下载跳转
- 预签名 URL

`admin-service` 提供：

- 文件治理
- 后台查询
- 手工删除
- 租户与策略管理

不要把控制面查询重新混进高频读写接口。

### 3.3 路径表达资源，不表达实现细节

推荐：

- `/upload-sessions`
- `/files/{file_id}`
- `/tenants/{tenant_id}/policy`

避免：

- `/doUpload`
- `/getFileUrl`
- `/completeMultipartNow`

动作型操作可以作为资源子路径或 action path，但仍要围绕资源建模。

### 3.4 契约显式，不依赖隐式 header 魔法

新 API 应尽量减少历史遗留的隐式 header 语义。

允许保留少量基础头：

- `Authorization`
- `X-Request-Id`
- `Idempotency-Key`

但租户、主体、权限等核心语义应由认证上下文或显式字段表达，不依赖多套历史私有 header 拼装。

数据面身份上下文以 [data-plane-auth-context-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-plane-auth-context-spec.md) 为准；
管理面鉴权以 [admin-auth-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-auth-spec.md) 为准。

## 4. API 域划分

## 4.1 `upload-service`

负责：

- 创建上传会话
- 查询上传会话
- 代理内容上传
- presign part
- 列分片
- complete / abort

推荐路径：

- `POST /api/v1/upload-sessions`
- `GET /api/v1/upload-sessions/{uploadSessionId}`
- `PUT /api/v1/upload-sessions/{uploadSessionId}/content`
- `POST /api/v1/upload-sessions/{uploadSessionId}/parts/presign`
- `GET /api/v1/upload-sessions/{uploadSessionId}/parts`
- `POST /api/v1/upload-sessions/{uploadSessionId}/complete`
- `POST /api/v1/upload-sessions/{uploadSessionId}/abort`

说明：

- `PUT /content` 只用于代理上传场景
- 大文件默认走 presigned upload，不走服务端分片中转

## 4.2 `access-service`

负责：

- 文件元信息读取
- 下载授权
- 预签名 URL 签发
- 302 跳转

推荐路径：

- `GET /api/v1/files/{fileId}`
- `POST /api/v1/files/{fileId}/access-tickets`
- `GET /api/v1/files/{fileId}/download`
- `GET /api/v1/access-tickets/{ticket}/redirect`

说明：

- 浏览器友好场景优先 `download -> 302`
- 服务端集成优先 `access-tickets` 或直接返回 presigned URL
- 即便文件为 `PUBLIC`，`/api/v1/files/*` 默认仍要求 Bearer Token；匿名访问只发生在最终 public URL 或 `access-ticket redirect`

## 4.3 `admin-service`

负责：

- 租户管理
- 策略管理
- 配额查询
- 文件治理
- 审计查询

推荐路径：

- `GET /api/admin/v1/tenants`
- `POST /api/admin/v1/tenants`
- `GET /api/admin/v1/tenants/{tenantId}`
- `PATCH /api/admin/v1/tenants/{tenantId}`
- `GET /api/admin/v1/tenants/{tenantId}/policy`
- `PATCH /api/admin/v1/tenants/{tenantId}/policy`
- `GET /api/admin/v1/tenants/{tenantId}/usage`
- `GET /api/admin/v1/files`
- `GET /api/admin/v1/files/{fileId}`
- `DELETE /api/admin/v1/files/{fileId}`
- `GET /api/admin/v1/audit-logs`

## 4.4 `job-service`

`job-service` 第一阶段不提供面向外部调用方的公共 north-south API。

它主要通过：

- 定时任务
- outbox
- 内部 worker

执行清理、修复、补偿和对账。

## 5. 契约规范

## 5.1 版本策略

统一使用显式版本前缀：

- `/api/v1`
- `/api/admin/v1`

同一个大版本内允许：

- 新增字段
- 新增可选查询参数
- 新增向后兼容的枚举值

同一个大版本内不允许：

- 修改字段语义
- 重命名已发布字段
- 在无版本升级的情况下改变错误码语义

## 5.2 ID 与时间格式

统一约束：

- 业务 ID 使用 `ULID` 或等价的时间有序字符串
- JSON 字段使用驼峰或下划线只能二选一，建议统一驼峰
- 时间统一使用 RFC3339 / ISO8601

建议字段：

- `uploadSessionId`
- `fileId`
- `tenantId`
- `createdAt`
- `updatedAt`

## 5.3 成功响应

建议统一响应结构：

```json
{
  "requestId": "01HQ...",
  "data": {}
}
```

列表型响应建议：

```json
{
  "requestId": "01HQ...",
  "data": [],
  "page": {
    "nextCursor": "01HR..."
  }
}
```

## 5.4 错误响应

建议统一错误结构：

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

- `code` 稳定且可被客户端判定
- `message` 便于人看和日志排查
- `details` 仅放结构化补充信息

具体错误码以 [error-code-registry.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/error-code-registry.md) 为准。

## 5.5 幂等

以下操作默认要求支持幂等：

- 创建上传会话
- complete upload
- abort upload
- 管理员删除文件

建议：

- 客户端可传 `Idempotency-Key`
- 服务端同时结合资源状态机保证自然幂等

## 5.6 分页

后台列表查询统一使用 cursor pagination。

避免：

- 大 offset 分页
- 结果集越大越慢的后台接口

建议字段：

- `cursor`
- `limit`
- `nextCursor`

## 6. 与领域模型的映射

新 API 应直接映射到核心模型：

- `UploadSession`
- `FileAsset`
- `BlobObject`
- `TenantPolicy`
- `TenantUsage`

禁止继续让 API 结构围绕历史 controller 或历史 Java service 方法命名。

## 7. OpenAPI 落地方式

当前 OpenAPI 正式契约拆为：

- `api/openapi/upload-service.yaml`
- `api/openapi/access-service.yaml`
- `api/openapi/admin-service.yaml`

要求：

- 每个服务一份 OpenAPI
- 公共 schema 可以抽到共享 components
- 代码生成产物与手写代码分离

这份设计文档负责定义 API 方向；
字段级契约以这些 OpenAPI 文件为准，并在后续迭代中持续细化。

OpenAPI 如何从当前骨架升级为正式外部契约，以 [openapi-contract-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/openapi-contract-spec.md) 为准。

## 8. 明确不做的事情

- 不维护 legacy API 兼容矩阵
- 不同时长期维护 old API 和 new API
- 不为历史路径额外引入 `adapter` 层作为 canonical 实现
- 不把同一能力拆成多套风格不一致的接口

## 9. 最终结论

`zhi-file-service-go` 的外部接口应收敛为：

- 一套新的 canonical API
- 统一的资源命名和版本策略
- 统一的错误、分页、幂等和鉴权规则

后续所有服务与调用方都以这套新 API 为准，不再以旧 `file-service` 的历史路径作为设计边界。
