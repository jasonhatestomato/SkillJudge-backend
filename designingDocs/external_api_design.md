# 对外 API 设计草案

## 1. 目标

当前系统适合设计成异步任务型 API，而不是同步长请求。

推荐的整体流程：

1. 上传离线视频，拿到 `video_id`
2. 提交视频分析任务，拿到 `job_id`
3. 轮询任务状态，直到 `succeeded` / `failed` / `cancelled`
4. 任务成功后拉取最终结果

这样设计的原因：

- 单次分析耗时较长，包含 agent、多 MCP、报告生成
- 视频、证据图、结构化产物都较大，不适合同步一次性返回
- 后续可以自然扩展 webhook、SSE、重试、缓存与复用

## 2. 设计原则

- 对外主输入优先使用 ID，不直接依赖本地绝对路径
- 状态查询与结果查询分离
- 结果返回结构化 JSON，报告与证据图片通过 URL 引用
- 时间字段同时返回秒数和字符串，方便程序消费和前端展示
- 对外契约以业务语义为主，不暴露内部实现细节

说明：

- `video_path` / `rubric_md_path` 可以作为内部兼容输入保留，但不建议作为正式公开字段
- 图片证据优先返回 URL，不建议在主结果接口里直接返回大量 base64
- 如需单文件导出，可额外生成自包含 HTML，在报告内部嵌入 base64 缩略图

## 3. 核心对象

### 3.1 视频对象

```json
{
  "video_id": "vid_001",
  "file_name": "demo.mp4",
  "storage_key": "videos/2026/04/01/vid_001.mp4",
  "local_path": "/mnt/data/videos/vid_001.mp4",
  "status": "uploaded",
  "size_bytes": 182736451,
  "duration_sec": 512.4
}
```

说明：

- `storage_key` 用于对象存储或内部文件索引
- `local_path` 可选，仅在内网部署且确有需要时返回

### 3.2 量规对象

量规输入建议支持两种模式：

1. `rubric_id`
2. 直接传量规文本内容

示例：

```json
{
  "type": "rubric_id",
  "value": "rubric_gearbox_v1"
}
```

```json
{
  "type": "content",
  "value": "# 评价指标\n..."
}
```

### 3.3 任务对象

```json
{
  "job_id": "job_20260401_abcd",
  "status": "queued",
  "progress": 0,
  "current_stage": "queued",
  "message": "任务已创建",
  "created_at": "2026-04-01T10:00:00Z",
  "updated_at": "2026-04-01T10:00:00Z"
}
```

状态枚举建议：

- `queued`
- `running`
- `succeeded`
- `failed`
- `cancelled`

## 4. API 列表

主流程建议保留 4 个接口：

1. `POST /api/v1/videos`
2. `POST /api/v1/analysis-jobs`
3. `GET /api/v1/analysis-jobs/{job_id}`
4. `GET /api/v1/analysis-jobs/{job_id}/result`

可选扩展接口：

- `GET /api/v1/analysis-jobs/{job_id}/events`
- `POST /api/v1/analysis-jobs/{job_id}/cancel`
- `POST /api/v1/rubrics`

## 5. 接口 1：上传视频

### 5.1 请求

```http
POST /api/v1/videos
Content-Type: multipart/form-data
```

表单字段：

- `file`: 视频文件

### 5.2 响应

```json
{
  "video_id": "vid_001",
  "file_name": "demo.mp4",
  "storage_key": "videos/2026/04/01/vid_001.mp4",
  "local_path": "/mnt/data/videos/vid_001.mp4",
  "status": "uploaded",
  "size_bytes": 182736451,
  "duration_sec": 512.4
}
```

### 5.3 说明

- 若视频较大，推荐后续支持分片上传或直传对象存储
- 当前阶段简单版可先走单文件上传

## 6. 接口 2：创建分析任务

### 6.1 请求

```http
POST /api/v1/analysis-jobs
Content-Type: application/json
```

