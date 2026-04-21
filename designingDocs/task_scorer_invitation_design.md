# 教师端 Task 评分员邀请与绑定设计草案

## 文档目的

本文档用于整理并细化教师端以下需求的后端与前端实现方案：

1. 在创建 `task` 的最后一步增加“邀请评分员”能力。
2. 教师输入评分员姓名、邮箱后，系统自动复用或创建评分员账号。
3. 系统仅在首次创建评分员账号时发送开通邮件；本校已存在评分员改为系统内通知与确认。
4. `task` 创建完成后，仍支持继续添加、补发邀请、移除或替换评分员。

本文档重点描述：

- 业务边界
- 数据模型
- 接口设计
- 核心流程
- 状态语义
- 前端改造建议
- 实施顺序

本文档不直接给出代码实现，但应足够细化到可以继续拆分开发任务。

## 当前实现更新（2026-04-19）

在原草案基础上，当前实现已进一步收敛为两段式流程：

1. 学校级评分员预创建
   - 新建任务第 4 步不再维护“待邀请列表”
   - 教师可在评分员下拉框旁点击“邀请新评分员”弹窗
   - 弹窗提交后立即调用学校级 `provision` 接口
   - 若邮箱不存在：创建评分员账号并发送开通邮件
   - 若邮箱已存在且属于本校评分员：直接复用账号
   - 无论新旧账号，成功后都应立即进入当前学校的 `scorer pool`

2. Task 级确认通知
   - `task` 创建成功后，系统按本次实际参与预分配的评分员创建 `task_scorers` 与站内待确认通知
   - 已预分配但未确认的评分员，不出现在评分员待评分列表中
   - 评分员确认后，预分配才转为可执行的正式评分任务

3. 评分员任务列表约束
   - 评分员端 `GET /api/v1/tasks/my`
   - 单任务详情 `GET /api/v1/tasks/:id`
   - 实际提交 `POST /api/v1/tasks/:id/submit`
   以上链路都必须受 `task_scorers.status = accepted` 约束，防止预分配任务提前暴露

## 当前实现更新（2026-04-20）

在原有邀请与确认链路之上，本轮又新增了“任务列表页分配详情弹窗 + 保守再分配”能力：

1. 入口位置调整
   - 不再把评分员管理作为 `task` 详情页的主入口
   - 教师端改为在 `task` 列表页操作区提供：
     - `查看评分详情`
     - `查看成绩汇总`
     - `评分员分配详情`
   - 第三个入口点击后打开大弹窗，集中展示评分员情况、待调整视频和再分配组件

2. 再分配边界
   - 当前只允许 `manual_status = pending` 的视频参与重新分配
   - `in_progress` / `submitted` / `evaluation_status = completed` 一律不允许调整
   - 这样可以避免覆盖评分员已开始或已完成的工作

3. 再分配方式
   - `平均分配`
     - 多个目标评分员
     - 系统随机打散视频后平均切分
     - 余数按“前几个评分员多 1 条”处理，保证不重不漏
   - `指定数量随机分配`
     - 教师只填写“每位评分员需要接收多少条”
     - 后端校验数量之和必须等于已选视频总数
     - 系统随机打散后按数量切片分配

4. 替换待确认评分员
   - 若原评分员仍为 `task_scorers.status = pending`
   - 且其名下视频在本次操作后全部被移走
   - 则原 invitation 改为 `cancelled`
   - 原 `task_scorers` 关系改为 `inactive`

5. 再分配后的通知规则
   - 目标评分员若已是当前 task 的 `accepted` 评分员：
     - 不再重复确认
   - 目标评分员若尚未加入 task 或仍为 `pending`：
     - 自动创建或刷新 task 关系
     - 自动生成新的站内待确认通知

## 需求摘要

用户给出的约束如下：

