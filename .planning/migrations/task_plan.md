# migrations task plan

## Goal

建立 canonical schema、迁移执行链路和环境 bootstrap 入口，保证所有服务实现都建立在同一套数据库事实之上。

## Inputs

- `docs/data-model-spec.md`
- `docs/migration-bootstrap-spec.md`
- `docs/data-protection-recovery-spec.md`
- `docs/development-workflow-spec.md`

## Phases

### Phase 1

- 固化 `migrations/` 目录结构
- 固化 migration runner 约定

### Phase 2

- 落 tenant/file/upload/audit/infra schema
- 建立最小可用索引和约束

### Phase 3

- 打通 db create / migrate up / reset / bucket init / seed

### Phase 4

- 验证迁移顺序、幂等和本地 reset
- 验证与数据保护文档一致

## Deliverables

- `migrations/`
- 迁移执行命令或脚本
- 本地 bootstrap 链路

## Exit Criteria

- 新环境可以从零建库并升级到最新版本
- schema 与 `data-model-spec.md` 一致
- 服务启动不承担 migration 责任
