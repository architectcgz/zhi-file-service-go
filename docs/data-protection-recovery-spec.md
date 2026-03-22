# zhi-file-service-go 数据保护与恢复规范文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的数据保护、对象存储保护和恢复规范。

它解决的问题：

1. PostgreSQL 和对象存储要怎么做备份、恢复和误删保护
2. 文件逻辑删除、物理删除、对象版本保留如何形成闭环
3. 发布失败、误删、对象损坏、存储配置错误时怎么恢复
4. 对一个文件服务来说，哪些数据保护能力必须在架构阶段就写清楚

配套文档：

- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md)
- [storage-abstraction-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/storage-abstraction-spec.md)
- [job-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/job-service-implementation-spec.md)
- [deployment-runtime-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/deployment-runtime-spec.md)
- [migration-bootstrap-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/migration-bootstrap-spec.md)
- [admin-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-service-implementation-spec.md)

## 2. 核心结论

### 2.1 文件服务必须同时保护数据库事实和对象事实

本项目的关键数据不只有 PostgreSQL。

必须同时保护：

- PostgreSQL 中的租户、文件、上传、审计、outbox 数据
- 对象存储中的文件对象、multipart 中间对象、版本信息

只备份数据库而不考虑对象存储，不算完成的数据保护方案。

### 2.2 删除必须分层

删除固定分三层：

1. 控制面逻辑删除
2. 冷却期保留
3. `job-service` 物理删除

不要把管理员删除文件实现成“立刻删对象存储”。

### 2.3 生产环境必须具备恢复窗口

生产环境至少要有：

- PostgreSQL 备份
- 对象存储版本保留或等价恢复策略
- 明确的恢复流程
- 定期恢复演练

否则一次误删、错误 migration 或错误 cleanup 就足以造成不可逆损失。

### 2.4 数据保护不等于无限保留

必须同时定义：

- 恢复窗口
- 保留期
- 生命周期清理

否则要么无法恢复，要么存储成本和清理复杂度失控。

## 3. 保护对象分类

## 3.1 PostgreSQL

关键保护对象：

- `tenant.*`
- `file.*`
- `upload.*`
- `audit.*`
- `infra.outbox_events`

## 3.2 对象存储

关键保护对象：

- public bucket 对象
- private bucket 对象
- multipart 进行中的临时对象
- 复制或访问级别切换过程中的中间状态

## 3.3 派生与可重建数据

以下内容可视为可重建或可容忍短期丢失：

- 短 TTL 访问票据
- 临时缓存
- 本地进程缓存

不要为这些短期派生状态设计过重的保护机制。

## 4. 删除与保留策略

## 4.1 文件删除分层

固定流程：

1. `admin-service` 将 `file_assets` 标记为删除终态
2. 相关引用计数更新
3. 记录审计日志
4. 进入保留窗口
5. `job-service` 在窗口结束后复核并物理删除对象

### 保护要求

- 物理删除前必须再次检查 refcount
- 物理删除前必须再次检查文件是否仍处于删除终态
- 多实例 cleanup 必须走分布式锁

## 4.2 保留窗口

必须固定一个最小保留窗口。

第一阶段默认值固定为：

- `file delete retention`: `7d`

在该窗口内：

- 逻辑上不可访问
- 物理上暂不删除
- 允许人工恢复

如果后续需要调整该值，必须同步更新配置注册表、运行文档和清理任务实现。

## 4.3 上传中间态保留

对以下对象允许更短保留：

- 已过期 upload session
- multipart 未完成对象
- 已终态 `upload_session_parts`

但必须和状态机及 job 清理策略一致，不得直接拍脑袋缩短。

## 4.4 审计和 outbox 保留

固定要求：

- 审计日志保留期必须长于普通业务调试日志
- outbox 已发布记录必须按保留策略清理

清理策略要与排障窗口匹配。

## 5. PostgreSQL 备份与恢复

## 5.1 备份目标

生产环境必须至少满足：

- 每日全量备份
- 更高频率的 WAL / 增量归档
- 支持时间点恢复

这意味着 PostgreSQL 应具备 PITR 能力，而不是只有某个时间点快照。

## 5.2 恢复目标

