# AI 评测接口

本文档仅整理当前需要与 AI 侧对齐的 3 个服务间接口。

当前模式：

- 业务后端调用 AI 侧创建分析任务
- 业务后端按 `jobId` 轮询任务状态
- 当状态为 `completed` 时，业务后端主动拉取最终结果并入库
- 当前阶段不设计分期交付协议，仅保留可选的 `result_slices` 扩展字段
- 业务侧当前会在视频上传确认成功后自动发起 AI 创建请求

## 0. 当前阶段最小改造约束

当前阶段只对模型服务做最小改造，业务侧现有接口保持不变。

### 0.1 输入方式

- 业务侧继续传入 `video.type=url`
- 模型侧负责根据 URL 下载视频
- 当前不要求业务侧先把视频上传到模型侧

`video.value` 在当前阶段应理解为“模型侧可访问的下载地址”，通常是：

- OBS 对象存储的签名 URL
- 业务侧生成的短时播放 URL
- 其他模型侧网络可达的直链地址

### 0.2 模型侧缓存职责

模型侧分两类缓存：

- 视频临时缓存
- 任务结果缓存

#### 视频临时缓存

模型侧下载完成后，视频先落本地临时目录，例如：

- `/tmp/skilljudge-ai/{jobId}/source.mp4`

缓存策略：

- 视频缓存保留 5 分钟
- 5 分钟从“任务成功产出报告”开始计算
- 这样可以覆盖 Gemini 结果异常、模型超时、需要立即重试等场景

#### 任务结果缓存

模型侧还应保留一份短期 job 状态和结果，建议 TTL 为 24 小时。

目的：

- 避免业务侧在轮询成功前因网络抖动丢失结果
- 允许业务侧在写 PG 失败后，基于 `jobId` 再次拉取结果

### 0.3 模型侧存储建议

当前阶段不建议模型侧引入 MongoDB。

建议存储划分如下：

- 本地磁盘：视频临时文件和必要的转码中间文件
- Redis：job 状态、短期结果、缓存过期时间
- 业务侧 PostgreSQL：最终 AI 报告和摘要字段

### 0.4 最终报告存储原则

最终结构化报告仍由业务侧在成功拉取结果后写入 PostgreSQL：

- `ai_evaluations.result_data`
- `ai_evaluations.total_score`
- `videos.ai_status`
- `videos.ai_score`

模型侧不承担长期报告库职责，只提供短期补偿窗口。

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

- `video.type`：当前固定为 `url`
- `video.value`：必填，模型侧可访问的视频下载地址
- `rubric.type`：当前固定为 `content`
- `rubric.value`：必填，量规 JSON 序列化后的字符串
- `rubricData`：可选，量规结构化 JSON
- `metadata`：可选，附加业务字段
- `metadata.evaluationId`：建议保留，用于业务侧和 AI 侧联调排查
- `metadata.storagePath`：可选，仅用于排障定位，不作为模型侧直接读取对象
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
5. 将结果信息写入业务侧 PostgreSQL

## 5. 推荐的补偿策略

为避免以下情况导致结果丢失：

- 模型侧已完成，但业务侧查询状态时网络异常
- 业务侧拿到 `completed` 后，拉结果接口失败
- 业务侧拉到结果后，写 PostgreSQL 失败

当前推荐：

- 模型侧将 job 状态和 result 在 Redis 中保留 24 小时
- 业务侧保存 `jobId`
- 业务侧写库失败时，后续可按 `jobId` 再次补拉结果

## 6. 推荐的清理策略

### 视频文件

- 任务成功后开始计算 5 分钟 TTL
- TTL 到期后删除本地视频缓存

### 结果缓存

- job 状态和 result 建议保留 24 小时
- 到期后由模型侧清理 Redis key

该策略满足当前最小改造目标：

- 不引入模型侧长期数据库
- 不改变业务侧接口形态
- 保留短期重试与补偿能力
