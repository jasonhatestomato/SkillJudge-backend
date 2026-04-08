# SkillJudge Phase 2 计划

## 1. 当前目标

当前阶段已进入 `Phase 2`。现阶段目标不是继续补齐 MVP 主链，而是在现有基础上完成核心功能收口，重点推进 AI 视频分析接入和人机评分对比能力。

当前主参考文档：

- [`designingDocs/phase.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/phase.md)
- [`designingDocs/api.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/api.md)
- [`designingDocs/platform-schema-reference.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/platform-schema-reference.md)
- [`designingDocs/auth.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/auth.md)

当前数据库主线：

- 业务主线：`schools -> projects -> tasks -> videos -> ai_evaluations / manual_evaluations`
- 权限主线：`users -> user_roles -> roles -> role_permissions -> permissions`

## 2. 当前基线

当前代码仍是模块化单体，但业务主链已经基本成型。

已完成模块：

- `auth / user`
  - 登录、刷新、登出
  - `/users/me`
  - 管理员用户 CRUD
  - 正式 RBAC 查询链
  - `/health`、`/ready`
- `project`
  - 项目 CRUD
- `rubric`
  - 评分细则 CRUD
  - Excel 模板上传解析
- `task`
  - 创建批次
  - 项目下批次列表
  - 批次详情
- `video`
  - 上传凭证
  - 上传确认
  - 视频列表
  - 视频详情
  - 视频删除
  - 华为云 OBS + STS
- `scoring`
  - 评分员分配
  - 我的评分列表
  - 评分详情
  - 提交人工评分
- `ai`
  - 单视频触发 AI 评估
  - 任务维度批量触发 AI 评估
  - AI 评估状态查询
  - AI 评估结果查询
  - 外部 AI 任务创建
  - 外部 AI 任务轮询
  - AI 结果落库与视频摘要回填

当前阶段判断：

- `任务分配` 后端能力已基本完成，当前主要是联调和体验收口
- `学生查分` 后端主链已经落地，当前主要是前后端展示细节收口
- `视频播放优化` 主要由前端承接，后端只需稳定提供 AI 返回的阶段点、关键时间点和错误点数据
- 当前后端主开发重点应转为：
  - AI 视频分析正式接入
  - 人机评分对比结果收口
  - AI 结果结构与前端展示对齐

## 3. 已完成主链

当前已完成的主链是：

1. 登录
2. 创建项目
3. 创建批次
4. 创建或导入评分细则
5. 在批次下上传视频
6. 分配评分员
7. 评分员查看自己的任务
8. 查看评分详情
9. 提交人工评分

对应主链：

`project -> task -> video -> assign scorer -> my tasks -> detail -> submit`

当前已补充的 AI 评分链路：

1. 教师或管理员触发 AI 评分
2. 后端按 `video` 创建 `ai_evaluations`
3. 后端向外部 AI 创建分析任务并保存 `job_id`
4. 后端后台轮询 AI 状态与结果
5. 后端回填 `ai_evaluations`、`videos.ai_status`、`videos.ai_score`
6. 前端通过视频详情、评分详情或 AI 评估接口读取结果

当前已补充的学生查分链路：

1. 上传视频时写入或从文件名兜底解析 `student_id`
2. 学生登录后查询自己的视频列表
3. 学生进入视频详情查看 AI / 人工评分结果
4. 学生详情页消费 AI 返回的阶段点和关键时间点

