SkillJudge 数据库设计文档
一、数据库选型说明
- 主数据库: PostgreSQL 15+ (关系型数据，支持 JSONB)
- 文档库: MongoDB (AI 评测结果等 JSON 数据)
- 缓存: Redis (Session、Token、热点数据)
- 对象存储: OSS (视频、文件)

---
二、PostgreSQL 表设计
2.1 用户相关表
users (用户表)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(100),
    phone VARCHAR(20),
    real_name VARCHAR(50),
    avatar_url VARCHAR(500),
    role VARCHAR(20) NOT NULL,  -- 'admin', 'school_admin', 'teacher', 'scorer', 'student'
    status VARCHAR(20) DEFAULT 'active',  -- 'active', 'inactive', 'banned'
    school_id UUID REFERENCES schools(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP,
    metadata JSONB,  -- 扩展字段

    INDEX idx_username (username),
    INDEX idx_email (email),
    INDEX idx_school_id (school_id),
    INDEX idx_role (role),
    INDEX idx_status (status)
);

COMMENT ON TABLE users IS '用户表';
COMMENT ON COLUMN users.role IS '角色: admin-系统管理员, school_admin-学校管理员, teacher-教师, scorer-评分员, student-学生';
COMMENT ON COLUMN users.metadata IS '扩展字段，存储其他自定义信息';
schools (学校表)
CREATE TABLE schools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) UNIQUE,  -- 学校代码
    province VARCHAR(50),
    city VARCHAR(50),
    district VARCHAR(50),
    address VARCHAR(255),
    contact_person VARCHAR(50),
    contact_phone VARCHAR(20),
    contact_email VARCHAR(100),
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB,

    INDEX idx_code (code),
    INDEX idx_status (status)
);

COMMENT ON TABLE schools IS '学校表';
roles (角色表 - RBAC)
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL,
    code VARCHAR(50) UNIQUE NOT NULL,  -- 'teacher', 'scorer', etc.
    description TEXT,
    permissions JSONB,  -- 权限列表
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE roles IS '角色表';
COMMENT ON COLUMN roles.permissions IS '权限列表: ["project:create", "project:read", ...]';
user_roles (用户角色关联表)
CREATE TABLE user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(user_id, role_id),
    INDEX idx_user_id (user_id),
    INDEX idx_role_id (role_id)
);

---
2.2 项目相关表
projects (项目表)
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    school_id UUID REFERENCES schools(id),
    creator_id UUID REFERENCES users(id),  -- 创建者（教师）
    rubric_id UUID REFERENCES scoring_rubrics(id),  -- 评分细则
    status VARCHAR(20) DEFAULT 'draft',  -- 'draft', 'in_progress', 'completed', 'archived'
    deadline TIMESTAMP,
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    tags VARCHAR(255)[],  -- 标签数组
    experiment_type VARCHAR(50),  -- 实验类型
    grade_level VARCHAR(20),  -- 年级
    subject VARCHAR(50),  -- 学科
    total_videos INT DEFAULT 0,
    completed_videos INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB,

    INDEX idx_school_id (school_id),
    INDEX idx_creator_id (creator_id),
    INDEX idx_status (status),
    INDEX idx_deadline (deadline)
);

COMMENT ON TABLE projects IS '评测项目表';
COMMENT ON COLUMN projects.status IS '项目状态: draft-草稿, in_progress-进行中, completed-已完成, archived-已归档';
scoring_rubrics (评分细则表)
CREATE TABLE scoring_rubrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    total_score INT NOT NULL DEFAULT 100,
    template_type VARCHAR(50),  -- 模板类型
    school_id UUID REFERENCES schools(id),
    creator_id UUID REFERENCES users(id),
    is_template BOOLEAN DEFAULT false,  -- 是否为模板
    is_public BOOLEAN DEFAULT false,    -- 是否公开
    items JSONB NOT NULL,  -- 评分项数据（一、二级指标）
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_school_id (school_id),
    INDEX idx_creator_id (creator_id),
    INDEX idx_is_template (is_template)
);

