SkillJudge API 接口设计文档
一、API 设计原则
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
说明
200
成功
201
创建成功
204
删除成功（无返回内容）
400
请求参数错误
401
未认证
403
无权限
404
资源不存在
409
资源冲突
422
业务逻辑错误
429
请求过于频繁
500
服务器错误
503
服务不可用

---
二、认证与鉴权
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
三、用户管理 API
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
POST /api/v1/users/batch-import
Authorization: Bearer {token}
Content-Type: multipart/form-data

file: users.xlsx

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
四、项目管理 API
4.1 创建项目
POST /api/v1/projects
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "2026年春季物理实验期中考试",
  "description": "凸透镜成像实验评测",
  "schoolId": "uuid",
  "rubricId": "uuid",  // 可选，选择已有评分细则
  "deadline": "2026-04-30T23:59:59Z",
  "tags": ["物理", "期中考试", "初三"],
  "experimentType": "凸透镜成像",
  "gradeLevel": "初三",
  "subject": "物理",
  "metadata": {}
}

Response:
{
  "code": 201,
  "data": {
    "id": "uuid",
    "name": "...",
    "status": "draft",
    ...
  }
}
4.2 查询项目列表
GET /api/v1/projects?page=1&pageSize=20&status=in_progress&schoolId=uuid&keyword=物理
Authorization: Bearer {token}

Query Parameters:
- page: 页码（默认1）
- pageSize: 每页数量（默认20）
- status: 项目状态（draft, in_progress, completed, archived）
- schoolId: 学校ID
- creatorId: 创建者ID
- keyword: 关键词搜索
- startDate: 开始日期（YYYY-MM-DD）
- endDate: 结束日期（YYYY-MM-DD）

Response:
{
  "code": 200,
  "data": {
    "items": [
      {
        "id": "uuid",
        "name": "...",
        "status": "in_progress",
        "totalVideos": 50,
        "completedVideos": 30,
        "creator": {...},
        "school": {...},
        "createdAt": "...",
        ...
      }
    ],
    "pagination": {...}
  }
}
4.3 获取项目详情
GET /api/v1/projects/:id
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "name": "...",
    "description": "...",
    "status": "in_progress",
    "rubric": {...},  // 评分细则详情
    "statistics": {
      "totalVideos": 50,
      "completedVideos": 30,
      "pendingVideos": 20,
      "avgAiScore": 75.5,
      "avgManualScore": 78.2,
      "completionRate": 60.0
    },
    ...
  }
}
4.4 更新项目
PATCH /api/v1/projects/:id
Authorization: Bearer {token}

{
  "name": "新项目名称",
  "status": "in_progress",
  "deadline": "2026-05-30T23:59:59Z"
}

Response:
{
  "code": 200,
  "data": {...}  // 更新后的项目
}
4.5 删除项目
DELETE /api/v1/projects/:id
Authorization: Bearer {token}

Response: 204 No Content
4.6 获取项目统计数据
GET /api/v1/projects/:id/statistics
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "overview": {
      "totalVideos": 100,
      "completedVideos": 85,
      "avgAiScore": 76.5,
      "avgManualScore": 78.3,
      "avgScoreDifference": 1.8
    },
    "scoreDistribution": [
      {"range": "0-60", "count": 5},
      {"range": "60-70", "count": 15},
      {"range": "70-80", "count": 40},
      {"range": "80-90", "count": 30},
      {"range": "90-100", "count": 10}
    ],
    "itemAnalysis": [
      {
        "itemName": "实验准备",
        "avgScore": 8.5,
        "fullScore": 10,
        "avgDeduction": 1.5
      },
      ...
    ],
    "timeline": [
      {"date": "2026-03-18", "completed": 10},
      {"date": "2026-03-19", "completed": 15},
      ...
    ]
  }
}

