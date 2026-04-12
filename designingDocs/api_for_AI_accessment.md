# AI 评分接入方案

## 1. 背景与目标

当前系统已经具备以下基础能力：

- 按 `task` 维度上传视频
- 按 `task` 为视频分配评分员
- 评分员基于视频和量规完成人工评分
- 在 `videos` 表中预留了 `ai_status`、`ai_score`
- 在视频详情和评分详情 DTO 中预留了 `aiEvaluation`

当前后端已经完成基础 AI 评分链路接入，并复用了数据库中的 `ai_evaluations` 表。现阶段需要继续根据最新外部对接协议，把后端与 AI 服务之间的交互统一到“创建任务 + 轮询状态 + 拉取结果”的模式。

本方案的目标是：

- 在不推翻现有后端结构的前提下接入 AI 评分
- 保持 AI 评分为异步任务模式
- 让 AI 评分与现有 `video`、`task`、`rubric`、`manual evaluation` 流程自然衔接
- 让前端可以稳定查询 AI 状态和 AI 结果

## 2. 设计原则

- AI 评分的业务主体是现有 `video`，不重新引入独立的“评分任务实体”
- AI 评分结果采用异步 job 模型，而不是同步长请求
- 对外接口使用现有业务 ID，不暴露本地路径
- AI 原始结果与对前端暴露的业务结果分层存储
- `videos.ai_status`、`videos.ai_score` 只承担摘要和快速展示职责
- AI 不直接改写人工评分主流程状态，避免影响当前人工评分链路

## 3. 与现有系统的关系

### 3.1 当前业务链路

当前后端的实际链路是：

1. 创建项目
2. 在项目下创建 `task`
3. 视频按 `task` 上传
4. 教师或管理员为视频分配评分员
5. 评分员在 `/api/v1/tasks/:id` 查看待评视频详情
6. 评分员提交人工评分

也就是说：

- 当前系统的“评分对象”本质上是 `video`
- `task` 是项目、量规、视频、评分权限之间的稳定桥梁
- scorer 侧的 `/api/v1/tasks/:id` 实际是“被分配视频的评分视图”

### 3.2 为什么不能直接照搬 AI 方文档

AI 方给出的 `external_api_design.md` 是合理的异步分析方案，但不是当前系统的直接接口设计：

- 该文档假设对外存在独立的 `POST /api/v1/videos` 上传接口
- 该文档假设创建分析任务时需要显式传 `video`、`rubric`
- 该文档更像一个通用分析平台 API，而不是当前教学评分系统的业务 API

而当前后端已经有：

- 自己的上传流程：`/api/v1/videos/upload-credential`、`/api/v1/videos/:id/confirm-upload`
- 现成的 `task` 和 `rubric` 绑定关系
- 已有的人工评分页面和 DTO 结构

因此更适合采用“在现有领域模型上增加 AI evaluation 子域”的做法。

## 4. 总体方案

### 4.1 核心思路

复用平台库中已经存在的 `ai_evaluations` 表来记录 AI 任务状态和 AI 结果。

其中：

- `video` 是 AI 评分的业务归属对象
- `ai_evaluations` 是 AI 评分的异步执行对象
- `videos.ai_status` 和 `videos.ai_score` 是给现有列表页和详情页快速读取的摘要字段

### 4.2 当前阶段的最小改造边界

当前阶段优先只改“模型服务”这一层，不改业务侧对外接口形态：

- 业务侧仍然向模型侧提交 `video.type=url`
- URL 可以是对象存储中的 OBS 播放地址或签名下载地址
- 模型侧负责下载视频、必要的预处理、调用视觉模型、生成结构化报告
- 业务侧仍然通过 `jobId` 轮询模型侧，拿到最终结果后写入平台库 `ai_evaluations.result_data`

也就是说：

- 模型侧负责“处理”
- 业务侧负责“最终持久化”和“对前端查询输出”