1. 评分员归属在学校下面，同个学校的全部评分员都可以添加邀请。
2. 增删项目评分员指的是调整项目与评分员的依赖关系，而不是做账号层面的增删。
3. 每次新建项目都有一个邀请，让评分员确认，但是账号只需要创建一次。
4. 已存在评分员不再重复发送外链邀请邮件，而是在系统内收到待确认任务通知。

结合当前系统语义，本文档统一按 `task` 维度设计，不在 `project` 维度保存评分员关系。

## 现状与差距

### 当前已存在能力

当前系统已经具备以下基础能力：

- `users / roles / user_roles` 权限链
- `schools -> projects -> tasks -> videos` 主业务链
- 按学校筛选可分配评分员
- 按 `video.scorer_id` 执行视频到评分员的实际分配

### 当前实现的核心限制

当前真实后端里，评分员相关能力主要仍是“视频分配”，不是“任务级评分员关系管理”。

现有问题：

1. 当前没有 `task -> scorer` 的独立关系表。
2. 当前没有评分员邀请记录表。
3. 当前没有邮件发送能力。
4. 当前前端向导中存在 `/api/v1/wizard/owners/.../reviewer-assignments` 这样的接口调用，但真实后端并没有对应正式接口。
5. 当前 `users.email` 没有唯一约束，只能按 `username` 保证唯一，无法可靠支持“按邮箱复用账号”。

因此，本需求不能简单复用现有 `POST /api/v1/tasks/:id/assignments`。

### 为什么不能直接复用现有视频分配接口

`POST /api/v1/tasks/:id/assignments` 当前语义是：

- 指定某个 `task`
- 将一批 `videoIds`
- 分配给一批 `scorerIds`

它解决的是“谁来评哪些视频”，而不是：

- 这个 `task` 当前有哪些评分员
- 谁被邀请过
- 谁确认了
- 谁被移除但账号仍保留

因此本轮需要把“任务级评分员绑定”和“视频实际分配”拆开。

## 设计目标

本轮目标如下：

1. 支持在 `task` 创建最后一步邀请评分员。
2. 支持在 `task` 创建完成后继续维护评分员关系。
3. 评分员账号按邮箱复用，不重复创建。
4. 每次邀请都留下独立邀请记录。
5. 评分员仅限当前 `task` 所属学校。
6. 评分员关系调整不影响历史账号与历史评分数据。

## 非目标

本轮不包含以下内容：

- 不改造为邮箱登录
- 不做双评、多评、复评、仲裁
- 不做组织级跨校评分员共享
- 不做学校管理员之外的更复杂审批流
- 不自动在邀请完成后立即重分配历史已提交评分的视频

## 核心业务语义

本轮需要明确区分 3 件事情：

### 1. 评分员账号

表示一个 `user`，角色为 `scorer`，归属某个学校。

特点：

- 账号只创建一次
- 按邮箱识别和复用
- 账号存在并不代表参与某个 `task`

### 2. Task 与评分员关系

表示某个评分员当前是否属于某个 `task`。

特点：

- 是 `task` 级关系，不是账号本身
- 可添加、停用、恢复
- 不等于具体视频已分配

### 3. 邀请 / 通知记录

表示一次“通知评分员确认参与某个 task”的动作。

特点：

- 每次通知都产生一条新记录
- 账号可以复用，但邀请记录不能复用
- 可用于站内待确认列表、补发通知、追踪是否确认

## 数据模型设计

## 邀请渠道拆分

本需求需要明确区分两类触达渠道：

### 1. 新账号开通邮件

触发条件：

- 按邮箱未找到现有评分员账号
- 系统需要自动创建 `scorer` 账号

行为：

- 创建账号
- 生成临时密码
- 发送邮件
- 邮件中包含账号信息、临时密码与确认链接

### 2. 系统内任务确认通知

触发条件：

- 评分员账号在本校已存在
- 或新账号已经创建并登录系统后，需要查看当前待确认任务

行为：