必须至少定义以下目标：

- `RPO <= 15m`
- `RTO <= 2h`

如果当前环境暂时达不到，也必须把目标写明，不能处于无目标状态。

## 5.3 恢复场景

必须覆盖：

- 错误 migration
- 误删租户或文件元数据
- 数据库实例故障
- 某个 schema 被错误修改

## 5.4 恢复原则

原则：

- 先恢复到隔离环境验证
- 再决定部分恢复还是整库恢复
- 不允许直接在生产库上临场手改数据假装完成恢复

## 6. 对象存储保护

## 6.1 版本保留

生产环境最低要求：

- 对 private bucket 开启 object versioning 或等价能力
- 对 public bucket 具备明确、可演练的恢复方案

目标：

- 防误删
- 防错误 cleanup
- 支持人工回滚到前一版本

## 6.2 加密

最低要求：

- 对象传输必须走 TLS
- 服务端存储加密必须开启

推荐：

- S3 SSE-S3 作为最低基线
- 生产环境优先 SSE-KMS 或等价密钥管理方案

不要把“现在先不配”长期默认化。

## 6.3 Bucket 权限

要求：

- public bucket 只开放明确需要匿名访问的路径
- private bucket 不允许匿名访问
- 服务账号按最小权限分配

## 6.4 生命周期策略

不同对象必须区分生命周期：

- 正式文件对象：按业务保留
- multipart 中间对象：短周期清理
- 删除保留窗口内对象版本：按恢复窗口保留

不要对整个 bucket 粗暴套一个统一 TTL。

## 7. 恢复流程规范

## 7.1 误删文件恢复

推荐流程：

1. 根据审计日志和 file_id 确认删除动作
2. 检查 `file_assets` 是否仍在保留窗口内
3. 检查对象版本或物理对象是否仍存在
4. 恢复元数据状态或恢复对象版本
5. 校验访问链路、refcount、tenant_usage

## 7.2 错误 cleanup 恢复

若 `job-service` 错删对象：

1. 先停止对应 cleanup job
2. 确认受影响对象范围
3. 通过对象版本或备份恢复对象
4. 对账 `blob_objects` / `file_assets`
5. 修复后再恢复任务调度

## 7.3 错误 migration 恢复

若 migration 导致数据损坏：

1. 先冻结发布
2. 确认影响版本
3. 评估 forward-fix 还是数据库恢复
4. 在隔离环境演练恢复
5. 再进入生产修复

## 8. 演练与审计

## 8.1 恢复演练

生产环境最低要求：

- 每季度一次 PostgreSQL 恢复演练
- 每季度一次对象恢复演练
- 每次演练要有记录和结果

## 8.2 审计要求

以下动作必须可审计：

- 删除文件
- 恢复文件
- 手工删除对象
- 手工修复元数据
- 恢复数据库

## 9. 运行时检查项

发布前至少检查：

1. 备份任务正常
2. 最近一次备份成功
3. 对象版本或恢复策略已启用
4. cleanup 保留窗口配置正确
5. 恢复联系人与 runbook 明确

## 10. 禁止事项

以下做法默认禁止：

- 管理员删除文件时同步直删对象存储
- 没有 refcount 复核就物理删对象
- 没有恢复窗口就开启激进 cleanup
- 生产环境没有备份验证就发布重大 schema 变更
- 把对象存储当成“反正能重新传”的无状态缓存

## 11. Code Review 拦截项

看到以下实现应直接拦截：

- 删除链路没有保留窗口
- cleanup 任务没有再次校验 refcount 和删除终态
- 生产配置里没有任何对象恢复策略
- 恢复流程完全依赖手工临时 SQL
- 加密和传输保护未定义

## 12. 最终建议

这份文档的核心只有五条：

1. 文件服务必须同时保护 PostgreSQL 和对象存储事实
2. 删除必须分成逻辑删除、保留窗口、物理删除三层
3. 生产环境必须具备数据库备份和对象恢复能力
4. 数据保护必须和生命周期清理同时设计
5. 恢复流程和演练必须先写清楚，不能等事故发生后补

如果这层不先补齐，后面即使上传、访问、治理都写完了，仍然不能算一个工程上完整的文件服务。
