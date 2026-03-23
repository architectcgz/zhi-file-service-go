# delivery-validation task plan

## Goal

补齐仓库级交付能力，让 zhi-file-service-go 不只是“代码能编译测试”，还具备 contract、e2e、部署和统一验证入口。

## Inputs

- `docs/test-validation-spec.md`
- `docs/deployment-runtime-spec.md`
- `docs/development-workflow-spec.md`
- `docs/slo-observability-spec.md`
- `api/openapi/upload-service.yaml`
- `api/openapi/access-service.yaml`
- `api/openapi/admin-service.yaml`

## Phases

### Phase 1 (`completed`)

- 把 `scripts/test/contract.sh` 从“YAML 语法校验”升级为真正的 contract test 入口
- 在 `test/contract/` 下补 upload/access/admin 的核心契约测试
- 明确哪些 contract test 只做 schema/assertion，哪些需要真实服务进程

### Phase 2 (`completed`)

- 在 `test/e2e/` 下补跨服务闭环：
- upload -> access 下载
- admin 逻辑删除 -> job 物理清理
- upload fail/expire -> job repair/cleanup 收敛

### Phase 3 (`completed`)

- 补 `deployments/helm/*` 的正式 chart 骨架
- 补 `deployments/kustomize/base` 与 `overlays/{dev,test,prod}`
- 对齐资源配额、探针、HPA/PDB、配置注入和对象存储/数据库依赖

### Phase 4 (`pending`)

- 收口 `test/performance/` 目录，把 access 及后续服务性能资产纳入统一入口
- 增加 Prometheus/Grafana 说明，避免 dashboard 只存在 JSON 文件而无法复现
- 让 `make test-contract`、`make test-e2e`、必要时 `make test-performance` 真正可执行

## Deliverables

- `test/contract`
- `test/e2e`
- `test/performance`
- `deployments/helm`
- `deployments/kustomize`
- 对应脚本与 Makefile 收口

## Exit Criteria

- contract / e2e / deployment 不再是占位目录
- 发布前最小验证链路可以通过 Makefile 一次触发
- 运行时、观测、部署配置和设计文档保持一致
- 性能入口统一收口前，本模块仍保持活跃