- 写入一条任务确认通知记录
- 评分员登录后若存在未确认任务，优先弹窗提醒
- 评分员可在消息中心查看通知列表并确认

## 1. `users.email` 约束调整

### 目标

支持“按邮箱查找是否已有账号”。

### 建议

1. 查询邮箱时统一做规范化：
   - `trim`
   - `lower`
2. 为规范化后的邮箱增加唯一约束。

### 建议规则

- `NULL` 邮箱允许存在
- 非空邮箱必须唯一
- 业务代码中禁止按原值直接比较邮箱

### 兼容性说明

若历史库中已有重复邮箱数据，迁移前需要先做一次数据清理。

## 2. 新表 `task_scorers`

### 用途

保存“某个 task 当前有哪些评分员”的稳定关系。

### 建议字段

```sql
create table task_scorers (
  id uuid primary key default gen_random_uuid(),
  task_id uuid not null references tasks(id),
  scorer_id uuid not null references users(id),
  status varchar(20) not null default 'pending',
  invited_by uuid null references users(id),
  invited_at timestamptz null,
  accepted_at timestamptz null,
  removed_at timestamptz null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  metadata jsonb not null default '{}'::jsonb,
  unique(task_id, scorer_id)
);

create index idx_task_scorers_task_id on task_scorers(task_id);
create index idx_task_scorers_scorer_id on task_scorers(scorer_id);
```

### 状态建议

- `pending`
  - 已关联到任务，但尚未确认
- `accepted`
  - 已确认参与当前任务
- `declined`
  - 明确拒绝参与当前任务
- `inactive`
  - 教师已将其从当前任务中移除

### 设计说明

- `(task_id, scorer_id)` 唯一，确保同一任务下同一评分员只存在一条当前关系。
- “移除评分员”建议改状态，不直接物理删除。
- `metadata` 可预留给后续前端来源、备注、邀请渠道等扩展字段。

## 3. 新表 `task_scorer_invitations`

### 用途

保存每一次任务确认通知行为，满足“账号只创建一次，但每次 task 都要单独通知确认”的要求。

### 建议字段

```sql
create table task_scorer_invitations (
  id uuid primary key default gen_random_uuid(),
  task_id uuid not null references tasks(id),
  scorer_id uuid not null references users(id),
  email_snapshot varchar(100) not null,
  real_name_snapshot varchar(50) null,
  token_hash varchar(255) not null,
  status varchar(20) not null default 'sent',
  sent_at timestamptz not null default now(),
  expires_at timestamptz not null,
  responded_at timestamptz null,
  created_by uuid null references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  metadata jsonb not null default '{}'::jsonb
);

create index idx_task_scorer_invitations_task_id on task_scorer_invitations(task_id);
create index idx_task_scorer_invitations_scorer_id on task_scorer_invitations(scorer_id);
```

### 状态建议

- `sent`
  - 通知已生成
  - 对新账号可表示邮件已准备发送或已发送
  - 对已有账号可表示站内待确认通知已生成
- `accepted`
  - 评分员已点击邀请确认
- `expired`
  - 邀请已过期
- `cancelled`
  - 邀请已被取消或被后续操作废弃

### 设计说明

- `email_snapshot` 与 `real_name_snapshot` 保存通知发出时的快照。
- `token_hash` 不应直接保存明文 token；已有账号站内确认可直接按通知 ID 操作。
- 一次补发通知应新增一条记录，而不是覆盖旧记录。
- `metadata` 建议记录：
  - `deliveryChannel=email|in_app|email_and_in_app`
  - `mailStatus`
  - `inAppRead`
  - `inAppReadAt`

## 4. 与现有视频分配表的关系

当前系统的实际评分执行仍依赖：

- `videos.scorer_id`
- `manual_evaluations.scorer_id`

因此职责划分建议如下：

- `task_scorers`
  - 负责“这个 task 当前有哪些评分员”
