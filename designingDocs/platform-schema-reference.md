# SkillJudge 平台库当前表结构说明

## 1. 文档范围

- 适用数据库：`skilljudge_platform_db`
- 结构基准：以当前实际运行中的平台库结构为准
- 更新时间：2026-03-24
- 不包含：`skilljudge_hardware_db`、`skilljudge_ai_db` 的详细表设计。这两个库目前仍在单独设计中

这份文档的目标只有两件事：

1. 把当前平台库每张表、每个字段的含义写清楚。
2. 把表与表之间的关系讲清楚，方便后续开发、排错、接口联调和结构继续演进。

## 2. 当前平台库总览

当前平台库共有 15 张表：

| 模块 | 表名 | 作用 |
| --- | --- | --- |
| 组织 | `schools` | 学校主数据 |
| 组织 | `users` | 用户主数据 |
| RBAC | `roles` | 角色定义 |
| RBAC | `permissions` | 权限字典 |
| RBAC | `user_roles` | 用户与角色关联 |
| RBAC | `role_permissions` | 角色与权限关联 |
| 业务 | `scoring_rubrics` | 评分细则 |
| 业务 | `projects` | 实验项目 |
| 业务 | `tasks` | 考试批次 |
| 业务 | `videos` | 视频与评分摘要 |
| 评分 | `ai_evaluations` | AI 评测记录 |
| 评分 | `manual_evaluations` | 人工评测记录 |
| 系统 | `notifications` | 站内通知 |
| 系统 | `audit_logs` | 审计日志 |
| 统计 | `project_statistics` | 项目统计汇总 |

## 3. 核心设计说明

### 3.1 当前主业务链

当前平台库的核心业务链路是：

`schools -> projects -> tasks -> videos -> ai_evaluations / manual_evaluations`

同时，人员与权限链路是：

`users -> user_roles -> roles -> role_permissions -> permissions`

### 3.2 需要特别注意的兼容字段

- `users.role` 仍然存在，主要用于兼容旧逻辑或做快速角色判断。
- `roles.permissions` 仍然存在，主要用于兼容旧版 JSONB 权限设计。
- 当前正式的 RBAC 关系应优先看：`user_roles -> roles -> role_permissions -> permissions`。

### 3.3 当前评分字段的职责边界

- `videos.ai_score`、`videos.manual_score` 只保存视频维度的评分摘要。
- AI 评分明细以 `ai_evaluations.result_data` 为主。
- 人工评分明细以 `manual_evaluations.score_details` 为主。
- `score_difference` 已不再单独存储在 `videos` 表中，查询时动态计算：

```sql
SELECT
    id,
    ai_score,
    manual_score,
    (ai_score - manual_score) AS score_difference
FROM videos;
```

## 4. 表结构说明

### 4.1 `schools`

用途：存储学校基础信息，是用户、评分细则、项目的组织归属根节点。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 学校主键 | 主键，默认 `gen_random_uuid()` |
| `name` | `VARCHAR(100)` | 学校名称 | 必填 |
| `code` | `VARCHAR(50)` | 学校编码 | 唯一，可用于外部系统对接 |
| `province` | `VARCHAR(50)` | 省份 | 可空 |
| `city` | `VARCHAR(50)` | 城市 | 可空 |
| `district` | `VARCHAR(50)` | 区县 | 可空 |
| `address` | `VARCHAR(255)` | 详细地址 | 可空 |
| `contact_person` | `VARCHAR(50)` | 联系人 | 可空 |
| `contact_phone` | `VARCHAR(20)` | 联系电话 | 可空 |
| `contact_email` | `VARCHAR(100)` | 联系邮箱 | 可空 |
| `status` | `VARCHAR(20)` | 学校状态 | 默认 `active`，当前未做数据库级枚举约束 |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |
| `metadata` | `JSONB` | 扩展信息 | 用于放非固定结构字段 |

关系：

- 一个学校可以拥有多个用户。
- 一个学校可以拥有多个评分细则。
- 一个学校可以拥有多个项目。

### 4.2 `users`

用途：存储平台用户，包括平台管理员、校长、学校管理员、教师、评分员、学生。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 用户主键 | 主键 |
| `username` | `VARCHAR(50)` | 登录用户名 | 唯一，必填 |
| `password_hash` | `VARCHAR(255)` | 密码哈希 | 存储哈希值，不存明文 |
| `email` | `VARCHAR(100)` | 邮箱 | 可空 |
| `phone` | `VARCHAR(20)` | 手机号 | 可空 |
| `real_name` | `VARCHAR(50)` | 真实姓名 | 可空 |
| `avatar_url` | `VARCHAR(500)` | 头像地址 | 可空 |
| `role` | `VARCHAR(20)` | 兼容用角色字段 | 常见值：`admin`、`principal`、`school_admin`、`teacher`、`scorer`、`student` |
| `status` | `VARCHAR(20)` | 用户状态 | 默认 `active`，当前未做数据库级枚举约束 |
| `school_id` | `UUID` | 所属学校 | 外键到 `schools.id`，平台管理员可为空 |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |
| `last_login_at` | `TIMESTAMP` | 最近登录时间 | 可空 |
| `metadata` | `JSONB` | 扩展信息 | 可空 |

