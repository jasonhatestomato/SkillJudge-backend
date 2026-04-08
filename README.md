# SkillJudge Backend

SkillJudge 后端当前处于 `Phase 2`，重点是完成 AI 视频分析接入、人机评分对比和学生查分链路收口。当前代码形态是模块化单体，技术栈为 `Go + Gin + GORM + PostgreSQL + Redis + JWT + OBS(STS)`。

## 当前主链

当前已经完成并可联调的主链：

1. 登录
2. 创建项目
3. 创建批次
4. 创建或导入评分细则
5. 在批次下上传视频
6. 分配评分员
7. 评分员查看自己的任务
8. 查看评分详情
9. 提交人工评分

对应业务链：

`project -> task -> video -> assign scorer -> my tasks -> detail -> submit`

## 设计文档

设计文档统一以 [`designingDocs/`](/Users/jason/go/src/SkillJudge/backend/designingDocs) 为准，优先参考：

- [`designingDocs/phase.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/phase.md)
- [`designingDocs/api.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/api.md)
- [`designingDocs/platform-schema-reference.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/platform-schema-reference.md)
- [`designingDocs/auth.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/auth.md)
- [`designingDocs/db_connect.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/db_connect.md)

说明：

- [`designingDocs/api.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/api.md) 不应直接改动。
- 当前数据库结构以 [`designingDocs/platform-schema-reference.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/platform-schema-reference.md) 为主。

## 当前已实现模块

- `auth / user`
  - 登录、刷新、登出
  - `/users/me`
  - 用户 CRUD
  - 用户批量创建
  - 正式 RBAC 查询链
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
  - 单视频 AI 评估触发
  - 任务批量 AI 评估触发
  - AI 状态查询
  - AI 结果查询
  - 外部 AI 轮询与结果落库
- `student`
  - 我的结果视频列表
  - 我的结果视频详情

## 当前已实现接口

- `GET /health`
- `GET /ready`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/users/me`
- `PATCH /api/v1/users/me`
- `POST /api/v1/users`
- `POST /api/v1/users/batch`
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

## 关键约定

数据库主线：

- 业务主线：`schools -> projects -> tasks -> videos -> ai_evaluations / manual_evaluations`
- 权限主线：`users -> user_roles -> roles -> role_permissions -> permissions`

认证与权限：

- 登录方式只支持 `username + password`
- `accessToken` 默认有效期 `1h`
- `refreshToken` 默认有效期 `7d`
- Redis 会话 key：`session:{userId}:{sessionId}`
- 运行时权限读取已切到：
  - `user_roles -> role_permissions -> permissions`
- 当前数据库里的学校负责人角色使用 `school_leader`
- 后端实现中 `school_leader` 与 `school_admin` 按同一学校级管理边界处理
- `POST /api/v1/users/batch` 使用 `application/json`，由前端先解析表格再传 `items`
- 用户创建时：
  - `admin` 创建非 `admin` 用户必须提供 `schoolId`
  - `school_admin` / `school_leader` 创建用户时可省略 `schoolId`，后端会自动继承当前操作者学校

视频上传：

- 当前采用华为云 OBS + STS
- `POST /api/v1/videos/upload-credential` 请求体使用 `taskId`
- `GET /api/v1/videos` 查询参数使用 `taskId`
- 对象 key：
  - `projects/{projectId}/tasks/{taskId}/videos/{videoId}/{filename}`

评分：

- 当前仍按单评模式实现
- 评分对象是 `video`
- `videos` 保存评分摘要
- `manual_evaluations` 保存人工评分明细
- `ai_evaluations` 保存 AI 评分状态和完整结果
- 当前 `GET /api/v1/tasks/:id` 是双语义：
  - `scorer` 访问时返回评分详情
  - 非 `scorer` 访问时返回批次详情

AI：

- 当前外部 AI 采用“创建任务 + 轮询状态 + 拉取结果”模式
- `confirm-upload` 成功后，若 AI 已配置，后端会自动触发单视频 AI 评估创建
- 一个 `video` 对应一条 `ai_evaluations`
- 一条 `ai_evaluations` 对应一个外部 `job_id`
- 批量 AI 触发底层仍逐视频创建

学生查分：

- 上传视频时支持从文件名 `姓名_uuid` 中兜底解析 `student_id`
- 学生端当前通过 `videos.student_id = 当前用户.id` 查询自己的视频和评分结果

评分细则模板：

- 当前短期内只保证兼容：
  - [`designingDocs/发动机曲柄连杆机构拆装评分表.xlsx`](/Users/jason/go/src/SkillJudge/backend/designingDocs/发动机曲柄连杆机构拆装评分表.xlsx)

## 本地开发

程序启动时会自动读取项目根目录下的 [`.env`](/Users/jason/go/src/SkillJudge/backend/.env)。

Bootstrap 约定：

- 当前 `BOOTSTRAP_ENABLED` 默认应保持为 `false`
- 只有在明确需要初始化内置角色、权限、默认管理员时，才临时设置为 `true`
- 如果数据库已经由专人维护正式基础数据，日常启动不要开启 bootstrap seed

示例模板：

- [`.env.example`](/Users/jason/go/src/SkillJudge/backend/.env.example)

如果本地通过 SSH 隧道连接 PostgreSQL 和 Redis，先执行：

```bash
ssh -L 15432:192.168.0.146:5432 -L 27018:192.168.0.146:27017 -L 16379:192.168.0.146:6379 root@123.60.51.11
```

然后启动服务：

```bash
cd /Users/jason/go/src/SkillJudge/backend
go run ./cmd/server
```

健康检查：

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/ready
```

## Phase 2 边界

纳入 `Phase 2`：

- AI 视频分析集成
- 人机评分对比
- 学生查分
- 任务分配收口
- 视频播放优化的后端配合

当前不纳入 `Phase 2`：

- `task` 更新/删除
- 视频批量上传
- 视频转码
- 双评、多评、复评、仲裁
- 通知、统计、复杂报表
- AI 深度工程化调度

## 当前状态

当前 `Phase 2` 联调主链已经基本打通，下一步主要是：

1. 继续收口联调问题
2. 稳定 AI 结果结构与前端展示
3. 视需要再推进工程化增强

联调文档可参考：

- [test_auth.md](/Users/jason/go/src/SkillJudge/backend/test_curl/test_auth.md)
- [test_video.md](/Users/jason/go/src/SkillJudge/backend/test_curl/test_video.md)
- [test_phase1_flow.md](/Users/jason/go/src/SkillJudge/backend/test_curl/test_phase1_flow.md)
