# admin-service task plan

## Goal

落租户治理、策略修改、使用量查询、后台文件治理和审计查询，并把 destructive 审计约束真正落到实现里。

## Inputs

- `docs/admin-service-implementation-spec.md`
- `docs/admin-auth-spec.md`
- `docs/error-code-registry.md`
- `api/openapi/admin-service.yaml`

## Phases

### Phase 1

- 固化 `AdminContext`
- 固化角色矩阵、tenant scope、destructive reason 规则

### Phase 2

- 实现 tenant / policy / usage 相关接口

### Phase 3

- 实现后台文件查询、删除与审计查询

### Phase 4

- 收口审计、分页、筛选 SQL 和配置键

### Phase 5

- 补 destructive、删除幂等、usage/refcount、分页筛选测试

## Deliverables

- `internal/services/admin`
- 对应 transport / infra / tests

## Exit Criteria

- `api/openapi/admin-service.yaml` 全量路径闭环
- 权限矩阵、审计事务、删除语义无分叉
- 配置键与注册表完全一致