关系：

- 多个用户属于一个学校。
- 一个用户可以创建多个评分细则、项目、任务。
- 一个用户可以作为学生上传多个视频。
- 一个用户可以作为评分员处理多个视频，并产生多条人工评分记录。
- 一个用户可以收到多条通知。
- 一个用户可以产生多条审计日志。
- 一个用户可以拥有多个角色。

### 4.3 `roles`

用途：定义系统角色。当前既保留旧版 JSON 权限字段，也承载正式 RBAC 角色元数据。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 角色主键 | 主键 |
| `name` | `VARCHAR(50)` | 角色名称 | 唯一，例如“学校管理员” |
| `code` | `VARCHAR(50)` | 角色编码 | 唯一，例如 `school_admin` |
| `description` | `TEXT` | 角色说明 | 可空 |
| `permissions` | `JSONB` | 旧版权限列表 | 兼容字段，默认空数组 `[]` |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |
| `scope_type` | `VARCHAR(20)` | 角色作用域 | `platform` 或 `school` |
| `is_builtin` | `BOOLEAN` | 是否内置角色 | 默认 `true` |
| `is_super_admin` | `BOOLEAN` | 是否超级管理员角色 | 默认 `false` |
| `status` | `VARCHAR(20)` | 角色状态 | `active` 或 `disabled` |

关系：

- 一个角色可以赋给多个用户。
- 一个角色可以关联多个权限。

补充说明：

- `roles.permissions` 仍可用于兼容旧逻辑，但正式授权关系应以 `role_permissions` 为准。
- 当前“校长”角色已存在，且权限暂时与“学校管理员”保持一致。

### 4.4 `permissions`

用途：权限字典表，定义最细粒度的权限点。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 权限主键 | 主键 |
| `code` | `VARCHAR(100)` | 权限编码 | 唯一，例如 `project:create` |
| `resource_code` | `VARCHAR(50)` | 资源编码 | 例如 `project` |
| `action_code` | `VARCHAR(50)` | 动作编码 | 例如 `create` |
| `name` | `VARCHAR(100)` | 权限名称 | 可读名称或同 `code` |
| `description` | `TEXT` | 权限说明 | 可空 |
| `module` | `VARCHAR(50)` | 所属模块 | 通常与 `resource_code` 一致 |
| `is_builtin` | `BOOLEAN` | 是否内置权限 | 默认 `true` |
| `status` | `VARCHAR(20)` | 权限状态 | `active` 或 `disabled` |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |

关系：

- 一个权限可以被多个角色引用。
- `permissions` 与 `roles` 之间是多对多关系，通过 `role_permissions` 连接。

关键约束：

- `code` 唯一。
- `(resource_code, action_code)` 组合唯一。

### 4.5 `user_roles`

用途：用户与角色的关联表，用于给用户授予角色。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 关联记录主键 | 主键 |
| `user_id` | `UUID` | 用户 ID | 外键到 `users.id` |
| `role_id` | `UUID` | 角色 ID | 外键到 `roles.id` |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `status` | `VARCHAR(20)` | 分配状态 | `active`、`disabled`、`expired` |
| `granted_by` | `UUID` | 授权人 | 外键到 `users.id`，可空 |
| `starts_at` | `TIMESTAMP` | 生效时间 | 可空 |
| `ends_at` | `TIMESTAMP` | 失效时间 | 可空 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |

关系：

- 多条 `user_roles` 记录可以指向同一个用户。
- 多条 `user_roles` 记录可以指向同一个角色。
- `granted_by` 表示是谁授予了这个角色，本质上也是一个用户。

关键约束：

- `(user_id, role_id)` 组合唯一，避免同一用户重复挂同一角色。

### 4.6 `role_permissions`

用途：角色与权限的关联表，是正式 RBAC 授权链路中的关键中间表。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `role_id` | `UUID` | 角色 ID | 外键到 `roles.id` |
| `permission_id` | `UUID` | 权限 ID | 外键到 `permissions.id` |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `created_by` | `UUID` | 授权操作人 | 外键到 `users.id`，可空 |

关系：

- 多个角色可以共享同一个权限。
- 一个角色可以包含多个权限。

关键约束：

- 主键是 `(role_id, permission_id)` 复合主键。

### 4.7 `scoring_rubrics`