---
五、评分细则 API
5.1 创建评分细则
POST /api/v1/rubrics
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "凸透镜成像实验评分细则",
  "description": "初三物理实验评分标准",
  "totalScore": 100,
  "templateType": "physics_experiment",
  "isTemplate": false,
  "isPublic": false,
  "items": [
    {
      "id": "1",
      "name": "实验准备",
      "score": 10,
      "subItems": [
        {
          "id": "1-1",
          "requirement": "检查实验器材",
          "score": 5,
          "fullScoreStandard": "全部检查完毕且记录",
          "deductionItems": "每缺一项扣1分",
          "dangerousOperation": "未戴护目镜扣2分"
        }
      ]
    },
    ...
  ]
}

Response:
{
  "code": 201,
  "data": {...}  // 创建的评分细则
}
5.2 上传评分细则模板（Excel）
POST /api/v1/rubrics/upload-template
Authorization: Bearer {token}
Content-Type: multipart/form-data

file: rubric_template.xlsx
name: "凸透镜成像实验评分细则"
description: "..."

Response:
{
  "code": 201,
  "data": {
    "id": "uuid",
    "name": "...",
    "items": [...]  // 解析后的评分项
  }
}
5.3 查询评分细则列表
GET /api/v1/rubrics?page=1&pageSize=20&isTemplate=true&keyword=物理
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "items": [...],
    "pagination": {...}
  }
}
5.4 获取评分细则详情
GET /api/v1/rubrics/:id
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "name": "...",
    "totalScore": 100,
    "items": [...]
  }
}
5.5 更新评分细则
PATCH /api/v1/rubrics/:id
Authorization: Bearer {token}

{
  "name": "新名称",
  "items": [...]
}
5.6 删除评分细则
DELETE /api/v1/rubrics/:id
Authorization: Bearer {token}

Response: 204 No Content

---
六、视频管理 API
6.1 获取视频上传凭证
POST /api/v1/videos/upload-credential
Authorization: Bearer {token}
Content-Type: application/json

{
  "projectId": "uuid",
  "filename": "student_001.mp4",
  "fileSize": 1048576000,
  "studentId": "uuid",  // 可选
  "studentName": "张三",
  "studentNumber": "20260301"
}

Response:
{
  "code": 200,
  "data": {
    "videoId": "uuid",
    "uploadUrl": "https://oss.example.com/...",
    "uploadId": "...",  // 分片上传ID
    "credential": {
      "accessKeyId": "...",
      "accessKeySecret": "...",
      "securityToken": "...",
      "expiration": "2026-03-18T16:30:00Z"
    }
  }
}
6.2 确认视频上传完成
POST /api/v1/videos/:id/confirm-upload
Authorization: Bearer {token}
Content-Type: application/json

{
  "uploadId": "...",
  "parts": [
    {"partNumber": 1, "etag": "..."},
    {"partNumber": 2, "etag": "..."}
  ]
}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "status": "transcoding",
    "storageUrl": "..."
  }
}
6.3 批量上传视频（多个文件）
POST /api/v1/videos/batch-upload
Authorization: Bearer {token}
Content-Type: application/json

{
  "projectId": "uuid",
  "videos": [
    {
      "filename": "student_001.mp4",
      "fileSize": 1048576000,
      "studentName": "张三",
      "studentNumber": "20260301"
    },
    ...
  ]
}

Response:
{
  "code": 200,
  "data": {
    "videos": [
      {"videoId": "uuid", "uploadUrl": "...", ...},
      ...
    ]
  }
}
6.4 查询视频列表
GET /api/v1/videos?projectId=uuid&page=1&pageSize=20&status=ready&studentNumber=20260301
Authorization: Bearer {token}

Query Parameters:
- projectId: 项目ID（必填）
- page: 页码
- pageSize: 每页数量
- status: 视频状态（uploaded, transcoding, ready, failed）
- studentId: 学生ID
- studentNumber: 学号
- keyword: 搜索关键词