COMMENT ON TABLE scoring_rubrics IS '评分细则表';
COMMENT ON COLUMN scoring_rubrics.items IS '评分项 JSON 数据，包含一、二级指标及配分';

-- items JSONB 结构示例：
/*
[
  {
    "id": "1",
    "name": "实验准备",
    "score": 10,
    "subItems": [
      {
        "id": "1-1",
        "requirement": "检查实验器材",
        "score": 5,
        "fullScoreStandard": "全部检查完毕",
        "deductionItems": "每缺一项扣1分",
        "dangerousOperation": "未戴护目镜"
      }
    ]
  }
]
*/
videos (视频表)
CREATE TABLE videos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    student_id UUID REFERENCES users(id),  -- 关联学生
    student_name VARCHAR(50),  -- 冗余字段，便于查询
    student_number VARCHAR(50),  -- 学号
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(255),
    file_size BIGINT,  -- 文件大小（字节）
    duration INT,  -- 视频时长（秒）
    resolution VARCHAR(20),  -- 分辨率 1920x1080
    format VARCHAR(20),  -- 格式 mp4
    storage_path VARCHAR(500) NOT NULL,  -- 存储路径
    thumbnail_url VARCHAR(500),  -- 缩略图
    status VARCHAR(20) DEFAULT 'uploaded',  -- 'uploading', 'uploaded', 'transcoding', 'ready', 'failed'
    transcode_status VARCHAR(20),  -- 转码状态
    upload_progress INT DEFAULT 0,
    uploaded_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB,

    INDEX idx_project_id (project_id),
    INDEX idx_student_id (student_id),
    INDEX idx_status (status),
    INDEX idx_student_number (student_number)
);

COMMENT ON TABLE videos IS '视频表';
COMMENT ON COLUMN videos.status IS '视频状态: uploading-上传中, uploaded-已上传, transcoding-转码中, ready-就绪, failed-失败';
video_tasks (视频评测任务表)
CREATE TABLE video_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    scorer_id UUID REFERENCES users(id),  -- 分配的评分员
    rubric_id UUID REFERENCES scoring_rubrics(id),
    status VARCHAR(20) DEFAULT 'pending',  -- 'pending', 'in_progress', 'completed', 'skipped'
    ai_status VARCHAR(20) DEFAULT 'pending',  -- AI 评测状态
    manual_status VARCHAR(20) DEFAULT 'pending',  -- 人工评测状态
    ai_score DECIMAL(5,2),
    manual_score DECIMAL(5,2),
    assigned_at TIMESTAMP,
    completed_at TIMESTAMP,
    deadline TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_project_id (project_id),
    INDEX idx_video_id (video_id),
    INDEX idx_scorer_id (scorer_id),
    INDEX idx_status (status),
    INDEX idx_ai_status (ai_status),
    INDEX idx_manual_status (manual_status)
);

COMMENT ON TABLE video_tasks IS '视频评测任务表';

---
2.3 评分相关表
ai_evaluations (AI 评测结果表)
CREATE TABLE ai_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES video_tasks(id) ON DELETE CASCADE,
    video_id UUID NOT NULL REFERENCES videos(id),
    model_version VARCHAR(50),  -- 模型版本
    total_score DECIMAL(5,2),
    status VARCHAR(20) DEFAULT 'processing',  -- 'processing', 'completed', 'failed'
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT,
    result_data JSONB,  -- AI 返回的完整结果 (videostage, videopoint, report)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_task_id (task_id),
    INDEX idx_video_id (video_id),
    INDEX idx_status (status)
);

COMMENT ON TABLE ai_evaluations IS 'AI 评测结果表';
COMMENT ON COLUMN ai_evaluations.result_data IS 'AI 返回的完整 JSON 数据';

