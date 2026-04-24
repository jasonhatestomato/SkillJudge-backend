# Phase 1 主链联调文档

本文档用于联调当前 `Phase 1` 的完整主链：

1. 登录
2. 创建项目
3. 创建评分细则
4. 创建批次
5. 上传视频
6. 分配评分员
7. 评分员查看我的任务
8. 评分员查看评分详情
9. 评分员提交人工评分

本文档尽量采用 Postman 环境变量风格，方便直接导入或手动替换。

建议准备这些环境变量：

```text
baseUrl = http://127.0.0.1:8080

teacherUsername = 教师账号
teacherPassword = 教师密码
teacherAccessToken = 教师登录后 accessToken

scorerUsername = 评分员账号
scorerPassword = 评分员密码
scorerAccessToken = 评分员登录后 accessToken

schoolId = 学校 ID
projectId = 项目 ID
rubricId = 评分细则 ID
taskId = 批次 ID
videoId = 视频 ID
uploadId = 上传凭证返回的 uploadId
scorerUserId = 评分员用户 ID
```

## 1. 服务检查

### 1.1 健康检查

```bash
curl --location '{{baseUrl}}/health'
```

### 1.2 就绪检查

```bash
curl --location '{{baseUrl}}/ready'
```

## 2. 教师登录

```bash
curl --location '{{baseUrl}}/api/v1/auth/login' \
--header 'Content-Type: application/json' \
--data '{
  "username": "{{teacherUsername}}",
  "password": "{{teacherPassword}}"
}'
```

期望：

- HTTP `200`
- 取返回中的 `data.accessToken` 填入 `{{teacherAccessToken}}`

## 3. 创建项目

```bash
curl --location '{{baseUrl}}/api/v1/projects' \
--header 'Authorization: Bearer {{teacherAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "name": "发动机曲柄连杆机构拆装实验",
  "description": "Phase 1 联调项目",
  "schoolId": "{{schoolId}}",
  "status": "draft",
  "experimentType": "physical",
  "subject": "机械",
  "gradeLevel": "高职"
}'
```

期望：

- HTTP `201`
- 取返回中的 `data.id` 填入 `{{projectId}}`

## 4. 创建评分细则

当前可以用手工 JSON 创建，也可以走 Excel 上传。联调主链推荐先用手工 JSON。

```bash
curl --location '{{baseUrl}}/api/v1/rubrics' \
--header 'Authorization: Bearer {{teacherAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "name": "发动机曲柄连杆机构拆装评分细则",
  "description": "Phase 1 联调评分细则",
  "totalScore": 100,
  "templateType": "manual",
  "isTemplate": false,
  "isPublic": false,
  "schoolId": "{{schoolId}}",
  "items": [
    {
      "id": "1",
      "name": "准备工作",
      "score": 20,
      "subItems": [
        {
          "id": "1-1",
          "requirement": "工具准备齐全",
          "score": 10,
          "fullScoreStandard": "工具和材料准备完整",
          "deductionItems": "缺少工具扣分"
        },
        {
          "id": "1-2",
          "requirement": "防护措施规范",
          "score": 10,
          "fullScoreStandard": "穿戴规范",
          "deductionItems": "未按要求穿戴扣分"
        }
      ]
    },
    {
      "id": "2",
      "name": "拆装过程",
      "score": 80,
      "subItems": [
        {
          "id": "2-1",
          "requirement": "拆装步骤正确",
          "score": 40,
          "fullScoreStandard": "步骤完整、顺序正确",
          "deductionItems": "步骤错误扣分"
        },
        {
          "id": "2-2",
          "requirement": "操作规范",
          "score": 40,
          "fullScoreStandard": "操作平稳、无违规",
          "deductionItems": "危险操作扣分"
        }
      ]
    }
  ]
}'
```

期望：

- HTTP `201`
- 取返回中的 `data.id` 填入 `{{rubricId}}`

## 5. 创建批次

```bash
curl --location '{{baseUrl}}/api/v1/projects/{{projectId}}/tasks' \
--header 'Authorization: Bearer {{teacherAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "name": "2026 春季第一次提交",
  "description": "Phase 1 联调批次",
  "rubricId": "{{rubricId}}",
  "startDate": "2026-03-25T08:00:00+08:00",
  "deadline": "2026-04-10T23:59:59+08:00"
}'
```

期望：

- HTTP `201`
- 取返回中的 `data.id` 填入 `{{taskId}}`

## 6. 获取视频上传凭证

注意：当前代码实现已经切到 `taskId`，不再传 `projectId`。

```bash
curl --location '{{baseUrl}}/api/v1/videos/upload-credential' \
--header 'Authorization: Bearer {{teacherAccessToken}}' \
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
- 取返回中的：
  - `data.videoId` 写入 `{{videoId}}`
  - `data.uploadId` 写入 `{{uploadId}}`
  - `data.credential` 供前端 OBS SDK 使用

## 7. 前端完成 OBS 分片上传

后端当前已经完成：

- 发起 multipart upload
- 返回 STS 临时凭证
- 返回 `uploadId`

前端需要做：

1. 用 `credential.accessKeyId`
2. 用 `credential.accessKeySecret`
3. 用 `credential.securityToken`
4. 用 `uploadId`
5. 走 OBS SDK 分片上传
6. 收集每个分片的 `partNumber` 和 `etag`

## 8. 确认上传完成

```bash
curl --location '{{baseUrl}}/api/v1/videos/{{videoId}}/confirm-upload' \
--header 'Authorization: Bearer {{teacherAccessToken}}' \
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
- `data.status = ready`
- `data.uploadProgress = 100`

