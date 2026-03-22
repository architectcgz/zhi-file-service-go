# zhi-file-service-go 前端上传进度规范

## 1. 目标

这份文档定义前端上传进度在 `zhi-file-service-go` 中的职责边界和实现口径。

它回答三个问题：

1. 前端上传进度是否需要后端接口支持
2. 实时进度应该由前端本地计算，还是由后端实时回传
3. `INLINE`、`PRESIGNED_SINGLE`、`DIRECT` 三种模式分别应该怎么做

配套文档：

- [api/openapi/upload-service.yaml](/home/azhi/workspace/projects/zhi-file-service-go/api/openapi/upload-service.yaml)
- [upload-service-implementation-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-service-implementation-spec.md)
- [upload-session-state-machine-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/upload-session-state-machine-spec.md)
- [business-integration-spec.md](/home/azhi/workspace/projects/zhi-file-service-go/docs/business-integration-spec.md)

## 2. 核心结论

### 2.1 Phase 1 需要后端查询接口，但不需要单独的“上报进度接口”

前端上传进度在第一阶段需要后端提供 authoritative 查询能力，但不需要新增：

- `POST /api/v1/upload-sessions/{id}/progress`
- WebSocket 实时字节推送
- SSE 实时字节推送

当前已定义的接口已经足够支撑前端上传进度与恢复：

- `GET /api/v1/upload-sessions/{uploadSessionId}`
- `GET /api/v1/upload-sessions/{uploadSessionId}/parts`

### 2.2 实时百分比由前端本地计算

页面上“正在上传 37%”这类实时百分比，默认由前端基于浏览器上传事件或对象存储 SDK 回调自行计算。

原因：

- 浏览器本地拿到的是最实时的字节流进度
- 若再要求前端把进度回写给后端，只会增加接口、写放大和一致性复杂度
- 后端拿到客户端自报进度后，也无法把它当成最终事实

### 2.3 后端负责 authoritative 状态，不负责毫秒级字节进度广播

后端负责：

- 会话当前状态是否还有效
- 上传是否已经进入 `COMPLETING` 或 `COMPLETED`
- `DIRECT` 模式下 authoritative uploaded parts 是什么
- 最终是否成功生成 `fileId`

后端不负责：

- 每 100ms 推送一次字节进度
- 把客户端上报的 `loadedBytes` 持久化成事实
- 把“本地已传到 100%”直接视为“上传已完成”

## 3. 进度口径

### 3.1 UI 实时进度口径

前端 UI 的实时进度条应以本地字节进度为准：

- `INLINE`: 浏览器到 `upload-service` 的请求发送进度
- `PRESIGNED_SINGLE`: 浏览器到对象存储 `PUT` 的发送进度
- `DIRECT`: 各 part 上传进度加权汇总后的总进度

### 3.2 后端状态口径

后端接口返回的是 authoritative 状态，不是高频字节流：

- `status`
- `totalParts`
- `uploadedParts`
- `fileId`

其中：

- `status` 适合判断当前阶段和终态
- `uploadedParts` 适合做断点续传恢复和粗粒度观察
- `uploadedParts / totalParts` 不是所有场景下的精确百分比

### 3.3 `DIRECT` 模式可恢复的进度口径

对 `DIRECT` 模式，页面刷新或客户端重启后，可通过 `GET /api/v1/upload-sessions/{uploadSessionId}/parts` 返回的 `sizeBytes` 重新估算已完成字节数。

推荐算法：

`sum(parts.sizeBytes) / session.sizeBytes`

这用于恢复 UI 进度，不替代最终 complete 校验。

### 3.4 `PRESIGNED_SINGLE` 不提供可靠的中途后端字节进度

对 `PRESIGNED_SINGLE`，对象直接上传到对象存储，中途上传了多少字节，后端默认无法可靠、低成本、通用地观察。

因此：

