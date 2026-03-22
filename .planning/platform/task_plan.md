# platform task plan

## Goal

构建 `internal/platform`，统一四个服务的配置、启动、HTTP server、日志、metrics、trace、依赖初始化和进程生命周期。

## Inputs

- `CODEX.md`
- `docs/service-layout-spec.md`
- `docs/deployment-runtime-spec.md`
- `docs/slo-observability-spec.md`
- `docs/configuration-registry-spec.md`

## Phases

### Phase 1

- 固化 `internal/platform` 包边界
- 固化服务 bootstrap 模型
- 固化统一 probe / metrics 暴露方案

### Phase 2

- 实现 config / logger / observability 基础能力
- 实现 HTTP server / middleware 骨架
- 实现 PostgreSQL / Redis / storage client 初始化骨架

### Phase 3

- 为四个 `cmd/<service>` 提供统一装配模板
- 固化 startup / readiness / liveness / shutdown 行为

### Phase 4

- 提供 testkit 和开发辅助
- 让后续服务测试直接复用平台层能力

## Deliverables

- `internal/platform/bootstrap`
- `internal/platform/config`
- `internal/platform/httpserver`
- `internal/platform/middleware`
- `internal/platform/observability`
- `internal/platform/persistence`
- `internal/platform/redis`
- `internal/platform/testkit`

## Exit Criteria

- 四个服务都能基于同一套平台骨架启动
- probe、metrics、日志、trace 行为一致
- 平台层不承载业务规则
