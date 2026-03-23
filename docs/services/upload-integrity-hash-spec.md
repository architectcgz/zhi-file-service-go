# zhi-file-service-go 上传完整性与哈希契约文档

## 1. 目标

这份文档定义新上传 API 的文件完整性和哈希契约。

它解决的问题：

1. API 里的哈希字段到底叫什么、长什么样
2. 哪种哈希才是 dedup / 秒传 / 续传的 canonical hash
3. `INLINE`、`PRESIGNED_SINGLE`、`DIRECT` 三种模式如何验证内容完整性
4. `blob_objects.hash_value` 什么时候允许写入

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md)
- [upload-session-state-machine-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-session-state-machine-spec.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)

## 2. 核心结论

### 2.1 公共 API 使用 `contentHash`

新 API 不再使用语义模糊的裸 `checksum` 字段承载整文件哈希。

公共 API 统一使用：

```json
{
  "contentHash": {
    "algorithm": "SHA256",
    "value": "hex-lowercase"
  }
}
```

### 2.2 Canonical 文件哈希固定为 `SHA256`

第一阶段公共 API 只接受：

- `SHA256`

为了保留明确的业务错误语义，OpenAPI 中 `contentHash.algorithm` 保持为字符串字段，
由服务在业务校验阶段返回 `UPLOAD_HASH_UNSUPPORTED`，而不是把该场景折叠成通用 `INVALID_ARGUMENT`。

不把以下值作为 canonical 文件哈希：

- `ETag`
- `MD5`
- provider 私有 checksum 字段
- multipart 各 part checksum

这些值可以作为 provider 兼容信息存在，但不能替代 canonical 文件哈希。

### 2.3 `blob_objects.hash_value` 只能保存已验证哈希

固定规则：

- `upload.upload_sessions.file_hash` 可以保存客户端声明哈希
- `file.blob_objects.hash_value` 只能保存已验证哈希

也就是说，`blob_objects` 永远不落未验证的客户端声明值。

## 3. API 字段契约

## 3.1 `CreateUploadSessionRequest`

建议字段：

```json
{
  "fileName": "a.png",
  "contentType": "image/png",
  "sizeBytes": 12345,
  "uploadMode": "DIRECT",
  "contentHash": {
    "algorithm": "SHA256",
    "value": "..."
  }
}
```

规则：

- `DIRECT` 与 `PRESIGNED_SINGLE` 默认要求提供 `contentHash`
- `INLINE` 可以不传，服务端可边写边算

## 3.2 `CompleteUploadSessionRequest`

允许再次回传 `contentHash`，用于一致性复核。

规则：

- 如果 create 与 complete 都传，二者必须完全一致
- 如果不一致，直接失败

## 3.3 Multipart Part

对 multipart part：

- `etag` 只是 provider 侧 part 标识
- part checksum 只作为分片级校验信息

它们都不是整文件 canonical hash。

## 4. 持久化映射

## 4.1 `upload.upload_sessions`

固定语义：

- `hash_algorithm`
- `file_hash`

这两个字段在 session 阶段保存“声明哈希”。

在 complete 成功后：

- 若验证通过，则该值与最终 verified hash 一致
- 若验证失败，则 session 进入失败路径

## 4.2 `file.blob_objects`

固定语义：

- `hash_algorithm`
- `hash_value`

只能写 verified hash。

如果对象哈希无法验证，则：

- 不能把该对象注册为可 dedup 的 canonical blob
- 不能进入正常 complete 成功路径

## 4.3 `file.file_assets`

`file_hash` 是读取优化快照。

它的来源必须是已经验证成功的 canonical hash。

## 5. 三种上传模式的验证规则

## 5.1 `INLINE`

规则：

- 服务端流式写对象时同步计算 SHA256
- complete 时以服务端实算值为准

这是最容易做到强验证的模式。

## 5.2 `PRESIGNED_SINGLE`

规则：

- 客户端创建会话时提交 `contentHash`
- 直传时必须携带 provider 支持的 checksum 头或等价校验信息
- complete 时服务端通过对象存储 metadata / checksum 能力验证 SHA256

如果 provider 无法给出可验证的 SHA256 结果，则该上传模式在 v1 中不应作为 canonical 成功路径启用。

## 5.3 `DIRECT` multipart

规则：

- 创建会话时提交整文件 `contentHash`
- 上传 part 时可带 part 级校验信息
- complete 时必须通过 provider 的 multipart checksum 能力或可组合校验结果验证整文件哈希

同样，如果 provider 不能支持最终整文件 SHA256 验证，则不能把结果写入 `blob_objects.hash_value`。

## 6. Dedup / 秒传 / 续传规则

## 6.1 续传匹配

续传匹配允许使用：

- `tenant_id`
- `owner_id`
- `upload_mode`
- `expected_size`
- `declared contentHash`

因为它只是匹配活跃 session，不会直接创建最终文件引用。

## 6.2 秒传 / instant upload

秒传要求同时满足：

1. 客户端提供 `contentHash`
2. 存在同租户内已验证的 blob
3. `hash_algorithm + hash_value + size_bytes + bucket` 一致

也就是说：

- 秒传依赖“已有 verified blob”
- 不依赖“另一个未完成会话的声明哈希”

## 6.3 去重作用域

第一阶段固定为：

- 同租户内 dedup

不做默认跨租户物理对象共享。

## 7. 错误码

建议保留以下错误码：

- `UPLOAD_HASH_REQUIRED`
- `UPLOAD_HASH_INVALID`
- `UPLOAD_HASH_MISMATCH`
- `UPLOAD_HASH_UNSUPPORTED`
- `UPLOAD_COMPLETE_IN_PROGRESS`

推荐语义：

- 缺少必填 `contentHash`：`400`
- 算法不支持：`400`
- create/complete 哈希不一致：`409`
- provider 校验结果与声明哈希不一致：`409`

## 8. 禁止事项

以下做法默认禁止：

- 把 `ETag` 当作文件哈希
- 允许 `MD5` 成为 dedup canonical hash
- 把未验证客户端哈希写进 `blob_objects`
- 用 multipart part checksum 冒充整文件 hash

## 9. 最终结论

上传完整性契约必须固定为：

- 公共 API 使用 `contentHash`
- Canonical 算法固定 `SHA256`
- `blob_objects.hash_value` 只保存 verified hash

否则后续 dedup、秒传、complete、一致性和审计都会反复返工。
