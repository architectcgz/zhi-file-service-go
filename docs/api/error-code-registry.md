# zhi-file-service-go 错误码注册表

## 1. 目标

这份文档定义 `zhi-file-service-go` 的公共错误码注册表。

它解决的问题：

1. `error.code` 到底有哪些稳定值
2. HTTP status 与错误码如何映射
3. 各服务如何扩展错误码而不互相冲突
4. 客户端如何按错误码而不是错误文案做判定

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [api/openapi/upload-service.yaml](/home/azhi/workspace/projects/zhi-file-service-go/api/openapi/upload-service.yaml)
- [api/openapi/access-service.yaml](/home/azhi/workspace/projects/zhi-file-service-go/api/openapi/access-service.yaml)
- [api/openapi/admin-service.yaml](/home/azhi/workspace/projects/zhi-file-service-go/api/openapi/admin-service.yaml)

## 2. 基本规则

错误响应统一结构：

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

规则：

- `code` 稳定、面向程序
- `message` 面向人读
- `details` 只放结构化补充信息

## 3. 命名规范

统一使用：

- 全大写
- 下划线分隔

例如：

- `UPLOAD_SESSION_NOT_FOUND`
- `TENANT_QUOTA_EXCEEDED`
- `ACCESS_TICKET_EXPIRED`

禁止：

- `UploadSessionNotFound`
- `upload.session.not_found`
- `ERR_XXX`

## 4. HTTP Status 映射原则

| HTTP Status | 语义 |
|------|------|
| `400` | 参数非法、契约不满足 |
| `401` | 未认证或 token 无效 |
| `403` | 已认证但无权访问 |
| `404` | 资源不存在或票据无效 |
| `409` | 资源状态冲突、幂等冲突、并发冲突 |
| `413` | 上传内容过大 |
| `429` | 触发限流 |
| `500` | 服务内部错误 |
| `503` | 依赖服务不可用 |

## 5. 通用错误码

| code | HTTP | 说明 |
|------|------|------|
| `INVALID_ARGUMENT` | `400` | 请求参数非法 |
| `PAYLOAD_TOO_LARGE` | `413` | 内容超出限制 |
| `UNAUTHORIZED` | `401` | 未认证或 token 无效 |
| `FORBIDDEN` | `403` | 已认证但无权限 |
| `NOT_FOUND` | `404` | 通用不存在 |
| `CONFLICT` | `409` | 通用冲突 |
| `RATE_LIMITED` | `429` | 触发限流 |
| `INTERNAL_ERROR` | `500` | 服务内部错误 |
| `SERVICE_UNAVAILABLE` | `503` | 依赖或本服务不可用 |

## 6. upload-service 错误码

| code | HTTP | 说明 |
|------|------|------|
| `UPLOAD_SESSION_NOT_FOUND` | `404` | 上传会话不存在 |
| `UPLOAD_SESSION_STATE_CONFLICT` | `409` | 当前状态不允许该操作 |
| `UPLOAD_COMPLETE_IN_PROGRESS` | `409` | 另一个 complete 正在进行 |
| `UPLOAD_MODE_INVALID` | `400` | 上传模式与请求不匹配 |
| `UPLOAD_HASH_REQUIRED` | `400` | 当前模式缺少必填 `contentHash` |
| `UPLOAD_HASH_INVALID` | `400` | 哈希格式非法 |
| `UPLOAD_HASH_UNSUPPORTED` | `400` | 哈希算法不支持 |
| `UPLOAD_HASH_MISMATCH` | `409` | 声明哈希与验证结果不一致 |
| `UPLOAD_PARTS_MISSING` | `409` | complete 时缺少分片 |
| `UPLOAD_MULTIPART_NOT_FOUND` | `409` | provider 侧 multipart 上传上下文已丢失，当前 session 需重新发起 |
| `UPLOAD_MULTIPART_CONFLICT` | `409` | provider 侧 multipart 状态与 complete 请求冲突，例如 part 顺序、etag 或最小分片大小不满足 |
| `TENANT_QUOTA_EXCEEDED` | `409` | 超出 tenant 配额 |
| `MIME_TYPE_NOT_ALLOWED` | `400` | MIME 类型不允许 |

## 7. access-service 错误码

以下错误码由 `access-service` 为主使用，其中 `FILE_NOT_FOUND` 也可由 `admin-service` 复用。

| code | HTTP | 说明 |
|------|------|------|
| `FILE_NOT_FOUND` | `404` | 文件不存在 |
| `FILE_ACCESS_DENIED` | `403` | 文件不可访问 |
| `ACCESS_TICKET_INVALID` | `404` | ticket 非法或不存在 |
| `ACCESS_TICKET_EXPIRED` | `404` | ticket 已过期 |
| `DOWNLOAD_NOT_ALLOWED` | `403` | 不允许下载或预览 |
| `TENANT_SCOPE_DENIED` | `403` | 认证主体租户范围不匹配 |

## 8. admin-service 错误码

| code | HTTP | 说明 |
|------|------|------|
| `ADMIN_PERMISSION_DENIED` | `403` | 角色或权限不足 |
| `TENANT_SCOPE_DENIED` | `403` | 管理员 tenant scope 不匹配 |
| `TENANT_NOT_FOUND` | `404` | 租户不存在 |
| `TENANT_STATUS_INVALID` | `409` | 租户状态不允许当前操作 |
| `TENANT_POLICY_INVALID` | `400` | 策略参数非法 |
| `AUDIT_QUERY_INVALID` | `400` | 审计查询参数非法 |

## 9. `details` 字段规范

推荐保留以下结构化 key：

- `field`
- `reason`
- `resourceType`
- `resourceId`
- `uploadSessionId`
- `providerUploadId`
- `operation`
- `providerErrorType`
- `currentStatus`
- `allowedStatuses`
- `retryAfterSeconds`
- `limit`

例如：

```json
{
  "error": {
    "code": "UPLOAD_SESSION_STATE_CONFLICT",
    "message": "upload session cannot be completed in current status",
    "details": {
      "resourceType": "uploadSession",
      "resourceId": "01HQ...",
      "currentStatus": "ABORTED",
      "allowedStatuses": ["UPLOADING", "COMPLETING"]
    }
  }
}
```

## 10. 扩展规则

新增错误码时必须同时更新：

1. 本注册表
2. 对应 OpenAPI
3. transport 层错误映射
4. 至少一条测试用例

禁止：

- 直接返回未注册的新 `error.code`
- 用错误文案代替稳定错误码
- 同一个错误码在不同接口表达不同含义

## 11. 最终结论

`error.code` 必须被当作稳定契约管理，而不是实现细节。

只要外部 API 还要被客户端长期依赖，这份注册表就必须持续维护。
