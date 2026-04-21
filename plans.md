# SkillJudge 当前计划

## 1. 当前阶段

当前项目已完成 `Phase 2` 的大部分主链，现阶段后端重心不再是继续补 MVP，而是围绕教师交付、统计口径、报告能力以及 `task` 级评分员邀请与绑定能力做收口。

主参考文档：

- [`designingDocs/phase.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/phase.md)
- [`designingDocs/api.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/api.md)
- [`designingDocs/platform-schema-reference.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/platform-schema-reference.md)
- [`designingDocs/auth.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/auth.md)

当前数据库主链：

- 业务主链：`schools -> projects -> tasks -> videos -> ai_evaluations / manual_evaluations`
- 权限主链：`users -> user_roles -> roles -> role_permissions -> permissions`

## 2. 当前基线

当前代码仍是模块化单体，但主业务链路已经成型。

已完成能力：

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
  - 批量创建有限并发
  - 轮询有限并发

当前已完成的重要结构调整：

- RBAC 已切到正式权限链：`user_roles -> role_permissions -> permissions`
- `task` 已成为正式业务中间层
- `project` 已从 `rubric` 绑定职责中收缩
- `video` 已迁到任务维度
- 视频对象 key 已切到：
  - `projects/{projectId}/tasks/{taskId}/videos/{videoId}/{filename}`
- 评分当前按单评模式实现
- AI 评估当前按“单视频单任务”实现
- AI 对接协议已统一为：
  - 创建任务
  - 轮询状态
  - 拉取结果

## 3. 当前已落地主链

人工评分主链：

1. 登录
2. 创建项目
3. 创建批次
4. 创建或导入评分细则
5. 在批次下上传视频
6. 分配评分员
7. 评分员查看自己的任务
8. 查看评分详情
9. 提交人工评分

AI 评估主链：

1. 教师或管理员触发 AI 评分
2. 后端按 `video` 创建 `ai_evaluations`
3. 后端向外部 AI 创建分析任务并保存 `job_id`
4. 后端后台轮询 AI 状态与结果
5. 后端回填 `ai_evaluations`、`videos.ai_status`、`videos.ai_score`
6. 前端通过视频详情、评分详情或 AI 评估接口读取结果

学生查分主链：

1. 上传视频时写入或从文件名兜底解析 `student_id`
2. 学生登录后查询自己的视频列表
3. 学生进入视频详情查看 AI / 人工评分结果
4. 学生详情页消费 AI 返回的阶段点和关键时间点

## 4. 当前主要缺口

当前不是缺“有没有接口”，而是缺“状态口径是否统一”。最大问题集中在 `videos` 表三类状态的职责边界：

- `manual_status`
  - 人工评分子流程状态
- `ai_status`
  - AI 评估子流程状态
- `evaluation_status`
  - 视频整体任务完成状态

旧实现中，`evaluation_status` 被部分当成“人工是否完成”，部分当成“整体是否完成”，导致：

- 评分员视角和教师视角的完成态混用
- `completed_at` 语义不稳定
- 任务/项目聚合统计依赖的完成口径不稳定
- 后续 `scoreboard`、报告、导出接口没有稳固的状态基础

## 5. 视频状态最小改造方案

本轮优先落地最小改造，不新增表，不改现有主接口路径，只统一状态语义和聚合规则。

### 5.1 字段职责

- `manual_status`
  - 只表示人工评分进度
  - `pending / in_progress / submitted`
- `ai_status`
  - 只表示 AI 评估进度
  - `pending / processing / completed / failed`
- `evaluation_status`
  - 只表示视频整体流程状态
  - `pending / in_progress / completed / failed`
- `videos.completed_at`
  - 只表示整条视频流程最终完成时间
  - 即整体 `evaluation_status = completed` 的时间

### 5.2 聚合规则

整体视频完成规则：

- `manual_status = submitted`
- 且 `ai_status = completed`
- 才能得到 `evaluation_status = completed`