Response:
{
  "code": 200,
  "data": {
    "items": [
      {
        "id": "uuid",
        "filename": "...",
        "studentName": "张三",
        "studentNumber": "20260301",
        "duration": 300,
        "fileSize": 1048576000,
        "status": "ready",
        "thumbnailUrl": "...",
        "uploadedAt": "...",
        "task": {
          "id": "uuid",
          "status": "completed",
          "aiScore": 75.5,
          "manualScore": 78.0
        }
      },
      ...
    ],
    "pagination": {...}
  }
}
6.5 获取视频详情
GET /api/v1/videos/:id
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "filename": "...",
    "studentName": "张三",
    "duration": 300,
    "status": "ready",
    "playUrl": "https://cdn.example.com/...",  // 临时签名URL
    "thumbnailUrl": "...",
    "task": {...},  // 关联的评测任务
    "aiEvaluation": {...},  // AI 评测结果
    "manualEvaluation": {...}  // 人工评测结果
  }
}
6.6 获取视频播放URL
GET /api/v1/videos/:id/play-url
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "playUrl": "https://cdn.example.com/...",
    "expiresIn": 3600
  }
}
6.7 删除视频
DELETE /api/v1/videos/:id
Authorization: Bearer {token}

Response: 204 No Content

---
七、评测任务 API
7.1 创建评测任务（分配任务）
POST /api/v1/projects/:projectId/tasks
Authorization: Bearer {token}
Content-Type: application/json

{
  "videoIds": ["uuid1", "uuid2", ...],  // 待评测的视频
  "scorerIds": ["uuid1", "uuid2", ...],  // 评分员
  "assignmentStrategy": "average",  // average: 均分, specific: 指定分配
  "specificAssignments": [  // 当 strategy 为 specific 时使用
    {"videoId": "uuid1", "scorerId": "scorer1"},
    ...
  ],
  "deadline": "2026-04-30T23:59:59Z",
  "enableAI": true  // 是否启用AI评测
}

Response:
{
  "code": 201,
  "data": {
    "total": 50,
    "created": 50,
    "tasks": [...]
  }
}
7.2 查询评测任务列表
教师查询（所有任务）
GET /api/v1/projects/:projectId/tasks?page=1&pageSize=20&status=completed
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "items": [
      {
        "id": "uuid",
        "video": {...},
        "scorer": {...},
        "status": "completed",
        "aiScore": 75.5,
        "manualScore": 78.0,
        "scoreDifference": 2.5,
        "completedAt": "..."
      },
      ...
    ],
    "pagination": {...}
  }
}
评分员查询（我的任务）
GET /api/v1/tasks/my?page=1&pageSize=20&status=pending
Authorization: Bearer {token}

Query Parameters:
- page, pageSize
- status: pending, in_progress, completed, skipped
- projectId: 项目ID

Response:
{
  "code": 200,
  "data": {
    "items": [
      {
        "id": "uuid",
        "project": {...},
        "video": {...},
        "rubric": {...},
        "status": "pending",
        "deadline": "..."
      },
      ...
    ],
    "pagination": {...}
  }
}
7.3 获取任务详情
GET /api/v1/tasks/:id
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "project": {...},
    "video": {
      "id": "uuid",
      "playUrl": "...",
      "duration": 300,
      "studentName": "张三"
    },
    "rubric": {...},  // 评分细则
    "aiEvaluation": {  // AI 评测结果（如果启用）
      "score": 75.5,
      "videostage": [...],
      "videopoint": [...],
      "report": {...}
    },
    "manualEvaluation": {  // 人工评测结果（如果已评分）
      "score": 78.0,
      "scoreDetails": [...],
      "comments": "..."
    },
    "status": "in_progress",
    "deadline": "..."
  }
}
7.4 开始评分
POST /api/v1/tasks/:id/start
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "status": "in_progress",
    "startedAt": "..."
  }
}
7.5 提交评分
POST /api/v1/tasks/:id/submit
Authorization: Bearer {token}
Content-Type: application/json