用途：评分细则模板。一个 `task` 必须绑定一个评分细则。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 评分细则主键 | 主键 |
| `name` | `VARCHAR(100)` | 评分细则名称 | 必填 |
| `description` | `TEXT` | 评分细则说明 | 可空 |
| `total_score` | `INT` | 总分 | 默认 `100` |
| `template_type` | `VARCHAR(50)` | 模板类型 | 可空 |
| `school_id` | `UUID` | 所属学校 | 外键到 `schools.id` |
| `creator_id` | `UUID` | 创建人 | 外键到 `users.id` |
| `is_template` | `BOOLEAN` | 是否模板 | 默认 `false` |
| `is_public` | `BOOLEAN` | 是否公开 | 默认 `false` |
| `items` | `JSONB` | 评分项结构 | 必填，存一级指标、二级指标、配分等 |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |

关系：

- 一个学校可以有多个评分细则。
- 一个用户可以创建多个评分细则。
- 一个评分细则可以被多个 `task` 使用。
- 人工评分记录也会回写其使用的 `rubric_id`。

### 4.8 `projects`

用途：实验项目。一个学校可以有多个项目，一个项目下可以有多个考试批次。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 项目主键 | 主键 |
| `name` | `VARCHAR(100)` | 项目名称 | 必填 |
| `description` | `TEXT` | 项目说明 | 可空 |
| `school_id` | `UUID` | 所属学校 | 外键到 `schools.id` |
| `creator_id` | `UUID` | 创建人 | 外键到 `users.id` |
| `status` | `VARCHAR(20)` | 项目状态 | 常用：`draft`、`in_progress`、`completed`、`archived` |
| `deadline` | `TIMESTAMP` | 截止时间 | 可空 |
| `start_date` | `TIMESTAMP` | 开始时间 | 可空 |
| `end_date` | `TIMESTAMP` | 结束时间 | 可空 |
| `tags` | `VARCHAR(255)[]` | 标签数组 | 可空 |
| `experiment_type` | `VARCHAR(50)` | 实验类型 | 可空 |
| `grade_level` | `VARCHAR(20)` | 年级 | 可空 |
| `subject` | `VARCHAR(50)` | 学科 | 可空 |
| `total_videos` | `INT` | 项目视频总数 | 摘要统计，默认 `0` |
| `completed_videos` | `INT` | 已完成评分视频数 | 摘要统计，默认 `0` |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |
| `metadata` | `JSONB` | 扩展信息 | 可空 |

关系：

- 一个学校可以有多个项目。
- 一个用户可以创建多个项目。
- 一个项目可以包含多个 `task`。
- 一个项目可以对应项目级统计汇总。

### 4.9 `tasks`

用途：考试批次。一个项目下可以有多个批次，一个批次绑定一套评分细则。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 批次主键 | 主键 |
| `project_id` | `UUID` | 所属项目 | 外键到 `projects.id` |
| `name` | `VARCHAR(100)` | 批次名称 | 必填；同一项目内唯一 |
| `description` | `TEXT` | 批次说明 | 可空 |
| `rubric_id` | `UUID` | 绑定的评分细则 | 外键到 `scoring_rubrics.id` |
| `creator_id` | `UUID` | 创建人 | 外键到 `users.id` |
| `status` | `VARCHAR(20)` | 批次状态 | 常用：`draft`、`in_progress`、`completed`、`archived` |
| `start_date` | `TIMESTAMP` | 开始时间 | 可空 |
| `deadline` | `TIMESTAMP` | 截止时间 | 可空 |
| `end_date` | `TIMESTAMP` | 结束时间 | 可空 |
| `total_videos` | `INT` | 批次视频总数 | 摘要统计，默认 `0` |
| `completed_videos` | `INT` | 批次完成评分视频数 | 摘要统计，默认 `0` |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |
| `metadata` | `JSONB` | 扩展信息 | 可空 |

关系：

- 一个项目可以有多个批次。
- 一个评分细则可以被多个批次使用。
- 一个批次可以包含多个视频。
- 一个批次可以对应多条 AI 评测记录和多条人工评测记录。

关键约束：

- `(project_id, name)` 组合唯一。

### 4.10 `videos`

