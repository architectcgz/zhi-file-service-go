# zhi-file-service-go access-service 实现细节文档

## 1. 目标

这份文档定义 `access-service` 的实现细节，重点固定高频读路径的实现边界。

它要解决的问题：

1. 文件访问链路怎么做到低延迟、低跳数
2. 文件详情、下载跳转、访问票据如何拆分
3. 什么数据必须直接读 `file_assets` 冗余列，避免后期回表返工

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [data-plane-auth-context-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-plane-auth-context-spec.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)
- [storage-abstraction-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/storage-abstraction-spec.md)

## 2. 服务职责

明确职责：

- 查询文件访问所需的最小元信息
- 校验访问权限
- 创建访问票据
- 返回 presigned GET URL 或执行 302 跳转

明确不负责：

- 文件上传
- 文件治理写操作
- 租户治理
- 引用计数修复

## 3. 服务内包结构

推荐结构：

```text
internal/services/access/
  domain/
    file_view.go
    access_policy.go
    access_ticket.go
    errors.go
  app/
    commands/
      create_access_ticket.go
    queries/
      get_file.go
      resolve_download.go
      redirect_by_ticket.go
  ports/
    file_read_repository.go
    tenant_policy_reader.go
    access_ticket_issuer.go
    object_locator.go
    presign_reader.go
  transport/http/
    handler.go
    request.go
    response.go
  infra/
    postgres/
    token/
    storage/
```

## 4. 读取模型约束

`access-service` 的热路径原则是：

- 单次请求尽量一次主查询完成
- 不在读路径 join 太多写侧表
- 不为了“更规范”牺牲响应延迟

因此 `file.file_assets` 必须冗余并可直接读取：

- `storage_provider`
- `bucket_name`
- `object_key`
- `access_level`
- `content_type`
- `size_bytes`
- `status`

这样 `GET /files/{fileId}` 和下载授权可以直接从 `file_assets` 投影完成。

## 5. 访问票据策略

第一阶段默认采用：

- 无状态签名票据

推荐实现：

- HMAC 签名 token 或 JWS

默认不引入：

- 持久化 `access_tickets` 表
- Redis 作为主票据存储

只有在需要以下能力时才引入 Redis：

- 单次消费
- 强制失效
- 大规模即时撤销

## 6. 核心用例实现

## 6.1 `GetFile`

流程：

1. 读取 `file_assets` 访问投影
2. 校验文件状态不能是删除终态
3. 基于认证上下文做租户或主体校验
4. 返回最小详情

约束：

- 默认不查 `blob_objects`
- 默认不请求对象存储
- `PUBLIC` 文件的 metadata 读取仍要求 Bearer Token，不开放匿名查询

## 6.2 `CreateAccessTicket`

流程：

1. 读取文件投影
2. 校验访问权限
3. 按策略决定是否允许下载、预览或附件下载
4. 生成带过期时间的签名票据
5. 返回票据及可选 redirect URL

票据至少包含：

- `file_id`
- `tenant_id`
- `subject`
- `expires_at`
- `disposition`

禁止：

- 为每个票据都写数据库
- 票据里塞入大量文件元数据

## 6.3 `ResolveDownload`

流程：

1. 读取文件投影
2. 校验权限
3. 对 public 文件优先直接返回 public URL
4. 对 private 文件调用 `PresignManager` 生成 GET URL
5. 按 north-south 正式契约返回 `302 Found`，并在 `Location` header 中给出最终下载地址

约束：

- 不代理大文件内容
- 不通过服务本身转发文件字节流
- 匿名访问只发生在最终 public URL 或 ticket redirect，不发生在 `/api/v1/files/*` API 本身

## 6.4 `RedirectByTicket`

流程：

1. 验签票据
2. 校验过期时间
3. 读取最小文件投影
4. 生成最终下载地址
5. 返回 302

约束：

- 票据校验失败直接返回统一错误
- 不允许通过票据绕过文件状态检查
- 这是数据面 north-south API 中唯一不要求 Bearer Token 的公开入口

## 7. Repository 与存储调用

建议只保留：

- `FileReadRepository`
- `TenantPolicyReader`
- `AccessTicketIssuer`
- `ObjectLocator`
- `PresignManager`

读路径基本不需要事务。

仅在极少数需要“读后记账”的场景，才允许引入短事务，但第一阶段不做下载计数强一致写入。

## 8. 性能约束

访问链路是高 QPS 路径，必须坚持：

- 不同步调用 `admin-service`
- 不同步调用 `upload-service`
- 不在请求链路里查多张冷表
- 不把下载流量回源到服务进程

默认优化方向：

- 热点文件详情可加本地短缓存或 Redis 缓存
- 但缓存只能加速，不能成为唯一事实源

## 9. 配置项

建议至少提供：

- `access.ticket_signing_key`
- `access.ticket_ttl`
- `access.download_redirect_ttl`
- `access.public_url_enabled`
- `access.private_presign_ttl`

## 10. 可观测性

关键指标：

- `file_get_total`
- `access_ticket_issue_total`
- `download_redirect_total`
- `download_redirect_failed_total`
- `access_ticket_verify_failed_total`

关键日志字段：

- `file_id`
- `tenant_id`
- `ticket`
- `access_level`
- `download_disposition`

关键 trace span：

- `access.get_file`
- `access.issue_ticket`
- `access.resolve_download`
- `access.redirect_by_ticket`

## 11. 测试要求

必须覆盖：

- 文件状态与访问级别判断
- 票据签发与验签
- public / private 下载两条路径
- 过期票据与篡改票据
- presign GET 集成测试

## 12. Code Review 检查项

看到以下实现应直接拦截：

- 下载接口代理文件字节流
- 每次访问都 join `blob_objects`
- 用数据库持久化所有访问票据
- 访问链路同步依赖其他业务服务
- 在 access handler 中写业务状态
