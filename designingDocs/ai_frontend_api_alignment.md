# AI 评估前端接口对齐文档

本文档只整理前端会直接调用或直接消费返回结果的 AI 评估相关接口。

不包含：

- 后端与外部 AI 服务之间的内部调用
- 后端轮询外部 AI 服务的内部接口

说明：

- 外部 AI 服务采用后端业务轮询模式
- 前端不直接访问 AI 侧 `analysis-jobs` 接口
- 当前上传成功并执行 `confirm-upload` 后，若后端 AI 已配置，会自动触发单视频 AI 评估创建
- 因此前端当前主要负责查询和展示；`/ai-evaluations` 触发接口更多用于手动重跑或补触发

## 1. 通用说明

### 1.1 鉴权

除特别说明外，本文档中的接口都需要登录态：

```http
Authorization: Bearer {accessToken}
```

### 1.2 统一响应外层

成功响应：

```json
{
  "code": 200,
  "message": "success",
  "data": {},
  "timestamp": "2026-04-06T10:00:00Z"
}
```

失败响应：

```json
{
  "code": 409,
  "message": "ai evaluation is already processing",
  "errors": null,
  "timestamp": "2026-04-06T10:00:00Z"
}
```

### 1.3 当前 AI 状态口径

- `processing`：AI 正在评估
- `completed`：AI 评估完成
- `failed`：AI 评估失败

### 1.4 force 字段说明

`force` 表示是否允许对同一个视频重新发起一轮 AI 评估。

规则如下：

- 如果该视频从未发起过 AI 评估，可以直接发起
- 如果最新一条 AI 评估状态是 `processing`，不允许再次发起
- 如果最新一条 AI 评估状态是 `completed` 或 `failed`：
  - `force=false`：不允许重跑
  - `force=true`：允许新开一轮评估

## 2. 单视频发起 AI 评估

### URL

`/api/v1/videos/:id/ai-evaluations`

### Method

`POST`

### 权限

需要 `task:update`

### Request Body

```json
{
  "force": false
}
```

字段说明：

- `force`：可选，默认可理解为 `false`

### Response Body

```json
{
  "code": 201,
  "message": "success",
  "data": {
    "id": "7f4f9d3e-1111-2222-3333-444444444444",
    "videoId": "a33f8b9d-1111-2222-3333-444444444444",
    "taskId": "b84d7d4a-1111-2222-3333-444444444444",
    "status": "processing",
    "modelVersion": null,
    "totalScore": null,
    "errorMessage": null,
    "createdAt": "2026-04-06T10:00:00Z",
    "updatedAt": "2026-04-06T10:00:00Z",
    "startedAt": "2026-04-06T10:00:00Z",
    "completedAt": null
  },
  "timestamp": "2026-04-06T10:00:00Z"
}
```

说明：

- 成功后返回的是新创建的 AI 评估记录
- 创建成功时，初始状态一定是 `processing`

### 常见错误码

- `400`
  - `invalid video id`
  - `invalid request payload`
  - `video is not ready for ai evaluation`
  - `video task is required`
  - `ai evaluation already exists, use force=true to create a new run`
- `403`
  - 当前用户无权操作该任务
- `404`
  - `video not found`
- `409`
  - `ai evaluation is already processing`
- `502`
  - `ai provider request failed`
- `500`
  - AI 服务未配置，或其他服务端异常

### 前端使用建议

- 首次发起时直接传 `{ "force": false }` 即可
- 如果后端返回“已存在，需要 `force=true`”，前端应提示用户“是否重新评估”
- 用户确认后，再用 `{ "force": true }` 重试
- 如果返回“already processing”，前端不应重复提交，改为轮询状态或刷新详情

## 3. 任务下批量发起 AI 评估

### URL

`/api/v1/tasks/:id/ai-evaluations`

### Method

`POST`

### 权限

需要 `task:update`

### Request Body

```json
{
  "videoIds": [
    "a33f8b9d-1111-2222-3333-444444444444",
    "c71d8d56-1111-2222-3333-444444444444"
  ],
  "force": false
}
```

字段说明：

- `videoIds`：必填，要批量发起 AI 评估的视频 ID 列表
- `force`：可选，含义同单视频接口

### Response Body

