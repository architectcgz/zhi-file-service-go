# zhi-file-service-go 部署与运行时规范文档

## 1. 目标

这份文档定义 `zhi-file-service-go` 的部署与运行时规范。

它解决的问题：

1. 四个服务在 Kubernetes 中怎么部署
2. 配置如何分层与注入
3. readiness / liveness / startup probe 怎么定义
4. 各服务默认资源规格、扩缩容和滚动发布怎么设

配套文档：

- [architecture-upgrade-design.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/architecture-upgrade-design.md)
- [service-layout-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/service-layout-spec.md)
- [job-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/job-service-implementation-spec.md)
- [slo-observability-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/slo-observability-spec.md)
- [configuration-registry-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/configuration-registry-spec.md)
- [data-protection-recovery-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/data-protection-recovery-spec.md)

## 2. 核心结论

### 2.1 部署单元

第一阶段每个服务一个 Deployment：

- `upload-service`
- `access-service`
- `admin-service`
- `job-service`

不把：

- HTTP 服务
- worker
- cleanup scheduler

混在同一个进程里。

### 2.2 配置来源

统一采用：

- 环境变量
- Secret
- ConfigMap

不在容器内依赖本地配置文件作为主配置源。

### 2.3 运行时必须无状态

除了数据库、对象存储、Redis 这类外部依赖外，服务进程本身必须视为无状态。

这意味着：

- Pod 可随时重建
- 不把会话、票据、锁状态写本地磁盘
- 本地缓存只能作为加速层

## 3. 配置分层

推荐分四层：

1. `defaults`
2. `env`
3. `secret`
4. 启动参数覆盖

覆盖顺序从低到高。

## 3.1 通用配置

建议统一前缀：

- `APP_`
- `HTTP_`
- `DB_`
- `REDIS_`
- `STORAGE_`
- `OTEL_`

例如：

- `APP_ENV`
- `APP_SERVICE_NAME`
- `HTTP_PORT`
- `DB_DSN`
- `REDIS_ADDR`
- `STORAGE_ENDPOINT`

## 3.2 服务私有配置

例如：

- `UPLOAD_MAX_INLINE_SIZE`
- `ACCESS_TICKET_TTL`
- `ADMIN_DELETE_REQUIRES_REASON`
- `JOB_LOCK_TTL`

## 3.3 Secret 规范

必须放 Secret 的内容：

- `DB_DSN`
- `REDIS_PASSWORD`
- `STORAGE_ACCESS_KEY`
- `STORAGE_SECRET_KEY`
- `ACCESS_TICKET_SIGNING_KEY`
- `ADMIN_AUTH_JWKS` 或等价认证密钥配置

禁止把这些值写入：

- Helm values 明文
- Git 仓库
- 普通 ConfigMap

## 4. 启动顺序

## 4.1 启动流程

所有服务统一遵循：

1. 读取配置
2. 初始化 logger / metrics / trace
3. 初始化数据库连接池
4. 初始化 Redis / 对象存储 client
5. 注册 HTTP handler 或 job scheduler
6. 开始接收流量

只有在依赖成功初始化后才能进入 ready。

## 4.2 依赖失败策略

默认：

- PostgreSQL 初始化失败：启动失败
- 对象存储初始化失败：启动失败
- Redis 初始化失败：若该服务 Redis 为可选依赖，可降级启动，但必须打 `WARN`

## 5. Health Probes

第一阶段统一约定：

- `GET /live`：liveness probe 入口
- `GET /ready`：startup / readiness probe 入口
- `GET /metrics`：Prometheus 抓取入口
- 三者默认暴露在同一个服务 HTTP 端口，不单独拆管理端口

## 5.1 `startupProbe`

目的：

- 防止慢启动服务在依赖初始化期间被过早杀死

建议：

- 默认探测 `GET /ready`
- 检查配置已加载
- 检查关键依赖初始化已完成

## 5.2 `readinessProbe`

ready 的最小条件：

