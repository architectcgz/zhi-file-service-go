# admin-service progress

## Status

- `completed`

## Notes

- 已完成控制面规则层、Postgres 仓储、HTTP handler、runtime 注册与 `cmd/admin-service` 启动接线
- 已补齐 handler/runtime 测试，并验证目标范围 `go test`、`go test -race` 可通过
- 管理面鉴权已切到 JWKS resolver；runtime 通过 `ADMIN_AUTH_JWKS` 与 `ADMIN_AUTH_ALLOWED_ISSUERS` 构建生产认证链路，并覆盖 inline/remote JWKS、密钥轮换与 issuer allowlist
- `auth_dev.go` 仍保留为开发辅助实现，但不再代表 admin-service 的主认证路径；Phase 7 已收口完成