```json
{
  "video": {
    "type": "video_id",
    "value": "vid_001"
  },
  "rubric": {
    "type": "rubric_id",
    "value": "rubric_gearbox_v1"
  },
  "options": {
    "report_format": "html",
    "need_stage_segmentation": true,
    "need_error_points": true,
    "need_report": true
  }
}
```

兼容输入也可支持：

```json
{
  "video": {
    "type": "local_path",
    "value": "/mnt/data/videos/demo.mp4"
  },
  "rubric": {
    "type": "content",
    "value": "# 评价指标\n..."
  }
}
```

### 6.2 响应

```json
{
  "job_id": "job_20260401_abcd",
  "status": "queued"
}
```

### 6.3 输入字段建议

`video`：

- `type`: `video_id | local_path`
- `value`: 字符串

`rubric`：

- `type`: `rubric_id | content | local_path`
- `value`: 字符串

`options`：

- `report_format`: `html | md`
- `need_stage_segmentation`: 是否需要视频阶段分解
- `need_error_points`: 是否需要错误操作标注
- `need_report`: 是否生成可展示报告

说明：

- 不建议对外暴露 `run_mode=smoke`
- `output_basename` 不应作为对外契约的一部分

## 7. 接口 3：查询任务状态

这个接口用于前端或业务侧轮询。

### 7.1 请求

```http
GET /api/v1/analysis-jobs/{job_id}
```

### 7.2 响应

```json
{
  "job_id": "job_20260401_abcd",
  "status": "running",
  "progress": 45,
  "current_stage": "error_detection",
  "message": "正在分析输入轴分解阶段",
  "created_at": "2026-04-01T10:00:00Z",
  "updated_at": "2026-04-01T10:03:12Z"
}
```

### 7.3 `current_stage` 建议枚举

- `queued`
- `precheck`
- `global_understanding`
- `stage_segmentation`
- `error_detection`
- `scoring`
- `report_generation`
- `completed`

### 7.4 说明

- 前端可按固定间隔轮询
- 如果后续需要降低轮询压力，可补充 SSE 或 webhook

## 8. 接口 4：查询最终结果

任务成功后，由该接口返回完整结构化结果。

### 8.1 请求

```http
GET /api/v1/analysis-jobs/{job_id}/result
```

### 8.2 响应

```json
{
  "job_id": "job_20260401_abcd",
  "status": "succeeded",
  "video_stages": [
    {
      "stage_id": "stage_1",
      "name": "作业准备",
      "stage_type": "preparation",
      "start_sec": 0,
      "end_sec": 30,
      "start_time": "00:00:00",
      "end_time": "00:00:30"
    },
    {
      "stage_id": "stage_2",
      "name": "输入轴分解",
      "stage_type": "disassembly",
      "start_sec": 30,
      "end_sec": 75,
      "start_time": "00:00:30",
      "end_time": "00:01:15"
    }
  ],
  "video_points": [
    {
      "point_id": "point_1",
      "name": "未锁止工具车及台架",
      "type": "general_error",
      "severity": "general",
      "start_sec": 0,
      "end_sec": 15,
      "start_time": "00:00:00",
      "end_time": "00:00:15",
      "feedback": "视频中未见锁止工具车和台架的动作。",
      "evidences": [
        {
          "evidence_id": "ev_001",
          "kind": "image",
          "time_sec": 8.2,
          "url": "https://example.com/evidences/ev_001.jpg"
        }
      ]
    },
    {
      "point_id": "point_2",
      "name": "缺失输入轴分解全部步骤",
      "type": "serious_error",
      "severity": "serious",
      "start_sec": 16,
      "end_sec": 16,
      "start_time": "00:00:16",
      "end_time": "00:00:16",
      "feedback": "未见输入轴分解操作，直接进入后续环节。"
    }
  ],
  "summary": {
    "overall_description": "该生操作流程熟练度尚可，但在标准作业流程的执行上存在重大缺失。",
    "score": 69,
    "max_score": 100
  },
  "details": [
    {
      "title": "一、作业准备",
      "full_score": 10,
      "ai_score": 2,
      "items": [
        {
          "subtitle": "个人防护",
          "full_score": 2,
          "ai_score": 2,
          "status": "correct",
          "feedback": "穿戴了工作服和手套，符合要求。"
        },
        {
          "subtitle": "设备锁止",
          "full_score": 2,
          "ai_score": 0,
          "status": "incorrect",
          "feedback": "视频中未见锁止工具车和台架的动作。"
        }
      ]
    }
  ],
  "artifacts": {
    "report_html_url": "https://example.com/reports/job_20260401_abcd.html",
    "analysis_json_url": "https://example.com/results/job_20260401_abcd/analysis.json",
    "evidence_index_json_url": "https://example.com/results/job_20260401_abcd/evidence_index.json"
  }
}
```