- 页面实时百分比必须来自前端本地
- 后端只负责在 complete 阶段校验对象是否存在、大小是否匹配

## 4. 前后端职责划分

### 4.1 前端职责

前端负责：

- 调用创建上传会话接口
- 维护本地上传进度条
- 在 `DIRECT` 模式保存已成功 part 的本地结果
- 在刷新恢复、失败重试时轮询后端状态
- 在上传数据完成后调用 `complete`
- 在用户取消时调用 `abort`

### 4.2 后端职责

后端负责：

- 维护 `UploadSession` 状态机
- 提供可轮询的会话详情接口
- 对 `DIRECT` 提供 authoritative parts 查询
- 在 `complete` 阶段再次校验对象事实
- 返回最终 `fileId` 或终态错误

## 5. 后端接口使用规则

这些接口都属于数据面 north-south API，默认仍要求 Bearer Token，不提供匿名进度查询入口。

### 5.1 `GET /api/v1/upload-sessions/{uploadSessionId}`

用途：

- 轮询会话状态
- 判断是否进入 `COMPLETING`
- 判断是否已经 `COMPLETED`
- 页面刷新后恢复会话上下文

不应用于：

- 100ms 级高频字节进度刷新
- 替代浏览器本地上传事件

### 5.2 `GET /api/v1/upload-sessions/{uploadSessionId}/parts`

用途：

- `DIRECT` 模式查看 authoritative uploaded parts
- 页面刷新后恢复 multipart 上传进度
- complete 前做断点续传对齐

不应用于：

- `INLINE` 模式实时进度
- `PRESIGNED_SINGLE` 模式精确字节进度

### 5.3 `POST /api/v1/upload-sessions/{uploadSessionId}/complete`

用途：

- 把“数据已写入对象存储”推进为“逻辑文件已完成”
- 获取最终 `fileId`

必须强调：

- 本地上传到 100% 只是“数据发送完成”
- 收到 `complete` 成功响应或轮询到 `COMPLETED`，才算业务上的上传完成

### 5.4 `POST /api/v1/upload-sessions/{uploadSessionId}/abort`

用途：

- 用户主动取消上传
- 前端放弃当前会话，明确推进为终态

## 6. 各上传模式建议实现

### 6.1 `INLINE`

推荐流程：

1. 调 `POST /api/v1/upload-sessions`
2. 使用 `XMLHttpRequest.upload.onprogress` 或等价浏览器能力展示本地字节进度
3. 调 `PUT /api/v1/upload-sessions/{uploadSessionId}/content` 上传二进制内容
4. 请求成功后立即调 `POST /api/v1/upload-sessions/{uploadSessionId}/complete`
5. complete 返回前，UI 展示“处理中”而不是直接判定成功

说明：

- `INLINE` 的实时进度不需要单独后端接口
- 浏览器本地进度通常比后端轮询更及时
- 若页面刷新或请求超时，可用 `GET /api/v1/upload-sessions/{uploadSessionId}` 恢复状态

### 6.2 `PRESIGNED_SINGLE`

推荐流程：

1. 调 `POST /api/v1/upload-sessions`
2. 从响应中获取 `putUrl`
3. 浏览器直接向对象存储发起单对象 `PUT`
4. 使用浏览器本地上传事件展示实时百分比
5. 上传成功后调 `POST /api/v1/upload-sessions/{uploadSessionId}/complete`
6. 若 complete 请求超时或页面刷新，轮询 `GET /api/v1/upload-sessions/{uploadSessionId}`

说明：

- `PRESIGNED_SINGLE` 的中途字节进度以后端视角不可可靠获取
- 不要尝试为这个模式补一个“后端实时进度接口”
- 后端在这个模式里主要负责会话状态和 complete 校验

### 6.3 `DIRECT`

推荐流程：

