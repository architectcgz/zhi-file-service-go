# access-service findings

## Confirmed Constraints

- `/api/v1/files/*` 默认仍要求 Bearer Token
- 匿名访问只发生在最终 public URL 或 ticket redirect 落点
- access ticket 第一阶段默认无状态签名票据
- 热路径读取直接基于 `file_assets` 投影
