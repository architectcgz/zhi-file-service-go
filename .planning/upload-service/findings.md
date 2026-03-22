# upload-service findings

## Confirmed Constraints

- 上传统一收敛到 `upload-sessions`
- `DIRECT` 与 `PRESIGNED_SINGLE` 默认要求 `contentHash`
- 当前只支持 `SHA256`
- 大文件默认不走服务端分片中转
- `CompleteUploadSession` 是三阶段关键链路