1. 调 `POST /api/v1/upload-sessions`
2. 调 `POST /api/v1/upload-sessions/{uploadSessionId}/parts/presign`
3. 并发上传各 part，并在前端本地汇总实时进度
4. 上传过程中按需轮询 `GET /api/v1/upload-sessions/{uploadSessionId}`
5. 刷新恢复、断点续传或 complete 前对齐时，调 `GET /api/v1/upload-sessions/{uploadSessionId}/parts`
6. 确认 parts 齐全后调 `POST /api/v1/upload-sessions/{uploadSessionId}/complete`

说明：

- `DIRECT` 模式下，后端可以提供 part 级 authoritative 观察
- 但这仍然不是高频实时字节推送接口
- `GET /parts` 更适合恢复、对齐和断点续传，不适合每几百毫秒刷一次

### 6.4 `DIRECT` complete 前的推荐做法

如果满足以下任一条件，前端在 complete 前应先调用一次 `GET /parts` 做 authoritative 对齐：

- 页面发生过刷新
- 存在 part 重试
- 本地缓存的 part 结果不完整
- 上传过程出现过网络抖动或超时

如果是同一页面内的顺序 happy path，前端可直接基于本地已成功 part 结果发起 complete；后端在 complete 阶段仍会再次以 provider authoritative parts 做校验。

## 7. 轮询建议

### 7.1 基本轮询策略

建议值：

- 上传进行中：`GET /api/v1/upload-sessions/{uploadSessionId}` 每 `1-2s`
- `DIRECT` 大文件或断点续传：`GET /api/v1/upload-sessions/{uploadSessionId}/parts` 每 `2-3s`
- `complete` 请求超时或收到“处理中”：`GET /api/v1/upload-sessions/{uploadSessionId}` 每 `1s`

### 7.2 停止轮询条件

出现以下任一状态即可停止：

- `COMPLETED`
- `FAILED`
- `ABORTED`
- `EXPIRED`

如果用户离开页面但上传任务仍要继续，可按前端容器能力降频轮询，不要求保持高频。

### 7.3 轮询不是进度条主驱动

轮询的主要价值是：

- 恢复状态
- 确认终态
- 发现服务端拒绝或过期

轮询不应替代本地上传回调驱动的进度条。

## 8. UI 状态映射建议

| 后端状态 | 前端展示建议 | 说明 |
|------|------|------|
| `INITIATED` | 准备上传 | 会话已创建，尚未确认数据进入对象存储 |
| `UPLOADING` | 正在上传 | 结合本地进度条展示百分比 |
| `COMPLETING` | 正在处理 | 进度条可停在 `100%`，文案应改成“处理中/校验中” |
| `COMPLETED` | 上传成功 | 以 `fileId` 为准进入业务落库或提交表单 |
| `FAILED` | 上传失败 | 提示重试或新建会话 |
| `ABORTED` | 已取消 | 用户主动取消，不再继续上传 |
| `EXPIRED` | 已过期 | 提示重新发起上传 |

关键约束：

- 本地进度到 `100%` 时，不应立刻显示“上传成功”
- 必须等 `complete` 成功或 session 进入 `COMPLETED`

## 9. 明确禁止的做法

以下做法在第一阶段明确禁止：

- 新增 `POST /progress` 让前端回写 `loadedBytes`
- 把客户端自报进度持久化为权威事实
- 把 `uploadedParts / totalParts` 当作所有模式下的精确百分比
- `PRESIGNED_SINGLE` 依赖后端提供实时字节进度
- 本地进度到 `100%` 就直接把文件视为已完成
- 用高频轮询替代浏览器原生上传事件

## 10. 第二阶段扩展边界

只有在出现以下需求时，才考虑新增更重的进度通道：

- 服务端异步导入、转存、转码
- 跨设备观察同一上传任务
- 长耗时后台处理需要服务端主动推送阶段变化

在这些场景出现前，Phase 1 保持：

- 前端本地实时进度
- 后端 authoritative 轮询接口
- 无额外 progress report API