{
  "scoreDetails": [
    {
      "itemId": "1-1",
      "score": 5,
      "deduction": 0,
      "comment": "操作规范"
    },
    {
      "itemId": "1-2",
      "score": 3,
      "deduction": 2,
      "comment": "未记录型号"
    },
    ...
  ],
  "totalScore": 78.0,
  "comments": "整体操作流程较为规范，但细节处理有待改进"
}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "status": "completed",
    "manualEvaluation": {...},
    "comparison": {  // 与AI评分对比
      "aiScore": 75.5,
      "manualScore": 78.0,
      "difference": 2.5,
      "itemDifferences": [...]
    }
  }
}
7.6 保存草稿
POST /api/v1/tasks/:id/draft
Authorization: Bearer {token}
Content-Type: application/json

{
  "scoreDetails": [...],
  "totalScore": 78.0
}

Response:
{
  "code": 200,
  "message": "Draft saved"
}

---
八、AI 评测 API
8.1 触发 AI 评测
POST /api/v1/ai/evaluate
Authorization: Bearer {token}
Content-Type: application/json

{
  "videoId": "uuid",
  "taskId": "uuid",
  "rubricId": "uuid",
  "callbackUrl": "https://api.example.com/callback"  // 可选
}

Response:
{
  "code": 200,
  "data": {
    "evaluationId": "uuid",
    "status": "processing",
    "estimatedTime": 300  // 预计耗时（秒）
  }
}
8.2 批量触发 AI 评测
POST /api/v1/ai/batch-evaluate
Authorization: Bearer {token}
Content-Type: application/json

{
  "projectId": "uuid",
  "videoIds": ["uuid1", "uuid2", ...]
}

Response:
{
  "code": 200,
  "data": {
    "total": 50,
    "queued": 50,
    "estimatedTime": 7200
  }
}
8.3 查询 AI 评测结果
GET /api/v1/ai/evaluations/:id
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "id": "uuid",
    "videoId": "uuid",
    "taskId": "uuid",
    "status": "completed",  // processing, completed, failed
    "modelVersion": "v1.2.0",
    "totalScore": 75.5,
    "result": {
      "videostage": [
        {
          "name": "实验准备",
          "color": "purple",
          "start_time": "00:00:00",
          "end_time": "00:00:30"
        },
        ...
      ],
      "videopoint": [
        {
          "name": "未锁止工具车",
          "type": "general error",
          "start_time": "00:00:05",
          "end_time": "00:00:10"
        },
        ...
      ],
      "report": {
        "overallDescription": "...",
        "score": 75.5,
        "details": [
          {
            "title": "一、实验准备",
            "label": "扣分",
            "labelColor": "purple",
            "subscore": 8,
            "subDetails": [
              {
                "subtitle": "个人防护",
                "subsubscore": 2,
                "AIscore": 2,
                "status": "correct",
                "feedback": "..."
              },
              ...
            ]
          },
          ...
        ]
      }
    },
    "completedAt": "..."
  }
}
8.4 AI 评测回调接口（供 AI 服务调用）
POST /api/v1/ai/callback
Content-Type: application/json
X-AI-Signature: {signature}  // 签名验证

{
  "evaluationId": "uuid",
  "taskId": "uuid",
  "videoId": "uuid",
  "status": "completed",
  "result": {...},  // 完整的评测结果
  "modelVersion": "v1.2.0",
  "processingTime": 285
}

Response:
{
  "code": 200,
  "message": "Received"
}

---
九、统计分析 API
9.1 平台数据总览
GET /api/v1/analytics/overview
Authorization: Bearer {token}

Query Parameters:
- schoolId: 学校ID（可选）
- startDate: 开始日期
- endDate: 结束日期

