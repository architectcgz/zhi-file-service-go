# upload-service task plan

## Goal

打通 upload session 主链路，覆盖创建会话、代理上传、part presign、complete、abort、dedup 和元数据提交。

## Inputs

- `docs/upload-service-implementation-spec.md`
- `docs/upload-session-state-machine-spec.md`
- `docs/upload-integrity-hash-spec.md`
- `api/openapi/upload-service.yaml`

## Phases

### Phase 1

- 落 Session 领域对象、状态枚举、不变量
- 落 repository / storage / tx / outbox 端口

### Phase 2

- 实现 create session / get session / list parts

### Phase 3

- 实现 inline upload / multipart presign
- 收口 `INLINE` / `PRESIGNED_SINGLE` / `DIRECT` 模式语义

### Phase 4

- 实现 complete / abort
- 落 dedup、usage、refcount、outbox 提交

### Phase 5

- 补状态机、哈希、并发 complete、补偿路径测试
- 补热路径压测与 Grafana 观测

## Deliverables

- `internal/services/upload`
- 对应 transport / infra / tests

## Exit Criteria

- `api/openapi/upload-service.yaml` 全量路径闭环
- complete 链路满足事务边界与幂等要求
- 哈希契约、状态机、outbox、usage 更新一致