## 9. 分配评分员

当前单评模式下，推荐先用指定分配。

```bash
curl --location '{{baseUrl}}/api/v1/tasks/{{taskId}}/assignments' \
--header 'Authorization: Bearer {{teacherAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "videoIds": ["{{videoId}}"],
  "scorerIds": ["{{scorerUserId}}"],
  "assignmentStrategy": "specific",
  "specificAssignments": [
    {
      "videoId": "{{videoId}}",
      "scorerId": "{{scorerUserId}}"
    }
  ]
}'
```

期望：

- HTTP `201`
- `data.created = 1`

## 10. 评分员登录

```bash
curl --location '{{baseUrl}}/api/v1/auth/login' \
--header 'Content-Type: application/json' \
--data '{
  "username": "{{scorerUsername}}",
  "password": "{{scorerPassword}}"
}'
```

期望：

- HTTP `200`
- 取返回中的 `data.accessToken` 填入 `{{scorerAccessToken}}`

## 11. 评分员查看我的任务

```bash
curl --location '{{baseUrl}}/api/v1/tasks/my?page=1&pageSize=20&status=pending&projectId={{projectId}}' \
--header 'Authorization: Bearer {{scorerAccessToken}}'
```

期望：

- HTTP `200`
- 返回中包含刚分配的视频任务

## 12. 评分员查看评分详情

注意：当前实现中，这里的 `id` 是评分任务视图 ID，当前落在 `video.id`。

```bash
curl --location '{{baseUrl}}/api/v1/tasks/{{videoId}}' \
--header 'Authorization: Bearer {{scorerAccessToken}}'
```

期望：

- HTTP `200`
- 返回中包含：
  - `project`
  - `video`
  - `rubric`
  - `manualEvaluation`（如果尚未评分，可能为空）
  - `status`
  - `deadline`

## 13. 评分员提交人工评分

```bash
curl --location '{{baseUrl}}/api/v1/tasks/{{videoId}}/submit' \
--header 'Authorization: Bearer {{scorerAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "scoreDetails": [
    {
      "itemId": "1-1",
      "score": 10,
      "deduction": 0,
      "comment": "工具准备完整"
    },
    {
      "itemId": "1-2",
      "score": 8,
      "deduction": 2,
      "comment": "防护佩戴基本规范"
    },
    {
      "itemId": "2-1",
      "score": 36,
      "deduction": 4,
      "comment": "步骤基本正确"
    },
    {
      "itemId": "2-2",
      "score": 35,
      "deduction": 5,
      "comment": "有轻微不规范操作"
    }
  ],
  "totalScore": 89,
  "comments": "整体流程较规范，细节有少量扣分。"
}'
```

期望：

- HTTP `200`
- 返回：
  - `data.status = completed`
  - `data.manualEvaluation`
  - `data.comparison`（当前如无 AI 分数，则 `difference` 可能为空）

## 14. 再次查看评分详情

```bash
curl --location '{{baseUrl}}/api/v1/tasks/{{videoId}}' \
--header 'Authorization: Bearer {{scorerAccessToken}}'
```

期望：

- HTTP `200`
- `manualEvaluation` 已带出本次提交结果
- `status = completed`

## 15. 常见失败场景

### 15.1 未传 `taskId` 获取上传凭证

```bash
curl --location '{{baseUrl}}/api/v1/videos/upload-credential' \
--header 'Authorization: Bearer {{teacherAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "filename": "student_001.mp4",
  "fileSize": 104857600,
  "studentName": "张三",
  "studentNumber": "20260301"
}'
```

期望：

- HTTP `400`

### 15.2 已完成评分再次提交

重复执行第 13 步：

```bash
curl --location '{{baseUrl}}/api/v1/tasks/{{videoId}}/submit' \
--header 'Authorization: Bearer {{scorerAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "scoreDetails": [
    {
      "itemId": "1-1",
      "score": 10
    }
  ],
  "totalScore": 10
}'
```

期望：

- HTTP `409`

### 15.3 评分员查看不属于自己的任务

如果使用别的评分员账号访问：

```bash
curl --location '{{baseUrl}}/api/v1/tasks/{{videoId}}' \
--header 'Authorization: Bearer {{accessTokenOfAnotherScorer}}'
```

期望：

- HTTP `404` 或当前实现语义下的未找到

## 16. 当前联调注意事项

- `video` 上传和列表查询现在都基于 `taskId`
- `GET /api/v1/tasks/:id` 当前有双重语义：
  - 评分员访问：评分详情
  - 非评分员访问：批次详情
- 当前评分任务视图 ID 落在 `video.id`
- 当前只支持单评
- 当前不做草稿保存、双评、多评、仲裁、AI 真实联调、视频转码
