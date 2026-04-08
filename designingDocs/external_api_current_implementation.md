# AI 评测接口

本文档仅整理当前需要与 AI 侧对齐的 3 个服务间接口。

当前模式：

- 业务后端调用 AI 侧创建分析任务
- 业务后端按 `jobId` 轮询任务状态
- 当状态为 `completed` 时，业务后端主动拉取最终结果并入库
- 当前阶段不设计分期交付协议，仅保留可选的 `result_slices` 扩展字段
- 业务侧当前会在视频上传确认成功后自动发起 AI 创建请求

## 1. 创建分析任务

### URL

`POST /api/v1/analysis-jobs`

### Request Headers

```http
Content-Type: application/json
Authorization: Bearer {token}
```

### Request Body

```json
{
  "video": {
    "type": "url",
    "value": "https://example.com/videos/demo.mp4"
  },
  "rubric": {
    "type": "content",
    "value": "{\"totalScore\":100,\"items\":[...]}"
  },
  "rubricData": {
    "totalScore": 100,
    "items": [
      {
        "name": "动作规范",
        "score": 40,
        "subItems": [
          {
            "requirement": "起势",
            "score": 10
          }
        ]
      }
    ]
  },
  "metadata": {
    "evaluationId": "cb6e5e70-96a6-4c0f-b850-7f1143c6b6bf",
    "taskId": "3ed25c4d-3e18-4fd1-9bf5-cf6762f72fb2",
    "videoId": "7a4c7e4b-bf85-45b1-98a2-5d5dcb620f36",
    "rubricId": "5b87fa64-3ee5-4b30-8738-d9a6fcb5d5e8",
    "storagePath": "videos/2026/04/demo.mp4"
  },
  "options": {
    "needStageSegmentation": true,
    "needErrorPoints": true,
    "needReport": true,
    "reportFormat": "html"
  }
}
```

### 字段约束

- `video.type`：可选 `url` 或 `path`
- `video.value`：必填，视频可访问 URL 或 path 路径
- `rubric.type`：当前固定为 `content`
- `rubric.value`：必填，量规 JSON 序列化后的字符串
- `rubricData`：可选，量规结构化 JSON
- `metadata`：可选，附加业务字段
- `metadata.evaluationId`：建议保留，用于业务侧和 AI 侧联调排查
- `options`：可选，当前实际会传入
  - `needStageSegmentation`
  - `needErrorPoints`
  - `needReport`
  - `reportFormat`

### Response Body

```json
{
  "jobId": "job_1dfe9c74b2cb",
  "evaluationId": "cb6e5e70-96a6-4c0f-b850-7f1143c6b6bf",
  "status": "queued",
  "acceptedAt": "2026-04-07T08:00:00Z"
}
```

### Response 字段

- `jobId`：AI 侧任务 ID
- `evaluationId`：原样回传，当前建议保留
- `status`：创建成功后返回 `queued`
- `acceptedAt`：任务受理时间

## 2. 查询任务状态

### URL

`GET /api/v1/analysis-jobs/{job_id}`

### Request Headers

```http
Authorization: Bearer {token}
```

### Request Body

无

### Response Body

```json
{
  "jobId": "job_1dfe9c74b2cb",
  "status": "processing",
  "progress": 45,
  "message": "正在生成评分结果",
  "createdAt": "2026-04-07T08:00:00Z",
  "updatedAt": "2026-04-07T08:00:08Z",
  "result_slices": null
}
```

### 字段说明

- `jobId`：AI 侧任务 ID
- `status`：任务状态
- `progress`：可选，AI 侧如能提供则返回
- `message`：可选，当前状态说明
- `createdAt`：任务创建时间
- `updatedAt`：任务最近更新时间
- `result_slices`：可选，预留给后续阶段性交付扩展；当前阶段不要求结构定义，也不作为业务侧实现依赖

### 状态枚举

- `queued`
- `processing`
- `completed`
- `failed`
- `cancelled`

## 3. 查询任务结果

### URL

`GET /api/v1/analysis-jobs/{job_id}/result`

### Request Headers

```http
Authorization: Bearer {token}
```

### Request Body

无

### Response Body

```json
{
  "jobId": "job_20260401_abcd",
  "videoStages": [
    {
      "stageId": "stage_1",
      "name": "作业准备",
      "stageType": "preparation",
      "startSec": 0,
      "endSec": 30,
      "startTime": "00:00:00",
      "endTime": "00:00:30"
    },
    {
      "stageId": "stage_2",
      "name": "输入轴分解",
      "stageType": "disassembly",
      "startSec": 30,
      "endSec": 75,
      "startTime": "00:00:30",
      "endTime": "00:01:15"
    }
  ],
  "videoPoints": [
    {
      "pointId": "point_1",
      "name": "未锁止工具车及台架",
      "type": "general_error",
      "severity": "general",
      "startSec": 0,
      "endSec": 15,
      "startTime": "00:00:00",
      "endTime": "00:00:15",
      "feedback": "视频中未见锁止工具车和台架的动作。",
      "evidences": [
        {
          "evidenceId": "ev_001",
          "kind": "image",
          "timeSec": 8.2,
          "url": "https://example.com/evidences/ev_001.jpg"
        }
      ]
    },
    {
      "pointId": "point_2",
      "name": "缺失输入轴分解全部步骤",
      "type": "serious_error",
      "severity": "serious",
      "startSec": 16,
      "endSec": 16,
      "startTime": "00:00:16",
      "endTime": "00:00:16",
      "feedback": "未见输入轴分解操作，直接进入后续环节。"
    }
  ],
  "summary": {
    "overallDescription": "该生操作流程熟练度尚可，但在标准作业流程的执行上存在重大缺失。",
    "score": 69,
    "maxScore": 100
  },
  "details": [
    {
      "title": "一、作业准备",
      "fullScore": 10,
      "aiScore": 2,
      "items": [
        {
          "subtitle": "个人防护",
          "fullScore": 2,
          "aiScore": 2,
          "status": "correct",
          "feedback": "穿戴了工作服和手套，符合要求。"
        },
        {
          "subtitle": "设备锁止",
          "fullScore": 2,
          "aiScore": 0,
          "status": "incorrect",
          "feedback": "视频中未见锁止工具车和台架的动作。"
        }
      ]
    }
  ],
  "artifacts": {
    "reportHtmlUrl": "https://example.com/reports/job_20260401_abcd.html",
    "analysisJsonUrl": "https://example.com/results/job_20260401_abcd/analysis.json",
    "evidenceIndexJsonUrl": "https://example.com/results/job_20260401_abcd/evidence_index.json"
  }
}
```

### Response 字段说明

- `jobId`：AI 侧任务 ID
- `summary`：AI 总体描述和总分
- `details`：按量规项展开的明细评分
- `videoStages`：视频阶段划分结果
- `videoPoints`：关键点、错误点、证据点
- `artifacts`：报告页、分析 JSON、证据索引等产物地址
- `severity` 枚举：`general` / `serious` / `critical`

## 4. 业务侧处理方式

业务后端按以下方式使用这 3 个接口：

1. 调用 `POST /api/v1/analysis-jobs` 创建任务
2. 保存 AI 侧返回的 `jobId`
3. 定时轮询 `GET /api/v1/analysis-jobs/{job_id}` 直到状态结束
4. 当状态为 `completed` 时，调用 `GET /api/v1/analysis-jobs/{job_id}/result`
5. 将结果信息进行入库操作