- `task_scorer_invitations`
  - 负责“对这个 task 发过哪些邀请”
- 现有 `/tasks/:id/assignments`
  - 负责“哪些视频分给哪个评分员”

三者不应混用。

## 业务规则

## 1. 学校范围约束

评分员必须与 `task.project.school_id` 属于同一学校。

### 处理规则

1. 邀请时，根据 `task -> project -> school_id` 取学校范围。
2. 若邮箱对应账号不存在：
   - 自动创建 `role = scorer`
   - `school_id = task.project.school_id`
3. 若邮箱对应账号已存在：
   - 必须为 `scorer`
   - 必须与当前 `task` 同校
   - 否则返回错误

### 不允许的情况

- 已存在用户但属于别的学校
- 已存在用户但角色不是 `scorer`
- 已存在用户但状态不是 `active`

这些情况都不应被系统自动纠正，应直接返回业务错误，由人工处理。

## 2. 账号创建规则

### 输入

- `name`
- `email`

### 行为

后端以邮箱为主键语义：

1. 先按规范化邮箱查询用户
2. 不存在则自动创建账号
3. 存在则直接复用账号

### 创建新账号时

系统自动生成：

- `username`
- `temporary password`
- `role = scorer`
- `school_id = task school`

### 用户名生成建议

用户名需避免与现有账号冲突。建议规则：

```text
scorer_<schoolCodeOrShortId>_<random>
```

例如：

```text
scorer_s001_ab12cd
```

### 密码策略建议

- 随机密码长度不少于 12 位
- 包含大小写字母、数字
- 仅在首次创建账号时通过邮件发送一次
- 后续可补“首次登录强制修改密码”

## 3. 邀请规则

### 触发时机

- 新建 `task` 的最后一步
- `task` 创建后教师再次添加评分员
- 教师对已有评分员补发邀请

### 业务要求

- 每次邀请都创建新 invitation 记录
- 不覆盖历史 invitation
- 允许同一个评分员在同一个 task 上存在多条历史 invitation

### 建议过期时间

- 默认 7 天
- 过期后可补发

## 4. 移除规则

“移除评分员”不应删除用户账号，也不应删除历史评分记录。

建议行为：

1. 将 `task_scorers.status` 改为 `inactive`
2. 设置 `removed_at`
3. 保留历史 invitation
4. 已提交的 `manual_evaluations` 不回滚

### 视频分配处理建议

本轮先不自动强制回收已分配视频。

可接受的最小策略：

- 已提交评分的视频不动
- 未提交评分的视频由教师后续手动重新分配

后续如要做自动回收，可另起专题设计。

## 服务设计建议

建议新增独立模块，例如：

```text
backend/internal/modules/taskscorer/
```

而不是继续堆到现有 `scoring` 模块里。

原因：

- 当前 `scoring` 更偏“评测执行”
- 本轮新增的是“任务级评分员关系”和“邀请生命周期”
- 单独模块更清晰，也更利于后续扩展

## 核心服务方法草案

## 1. `InviteScorerToTask`

### 输入

- `taskId`
- `name`
- `email`
- `actor`

### 目标

1. 校验 actor 是否有权管理该 task
2. 查找或创建评分员账号
3. upsert `task_scorers`
4. 创建 invitation
5. 根据账号是否新建，分别走邮件或系统内通知

### 建议伪流程

```text
1. 加载 task，上卷 project 和 school
2. 校验 actor 对 task 的管理权限
3. 规范化 email
4. 按 email 查 user
5. 若 user 不存在，则创建 scorer 用户
6. 若 user 存在，则校验 role/status/school
7. upsert task_scorers
8. 创建 task_scorer_invitations
9. 若 `userCreated=true`，事务外发送开通邮件
10. 若 `userCreated=false`，仅保留系统内通知，等待评分员登录确认
```

### 事务边界建议

数据库写入与邮件发送分离：

