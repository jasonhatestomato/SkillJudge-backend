# Task 分析报告异步生成与消息队列草案

## 文档目的

本草案用于明确 `task` 级分析报告的异步生成方式，重点回答以下问题：

1. 为什么分析报告需要改成异步生成
2. 数据库、API、消息队列、worker 分别承担什么职责
3. 前端如何通过轮询拿到“正在生成 / 已完成 / 失败”的状态
4. 如果采用消息队列，消息内容和消费流程应如何设计

本文档只讨论架构和数据流，不展开具体代码实现。

## 目标

- 分析报告以 `task` 为边界
- 分析报告属于最终交付产物
- 仅当 `task` 下全部视频评测完成后，才允许触发生成
- `POST` 接口不阻塞等待 PDF 生成完成
- 前端在点击“生成分析报告”后，可以立即收到“已受理，正在生成”的信号
- 前端通过轮询查询接口获取最终状态

## 当前问题

如果 `POST /api/v1/tasks/:taskId/analysis-report` 采用同步执行，后端会在一个请求内完成：

- 聚合分析数据
- 组装 HTML
- 调用 Chrome/Chromium 转 PDF
- 上传 OSS
- 更新数据库

这种方式的问题是：

- 请求耗时长，前端体验差
- 浏览器渲染和上传是慢操作，不适合阻塞业务请求
- 请求超时和网络抖动会直接影响用户体验
- 后续如果任务量增加，同步请求的稳定性会变差

因此，分析报告应改成：

- `POST` 只负责受理
- 由后台异步 worker 真正生成报告
- 前端通过 `GET` 轮询状态

## 总体架构

推荐的职责划分如下：

- PostgreSQL
  - 作为报告状态和结果的唯一真相源
  - 存储 `task_analysis_reports`

- API 服务
  - 受理生成请求
  - 创建报告记录
  - 投递消息到队列
  - 查询报告状态

- 消息队列
  - 负责通知“有新的分析报告需要处理”
  - 不存最终状态，不作为真相源

- Worker
  - 消费消息
  - 拉取数据库状态
  - 执行 HTML 渲染、浏览器转 PDF、OSS 上传
  - 更新报告状态

- 前端
  - 调用 `POST` 触发生成
  - 调用 `GET` 轮询状态
  - 根据状态展示“生成中 / 可查看 / 失败”

## 核心原则

### 1. 数据库是状态真相源

消息队列只负责“触发处理”，不负责存最终状态。

也就是说：

- 报告是否正在生成
- 是否生成成功
- 文件地址是什么
- 失败原因是什么

都必须落在 `task_analysis_reports` 表里。

### 2. 消息只传最小标识

不要把完整分析结果、HTML、PDF 地址放进消息体。

消息只需要传：

- `report_id`
- `task_id`
- `requested_by`
- `type`

worker 收到消息后，再回数据库查询完整上下文。

### 3. 消费必须幂等

即使队列重复投递、worker 重试、多实例并发消费，也不能生成多份冲突结果。

因此必须有：

- 状态检查
- 条件更新
- 明确的最新记录选择规则

## 数据库状态设计

如果采用异步队列，建议把 `task_analysis_reports.status` 扩展为：

- `queued`
- `processing`
- `ready`
- `failed`

状态语义：

- `queued`
  - 报告生成请求已被受理
  - 报告记录已创建
  - 等待 worker 处理

- `processing`
  - worker 已抢占该任务
  - 正在执行分析结果组装、HTML 渲染、PDF 转换和上传

- `ready`
  - 报告已生成成功
  - 文件已上传 OSS
  - 前端可查看/下载

- `failed`
  - 报告生成失败
  - 可用于前端展示失败提示和后端排查

## 对现有表设计草案的调整

相对于现有 `task_analysis_reports` 草案，需要有一个明确变更：

- `status` 枚举从
  - `processing | ready | failed`
- 调整为
  - `queued | processing | ready | failed`

其余字段可以维持现有草案不变。

## API 设计

### 1. 触发生成

#### `POST /api/v1/tasks/:taskId/analysis-report`

语义：

- 受理某个 `task` 的分析报告生成请求
- 不同步等待 PDF 完成
- 立即返回当前报告状态

处理规则：

1. 校验 `task` 是否存在
2. 校验调用者是否有权限
3. 校验 `allVideosCompleted = true`
4. 查当前 `task` 最新一条报告记录
5. 若最新记录状态为：
   - `queued`
     - 直接返回当前记录
   - `processing`
     - 直接返回当前记录
   - `ready`
     - 当前阶段直接返回当前记录，不重复生成
   - `failed`
     - 创建一条新的 `queued` 记录，并投递消息
6. 若不存在记录：
   - 创建一条新的 `queued` 记录，并投递消息
7. 返回 `202 Accepted`

推荐响应体：

```json
{
  "code": 202,
  "message": "accepted",
  "data": {
    "taskId": "task_uuid",
    "reportId": "report_uuid",
    "reportStatus": "queued"
  },
  "timestamp": "2026-04-17T16:00:00Z"
}
```

### 2. 查询状态

#### `GET /api/v1/tasks/:taskId/analysis-report`

语义：

- 查询当前 `task` 最新一份分析报告的状态和访问地址

推荐返回：

#### 生成中

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "taskId": "task_uuid",
    "reportId": "report_uuid",
    "reportStatus": "processing",
    "generatedAt": null,
    "url": null,
    "errorMessage": null
  },
  "timestamp": "2026-04-17T16:00:10Z"
}
```

#### 已完成

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "taskId": "task_uuid",
    "reportId": "report_uuid",
    "reportStatus": "ready",
    "generatedAt": "2026-04-17T16:03:00Z",
    "url": "https://oss.example.com/reports/task-analysis.pdf",
    "errorMessage": null
  },
  "timestamp": "2026-04-17T16:03:05Z"
}
```

