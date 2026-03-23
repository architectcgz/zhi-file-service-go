# zhi-file-service-go Docs

文档按职责分为六个目录，建议按以下顺序阅读。

---

## 架构与设计 `architecture/`

系统整体设计决策与数据模型，适合初次了解系统的开发者首先阅读。

| 文档 | 说明 |
|------|------|
| [architecture-style-decision.md](architecture/architecture-style-decision.md) | Clean-ish + DDD-lite + CQRS-lite 架构风格决策 |
| [architecture-upgrade-design.md](architecture/architecture-upgrade-design.md) | Go 重写版 file-service 的架构升级设计 |
| [service-layout-spec.md](architecture/service-layout-spec.md) | monorepo 包结构与服务装配规范 |
| [data-model-spec.md](architecture/data-model-spec.md) | 数据模型与表设计 |
| [storage-abstraction-spec.md](architecture/storage-abstraction-spec.md) | 对象存储抽象与 multipart / presign 规范 |

---

## 服务实现规范 `services/`

各微服务的内部实现细节与核心设计，开发对应服务时必读。

| 文档 | 说明 |
|------|------|
| [upload-service-implementation-spec.md](services/upload-service-implementation-spec.md) | upload-service 实现细节 |
| [upload-session-state-machine-spec.md](services/upload-session-state-machine-spec.md) | UploadSession 状态机设计 |
| [upload-integrity-hash-spec.md](services/upload-integrity-hash-spec.md) | contentHash、SHA256 验证、dedup 与秒传契约 |
| [access-service-implementation-spec.md](services/access-service-implementation-spec.md) | access-service 实现细节 |
| [admin-service-implementation-spec.md](services/admin-service-implementation-spec.md) | admin-service 实现细节 |
| [job-service-implementation-spec.md](services/job-service-implementation-spec.md) | job-service 实现细节 |

---

## API 与契约 `api/`

对外接口规范、错误码、事件契约与鉴权模型。

| 文档 | 说明 |
|------|------|
| [api-design-spec.md](api/api-design-spec.md) | API 体系、路径规范与 OpenAPI 落地方向 |
| [openapi-contract-spec.md](api/openapi-contract-spec.md) | OpenAPI 正式外部契约规范 |
| [error-code-registry.md](api/error-code-registry.md) | 公共错误码、HTTP status 映射与扩展规则 |
| [outbox-event-spec.md](api/outbox-event-spec.md) | outbox event_type、payload 与消费幂等契约 |
| [data-plane-auth-context-spec.md](api/data-plane-auth-context-spec.md) | 数据面 Bearer Token、AuthContext 契约 |
| [admin-auth-spec.md](api/admin-auth-spec.md) | 管理面角色、tenant scope、审计要求 |

---

## 接入与集成 `integration/`

业务系统如何接入文件服务，包含示例代码与前端对接规范。

| 文档 | 说明 |
|------|------|
| [business-integration-spec.md](integration/business-integration-spec.md) | 该存什么、不该存什么，fileId / URL / ticket 接入约定 |
| [business-integration-examples.md](integration/business-integration-examples.md) | 公开头像、私有附件、私有预览三类标准接法示例 |
| [frontend-upload-progress-spec.md](integration/frontend-upload-progress-spec.md) | 前端上传进度口径、前后端职责划分与三种上传模式接法 |

---

## 运维与部署 `ops/`

部署、配置、数据库迁移、可观测性与数据保护规范。

| 文档 | 说明 |
|------|------|
| [deployment-runtime-spec.md](ops/deployment-runtime-spec.md) | Kubernetes 部署、健康检查、优雅关闭规范 |
| [configuration-registry-spec.md](ops/configuration-registry-spec.md) | 统一配置键、环境变量、默认值与 Secret 约束 |
| [migration-bootstrap-spec.md](ops/migration-bootstrap-spec.md) | 初始化建表、schema 演进与环境 bootstrap 规范 |
| [slo-observability-spec.md](ops/slo-observability-spec.md) | SLO、Prometheus 指标、Grafana 仪表盘、日志与告警规范 |
| [data-protection-recovery-spec.md](ops/data-protection-recovery-spec.md) | 数据库与对象存储的数据保护、误删恢复与演练规范 |

---

## 开发规范 `dev/`

代码风格、开发工作流与测试规范，所有贡献者必读。

| 文档 | 说明 |
|------|------|
| [code-style-guide.md](dev/code-style-guide.md) | 代码风格、分层约束、命名规范与禁止事项 |
| [development-workflow-spec.md](dev/development-workflow-spec.md) | 开发启动、命令约定与本地环境配置 |
| [test-validation-spec.md](dev/test-validation-spec.md) | 单测、集成测试、契约测试、E2E 与压测准入规范 |

---

## 相关契约文件

- `api/openapi/upload-service.yaml` — 上传服务 OpenAPI 正式契约
- `api/openapi/access-service.yaml` — 访问服务 OpenAPI 正式契约
- `api/openapi/admin-service.yaml` — 管理服务 OpenAPI 正式契约
