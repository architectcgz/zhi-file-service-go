# zhi-file-service-go 业务接入约定文档

## 1. 目标

这份文档面向接入 `zhi-file-service-go` 的业务系统。

它只回答业务侧最容易做错的几个问题：

1. 业务库里到底应该存 `fileId` 还是 URL
2. `downloadUrl`、`redirectUrl`、presigned URL、access ticket 分别是什么
3. 浏览器场景、服务端场景各该怎么接
4. 哪些做法会导致后续 CDN、权限策略、域名切换时返工

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [access-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/access-service-implementation-spec.md)
- [data-plane-auth-context-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-plane-auth-context-spec.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)
- [business-integration-examples.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/business-integration-examples.md)
- [frontend-upload-progress-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/frontend-upload-progress-spec.md)

## 2. 核心结论

### 2.1 业务侧长期保存 `fileId`

业务库、跨服务调用、消息体、业务对象关联字段，长期保存的 canonical 引用必须是 `fileId`。

不要把以下内容当成长期主引用：

- public URL
- presigned GET URL
- `redirectUrl`
- access ticket

### 2.2 URL 是派生访问值，不是业务主键

`file-service` 负责根据 `fileId` 派生：

- public 文件的 `downloadUrl`
- private 文件的 presigned GET URL
- `access-ticket` 与 `redirectUrl`
- `GET /files/{fileId}/download` 的最终 302 `Location`

这些值都属于访问层派生结果，不是业务层稳定标识。

### 2.3 业务端可以短缓存派生 URL，但不能拥有 URL 生成规则

业务端允许：

- 在页面层、接口响应层、短 TTL 缓存层暂存 `downloadUrl`
- 在 public 场景把 `downloadUrl` 当作展示优化字段

业务端不允许：

- 自己拼对象存储 URL
- 自己推导 bucket / key / CDN 域名
- 长期持久化 private presigned URL
- 用历史 URL 反向当文件主键

## 3. 应该存什么

### 3.1 必存字段

业务表如果要关联文件，至少存：

- `file_id`

这是唯一必须的长期字段。

### 3.2 可选冗余字段

如果业务页面需要减少一次详情查询，可以冗余存以下非主键信息：

- `file_name`
- `content_type`
- `size_bytes`
- `access_level`

这些字段只作为展示优化，不替代 `fileId`。

### 3.3 不应长期存储的字段

以下字段不应作为长期持久化引用：

- `download_url`
- `redirect_url`
- `access_ticket`
- private presigned URL

原因：

- 这些值可能过期
- 这些值可能因为 CDN、域名、bucket、权限策略变化而变化
- 它们不是业务稳定标识

## 4. 各类访问值的语义

### 4.1 `fileId`

用途：

- 文件的稳定业务引用
- 业务系统之间传递文件关联关系
- 后续查询详情、下载、删除、治理的入口

生命周期：

- 只要文件仍是业务上有效对象，就应持续可用

### 4.2 `downloadUrl`

用途：

- public 文件的直接访问地址
- 业务前端展示图片、附件链接时的便利字段

特点：

- 属于派生值
- 可能随网关、CDN、public base URL 调整而变化
- 允许业务端短缓存，不应作为唯一事实源

### 4.3 `redirectUrl`

用途：

- access ticket 对应的跳转入口

特点：

- 绑定短时票据
- 更适合浏览器立即使用
- 不适合长期持久化

### 4.4 presigned GET URL

用途：

- private 文件的临时下载地址

特点：

- 强时效
- 过期后不可用
- 不能写进业务表作为长期字段

### 4.5 access ticket

用途：

- 让浏览器或受限场景在短时间内完成访问跳转

特点：

- 默认短 TTL
- 默认不持久化为长期业务数据
- 过期后必须重新申请

## 5. 业务侧推荐接法

### 5.1 上传后落库

上传完成后，业务侧拿到 `fileId`，将其写入业务表。

例如：

```json
{
  "avatarFileId": "01JPA7ZZAMN4XQ1F6TQ8Y2DPRC"
}
```

不要在上传完成后把对象存储 URL 写入业务主表替代 `fileId`。

### 5.2 浏览器下载或预览

推荐流程：

1. 业务侧持有 `fileId`
2. 调 `GET /api/v1/files/{fileId}` 获取最小文件视图
3. 对 public 文件，如响应已带 `downloadUrl`，可直接使用
4. 对 private 文件或需要统一体验的场景，调 `GET /api/v1/files/{fileId}/download`
5. 由 `file-service` 返回 302 到最终地址

### 5.3 浏览器短时匿名访问

如果需要把一次访问委托给浏览器跳转：

1. 业务侧先调用 `POST /api/v1/files/{fileId}/access-tickets`
2. 拿到 `ticket` 和 `redirectUrl`
3. 浏览器立即访问 `redirectUrl`

这里业务侧仍然长期保存 `fileId`，不是 `ticket` 或 `redirectUrl`。

### 5.4 服务端集成

服务端场景默认也是围绕 `fileId` 调用：

1. 业务系统保存 `fileId`
2. 真正需要下载时，再根据 `fileId` 调 `download` 或签发 `access-ticket`

不要把 private presigned URL 发到 MQ、DB、审计日志里长期传播。

## 6. 允许的优化方式

允许做的优化：

- 页面层缓存 public `downloadUrl`
- 列表查询时把 `downloadUrl` 一并下发给前端
- 失败后根据 `fileId` 重新获取最新派生地址

不允许做的优化：

- 业务端本地拼接对象 URL
- 业务端缓存 private presigned URL 很长时间
- 用 URL 去重或反查业务记录

## 7. 典型反模式

以下做法默认视为错误接法：

- 用户头像表只存 `https://cdn.../avatar.png`，不存 `fileId`
- private 文件下载地址写进数据库，第二天继续复用
- 业务网关自己根据 bucket 和 key 拼下载链接
- 把 access ticket 当成长期业务凭证保存
- public 文件从一开始就不保留 `fileId`

## 8. 一句话规则

如果只记一条：

业务侧长期保存 `fileId`；URL、ticket、redirect 都由 `file-service` 按访问场景临时派生。