-- result_data JSONB 结构示例（参考 PRD）：
/*
{
  "videostage": [...],
  "videopoint": [...],
  "report": {
    "overallDescription": "...",
    "score": 69,
    "details": [...]
  }
}
*/
manual_evaluations (人工评测结果表)
CREATE TABLE manual_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES video_tasks(id) ON DELETE CASCADE,
    video_id UUID NOT NULL REFERENCES videos(id),
    scorer_id UUID NOT NULL REFERENCES users(id),
    rubric_id UUID REFERENCES scoring_rubrics(id),
    total_score DECIMAL(5,2),
    score_details JSONB,  -- 详细评分数据
    comments TEXT,  -- 评语
    status VARCHAR(20) DEFAULT 'in_progress',  -- 'in_progress', 'submitted'
    started_at TIMESTAMP,
    submitted_at TIMESTAMP,
    time_spent INT,  -- 评分耗时（秒）
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_task_id (task_id),
    INDEX idx_video_id (video_id),
    INDEX idx_scorer_id (scorer_id),
    INDEX idx_status (status)
);

COMMENT ON TABLE manual_evaluations IS '人工评测结果表';
COMMENT ON COLUMN manual_evaluations.score_details IS '每个评分项的得分详情';

-- score_details JSONB 结构示例：
/*
[
  {
    "item_id": "1-1",
    "score": 5,
    "deduction": 0,
    "comment": "操作规范"
  }
]
*/
evaluation_comparisons (评分对比表)
CREATE TABLE evaluation_comparisons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES video_tasks(id) ON DELETE CASCADE,
    video_id UUID NOT NULL REFERENCES videos(id),
    ai_evaluation_id UUID REFERENCES ai_evaluations(id),
    manual_evaluation_id UUID REFERENCES manual_evaluations(id),
    ai_score DECIMAL(5,2),
    manual_score DECIMAL(5,2),
    score_difference DECIMAL(5,2),  -- 分差
    item_differences JSONB,  -- 各项分差
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_task_id (task_id),
    INDEX idx_video_id (video_id)
);

COMMENT ON TABLE evaluation_comparisons IS '评分对比表';

---
2.4 通知与日志表
notifications (通知表)
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,  -- 'task_assigned', 'task_deadline', 'evaluation_completed', etc.
    title VARCHAR(200) NOT NULL,
    content TEXT,
    link_url VARCHAR(500),
    is_read BOOLEAN DEFAULT false,
    read_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_user_id (user_id),
    INDEX idx_is_read (is_read),
    INDEX idx_created_at (created_at)
);

COMMENT ON TABLE notifications IS '通知表';
audit_logs (审计日志表)
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,  -- 'user.login', 'project.create', etc.
    resource_type VARCHAR(50),
    resource_id UUID,
    ip_address VARCHAR(45),
    user_agent TEXT,
    request_method VARCHAR(10),
    request_path VARCHAR(500),
    request_body JSONB,
    response_status INT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_user_id (user_id),
    INDEX idx_action (action),
    INDEX idx_resource_type (resource_type),
    INDEX idx_created_at (created_at)
);

COMMENT ON TABLE audit_logs IS '审计日志表';

---
2.5 统计分析表
project_statistics (项目统计表)
CREATE TABLE project_statistics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    total_videos INT DEFAULT 0,
    completed_videos INT DEFAULT 0,
    pending_videos INT DEFAULT 0,
    avg_ai_score DECIMAL(5,2),
    avg_manual_score DECIMAL(5,2),
    avg_score_difference DECIMAL(5,2),
    completion_rate DECIMAL(5,2),  -- 完成率百分比
    statistics_data JSONB,  -- 详细统计数据
    calculated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_project_id (project_id)
);

COMMENT ON TABLE project_statistics IS '项目统计表';

