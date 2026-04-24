# 视频模块联调说明与 `curl`

本文档对应当前代码实现，不再使用 `projectId` 作为视频上传和列表查询的主输入，统一改为 `taskId`。

当前视频主链：

1. 先创建 `project`
2. 再创建 `task`
3. 最后在 `task` 下上传和查询视频

建议先在 Postman 中准备这些环境变量：

```text
baseUrl = http://127.0.0.1:8080
accessToken = 登录后返回的 accessToken
projectId = 已创建项目 ID
taskId = 已创建批次 ID
videoId = 上传凭证接口返回的 videoId
uploadId = 上传凭证接口返回的 uploadId
```

## 1. 先创建或确认 `task`

### 1.1 创建批次

```bash
curl --location '{{baseUrl}}/api/v1/projects/{{projectId}}/tasks' \
--header 'Authorization: Bearer {{accessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "name": "2026春季第一次提交",
  "description": "发动机曲柄连杆机构拆装实验第一批次",
  "rubricId": "请替换为真实评分细则ID",
  "startTime": "2026-03-25T08:00:00+08:00",
  "deadline": "2026-04-10T23:59:59+08:00"
}'
```

期望：
- HTTP `201`
- 响应中包含 `data.id`
- 将返回的 `data.id` 写入 `{{taskId}}`

### 1.2 查询项目下批次列表

```bash
curl --location '{{baseUrl}}/api/v1/projects/{{projectId}}/tasks' \
--header 'Authorization: Bearer {{accessToken}}'
```

## 2. 获取上传凭证

注意：当前接口已经改为传 `taskId`，不再传 `projectId`。

```bash
curl --location '{{baseUrl}}/api/v1/videos/upload-credential' \
--header 'Authorization: Bearer {{accessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "taskId": "{{taskId}}",
  "filename": "student_001.mp4",
  "fileSize": 104857600,
  "studentName": "张三",
  "studentNumber": "20260301"
}'
```

期望：
- HTTP `200`
- 响应中包含：
  - `data.videoId`
  - `data.uploadUrl`
  - `data.uploadId`
  - `data.credential`

当前 `credential` 结构：

```json
{
  "accessKeyId": "...",
  "accessKeySecret": "...",
  "securityToken": "...",
  "expiration": "2026-03-25T12:00:00Z"
}
```

说明：
- 这是华为云 OBS 的 STS 临时凭证
- 当前代码里对象 key 规则已经是：
  - `projects/{projectId}/tasks/{taskId}/videos/{videoId}/{filename}`
- `projectId` 由后端通过 `taskId -> task.project_id` 自动反查，不需要前端重复传

## 3. 前端分片上传说明

当前后端已经完成的职责：
- 发起 multipart upload
- 返回 `uploadId`
- 返回 STS 临时凭证

前端需要做的事：

1. 使用 `credential.accessKeyId`
2. 使用 `credential.accessKeySecret`
3. 使用 `credential.securityToken`
4. 使用 `uploadId`
5. 按 OBS SDK 完成 multipart 分片上传
6. 收集每个分片的：
   - `partNumber`
   - `etag`
7. 上传完成后调用 `confirm-upload`

## 4. 确认上传完成

```bash
curl --location '{{baseUrl}}/api/v1/videos/{{videoId}}/confirm-upload' \
--header 'Authorization: Bearer {{accessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "uploadId": "{{uploadId}}",
  "parts": [
    {
      "partNumber": 1,
      "etag": "请替换为真实ETag"
    }
  ]
}'
```

期望：
- HTTP `200`
- 返回视频详情
- `data.status = ready`
- `data.uploadProgress = 100`

## 5. 查询视频列表

注意：当前接口已经改为按 `taskId` 查询，不再按 `projectId` 查询。

```bash
curl --location '{{baseUrl}}/api/v1/videos?taskId={{taskId}}&page=1&pageSize=20' \
--header 'Authorization: Bearer {{accessToken}}'
```

常用筛选参数：
- `status`
- `studentId`
- `studentNumber`
- `keyword`

## 6. 查询视频详情

```bash
curl --location '{{baseUrl}}/api/v1/videos/{{videoId}}' \
--header 'Authorization: Bearer {{accessToken}}'
```

期望：
- HTTP `200`
- 响应中会返回：
  - `task`
  - `project`
  - `creator`
  - `scorer`
  - `evaluationStatus`
  - `aiStatus`
  - `manualStatus`
  - `playUrl`

## 7. 删除视频

```bash
curl --location --request DELETE '{{baseUrl}}/api/v1/videos/{{videoId}}' \
--header 'Authorization: Bearer {{accessToken}}'
```

期望：
- HTTP `204`

## 8. 前后端对齐结论

需要前端同步的变更只有两处：

1. `POST /api/v1/videos/upload-credential`
- 旧：`projectId`
- 新：`taskId`

2. `GET /api/v1/videos`
- 旧：`projectId`
- 新：`taskId`

不变的接口：
- `POST /api/v1/videos/:id/confirm-upload`
- `GET /api/v1/videos/:id`
- `DELETE /api/v1/videos/:id`

业务原因：
- 当前数据库主链已经是 `project -> task -> video`
- 一个 `project` 下可能有多个 `task`
- 所以上传和列表查询必须明确落到某个 `task`
