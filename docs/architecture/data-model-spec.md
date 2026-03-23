# zhi-file-service-go 数据模型与表设计文档

## 1. 目标

本设计文档定义 `zhi-file-service-go` 的 PostgreSQL 数据模型。

前提约束：

- 当前 `file-service` 仍处于测试阶段
- 不考虑沿用旧表，也不以平滑迁移旧表为前提
- 可以基于 Go 版架构重新定义 canonical schema

本设计的目标是：

- 为 Go 重写版提供清晰、稳定、可扩展的数据模型
- 支撑多服务架构下的高吞吐上传与访问
- 保证对象存储、文件元数据、租户配额、上传状态机的一致性
- 为后续 `sqlc + pgx` 这类偏显式 SQL 的实现方式提供良好基础

## 2. 关键结论

### 2.1 推荐数据库形态

推荐使用：

- 1 个 PostgreSQL 集群
- 1 个业务数据库，例如 `zhi_file_service`
- 多个逻辑 schema 分域，而不是每个服务独占一个数据库

推荐 schema 划分：

- `tenant`: 租户与配额
- `file`: 文件与物理对象
- `upload`: 上传会话与分片状态
- `audit`: 管理员审计
- `infra`: outbox 与内部基础设施表

原因：

- 上传 complete 链路需要在文件元数据、对象元数据、租户使用量之间维持强一致
- 若过早拆成多个数据库，会立刻引入分布式事务与补偿复杂度
- 逻辑 schema 分域已经足够表达表所有权与代码边界

### 2.2 推荐 canonical 表

推荐定义以下核心表：

- `tenant.tenants`
- `tenant.tenant_policies`
- `tenant.tenant_usage`
- `file.blob_objects`
- `file.file_assets`
- `upload.upload_sessions`
- `upload.upload_session_parts`
- `upload.upload_dedup_claims`
- `audit.admin_audit_logs`
- `infra.outbox_events`

### 2.3 不建议第一阶段引入的表

- 单独的 `storage_nodes` / `storage_service_registry`
- 单独的 `access_tickets` 持久化表
- 单独的 `background_jobs` 调度中心表

原因：

- access ticket 更适合放 Redis 或纯签名 token
- job-service 可以先基于主表扫描 + outbox 工作
- 过早抽象这些表不会提升核心上传和访问能力

## 3. 设计原则

### 3.1 文件逻辑引用与物理对象分离

必须区分：

- `file_assets`: 业务可见的“文件”
- `blob_objects`: 对象存储中的物理对象

这样才能稳定支持：

- 同租户内去重
- 引用计数
- 逻辑删除与物理清理分离

### 3.2 上传状态机单独建模

上传过程不是文件本身，而是独立的临时状态机。

因此：

- 上传过程放到 `upload_sessions`
- 分片观察快照、审计信息与幂等辅助数据放到 `upload_session_parts`
- 完成上传后才产生 `file_assets`

### 3.3 控制面和统计面分离

租户配置和租户使用量必须拆分：

- `tenant_policies`: 配额与规则
- `tenant_usage`: 聚合统计

否则高频更新的使用量会污染低频变更的配置数据。

### 3.4 热读路径优先

`access-service` 是高 QPS 读取路径。

因此 `file_assets` 要做读优化，允许存储对访问链路有价值的冗余字段，例如：

- `storage_provider`
- `bucket_name`
- `object_key`

这样按 `file_id` 读取时不必每次 join `blob_objects`。

## 4. 命名与通用约定

### 4.1 主键类型

推荐统一使用 `VARCHAR(26)` 的 `ULID` 或等价的时间有序字符串 ID。

原因：

- 有序写入更友好
- 便于日志排查
- 跨服务生成更简单

如：

- `tenant_id`
- `file_id`
- `blob_object_id`
- `upload_session_id`
- `event_id`
- `audit_log_id`

### 4.2 时间字段

