# access-service findings

## Confirmed Constraints

- `/api/v1/files/*` 默认仍要求 Bearer Token
- 匿名访问只发生在最终 public URL 或 ticket redirect 落点
- access ticket 第一阶段默认无状态签名票据
- 热路径读取直接基于 `file_assets` 投影

## Implementation Findings

- `GetFile`、`CreateAccessTicket`、`ResolveDownload`、`RedirectByAccessTicket` 已有 app / transport / runtime 闭环
- public object URL 与 private presign GET 已在 `infra/storage` 中落地
- 当前缺口主要在 Phase 5：没有独立的 access benchmark、k6 场景和 Grafana dashboard
- contract / e2e 不再并入本模块，统一放到 `delivery-validation`
