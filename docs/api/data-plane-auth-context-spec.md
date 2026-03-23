# zhi-file-service-go 数据面身份上下文契约文档

## 1. 目标

这份文档定义 `upload-service` 与 `access-service` 的数据面身份上下文契约。

它解决的问题：

1. 数据面 API 到底信任什么身份信息
2. `tenant_id` 和 `owner_id` 从哪里来
3. 哪些字段进入 Go 运行时 `AuthContext`
4. 哪些 header 明确禁止作为公开契约

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md)
- [access-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/access-service-implementation-spec.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)

## 2. 适用范围

本契约只适用于数据面 north-south API：

- `/api/v1/upload-sessions*`
- `/api/v1/files/*`
- `/api/v1/access-tickets/*`

不适用于：

- `admin-service`
- 服务间内部调度
- 后台 job

## 3. 核心结论

### 3.1 外部认证入口固定为 Bearer Token

公开数据面 API 只接受：

- `Authorization: Bearer <token>`
- `X-Request-Id` 可选

唯一例外：

- `GET /api/v1/access-tickets/{ticket}/redirect` 可不带 `Authorization`
- 因为 `ticket` 本身就是短时访问凭证

不把以下 header 作为公开契约：

- `X-Tenant-Id`
- `X-User-Id`
- `X-App-Id`
- `X-Owner-Id`

这些 header 即使以后在网关或内部链路出现，也不能作为对外稳定接口语义。

### 3.2 服务只信任标准化后的 `AuthContext`

进入 use case 之前，transport / middleware 必须把认证结果标准化成统一的 `AuthContext`。

业务代码只读取 `AuthContext`，不直接解析 JWT claim 或 header。

### 3.3 `tenant_id` 与 `owner_id` 的来源固定

固定规则：

- `tenant_id` 来自认证上下文
- `owner_id` 固定等于 `AuthContext.SubjectID`

这意味着：

- `upload.upload_sessions.owner_id = AuthContext.SubjectID`
- `file.file_assets.owner_id = AuthContext.SubjectID`

`owner_id` 不允许从 request body 或 query 中传入。

## 4. Token Claim 契约

## 4.1 必填 claim

推荐要求以下 claim：

| Claim | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sub` | string | 是 | 数据面主体 ID |
| `tenant_id` | string | 是 | 租户 ID |
| `subject_type` | string | 是 | `USER` / `APP` |
| `scope` | string 或 string[] | 是 | 最小权限集合 |
| `iss` | string | 是 | 签发方 |
| `aud` | string 或 string[] | 是 | 必须覆盖数据面 audience |
| `iat` | number | 是 | 签发时间 |
| `exp` | number | 是 | 过期时间 |

建议 audience 固定为：

- `zhi-file-data-plane`

## 4.2 可选 claim

| Claim | 类型 | 说明 |
|------|------|------|
| `jti` | string | token ID，用于审计和追踪 |
| `client_id` | string | 调用方 client 标识 |
| `scope_version` | string | 可选 scope 版本 |

## 4.3 scope 约束

第一阶段至少保留两类 scope：

- `file:read`
- `file:write`

推荐映射：

- `upload-service` 写接口要求 `file:write`
- `access-service` 读接口要求 `file:read`

## 5. Go 运行时上下文

推荐标准化为：

```go
type AuthContext struct {
    RequestID   string
    SubjectID   string
    SubjectType string
    TenantID    string
    ClientID    string
    TokenID     string
    Scopes      []string
}
```

约束：

- `SubjectID` 来自 `sub`
- `TenantID` 来自 `tenant_id`
- `Scopes` 在 middleware 中归一化为 `[]string`

## 6. 服务行为映射

## 6.1 `upload-service`

固定规则：

- 创建上传会话时 `tenant_id = AuthContext.TenantID`
- 创建上传会话时 `owner_id = AuthContext.SubjectID`
- 续传匹配与 dedup claim 作用域按 `tenant_id + owner_id` 或 `tenant_id + hash` 执行

禁止：

- 从 request body 提交 `ownerId`
- 从 query/header 注入 `tenantId`

## 6.2 `access-service`

固定规则：

- 读取文件时必须校验文件所属 `tenant_id` 与 `AuthContext.TenantID`
- 访问票据默认绑定 `tenant_id` 与 `subject`
- ticket redirect 仍必须复核文件状态
- 除 `GET /api/v1/access-tickets/{ticket}/redirect` 外，数据面 north-south API 仍要求 Bearer Token
- `PUBLIC` 文件的匿名访问只发生在最终 public URL 或 redirect 落点，不发生在 `/api/v1/files/*` north-south 接口

### 6.3 `subject_type`

第一阶段允许：

- `USER`
- `APP`

规则：

- `USER` 适用于终端用户或业务用户
- `APP` 适用于服务端代表某个租户进行调用

二者都必须带 `tenant_id`。

## 7. 网关模式

如果前面有 API Gateway 或认证网关：

- 网关可以提前校验 token
- 但下游服务仍必须拿到统一的标准化身份上下文

允许两种实现：

1. 网关透传原始 Bearer Token，由服务自身校验
2. 网关生成内部签名身份上下文，由服务校验内部签名后构造 `AuthContext`

无论采用哪种实现，都必须满足：

- 公开 API 仍只暴露 `Authorization`
- 外部客户端不能直接传入内部身份 header
- 网关必须清洗来自外部的伪造内部 header

## 8. 错误语义

推荐固定：

- token 缺失或签名非法：`401 UNAUTHORIZED`
- token 有效但缺少 scope：`403 FORBIDDEN`
- token 有效但 `tenant_id` 缺失：`403 FORBIDDEN`
- token 有效但访问跨租户资源：`403 FORBIDDEN`

推荐错误码：

- `UNAUTHORIZED`
- `FORBIDDEN`
- `TENANT_SCOPE_DENIED`
- `FILE_ACCESS_DENIED`

## 9. 禁止事项

以下做法默认禁止：

- 对外暴露 `X-Tenant-Id` / `X-User-Id`
- handler 直接解析 JWT claim 并把结果散传
- 从 body/query 中接受 `tenant_id` 覆盖认证上下文
- 在数据面复用 `admin-service` 的管理员 token

## 10. 最终结论

数据面身份契约必须收敛为：

- Bearer Token 作为默认公开认证入口，ticket redirect 作为短时凭证例外
- `AuthContext` 作为唯一业务可见身份来源
- `tenant_id` 与 `owner_id` 从认证上下文固定映射

只有这样，上传、访问、续传、去重和鉴权逻辑才能稳定落地而不返工。