### 4.3 最小改造后的职责分层

#### 业务侧职责

- 创建 AI 评估记录
- 调用模型侧创建分析任务
- 保存模型侧返回的 `jobId`
- 轮询模型侧任务状态
- 在任务完成后拉取最终结果
- 将最终结果持久化到 PostgreSQL 的 `ai_evaluations`
- 前端查询报告时优先只读业务侧 PostgreSQL

#### 模型侧职责

- 根据业务侧传入的 URL 下载视频
- 如有必要执行降帧、降分辨率、转码等预处理
- 调用 Gemini 或其他视觉模型
- 输出统一结构化结果
- 在短时间内保留任务状态、结果和视频缓存，供业务侧补拉与重试

### 4.4 为什么最终报告仍然存业务侧 PG

当前平台后端已经具备以下能力：

- `ai_evaluations` 可保存 AI 状态、总分、完整结果
- `videos.ai_status`、`videos.ai_score` 可保存摘要信息
- 当前前端和业务查询链路已经围绕平台库展开

因此当前阶段不再引入 MongoDB 作为模型侧报告库，避免出现：

- 一份结果在模型侧
- 一份结果在业务侧
- 双方状态不一致
- 前端查询路径分裂

当前推荐做法是：

- 业务侧 PostgreSQL 是最终事实源
- 模型侧只保留短期任务缓存，不承担长期报告存储职责

### 4.5 建议后的主流程

1. 视频上传完成，状态变为 `ready`
2. 视频上传确认后，若 AI 已配置，则后端自动触发该视频的 AI 评分
3. 后端创建 `ai_evaluations` 记录，状态为 `processing`
4. 后端调用外部 AI 服务提交分析任务
5. 外部 AI 服务异步执行
6. 后端按 `jobId` 轮询 AI 任务状态
7. 当状态为 `completed` 时，后端主动拉取 AI 结果
8. 后端更新 `ai_evaluations`
9. 后端同步更新 `videos.ai_status`、`videos.ai_score`
10. 视频详情和评分详情接口返回最新 AI 结果

### 4.6 模型侧临时缓存策略

#### 视频缓存

模型侧收到任务后，先将视频下载到本地临时目录，例如：

- `/tmp/skilljudge-ai/{jobId}/source.mp4`

缓存策略建议如下：

- 视频下载成功后开始处理
- 视频缓存的 5 分钟倒计时，从“成功产出报告”开始计算，而不是从下载成功开始计算
- 这样可以覆盖“模型结果异常，需要快速重跑一次”的场景

推荐规则：

- 任务成功后：视频临时文件保留 5 分钟
- 任务失败后：可保留 5 到 15 分钟，便于排障；若当前阶段只做最小实现，也可以统一按 5 分钟处理
- 到期后由后台清理任务删除本地临时文件

#### 任务结果缓存

模型侧还需要短期保留任务状态和最终结果，原因是：

- 业务侧创建任务成功后，轮询和拉取结果可能因为网络或服务抖动失败
- 即使模型侧已经完成分析，也需要给业务侧留出一个补拉窗口

因此建议：

- 模型侧任务状态和结果保留 24 小时
- 业务侧在写入 PG 成功后，不依赖模型侧长期保存

### 4.7 模型侧存储建议

当前阶段不建议模型侧引入 MongoDB 作为报告存储。

建议采用：

- 本地磁盘：存视频临时文件
- Redis：存任务状态和短期结果
- 业务侧 PostgreSQL：存最终报告

#### 本地磁盘

用于保存：

- 下载后的源视频
- 必要时的转码后中间文件

特点：

- 适合大文件
- 便于转码、重试和快速删除

#### Redis

用于保存：

- `jobId`
- `status`
- `evaluationId`
- `videoId`
- `videoUrl` 或其摘要
- `completedAt`
- `videoCachePath`
- `videoExpireAt`
- 模型侧返回的结构化结果 JSON
- `errorMessage`