统一使用 `TIMESTAMPTZ`。

推荐保留：

- `created_at`
- `updated_at`

按需要增加：

- `completed_at`
- `aborted_at`
- `failed_at`
- `deleted_at`
- `expires_at`

### 4.3 状态字段

状态字段统一使用大写字符串枚举，不使用魔法数字。

### 4.4 JSON 字段

仅在以下场景使用 `JSONB`：

- 审计详情
- outbox payload
- 少量扩展元数据

不把核心查询字段埋进 JSONB。

## 5. Schema 设计

## 5.1 `tenant.tenants`

用途：

- 表示租户身份与生命周期

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| tenant_id | VARCHAR(32) PK | 租户唯一标识，例如 `blog` |
| tenant_name | VARCHAR(128) NOT NULL | 展示名称 |
| status | VARCHAR(16) NOT NULL | `ACTIVE` / `SUSPENDED` / `DELETED` |
| contact_email | VARCHAR(255) | 联系邮箱 |
| description | VARCHAR(500) | 描述信息 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 更新时间 |
| deleted_at | TIMESTAMPTZ | 软删除时间 |

约束：

- `tenant_id` 作为业务主键
- `status` check constraint

索引：

- `idx_tenants_status(status)`
- `idx_tenants_created_at(created_at desc)`

说明：

- 此表只关心“是谁”和“当前状态”
- 不混入高频变化的配额统计

## 5.2 `tenant.tenant_policies`

用途：

- 存储租户上传与访问策略

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| tenant_id | VARCHAR(32) PK FK | 对应 `tenant.tenants.tenant_id` |
| max_storage_bytes | BIGINT NOT NULL | 存储配额 |
| max_file_count | BIGINT NOT NULL | 文件数配额 |
| max_single_file_size | BIGINT NOT NULL | 单文件大小限制 |
| allowed_mime_types | TEXT[] | 允许的 MIME 类型 |
| allowed_extensions | TEXT[] | 允许的扩展名 |
| default_access_level | VARCHAR(16) NOT NULL | `PUBLIC` / `PRIVATE` |
| auto_create_enabled | BOOLEAN NOT NULL DEFAULT FALSE | 是否允许自动创建租户 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 更新时间 |

索引：

- 主键已足够

说明：

- 与 `tenant_usage` 分离后，配置更新和使用量更新不会互相干扰
- 后续若要支持租户级 bucket 或 region 覆盖，也应加在这里

## 5.3 `tenant.tenant_usage`

用途：

- 存储租户聚合使用量

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| tenant_id | VARCHAR(32) PK FK | 对应租户 |
| used_storage_bytes | BIGINT NOT NULL DEFAULT 0 | 已用存储 |
| used_file_count | BIGINT NOT NULL DEFAULT 0 | 已用文件数 |
| last_upload_at | TIMESTAMPTZ | 最近一次上传时间 |
| version | BIGINT NOT NULL DEFAULT 0 | 乐观锁版本号 |
| updated_at | TIMESTAMPTZ NOT NULL | 更新时间 |

约束：

- 使用量字段必须 `>= 0`

索引：

- `idx_tenant_usage_updated_at(updated_at desc)`

说明：

- 此表是热点更新表
- 写入 complete 流程时按 `tenant_id` 行级更新

## 5.4 `file.blob_objects`

用途：

- 表示对象存储中的物理对象
- 用于去重与物理清理

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| blob_object_id | VARCHAR(26) PK | 物理对象 ID |
| tenant_id | VARCHAR(32) NOT NULL | 租户隔离边界 |
| storage_provider | VARCHAR(32) NOT NULL | `MINIO` / `S3` |
| bucket_name | VARCHAR(128) NOT NULL | bucket |
| object_key | VARCHAR(512) NOT NULL | 对象 key |
| hash_value | VARCHAR(128) NOT NULL | 文件哈希 |
| hash_algorithm | VARCHAR(16) NOT NULL | `MD5` / `SHA256` |
| file_size | BIGINT NOT NULL | 文件大小 |
| content_type | VARCHAR(255) | MIME 类型 |
| reference_count | INTEGER NOT NULL DEFAULT 0 | 引用计数 |
| storage_class | VARCHAR(32) | 存储等级 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 更新时间 |
| deleted_at | TIMESTAMPTZ | 逻辑删除时间 |