用途：视频主表。保存文件信息、归属批次、学生、分配评分员，以及视频维度的评分摘要。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 视频主键 | 主键 |
| `student_id` | `UUID` | 上传学生 ID | 外键到 `users.id`，业务上通常应为学生用户 |
| `student_name` | `VARCHAR(50)` | 学生姓名快照 | 冗余字段，避免列表页频繁回表 |
| `student_number` | `VARCHAR(50)` | 学号快照 | 冗余字段 |
| `filename` | `VARCHAR(255)` | 存储后的文件名 | 必填 |
| `original_filename` | `VARCHAR(255)` | 原始文件名 | 可空 |
| `file_size` | `BIGINT` | 文件大小 | 单位通常为字节 |
| `duration` | `INT` | 视频时长 | 单位通常为秒 |
| `resolution` | `VARCHAR(20)` | 分辨率 | 例如 `1920x1080` |
| `format` | `VARCHAR(20)` | 文件格式 | 例如 `mp4` |
| `storage_path` | `VARCHAR(500)` | 存储路径 | 必填 |
| `thumbnail_url` | `VARCHAR(500)` | 缩略图地址 | 可空 |
| `status` | `VARCHAR(20)` | 视频处理状态 | 常用：`uploading`、`uploaded`、`transcoding`、`ready`、`failed` |
| `transcode_status` | `VARCHAR(20)` | 转码状态 | 可空，具体值由转码流程决定 |
| `upload_progress` | `INT` | 上传进度 | 默认 `0` |
| `uploaded_at` | `TIMESTAMP` | 上传完成时间 | 可空 |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |
| `metadata` | `JSONB` | 扩展信息 | 可空 |
| `task_id` | `UUID` | 所属考试批次 | 外键到 `tasks.id`，必填 |
| `scorer_id` | `UUID` | 分配评分员 | 外键到 `users.id`，可空 |
| `evaluation_status` | `VARCHAR(20)` | 总体评分状态 | `pending`、`in_progress`、`completed` |
| `ai_status` | `VARCHAR(20)` | AI 评分状态 | 当前常见值为 `pending`、`processing`、`completed` |
| `manual_status` | `VARCHAR(20)` | 人工评分状态 | 当前常见值为 `pending`、`in_progress`、`submitted` |
| `ai_score` | `DECIMAL(5,2)` | AI 总分摘要 | 从 AI 评测记录回填 |
| `manual_score` | `DECIMAL(5,2)` | 人工总分摘要 | 从人工评测记录回填 |
| `assigned_at` | `TIMESTAMP` | 评分员分配时间 | 可空 |
| `completed_at` | `TIMESTAMP` | 评分完成时间 | 可空 |

关系：

- 一个批次可以有多个视频。
- 一个学生可以对应多个视频。
- 一个评分员可以被分配多个视频。
- 一个视频可以对应多条 AI 评测记录。
- 一个视频可以对应多条人工评测记录。

补充说明：

- 业务上是一条视频对应一位学生。
- `videos` 是视频维度的摘要表，不承担 AI 明细和人工小分明细的最终存储职责。

### 4.11 `ai_evaluations`

用途：保存 AI 评测结果记录。适合存状态、总分、错误信息和结果入口。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | AI 评测记录主键 | 主键 |
| `task_id` | `UUID` | 所属批次 | 外键到 `tasks.id` |
| `video_id` | `UUID` | 对应视频 | 外键到 `videos.id` |
| `model_version` | `VARCHAR(50)` | 模型版本 | 可空 |
| `total_score` | `DECIMAL(5,2)` | AI 总分 | 可空 |
| `status` | `VARCHAR(20)` | AI 评测状态 | 默认 `processing` |
| `started_at` | `TIMESTAMP` | 开始评测时间 | 可空 |
| `completed_at` | `TIMESTAMP` | 完成评测时间 | 可空 |
| `error_message` | `TEXT` | 失败原因 | 可空 |
| `result_data` | `JSONB` | AI 完整结果 JSON | 可空，存小分、分项解释、结构化结果等 |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |

关系：

- 多条 AI 评测记录可以指向同一个视频。
- 多条 AI 评测记录可以指向同一个批次。

补充说明：

- 当前库结构没有限制一个视频只能有一条 AI 评测记录，所以它天然支持重跑、重试、模型版本切换后的历史留存。
- 视频列表页通常读取 `videos.ai_score`，明细或历史追踪则看 `ai_evaluations`。

### 4.12 `manual_evaluations`

用途：保存人工评分结果，包括总分、分项明细、评语和评分耗时。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 人工评测记录主键 | 主键 |
| `task_id` | `UUID` | 所属批次 | 外键到 `tasks.id` |
| `video_id` | `UUID` | 对应视频 | 外键到 `videos.id` |
| `scorer_id` | `UUID` | 评分员 ID | 外键到 `users.id` |
| `rubric_id` | `UUID` | 评分细则 ID | 外键到 `scoring_rubrics.id`，可空但业务上通常应有值 |
| `total_score` | `DECIMAL(5,2)` | 人工总分 | 可空 |
| `score_details` | `JSONB` | 分项得分明细 | 可空，保存各评分项分数和说明 |
| `comments` | `TEXT` | 评分评语 | 可空 |
| `status` | `VARCHAR(20)` | 人工评分状态 | 默认 `in_progress` |
| `started_at` | `TIMESTAMP` | 开始评分时间 | 可空 |
| `submitted_at` | `TIMESTAMP` | 提交评分时间 | 可空 |
| `time_spent` | `INT` | 耗时 | 一般可理解为秒数 |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |
| `updated_at` | `TIMESTAMP` | 更新时间 | 默认当前时间 |

