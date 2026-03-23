# zhi-file-service-go 业务接入示例文档

## 1. 目标

这份文档给业务研发提供可直接照着接的标准示例。

它只覆盖三类第一阶段高频场景：

1. 公开头像展示
2. 私有附件下载
3. 私有文件短时预览跳转

配套文档：

- [business-integration-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/business-integration-spec.md)
- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [access-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/access-service-implementation-spec.md)
- [api/openapi/access-service.yaml](/home/azhi/workspace/projects/zhi-file-service-go/api/openapi/access-service.yaml)

## 2. 统一前提

所有示例都基于同一条规则：

- 业务库长期保存 `fileId`
- URL、ticket、redirect 都由 `file-service` 按访问场景临时派生

统一约束：

- 业务端不自己拼对象存储 URL
- 业务端不长期保存 private presigned URL
- 除 ticket redirect 外，数据面 north-south API 仍要求 Bearer Token

## 3. 场景一：公开头像展示

### 3.1 业务表建议

```sql
alter table users
  add column avatar_file_id varchar(26);
```

只保存：

- `avatar_file_id`

不要保存：

- `avatar_url`

### 3.2 上传完成后落库

业务侧在上传完成后拿到：

```json
{
  "fileId": "01JPA7ZZAMN4XQ1F6TQ8Y2DPRC"
}
```

然后写入业务表：

```json
{
  "userId": "u_001",
  "avatarFileId": "01JPA7ZZAMN4XQ1F6TQ8Y2DPRC"
}
```

### 3.3 列表或详情展示

推荐流程：

1. 业务服务读取 `avatar_file_id`
2. 调 `GET /api/v1/files/{fileId}`
3. 如果返回中已包含 `downloadUrl`，直接返回给前端

示例响应：

```json
{
  "requestId": "01JPACXDEHMGYNY4VAVW7H1MK4",
  "data": {
    "fileId": "01JPACWQ6A9Y0MST6FZFSGC4Y2",
    "tenantId": "demo",
    "fileName": "avatar.png",
    "contentType": "image/png",
    "sizeBytes": 182044,
    "accessLevel": "PUBLIC",
    "status": "ACTIVE",
    "downloadUrl": "https://cdn.example.com/public/demo/avatar.png"
  }
}
```

### 3.4 业务端优化方式

允许：

- 把 `downloadUrl` 作为响应字段下发给前端
- 在页面层短缓存 `downloadUrl`
- 失效后用 `fileId` 重新查询

不允许：

- 只在业务表保存 `avatar_url`
- 前端或业务端自行拼 `https://cdn/...`

## 4. 场景二：私有附件下载

典型场景：

- 订单发票
- 合同附件
- 私有报告

### 4.1 业务表建议

```sql
alter table invoices
  add column pdf_file_id varchar(26) not null;
```

### 4.2 浏览器下载

推荐流程：

1. 业务服务持有 `pdf_file_id`
2. 前端点击“下载”
3. 业务前端直接请求 `GET /api/v1/files/{fileId}/download`
4. `file-service` 返回 `302 Found`
5. 浏览器跟随 `Location` 下载

这里最终 `Location` 会是：

- public 文件：public URL
- private 文件：短时 presigned GET URL

### 4.3 为什么不用业务端自己拿 URL 落库

因为 private 下载地址：

- 会过期
- 可能随权限策略变化
- 可能随域名或存储策略变化

所以业务系统长期只保留：

- `pdf_file_id`

### 4.4 业务接口示例

业务接口只返回：

```json
{
  "invoiceId": "inv_001",
  "pdfFileId": "01JPACWQ6A9Y0MST6FZFSGC4Y1"
}
```

不要返回长期可复用的 private 下载 URL。

## 5. 场景三：私有文件短时预览跳转

典型场景：

- 在浏览器里打开 PDF 预览
- 短时间内把文件访问交给前端跳转
- 需要附件名和 `inline/attachment` 控制

### 5.1 推荐流程

1. 业务服务持有 `fileId`
2. 业务服务调用 `POST /api/v1/files/{fileId}/access-tickets`
3. 指定短 TTL 和 `responseDisposition`
4. 拿到 `ticket` 与 `redirectUrl`
5. 立即把 `redirectUrl` 返回给前端
6. 前端直接打开 `redirectUrl`

请求示例：

```json
{
  "expiresInSeconds": 120,
  "responseDisposition": "inline"
}
```

响应示例：

```json
{
  "requestId": "01JPACYQJW8TRVQF1F6N1WYJ16",
  "data": {
    "ticket": "at_01JPACYY3AS4K8Z9YV5SXVJ2QX",
    "redirectUrl": "/api/v1/access-tickets/at_01JPACYY3AS4K8Z9YV5SXVJ2QX/redirect",
    "expiresAt": "2026-03-21T10:05:00Z"
  }
}
```

### 5.2 业务端该保存什么

长期保存：

- `fileId`

短时返回给前端但不落库：

- `ticket`
- `redirectUrl`

### 5.3 适用边界

适合：

- 前端立即打开预览页
- 临时下载跳转
- 需要把 Bearer 保护面转换成一次性短时访问入口

不适合：

- 把 `redirectUrl` 写入数据库
- 第二天继续复用同一张 ticket

## 6. 三类场景怎么选

### 6.1 公开静态展示

例如：

- 头像
- 公开封面图
- 公开商品图

优先：

- `GET /api/v1/files/{fileId}`
- 使用返回里的 `downloadUrl`

### 6.2 受保护下载

例如：

- 发票
- 合同
- 仅登录用户可下载的附件

优先：

- `GET /api/v1/files/{fileId}/download`

### 6.3 浏览器临时跳转

例如：

- 预览页打开 PDF
- 向前端下发短时可访问入口

优先：

- `POST /api/v1/files/{fileId}/access-tickets`
- 再访问 `redirectUrl`

## 7. 常见错误

以下接法默认视为错误：

- 头像业务表只存 `avatar_url`
- 订单附件把 private presigned URL 写入数据库
- 预览链接把 `redirectUrl` 持久化，第二天继续用
- 业务端自己拼 `bucket + object_key + domain`
- 业务侧不保留 `fileId`，只保留 URL

## 8. 最小接入规则

如果业务研发只看一屏：

1. 上传完成后只落 `fileId`
2. public 展示时再查 `downloadUrl`
3. private 下载时走 `/files/{fileId}/download`
4. private 短时跳转时走 `access-ticket`
5. 任何 URL、ticket、redirect 都不要替代 `fileId`