评分员自己的任务完成规则：

- 只看 `manual_status`
- `manual_status = submitted` 时，评分员任务视角应显示为 `completed`

建议的组合规则：

| `manual_status` | `ai_status` | `evaluation_status` | 评分员视角 |
| --- | --- | --- | --- |
| `pending` | `pending` | `pending` | `pending` |
| `pending` | `processing` | `in_progress` | `pending` |
| `pending` | `completed` | `in_progress` | `pending` |
| `pending` | `failed` | `failed` | `pending` |
| `in_progress` | `pending` | `in_progress` | `in_progress` |
| `in_progress` | `processing` | `in_progress` | `in_progress` |
| `in_progress` | `completed` | `in_progress` | `in_progress` |
| `in_progress` | `failed` | `failed` | `in_progress` |
| `submitted` | `pending` | `in_progress` | `completed` |
| `submitted` | `processing` | `in_progress` | `completed` |
| `submitted` | `completed` | `completed` | `completed` |
| `submitted` | `failed` | `failed` | `completed` |

### 5.3 最小改造范围

1. 新增统一状态解析逻辑
- 提供共享 helper
- 统一计算：
  - `evaluation_status`
  - scorer task 视角的 `status`
  - `completed_at`

2. 修正人工评分提交链路
- 提交人工评分后：
  - 必须更新 `manual_status`
  - 必须根据当前 `ai_status` 统一重算 `evaluation_status`
  - 只有整体完成时才写 `videos.completed_at`

3. 修正 AI 创建 / 轮询 / 完成 / 失败链路
- AI 状态进入 `processing / completed / failed` 时：
  - 必须根据当前 `manual_status` 重算 `evaluation_status`
  - 只有整体完成时才写 `videos.completed_at`
- AI 重跑进入 `processing` 时：
  - 若视频原本整体完成，应回退为 `in_progress`
  - `videos.completed_at` 需要清空

4. 修正评分员接口的状态语义
- `GET /api/v1/tasks/my`
- scorer 视角的 `GET /api/v1/tasks/:id`
- `POST /api/v1/tasks/:id/submit`
- 上述接口中的 `status` 字段应改为 scorer task 视角状态
- 同时补充：
  - `manualStatus`
  - `aiStatus`
  - `evaluationStatus`

5. 修正任务 / 项目聚合统计
- 任务、项目的 `completedVideos` 继续看 `evaluation_status = completed`
- 但前提是 `evaluation_status` 必须由统一规则维护
- DTO 补充：
  - `allVideosCompleted`
  - `completionRate`

### 5.4 不在本轮范围

- 不新增独立 scoring_task 主表
- 不引入双评、多评、复评、仲裁
- 不改 AI 外部协议
- 不新增批量导出异步任务表
- 不在本轮实现学生报告 / 通用消息通知

### 5.5 实施顺序

1. 整理 `plans.md`，确认统一口径
2. 新增共享状态聚合 helper
3. 改人工评分提交链路
4. 改 AI 创建、轮询、完成、失败链路
5. 改评分员列表 / 详情 / 提交返回
6. 改任务 / 项目 DTO 统计字段
7. 运行 `go test ./...` 回归验证

### 5.6 验收标准

- 评分员提交后，评分员视角任务立即为 `completed`
- AI 未完成时，视频整体 `evaluation_status` 仍为 `in_progress`
- AI 和人工都完成后，视频整体 `evaluation_status = completed`
- AI 重跑时，整体状态可从 `completed` 回退到 `in_progress`
- `videos.completed_at` 只在整体完成时写入
- 任务 / 项目 `completedVideos` 与整体完成态保持一致
- 项目 / 任务返回 `allVideosCompleted`、`completionRate`

## 6. 本轮新增实施范围

在完成状态口径统一后，本轮继续落地教师端最直接依赖的两块能力：

1. `Task` 级完成状态提示
2. 任务成绩汇总页与前端 Excel 导出