- 数据写入在事务内
- 邮件发送仅对新账号在事务提交后执行

原因：

- 避免邮件失败导致 DB 回滚
- 便于后续补“重试发送”

## 2. `ListTaskScorers`

### 目标

返回当前 task 下的评分员关系，以及最近邀请状态。

### 返回内容建议

- `taskScorerId`
- `scorerId`
- `realName`
- `username`
- `email`
- `status`
- `invitedAt`
- `acceptedAt`
- `lastInvitationStatus`
- `lastInvitationSentAt`

## 3. `RemoveTaskScorer`

### 目标

将评分员从当前 task 关系中停用。

### 行为

- 校验权限
- 校验关系存在
- 将 `task_scorers.status = inactive`
- 设置 `removed_at`

## 4. `ResendTaskScorerInvitation`

### 目标

对已关联到 task 的评分员重新发起一次通知。

### 行为

- 校验权限
- 校验 `task_scorers` 关系存在且不是 `inactive`
- 新增一条通知记录
- 若该账号是首次系统建号且仍需开通凭证，则可继续发送邮件
- 否则仅生成新的站内通知

## 5. `AcceptTaskScorerInvitation`

### 目标

评分员通过邮件中的确认链接确认参与当前任务。

## 6. `ListMyTaskScorerNotifications`

### 目标

评分员查看自己的待确认任务通知，用于登录后弹窗与消息中心列表。

### 建议返回内容

- `invitationId`
- `taskId`
- `taskName`
- `projectId`
- `projectName`
- `status`
- `isRead`
- `sentAt`
- `deliveryChannel`

## 7. `AcceptMyTaskScorerNotification`

### 目标

评分员在系统内确认参与当前任务。

### 行为

- 校验该通知属于当前评分员
- 更新通知状态为 `accepted`
- 更新 `task_scorers.status = accepted`
- 标记通知为已读

### 行为

- 验证 token
- 读取 invitation
- 校验未过期、未失效
- 更新 invitation 状态为 `accepted`
- 更新 `task_scorers.status = accepted`
- 写入 `accepted_at`

## 接口设计草案

本轮建议使用真实 task 路径，不继续使用 `/wizard/...` 伪接口。

所有响应均沿用当前标准响应包裹结构：

```json
{
  "code": 200,
  "message": "success",
  "data": {},
  "timestamp": "2026-04-19T10:00:00Z"
}
```

## 1. 查询任务评分员列表

### `GET /api/v1/tasks/:taskId/scorers`

### 语义

查询某个 task 当前已绑定的评分员，以及最近邀请状态。

### response body 示例

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "taskScorerId": "uuid",
        "scorerId": "uuid",
        "username": "scorer_s001_ab12cd",
        "realName": "张老师",
        "email": "zhang@example.com",
        "status": "pending",
        "invitedAt": "2026-04-19T10:00:00Z",
        "acceptedAt": null,
        "lastInvitation": {
          "invitationId": "uuid",
          "status": "sent",
          "sentAt": "2026-04-19T10:00:00Z",
          "expiresAt": "2026-04-26T10:00:00Z"
        }
      }
    ]
  },
  "timestamp": "2026-04-19T10:00:00Z"
}
```

## 2. 邀请评分员加入任务

### `POST /api/v1/tasks/:taskId/scorers/invite`

### request body

```json
{
  "name": "张老师",
  "email": "zhang@example.com"
}
```

### 语义

- 如果邮箱对应账号不存在，则自动创建评分员账号
- 如果已存在，则直接复用
- 无论新老账号，都为当前 task 新建一次 invitation

### response body 示例

```json
{
  "code": 201,
  "message": "success",
  "data": {
    "scorerId": "uuid",
    "userCreated": true,
    "taskRelationCreated": true,
    "taskRelationStatus": "pending",
    "invitationId": "uuid",
    "invitationStatus": "sent"
  },
  "timestamp": "2026-04-19T10:00:00Z"
}
```

## 3. 补发邀请

### `POST /api/v1/tasks/:taskId/scorers/:scorerId/resend-invite`

### request body

```json
{}
```

### response body 示例

```json
{
  "code": 201,
  "message": "success",
  "data": {
    "invitationId": "uuid",
    "invitationStatus": "sent"
  },
  "timestamp": "2026-04-19T10:00:00Z"
}
```

## 4. 移除任务评分员

### `DELETE /api/v1/tasks/:taskId/scorers/:scorerId`

### 语义

仅移除任务关系，不删除账号。

### response body 示例

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "taskId": "uuid",
    "scorerId": "uuid",
    "status": "inactive"
  },
  "timestamp": "2026-04-19T10:00:00Z"
}
```

