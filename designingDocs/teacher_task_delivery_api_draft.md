# 教师端 Task 级交付能力接口草案

## 文档目的

本草案用于教师端以下 3 类能力的前后端联调设计评审：

1. `task` 级完成状态提示
2. 查看任务成绩汇总与导出任务成绩单
3. 查看任务成绩分析与生成任务分析报告

本轮先只做 `task` 级完成统计，不做 `project` 级完成提示聚合。

## 统一约束

- 所有接口均沿用当前标准响应包裹结构：

```json
{
  "code": 200,
  "message": "success",
  "data": {},
  "timestamp": "2026-04-16T10:00:00Z"
}
```

- 教师端相关接口均需要鉴权。
- 查询类接口使用 `GET`，`request body` 为空。
- 生成类接口统一采用“异步触发 + 状态查询”的模式，避免文件导出、PDF 渲染、批量报告生成阻塞业务请求。
- 本文档只设计接口 URL、Method、query/request body、response body 以及语义，不涉及数据库表、服务拆分和具体实现细节。

## 1. Task 级完成状态提示

### 范围收紧

本轮不在项目列表上增加“项目整体完成状态”按钮，只在任务列表上展示状态提示。

前端表现建议：

- 在任务列表每一行前增加一个勾选图标或状态按钮。
- 若该任务下全部视频均已完成评测，则高亮显示。
- 若未全部完成，则灰色显示。

### 完成判定口径

单视频完成条件：

- `manual_status = submitted`
- 且 `ai_status = completed`

任务完成聚合字段：

- `totalVideos`
- `completedVideos`
- `allVideosCompleted`
- `completionRate`

字段说明：

- `totalVideos`
  - 当前任务下的视频总数。
- `completedVideos`
  - 当前任务下，已经完成完整评测流程的视频数量。
  - 完整评测流程指：`manual_status = submitted` 且 `ai_status = completed`。
- `allVideosCompleted`
  - 当前任务下是否全部视频都已完成评测。
  - 可直接作为任务列表前状态按钮的高亮依据。
- `completionRate`
  - 当前任务的完成率，计算口径为 `completedVideos / totalVideos * 100`。
  - 用于展示任务进度百分比。

### 接口设计

这一块不新增接口，直接复用现有任务接口。

#### `GET /api/v1/projects/:projectId/tasks`

语义：

- 查询某项目下的任务列表。
- 前端任务列表页直接使用返回的聚合字段渲染状态按钮。

request body：

```json
{}
```

response body 关键字段基线：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "id": "task_uuid",
        "name": "发动机拆装实验 1 班",
        "status": "published",
        "totalVideos": 32,
        "completedVideos": 30,
        "allVideosCompleted": false,
        "completionRate": 93.75,
        "deadline": "2026-04-20T23:59:59Z"
      }
    ],
    "pagination": {
      "page": 1,
      "pageSize": 20,
      "total": 1,
      "totalPages": 1
    }
  },
  "timestamp": "2026-04-16T10:00:00Z"
}
```

#### `GET /api/v1/tasks/:taskId`

语义：

- 查询单个任务详情。
- 若前端后续需要在任务详情页顶部也显示完成状态，可复用此接口中的同名聚合字段。

request body：

```json
{}
```

response body 关键字段基线：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "task_uuid",
    "name": "发动机拆装实验 1 班",
    "status": "published",
    "totalVideos": 32,
    "completedVideos": 30,
    "allVideosCompleted": false,
    "completionRate": 93.75
  },
  "timestamp": "2026-04-16T10:00:00Z"
}
```

## 2. 任务成绩汇总页与任务成绩单导出

### 功能流程

1. 教师在任务列表行末点击“查看成绩”。
2. 前端进入任务成绩页或打开弹窗，调用成绩汇总接口加载当前任务下全部学生成绩。
3. 教师可按姓名、学号、完成状态进行筛选，并按 AI 分、人工分、完成时间排序。
4. 成绩页允许“已完成学生显示成绩，未完成学生显示未完成状态”。
5. 教师点击“导出任务成绩单”时，前端基于当前查询结果直接合成 Excel 并下载。

### 当前导出策略

本阶段不设计后端异步成绩单导出接口，原因如下：