### 6.1 Task 级完成状态提示

目标：

- 只做 `task` 级完成统计，不做 `project` 级完成提示聚合。
- 教师端任务列表每一行前增加状态按钮或勾选图标。
- 若任务下全部视频都完成评测，则高亮；否则灰色。

单视频完成口径：

- `manual_status = submitted`
- 且 `ai_status = completed`

任务聚合字段：

- `totalVideos`
  - 当前任务下的视频总数。
- `completedVideos`
  - 当前任务下已经完成完整评测流程的视频数量。
- `allVideosCompleted`
  - 当前任务下是否全部视频都已完成评测。
- `completionRate`
  - 当前任务完成率，计算口径为 `completedVideos / totalVideos * 100`。

接口策略：

- 不新增接口。
- 直接复用：
  - `GET /api/v1/projects/:projectId/tasks`
  - `GET /api/v1/tasks/:taskId`
- 上述字段由后端聚合后稳定返回给前端，不由前端自行遍历视频计算。

前端实施点：

- 教师端任务列表消费任务聚合字段。
- 使用 `allVideosCompleted` 控制图标高亮状态。
- 使用 `completedVideos / totalVideos` 与 `completionRate` 展示进度。

### 6.2 任务成绩汇总页与前端 Excel 导出

目标：

- 教师在任务列表行末点击“查看成绩”后，进入任务成绩页或弹窗。
- 页面展示该任务下全部学生的最新成绩与完成状态。
- 支持“已完成显示成绩，未完成显示未完成”。
- 导出由前端基于当前查询结果直接合成 Excel，不新增后端异步导出任务。

接口设计：

- 新增 `GET /api/v1/tasks/:taskId/scoreboard`

语义：

- 返回某个任务下全部学生的视频评测成绩汇总。
- 用于成绩页展示。
- 也作为前端导出 Excel 的数据源。
- 必须支持部分学生已完成、部分学生未完成的混合结果。

query 参数：

- `page`
- `pageSize`
- `keyword`
  - 按学生姓名、学号模糊搜索
- `scope`
  - `page` / `all`
  - 页面展示默认使用 `page`
  - 前端导出时使用 `all` 拉取当前筛选条件下的全量结果
- `evaluationStatus`
  - `completed` / `in_progress` / `failed`
- `sortBy`
  - `studentNumber` / `studentName` / `aiScore` / `manualScore` / `completedAt`
- `sortOrder`
  - `asc` / `desc`

返回字段基线：

- `task`
  - `id`
  - `name`
  - `status`
  - `totalVideos`
  - `completedVideos`
  - `allVideosCompleted`
  - `completionRate`
- `summary`
  - `totalStudents`
  - `completedStudents`
  - `averageAIScore`
  - `averageManualScore`
- `items[]`
  - `videoId`
  - `studentName`
  - `studentNumber`
  - `aiScore`
  - `manualScore`
  - `aiStatus`
  - `manualStatus`
  - `evaluationStatus`
  - `completedAt`
  - `displayStatus`
- `pagination`

展示规则：

- `evaluationStatus = completed`
  - 前端显示成绩。
- `evaluationStatus != completed`
  - 前端主文案先统一显示“未完成”。
- 后端仍返回 `aiStatus` / `manualStatus`，为后续更细粒度提示预留。

职责划分：

- 后端负责返回实时成绩与状态。
- 前端负责基于当前查询结果直接导出 Excel。
- 若后续单个任务学生规模显著增大，再补后端异步导出接口。

### 6.3 任务成绩分析页与任务分析报告

目标：

- 分析页面和分析报告都属于 `task` 级最终交付能力。
- 两者都只在任务全部完成后开放，不参与过程追踪。
- 当前阶段暂不纳入批量学生报告。

开放条件：

- `allVideosCompleted = true`

接口设计：

- `GET /api/v1/tasks/:taskId/analysis`
  - 返回任务最终分析页面所需数据
  - 仅当任务全部完成后允许访问
