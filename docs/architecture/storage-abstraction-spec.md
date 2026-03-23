# zhi-file-service-go 对象存储抽象设计文档

## 1. 目标

本设计文档定义 `zhi-file-service-go` 的对象存储抽象层。

它解决以下问题：

1. `upload-service` 和 `access-service` 如何共享一套存储能力，而不是各自直接耦合 S3 SDK
2. multipart、单对象 presign、对象 copy/delete、对象 metadata 读取的边界如何统一
3. bucket 选择、对象 key 命名、URL 解析、异常分类如何标准化

这份文档和以下文档配套使用：

- [architecture-upgrade-design.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/architecture-upgrade-design.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)
- [upload-session-state-machine-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-session-state-machine-spec.md)
- [data-protection-recovery-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-protection-recovery-spec.md)

## 2. 关键结论

### 2.1 不做独立的 `storage-service`

对象存储能力在第一阶段应作为 Go 代码库中的共享模块存在，不应做成网络服务。

原因：

- 上传和访问都是热路径
- 多一跳 RPC 只会放大延迟和故障面
- 存储抽象天然是基础设施适配层，不是独立业务域

### 2.2 抽象要按能力拆分，而不是一个巨型接口

当前 Java 里的 `ObjectStoragePort` 已经能工作，但对 Go 多服务架构来说还偏大。

Go 版推荐拆成以下能力接口：

- `BucketResolver`
- `ObjectLocator`
- `ObjectReader`
- `ObjectWriter`
- `MultipartManager`
- `PresignManager`

这样可以让不同服务只依赖自己真正需要的最小接口。

### 2.3 默认目标是 S3 兼容

Go 版以 S3 协议为主抽象，兼容：

- MinIO
- AWS S3
- 其他 S3-compatible 存储

不再把“本地文件系统”作为主要实现路径。

## 3. 服务与存储能力映射

### 3.1 `upload-service`

需要的能力：

- bucket 解析
- object key 规划
- multipart create / list parts / complete / abort
- 单对象 metadata 读取
- 单对象 delete
- part presign
- object put presign
- 受控代理上传场景下的服务端 `uploadPart`

### 3.2 `access-service`

需要的能力：

- bucket 解析
- public object URL 解析
- private object presign GET 或临时可访问 URL
- access level 切换时的 object copy / delete

### 3.3 `admin-service`

需要的能力：

- object metadata 读取
- 手工删除对象
- 可选的 bucket/object 健康检查

### 3.4 `job-service`

需要的能力：

- multipart abort
- orphan object delete
- 对账时 object metadata 读取

## 4. 推荐包结构

推荐在 Go 版中使用：

```text
pkg/storage/
  types.go
  errors.go
  resolver.go
  reader.go
  writer.go
  multipart.go
  presign.go
  s3/
    provider.go
    bucket_resolver.go
    object_locator.go
    object_reader.go
    object_writer.go
    multipart_manager.go
    presign_manager.go
```

说明：

- `pkg/storage` 只放抽象和通用类型
- `pkg/storage/s3` 放 S3-compatible 具体实现
- service 层不直接 import AWS SDK

## 5. 推荐接口设计

## 5.1 基础类型

建议定义以下基础类型：

```go
type AccessLevel string

const (
    AccessLevelPublic  AccessLevel = "PUBLIC"
    AccessLevelPrivate AccessLevel = "PRIVATE"
)

type BucketRef struct {
    Provider   string
    BucketName string
    PublicBase string
}

type ObjectRef struct {
    Provider   string
    BucketName string
    ObjectKey  string
}

type UploadedPart struct {
    PartNumber int
    ETag       string
    SizeBytes  int64
}

type ObjectMetadata struct {
    SizeBytes   int64
    ContentType string
    ETag        string
}
```

## 5.2 `BucketResolver`

职责：

- 按访问级别和策略解析 bucket
- 处理默认 bucket / public bucket / private bucket

建议接口：

```go
type BucketResolver interface {
    Resolve(accessLevel AccessLevel) (BucketRef, error)
    Normalize(bucketName string) string
}
```

## 5.3 `ObjectLocator`

职责：

- 解析 public object URL
- 为访问链路提供 canonical 对象定位

建议接口：

```go
type ObjectLocator interface {
    ResolveObjectURL(ref ObjectRef) (string, error)
}
```

## 5.4 `ObjectReader`

职责：

- 读取对象 metadata
- 可选的 exists / head 能力

建议接口：

```go
type ObjectReader interface {
    HeadObject(ctx context.Context, ref ObjectRef) (ObjectMetadata, error)
}
```

说明：

- Go 版第一阶段不建议暴露“直接读取对象内容”给业务服务
- 服务端不负责代理大文件下载

## 5.5 `ObjectWriter`

职责：

- 删除对象
- 复制对象
- 可选的小文件服务端直写

建议接口：

```go
type ObjectWriter interface {
    DeleteObject(ctx context.Context, ref ObjectRef) error
    CopyObject(ctx context.Context, source ObjectRef, target ObjectRef) error
}
```