约束：

- `UNIQUE (tenant_id, storage_provider, bucket_name, object_key)`
- `UNIQUE (tenant_id, hash_algorithm, hash_value, bucket_name)`
- `reference_count >= 0`

索引：

- `idx_blob_objects_hash(tenant_id, hash_algorithm, hash_value, bucket_name)`
- `idx_blob_objects_ref_count(reference_count)`
- `idx_blob_objects_deleted_at(deleted_at)`

说明：

- 去重作用域建议仍然限定在同租户内
- 跨租户共享物理对象虽然更省空间，但隔离性、审计性和删除语义更差，不建议作为默认策略

## 5.5 `file.file_assets`

用途：

- 表示业务侧可见文件
- 面向读取路径做适当冗余

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| file_id | VARCHAR(26) PK | 文件 ID |
| tenant_id | VARCHAR(32) NOT NULL | 租户 ID |
| owner_id | VARCHAR(64) NOT NULL | 文件所有者 |
| blob_object_id | VARCHAR(26) NOT NULL | 指向物理对象 |
| original_filename | VARCHAR(255) NOT NULL | 原始文件名 |
| content_type | VARCHAR(255) | MIME 类型 |
| file_size | BIGINT NOT NULL | 文件大小 |
| access_level | VARCHAR(16) NOT NULL | `PUBLIC` / `PRIVATE` |
| status | VARCHAR(16) NOT NULL | `ACTIVE` / `DELETED` |
| storage_provider | VARCHAR(32) NOT NULL | 访问链路读优化字段 |
| bucket_name | VARCHAR(128) NOT NULL | 访问链路读优化字段 |
| object_key | VARCHAR(512) NOT NULL | 访问链路读优化字段 |
| file_hash | VARCHAR(128) | 文件哈希快照 |
| metadata | JSONB | 轻量扩展元数据 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 更新时间 |
| deleted_at | TIMESTAMPTZ | 删除时间 |

约束：

- `status` check constraint
- `access_level` check constraint

索引：

- `idx_file_assets_tenant_owner(tenant_id, owner_id, created_at desc)`
- `idx_file_assets_tenant_status(tenant_id, status, created_at desc)`
- `idx_file_assets_blob_object(blob_object_id)`
- `idx_file_assets_object_locator(tenant_id, bucket_name, object_key)`

说明：

- `file_assets` 故意冗余对象定位字段，这是为高频访问链路服务
- `blob_object_id` 仍然保留，用于引用计数和后台清理

## 5.6 `upload.upload_sessions`

用途：