#### 失败

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "taskId": "task_uuid",
    "reportId": "report_uuid",
    "reportStatus": "failed",
    "generatedAt": null,
    "url": null,
    "errorMessage": "chrome render timeout"
  },
  "timestamp": "2026-04-17T16:03:05Z"
}
```

## 前端交互流程

前端交互建议固定为：

1. 用户点击“生成分析报告”
2. 前端调用 `POST /api/v1/tasks/:taskId/analysis-report`
3. 若返回：
   - `queued`
   - `processing`
   则前端立刻提示“正在生成分析报告”
4. 前端每隔一段时间轮询 `GET /api/v1/tasks/:taskId/analysis-report`
5. 根据状态显示：
   - `queued`
     - 排队中
   - `processing`
     - 正在生成
   - `ready`
     - 可查看 / 可下载
   - `failed`
     - 生成失败

推荐前端轮询间隔：

- 2 到 5 秒

## 消息队列设计

### 消息体建议

消息中只保留最小标识：

```json
{
  "type": "task_analysis_report_generate",
  "reportId": "report_uuid",
  "taskId": "task_uuid",
  "requestedBy": "user_uuid",
  "createdAt": "2026-04-17T16:00:00Z"
}
```

字段语义：

- `type`
  - 消息类型，便于队列中后续扩展其他异步任务

- `reportId`
  - 分析报告记录主键
  - worker 收到后应先按此字段查数据库

- `taskId`
  - 任务 ID
  - 便于日志和排查

- `requestedBy`
  - 触发人
  - 便于审计和日志记录

- `createdAt`
  - 消息创建时间

### 为什么消息里不放完整分析数据

不建议直接把分析结果放进消息里，原因如下：

- 消息体会变大
- 数据结构更容易演进失败
- 重试和排查更复杂
- 真实数据仍然应该以数据库为准

因此，worker 收到消息后应该：

- 查 `task_analysis_reports`
- 查 `tasks`
- 查分析数据
- 再生成报告

## Worker 消费流程

worker 收到消息后的推荐流程如下：

1. 根据 `reportId` 查 `task_analysis_reports`
2. 若记录不存在，直接丢弃消息
3. 若记录状态不是 `queued`，直接忽略
4. 尝试将状态从 `queued` 更新为 `processing`
5. 只有更新成功的 worker 才继续执行
6. 重新查询 `task`、分析数据和模板数据
7. 渲染 HTML
8. 调用 Chrome/Chromium 生成 PDF
9. 上传 OSS
10. 更新记录为：
    - `status = ready`
    - `storage_path`
    - `public_url`
    - `generated_at`
11. 若过程中失败：
    - `status = failed`
    - `error_message = ...`

## 幂等与并发控制

这部分是消息队列方案里最重要的环节。

### API 侧幂等

`POST /analysis-report` 时：

- 如果已有最新记录处于 `queued`
  - 不再重复创建新记录
- 如果已有最新记录处于 `processing`
  - 不再重复创建新记录
- 如果已有最新记录处于 `ready`
  - 当前阶段直接返回

### Worker 侧幂等

即使同一消息被重复消费，也必须确保只有一个 worker 真正生成。

建议方式：

- 使用条件更新：

```sql
update task_analysis_reports
set status = 'processing', updated_at = now()
where id = :report_id and status = 'queued';
```

如果影响行数为 `0`，说明：

- 已经被其他 worker 抢占
- 或状态已变化

这时当前 worker 直接退出。

## 队列选型建议

### 方案 A：数据库轮询

不引入外部 MQ，由 worker 定时扫描 `status = queued` 的记录。

优点：

- 实现最简单
- 不新增基础设施
- 状态与任务天然一致

缺点：

- 严格说不是真正的消息队列
- 吞吐和时效一般

适用场景：

- 当前阶段先快速落地

### 方案 B：Redis Streams

API 在创建 `queued` 记录后，向 Redis Stream 推送一条消息。

优点：

- 接入轻量
- 有消费组和 ack 机制
- 比简单轮询更像正式异步任务系统

缺点：

- 仍需自行处理幂等、失败重试、死信

适用场景：

- 已有 Redis
- 想尽快形成 MQ 化架构

### 方案 C：RabbitMQ

适合后续异步任务明显增多时统一收口。

优点：

- 语义成熟
- 死信、重试、消费控制更完整

缺点：

- 引入成本和运维成本更高

适用场景：

- 未来除了分析报告，还会有大量导出、学生报告、通知类异步任务

## 当前阶段建议

如果只针对任务分析报告这一项能力，建议采用分阶段路线：

### 第一阶段

- 保留 `task_analysis_reports` 作为状态表
- 将 `POST` 改成异步受理
- worker 后台扫描或消费 `queued` 任务
- 前端通过 `GET` 轮询状态

### 第二阶段

- 若后续异步任务增多
- 再将“扫描 `queued` 记录”的方式升级为正式 MQ
- 业务接口与数据库表结构不需要大改

## 结论

推荐的最终口径如下：

- 报告状态以 `task_analysis_reports` 为准
- 采用异步生成
- `POST` 只负责受理，不负责同步生成 PDF
- `GET` 用于前端轮询状态
- 状态枚举建议为：
  - `queued`
  - `processing`
  - `ready`
  - `failed`
- 若采用消息队列，消息只传最小标识，不传大数据
- worker 消费必须通过数据库条件更新实现幂等与并发保护

当前最稳的落地路线是：

1. 先把接口改成异步
2. 以 `task_analysis_reports` 作为状态表
3. 先实现数据库轮询版或 Redis Streams 版 worker
4. 后续再根据异步任务规模决定是否引入更完整的 MQ