特点：

- 自带 TTL，适合短期缓存
- 服务重启后仍可保留短期任务结果
- 便于业务侧在写库失败时重新查询

#### PostgreSQL

继续由业务侧负责：

- `ai_evaluations.status`
- `ai_evaluations.total_score`
- `ai_evaluations.result_data`
- `videos.ai_status`
- `videos.ai_score`

### 4.8 失败补偿与查询策略

建议统一采用以下查询策略：

- 用户或前端查询 AI 报告时，默认只查询业务侧 PostgreSQL
- 如果业务侧已成功落库，则不再依赖模型侧
- 如果业务侧在模型侧已完成后因网络或瞬时错误导致未落库，可通过保留的 `jobId` 再次从模型侧补拉结果

因此模型侧保留 24 小时结果缓存的目的，不是承担长期查询，而是承担补偿窗口。

## 5. 数据模型设计

## 5.1 保留现有字段

保留 `videos` 表中的以下字段：

- `ai_status`
- `ai_score`

用途：

- `ai_status` 用于视频列表、评分列表快速展示 AI 当前进度
- `ai_score` 用于与人工评分做对比

说明：

- `evaluation_status` 继续表示当前人工评分主状态
- AI 评分不直接驱动 `evaluation_status`

## 5.2 复用现有表：`ai_evaluations`

当前平台库文档中，`ai_evaluations` 已经存在，字段定义如下：

```sql
ai_evaluations
---------------
id uuid pk
task_id uuid not null
video_id uuid not null
model_version varchar(50)
total_score decimal(5,2)
status varchar(20) not null default 'processing'
started_at timestamp
completed_at timestamp
error_message text
result_data jsonb
created_at timestamp
updated_at timestamp
```

本方案优先复用这套现有表结构，不重复建表。

如果后续确实需要更强的任务追踪能力，例如：

- 外部 provider job id
- 任务阶段
- 处理进度
- 原始请求体
- 产物 URL 独立字段

则再通过 migration 增量扩展，而不是现在直接重做表设计。

### 5.3 字段说明

- `video_id`
  关联现有视频
- `task_id`
  便于权限校验、按任务批量查询
- `status`
  AI 任务状态
- `model_version`
  AI 模型版本
- `total_score`
  AI 总分摘要
- `result_data`
  AI 完整结果 JSON，包含结构化明细、分项解释、证据索引入口等
- `error_message`
  失败诊断信息

### 5.4 当前表已满足的核心用途

现有 `ai_evaluations` 已经足够承接当前阶段：

- 一个视频多次 AI 评测历史
- AI 总分摘要回填
- AI 完整结果落库
- 失败重试留痕

### 5.5 当前轮询方案建议补充的字段

轮询模式下，业务后端需要保存 AI 侧返回的 `jobId`，否则无法稳定查询状态和结果。

因此建议优先补充：

- `provider_job_id`

建议类型：

- `varchar(100)` 或同等长度字符串字段

建议索引：

- `provider_job_id` 唯一或普通索引

### 5.6 后续可补充的索引或字段

- `(video_id, created_at desc)`
- `(task_id, created_at desc)`
- `(status)`

## 6. 状态机设计

### 6.1 AI 任务状态

`ai_evaluations.status` 建议优先按当前库设计统一为：

- `processing`
- `completed`
- `failed`

说明：

- 当前数据库参考文档里该字段默认值就是 `processing`
- 当前阶段不强行引入 `queued/running/succeeded/cancelled` 这套更细粒度状态
- 如果后续真的需要更细阶段，再在当前表结构上扩展即可

### 6.2 视频摘要状态

`videos.ai_status` 建议统一为：

- `pending`
- `processing`
- `completed`
- `failed`

映射关系建议：

- 创建 AI 任务后 -> `processing`
- AI 任务成功 -> `completed`
- `failed` -> `failed`

说明：