关系：

- 多条人工评分记录可以指向同一个视频。
- 多条人工评分记录可以指向同一个批次。
- 多条人工评分记录可以由同一个评分员产生。
- 多条人工评分记录可以引用同一评分细则。

补充说明：

- 当前结构允许一个视频存在多条人工评分记录，因此可扩展到复评、重评、仲裁等场景。
- 视频维度摘要分使用 `videos.manual_score`，小分和评语明细使用 `manual_evaluations.score_details` 与 `comments`。

### 4.13 `notifications`

用途：用户通知表，用于系统消息、任务分配通知、结果通知等。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 通知主键 | 主键 |
| `user_id` | `UUID` | 接收用户 ID | 外键到 `users.id` |
| `type` | `VARCHAR(50)` | 通知类型 | 例如系统消息、任务通知、结果通知 |
| `title` | `VARCHAR(200)` | 标题 | 必填 |
| `content` | `TEXT` | 内容 | 可空 |
| `link_url` | `VARCHAR(500)` | 跳转链接 | 可空 |
| `is_read` | `BOOLEAN` | 是否已读 | 默认 `false` |
| `read_at` | `TIMESTAMP` | 已读时间 | 可空 |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |

关系：

- 一个用户可以收到多条通知。

### 4.14 `audit_logs`

用途：审计日志表，用于记录用户行为和请求轨迹，服务于排障、安全审计、合规留痕。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 日志主键 | 主键 |
| `user_id` | `UUID` | 操作用户 ID | 外键到 `users.id`，可空，兼容匿名请求或系统动作 |
| `action` | `VARCHAR(100)` | 操作名称 | 必填，例如 `project.create` |
| `resource_type` | `VARCHAR(50)` | 资源类型 | 例如 `project`、`video` |
| `resource_id` | `UUID` | 资源 ID | 可空 |
| `ip_address` | `VARCHAR(45)` | IP 地址 | 可空，兼容 IPv4/IPv6 |
| `user_agent` | `TEXT` | 浏览器或客户端标识 | 可空 |
| `request_method` | `VARCHAR(10)` | HTTP 方法 | 例如 `GET`、`POST` |
| `request_path` | `VARCHAR(500)` | 请求路径 | 可空 |
| `request_body` | `JSONB` | 请求体摘要 | 可空 |
| `response_status` | `INT` | 响应状态码 | 可空 |
| `error_message` | `TEXT` | 错误信息 | 可空 |
| `created_at` | `TIMESTAMP` | 创建时间 | 默认当前时间 |

关系：

- 一个用户可以对应多条审计日志。
- 审计日志也允许没有 `user_id`，用于系统级事件记录。

### 4.15 `project_statistics`

用途：项目级统计汇总表，用于缓存项目的聚合指标，减少频繁联表实时计算。

| 字段 | 类型 | 含义 | 备注 |
| --- | --- | --- | --- |
| `id` | `UUID` | 统计记录主键 | 主键 |
| `project_id` | `UUID` | 对应项目 ID | 外键到 `projects.id` |
| `total_videos` | `INT` | 项目视频总数 | 默认 `0` |
| `completed_videos` | `INT` | 已完成评分视频数 | 默认 `0` |
| `pending_videos` | `INT` | 未完成评分视频数 | 默认 `0` |
| `avg_ai_score` | `DECIMAL(5,2)` | AI 平均分 | 可空 |
| `avg_manual_score` | `DECIMAL(5,2)` | 人工平均分 | 可空 |
| `avg_score_difference` | `DECIMAL(5,2)` | 平均分差 | 由 `ai_score - manual_score` 聚合得到 |
| `completion_rate` | `DECIMAL(5,2)` | 完成率 | 一般按百分比存储 |
| `statistics_data` | `JSONB` | 额外统计数据 | 可空，可放分布、趋势等 |
| `calculated_at` | `TIMESTAMP` | 最近计算时间 | 默认当前时间 |

关系：

- 逻辑上一个项目应对应一条项目统计。
- 当前库中 `project_id` 只有普通索引，没有唯一约束，所以“一项目一条统计”是业务约定，不是数据库强约束。

补充说明：

- `avg_score_difference` 仍然保留在聚合表中是合理的，因为这是缓存统计值。
- 视频层的 `score_difference` 已经删除，避免逐行冗余存储。

## 5. 表与表之间的关系

### 5.1 组织与权限关系