```json
{
  "code": 201,
  "message": "success",
  "data": {
    "total": 2,
    "processing": 1,
    "failed": 1,
    "errors": [
      {
        "videoId": "c71d8d56-1111-2222-3333-444444444444",
        "error": "ai evaluation already exists, use force=true to create a new run"
      }
    ]
  },
  "timestamp": "2026-04-06T10:00:00Z"
}
```

字段说明：

- `total`：本次请求中的视频总数
- `processing`：成功进入 AI 处理中状态的数量
- `failed`：本次处理失败的数量
- `errors`：逐视频失败明细

说明：

- 这是“部分成功”接口，不是事务型全成功/全失败
- 某些视频成功、某些视频失败是正常结果

### 常见错误码

- `400`
  - `invalid task id`
  - `invalid request payload`
  - `invalid videoIds`
  - `videoIds are required`
- `403`
  - 当前用户无权操作该任务
- `404`
  - `task not found`
- `500`
  - 服务端异常

注意：

- 逐视频的业务失败不会进入 HTTP 错误码，而是体现在 `data.errors` 中

### 前端使用建议

- 前端批量发起后，应根据返回里的 `processing`、`failed`、`errors` 给出汇总反馈
- 不要把 `201` 直接理解成全部成功，要以 `data` 为准
- 如果需要支持“只重跑失败项”，前端可以从 `errors` 中筛选，再次提交对应 `videoIds`

## 4. 查询 AI 评估状态

### URL

`/api/v1/ai-evaluations/:id`

### Method

`GET`

### 权限

需要 `task:read`

### Request Body

无

### Response Body

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "7f4f9d3e-1111-2222-3333-444444444444",
    "videoId": "a33f8b9d-1111-2222-3333-444444444444",
    "taskId": "b84d7d4a-1111-2222-3333-444444444444",
    "status": "processing",
    "modelVersion": "model-x",
    "totalScore": 85.5,
    "errorMessage": null,
    "createdAt": "2026-04-06T10:00:00Z",
    "updatedAt": "2026-04-06T10:05:00Z",
    "startedAt": "2026-04-06T10:00:00Z",
    "completedAt": null
  },
  "timestamp": "2026-04-06T10:05:00Z"
}
```

字段说明：

- `status`：`processing` / `completed` / `failed`
- `totalScore`：只有已完成时才可能有值
- `errorMessage`：只有失败时才可能有值

### 常见错误码

- `400`
  - `invalid ai evaluation id`
- `403`
  - 当前用户无权读取该任务
- `404`
  - `ai evaluation not found`

### 前端使用建议

- 发起评估成功后，前端可以保存返回的 `id`，后续轮询这个接口
- 轮询在 `status=completed` 或 `status=failed` 后应停止
- 如果前端本身已经在视频详情页或评分详情页，也可以直接刷新详情接口，不一定必须单独轮询这个接口

## 5. 查询 AI 评估完整结果

### URL

`/api/v1/ai-evaluations/:id/result`

### Method

`GET`

### 权限

需要 `task:read`

### Request Body

无

### Response Body

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "7f4f9d3e-1111-2222-3333-444444444444",
    "videoId": "a33f8b9d-1111-2222-3333-444444444444",
    "taskId": "b84d7d4a-1111-2222-3333-444444444444",
    "status": "completed",
    "modelVersion": "model-x",
    "summary": {
      "overallDescription": "整体表现稳定",
      "score": 85.5,
      "maxScore": 100
    },
    "details": [
      {
        "title": "动作规范",
        "fullScore": 40,
        "aiScore": 34,
        "items": [
          {
            "subtitle": "起势",
            "fullScore": 10,
            "aiScore": 8,
            "status": "ok",
            "feedback": "起势较稳"
          }
        ]
      }
    ],
    "videoStages": [],
    "videoPoints": [],
    "artifacts": {
      "reportHtmlUrl": "https://example.com/report.html",
      "analysisJsonUrl": "https://example.com/analysis.json",
      "evidenceIndexJsonUrl": "https://example.com/evidence.json"
    },
    "completedAt": "2026-04-06T10:08:00Z"
  },
  "timestamp": "2026-04-06T10:08:00Z"
}
```

字段说明：

- `summary`：AI 总体描述和总分
- `details`：按量规项映射后的分组结果
- `videoStages`：视频阶段划分结果
- `videoPoints`：视频中的关键点、错误点、证据点
- `artifacts`：外部产物地址，例如报告页、分析 JSON、证据索引

