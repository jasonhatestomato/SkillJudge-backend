# 系统介绍
这是一个物理实验评分系统的后端部分，主要用于高中生进行物理实验时，通过物理设备采集视频，采集完成后上传视频以及对应的评分细则到系统，分配给对应评分员进行打分。后端主要基于Go语言的Gin框架。

# 系统架构
采用 前后端分离 + 微服务架构，分层设计：
┌─────────────────────────────────────────────────────────────┐
│                        前端层 (Frontend)                      │
│  教师端 Web / 评分员端 Web / 管理端 Web / 学生端 H5(可选)         │
└─────────────────────────────────────────────────────────────┘
                              ↓ HTTPS/WebSocket
┌─────────────────────────────────────────────────────────────┐
│                     API 网关层 (Gateway)                      │
│          Nginx / Kong / Traefik (负载均衡、鉴权、限流)          │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                     后端服务层 (Backend)                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │用户服务  │  │项目服务  │  │评分服务  │  │AI服务    │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │视频服务  │  │通知服务  │  │统计服务  │  │文件服务  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                    数据存储层 (Storage)                       │
│  MySQL / PostgreSQL | MongoDB | Redis | OSS/S3 | MQ         │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                   AI 模型服务层 (AI Engine)                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘

# 人员设计
主要有五类角色以及对应权限：
- 系统管理员：全部权限
- 学校管理员：本校数据管理、教师管理
- 教师：创建项目、查看报告、分配任务
- 评分员：评分、查看待评测任务
- 学生：查看自己的成绩

# 微服务划分-核心服务
1. 用户服务 (User Service)
  - 用户注册、登录、权限管理
  - 角色管理 (RBAC)
  - 学校、教师、评分员、学生管理
  - 与学校用户平台对接接口(暂时忽略)
2. 项目服务 (Project Service)
  - 项目 CRUD
  - 项目状态管理
  - 任务分配逻辑
  - 评分细则管理
3. 评分服务 (Scoring Service)
  - 评分记录管理
  - 人工评分提交
  - AI评分结果存储
  - 评分对比计算
4. 视频服务 (Video Service)
  - 视频上传（分片上传）
  - 视频转码、压缩
  - 视频存储管理
  - 视频流式播放
  - 视频元数据管理
5. AI服务 (AI Service)
  - AI 模型调用封装
  - 视频分析任务队列
  - 结果解析与存储
  - 模型版本管理
6. 文件服务 (File Service)
  - Excel 评分细则上传解析
  - 文件存储 (OSS/S3)
  - 文件下载
7. 通知服务 (Notification Service)
  - 任务提醒
  - 邮件/短信通知
  - WebSocket 实时消息推送
8. 统计服务 (Analytics Service)
  - 数据汇总统计
  - 可视化报表生成
  - 多维度分析


# 接口设计