- 教师需要基于“当前最新查询结果”直接导出，而不是后台生成一个延迟的快照文件。
- 成绩页允许部分学生已完成、部分学生未完成，前端直接拿最新列表数据导出更符合页面语义。
- 未完成学生在导出结果中也需要保留状态，这类即时导出更适合前端完成。

后端职责：

- 稳定提供 `scoreboard` 查询接口。
- 返回已完成与未完成学生的实时状态与成绩数据。

前端职责：

- 基于 `scoreboard` 查询结果组装 Excel。
- 已完成学生显示成绩。
- 未完成学生显示“未完成”。
- 如后续需要细分，也可依据 `aiStatus` / `manualStatus` 展示具体未完成阶段。

### 2.1 查询任务成绩汇总

#### `GET /api/v1/tasks/:taskId/scoreboard`

语义：

- 返回某个任务下全部学生的视频评测成绩汇总。
- 用于成绩页/弹窗展示。
- 也作为前端导出 Excel 的数据源。
- 接口必须支持“部分学生已完成、部分学生未完成”的混合结果返回。

query 参数：

- `page`
- `pageSize`
- `keyword`
  - 按学生姓名、学号模糊搜索
- `scope`
  - `page` / `all`
  - 页面展示默认使用 `page`
  - 前端导出时可使用 `all` 拉取当前筛选条件下的全量结果
- `evaluationStatus`
  - 例如：`completed` / `in_progress` / `failed`
- `sortBy`
  - 例如：`studentNumber` / `studentName` / `aiScore` / `manualScore` / `completedAt`
- `sortOrder`
  - `asc` / `desc`

request body：

```json
{}
```

response body：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "task": {
      "id": "task_uuid",
      "name": "发动机拆装实验 1 班",
      "status": "published",
      "totalVideos": 32,
      "completedVideos": 30,
      "allVideosCompleted": false,
      "completionRate": 93.75
    },
    "summary": {
      "totalStudents": 32,
      "completedStudents": 30,
      "averageAIScore": 82.5,
      "averageManualScore": 80.2
    },
    "items": [
      {
        "videoId": "video_uuid",
        "studentName": "张三",
        "studentNumber": "20260001",
        "aiScore": 85,
        "manualScore": 82,
        "aiStatus": "completed",
        "manualStatus": "submitted",
        "evaluationStatus": "completed",
        "completedAt": "2026-04-16T09:30:00Z",
        "displayStatus": "已完成"
      },
      {
        "videoId": "video_uuid_2",
        "studentName": "李四",
        "studentNumber": "20260002",
        "aiScore": 79,
        "manualScore": null,
        "aiStatus": "completed",
        "manualStatus": "pending",
        "evaluationStatus": "in_progress",
        "completedAt": null,
        "displayStatus": "未完成"
      }
    ],
    "pagination": {
      "page": 1,
      "pageSize": 20,
      "total": 32,
      "totalPages": 2
    }
  },
  "timestamp": "2026-04-16T10:00:00Z"
}
```

## 3. 任务成绩分析页与任务分析报告

### 功能流程

1. 教师在任务列表行末点击“成绩分析”。
2. 仅当任务下全部视频完成评测后，分析页面才允许打开。
3. 前端进入任务分析页或弹窗，调用分析数据接口展示最终统计结果。
4. 教师点击“生成任务分析报告”，前端触发报告生成。
5. 后端立即返回“已受理，正在生成”的状态。
6. 前端轮询查询当前任务对应分析报告状态，并在完成后展示访问地址。

### 3.1 查询任务成绩分析

#### `GET /api/v1/tasks/:taskId/analysis`

语义：

- 返回任务级最终分析结果，用于任务分析页展示。
- 分析页面属于最终交付页面，仅当 `allVideosCompleted = true` 时允许访问。

query 参数：

- 无

request body：

```json
{}
```

response body：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "task": {
      "id": "task_uuid",
      "name": "发动机拆装实验 1 班",
      "rubricTotalScore": 50,
      "totalVideos": 32,
      "completedVideos": 30,
      "allVideosCompleted": true
    },
    "scoreSummary": {
      "averageAIScore": 41.2,
      "averageManualScore": 40.1,
      "highestAIScore": 48,
      "lowestAIScore": 31,
      "highestManualScore": 47,
      "lowestManualScore": 30
    },
    "manualScoreDistribution": [
      {
        "label": "0-20",
        "count": 0
      },
      {
        "label": "21-40",
        "count": 4
      },
      {
        "label": "41-60",
        "count": 7
      },
      {
        "label": "61-80",
        "count": 12
      },
      {
        "label": "81-100",
        "count": 9
      }
    ],
    "aiScoreDistribution": [
      {
        "label": "0-20",
        "count": 0
      },
      {
        "label": "21-40",
        "count": 3
      },
      {
        "label": "41-60",
        "count": 8
      },
      {
        "label": "61-80",
        "count": 10
      },
      {
        "label": "81-100",
        "count": 6
      }
    ],
    "scoreGapDistribution": [
      {
        "label": "0-5",
        "count": 18
      },
      {
        "label": "6-10",
        "count": 8
      },
      {
        "label": "11+",
        "count": 4
      }
    ]
  },
  "timestamp": "2026-04-16T10:00:00Z"
}
```