Response:
{
  "code": 200,
  "data": {
    "totalProjects": 50,
    "totalVideos": 5000,
    "totalUsers": 200,
    "activeProjects": 15,
    "avgAiScore": 76.5,
    "avgManualScore": 78.3,
    "avgProcessingTime": 120,  // AI 平均处理时间（秒）
    "systemLoad": {
      "cpu": 45.5,
      "memory": 60.2,
      "storage": 70.5
    }
  }
}
9.2 学校维度分析
GET /api/v1/analytics/schools/:schoolId
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "school": {...},
    "projects": 20,
    "videos": 2000,
    "teachers": 50,
    "scorers": 30,
    "students": 1000,
    "avgScore": 77.5,
    "scoreDistribution": [...],
    "topProjects": [...]  // 按参与人数排序
  }
}
9.3 导出报表
POST /api/v1/analytics/export
Authorization: Bearer {token}
Content-Type: application/json

{
  "type": "project",  // project, school, user
  "format": "xlsx",  // xlsx, csv, pdf
  "projectId": "uuid",
  "filters": {
    "startDate": "2026-03-01",
    "endDate": "2026-03-31"
  }
}

Response:
{
  "code": 200,
  "data": {
    "taskId": "uuid",
    "status": "processing",
    "estimatedTime": 60
  }
}

// 查询导出任务状态
GET /api/v1/analytics/export/:taskId
Response:
{
  "code": 200,
  "data": {
    "taskId": "uuid",
    "status": "completed",
    "downloadUrl": "https://...",
    "expiresIn": 3600
  }
}

---
十、通知 API
10.1 获取通知列表
GET /api/v1/notifications?page=1&pageSize=20&isRead=false
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "items": [
      {
        "id": "uuid",
        "type": "task_assigned",
        "title": "您有新的评分任务",
        "content": "项目【2026春季物理实验】分配了5个视频给您评分",
        "linkUrl": "/tasks/xxx",
        "isRead": false,
        "createdAt": "..."
      },
      ...
    ],
    "pagination": {...},
    "unreadCount": 10
  }
}
10.2 标记为已读
PATCH /api/v1/notifications/:id/read
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "message": "Marked as read"
}
10.3 全部标记为已读
POST /api/v1/notifications/mark-all-read
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "message": "All marked as read"
}
10.4 WebSocket 实时通知
// 客户端连接
const ws = new WebSocket('wss://api.example.com/ws?token={accessToken}');

ws.onmessage = (event) => {
  const notification = JSON.parse(event.data);
  /*
  {
    "type": "notification",
    "data": {
      "id": "uuid",
      "type": "task_assigned",
      "title": "...",
      "content": "...",
      "linkUrl": "..."
    }
  }
  */
};

---
十一、文件管理 API
11.1 上传文件（通用）
POST /api/v1/files/upload
Authorization: Bearer {token}
Content-Type: multipart/form-data

file: document.pdf
type: document  // document, image, excel

Response:
{
  "code": 200,
  "data": {
    "fileId": "uuid",
    "filename": "document.pdf",
    "url": "https://...",
    "size": 1048576
  }
}
11.2 下载文件
GET /api/v1/files/:id/download
Authorization: Bearer {token}

Response:
302 Redirect to signed URL

---
十二、系统配置 API
12.1 获取系统配置
GET /api/v1/system/config
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "videoUploadLimit": 52428800,  // 50GB (字节)
    "allowedVideoFormats": ["mp4", "avi", "mov"],
    "maxVideoSize": 5368709120,  // 5GB
    "aiModelVersion": "v1.2.0",
    "features": {
      "aiEvaluation": true,
      "cloudUpload": false,
      "multiLanguage": false
    }
  }
}

---
十三、健康检查 API
13.1 健康检查
GET /api/health

Response:
{
  "status": "healthy",
  "version": "1.0.0",
  "services": {
    "database": "healthy",
    "redis": "healthy",
    "objectStorage": "healthy",
    "aiService": "healthy"
  },
  "timestamp": "2026-03-18T15:30:00Z"
}
13.2 服务状态
GET /api/v1/system/status
Authorization: Bearer {token}

Response:
{
  "code": 200,
  "data": {
    "uptime": 864000,  // 运行时间（秒）
    "activeUsers": 150,
    "activeProjects": 20,
    "queueStatus": {
      "aiTasks": 50,
      "videoTranscode": 10
    }
  }
}