## 9. 结果字段设计建议

### 9.1 `video_stages`

用于视频阶段分解。

字段建议：

- `stage_id`
- `name`
- `stage_type`
- `start_sec`
- `end_sec`
- `start_time`
- `end_time`

说明：

- 后端不建议直接返回 `color`
- 前端可根据 `stage_type` 自行映射颜色

### 9.2 `video_points`

用于错误操作、风险操作或关键操作缺失点标注。

字段建议：

- `point_id`
- `name`
- `type`
- `severity`
- `start_sec`
- `end_sec`
- `start_time`
- `end_time`
- `feedback`
- `evidences`

`severity` 建议枚举：

- `general`
- `serious`
- `critical`

`type` 可按业务定义更细枚举，例如：

- `general_error`
- `serious_error`
- `critical_error`
- `missing_step`
- `safety_risk`

### 9.3 `summary`

用于页面总览。

字段建议：

- `overall_description`
- `score`
- `max_score`

### 9.4 `details`

用于逐项评分明细。

建议统一命名，避免混用 `subscore` / `subsubscore` / `AIscore`。

推荐结构：

- 一级：`title`、`full_score`、`ai_score`
- 二级：`subtitle`、`full_score`、`ai_score`、`status`、`feedback`

`status` 建议枚举：

- `correct`
- `incorrect`
- `partial`
- `uncertain`
- `not_applicable`

## 10. 图片与证据返回方式

推荐主方案：返回 URL。

例如：

```json
{
  "evidence_id": "ev_001",
  "kind": "image",
  "time_sec": 8.2,
  "url": "https://example.com/evidences/ev_001.jpg"
}
```

原因：

- 结果 JSON 不会过大
- 前端展示和缓存更简单
- HTML 报告可以直接引用
- 后续扩展视频 clip 也更自然

不推荐默认方案：在主结果接口中直接返回大量 base64。

可接受的例外：

- 导出单文件 HTML 时，将少量缩略图转为 base64 内嵌

## 11. 错误响应建议

统一错误格式：

```json
{
  "error": {
    "code": "invalid_video_source",
    "message": "video file not found",
    "retryable": false
  }
}
```

常见错误码建议：

- `invalid_video_source`
- `invalid_rubric_source`
- `job_not_found`
- `result_not_ready`
- `unsupported_file_type`
- `upload_failed`
- `internal_error`

## 12. 与当前系统的映射关系

当前系统内部已有这些产物：

- `run_summary.json`
- `analysis.json`
- `evidence_index.json`
- `report.html`

建议的对外接口可以这样映射：

- `summary` 主要来自 `run_summary.json`
- `details` 主要来自 `analysis.json`
- `video_points` 和证据索引可来自 `analysis.json` 与 `evidence_index.json`
- `artifacts.report_html_url` 对应最终 HTML 报告

也就是说：

- 内部继续保留当前产物组织方式
- 对外新增一层稳定的业务语义封装

## 13. 最终建议

当前版本先定这 4 个主接口即可：

1. 上传视频
2. 创建分析任务
3. 查询任务状态
4. 查询分析结果

这样可以最快落地，并且与现有异步 job 架构保持一致。

如果后续要增强，再补：

- webhook 回调
- SSE 事件流
- 任务取消
- 量规上传与管理
- 报告导出