- `pending` 表示尚未发起 AI 评分
- `processing`、`completed` 是当前库文档中的常见值
- 这里补充 `failed`，用于覆盖失败场景，避免失败后无状态可落

### 6.3 人工评分摘要状态

`videos.manual_status` 建议统一为：

- `pending`
- `in_progress`
- `submitted`

说明：

- 这套状态与当前数据库文档一致
- 也更贴近 `manual_evaluations.status` 的业务语义

### 6.4 总体评分状态

`videos.evaluation_status` 建议继续保持：

- `pending`
- `in_progress`
- `completed`

### 6.5 当前阶段扩展

当前 `ai_evaluations` 表中没有独立的 `current_stage` 字段，因此当前阶段不把它作为正式持久化字段。

如果后续确实需要更细粒度的任务进度展示，再考虑扩展字段，例如：

- `queued`
- `precheck`
- `global_understanding`
- `stage_segmentation`
- `error_detection`
- `scoring`
- `report_generation`
- `completed`

## 7. 接口设计

## 7.1 单视频触发 AI 评分

```http
POST /api/v1/videos/:id/ai-evaluations
Authorization: Bearer {token}
Content-Type: application/json
```

Request Body:

```json
{
  "force": false
}
```

说明：

- `:id` 为现有 `videoId`
- 不要求前端再传 `taskId` 或 `rubricId`
- 后端根据 `video.task_id` 和 `task.rubric_id` 自动解析
- `force=true` 表示即使已有成功结果也允许重新发起一次

Response:

```json
{
  "code": 201,
  "message": "success",
  "data": {
    "id": "uuid",
    "videoId": "uuid",
    "taskId": "uuid",
    "status": "processing",
    "modelVersion": null,
    "totalScore": null,
    "errorMessage": null,
    "startedAt": "2026-04-02T10:00:00Z",
    "completedAt": null,
    "createdAt": "2026-04-02T10:00:00Z",
    "updatedAt": "2026-04-02T10:00:00Z"
  },
  "timestamp": "2026-04-02T10:00:00Z"
}
```

权限建议：

- `admin`
- `school_admin`
- `school_leader`
- `teacher`

规则建议：

- 视频必须存在
- 视频必须处于 `ready`
- 视频必须绑定 `task`
- `task` 必须存在且能解析到 `rubric`
- 同一视频若已有 `processing` 的 AI 任务，默认不允许重复创建

## 7.2 任务下批量触发 AI 评分

```http
POST /api/v1/tasks/:id/ai-evaluations
Authorization: Bearer {token}
Content-Type: application/json
```

Request Body:

```json
{
  "videoIds": [
    "uuid1",
    "uuid2"
  ],
  "force": false
}
```

Response:

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
        "videoId": "uuid2",
        "error": "video is already being evaluated"
      }
    ]
  },
  "timestamp": "2026-04-02T10:00:00Z"
}
```

说明：

- 用于教师在任务维度批量触发
- 采用部分成功语义，不建议整批事务

## 7.3 查询 AI 任务状态

```http
GET /api/v1/ai-evaluations/:id
Authorization: Bearer {token}
```

Response:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "uuid",
    "videoId": "uuid",
    "taskId": "uuid",
    "status": "processing",
    "modelVersion": "v1.2.0",
    "totalScore": null,
    "errorMessage": null,
    "createdAt": "2026-04-02T10:00:00Z",
    "updatedAt": "2026-04-02T10:03:00Z",
    "startedAt": "2026-04-02T10:00:10Z",
    "completedAt": null
  },
  "timestamp": "2026-04-02T10:03:00Z"
}
```

## 7.4 查询 AI 任务结果

```http
GET /api/v1/ai-evaluations/:id/result
Authorization: Bearer {token}
```