说明：

- 分布图统一按满分归一化后分桶，避免不同实验因满分不同导致图表不可比较。
- 页面主分数展示仍然采用任务原始分值，不强制转成百分制。

### 3.2 生成任务分析报告

#### `POST /api/v1/tasks/:taskId/analysis-report`

语义：

- 受理当前任务的最终分析报告生成请求。
- 报告属于最终交付产物，仅当 `allVideosCompleted = true` 时允许生成。
- 采用异步生成：
  - `POST` 不同步等待 PDF 完成
  - 后端只创建或复用报告记录，并由后台 worker 处理
- 当前阶段不支持显式重生成；若已有 `queued / processing / ready` 报告，则直接返回当前有效记录。

权限语义：

- 教师/管理员可用。
- 需通过任务可管理范围校验。

request body：

```json
{}
```

response body：

```json
{
  "code": 202,
  "message": "accepted",
  "data": {
    "reportId": "analysis_report_uuid",
    "taskId": "task_uuid",
    "reportStatus": "queued",
    "reportFormat": "pdf",
    "fileName": null,
    "url": null,
    "generatedAt": null,
    "errorMessage": null
  },
  "timestamp": "2026-04-16T10:10:00Z"
}
```

状态枚举：

- `queued`
- `processing`
- `ready`
- `failed`

### 3.3 查询任务分析报告状态

#### `GET /api/v1/tasks/:taskId/analysis-report`

语义：

- 查询当前任务当前有效分析报告的状态与访问地址。
- 当前阶段不按 `reportId` 查询，直接按 `taskId` 查询当前有效报告。

request body：

```json
{}
```

response body：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "reportId": "analysis_report_uuid",
    "taskId": "task_uuid",
    "reportStatus": "ready",
    "reportFormat": "pdf",
    "fileName": "task-analysis-report-20260416.pdf",
    "url": "https://oss.example.com/reports/task-analysis-report-20260416.pdf",
    "generatedAt": "2026-04-16T10:10:15Z",
    "errorMessage": null
  },
  "timestamp": "2026-04-16T10:10:15Z"
}
```

说明：

- 当返回 `reportStatus = queued` 或 `processing` 时，前端应展示“正在生成分析报告”。
- 当前阶段推荐前端以 2 到 5 秒为间隔轮询本接口。

## 接口清单汇总

### 复用现有接口

- `GET /api/v1/projects/:projectId/tasks`
- `GET /api/v1/tasks/:taskId`

### 新增接口

- `GET /api/v1/tasks/:taskId/scoreboard`
- `GET /api/v1/tasks/:taskId/analysis`
- `POST /api/v1/tasks/:taskId/analysis-report`
- `GET /api/v1/tasks/:taskId/analysis-report`

## 当前建议

- 第一块不新增接口，先把任务列表上的聚合字段用起来。
- 第二块改为“前端基于 `scoreboard` 实时查询结果直接导出 Excel”，不再新增后端成绩单导出接口。
- 第三块按 `task` 级最终交付设计：
  - 分析页面只在任务全部完成后开放
  - 分析报告只在任务全部完成后生成
- 当前阶段不纳入批量学生报告能力。
- 如果后续单个任务学生规模显著增大，前端全量拉取再导出变重，再补后端异步成绩单导出接口更合适。