---
三、MongoDB 集合设计
3.1 ai_analysis_results (AI 分析结果集合)
{
  _id: ObjectId,
  videoId: UUID,  // 关联 PostgreSQL videos.id
  taskId: UUID,   // 关联 video_tasks.id
  projectId: UUID,
  modelVersion: String,
  analysisTimestamp: ISODate,

  // 视频阶段分解
  videostage: [
    {
      name: String,          // "作业准备"
      color: String,         // "purple"
      start_time: String,    // "00:00:00"
      end_time: String       // "00:00:30"
    }
  ],

  // 错误操作标注
  videopoint: [
    {
      name: String,          // "未锁止工具车及台架"
      type: String,          // "general error" / "serious error"
      start_time: String,
      end_time: String
    }
  ],

  // 评分报告
  report: {
    overallDescription: String,
    score: Number,
    details: [
      {
        title: String,       // "一、作业准备"
        label: String,       // "扣分" / "满分" / "缺失"
        labelColor: String,  // "purple" / "blue" / "red"
        subscore: Number,
        subDetails: [
          {
            subtitle: String,      // "个人防护"
            subsubscore: Number,
            AIscore: Number,
            status: String,        // "correct" / "incorrect"
            feedback: String
          }
        ]
      }
    ]
  },

  // 原始返回数据（备份）
  rawData: Object,

  createdAt: ISODate,
  updatedAt: ISODate
}
3.2 system_logs (系统日志集合)
{
  _id: ObjectId,
  level: String,        // "info", "warn", "error"
  service: String,      // "user-service", "video-service"
  message: String,
  error: Object,
  context: Object,
  timestamp: ISODate
}

---
四、Redis 数据结构
4.1 缓存 Key 设计
# Session
session:{userId}:{token}  -> JSON  (TTL: 7天)

# 用户信息缓存
user:{userId}  -> JSON  (TTL: 1小时)

# 权限缓存
user:permissions:{userId}  -> JSON  (TTL: 30分钟)

# 评分细则缓存
rubric:{rubricId}  -> JSON  (TTL: 1小时)

# 项目缓存
project:{projectId}  -> JSON  (TTL: 10分钟)

# 视频上传进度
video:upload:progress:{videoId}  -> INT  (TTL: 24小时)

# AI 任务队列
queue:ai:tasks  -> List

# 视频转码队列
queue:video:transcode  -> List

# 在线用户
online:users  -> Set

# 通知队列
queue:notifications:{userId}  -> List

# 统计数据缓存
stats:project:{projectId}  -> JSON  (TTL: 5分钟)

# API 限流
ratelimit:{userId}:{endpoint}  -> INT  (TTL: 1分钟)

---
五、数据关系图
schools (学校)
  ├── users (用户)
  │     ├── projects (项目 - 作为创建者)
  │     ├── video_tasks (评测任务 - 作为评分员)
  │     └── manual_evaluations (人工评测)
  │
  └── projects (项目 - 所属学校)
        ├── scoring_rubrics (评分细则)
        ├── videos (视频)
        │     └── video_tasks (评测任务)
        │           ├── ai_evaluations (AI 评测)
        │           ├── manual_evaluations (人工评测)
        │           └── evaluation_comparisons (评分对比)
        └── project_statistics (项目统计)

---
六、索引优化建议
6.1 高频查询场景
查询场景
优化方案
按学校查询项目
projects.school_id 索引
按教师查询项目
projects.creator_id 索引
按项目查询视频
videos.project_id 索引
按评分员查询任务
video_tasks.scorer_id + status 复合索引
按学生查询成绩
videos.student_id 或 student_number 索引
评测状态查询
video_tasks.status, ai_status, manual_status 索引
时间范围查询
created_at, deadline 索引
6.2 复合索引建议
-- 评分员待评测任务查询
CREATE INDEX idx_video_tasks_scorer_status
ON video_tasks(scorer_id, status, created_at DESC);

-- 项目视频列表查询
CREATE INDEX idx_videos_project_status
ON videos(project_id, status, uploaded_at DESC);

