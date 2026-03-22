# zhi-file-service-go Docs

当前文档：

- [architecture-upgrade-design.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/architecture-upgrade-design.md): Go 重写版 `file-service` 的架构升级设计文档
- [architecture-style-decision.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/architecture-style-decision.md): `Clean-ish + DDD-lite + CQRS-lite` 架构风格决策
- [api-design-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/api-design-spec.md): 新 API 体系、路径规范与 OpenAPI 落地方向
- [business-integration-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/business-integration-spec.md): 业务侧该存什么、不该存什么，以及 `fileId` / URL / ticket 的接入约定
- [business-integration-examples.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/business-integration-examples.md): 公开头像、私有附件、私有预览三类标准业务接法示例
- [frontend-upload-progress-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/frontend-upload-progress-spec.md): 前端上传进度口径、前后端职责划分与三种上传模式接法
- [openapi-contract-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/openapi-contract-spec.md): OpenAPI 从骨架升级为正式外部契约的规范
- [data-plane-auth-context-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-plane-auth-context-spec.md): 数据面 Bearer Token、tenant_id / owner_id 映射与 AuthContext 契约
- [upload-integrity-hash-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-integrity-hash-spec.md): 上传 `contentHash`、SHA256 验证、dedup 与秒传契约
- [admin-auth-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-auth-spec.md): 管理面角色、tenant scope、destructive operation 审计要求
- [error-code-registry.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/error-code-registry.md): 公共错误码、HTTP status 映射与扩展规则
- [configuration-registry-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/configuration-registry-spec.md): 统一配置键、环境变量、默认值与 Secret 约束
- [outbox-event-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/outbox-event-spec.md): outbox `event_type`、payload 与消费幂等契约
- [slo-observability-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/slo-observability-spec.md): SLO、Prometheus 指标、Grafana 仪表盘、日志与告警规范
- [deployment-runtime-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/deployment-runtime-spec.md): Kubernetes 部署、配置分层、探针、资源规格与滚动发布规范
- [development-workflow-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/development-workflow-spec.md): 本地开发启动、Makefile 目标与脚本边界约定
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md): upload-service 模块划分、complete 链路和事务边界
- [access-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/access-service-implementation-spec.md): access-service 热路径、票据策略和下载跳转实现
- [admin-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/admin-service-implementation-spec.md): admin-service 租户治理、文件删除与审计实现
- [job-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/job-service-implementation-spec.md): job-service 调度模型、补偿任务与清理策略
- [code-style-guide.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/code-style-guide.md): Go 代码风格与工程约束
- [data-model-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-model-spec.md): PostgreSQL 表设计与索引规范
- [migration-bootstrap-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/migration-bootstrap-spec.md): 初始化建表、schema 演进与环境 bootstrap 规范
- [data-protection-recovery-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-protection-recovery-spec.md): 数据库与对象存储的数据保护、误删恢复与演练规范
- [upload-session-state-machine-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-session-state-machine-spec.md): UploadSession 状态机设计文档
- [storage-abstraction-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/storage-abstraction-spec.md): 对象存储抽象与 multipart / presign 规范
- [service-layout-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/service-layout-spec.md): monorepo 包结构与服务装配规范
- [test-validation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/test-validation-spec.md): 单测、集成测试、契约测试、E2E 与压测准入规范

实施计划：

- 仓库级实施计划统一维护在根目录 `.planning/`
- 每个模块一个目录，固定使用 `task_plan.md`、`findings.md`、`progress.md`

相关契约：

- `api/openapi/upload-service.yaml`: 上传服务 OpenAPI 正式契约
- `api/openapi/access-service.yaml`: 访问服务 OpenAPI 正式契约
- `api/openapi/admin-service.yaml`: 管理服务 OpenAPI 正式契约