| 主表 | 关系 | 从表 | 说明 |
| --- | --- | --- | --- |
| `schools` | 1:N | `users` | 一个学校下有多个用户 |
| `schools` | 1:N | `scoring_rubrics` | 一个学校下可维护多套评分细则 |
| `schools` | 1:N | `projects` | 一个学校可创建多个项目 |
| `users` | 1:N | `user_roles` | 一个用户可拥有多个角色 |
| `roles` | 1:N | `user_roles` | 一个角色可分配给多个用户 |
| `roles` | 1:N | `role_permissions` | 一个角色可关联多个权限 |
| `permissions` | 1:N | `role_permissions` | 一个权限可被多个角色共享 |
| `users` | 1:N | `role_permissions` | `created_by` 记录谁授予了角色权限 |
| `users` | 1:N | `user_roles` | `granted_by` 记录谁给用户授予了角色 |

### 5.2 业务主链关系

| 主表 | 关系 | 从表 | 说明 |
| --- | --- | --- | --- |
| `projects` | 1:N | `tasks` | 一个项目下有多个考试批次 |
| `scoring_rubrics` | 1:N | `tasks` | 一个批次绑定一套评分细则；一套细则可复用于多个批次 |
| `tasks` | 1:N | `videos` | 一个批次下有多个视频 |
| `users` | 1:N | `videos` | 一个学生可上传多个视频；`student_id` 指向学生 |
| `users` | 1:N | `videos` | 一个评分员可被分配多个视频；`scorer_id` 指向评分员 |
| `tasks` | 1:N | `ai_evaluations` | 一个批次中可有多条 AI 评测记录 |
| `videos` | 1:N | `ai_evaluations` | 一个视频可有多条 AI 评测历史 |
| `tasks` | 1:N | `manual_evaluations` | 一个批次中可有多条人工评分记录 |
| `videos` | 1:N | `manual_evaluations` | 一个视频可有多条人工评分历史 |
| `users` | 1:N | `manual_evaluations` | 一个评分员可产生多条人工评分记录 |
| `scoring_rubrics` | 1:N | `manual_evaluations` | 人工评分记录可带回其所用细则 |

### 5.3 系统与统计关系

| 主表 | 关系 | 从表 | 说明 |
| --- | --- | --- | --- |
| `users` | 1:N | `notifications` | 一个用户可接收多条通知 |
| `users` | 1:N | `audit_logs` | 一个用户可产生多条操作日志 |
| `projects` | 1:N（逻辑上 1:1） | `project_statistics` | 当前设计希望一个项目只有一条统计汇总 |

## 6. 典型查询路径

### 6.1 查询一个项目下的所有批次

`projects -> tasks`

### 6.2 查询一个批次下的所有视频

`tasks -> videos`

### 6.3 查询某个视频的评分摘要

直接读 `videos.ai_score`、`videos.manual_score`、`videos.evaluation_status`

### 6.4 查询某个视频的 AI 与人工评分明细

`videos -> ai_evaluations`

`videos -> manual_evaluations`

### 6.5 查询某个用户的最终权限

`users -> user_roles -> roles -> role_permissions -> permissions`

## 7. 当前结构中的几个重要约定

### 7.1 关于 `users.role`

- 这是兼容字段，不建议继续把它当作唯一权限来源。
- 如果后续平台端完全切到正式 RBAC，可以逐步把权限判断统一到 `user_roles + role_permissions`。

### 7.2 关于 `roles.permissions`

- 这是兼容旧版权限 JSON 的字段。
- 当前正式授权关系已经拆分到 `permissions` 和 `role_permissions`。
- 如果未来确认旧逻辑全部下线，可以考虑把它降级为纯缓存字段，甚至最终移除。

### 7.3 关于视频评分摘要与评分明细

- `videos` 只负责摘要。
- `ai_evaluations`、`manual_evaluations` 负责明细和历史。
- 分差不落库存储，统一在查询时计算。

### 7.4 关于 `project_statistics`

- 这是缓存型聚合表，不是原始事实表。
- 它适合存高频展示指标，但不应替代底层事实数据。

## 8. 一句话总结

当前平台库已经形成了比较清晰的两条主线：

- 业务主线：`学校 -> 项目 -> 批次 -> 视频 -> AI/人工评分`
- 权限主线：`用户 -> 角色 -> 权限`

如果后续继续演进，最值得保持稳定的就是这两条主线不要再混。

## 9. 现网字段速查附录

这一节只做一件事：把当前平台库每张表的字段按现网结构逐列列出来。

- 字段含义请看第 4 节。
- 这一节更适合开发联调、SQL 编写、接口出参核对。
- `可空` 一列中：`否` 表示 `NOT NULL`，`是` 表示允许为空。
- `默认值` 直接按数据库当前默认表达式展示。

### 9.1 `schools`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `name` | `character varying(100)` | 否 |  |
| `code` | `character varying(50)` | 是 |  |
| `province` | `character varying(50)` | 是 |  |
| `city` | `character varying(50)` | 是 |  |
| `district` | `character varying(50)` | 是 |  |
| `address` | `character varying(255)` | 是 |  |
| `contact_person` | `character varying(50)` | 是 |  |
| `contact_phone` | `character varying(20)` | 是 |  |
| `contact_email` | `character varying(100)` | 是 |  |
| `status` | `character varying(20)` | 是 | `'active'::character varying` |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `metadata` | `jsonb` | 是 |  |

