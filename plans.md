# SkillJudge Phase 1 计划

## 1. 当前目标

当前阶段目标是完成 `Phase 1` 的最小可运行主链，不继续扩展复杂功能。

当前主参考文档：

- [`designingDocs/phase.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/phase.md)
- [`designingDocs/api.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/api.md)
- [`designingDocs/platform-schema-reference.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/platform-schema-reference.md)
- [`designingDocs/auth.md`](/Users/jason/go/src/SkillJudge/backend/designingDocs/auth.md)

当前数据库主线：

- 业务主线：`schools -> projects -> tasks -> videos -> ai_evaluations / manual_evaluations`
- 权限主线：`users -> user_roles -> roles -> role_permissions -> permissions`

## 2. Phase 1 当前基线

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

## 6. 当前 Phase 1 边界

当前明确纳入 `Phase 1`：

- 登录与用户管理
- 项目管理
- 批次管理
- 评分细则管理与模板解析
- 视频上传主链
- 单评模式评分闭环

当前明确不纳入 `Phase 1`：

- `task` 更新/删除
- 视频批量上传
- 视频转码
- AI 评测
- 双评、多评、复评、仲裁
- 通知与统计深化

## 7. 当前遗留事项

当前最主要的遗留不是新功能，而是联调和收口：

1. 主链联调
- 登录
- 创建项目
- 创建批次
- 创建/上传评分细则
- 上传视频
- 分配评分员
- 评分员查看任务
- 查看评分详情
- 提交评分

2. 测试材料补齐
- `curl` / Postman 风格联调文档
- 关键失败场景说明

3. 联调后问题收口
- 参数对齐问题
- 数据初始化问题
- 权限边界问题
- 上传和评分状态问题

## 8. 当前结论

当前 `Phase 1` 主体工程已经基本完成，后续工作重心应转为：

1. 联调验证
2. 问题收口
3. 再决定是否进入下一阶段功能
