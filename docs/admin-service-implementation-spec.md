# zhi-file-service-go admin-service 实现细节文档

## 1. 目标

这份文档定义 `admin-service` 的实现细节，核心是固定控制面的写法，避免后期把后台治理重新混进数据面。

它解决的问题：

1. 租户、策略、文件治理如何组织到一个服务中
2. 管理员写操作的事务与审计怎么落
3. 文件删除为何必须走“逻辑删除 + 后台物理清理”

配套文档：

- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md)
- [admin-auth-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-auth-spec.md)
- [outbox-event-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/outbox-event-spec.md)
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)
- [service-layout-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/service-layout-spec.md)

## 2. 服务职责

明确职责：

- 租户创建、查询、更新
- 租户策略查询与修改
- 租户使用量查询
- 后台文件查询
- 后台文件删除
- 管理员审计日志查询

明确不负责：

- 上传
- 文件访问跳转
- 对象物理清理执行
- 长时间后台扫描任务

## 3. 服务内包结构

推荐结构：

```text
internal/services/admin/
  domain/
    tenant.go
    tenant_policy.go
    admin_file.go
    audit_log.go
    errors.go
  app/
    commands/
      create_tenant.go
      patch_tenant.go
      patch_tenant_policy.go
      delete_file.go
    queries/
      list_tenants.go
      get_tenant.go
      get_tenant_policy.go
      get_tenant_usage.go
      list_files.go
      get_file.go
      list_audit_logs.go
  ports/
    tenant_repository.go
    tenant_policy_repository.go
    tenant_usage_query.go
    admin_file_repository.go
    audit_log_repository.go
    outbox_publisher.go
    tx_manager.go
  transport/http/
    handler.go
    request.go
    response.go
  infra/
    postgres/
    outbox/
```

## 4. 核心实现原则

## 4.1 控制面允许更重，但不能混乱

`admin-service` 可以接受：

- 更复杂查询
- 更多筛选条件
- 更多治理规则

但不能接受：

- 在 handler 里堆业务逻辑
- 写操作无审计
- 删除文件时直接同步删对象存储

## 4.2 审计与业务变更同事务

凡是管理员写操作，默认要求：

- 业务变更成功
- 审计日志写入成功

二者必须同事务完成。

## 5. 核心用例实现

## 5.1 `CreateTenant`

流程：

1. 校验 `tenant_id` 唯一
2. 插入 `tenant.tenants`
3. 初始化 `tenant.tenant_policies`
4. 初始化 `tenant.tenant_usage`
5. 写入审计日志

全部放在同一事务内。

## 5.2 `PatchTenant`

允许更新：

- `tenant_name`
- `status`
- `contact_email`
- `description`

约束：

- `tenant_id` 不可变
- 冻结租户只影响后续上传和访问策略，不立即篡改历史文件元数据
- 当 `status` 变更为 `SUSPENDED`、`DELETED` 等 destructive 状态时，必须要求 `reason`
- destructive 状态变更必须与审计日志同事务提交

## 5.3 `PatchTenantPolicy`

流程：

1. 锁定租户策略行
2. 更新配额与白名单规则
3. 写审计日志

约束：

- 不在这里直接回写历史文件
- 政策变更对新请求生效，由读路径和写路径在运行时读取
- 当策略收紧具有 destructive 影响时，必须要求 `reason`
- `reason` 必须进入审计日志 `details`

## 5.4 `GetTenantUsage`

第一阶段直接查询 `tenant_usage` 聚合表。

不允许每次请求都临时扫描：

- `file_assets`
- `blob_objects`

做现场统计。

## 5.5 `ListFiles / GetFile`

查询基于后台投影字段，重点支持：

- tenant 过滤
- 状态过滤
- 创建时间倒序
- cursor pagination

查询实现目标：

- 后台可查
- SQL 可控
- 不回退成 ORM 动态拼装黑箱

## 5.6 `DeleteFile`

这是控制面最重要的写用例之一。

必须固定为：

### 阶段 A：逻辑删除事务

事务内执行：

1. 锁定 `file_assets`
2. 若已删除，直接返回幂等成功
3. 标记 `status=DELETED` / `deleted_at`
4. 对关联 `blob_object` 执行引用计数减一
5. 更新 `tenant_usage`
6. 写入审计日志
7. 写入 outbox 事件
8. 提交事务

### 阶段 B：后台物理清理

由 `job-service` 异步执行：

1. 读取 outbox 或清理任务
2. 只处理 `deleted_at + job.file_delete_retention <= now` 的记录
3. 若 blob 引用计数为 0，则删除对象存储对象
4. 标记物理清理结果

禁止：

- 管理接口同步删对象存储
- 在 HTTP 请求里等待大对象删除完成
- 删除文件但不更新引用计数和 usage

## 6. Repository 约束

建议拆分：

- `tenant_commands.sql`
- `tenant_queries.sql`
- `tenant_policy_commands.sql`
- `tenant_usage_queries.sql`
- `admin_file_commands.sql`
- `admin_file_queries.sql`
- `audit_log_queries.sql`

原则：

- 查询与命令分离
- 后台列表查询允许专用 projection SQL
- 审计写入通过 repository 明确封装，不散落在各 use case

## 7. 鉴权与权限

第一阶段默认：

- `admin-service` 只接入后台身份体系
- 不复用数据面用户令牌

至少区分：

- 只读管理员
- 治理管理员
- 超级管理员

权限判定放在：

- transport 层做基础身份校验
- app 层做资源级授权判断

## 8. 配置项

建议至少提供：

- `admin.list_default_limit`
- `admin.list_max_limit`
- `admin.delete_requires_reason`

说明：

- 审计是强约束，不提供关闭开关

## 9. 可观测性

关键指标：

- `admin_tenant_create_total`
- `admin_tenant_patch_total`
- `admin_policy_patch_total`
- `admin_file_delete_total`
- `admin_audit_query_total`

关键日志字段：

- `admin_id`
- `tenant_id`
- `file_id`
- `action`
- `resource_type`

关键 trace span：

- `admin.create_tenant`
- `admin.patch_tenant`
- `admin.patch_policy`
- `admin.delete_file`

## 10. 测试要求

必须覆盖：

- tenant 创建事务完整性
- policy 修改与审计写入
- 删除文件幂等
- 删除文件后 usage 和 refcount 变更
- 后台分页与筛选 SQL

## 11. Code Review 检查项

看到以下实现应直接拦截：

- 管理员删除文件时同步删对象存储
- 审计日志脱离主事务
- 后台查询直接复用热路径 handler DTO
- 用 upload-service RPC 获取后台查询数据