- `POST /api/v1/tasks/:taskId/analysis-report`
  - 异步受理当前任务最终分析报告生成请求
  - 不同步等待 PDF 完成
  - 当前阶段不支持显式重生成
- `GET /api/v1/tasks/:taskId/analysis-report`
  - 查询当前任务当前有效分析报告状态与访问地址

分析页面指标基线：

- `task`
  - `id`
  - `name`
  - `rubricTotalScore`
  - `totalVideos`
  - `completedVideos`
  - `allVideosCompleted`
- `scoreSummary`
  - `averageAIScore`
  - `averageManualScore`
  - `highestAIScore`
  - `lowestAIScore`
  - `highestManualScore`
  - `lowestManualScore`
- `manualScoreDistribution`
- `aiScoreDistribution`
- `scoreGapDistribution`

图表口径：

- 分布图按任务满分归一化后分桶，避免不同任务因满分不同导致图表不可比较
- 页面主分数仍保留原始分值展示，不默认转换为百分制

任务分析报告数据落地：

- 使用独立表 `task_analysis_reports`
- 当前数据库层面不对 `task_id` 做唯一约束
- 状态枚举收敛为：
  - `queued`
  - `processing`
  - `ready`
  - `failed`
- 业务层承担：
  - 幂等控制
  - 并发控制
  - 当前有效报告选择规则

异步生成策略：

- `POST /analysis-report` 只负责：
  - 校验任务是否已全部完成
  - 创建或复用 `queued` / `processing` / `ready` 记录
  - 立即返回受理结果
- 后端后台 worker 负责：
  - 扫描或消费 `queued` 报告
  - 抢占后更新为 `processing`
  - 聚合分析数据
  - 渲染 HTML
  - 调用 Chrome/Chromium 转 PDF
  - 上传 OSS
  - 更新为 `ready` 或 `failed`
- 前端负责：
  - 触发 `POST`
  - 轮询 `GET /analysis-report`
  - 根据 `queued / processing / ready / failed` 展示提示

### 6.4 本轮实施顺序

1. 收敛 `plans.md` 与接口草案口径
2. 实现 `GET /api/v1/tasks/:taskId/scoreboard`
3. 校验任务聚合字段在教师端任务接口中稳定返回
4. 教师端任务列表增加完成状态按钮和“查看成绩”入口
5. 教师端成绩页/弹窗接入 `scoreboard`
6. 前端基于 `scoreboard?scope=all` 结果实现 Excel 导出
7. 实现 `GET /api/v1/tasks/:taskId/analysis`
8. 将 `POST /api/v1/tasks/:taskId/analysis-report` 改为异步受理
9. 增加任务分析报告后台 worker
10. 实现 `GET /api/v1/tasks/:taskId/analysis-report`
11. 运行后端测试与前端最小联调验证

## 7. 当前结论

当前项目可以从“统一完成态”继续推进到“教师端交付最小闭环”。本轮优先落地：

- `task` 级完成状态提示
- 任务成绩汇总查询
- 前端实时导出成绩单
- 任务最终分析页面
- 任务分析报告后端生成与存储

学生报告与批量学生报告继续放在下一轮。

## 8. 并行专项：Task 评分员邀请与绑定

### 8.1 背景

教师端新增需求要求：

- 在创建 `task` 的最后一步邀请评分员
- 输入姓名、邮箱后自动复用或创建评分员账号
- 为当前 `task` 创建 invitation / 站内确认通知
- `task` 创建后仍可继续添加、补发邀请、移除评分员关系
- 已存在评分员不再发送新的外链邀请邮件，而是通过系统内通知确认

该能力与当前已存在的“视频分配给评分员”不是同一件事，必须补独立的 `task -> scorer` 关系层。

主参考文档：