- 统一表示所有上传流程
- 是 Go 版最核心的状态机表

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| upload_session_id | VARCHAR(26) PK | 会话 ID |
| tenant_id | VARCHAR(32) NOT NULL | 租户 ID |
| owner_id | VARCHAR(64) NOT NULL | 发起用户 |
| upload_mode | VARCHAR(32) NOT NULL | `INLINE` / `DIRECT` / `PRESIGNED_SINGLE` |
| target_access_level | VARCHAR(16) NOT NULL | 目标访问级别 |
| original_filename | VARCHAR(255) NOT NULL | 原始文件名 |
| content_type | VARCHAR(255) | MIME 类型 |
| expected_size | BIGINT NOT NULL | 预期大小 |
| file_hash | VARCHAR(128) | 文件哈希 |
| hash_algorithm | VARCHAR(16) | 哈希算法 |
| storage_provider | VARCHAR(32) NOT NULL | 存储提供商 |
| bucket_name | VARCHAR(128) NOT NULL | 目标 bucket |
| object_key | VARCHAR(512) | 目标 object key |
| provider_upload_id | VARCHAR(255) | multipart upload id |
| chunk_size_bytes | INTEGER NOT NULL DEFAULT 0 | 分片大小 |
| total_parts | INTEGER NOT NULL DEFAULT 1 | 总分片数 |
| completed_parts | INTEGER NOT NULL DEFAULT 0 | 已完成分片数 |
| file_id | VARCHAR(26) | 完成后生成的文件 ID |
| status | VARCHAR(16) NOT NULL | `INITIATED` / `UPLOADING` / `COMPLETING` / `COMPLETED` / `ABORTED` / `EXPIRED` / `FAILED` |
| failure_code | VARCHAR(64) | 失败码 |
| failure_message | VARCHAR(500) | 失败描述 |
| resumed_from_session_id | VARCHAR(26) | 可选，记录续传来源 |
| idempotency_key | VARCHAR(128) | 客户端幂等键 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 更新时间 |
| completed_at | TIMESTAMPTZ | 完成时间 |
| aborted_at | TIMESTAMPTZ | 中止时间 |
| failed_at | TIMESTAMPTZ | 失败时间 |
| expires_at | TIMESTAMPTZ NOT NULL | 过期时间 |

关键约束：

- `expected_size > 0`
- `total_parts >= 1`
- `completed_parts >= 0`
- `status` 为终态时，终态时间字段应可追溯

关键索引：

- `idx_upload_sessions_owner_active(tenant_id, owner_id, status, created_at desc)`
- `idx_upload_sessions_expires_at(expires_at)` with partial filter for non-terminal sessions
- `idx_upload_sessions_file_hash(tenant_id, owner_id, file_hash, expected_size, status)`
- `idx_upload_sessions_file_id(file_id)`

推荐唯一约束：

- 不做复杂全局唯一约束来“硬防”所有重复会话
- 续传匹配由应用层按 `tenant_id + owner_id + upload_mode + access_level + expected_size + file_hash + active status` 选择最近活跃会话

说明：

- 这张表替代旧设计里不够统一的 `upload_tasks`
- 所有上传协议最终都映射到这一套状态机

## 5.7 `upload.upload_session_parts`

用途：

- 保存服务侧观察到的 multipart 已确认分片快照

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| upload_session_id | VARCHAR(26) NOT NULL FK | 对应上传会话 |
| part_number | INTEGER NOT NULL | 分片号 |
| etag | VARCHAR(255) NOT NULL | 对象存储返回的 etag |
| part_size | BIGINT NOT NULL | 分片大小 |
| checksum | VARCHAR(255) | 可选校验和 |
| uploaded_at | TIMESTAMPTZ NOT NULL | 上传确认时间 |

主键：

- `PRIMARY KEY (upload_session_id, part_number)`

索引：

- `idx_upload_session_parts_uploaded_at(uploaded_at desc)`

说明：

- `completed_parts` 是 `upload_sessions` 的聚合快照
- 对象存储 `list parts` 才是 authoritative part list
- `upload_session_parts` 只保存服务侧观察值、审计信息和幂等辅助数据，不替代 provider 真相

## 5.8 `upload.upload_dedup_claims`

用途：

- 控制同一租户、同一哈希、同一 bucket 的并发上传抢占

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| tenant_id | VARCHAR(32) NOT NULL | 租户 |
| hash_algorithm | VARCHAR(16) NOT NULL | 哈希算法 |
| hash_value | VARCHAR(128) NOT NULL | 哈希值 |
| bucket_name | VARCHAR(128) NOT NULL | bucket |
| owner_token | VARCHAR(64) NOT NULL | 当前 claim 持有者 |
| expires_at | TIMESTAMPTZ NOT NULL | claim 过期时间 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 更新时间 |

