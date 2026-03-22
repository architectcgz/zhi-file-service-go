# admin-service findings

## Confirmed Constraints

- destructive operation 必须带 `reason`
- 审计与业务变更必须同事务
- 删除文件只做逻辑删除，物理删除交给 `job-service`
- 管理面使用独立身份体系，不复用数据面 token
