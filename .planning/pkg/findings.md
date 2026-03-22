# pkg findings

## Confirmed Constraints

- `pkg/` 只能放稳定共享能力
- 存储抽象属于共享包
- 业务领域对象必须留在各自服务内
- 共享错误模型必须与错误码注册表一致