主键：

- `PRIMARY KEY (tenant_id, hash_algorithm, hash_value, bucket_name)`

索引：

- `idx_upload_dedup_claims_expires_at(expires_at)`

说明：

- 这是一个并发控制表，不是业务事实表
- job-service 定期清理过期 claim

## 5.9 `audit.admin_audit_logs`

用途：

- 记录所有管理员控制面操作

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| audit_log_id | VARCHAR(26) PK | 审计日志 ID |
| admin_subject | VARCHAR(128) NOT NULL | 管理员标识 |
| action | VARCHAR(64) NOT NULL | 操作类型 |
| target_type | VARCHAR(64) NOT NULL | 目标对象类型 |
| target_id | VARCHAR(64) | 目标对象 ID |
| tenant_id | VARCHAR(32) | 涉及租户 |
| request_id | VARCHAR(64) | 请求链路 ID |
| ip_address | VARCHAR(64) | IP 地址 |
| user_agent | VARCHAR(255) | 可选 UA |
| details | JSONB | 详细审计信息 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |

索引：

- `idx_admin_audit_logs_admin(admin_subject, created_at desc)`
- `idx_admin_audit_logs_action(action, created_at desc)`
- `idx_admin_audit_logs_tenant(tenant_id, created_at desc)`
- `idx_admin_audit_logs_target(target_type, target_id, created_at desc)`

说明：

- 这张表是 append-only
- 未来若量很大，可以按月分区

## 5.10 `infra.outbox_events`

用途：

- 承载事务内事件，供 job-service 或异步发布器消费

建议字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| event_id | VARCHAR(26) PK | 事件 ID |
| service_name | VARCHAR(64) NOT NULL | 事件来源服务 |
| aggregate_type | VARCHAR(64) NOT NULL | 聚合类型 |
| aggregate_id | VARCHAR(64) NOT NULL | 聚合 ID |
| event_type | VARCHAR(64) NOT NULL | 事件类型 |
| payload | JSONB NOT NULL | 事件内容 |
| status | VARCHAR(16) NOT NULL | `PENDING` / `PUBLISHED` / `FAILED` |
| retry_count | INTEGER NOT NULL DEFAULT 0 | 重试次数 |
| next_attempt_at | TIMESTAMPTZ | 下次重试时间 |
| published_at | TIMESTAMPTZ | 已发布时间 |
| created_at | TIMESTAMPTZ NOT NULL | 创建时间 |
| updated_at | TIMESTAMPTZ NOT NULL | 更新时间 |

索引：

- `idx_outbox_events_status_next(status, next_attempt_at, created_at)`
- `idx_outbox_events_aggregate(aggregate_type, aggregate_id)`

说明：

- 这是系统异步一致性的基础表
- 例如上传完成后触发缩略图、审计、统计刷新，都可以通过它派发

## 6. 推荐枚举值

### 6.1 tenant status

- `ACTIVE`
- `SUSPENDED`
- `DELETED`

### 6.2 file asset status

- `ACTIVE`
- `DELETED`

### 6.3 access level

- `PUBLIC`
- `PRIVATE`

### 6.4 upload mode

- `INLINE`
- `DIRECT`
- `PRESIGNED_SINGLE`

### 6.5 upload session status

- `INITIATED`
- `UPLOADING`
- `COMPLETING`
- `COMPLETED`
- `ABORTED`
- `EXPIRED`
- `FAILED`

### 6.6 outbox status

- `PENDING`
- `PUBLISHED`
- `FAILED`

## 7. 推荐外键关系

建议保留以下强外键：

- `tenant.tenant_policies.tenant_id -> tenant.tenants.tenant_id`
- `tenant.tenant_usage.tenant_id -> tenant.tenants.tenant_id`
- `file.file_assets.blob_object_id -> file.blob_objects.blob_object_id`
- `upload.upload_session_parts.upload_session_id -> upload.upload_sessions.upload_session_id`