如果要支持 `INLINE` 直接上传：

```go
type InlineObjectWriter interface {
    PutObject(ctx context.Context, ref ObjectRef, contentType string, body io.Reader, size int64) error
}
```

## 5.6 `MultipartManager`

职责：

- multipart create
- 可选的服务端 upload part
- list authoritative parts
- complete
- abort

建议接口：

```go
type MultipartManager interface {
    CreateMultipartUpload(ctx context.Context, ref ObjectRef, contentType string) (uploadID string, err error)
    UploadPart(ctx context.Context, ref ObjectRef, uploadID string, partNumber int, body io.Reader, size int64) (etag string, err error)
    ListUploadedParts(ctx context.Context, ref ObjectRef, uploadID string) ([]UploadedPart, error)
    CompleteMultipartUpload(ctx context.Context, ref ObjectRef, uploadID string, parts []UploadedPart) error
    AbortMultipartUpload(ctx context.Context, ref ObjectRef, uploadID string) error
}
```

说明：

- `UploadPart` 不是前台默认路径，只用于受控代理上传或后台特殊任务
- `ListUploadedParts` 必须返回 authoritative parts，不能依赖本地缓存

## 5.7 `PresignManager`

职责：

- 生成单对象 PUT presign
- 生成 multipart part presign
- 生成对象 GET presign

建议接口：

```go
type PresignManager interface {
    PresignPutObject(ctx context.Context, ref ObjectRef, contentType string, ttl time.Duration) (url string, headers map[string]string, err error)
    PresignUploadPart(ctx context.Context, ref ObjectRef, uploadID string, partNumber int, ttl time.Duration) (url string, headers map[string]string, err error)
    PresignGetObject(ctx context.Context, ref ObjectRef, ttl time.Duration) (url string, err error)
}
```

说明：

- 返回 `headers` 很重要，避免不同 provider 对签名头要求不一致
- access-service 私有文件访问建议优先使用 `PresignGetObject`

## 6. 为什么要拆接口

如果继续沿用一个大而全的 `ObjectStorage` 接口，会有三个问题：

1. `access-service` 只需要读和 URL 解析，却被迫依赖 multipart 能力
2. 单元测试会变得臃肿，因为每个 mock 都要覆盖无关方法
3. 后续如果引入 CDN 或第二种 provider，很难做局部替换

拆接口后：

- `upload-service` 依赖 `BucketResolver + MultipartManager + PresignManager + ObjectReader + ObjectWriter`
- `access-service` 依赖 `BucketResolver + ObjectLocator + PresignManager + ObjectWriter`
- `job-service` 依赖 `BucketResolver + MultipartManager + ObjectReader + ObjectWriter`

## 7. bucket 设计

## 7.1 双 bucket 策略

推荐继续保留：

- public bucket
- private bucket

理由：

- 公开文件与私有文件访问语义天然不同
- 对 CDN、ACL、缓存控制更友好

### 7.2 bucket 选择规则

默认规则：

- `PUBLIC` -> `publicBucket`
- `PRIVATE` -> `privateBucket`

未来扩展点：

- 租户级 bucket 覆盖
- region 级 bucket 覆盖
- storage class 级 bucket 路由

但第一阶段不要让 bucket 路由规则过于动态。

### 7.3 bucket 名标准化

推荐保留 `Normalize(bucketName)` 能力，规则：

- 显式传入 bucket 时优先使用
- 空值回退到默认 bucket
- 返回值永远不允许为空

## 8. object key 设计

## 8.1 目标

object key 设计需要同时满足：

- 多租户隔离清晰
- 前缀分布均衡
- 人工排查可读
- 不依赖 bucket 语义也能看懂对象归属

## 8.2 推荐格式

推荐统一格式：

```text
{tenant_id}/{yyyy}/{mm}/{dd}/{owner_id}/{category}/{object_id}-{sanitized_filename}
```

例如：

```text
blog/2026/03/21/user-001/uploads/01HZZABCDEF-demo.mp4
im/2026/03/21/user-101/images/01HZZXYZ123-avatar.png
```

字段说明：

- `tenant_id`: 强隔离前缀
- `yyyy/mm/dd`: 运维与排查友好
- `owner_id`: 定位用户维度问题
- `category`: `uploads` / `images` / `attachments`
- `object_id`: 使用 ULID 或等价有序 ID
- `sanitized_filename`: 仅作为可读后缀，不参与身份判断

## 8.3 设计约束

- `object_key` 必须由服务端生成，客户端不能直接控制
- 文件名必须做 sanitize
- 不允许把 access level 放进 object key，access level 由 bucket 决定
- key 一旦完成写入，不建议在后续流程中改名

## 9. multipart 设计

## 9.1 设计原则

- multipart 的权威状态在对象存储，不在数据库
- `upload_session_parts` 是服务侧缓存 / 快照，不是唯一真相
- `complete` 前必须重新读取 authoritative parts

## 9.2 multipart create

输入：

- bucket
- object key
- content type

输出：

- `upload_id`

失败要求：

- 失败必须返回明确 provider error
- 不能创建半初始化数据库状态而不记录失败原因