-- 学生成绩查询
CREATE INDEX idx_videos_student
ON videos(student_id, project_id);

-- 审计日志查询
CREATE INDEX idx_audit_logs_user_action
ON audit_logs(user_id, action, created_at DESC);

---
七、数据迁移与版本管理
7.1 数据库迁移工具
推荐: Prisma Migrate / TypeORM Migrations / Flyway
迁移文件命名:
migrations/
├── 001_init_users_and_schools.sql
├── 002_create_projects.sql
├── 003_create_videos_and_tasks.sql
└── 004_create_evaluations.sql
7.2 版本管理策略
- 每个功能分支对应一个迁移文件
- 迁移文件只增不改（避免冲突）
- 生产环境迁移前备份数据库
- 使用事务保证迁移原子性

---
八、数据备份与恢复
8.1 备份策略
数据类型
备份频率
保留周期
方案
PostgreSQL
每日全量 + 实时 WAL
30天
pg_dump + WAL 归档
MongoDB
每日全量
30天
mongodump / Cloud Backup
Redis
每日 RDB + AOF
7天
RDB snapshot + AOF
对象存储
跨区域复制
永久
MinIO 镜像 / OSS 跨区域复制
8.2 恢复预案
- RTO (恢复时间目标): 1小时内
- RPO (恢复点目标): 最多丢失1小时数据
- 定期演练恢复流程

---
九、数据安全与合规
9.1 敏感数据处理
数据类型
处理方式
密码
bcrypt 加密存储 (rounds=10)
手机号
AES 加密存储
邮箱
部分脱敏展示 (展示前3位和@后)
身份证
AES 加密 + 脱敏展示
视频内容
访问权限控制 + 临时签名 URL
9.2 数据访问控制
- 数据库账号最小权限原则
- 应用层行级数据权限控制
- 审计所有敏感操作
- 定期审查权限配置
9.3 合规要求
- GDPR/个人信息保护法: 支持数据导出和删除
- 等保三级: 审计日志、权限分离、加密传输
- 数据留存: 评测数据保留至少3年

---
十、性能优化建议
10.1 查询优化
-- 使用 EXPLAIN ANALYZE 分析慢查询
EXPLAIN ANALYZE
SELECT * FROM video_tasks
WHERE scorer_id = '...' AND status = 'pending';

-- 使用物化视图加速复杂统计
CREATE MATERIALIZED VIEW project_stats_mv AS
SELECT
    project_id,
    COUNT(*) as total_videos,
    AVG(ai_score) as avg_ai_score,
    AVG(manual_score) as avg_manual_score
FROM video_tasks
GROUP BY project_id;

-- 定期刷新物化视图
REFRESH MATERIALIZED VIEW CONCURRENTLY project_stats_mv;
10.2 分区表设计
-- 按时间分区（月分区）
CREATE TABLE audit_logs (
    id UUID,
    created_at TIMESTAMP,
    ...
) PARTITION BY RANGE (created_at);

CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs
FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
10.3 读写分离
- 主库: 写操作 + 实时性要求高的读操作
- 从库: 统计查询、报表生成、数据导出
- 中间件: PgPool-II / ProxySQL

---
十一、数据字典生成
建议使用工具自动生成数据字典文档：
- SchemaSpy: 可视化 ER 图
- dbdocs.io: 在线数据库文档
- Prisma Studio: 可视化数据库管理

---
总结
本数据库设计：
1. ✅ 完整性: 覆盖所有业务需求
2. ✅ 扩展性: JSONB 字段支持灵活扩展
3. ✅ 性能: 合理的索引设计
4. ✅ 安全性: 数据加密、访问控制
5. ✅ 可维护性: 清晰的表结构和注释
6. ✅ 多存储: PostgreSQL + MongoDB + Redis 组合
下一步可以：
- 根据此设计生成 Prisma Schema
- 编写初始化 SQL 脚本
- 设计 API 接口