### 常见错误码

- `400`
  - `invalid ai evaluation id`
- `403`
  - 当前用户无权读取该任务
- `404`
  - `ai evaluation not found`
- `409`
  - `ai evaluation result not ready`

### 前端使用建议

- 只有在 `status=completed` 后再请求这个接口
- 如果返回 `409`，前端应继续轮询状态接口或刷新详情接口
- 如果只需要展示“最新 AI 结果摘要”，可优先使用详情接口里的 `aiEvaluation`，无需额外请求该接口

## 6. 现有详情接口中的 AI 字段

除了上面的 4 个专用接口，现有详情接口现在也会返回最新一条 AI 评估结果摘要。

### 6.1 视频详情

#### URL

`/api/v1/videos/:id`

#### Method

`GET`

#### 关键字段

```json
{
  "id": "video-id",
  "aiStatus": "completed",
  "aiScore": 85.5,
  "aiEvaluation": {
    "evaluationId": "evaluation-id",
    "status": "completed",
    "score": 85.5,
    "modelVersion": "model-x",
    "errorMessage": null,
    "startedAt": "2026-04-06T10:00:00Z",
    "completedAt": "2026-04-06T10:08:00Z",
    "summary": {
      "overallDescription": "整体表现稳定",
      "score": 85.5,
      "maxScore": 100
    },
    "details": [],
    "videoStages": [],
    "videoPoints": [],
    "artifacts": {}
  }
}
```

#### 前端使用建议

- 视频详情页可以直接消费 `aiStatus`、`aiScore`、`aiEvaluation`
- 如果只是做详情展示，通常不需要额外请求 `/api/v1/ai-evaluations/:id`

### 6.2 scorer 评分详情

#### URL

`/api/v1/tasks/:id`

说明：

- 当当前登录用户角色是 `scorer` 时，这个接口返回的是评分任务详情视图
- 该返回中同样会包含 `aiEvaluation`

#### Method

`GET`

#### 关键字段

```json
{
  "id": "video-id",
  "video": {},
  "rubric": {},
  "aiEvaluation": {
    "evaluationId": "evaluation-id",
    "status": "completed",
    "score": 85.5,
    "modelVersion": "model-x",
    "errorMessage": null,
    "startedAt": "2026-04-06T10:00:00Z",
    "completedAt": "2026-04-06T10:08:00Z",
    "summary": {},
    "details": [],
    "videoStages": [],
    "videoPoints": [],
    "artifacts": {}
  },
  "manualEvaluation": {}
}
```

#### 前端使用建议

- scorer 评分页可直接使用该字段做 AI 评分参考展示
- 如果只是为了给评分员展示 AI 辅助结果，一般不需要额外调结果接口

## 7. 推荐前端调用流程

### 场景一：教师或管理员触发单视频 AI 评估

1. 调 `POST /api/v1/videos/:id/ai-evaluations`
2. 成功后拿到 `evaluationId`
3. 轮询 `GET /api/v1/ai-evaluations/:id`
4. 状态变为 `completed` 后：
   - 直接刷新 `GET /api/v1/videos/:id`
   - 或调用 `GET /api/v1/ai-evaluations/:id/result` 拉完整结果

### 场景二：教师或管理员批量触发

1. 调 `POST /api/v1/tasks/:id/ai-evaluations`
2. 根据返回的 `processing` / `failed` / `errors` 给出批量反馈
3. 对于成功进入处理中的视频：
   - 刷新任务列表或视频列表
   - 进入详情页时再按需查询对应 AI 结果

### 场景三：评分员查看 AI 辅助结果

1. 调 `GET /api/v1/tasks/:id`
2. 直接读取返回中的 `aiEvaluation`
3. 如需更完整产物链接，可再访问 `/api/v1/ai-evaluations/:id/result`

## 8. 前后端对齐建议

- 前端要把 `201 Created` 和“全部成功”区分开，特别是批量接口
- 前端要支持 `force=true` 的重跑确认流程
- 前端展示状态时，按 `processing / completed / failed` 三态先实现即可
- 详情页优先消费现有详情接口里的 `aiEvaluation`，避免无意义重复请求
- 对于批量发起失败项，前端应优先展示 `videoId + error`，不要只提示“部分失败”
