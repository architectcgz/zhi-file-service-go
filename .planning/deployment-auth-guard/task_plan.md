# deployment-auth-guard task plan

## Goal

为数据面鉴权新增一层部署与交付守护，避免 `UPLOAD_AUTH_JWKS` / `ACCESS_AUTH_JWKS` 及相关 secret/env 契约在 Helm、Kustomize、脚本或后续重构中再次漂移。

## Inputs

- `deployments/helm/**`
- `deployments/kustomize/**`
- `scripts/tools/doctor.sh`
- `test/bootstrap/bootstrap_scripts_test.go`
- `docs/ops/configuration-registry-spec.md`
- `docs/api/data-plane-auth-context-spec.md`

## Phases

### Phase 1 (`completed`)

- 固化守护范围，确认只做 deployment/chart/script 级约束，不扩散到无关模块
- 识别当前缺失的自动校验点，形成最小改动方案

### Phase 2 (`completed`)

- 增加 deployment/chart 级 smoke checks 或等价自动守护
- 确保 upload/access 的 auth secret 契约在交付面可自动回归

### Phase 3 (`completed`)

- 补最小充分测试与文档说明
- 复核主工作树 planning 状态，准备下一个模块

## Deliverables

- upload/access 数据面 auth 配置的自动守护
- 对应测试与必要文档

## Exit Criteria

- `UPLOAD_AUTH_JWKS` / `ACCESS_AUTH_JWKS` 的必需契约可被自动校验
- issuer allowlist 不会被静态 env 覆盖失效
- 相关验证命令通过