### 9.2 `users`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `username` | `character varying(50)` | 否 |  |
| `password_hash` | `character varying(255)` | 否 |  |
| `email` | `character varying(100)` | 是 |  |
| `phone` | `character varying(20)` | 是 |  |
| `real_name` | `character varying(50)` | 是 |  |
| `avatar_url` | `character varying(500)` | 是 |  |
| `role` | `character varying(20)` | 否 |  |
| `status` | `character varying(20)` | 是 | `'active'::character varying` |
| `school_id` | `uuid` | 是 |  |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `last_login_at` | `timestamp without time zone` | 是 |  |
| `metadata` | `jsonb` | 是 |  |

### 9.3 `roles`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `name` | `character varying(50)` | 否 |  |
| `code` | `character varying(50)` | 否 |  |
| `description` | `text` | 是 |  |
| `permissions` | `jsonb` | 是 | `'[]'::jsonb` |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `scope_type` | `character varying(20)` | 否 | `'school'::character varying` |
| `is_builtin` | `boolean` | 否 | `true` |
| `is_super_admin` | `boolean` | 否 | `false` |
| `status` | `character varying(20)` | 否 | `'active'::character varying` |

### 9.4 `permissions`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `code` | `character varying(100)` | 否 |  |
| `resource_code` | `character varying(50)` | 否 |  |
| `action_code` | `character varying(50)` | 否 |  |
| `name` | `character varying(100)` | 否 |  |
| `description` | `text` | 是 |  |
| `module` | `character varying(50)` | 是 |  |
| `is_builtin` | `boolean` | 否 | `true` |
| `status` | `character varying(20)` | 否 | `'active'::character varying` |
| `created_at` | `timestamp without time zone` | 否 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 否 | `CURRENT_TIMESTAMP` |

### 9.5 `user_roles`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `user_id` | `uuid` | 否 |  |
| `role_id` | `uuid` | 否 |  |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `status` | `character varying(20)` | 否 | `'active'::character varying` |
| `granted_by` | `uuid` | 是 |  |
| `starts_at` | `timestamp without time zone` | 是 |  |
| `ends_at` | `timestamp without time zone` | 是 |  |
| `updated_at` | `timestamp without time zone` | 否 | `CURRENT_TIMESTAMP` |

### 9.6 `role_permissions`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `role_id` | `uuid` | 否 |  |
| `permission_id` | `uuid` | 否 |  |
| `created_at` | `timestamp without time zone` | 否 | `CURRENT_TIMESTAMP` |
| `created_by` | `uuid` | 是 |  |

### 9.7 `scoring_rubrics`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `name` | `character varying(100)` | 否 |  |
| `description` | `text` | 是 |  |
| `total_score` | `integer` | 否 | `100` |
| `template_type` | `character varying(50)` | 是 |  |
| `school_id` | `uuid` | 是 |  |
| `creator_id` | `uuid` | 是 |  |
| `is_template` | `boolean` | 是 | `false` |
| `is_public` | `boolean` | 是 | `false` |
| `items` | `jsonb` | 否 |  |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |

### 9.8 `projects`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `name` | `character varying(100)` | 否 |  |
| `description` | `text` | 是 |  |
| `school_id` | `uuid` | 是 |  |
| `creator_id` | `uuid` | 是 |  |
| `status` | `character varying(20)` | 是 | `'draft'::character varying` |
| `deadline` | `timestamp without time zone` | 是 |  |
| `start_date` | `timestamp without time zone` | 是 |  |
| `end_date` | `timestamp without time zone` | 是 |  |
| `tags` | `character varying(255)[]` | 是 |  |
| `experiment_type` | `character varying(50)` | 是 |  |
| `grade_level` | `character varying(20)` | 是 |  |
| `subject` | `character varying(50)` | 是 |  |
| `total_videos` | `integer` | 是 | `0` |
| `completed_videos` | `integer` | 是 | `0` |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `metadata` | `jsonb` | 是 |  |

### 9.9 `tasks`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `project_id` | `uuid` | 否 |  |
| `name` | `character varying(100)` | 否 |  |
| `description` | `text` | 是 |  |
| `rubric_id` | `uuid` | 否 |  |
| `creator_id` | `uuid` | 是 |  |
| `status` | `character varying(20)` | 是 | `'draft'::character varying` |
| `start_date` | `timestamp without time zone` | 是 |  |
| `deadline` | `timestamp without time zone` | 是 |  |
| `end_date` | `timestamp without time zone` | 是 |  |
| `total_videos` | `integer` | 是 | `0` |
| `completed_videos` | `integer` | 是 | `0` |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `metadata` | `jsonb` | 是 |  |