## 5. 接受邀请

### `POST /api/v1/task-scorer-invitations/:token/accept`

### 语义

评分员通过邮件链接确认参与当前任务。

### response body 示例

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "taskId": "uuid",
    "scorerId": "uuid",
    "taskRelationStatus": "accepted",
    "acceptedAt": "2026-04-19T10:10:00Z"
  },
  "timestamp": "2026-04-19T10:10:00Z"
}
```

## 邮件设计草案

## 1. 邮件能力

当前项目已新增正式邮件发送模块：

```text
backend/internal/platform/mail/
```

当前实现采用标准库直连 SMTP，已兼容 QQ 邮箱 SMTP 方案。

### 已接入配置项

- `MAIL_ENABLED`
- `MAIL_PROVIDER`
- `MAIL_SMTP_HOST`
- `MAIL_SMTP_PORT`
- `MAIL_SMTP_USERNAME`
- `MAIL_SMTP_PASSWORD`
- `MAIL_TIMEOUT`
- `MAIL_FROM_NAME`
- `MAIL_FROM_ADDRESS`
- `MAIL_INVITE_BASE_URL`

### 当前实现说明

1. `MAIL_ENABLED=false` 时，仅影响“新账号开通邮件”场景；已有账号的站内通知仍应可正常落库。
2. `MAIL_PROVIDER` 当前实现 `smtp`。
3. `MAIL_INVITE_BASE_URL` 应配置为前端可被评分员访问的公开地址，例如：

```text
http://your-domain.com
```

系统邮件中的确认链接会拼接为：

```text
{MAIL_INVITE_BASE_URL}/task-scorer-invitations/{token}/accept
```

为支持邮件中的点击确认，当前后端已补充：

- `GET /task-scorer-invitations/:token/accept`
  - 返回一个简易确认页面
- `POST /api/v1/task-scorer-invitations/:token/accept`
  - 实际执行 invitation 确认

## 2. 邮件内容建议

邮件中建议包含：

- 学校名称
- task 名称
- 评分员姓名
- 邀请确认链接
- 若首次创建账号，则附带：
  - `username`
  - `temporary password`

### 邮件内容区分

#### 首次创建账号时

邮件包含：

- 账号信息
- 临时密码
- 邀请确认链接

#### 已有账号再次邀请时

不再发送邮件。

已有账号通过系统内通知完成确认。

## 安全设计建议

## 1. token 存储

- 邮件链接里的 token 应为一次性随机串
- 数据库存储 `token_hash`
- 校验时比对 hash

## 2. 密码处理

- 随机密码只在首次创建账号时发一次
- 数据库存储 bcrypt hash
- 不在日志中打印明文密码

## 3. 邮箱查询

- 所有邮箱比较必须基于规范化结果
- 严禁大小写敏感比较

## 4. 过期策略

- invitation 默认 7 天过期
- 过期 invitation 不能继续接受
- 可补发新的 invitation

## 前端改造建议

## 1. 新建 Task 第 4 步

当前第 4 步建议拆成两个区域：

### 区域 A：已有评分员

能力：

- 拉取本校评分员列表
- 支持选择已有评分员
- 点击加入当前 task 并发送系统内通知

### 区域 B：邀请新评分员

输入：

- 姓名
- 邮箱

动作：

- 点击后调用 `POST /api/v1/tasks/:taskId/scorers/invite`
- 若账号已存在，仅生成站内通知
- 若账号不存在，创建账号并发送开通邮件

### 页面展示建议

展示字段：

- 姓名
- 邮箱
- 状态
- 最近邀请时间
- 是否已确认

## 2. Task 详情页增加评分员管理面板

建议在 `task` 详情页补充“评分员管理”区域，支持：

- 查看当前任务评分员
- 添加已有评分员
- 邀请新评分员
- 再次通知
- 移除评分员关系

### 状态显示建议

- `pending`
  - 待确认
- `accepted`
  - 已确认
- `declined`
  - 已拒绝
- `inactive`
  - 已移除

## 3. 与视频分配页面的关系

本轮前端需要明确区分两种操作：

### 任务评分员管理

解决：

- 谁属于当前 task
- 谁收到了邀请
- 谁确认了

### 视频分配

解决：

- 哪些视频分配给哪个评分员

前端不应再把这两套语义混在同一组临时状态里。

## 错误码与异常场景建议

建议补充以下业务错误：

- `task scorer email required`
- `task scorer name required`
- `task scorer email invalid`
- `task scorer already in another school`
- `task scorer role invalid`
- `task scorer not found`
- `task scorer relation not found`
- `task scorer invitation expired`
- `task scorer invitation invalid`

典型场景：

1. 邮箱已存在，但用户属于别的学校
   - 返回 409 或 400，提示无法复用
2. 邮箱已存在，但角色不是评分员
   - 返回 409 或 400
3. 评分员已在 task 中
   - 关系可复用，但 invitation 仍可新增
4. 新账号邮件发送失败
   - 当前实现中，`task_scorers` 与 `task_scorer_invitations` 已写入，但接口返回发送失败
   - invitation metadata 会记录 `mailStatus` / `mailError`
   - 可通过补发接口继续重试
5. 已有账号通知确认
   - 不依赖邮件
   - 评分员登录后通过站内弹窗或消息中心确认

## 实施顺序建议

建议按以下顺序推进。

其中需要特别明确：

- 数据库表与约束已完成创建。
- 后端真实读写闭环已完成。
- 邮件能力已按 SMTP 方案接入。

## 当前落地状态

截至当前轮次，数据库侧已完成：

1. `users.email` 规范化唯一约束
2. `task_scorers`
3. `task_scorer_invitations`

因此下面的实施顺序从后端代码层继续推进。

## 第 1 步：后端模型与基础查询

1. 在 `internal/model/models.go` 中补充：
   - `TaskScorer`
   - `TaskScorerInvitation`
2. 在 `user.Repository` 中补：
   - `FindByEmail(ctx, email string)`
3. 统一邮箱规范化逻辑：
   - `trim`
   - `lower`

## 第 2 步：后端真实接口与落库闭环

建议新增独立模块：

```text
backend/internal/modules/taskscorer/
```

第一阶段先实现最小闭环：

1. `GET /api/v1/tasks/:id/scorers`
2. `POST /api/v1/tasks/:id/scorers/invite`
3. `DELETE /api/v1/tasks/:id/scorers/:scorerId`

这一阶段的目标是先打通以下链路：

1. 按邮箱查用户
2. 用户不存在则创建 `scorer`
3. upsert `task_scorers`
4. 写入 `task_scorer_invitations`
5. 先返回业务成功结果

当前已完成。

## 第 3 步：补齐 invitation 生命周期接口

在第 2 步稳定后，再补：

1. `POST /api/v1/tasks/:taskId/scorers/:scorerId/resend-invite`
2. `POST /api/v1/task-scorer-invitations/:token/accept`

当前已完成，并补充：

- `GET /task-scorer-invitations/:token/accept`

## 第 4 步：邮件能力

当前已完成第一版接入，落地方式如下：

1. 新增 `platform/mail`
2. 使用 SMTP 发信，优先兼容 QQ 邮箱 SMTP
3. `InviteScorerToTask`、`ResendTaskScorerInvitation` 已与邮件发送打通
4. 邮件中已包含确认链接、task 信息，以及首次建号时的账号密码

当前行为：

- 邮件发送失败不回滚已写入的 `task_scorers` 与 `task_scorer_invitations`
- invitation metadata 会记录邮件成功或失败状态
- 首次建号但发送失败时，用户 metadata 中 `credentialReady=false`
- 后续补发时，系统会重新生成临时密码并再次发送

后续实现调整目标：

- 邮件只用于“首次创建账号”的开通场景
- 已存在评分员统一走站内通知确认，不再发送新的外链邀请邮件
- 评分员端新增登录弹窗与消息中心列表

推荐配置示例（QQ 邮箱）：

```env
MAIL_ENABLED=true
MAIL_PROVIDER=smtp
MAIL_SMTP_HOST=smtp.qq.com
MAIL_SMTP_PORT=465
MAIL_SMTP_USERNAME=your_account@qq.com
MAIL_SMTP_PASSWORD=your_authorization_code
MAIL_FROM_NAME=SkillJudge
MAIL_FROM_ADDRESS=your_account@qq.com
MAIL_INVITE_BASE_URL=http://your-backend-domain.com
MAIL_TIMEOUT=10s
```

## 第 5 步：教师端新建 Task 页面

1. 改造第 4 步 UI
2. 接入真实 task scorer API
3. 去掉对 `/wizard/owners/.../reviewer-assignments` 的依赖
4. 将“任务评分员管理”和“视频分配”在前端显式拆开

这是当前最优先的下一步。

## 第 6 步：评分员端站内通知与确认

1. 新增评分员待确认任务列表接口
2. 新增评分员系统内确认接口
3. 登录后若存在待确认任务，则优先弹窗提醒
4. 支持从消息中心查看与确认

## 第 7 步：Task 详情页管理能力

1. 增加评分员列表
2. 增加邀请与再次通知
3. 增加移除关系

## 第 8 步：视频分配联动优化

本轮可选，不作为主链阻塞项：

1. 当新增评分员后，允许教师手动分配未提交视频
2. 后续再讨论是否自动重分配未提交视频

## 当前建议的立即执行项

基于数据库与邮件能力已经就位，当前最优先的下一步是：

1. 将 taskscorer 服务改为：
   - 新账号发邮件
   - 已有账号只写站内通知
2. 增加评分员端待确认通知列表 / 已读 / 系统内确认接口
3. 教师端文案改为“再次通知”
4. 评分员端补登录弹窗与消息中心

## 验收口径

满足以下条件可认为主需求达成：

1. 教师创建 task 时可以输入评分员姓名和邮箱完成邀请。
2. 若该邮箱之前已存在评分员账号，则不重复建号。
3. 若该邮箱不存在，则自动创建评分员账号并发送开通邮件。
4. 评分员只能归属当前 task 所属学校。
5. task 创建后教师仍可以继续添加、补发邀请、移除评分员关系。
6. 移除评分员不会删除用户账号。
7. 每次邀请都能保留独立 invitation 记录。
8. 本校已存在评分员登录后能在系统内看到待确认任务并完成确认。

## 总结

本方案的核心是把原先混在一起的三类概念拆开：

1. `评分员账号`
2. `task 与评分员关系`
3. `邀请行为记录`

这样可以同时满足以下目标：

- 账号只创建一次
- 每个 task 都能单独邀请
- task 创建后仍可持续维护评分员
- 不破坏现有视频分配与人工评分主链

后续如进入实现阶段，建议优先从“表结构 + 后端真实接口”落地，再推进前端改造。