Response:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": "uuid",
    "status": "completed",
    "videoId": "uuid",
    "taskId": "uuid",
    "summary": {
      "overallDescription": "该生操作流程基本完整，但存在若干关键扣分点。",
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
          }
        ]
      }
    ],
    "videoStages": [
      {
        "stageId": "stage_1",
        "name": "作业准备",
        "stageType": "preparation",
        "startSec": 0,
        "endSec": 30,
        "startTime": "00:00:00",
        "endTime": "00:00:30"
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
      }
    ],
    "artifacts": {
      "reportHtmlUrl": "https://example.com/reports/job_20260401_abcd.html",
      "analysisJsonUrl": "https://example.com/results/job_20260401_abcd/analysis.json",
      "evidenceIndexJsonUrl": "https://example.com/results/job_20260401_abcd/evidence_index.json"
    },
    "modelVersion": "v1.2.0",
    "completedAt": "2026-04-02T10:06:00Z"
  },
  "timestamp": "2026-04-02T10:06:00Z"
}
```

说明：

- 若任务尚未完成，返回 `409` 或 `404/400` 均可，但建议统一为 `409 result not ready`
- 对前端返回的是稳定业务结构，不直接暴露 AI 原始内部字段命名

## 7.5 后端轮询外部 AI 服务

当前不再采用 callback 模式，统一改为业务后端主动轮询。

业务后端与 AI 服务之间使用如下 3 个接口：

- `POST /api/v1/analysis-jobs`
- `GET /api/v1/analysis-jobs/{job_id}`
- `GET /api/v1/analysis-jobs/{job_id}/result`

处理方式建议：

1. 创建本地 `ai_evaluations` 记录
2. 调用 `POST /api/v1/analysis-jobs`
3. 保存 AI 侧返回的 `jobId`
4. 定时轮询 `GET /api/v1/analysis-jobs/{job_id}`
5. 当状态为 `completed` 时，调用 `GET /api/v1/analysis-jobs/{job_id}/result`
6. 将最终结果落到 `ai_evaluations.result_data`

状态查询接口中的 `result_slices` 当前仅作为可选保留字段，当前阶段不要求定义具体结构，也不作为后端落库依赖。

## 8. 外部 AI 请求映射

## 8.1 不建议前端直传量规文本

当前系统已经有稳定的 `rubric` 实体和 JSON 结构，因此不建议前端创建 AI 任务时再传：

- `rubric content`
- `rubric local_path`

更合理的做法是由后端读取现有 `task.rubric`，并转换为 AI 服务需要的结构。

## 8.2 向 AI 服务发送的内容

后端给 AI 的请求建议包含：

- `video`
- `rubric`
- `rubricData`
- `metadata`
- `options`

示例：

```json
{
  "video": {
    "type": "url",
    "value": "https://storage.example.com/projects/{projectId}/tasks/{taskId}/videos/{videoId}.mp4"
  },
  "rubric": {
    "type": "content",
    "value": "{\"id\":\"uuid\",\"name\":\"变速器拆装评分细则\",\"totalScore\":100,\"items\":[...]}"
  },
  "rubricData": {
    "id": "uuid",
    "name": "变速器拆装评分细则",
    "totalScore": 100,
    "items": []
  },
  "metadata": {
    "evaluationId": "uuid",
    "taskId": "uuid",
    "videoId": "uuid",
    "rubricId": "uuid",
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

## 8.3 为什么不直接照搬 AI 方输入结构

原因如下：

- 当前系统中视频来源已经稳定，不需要重新支持 `local_path`
- 当前系统中量规由 `task` 绑定，不应由前端重复指定
- 当前系统权限和学校范围是围绕现有业务对象做的，后端自己解析上下文更安全

## 9. 与现有接口的整合方式

## 9.1 视频详情接口

在现有 `GET /api/v1/videos/:id` 中补充：

- `aiEvaluation`

推荐返回：

- 最新一条成功 AI 结果
- 或者最近一条 `processing` 中的 AI 任务摘要

示例：

```json
{
  "id": "video-uuid",
  "aiStatus": "completed",
  "aiScore": 75.5,
  "aiEvaluation": {
    "evaluationId": "eval-uuid",
    "status": "completed",
    "summary": {},
    "details": [],
    "videoStages": [],
    "videoPoints": [],
    "artifacts": {}
  }
}
```

## 9.2 评分详情接口

在现有 scorer 侧 `GET /api/v1/tasks/:id` 中补充同样的 `aiEvaluation`。

这样可以直接复用现在的页面结构：

- `rubric` 展示评分标准
- `aiEvaluation` 展示 AI 建议评分
- `manualEvaluation` 展示人工评分

## 9.3 提交人工评分后的对比

现有人工提交结果中已经有 `comparison.aiScore` 结构。

因此只要：

- 在 AI 任务完成时同步写 `videos.ai_score`

就可以继续复用现有对比逻辑，不需要再改动主提交流程。

## 10. 权限设计

### 10.1 触发 AI 评分

建议仅以下角色允许触发：

- `admin`
- `school_admin`
- `school_leader`
- `teacher`

不建议 `scorer` 触发 AI 评分，避免评分员在评分执行中反复发起新 AI 任务。

### 10.2 查询 AI 任务与结果

查询权限建议与视频访问权限一致：

- 能访问该视频的人，就能访问该视频下的 AI 状态和结果

这样可以直接复用现有：

- task/project scope 校验
- school scope 校验
- teacher 自有项目校验

## 11. 幂等与错误处理

### 11.1 创建任务幂等

默认策略：

- 同一视频如果已有 `processing` 的 AI 任务，禁止再次创建
- 若最新一次是 `failed` 或 `completed`，可通过 `force=true` 重跑

### 11.2 轮询幂等

轮询处理必须满足：

- 同一条状态被重复拉取，不会写坏结果
- 已完成任务再次查询到旧状态时，不应回退状态
- 相同结果被重复拉取时，不会造成重复写入或错误覆盖

建议策略：

- 仅允许状态单向推进
- 若当前已 `completed`，忽略更早的 `processing`
- 以本地已保存的最终态为准，避免轮询结果回退

### 11.3 常见错误码建议

- `video_not_found`
- `video_not_ready`
- `task_not_found`
- `rubric_not_found`
- `evaluation_not_found`
- `evaluation_already_processing`
- `evaluation_result_not_ready`
- `ai_provider_request_failed`
- `ai_provider_response_invalid`
- `internal_error`

## 12. 实施建议

## 12.1 当前阶段

先做最小闭环：

- 复用现有 `ai_evaluations` 表
- 新增单视频触发接口
- 新增状态查询接口
- 新增结果查询接口
- 新增后台轮询任务
- 完成视频详情、评分详情里的 `aiEvaluation` 回填

说明：

- 当前已经接入上传确认后的自动触发
- Phase 1 不做队列系统
- Phase 1 先支持单供应方

## 12.2 Phase 2

再补：

- 批量页面展示
- 更丰富的失败重试
- 任务级批量触发
- 轮询任务重试与补偿

## 12.3 Phase 3

最后再考虑：

- 上传完成后自动触发 AI
- SSE 或 websocket 推送任务状态
- 多 AI 供应方切换
- 人工评分与 AI 评分差异分析

## 13. 推荐结论

最终推荐方案是：

- 接受 AI 方“异步分析任务 + 状态查询 + 结果查询分离”的大方向
- 不直接照搬其外部接口路径和输入结构
- 以当前系统的 `video` 为 AI 评分归属对象
- 复用现有 `ai_evaluations` 子域承接 AI 任务状态和结果
- 通过保存 `provider_job_id` 对接 AI 侧轮询协议
- 保留 `videos.ai_status`、`videos.ai_score` 作为摘要字段
- 把 AI 结果回填进现有视频详情和评分详情接口

这样可以最大化复用现有后端实现，最小化对当前人工评分主流程的冲击，同时为后续 AI 评分扩展留下足够空间。