## 4. 当前已实现接口

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/users/me`
- `PATCH /api/v1/users/me`
- `POST /api/v1/users`
- `GET /api/v1/users`
- `PATCH /api/v1/users/:id`
- `DELETE /api/v1/users/:id`
- `POST /api/v1/projects`
- `GET /api/v1/projects`
- `GET /api/v1/projects/:id`
- `PATCH /api/v1/projects/:id`
- `DELETE /api/v1/projects/:id`
- `POST /api/v1/rubrics`
- `GET /api/v1/rubrics`
- `GET /api/v1/rubrics/:id`
- `PATCH /api/v1/rubrics/:id`
- `DELETE /api/v1/rubrics/:id`
- `POST /api/v1/rubrics/upload-template`
- `POST /api/v1/projects/:projectId/tasks`
- `GET /api/v1/projects/:projectId/tasks`
- `GET /api/v1/tasks/:id`
- `GET /api/v1/tasks/my`
- `POST /api/v1/tasks/:id/assignments`
- `POST /api/v1/tasks/:id/submit`
- `POST /api/v1/videos/upload-credential`
- `POST /api/v1/videos/:id/confirm-upload`
- `GET /api/v1/videos`
- `GET /api/v1/videos/:id`
- `DELETE /api/v1/videos/:id`
- `GET /api/v1/students/me/videos`
- `GET /api/v1/students/me/videos/:id`
- `POST /api/v1/videos/:id/ai-evaluations`
- `POST /api/v1/tasks/:id/ai-evaluations`
- `GET /api/v1/ai-evaluations/:id`
- `GET /api/v1/ai-evaluations/:id/result`
- `GET /health`
- `GET /ready`

## 5. 已完成的重要调整

- RBAC 已切到正式权限链：
  - `user_roles -> role_permissions -> permissions`
- `task` 已成为正式业务中间层
- `project` 已从 `rubric` 绑定职责中收缩
- `video` 已从项目维度迁到任务维度
- `video` 相关接口已改为使用 `taskId`
- 视频对象 key 已切到：
  - `projects/{projectId}/tasks/{taskId}/videos/{videoId}/{filename}`
- `scoring` 当前按单评模式实现
- 评分任务视图当前落在 `video` 上，而不是单独新增复杂任务主表
- AI 评估当前按“单视频单任务”实现：
  - 一条 `ai_evaluations`
  - 对应一个外部 `job_id`
  - 批量触发仅作为业务层批处理入口，底层仍逐视频创建
- AI 对接协议已统一为：
  - 创建任务
  - 轮询状态
  - 拉取结果

## 6. 当前 Phase 2 范围

当前明确纳入 `Phase 2`：

- AI 视频分析集成
  - 外部 AI 创建任务
  - 外部 AI 状态轮询
  - 外部 AI 结果拉取
  - AI 结果落库与摘要回填
- 人机评分对比
  - 视频详情中的 AI / manual 对比
  - 评分详情中的 AI / manual 对比
  - 前端展示字段对齐
- 任务分配收口
  - 当前后端能力已基本完成
  - 剩余工作以联调和边界修正为主
- 视频播放优化的后端配合
  - 稳定返回 `videoStages`
  - 稳定返回 `videoPoints`
  - 稳定返回时间戳相关字段
  - 不承担播放器交互本身实现

当前明确不纳入当前后端主开发范围：

- `task` 更新/删除
- 视频批量上传
- 视频转码
- 双评、多评、复评、仲裁
- 通知与统计深化
- 视频播放器 UI / 时间轴交互实现
- AI 评分的深度工程化增强：
  - 阶段性交付消费
  - 更细粒度进度展示
  - 完整调度异步化
  - 多实例任务认领
  - 更复杂的失败重试与补偿
- 数据统计基础功能

## 7. 当前遗留事项

当前最主要的遗留不是主链缺失，而是 `Phase 2` 核心能力的联调、收口和有限优化：

1. AI 视频分析联调
- 登录
- 创建项目
- 创建批次
- 创建/上传评分细则
- 上传视频
- 分配评分员
- 评分员查看任务
- 查看评分详情
- 提交评分
- 单视频触发 AI 评分
- 任务批量触发 AI 评分
- AI 状态查询
- AI 结果查询

2. 人机评分对比收口
- 对齐视频详情中的 `aiEvaluation`
- 对齐评分详情中的 `aiEvaluation`
- 对齐 AI 总分、分项明细、错误点、阶段点
- 对齐 AI 失败态、空结果、重跑后的最新结果展示

3. 视频播放优化的后端配合
- 确保 AI 返回的 `videoStages` 字段稳定
- 确保 AI 返回的 `videoPoints` 字段稳定
- 确保时间字段和秒级字段可直接供前端做时间轴标记和跳转
- 当前阶段不扩展播放器专用后端接口

4. 测试材料补齐
- `curl` / Postman 风格联调文档
- 关键失败场景说明
- AI 轮询链路联调样例
- AI 状态枚举和错误码对齐

5. AI 评分当前实施计划
- 第 1 步：外部 AI 联调收口
  - 按 `designingDocs/external_api_current_implementation.md` 对齐创建、状态、结果 3 个接口
  - 确认 `job_id` 已写入数据库并贯通查询链路
- 第 2 步：有限并发优化
  - 将任务批量触发从串行改为有限并发
  - 将后台轮询从串行改为有限并发
  - 控制 AI 侧请求并发上限，避免形成阻塞点
- 第 3 步：结果与展示收口
  - 对齐视频详情、评分详情中的 `aiEvaluation`
  - 对齐 AI 失败态和空结果场景
  - 对齐重跑语义和前端提示
- 第 4 步：后续增强预留
  - 保留 `result_slices` 扩展口，但当前阶段不消费
  - 后续按需要评估完整异步派发、进度字段、多实例认领

6. 联调后问题收口
- 参数对齐问题
- 数据初始化问题
- 权限边界问题
- 上传和评分状态问题
- AI 创建、轮询、结果映射问题

## 8. 有限并发实施方案

本阶段的有限并发优化只解决当前 AI 链路中的两个串行瓶颈，不改数据库结构，不改前端接口契约，不引入完整异步调度重构。

### 8.1 优化目标

- 保持现有 AI 评估业务模型不变：
  - 一个 `video`
  - 对应一条 `ai_evaluations`
  - 对应一个外部 `job_id`
- 保持前端接口不变：
  - `POST /api/v1/videos/:id/ai-evaluations`
  - `POST /api/v1/tasks/:id/ai-evaluations`
  - `GET /api/v1/ai-evaluations/:id`
  - `GET /api/v1/ai-evaluations/:id/result`
- 仅优化后端内部执行方式：
  - 批量创建从串行改为有限并发
  - 后台轮询从串行改为有限并发

### 8.2 当前瓶颈

- `BatchCreateForTask` 当前逐个视频顺序调用 AI 创建逻辑，批量触发时响应时间会线性增长
- `pollOnce` 当前逐条任务顺序轮询 AI 状态和结果，单条慢任务会拖住整轮

### 8.3 具体改造点

1. 批量创建有限并发
- 改造位置：
  - `internal/modules/ai/service.go`
- 改造方法：
  - 保留现有 `BatchCreateForTask` 入口和返回结构
  - 仍然先完成任务权限校验和视频集合查询
  - 将原来的串行 `for` 循环改成有限并发 worker 模式
  - 每个视频继续复用现有 `createForResolvedVideo(...)`
- 并发控制：
  - 使用固定并发上限
  - 使用 `WaitGroup + semaphore + Mutex` 汇总结果
- 返回语义保持不变：
  - `total`
  - `processing`
  - `failed`
  - `errors`

2. 轮询有限并发
- 改造位置：
  - `internal/modules/ai/service.go`
- 改造方法：
  - 保留现有 `RunPoller` 定时轮询结构
  - 保留数据库候选任务查询方式
  - 将 `pollOnce` 中逐条顺序处理改为有限并发处理
  - 每条任务继续复用现有 `pollEvaluation(...)`
- 并发控制：
  - 每轮最多同时处理固定数量的 polling 任务
  - 单条任务失败不影响其他任务继续执行
  - 单条任务增加独立超时控制，避免慢请求长期占用 worker

3. 配置化并发上限
- 改造位置：
  - `internal/config/config.go`
  - `.env.example`
- 新增配置项：
  - `AI_CREATE_CONCURRENCY`
  - `AI_POLL_CONCURRENCY`
- 默认建议值：
  - `AI_CREATE_CONCURRENCY=5`
  - `AI_POLL_CONCURRENCY=5`

### 8.4 不在本次实施范围

- 不修改数据库字段
- 不修改 `ai_evaluations` 状态机
- 不引入新的调度状态，如 `pending`、`dispatching`
- 不将创建任务彻底后台化
- 不实现多实例任务认领
- 不消费 `result_slices`
- 不修改前端接口请求体和响应体

### 8.5 预期代码变更范围

- `backend/internal/modules/ai/service.go`
  - 批量创建并发化
  - 轮询并发化
  - 单条 polling 超时控制
- `backend/internal/config/config.go`
  - 增加 AI 创建并发和轮询并发配置
- `backend/.env.example`
  - 增加并发配置示例

### 8.6 实施顺序

1. 增加并发配置项
2. 改造 `BatchCreateForTask` 为有限并发
3. 改造 `pollOnce` 为有限并发
4. 补充日志和错误收口
5. 运行 `go test ./...` 做回归验证

### 8.7 验收标准

- 批量触发接口仍保持原有返回结构
- 批量触发时多个视频可以并行创建 AI 任务
- 后台轮询时多个 `processing` 任务可以并行轮询
- 单条任务失败不阻塞其他任务
- 现有 AI 查询接口行为不变
- 编译与测试通过

## 9. 当前结论

当前项目已进入 `Phase 2`，`Phase 1` 主链已基本完成。后续工作重心应转为：

1. AI 视频分析正式接入
2. 人机评分对比收口
3. AI 轮询链路收口
4. 有限并发优化
5. 问题收口