- [`designingDocs/task_scorer_invitation_design.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/task_scorer_invitation_design.md)

### 8.2 当前状态

数据库侧已完成：

- `users.email` 规范化唯一约束
- `task_scorers`
- `task_scorer_invitations`

当前尚未完成：

- 新建 task 第 4 步“邀请新评分员”弹窗联调收尾
- 创建任务后按预分配评分员批量创建 task 级待确认通知
- 已完成视频的二次调整边界校验
- 真实环境邮件与本地联调验证

当前已完成：

- Go `model`
- `FindByEmail`
- `taskscorer` 模块
- 真实路由与 handler
- invitation 生命周期接口
- `internal/platform/mail/`
- SMTP 发信接入
- 邮件确认页
- 学校级 `scorer provision` 接口
- task 级 `notify scorer` 接口
- 新建 task 第 4 步弹窗式邀请入口
- task 列表页“评分员分配详情”入口
- task 列表页评分员分配详情弹窗
- 评分员登录后待确认弹窗 / 消息入口
- 评分员任务列表仅暴露 `accepted` 的 task
- 仅对 `manual_status = pending` 视频开放再分配
- 再分配支持：
  - 多评分员平均分配
  - 指定数量后随机分配
- 再分配后自动补发站内确认通知
- 原待确认评分员在已无分配视频时自动作废原通知并停用关系

### 8.3 当前收口策略

当前实现按“账号开通”和“任务确认”两条链分开：

- 新账号：学校级预创建时发送开通邮件
- 已有账号：学校级预创建仅复用账号，不发邮件
- task 创建成功后：仅对本 task 实际预分配到的评分员写站内待确认通知
- 评分员端：只有确认后，任务才进入待评分列表

入口位置固定为：

- 教师端 `task` 列表页操作区
- 顺序固定为：
  1. `查看评分详情`
  2. `查看成绩汇总`
  3. `评分员分配详情`
- `评分员分配详情` 打开大弹窗，不再以详情页内联面板为主入口

### 8.4 当前再分配策略

当前实现只做保守再分配：

- 仅允许 `manual_status = pending` 的视频进入“可重新分配”列表
- `in_progress` / `submitted` / `evaluation_status = completed` 一律不允许调整
- 若原评分员仍为 `task_scorers.status = pending`，且其名下视频被全部移走：
  - 原 `task_scorer_invitations` 中仍为 `sent` 的记录改为 `cancelled`
  - 原 `task_scorers` 关系改为 `inactive`

当前支持两种再分配方式：

1. `平均分配`
- 选择多个目标评分员
- 系统随机打散已选视频，再按人数平均切分
- 若除不尽，前 `remainder` 个评分员多拿 1 条
- 保证不重不漏

2. `指定数量随机分配`
- 教师只填写“每位评分员接收多少条”
- 后端校验数量之和必须等于已选视频总数
- 系统随机打散视频后按数量切片分配
- 不支持手工逐条拖拽指定

再分配后的通知规则：

- 目标评分员若已是当前 task 的 `accepted` 评分员：
  - 不重复确认
  - 直接接收新的分配结果
- 目标评分员若尚未加入 task 或仍为 `pending`：
  - 自动创建或刷新 `task_scorers`
  - 自动生成新的站内待确认通知

### 8.5 当前已具备能力

当前后端和前端已经具备：

1. 按邮箱查评分员账号
2. 学校级预创建 `scorer`
3. 若新建账号则发送开通邮件
4. `task` 级 upsert `task_scorers`
5. 写入 `task_scorer_invitations`
6. 站内通知已读 / 系统内确认
7. 评分员通过邮件链接确认参与
8. 评分员任务查询仅返回 `accepted` 的 task
9. 教师端 `task` 列表页操作入口和分配详情弹窗
10. `pending` 视频再分配与补通知

### 8.6 后续建议

1. 做真实 SMTP / 邮件模板联调和回归
2. 若后续要放开更激进的改派，再单独设计 `in_progress` 视频回收规则
3. 若需要更强的教师运营视图，再补“按评分员聚合的任务负载概览”