对以下关系建议逻辑约束而不是强外键：

- `upload.upload_sessions.file_id -> file.file_assets.file_id`

原因：

- upload complete 是状态机终态写入，避免因为复杂写入顺序让强外键成为耦合点

## 8. 事务与写入建议

### 8.1 上传完成写事务

`upload-service` 的 `complete` 流程固定为三阶段：

1. 阶段 A：短事务获取 `complete` 所有权
2. 阶段 B：事务外读取 provider authoritative parts / object 事实并完成对象侧动作
3. 阶段 C：短事务提交元数据

这里的数据库事务只覆盖阶段 C，必须避免把对象存储 complete / head 之类的外部 I/O 放进锁区间。

阶段 C 事务内建议执行：

1. 再次锁定 `upload_sessions` 并确认 `completion_token`
2. 校验阶段 B 已确认的对象事实与当前 session 仍一致，必要时对齐 `upload_session_parts`
3. 插入 `file.blob_objects` 或复用已有对象
4. 插入 `file.file_assets`
5. 更新 `tenant.tenant_usage`
6. 更新 `upload.upload_sessions.status=COMPLETED`
7. 写入 `infra.outbox_events`

### 8.2 删除文件事务

建议流程：

1. 逻辑删除 `file_assets`
2. 递减 `blob_objects.reference_count`
3. 递减 `tenant_usage`
4. 写入 outbox
5. 真正删除对象由 job-service 异步完成

## 9. 索引与性能建议

### 9.1 高优先级索引

第一批必须落地的索引：

- `file_assets(file_id)` 主键
- `file_assets(tenant_id, owner_id, created_at desc)`
- `file_assets(tenant_id, status, created_at desc)`
- `blob_objects(tenant_id, hash_algorithm, hash_value, bucket_name)`
- `upload_sessions(tenant_id, owner_id, status, created_at desc)`
- `upload_sessions(expires_at)` partial
- `upload_session_parts(upload_session_id, part_number)` 主键
- `tenant_usage(tenant_id)` 主键

### 9.2 不建议第一阶段上的索引

- 过多 JSONB GIN 索引
- 对低频字段的组合超级索引
- 为假想查询场景预建的宽索引

原则：

- 先覆盖核心读写路径
- 通过真实压测和慢 SQL 再补二级索引

## 10. 分区建议

第一阶段建议：

- 大部分表不分区
- `audit.admin_audit_logs` 可预留后续按月分区
- `infra.outbox_events` 若事件量大，可按时间归档

原因：

- 过早分区会增加实现和维护复杂度
- 当前系统仍在重写期，优先保证模型清晰

## 11. 删除与保留策略

### 11.1 软删除

建议软删除：

- `tenant.tenants`
- `file.file_assets`
- `file.blob_objects`

### 11.2 TTL / 定期清理

建议定期清理：

- `upload.upload_sessions` 中已过期终态会话
- `upload.upload_session_parts` 中已终态并超过保留期的记录
- `upload.upload_dedup_claims` 中过期 claim
- `infra.outbox_events` 中已发布且超过保留期的数据

## 12. DDL 风格建议

实现时建议：

- 使用 `golang-migrate`
- 每张表单独 migration 文件或按 schema 分组
- 显式写 constraint name 和 index name
- 避免把业务注释放进代码里，优先 `COMMENT ON`

## 13. 最终推荐

如果只保留一句结论，这份表设计的核心是：

1. 一个 PostgreSQL 数据库，多个逻辑 schema 分域
2. `blob_objects + file_assets` 负责对象与文件分离
3. `upload_sessions + upload_session_parts` 负责统一上传状态机
4. `tenant_policies + tenant_usage` 分离配置与统计
5. `outbox_events` 作为异步一致性基石

这套模型比现有测试版更适合 Go 多服务架构，也更适合后续按吞吐和稳定性目标做持续优化。
