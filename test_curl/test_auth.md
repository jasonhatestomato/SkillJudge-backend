# 认证与用户管理模块测试 `curl`

本文档中的命令尽量采用 Postman 环境变量风格，方便直接复制到 Postman 或手动替换后执行。

建议先在 Postman 中准备这些环境变量：

```text
baseUrl = http://127.0.0.1:8080
username = 你的测试用户名
password = 你的测试密码
accessToken = 登录后返回的 accessToken
refreshToken = 登录后返回的 refreshToken
adminUsername = 管理员用户名
adminPassword = 管理员密码
adminAccessToken = 管理员登录后返回的 accessToken
schoolId = 真实学校ID
targetUserId = 要更新或删除的目标用户ID
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1aWQiOiJiMDAwMDAwMC0wMDAwLTAwMDAtMDAwMC0wMDAwMDAwMDAwMDIiLCJyb2xlIjoic2Nob29sX2FkbWluIiwic2Nob29sSWQiOiJhMDAwMDAwMC0wMDAwLTAwMDAtMDAwMC0wMDAwMDAwMDAwMDEiLCJzaWQiOiI1NWRiNGE4Yy0wMDk5LTQyNTUtOGZmMS0wOTdkZTY3M2NmNzQiLCJ0eXAiOiJhY2Nlc3MiLCJpc3MiOiJza2lsbGp1ZGdlIiwic3ViIjoiYjAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAyIiwiZXhwIjoxNzc0Mjc3NDA5LCJuYmYiOjE3NzQyNzM4MDksImlhdCI6MTc3NDI3MzgwOSwianRpIjoiZDkxMjIyMWEtYTYwYi00ODZiLWE2YWEtYjgxMGMwMzQzN2M5In0.QRiOl2Y-D5x0GaknBA7nW5_uVnIRNO0NcwZoUtXP6Ok

## 1. 服务健康检查

### 1.1 检查服务存活

```bash
curl --location '{{baseUrl}}/health'
```

### 1.2 检查服务就绪

```bash
curl --location '{{baseUrl}}/ready'
```

## 2. 用户登录

```bash
curl --location 'http://127.0.0.1:8080/api/v1/auth/login' \
--header 'Content-Type: application/json' \
--data '{
  "username": "admin",
  "password": "$2b$10$dummyhashforadmin000000000000000000000000000"
}'
```

期望：
- HTTP `200`
- 响应中包含 `data.accessToken`
- 响应中包含 `data.refreshToken`

## 3. 获取当前用户信息

```bash
curl --location '{{baseUrl}}/api/v1/users/me' \
--header 'Authorization: Bearer {{accessToken}}'
```

期望：
- HTTP `200`
- 返回当前用户的 `id`、`username`、`role`、`status`

## 4. 更新当前用户资料

```bash
curl --location --request PATCH '{{baseUrl}}/api/v1/users/me' \
--header 'Authorization: Bearer {{accessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "email": "teacher01@example.com",
  "realName": "测试教师",
  "phone": "13800138000"
}'
```

期望：
- HTTP `200`
- 返回更新后的用户信息

## 5. 刷新令牌

```bash
curl --location 'http://127.0.0.1:8080/api/v1/auth/refresh' \
--header 'Content-Type: application/json' \
--data '{
  "refreshToken": "{{refreshToken}}"
}'
```

期望：
- HTTP `200`
- 返回新的 `accessToken`
- 返回新的 `refreshToken`

## 6. 登出

```bash
curl --location --request POST 'http://127.0.0.1:8080/api/v1/auth/logout' \
--header 'Authorization: Bearer {{accessToken}}'
```

期望：
- HTTP `200`
- 返回 `Logout successful`

登出后再访问：

```bash
curl --location 'http://127.0.0.1:8080/api/v1/users/me' \
--header 'Authorization: Bearer {{accessToken}}'
```

期望：
- HTTP `401`

## 7. 管理员登录

```bash
curl --location 'http://127.0.0.1:8080/api/v1/auth/login' \
--header 'Content-Type: application/json' \
--data '{
  "username": "{{adminUsername}}",
  "password": "{{adminPassword}}"
}'
```

期望：
- HTTP `200`
- 把返回的 `data.accessToken` 写入 `{{adminAccessToken}}`

## 8. 管理员创建用户

示例创建 `teacher` 用户。当前规则下，非 `admin` 角色必须传 `schoolId`。

```bash
curl --location 'http://127.0.0.1:8080/api/v1/users' \
--header 'Authorization: Bearer {{adminAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "username": "teacher_test_01",
  "password": "Teacher@123",
  "email": "teacher_test_01@example.com",
  "phone": "13900000001",
  "realName": "联调教师01",
  "role": "teacher",
  "schoolId": "{{schoolId}}"
}'
```

期望：
- HTTP `201`
- 返回新建用户 `id`

## 9. 管理员查询用户列表
----- why so many ?

### 9.1 默认分页查询

```bash
curl --location '{{baseUrl}}/api/v1/users?page=1&pageSize=20' \
--header 'Authorization: Bearer {{adminAccessToken}}'
```

### 9.2 按角色筛选

```bash
curl --location '{{baseUrl}}/api/v1/users?page=1&pageSize=20&role=teacher' \
--header 'Authorization: Bearer {{adminAccessToken}}'
```

### 9.3 按学校筛选

```bash
curl --location '{{baseUrl}}/api/v1/users?page=1&pageSize=20&schoolId={{schoolId}}' \
--header 'Authorization: Bearer {{adminAccessToken}}'
```

### 9.4 按关键字搜索

```bash
curl --location '{{baseUrl}}/api/v1/users?page=1&pageSize=20&keyword=teacher_test' \
--header 'Authorization: Bearer {{adminAccessToken}}'
```

## 10. 管理员更新用户

### 10.1 更新状态为 `inactive` fobbiden?

```bash
curl --location --request PATCH '{{baseUrl}}/api/v1/users/{{targetUserId}}' \
--header 'Authorization: Bearer {{adminAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "status": "inactive"
}'
```

### 10.2 修改角色为 `scorer`

```bash
curl --location --request PATCH '{{baseUrl}}/api/v1/users/{{targetUserId}}' \
--header 'Authorization: Bearer {{adminAccessToken}}' \
--header 'Content-Type: application/json' \
--data '{
  "role": "scorer"
}'
```

期望：
- HTTP `200`
- 返回更新后的用户信息

## 11. 管理员删除用户 fobbiden??

```bash
curl --location --request DELETE '{{baseUrl}}/api/v1/users/{{targetUserId}}' \
--header 'Authorization: Bearer {{adminAccessToken}}'
```

期望：
- HTTP `204`

## 12. 常见失败场景

### 12.1 用户名或密码错误

```bash
curl --location '{{baseUrl}}/api/v1/auth/login' \
--header 'Content-Type: application/json' \
--data '{
  "username": "wrong_user",
  "password": "wrong_password"
}'
```

期望：
- HTTP `401`

### 12.2 使用无效 token 访问受保护接口

```bash
curl --location '{{baseUrl}}/api/v1/users/me' \
--header 'Authorization: Bearer invalid-token'
```

期望：
- HTTP `401`

### 12.3 非管理员访问用户管理接口

```bash
curl --location '{{baseUrl}}/api/v1/users?page=1&pageSize=20' \
--header 'Authorization: Bearer {{accessToken}}'
```

期望：
- HTTP `403`

## 13. 使用说明

- 如果你直接在终端执行，需要把 `{{baseUrl}}`、`{{accessToken}}` 这些占位符替换成真实值。
- 如果你在 Postman 中使用，建议先建立一个环境，并把这些变量提前填好。
- 如果数据库里只有密码哈希，没有原始密码，需要数据库负责人提供原始密码，或者由后端重新创建测试账号。