---
十四、错误码定义
错误码
说明
1000
系统错误
1001
数据库错误
1002
缓存错误
1003
第三方服务错误
2000
参数错误
2001
参数缺失
2002
参数格式错误
2003
参数值超出范围
3000
认证失败
3001
Token 无效或过期
3002
用户名或密码错误
3003
账号已被禁用
4000
无权限
4001
无该资源访问权限
4002
无该操作权限
5000
资源不存在
5001
用户不存在
5002
项目不存在
5003
视频不存在
5004
任务不存在
6000
业务逻辑错误
6001
资源已存在
6002
资源状态不允许该操作
6003
超出配额限制
6004
文件格式不支持
6005
文件大小超出限制
7000
AI 服务错误
7001
AI 模型不可用
7002
AI 分析超时

---
十五、API 调用示例
完整业务流程示例
1. 教师创建项目并上传视频
// Step 1: 登录
const loginRes = await fetch('/api/v1/auth/login', {
  method: 'POST',
  body: JSON.stringify({username: 'teacher001', password: 'pwd123'})
});
const {accessToken} = (await loginRes.json()).data;

// Step 2: 创建项目
const projectRes = await fetch('/api/v1/projects', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    name: '2026春季物理实验',
    rubricId: 'rubric-uuid',
    deadline: '2026-04-30T23:59:59Z'
  })
});
const project = (await projectRes.json()).data;

// Step 3: 获取上传凭证
const credentialRes = await fetch('/api/v1/videos/upload-credential', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    projectId: project.id,
    filename: 'student_001.mp4',
    fileSize: 1048576000,
    studentName: '张三',
    studentNumber: '20260301'
  })
});
const {videoId, uploadUrl} = (await credentialRes.json()).data;

// Step 4: 上传视频（使用 OSS SDK）
// ... 上传逻辑 ...

// Step 5: 确认上传
await fetch(`/api/v1/videos/${videoId}/confirm-upload`, {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({uploadId: '...', parts: [...]})
});

// Step 6: 分配评测任务
await fetch(`/api/v1/projects/${project.id}/tasks`, {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    videoIds: [videoId],
    scorerIds: ['scorer-uuid-1'],
    enableAI: true
  })
});
2. 评分员评分
// Step 1: 获取我的待评测任务
const tasksRes = await fetch('/api/v1/tasks/my?status=pending', {
  headers: {'Authorization': `Bearer ${accessToken}`}
});
const tasks = (await tasksRes.json()).data.items;

// Step 2: 开始评分
const task = tasks[0];
await fetch(`/api/v1/tasks/${task.id}/start`, {
  method: 'POST',
  headers: {'Authorization': `Bearer ${accessToken}`}
});

// Step 3: 获取任务详情（包含视频和评分细则）
const taskDetailRes = await fetch(`/api/v1/tasks/${task.id}`, {
  headers: {'Authorization': `Bearer ${accessToken}`}
});
const taskDetail = (await taskDetailRes.json()).data;

// Step 4: 提交评分
await fetch(`/api/v1/tasks/${task.id}/submit`, {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    scoreDetails: [
      {itemId: '1-1', score: 5, deduction: 0, comment: '...'},
      ...
    ],
    totalScore: 78.0,
    comments: '整体操作规范'
  })
});

---
总结
本 API 文档涵盖了：
- ✅ 认证鉴权: JWT Token 机制
- ✅ 用户管理: CRUD + 批量导入
- ✅ 项目管理: 完整生命周期
- ✅ 评分细则: 模板上传与管理
- ✅ 视频管理: 分片上传 + 转码
- ✅ 评测任务: 分配与提交
- ✅ AI 评测: 触发与回调
- ✅ 统计分析: 多维度数据
- ✅ 实时通知: WebSocket 推送
下一步：
- 根据 API 设计实现后端服务
- 编写 Swagger/OpenAPI 文档
- 前端根据 API 开发界面