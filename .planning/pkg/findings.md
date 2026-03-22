# pkg findings

## Confirmed Constraints

- `pkg/` 只能放稳定共享能力
- 存储抽象属于共享包
- 业务领域对象必须留在各自服务内
- 共享错误模型必须与错误码注册表一致

## Implementation Findings

- `pkg/storage` 接口签名需要严格跟随 `docs/storage-abstraction-spec.md`
- `pkg/xerrors` 需要提供稳定 code 常量与 HTTP status 映射，避免各服务重复实现
- `ids` 与 `clock` 已形成可测试的最小共享基础