## 9.3 multipart upload part

支持两种模式：

1. 客户端拿 presigned part URL 直接上传
2. 受控场景下由 `upload-service` 中转分片

默认推荐是：

- 前台大文件优先 presigned part
- 服务端中转仅用于后台、低并发或受控网络环境

## 9.4 multipart list parts

要求：

- 必须按 `part_number` 升序返回
- 每个 part 至少返回 `part_number + etag + size_bytes`
- 不允许只返回 part number，complete 需要 etag

## 9.5 multipart complete

要求：

- 调用前必须读取 authoritative parts
- parts 需要按 part number 升序传给 provider
- 若客户端传来的 etag 和对象存储实际不一致，应直接失败

## 9.6 multipart abort

要求：

- abort 应视为幂等操作
- upload 已不存在时，应能安全返回
- `job-service` 可反复执行 abort 清理过期 session

## 10. presign 设计

## 10.1 PutObject presign

用于：

- `PRESIGNED_SINGLE`

要求：

- TTL 可配置
- content type 应纳入签名上下文
- 若 provider 需要额外 header，必须回传给客户端

## 10.2 UploadPart presign

用于：

- `DIRECT` multipart

要求：

- part number 必须纳入签名上下文
- upload id 必须纳入签名上下文
- TTL 应短于 session TTL

## 10.3 GetObject presign

用于：

- private 文件访问

要求：

- 默认短 TTL
- 不把用户身份直接编码到 object key
- 用户鉴权在业务层完成，presign 只是存储访问手段

## 11. URL 解析策略

### 11.1 public object URL

公开对象 URL 的优先级建议：

1. `cdnDomain + objectKey`
2. `publicEndpoint + /{bucket}/{objectKey}`
3. provider 原始 endpoint

### 11.2 private object URL

私有对象不要暴露固定 URL。

推荐：

- 由 access-service 每次鉴权后动态生成短期 presign GET

## 12. access level 切换

文件从 `PUBLIC` 切到 `PRIVATE`，或反向切换时，本质是对象迁移。

推荐行为：

1. 解析源 bucket 和目标 bucket
2. 使用同一个 object key 做 `CopyObject`
3. 新建或复用目标 `blob_object`
4. 事务内更新 `file_assets` 绑定
5. 递减旧 `blob_object.reference_count`
6. 若旧对象无引用，由异步 GC 删除

说明：

- 访问级别变更不要直接就地“修改 bucket 语义”
- bucket 是物理隔离边界，应明确 copy

## 13. 错误模型

建议在 `pkg/storage/errors.go` 里定义 canonical 错误类型：

```go
var (
    ErrObjectNotFound         = errors.New("storage: object not found")
    ErrMultipartNotFound      = errors.New("storage: multipart upload not found")
    ErrMultipartConflict      = errors.New("storage: multipart conflict")
    ErrPreconditionFailed     = errors.New("storage: precondition failed")
    ErrProviderUnavailable    = errors.New("storage: provider unavailable")
    ErrInvalidBucketConfig    = errors.New("storage: invalid bucket config")
    ErrInvalidPresignRequest  = errors.New("storage: invalid presign request")
)
```

要求：

- 业务层不要直接依赖 AWS SDK 原始异常
- adapter 负责把 provider-specific error 映射到 canonical error

## 14. 配置设计

推荐配置结构：

```yaml
storage:
  provider: s3
  s3:
    endpoint: http://minio:9000
    publicEndpoint: http://localhost:9000
    region: us-east-1
    accessKey: xxx
    secretKey: xxx
    pathStyleAccess: true
    defaultBucket: platform-files
    publicBucket: platform-files-public
    privateBucket: platform-files-private
    cdnDomain: ""
  presign:
    putObjectTTL: 15m
    uploadPartTTL: 15m
    getObjectTTL: 5m
  multipart:
    minPartSizeBytes: 5242880
    maxPartSizeBytes: 67108864
    maxParts: 10000
```

## 15. 测试要求

至少覆盖以下测试：

1. `PUBLIC` / `PRIVATE` bucket 解析正确
2. object key sanitize 正确
3. `CreateMultipartUpload -> ListParts -> Complete` 正常链路
4. presigned put URL 可正常上传对象
5. presigned upload part URL 可正常上传分片
6. presigned get URL 可正常读取私有对象
7. copy object 后目标对象存在
8. delete object 幂等
9. multipart abort 幂等
10. provider 错误能映射成 canonical error

推荐测试层次：

- 单元测试：mock storage 接口
- 集成测试：真实 MinIO
- 协议测试：覆盖 multipart 和 presign 行为

## 16. 最终建议

这份抽象设计的核心不在“把 S3 SDK 包起来”，而在三条边界：

1. 存储抽象是共享模块，不是独立服务
2. 抽象按能力拆分，不做大一统接口
3. multipart 和 presign 的权威语义必须在抽象层固定下来

这样 Go 版的 `upload-service`、`access-service` 和 `job-service` 才能共享同一套对象存储规范，同时保持各自代码边界清晰。