## 一、API 设计原则
1.1 RESTful 规范
- 使用标准 HTTP 方法: GET, POST, PUT, PATCH, DELETE
- 使用名词复数形式: /api/v1/projects, /api/v1/videos
- 使用子资源: /api/v1/projects/:id/videos
- HTTP 状态码遵循标准语义
1.2 版本控制
- URL 版本: /api/v1/*
- 向后兼容，废弃的 API 保留至少 6 个月
1.3 统一响应格式
成功响应
{
  "code": 200,
  "message": "success",
  "data": { ... },
  "timestamp": "2026-03-18T15:30:00Z"
}
分页响应
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [...],
    "pagination": {
      "page": 1,
      "pageSize": 20,
      "total": 100,
      "totalPages": 5
    }
  }
}
错误响应
{
  "code": 400,
  "message": "Validation failed",
  "errors": [
    {
      "field": "email",
      "message": "Invalid email format"
    }
  ],
  "timestamp": "2026-03-18T15:30:00Z"
}
1.4 通用状态码
状态码
200 成功
201 创建成功
204 删除成功（无返回内容）
400 请求参数错误
401 未认证
403 无权限
404 资源不存在
409 资源冲突
422 业务逻辑错误
429 请求过于频繁
500 服务器错误
503 服务不可用

## 二、认证与鉴权
2.1 认证流程
登录
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "teacher001",
  "password": "password123"
}

Response:
{
  "code": 200,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expiresIn": 3600,
    "user": {
      "id": "uuid",
      "username": "teacher001",
      "realName": "张老师",
      "role": "teacher",
      "school": {
        "id": "uuid",
        "name": "华东师范大学附属中学"
      }
    }
  }
}
刷新 Token
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refreshToken": "..."
}

Response: (同登录响应)
登出
POST /api/v1/auth/logout
Authorization: Bearer {accessToken}

Response:
{
  "code": 200,
  "message": "Logout successful"
}
2.2 权限检查
所有受保护的 API 需要在请求头携带 Token:
Authorization: Bearer {accessToken}

---
## 三、用户管理 API
3.1 获取当前用户信息
GET /api/v1/users/me
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "username": "teacher001",
    "email": "teacher@example.com",
    "realName": "张老师",
    "role": "teacher",
    "school": {...},
    "permissions": ["project:create", "project:read", ...]
  }
}
3.2 更新用户信息
PATCH /api/v1/users/me
Authorization: Bearer {token}
Content-Type: application/json

{
  "realName": "张三",
  "email": "new@example.com",
  "phone": "13800138000"
}

Response:
{
  "code": 200,
  "data": {...}  // 更新后的用户信息
}
3.3 用户管理（管理员）
创建用户
POST /api/v1/users
Authorization: Bearer {token}
Content-Type: application/json

{
  "username": "scorer001",
  "password": "initialPassword",
  "email": "scorer@example.com",
  "realName": "评分员A",
  "role": "scorer",
  "schoolId": "uuid"
}

Response:
{
  "code": 201,
  "data": {...}  // 创建的用户信息
}
批量导入用户
POST /api/v1/users/batch
Authorization: Bearer {token}
Content-Type: application/json

{
  "items": [
    {
      "row": 2,
      "username": "scorer001",
      "password": "initialPassword",
      "email": "scorer@example.com",
      "realName": "评分员A",
      "role": "scorer",
      "schoolId": "uuid"
    }
  ]
}

说明：
- 前端负责解析用户表格，再将用户数组通过 JSON 发送给后端
- `row` 为可选字段，用于回传错误定位
- 字段定义与单用户创建尽量保持一致
- `admin` 创建非 `admin` 用户时必须提供 `schoolId`
- `school_admin` / `school_leader` 创建用户时可省略 `schoolId`，后端会自动使用当前操作者学校

Response:
{
  "code": 200,
  "data": {
    "total": 100,
    "success": 95,
    "failed": 5,
    "errors": [
      {"row": 10, "error": "Duplicate username"},
      ...
    ]
  }
}
查询用户列表
GET /api/v1/users?page=1&pageSize=20&role=scorer&schoolId=uuid&keyword=张
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "items": [...],
    "pagination": {...}
  }
}
更新用户
PATCH /api/v1/users/:id
Authorization: Bearer {token}

{
  "role": "teacher",
  "status": "active"
}
删除用户
DELETE /api/v1/users/:id
Authorization: Bearer {token}

Response: 204 No Content

---

# 数据库设计
PostgreSQL 表设计
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

## 缓存 Key 设计
- Session
session:{userId}:{token}  -> JSON  (TTL: 7天)

- 用户信息缓存
user:{userId}  -> JSON  (TTL: 1小时)

- 权限缓存
user:permissions:{userId}  -> JSON  (TTL: 30分钟)

- 评分细则缓存
rubric:{rubricId}  -> JSON  (TTL: 1小时)

- 项目缓存
project:{projectId}  -> JSON  (TTL: 10分钟)

- 视频上传进度
video:upload:progress:{videoId}  -> INT  (TTL: 24小时)

- AI 任务队列
queue:ai:tasks  -> List

- 视频转码队列
queue:video:transcode  -> List

- 在线用户
online:users  -> Set

- 通知队列
queue:notifications:{userId}  -> List

- 统计数据缓存
stats:project:{projectId}  -> JSON  (TTL: 5分钟)

- API 限流
ratelimit:{userId}:{endpoint}  -> INT  (TTL: 1分钟)