- `GET /ready` 已可成功返回
- HTTP server 已监听
- 数据库连接正常
- 对象存储 client 可用
- 服务自身关键初始化已完成

对 `job-service`：

- 即使未拿到分布式锁，也可以 ready
- 因为锁影响的是调度，不影响进程健康

## 5.3 `livenessProbe`

liveness 只判断“进程是否卡死”，不做重依赖探测。

默认探测：

- `GET /live`

禁止：

- liveness 每次都探测数据库
- liveness 每次都探测对象存储

否则外部依赖抖动会把 Pod 不停打死。

## 6. 默认资源规格

第一阶段建议起点：

### 6.1 `access-service`

- requests: `cpu 250m / memory 256Mi`
- limits: `cpu 1000m / memory 512Mi`

### 6.2 `upload-service`

- requests: `cpu 500m / memory 512Mi`
- limits: `cpu 2000m / memory 1Gi`

### 6.3 `admin-service`

- requests: `cpu 200m / memory 256Mi`
- limits: `cpu 1000m / memory 512Mi`

### 6.4 `job-service`

- requests: `cpu 300m / memory 256Mi`
- limits: `cpu 1500m / memory 1Gi`

这些只是起始值，最终以压测和线上指标调优。

## 7. 副本与扩缩容

## 7.1 初始副本建议

- `access-service`: `2`
- `upload-service`: `2`
- `admin-service`: `2`
- `job-service`: `2`

`job-service` 可以多副本，但调度必须配合分布式锁。

## 7.2 HPA 建议

推荐基于：

- CPU
- Memory
- 自定义业务指标

### `access-service`

优先按：

- CPU
- `http_request_duration_seconds`

### `upload-service`

优先按：

- CPU
- active upload session 数
- complete 延迟

### `admin-service`

优先按：

- CPU
- 请求数

### `job-service`

不建议第一阶段直接用 HPA 按 outbox backlog 激进扩容。

原因：

- job 多副本并不天然提升有效吞吐
- 还要受分布式锁和下游对象存储限速约束

## 8. 滚动发布

推荐 Deployment 策略：

- `maxUnavailable: 0`
- `maxSurge: 1`

原因：

- 避免读写服务发布时瞬时容量下降

## 8.1 发布前检查

至少检查：

1. migration 已完成
2. 关键环境变量齐全
3. OpenAPI 契约变更已同步
4. metrics / trace 正常
5. readiness 成功

## 8.2 发布后检查

至少观察：

- 5xx 错误率
- p95 延迟
- Pod 重启数
- outbox backlog
- job lock 冲突

## 9. PDB 与优雅停机

## 9.1 PodDisruptionBudget

建议：

- `access-service`、`upload-service`、`admin-service` 都配置 PDB
- `minAvailable: 1`

`job-service` 也建议配置，但不要求和数据面完全相同。

## 9.2 优雅停机

所有服务必须支持：

- 接收 `SIGTERM`
- 停止接收新请求
- 等待在途请求完成
- 关闭连接池
- flush 日志和 trace

`job-service` 额外要求：

- 释放分布式锁
- 停止续租
- 尽量让当前任务安全收尾

## 10. 网络与安全

建议：

- 只暴露必要 Service
- 控制面对内网开放
- 数据面按网关接入
- 数据库和对象存储仅允许白名单访问

服务间调用如果存在，默认：

- mTLS 或内部签名

## 11. 目录落位建议

推荐：

```text
deployments/
  helm/
    upload-service/
    access-service/
    admin-service/
    job-service/
  kustomize/
    base/
    overlays/dev/
    overlays/test/
    overlays/prod/
```

原则：

- base 放通用模板
- overlay 放环境差异
- Secret 不直接进 Git

## 12. 最终结论

部署规范的目标不是“先跑起来”，而是：

- 能多副本稳定运行
- 能滚动发布
- 能观测
- 能在依赖波动和扩容时保持可控

如果不先把运行时规范固定，后面真正接 K8s、压测和上线时会集中返工。
