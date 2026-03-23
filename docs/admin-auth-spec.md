# zhi-file-service-go 管理面鉴权与权限模型文档

## 1. 目标

这份文档定义 `admin-service` 的鉴权和授权模型。

它解决的问题：

1. 管理员链路到底使用什么身份体系
2. 角色与权限如何映射到 API
3. destructive operation 如何强制带审计理由
4. 为什么 `admin-service` 不能复用数据面 token

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [admin-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-service-implementation-spec.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)

## 2. 适用范围

适用于：

- `/api/admin/v1/*`

不适用于：

- `/api/v1/*` 数据面接口
- job 内部补偿

## 3. 核心结论

### 3.1 管理面使用独立身份体系

`admin-service` 只接受后台管理员身份：

- 内部 SSO
- 专用管理员 IdP
- 后台专用 Bearer Token

不接受：

- 数据面用户 token
- upload/access 的普通租户 token

### 3.2 管理员主体固定映射到 `admin_subject`

固定规则：

- `admin_subject = token.sub`

写审计时不再做额外主体猜测。

### 3.3 授权模型分三层

至少固定三种角色：

- `admin.readonly`
- `admin.governance`
- `admin.super`

同时允许可选的：

- `tenant_scopes`
- `permissions`

用于做更细粒度限制。

## 4. Token Claim 契约

## 4.1 必填 claim

| Claim | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sub` | string | 是 | 管理员主体 ID |
| `roles` | string[] | 是 | 角色集合 |
| `iss` | string | 是 | 签发方 |
| `aud` | string 或 string[] | 是 | 必须覆盖 `zhi-file-admin` |
| `iat` | number | 是 | 签发时间 |
| `exp` | number | 是 | 过期时间 |

## 4.2 可选 claim

| Claim | 类型 | 说明 |
|------|------|------|
| `tenant_scopes` | string[] | 管理范围，`["*"]` 表示全局 |
| `permissions` | string[] | 细粒度权限 |
| `jti` | string | token ID |
| `name` | string | 管理员展示名 |

## 4.3 issuer allowlist

`admin-service` 默认要求 `iss` claim 存在。

如果部署时配置了 `ADMIN_AUTH_ALLOWED_ISSUERS`：

- `iss` 必须命中白名单中的某一项
- 白名单为空时，只校验 `iss` 存在，不做 pinning
- 建议生产环境显式配置，避免接入多个来源时出现误收 token

## 5. 标准化管理员上下文

推荐运行时结构：

```go
type AdminContext struct {
    RequestID    string
    AdminID      string
    Roles        []string
    TenantScopes []string
    Permissions  []string
    TokenID      string
}
```

## 6. 角色与 API 矩阵

| API | 最低角色 | 备注 |
|------|------|------|
| `GET /api/admin/v1/tenants` | `admin.readonly` | 支持租户范围过滤 |
| `GET /api/admin/v1/tenants/{tenantId}` | `admin.readonly` | 受 `tenant_scopes` 约束 |
| `GET /api/admin/v1/tenants/{tenantId}/policy` | `admin.readonly` | 受 `tenant_scopes` 约束 |
| `GET /api/admin/v1/tenants/{tenantId}/usage` | `admin.readonly` | 受 `tenant_scopes` 约束 |
| `GET /api/admin/v1/files*` | `admin.readonly` | 受 `tenant_scopes` 约束 |
| `GET /api/admin/v1/audit-logs` | `admin.readonly` | 可额外限制查看范围 |
| `PATCH /api/admin/v1/tenants/{tenantId}` | `admin.governance` | `status` 变更为 destructive 状态时必须要求 `reason` |
| `PATCH /api/admin/v1/tenants/{tenantId}/policy` | `admin.governance` | destructive 收紧必须带 `reason` 且审计 |
| `DELETE /api/admin/v1/files/{fileId}` | `admin.governance` | 必须写审计 reason |
| `POST /api/admin/v1/tenants` | `admin.super` | 默认仅超级管理员允许调用 |

## 7. Tenant Scope 规则

如果 token 带 `tenant_scopes`：

- `["*"]` 表示全局管理员
- 其他值表示只允许访问列出的 tenant

规则：

- 查询接口只能看到 scope 内 tenant
- 写接口只能修改 scope 内 tenant
- 跨 scope 请求直接 `403`

## 8. Destructive Operation 规则

以下操作默认视为 destructive：

- 删除文件
- 冻结租户
- 删除租户
- 大幅收紧 tenant policy

这些操作必须满足：

1. 有足够角色
2. 命中 tenant scope
3. 传入 `reason`
4. 审计日志落库

推荐在 request body 中统一使用：

```json
{
  "reason": "manual cleanup for abuse case"
}
```

## 9. 鉴权责任分层

### 9.1 transport 层

负责：

- Bearer token 校验
- claim 解析
- `AdminContext` 构造

### 9.2 app 层

负责：

- 角色校验
- tenant scope 校验
- 资源级授权
- destructive reason 校验

禁止把所有权限逻辑都塞进 middleware。

## 10. 错误语义

推荐固定：

- token 缺失或无效：`401 UNAUTHORIZED`
- token 有效但角色不足：`403 ADMIN_PERMISSION_DENIED`
- token 有效但 tenant scope 不匹配：`403 TENANT_SCOPE_DENIED`
- destructive request 缺少 `reason`：`400 INVALID_ARGUMENT`

## 11. 审计字段映射

写 `audit.admin_audit_logs` 时固定映射：

- `admin_subject = AdminContext.AdminID`
- `tenant_id` 来自目标资源
- `request_id = AdminContext.RequestID`
- `details.reason` 来自 request body
- `ip_address` / `user_agent` 来自 transport

## 12. 禁止事项

以下做法默认禁止：

- `admin-service` 复用数据面 token
- 用 `admin.readonly` 调用 destructive API
- destructive request 不带审计 reason
- 只在前端控制按钮显示，不在服务端做权限判断

## 13. 最终结论

管理面必须固定为：

- 独立身份体系
- 明确角色矩阵
- tenant scope 约束
- destructive operation 强制审计 reason

否则后台 API 在真正进入多人协作和线上治理后一定返工。
