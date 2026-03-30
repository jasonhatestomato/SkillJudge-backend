# SkillJudge Backend 协作说明

## 1. 项目定位

- 本仓库是 SkillJudge 物理实验评分系统后端。
- 当前处于 `Phase 1`，目标是先完成最小可运行主链。
- 当前代码形态是“模块化单体”，不是已拆分的真实微服务。
- 技术栈：
  - `Go`
  - `Gin`
  - `GORM`
  - `PostgreSQL`
  - `Redis`
  - `JWT`
  - 华为云 `OBS + STS`

## 2. 当前主链

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

## 3. 设计文档

设计文档统一以 [`designingDocs/`](/Users/jason/go/src/SkillJudge/backend/designingDocs) 为准。

优先参考：

- [`designingDocs/phase.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/phase.md)
- [`designingDocs/api.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/api.md)
- [`designingDocs/platform-schema-reference.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/platform-schema-reference.md)
- [`designingDocs/auth.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/auth.md)
- [`designingDocs/db_connect.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/db_connect.md)

说明：

- [`designingDocs/api.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/api.md) 不应直接改动。
- 数据库结构以 [`designingDocs/platform-schema-reference.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/platform-schema-reference.md) 为当前主参考。

## 4. 数据库主线与权限主线

当前数据库主线：

- 业务主线：`schools -> projects -> tasks -> videos -> ai_evaluations / manual_evaluations`
- 权限主线：`users -> user_roles -> roles -> role_permissions -> permissions`

当前实现约定：

- `users.role` 和 `roles.permissions` 仍在库中，但只作兼容字段使用。
- 运行时权限校验已经切到：
  - `user_roles -> role_permissions -> permissions`

## 5. 当前业务规则

- 登录方式只支持 `username + password`
- `accessToken` 默认有效期 `1h`
- `refreshToken` 默认有效期 `7d`
- Redis 会话 key：`session:{userId}:{sessionId}`
- 登出仅使当前会话失效
- 只有 `users.status = active` 允许登录

角色边界：

- `admin` 可跨校管理全部资源
- `school_admin` 只能管理本校资源
- `school_leader` 当前按学校负责人角色处理，权限边界与 `school_admin` 一致
- `teacher` 主要管理自己创建的项目、批次、评分细则与视频
- `scorer` 主要执行评分
- `student` 当前基本未展开

当前评分规则：

- `Phase 1` 只支持单评
- 一个视频只分配给一个评分员
- 评分对象是 `video`
- `videos` 表保存评分摘要
- `manual_evaluations` 保存人工评分明细

## 6. 代码结构

主要目录：

- [`cmd/server/main.go`](/Users/jason/go/src/SkillJudge/backend/cmd/server/main.go)
  - 程序入口
- [`internal/app/app.go`](/Users/jason/go/src/SkillJudge/backend/internal/app/app.go)
  - 依赖装配、路由注册
- [`internal/config/`](/Users/jason/go/src/SkillJudge/backend/internal/config)
  - 配置读取
- [`internal/bootstrap/`](/Users/jason/go/src/SkillJudge/backend/internal/bootstrap)
  - 启动种子与默认管理员初始化（默认关闭，需显式开启）
- [`internal/model/models.go`](/Users/jason/go/src/SkillJudge/backend/internal/model/models.go)
  - GORM 模型
- [`internal/http/middleware/`](/Users/jason/go/src/SkillJudge/backend/internal/http/middleware)
  - 鉴权与权限中间件
- [`internal/http/response/`](/Users/jason/go/src/SkillJudge/backend/internal/http/response)
  - 统一响应封装
- [`internal/platform/database/`](/Users/jason/go/src/SkillJudge/backend/internal/platform/database)
  - PostgreSQL 连接
- [`internal/platform/cache/`](/Users/jason/go/src/SkillJudge/backend/internal/platform/cache)
  - Redis 连接
- [`internal/platform/jwt/`](/Users/jason/go/src/SkillJudge/backend/internal/platform/jwt)
  - JWT 签发与解析
- [`internal/platform/storage/`](/Users/jason/go/src/SkillJudge/backend/internal/platform/storage)
  - 对象存储适配层，当前接华为云 OBS + STS

业务模块：

- [`internal/modules/auth/`](/Users/jason/go/src/SkillJudge/backend/internal/modules/auth)
  - 登录、刷新、登出、Redis 会话
- [`internal/modules/user/`](/Users/jason/go/src/SkillJudge/backend/internal/modules/user)
  - 当前用户、管理员用户管理、权限读取
- [`internal/modules/project/`](/Users/jason/go/src/SkillJudge/backend/internal/modules/project)
  - 项目管理、评分细则管理、评分细则 Excel 解析
- [`internal/modules/task/`](/Users/jason/go/src/SkillJudge/backend/internal/modules/task)
  - 批次管理
- [`internal/modules/video/`](/Users/jason/go/src/SkillJudge/backend/internal/modules/video)
  - 上传凭证、上传确认、视频列表、详情、删除
- [`internal/modules/scoring/`](/Users/jason/go/src/SkillJudge/backend/internal/modules/scoring)
  - 评分员分配、我的任务、评分详情、提交评分
- [`internal/modules/system/`](/Users/jason/go/src/SkillJudge/backend/internal/modules/system)
  - `/health`、`/ready`

## 7. 各模块职责

### auth / user

- 登录、刷新、登出
- `/users/me`
- 用户 CRUD
- 基于正式 RBAC 主链的权限读取

### project

- 项目 CRUD
- 当前 `project` 已从 `rubric` 绑定职责中收缩
- `rubricId` 不再作为项目层的持久化主职责

### rubric

- 评分细则 CRUD
- Excel 模板上传解析
- 当前细则内容存 PostgreSQL `JSONB`
- 运行时使用解析后的结构化 `items`

### task

- 一个 `task` 表示项目下的一次具体批次/轮次
- 当前已实现：
  - 创建
  - 项目下列表
  - 详情
- `task` 是当前 `project -> task -> video` 主链里的中间层

### video

- 视频归属已切到 `task`
- 上传流程：
  - 获取上传凭证
  - 前端分片直传 OBS
  - `confirm-upload`
  - 视频进入 `ready`
- 当前不做转码

### scoring

- 当前按单评模式实现
- 已实现：
  - 评分员分配
  - 我的任务列表
  - 评分详情
  - 提交人工评分
- 当前“评分任务视图”落在 `video` 上，没有额外独立复杂任务主表

## 8. 当前已实现接口

系统：

- `GET /health`
- `GET /ready`

认证与用户：

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/users/me`
- `PATCH /api/v1/users/me`
- `POST /api/v1/users`
- `GET /api/v1/users`
- `PATCH /api/v1/users/:id`
- `DELETE /api/v1/users/:id`

项目与评分细则：

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

批次：

- `POST /api/v1/projects/:projectId/tasks`
- `GET /api/v1/projects/:projectId/tasks`
- `GET /api/v1/tasks/:id`

视频：

- `POST /api/v1/videos/upload-credential`
- `POST /api/v1/videos/:id/confirm-upload`
- `GET /api/v1/videos`
- `GET /api/v1/videos/:id`
- `DELETE /api/v1/videos/:id`

评分：

- `GET /api/v1/tasks/my`
- `POST /api/v1/tasks/:id/assignments`
- `POST /api/v1/tasks/:id/submit`
- `GET /api/v1/tasks/:id`

说明：

- `GET /api/v1/tasks/:id` 当前是双语义：
  - `scorer` 访问时，返回评分详情视图
  - 非 `scorer` 访问时，返回批次详情
- 当前评分详情里的 `id` 实际落在 `video.id`

## 9. 评分细则模板约定

当前短期内只保证兼容：

- [`designingDocs/发动机曲柄连杆机构拆装评分表.xlsx`](/Users/jason/go/src/SkillJudge/backend/designingDocs/发动机曲柄连杆机构拆装评分表.xlsx)

当前解析规则：

- 只解析第一个 sheet
- 当前模板按 7 列结构解析：
  - `一级指标`
  - `一级指标分值`
  - `二级指标`
  - `二级指标分值`
  - `满分标准`
  - `扣分事项`
  - `备注`
- 若二级指标分值缺失，则按一级指标总分平均分配，最后一项补差
- “备注” 当前临时映射到 `dangerousOperation`

## 10. 视频上传约定

当前采用华为云 OBS 方案：

- 后端使用长期 `AK/SK` 向 IAM 申请 STS 临时凭证
- 前端使用临时凭证直传 OBS
- `confirm-upload` 成功后直接进入 `ready`

当前对象 key 规则：

- `projects/{projectId}/tasks/{taskId}/videos/{videoId}/{filename}`

当前接口约定：

- `POST /api/v1/videos/upload-credential` 请求体使用 `taskId`
- `GET /api/v1/videos` 查询参数使用 `taskId`

说明：

- 这里与 [`designingDocs/api.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/api.md) 中仍写 `projectId` 的地方存在差异。
- 现在以前端和后端已对齐的当前实现为准。

STS 返回结构：

- `accessKeyId`
- `accessKeySecret`
- `securityToken`
- `expiration`

## 11. 本地运行

程序会自动读取根目录下的 [`.env`](/Users/jason/go/src/SkillJudge/backend/.env)。

Bootstrap 约定：

- 当前 `BOOTSTRAP_ENABLED` 默认应保持为 `false`
- 只有在明确需要初始化内置角色、权限、默认管理员时，才临时设置为 `true`
- 对已经由数据库负责人维护正式数据的环境，不应在日常启动时开启 bootstrap seed

示例配置：

- [`.env.example`](/Users/jason/go/src/SkillJudge/backend/.env.example)

本地开发常用方式：

```bash
cd /Users/jason/go/src/SkillJudge/backend
go run ./cmd/server
```

如果本地通过 SSH 隧道访问数据库和 Redis，先建立隧道：

```bash
ssh -L 15432:192.168.0.146:5432 -L 27018:192.168.0.146:27017 -L 16379:192.168.0.146:6379 root@123.60.51.11
```

健康检查：

- `/health`：进程存活
- `/ready`：检查 PostgreSQL 与 Redis

## 12. 当前 Phase 1 边界

纳入 `Phase 1`：

- 登录与用户管理
- 项目管理
- 批次管理
- 评分细则管理与模板解析
- 视频上传主链
- 单评模式评分闭环

不纳入 `Phase 1`：

- `task` 更新/删除
- 视频批量上传
- 视频转码
- AI 评测
- 双评、多评、复评、仲裁
- 通知、统计、复杂报表

## 13. 当前状态

当前状态判断：

- `Phase 1` 主体工程已经基本完成
- 剩余工作主要是联调、问题收口和测试材料完善

当前已经补过的联调文档：

- [test_auth.md](/Users/jason/go/src/SkillJudge/backend/test_curl/test_auth.md)
- [test_video.md](/Users/jason/go/src/SkillJudge/backend/test_curl/test_video.md)
- [test_phase1_flow.md](/Users/jason/go/src/SkillJudge/backend/test_curl/test_phase1_flow.md)

## 14. 后续接手建议

接手时建议按下面顺序了解项目：

1. 先看 [`plans.md`](/Users/jason/go/src/SkillJudge/backend/plans.md)
2. 再看 [`designingDocs/platform-schema-reference.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/platform-schema-reference.md)
3. 再看 [`internal/app/app.go`](/Users/jason/go/src/SkillJudge/backend/internal/app/app.go) 了解模块装配和路由
4. 按模块阅读：
   - `auth/user`
   - `project/rubric`
   - `task`
   - `video`
   - `scoring`
5. 最后参考联调文档走一遍主链

当前最可能继续推进的方向：

- 完整联调与问题收口
- 视业务讨论结果补 `task` 更新/删除
- Phase 2 再规划 AI、多评、转码等能力