### 9.10 `videos`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `student_id` | `uuid` | 是 |  |
| `student_name` | `character varying(50)` | 是 |  |
| `student_number` | `character varying(50)` | 是 |  |
| `filename` | `character varying(255)` | 否 |  |
| `original_filename` | `character varying(255)` | 是 |  |
| `file_size` | `bigint` | 是 |  |
| `duration` | `integer` | 是 |  |
| `resolution` | `character varying(20)` | 是 |  |
| `format` | `character varying(20)` | 是 |  |
| `storage_path` | `character varying(500)` | 否 |  |
| `thumbnail_url` | `character varying(500)` | 是 |  |
| `status` | `character varying(20)` | 是 | `'uploaded'::character varying` |
| `transcode_status` | `character varying(20)` | 是 |  |
| `upload_progress` | `integer` | 是 | `0` |
| `uploaded_at` | `timestamp without time zone` | 是 |  |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `metadata` | `jsonb` | 是 |  |
| `task_id` | `uuid` | 否 |  |
| `scorer_id` | `uuid` | 是 |  |
| `evaluation_status` | `character varying(20)` | 否 | `'pending'::character varying` |
| `ai_status` | `character varying(20)` | 否 | `'pending'::character varying` |
| `manual_status` | `character varying(20)` | 否 | `'pending'::character varying` |
| `ai_score` | `numeric(5,2)` | 是 |  |
| `manual_score` | `numeric(5,2)` | 是 |  |
| `assigned_at` | `timestamp without time zone` | 是 |  |
| `completed_at` | `timestamp without time zone` | 是 |  |

### 9.11 `ai_evaluations`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `task_id` | `uuid` | 否 |  |
| `video_id` | `uuid` | 否 |  |
| `model_version` | `character varying(50)` | 是 |  |
| `total_score` | `numeric(5,2)` | 是 |  |
| `status` | `character varying(20)` | 是 | `'processing'::character varying` |
| `started_at` | `timestamp without time zone` | 是 |  |
| `completed_at` | `timestamp without time zone` | 是 |  |
| `error_message` | `text` | 是 |  |
| `result_data` | `jsonb` | 是 |  |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |

### 9.12 `manual_evaluations`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `task_id` | `uuid` | 否 |  |
| `video_id` | `uuid` | 否 |  |
| `scorer_id` | `uuid` | 否 |  |
| `rubric_id` | `uuid` | 是 |  |
| `total_score` | `numeric(5,2)` | 是 |  |
| `score_details` | `jsonb` | 是 |  |
| `comments` | `text` | 是 |  |
| `status` | `character varying(20)` | 是 | `'in_progress'::character varying` |
| `started_at` | `timestamp without time zone` | 是 |  |
| `submitted_at` | `timestamp without time zone` | 是 |  |
| `time_spent` | `integer` | 是 |  |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
| `updated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |

### 9.13 `notifications`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `user_id` | `uuid` | 否 |  |
| `type` | `character varying(50)` | 否 |  |
| `title` | `character varying(200)` | 否 |  |
| `content` | `text` | 是 |  |
| `link_url` | `character varying(500)` | 是 |  |
| `is_read` | `boolean` | 是 | `false` |
| `read_at` | `timestamp without time zone` | 是 |  |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |

### 9.14 `audit_logs`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `user_id` | `uuid` | 是 |  |
| `action` | `character varying(100)` | 否 |  |
| `resource_type` | `character varying(50)` | 是 |  |
| `resource_id` | `uuid` | 是 |  |
| `ip_address` | `character varying(45)` | 是 |  |
| `user_agent` | `text` | 是 |  |
| `request_method` | `character varying(10)` | 是 |  |
| `request_path` | `character varying(500)` | 是 |  |
| `request_body` | `jsonb` | 是 |  |
| `response_status` | `integer` | 是 |  |
| `error_message` | `text` | 是 |  |
| `created_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |

### 9.15 `project_statistics`

| 字段 | 类型 | 可空 | 默认值 |
| --- | --- | --- | --- |
| `id` | `uuid` | 否 | `gen_random_uuid()` |
| `project_id` | `uuid` | 否 |  |
| `total_videos` | `integer` | 是 | `0` |
| `completed_videos` | `integer` | 是 | `0` |
| `pending_videos` | `integer` | 是 | `0` |
| `avg_ai_score` | `numeric(5,2)` | 是 |  |
| `avg_manual_score` | `numeric(5,2)` | 是 |  |
| `avg_score_difference` | `numeric(5,2)` | 是 |  |
| `completion_rate` | `numeric(5,2)` | 是 |  |
| `statistics_data` | `jsonb` | 是 |  |
| `calculated_at` | `timestamp without time zone` | 是 | `CURRENT_TIMESTAMP` |
