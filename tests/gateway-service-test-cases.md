# Gateway 服务测试用例文档

**版本**：1.0
**日期**：2026-07-08
**需求来源**：gateway-service-prd.md (v1.1)、gateway-v1.2-prd.md (v1.2)
**覆盖率目标**：所有需求点均有对应用例

---

## 目录

1. [功能测试（TC-F）](#1-功能测试tc-f)
2. [边界测试（TC-E）](#2-边界测试tc-e)
3. [异常测试（TC-ERR）](#3-异常测试tc-err)
4. [状态转换测试（TC-ST）](#4-状态转换测试tc-st)
5. [需求-测试用例覆盖矩阵](#5-需求-测试用例覆盖矩阵)

---

## 1. 功能测试（TC-F）

### TC-F-001 健康检查接口返回正常

- **需求来源**：gateway-service-prd.md / 功能 1 路由聚合
- **优先级**：高
- **前置条件**：Gateway 服务已启动，监听 8082 端口
- **测试步骤**：
  1. 发送 `GET /health` 请求，不携带任何认证信息
  2. 检查 HTTP 响应状态码
  3. 检查响应体内容
- **预期结果**：
  - HTTP 状态码 200
  - 响应体为 `{"status":"ok"}`

### TC-F-002 微信登录接口正常流程

- **需求来源**：gateway-service-prd.md / Story 1
- **优先级**：高
- **前置条件**：User Service 已注册到 etcd，微信 code 有效
- **测试步骤**：
  1. 发送 `POST /api/v1/user/login`，请求体 `{"code": "valid_wx_code"}`
  2. 检查 HTTP 响应状态码
  3. 检查响应体中的 `access_token`、`is_bound_campus`、`school_id` 字段
- **预期结果**：
  - HTTP 状态码 200
  - 响应体格式为 `{"code": 0, "message": "ok", "data": {"access_token": "...", "is_bound_campus": true/false, "school_id": 123}, "trace_id": "..."}`
  - `access_token` 非空，`is_bound_campus` 为布尔值

### TC-F-003 登录接口无需 JWT 鉴权（白名单）

- **需求来源**：gateway-service-prd.md / 功能 2 白名单路由
- **优先级**：高
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `POST /api/v1/user/login`，请求体 `{"code": "some_code"}`，不携带 `Authorization` 头
  2. 检查响应状态码（预期非 401）
- **预期结果**：
  - 不返回 401（即不要求 Token）
  - 请求正常进入 login 业务逻辑处理

### TC-F-004 Refresh Token 接口无需鉴权（白名单）

- **需求来源**：gateway-service-prd.md / Story 3、功能 2
- **优先级**：高
- **前置条件**：Gateway 服务已启动，Refresh Token 机制已实现
- **测试步骤**：
  1. 发送 `POST /api/v1/user/refresh`，请求体 `{"refresh_token": "some_token"}`，不携带 `Authorization` 头
  2. 检查响应状态码（预期非 401）
- **预期结果**：
  - 不返回 401（即白名单，不要求 Access Token）
  - 请求正常进入 refresh 业务逻辑

### TC-F-005 受保护接口携带有效 JWT 正常访问

- **需求来源**：gateway-service-prd.md / Story 2
- **优先级**：高
- **前置条件**：已通过登录获取有效 Access Token
- **测试步骤**：
  1. 使用有效 Token 调用 `GET /api/v1/user/me`，请求头携带 `Authorization: Bearer <valid_token>`
  2. 检查响应状态码
  3. 检查响应体格式
- **预期结果**：
  - HTTP 状态码 200
  - 返回用户信息，格式符合统一响应体 `{code: 0, message: "ok", data: {...}, trace_id: "..."}`

### TC-F-006 JWT 解析后注入 user_id 和 role 到 gRPC metadata

- **需求来源**：gateway-service-prd.md / Story 2、gRPC Metadata 透传清单
- **优先级**：高
- **前置条件**：已获取有效 Access Token，User Service 侧日志可查看接收到的 metadata
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`，携带有效 Token
  2. 检查 User Service 侧接收到的 gRPC metadata
- **预期结果**：
  - gRPC metadata 中包含 `user-id`（十进制字符串）
  - gRPC metadata 中包含 `user-role`（十进制字符串）
  - 值与 JWT Claims 中的 `UserID` 和 `Role` 一致

### TC-F-007 school_id 注入下游 gRPC metadata

- **需求来源**：gateway-service-prd.md / Story 4、功能 4、gRPC Metadata 透传清单
- **优先级**：高
- **前置条件**：已绑定学校的用户，持有包含 `school_id` 的有效 Token
- **测试步骤**：
  1. 调用任意受保护接口（如 `GET /api/v1/user/me`），携带有效 Token
  2. 检查下游 gRPC 服务接收到的 metadata
- **预期结果**：
  - gRPC metadata 中包含 `school-id`（十进制字符串）
  - 值与 JWT Claims 中的 `school_id` 一致

### TC-F-008 Content Service 帖子列表查询（公开接口）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：Content Service 已注册到 etcd，学校 X 下存在若干帖子
- **测试步骤**：
  1. 发送 `GET /api/v1/content/posts`，不携带 JWT Token
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 200
  - 返回帖子列表，列表中所有帖子的 `school_id` 与当前学校一致
  - 响应格式符合统一响应体

### TC-F-009 Content Service 帖子详情查询（公开接口）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：Content Service 中存在帖子 ID 为 123 的帖子
- **测试步骤**：
  1. 发送 `GET /api/v1/content/posts/123`，不携带 JWT Token
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 200
  - 返回帖子详情数据

### TC-F-010 Content Service 发布帖子（需鉴权）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：已登录并绑定学校的用户，持有有效 Access Token
- **测试步骤**：
  1. 发送 `POST /api/v1/content/posts`，请求体 `{"title": "测试帖子", "content": "内容", "type": 1}`，携带有效 Token
  2. 检查响应状态码
  3. 检查 gRPC metadata 中的 `school-id`
- **预期结果**：
  - HTTP 状态码 200
  - 帖子创建成功
  - gRPC metadata 中包含正确的 `school-id`

### TC-F-011 Content Service 编辑帖子（需鉴权）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：当前用户是目标帖子的作者
- **测试步骤**：
  1. 发送 `PUT /api/v1/content/posts/123`，请求体 `{"title": "修改后的标题"}`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 帖子标题更新成功

### TC-F-012 Content Service 删除帖子（需鉴权）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：当前用户是目标帖子的作者
- **测试步骤**：
  1. 发送 `DELETE /api/v1/content/posts/123`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 帖子删除成功

### TC-F-013 Content Service 发表评论（需鉴权）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：已登录并绑定学校的用户
- **测试步骤**：
  1. 发送 `POST /api/v1/content/posts/123/comments`，请求体 `{"content": "好的评论"}`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 评论创建成功

### TC-F-014 Content Service 评论列表查询（公开接口）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：帖子 123 下存在评论
- **测试步骤**：
  1. 发送 `GET /api/v1/content/posts/123/comments`，不携带 JWT Token
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 200
  - 返回评论列表

### TC-F-015 Content Service 删除评论（需鉴权）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：当前用户是目标评论的作者
- **测试步骤**：
  1. 发送 `DELETE /api/v1/content/comments/456`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 评论删除成功

### TC-F-016 Content Service 点赞（需鉴权）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：已登录并绑定学校的用户
- **测试步骤**：
  1. 发送 `POST /api/v1/content/posts/123/like`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 点赞成功

### TC-F-017 Content Service 取消点赞（需鉴权）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：当前用户已对该帖子点赞
- **测试步骤**：
  1. 发送 `DELETE /api/v1/content/posts/123/like`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 取消点赞成功

### TC-F-018 Content Service 关键词搜索（公开接口）

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：高
- **前置条件**：ES 中存在可搜索的帖子
- **测试步骤**：
  1. 发送 `GET /api/v1/content/search?keyword=失物招领`，不携带 JWT Token
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 200
  - 返回搜索结果列表

### TC-F-019 IP 令牌桶限流正常请求放行

- **需求来源**：gateway-service-prd.md / Story 7、功能 5
- **优先级**：高
- **前置条件**：Gateway 启动，限流器默认配置（100 QPS，突发 200）
- **测试步骤**：
  1. 从同一 IP 发送 100 个请求（在 1 秒内）
  2. 统计每个请求的状态码
- **预期结果**：
  - 所有 100 个请求返回非 429 状态码（正常业务响应）

### TC-F-020 IP 令牌桶限流超限拒绝

- **需求来源**：gateway-service-prd.md / Story 7、功能 5
- **优先级**：高
- **前置条件**：Gateway 启动，限流器默认配置
- **测试步骤**：
  1. 从同一 IP 在 1 秒内发送 250 个请求（超过突发容量 200）
  2. 统计每个请求的状态码
- **预期结果**：
  - 前 200 个请求正常响应
  - 超出部分返回 HTTP 429
  - 429 响应体格式为 `{code: 30001, message: "rate limit exceeded", trace_id: "..."}`

### TC-F-021 全链路追踪：响应头包含 X-Trace-ID

- **需求来源**：gateway-service-prd.md / Story 6、功能 6
- **优先级**：高
- **前置条件**：Gateway 服务已启动，Jaeger 可用
- **测试步骤**：
  1. 发送 `GET /health` 请求
  2. 检查响应头中的 `X-Trace-ID`
- **预期结果**：
  - 响应头包含 `X-Trace-ID`，值为有效的 32 位 hex TraceID

### TC-F-022 全链路追踪：客户端传入 TraceID 被保留

- **需求来源**：gateway-service-prd.md / Story 6、功能 6
- **优先级**：中
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `GET /health` 请求，请求头携带 `X-Trace-ID: abcdef1234567890abcdef1234567890`
  2. 检查响应头中的 `X-Trace-ID`
- **预期结果**：
  - 响应头 `X-Trace-ID` 的值为 `abcdef1234567890abcdef1234567890`（原样保留）

### TC-F-023 全链路追踪：不传 TraceID 时自动生成

- **需求来源**：gateway-service-prd.md / Story 6、功能 6
- **优先级**：高
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 连续发送两个 `GET /health` 请求，均不携带 `X-Trace-ID` 头
  2. 检查两个响应头中的 `X-Trace-ID`
- **预期结果**：
  - 每个响应均包含 `X-Trace-ID`
  - 两个 TraceID 值不同（每次自动生成）

### TC-F-024 全链路追踪：trace_id 注入 gin.Context

- **需求来源**：gateway-service-prd.md / Story 6
- **优先级**：中
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送任意请求，记录响应头中的 `X-Trace-ID`
  2. 检查日志中是否包含该 `trace_id`
- **预期结果**：
  - 日志中能检索到该请求的 `trace_id`，与响应头中的值一致

### TC-F-025 全链路追踪：OTel Span 名称为路由路径

- **需求来源**：gateway-service-prd.md / Story 6、功能 6
- **优先级**：中
- **前置条件**：Gateway 服务已启动，Jaeger UI 可访问
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`（携带有效 Token）
  2. 在 Jaeger UI 中搜索对应 TraceID
  3. 检查 Span 名称
- **预期结果**：
  - Jaeger 中显示 Span 名称为 `/api/v1/user/me`

### TC-F-026 全链路追踪：gRPC 调用注入 TraceContext

- **需求来源**：gateway-service-prd.md / Story 6、功能 6
- **优先级**：中
- **前置条件**：Gateway 服务已启动，Jaeger UI 可访问
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`（携带有效 Token）
  2. 在 Jaeger UI 中搜索对应 TraceID
  3. 检查 Span 树
- **预期结果**：
  - Jaeger 中显示 Span 树：gateway HTTP Span → user-service gRPC Span
  - gRPC Span 为 HTTP Span 的子 Span

### TC-F-027 CORS 预检请求 OPTIONS 返回 204

- **需求来源**：gateway-service-prd.md / Story 5、功能 8
- **优先级**：高
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `OPTIONS /api/v1/user/me` 请求
  2. 检查响应状态码和响应头
- **预期结果**：
  - HTTP 状态码 204
  - 响应头包含 `Access-Control-Allow-Origin: *`
  - 响应头包含 `Access-Control-Allow-Methods`（含 GET,POST,PUT,PATCH,DELETE,OPTIONS）
  - 响应头包含 `Access-Control-Allow-Headers`（含 Authorization,Content-Type,X-Request-ID,X-Trace-ID）
  - 响应头包含 `Access-Control-Expose-Headers: X-Trace-ID`
  - 响应头包含 `Access-Control-Max-Age: 86400`

### TC-F-028 CORS 正常请求携带跨域头

- **需求来源**：gateway-service-prd.md / Story 5、功能 8
- **优先级**：高
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `GET /health` 请求
  2. 检查响应头中的 CORS 相关字段
- **预期结果**：
  - 响应头包含 `Access-Control-Allow-Origin: *`
  - 响应头包含 `Access-Control-Expose-Headers: X-Trace-ID`

### TC-F-029 统一错误响应格式：成功响应

- **需求来源**：gateway-service-prd.md / 功能 7
- **优先级**：高
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `GET /health` 请求
  2. 检查响应体字段
- **预期结果**：
  - 响应体包含 `code` 字段（值为 0）
  - 响应体包含 `message` 字段
  - 响应体包含 `trace_id` 字段

### TC-F-030 统一错误响应格式：鉴权错误

- **需求来源**：gateway-service-prd.md / 功能 7、错误码规范
- **优先级**：高
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`，不携带 Authorization 头
  2. 检查响应体字段
- **预期结果**：
  - HTTP 状态码 401
  - 响应体包含 `code: 20001`
  - 响应体包含 `message: "missing token"`
  - 响应体包含 `trace_id`

### TC-F-031 统一错误响应格式：限流错误

- **需求来源**：gateway-service-prd.md / 功能 7、错误码规范
- **优先级**：高
- **前置条件**：触发限流（从同一 IP 短时间发送大量请求）
- **测试步骤**：
  1. 超过限流阈值后查看 429 响应体
  2. 检查响应体字段
- **预期结果**：
  - HTTP 状态码 429
  - 响应体包含 `code: 30001`
  - 响应体包含 `message: "rate limit exceeded"`
  - 响应体包含 `trace_id`

### TC-F-032 统一错误响应格式：下游服务错误

- **需求来源**：gateway-service-prd.md / 功能 7、错误码规范、gRPC Code 映射
- **优先级**：高
- **前置条件**：下游 User Service 不可用（模拟服务宕机）
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`（携带有效 Token）
  2. 检查响应体字段
- **预期结果**：
  - HTTP 状态码 503
  - 响应体包含 `code`（5xxxx 或 9xxxx 范围）
  - 响应体包含 `message`
  - 响应体包含 `trace_id`

### TC-F-033 Refresh Token 换取 Access Token

- **需求来源**：gateway-service-prd.md / Story 3、功能 3
- **优先级**：高
- **前置条件**：已获取有效 Refresh Token
- **测试步骤**：
  1. 发送 `POST /api/v1/user/refresh`，请求体 `{"refresh_token": "<valid_refresh_token>"}`
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 200
  - 响应体 `data` 中包含新的 `access_token`
  - 新 `access_token` 可正常访问受保护接口

### TC-F-034 多租户隔离：绑定学校用户正常访问

- **需求来源**：gateway-service-prd.md / Story 4、功能 4
- **优先级**：高
- **前置条件**：用户已绑定学校，持有包含 `school_id` 的 Token
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 返回用户信息，`school_id` 与 Token 中的一致

### TC-F-035 多租户隔离：未绑定学校用户访问白名单接口

- **需求来源**：gateway-service-prd.md / Story 4、功能 4
- **优先级**：高
- **前置条件**：用户未绑定学校（`school_id` 为 0 或不存在），持有有效 Token
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`（白名单可读接口），携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200（白名单接口允许访问）
  - 返回用户信息，`school_id` 为空或 0

### TC-F-036 多租户隔离：未绑定学校用户访问写接口被拒绝

- **需求来源**：gateway-service-prd.md / Story 4、功能 4
- **优先级**：高
- **前置条件**：用户未绑定学校，持有有效 Token
- **测试步骤**：
  1. 调用 `POST /api/v1/content/posts`（非白名单写接口），携带有效 Token
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 403
  - 响应体 `{code: 20006, message: "campus not bound", trace_id: "..."}`

### TC-F-037 优雅停机：捕获 SIGTERM 信号

- **需求来源**：gateway-service-prd.md / Story 8、功能 9
- **优先级**：中
- **前置条件**：Gateway 进程正在运行
- **测试步骤**：
  1. 启动 Gateway 进程
  2. 发送 `kill -SIGTERM <pid>` 信号
  3. 观察进程行为
- **预期结果**：
  - 进程收到信号后开始优雅停机
  - 打印停机日志信息
  - 进程最终正常退出

### TC-F-038 优雅停机：进行中请求完成

- **需求来源**：gateway-service-prd.md / Story 8、功能 9
- **优先级**：中
- **前置条件**：Gateway 进程正在运行，有一个耗时较长的请求正在处理
- **测试步骤**：
  1. 发送一个需要较长时间处理的请求
  2. 在请求处理过程中发送 `kill -SIGTERM <pid>`
  3. 等待该请求完成
- **预期结果**：
  - 进行中的请求在 10 秒内正常完成并返回响应
  - 之后不再接受新请求

### TC-F-039 优雅停机：关闭 etcd 连接和 Tracer

- **需求来源**：gateway-service-prd.md / Story 8、功能 9
- **优先级**：中
- **前置条件**：Gateway 进程正在运行
- **测试步骤**：
  1. 发送 `kill -SIGTERM <pid>`
  2. 检查日志输出
- **预期结果**：
  - 日志中显示 etcd 连接已关闭
  - 日志中显示 Tracer 已关闭（span 已 flush）

### TC-F-040 用户更新昵称/头像

- **需求来源**：gateway-service-prd.md / User Service 路由表
- **优先级**：中
- **前置条件**：已登录用户，持有有效 Access Token
- **测试步骤**：
  1. 发送 `PUT /api/v1/user/info`，请求体 `{"nickname": "新昵称"}`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 用户昵称更新成功

### TC-F-041 用户绑定学校

- **需求来源**：gateway-service-prd.md / User Service 路由表
- **优先级**：高
- **前置条件**：已登录但未绑定学校的用户
- **测试步骤**：
  1. 发送 `PUT /api/v1/user/campus`，请求体 `{"school_id": 1}`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 学校绑定成功

### TC-F-042 Content Service 评论含 parent_id（一级评论）

- **需求来源**：gateway-v1.2-prd.md / Story 1、FR-1
- **优先级**：高
- **前置条件**：已登录并绑定学校的用户，帖子 123 存在
- **测试步骤**：
  1. 发送 `POST /api/v1/content/posts/123/comments`，请求体 `{"content": "一级评论"}`（不传 `parent_id`），携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 评论创建成功，`parent_id` 为 0（默认一级评论）
  - gRPC 请求中 `ParentId=0`

### TC-F-043 Content Service 评论含 parent_id（二级回复）

- **需求来源**：gateway-v1.2-prd.md / Story 1、FR-1
- **优先级**：高
- **前置条件**：已登录并绑定学校的用户，帖子 123 和父评论 456 存在
- **测试步骤**：
  1. 发送 `POST /api/v1/content/posts/123/comments`，请求体 `{"content": "回复内容", "parent_id": 456}`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200
  - 评论创建成功
  - gRPC 请求中 `ParentId=456`

### TC-F-044 查询评论回复列表

- **需求来源**：gateway-v1.2-prd.md / Story 2、FR-2
- **优先级**：高
- **前置条件**：评论 ID 456 下存在回复，已登录用户
- **测试步骤**：
  1. 发送 `GET /api/v1/content/comments/456/replies`，携带有效 Token
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 200
  - 响应体格式：`{"code": 0, "data": {"replies": [...], "next_cursor": "", "has_more": false}}`
  - 返回的回复数组为评论 456 的所有二级回复

### TC-F-045 查询评论回复列表支持游标分页

- **需求来源**：gateway-v1.2-prd.md / Story 2、FR-2
- **优先级**：中
- **前置条件**：评论 ID 456 下存在超过 `page_size` 条回复
- **测试步骤**：
  1. 发送 `GET /api/v1/content/comments/456/replies?page_size=2`，携带有效 Token
  2. 记录响应中的 `next_cursor` 和 `has_more`
  3. 若 `has_more=true`，使用 `cursor=<next_cursor>&page_size=2` 发起下一次请求
- **预期结果**：
  - 第一次请求返回 2 条回复，`has_more=true`
  - 第二次请求返回后续回复，不与第一次重复
  - 每次响应包含 `replies`、`next_cursor`、`has_more` 字段

---

## 2. 边界测试（TC-E）

### TC-E-001 JWT Token 恰好在过期前一秒有效

- **需求来源**：gateway-service-prd.md / Story 2、功能 2
- **优先级**：中
- **前置条件**：构造一个有效期剩余 1 秒的 Token
- **测试步骤**：
  1. 使用该 Token 调用 `GET /api/v1/user/me`
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 200（Token 尚未过期）

### TC-E-002 JWT Token 恰好过期后一秒无效

- **需求来源**：gateway-service-prd.md / Story 2、功能 2
- **优先级**：中
- **前置条件**：构造一个已过期 1 秒的 Token
- **测试步骤**：
  1. 使用该 Token 调用 `GET /api/v1/user/me`
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 401
  - 响应体 `{code: 20002, message: "token expired", trace_id: "..."}`

### TC-E-003 Access Token 默认有效期 24 小时

- **需求来源**：gateway-service-prd.md / Story 3、功能 3
- **优先级**：中
- **前置条件**：通过登录获取 Access Token
- **测试步骤**：
  1. 记录获取 Token 的时间
  2. 等待 24 小时后（或使用快进机制模拟），使用该 Token 调用受保护接口
- **预期结果**：
  - 24 小时后 Token 过期，返回 401 `{code: 20002}`

### TC-E-004 Refresh Token 默认有效期 7 天（168 小时）

- **需求来源**：gateway-service-prd.md / Story 3、功能 3
- **优先级**：中
- **前置条件**：通过登录获取 Refresh Token
- **测试步骤**：
  1. 使用有效 Refresh Token 调用 `/api/v1/user/refresh`（7 天内）
  2. 使用过期 Refresh Token 调用 `/api/v1/user/refresh`（7 天后）
- **预期结果**：
  - 7 天内：返回新的 `access_token`
  - 7 天后：返回 401 `{code: 20004, message: "refresh token expired"}`

### TC-E-005 限流突发容量恰好 200

- **需求来源**：gateway-service-prd.md / Story 7、功能 5
- **优先级**：中
- **前置条件**：限流器刚重置，桶内令牌满
- **测试步骤**：
  1. 从同一 IP 在极短时间内发送 200 个请求
  2. 统计响应状态码
- **预期结果**：
  - 前 200 个请求全部成功（非 429）
  - 第 201 个请求开始触发 429

### TC-E-006 限流桶令牌耗尽后按速率恢复

- **需求来源**：gateway-service-prd.md / Story 7、功能 5
- **优先级**：中
- **前置条件**：限流器刚重置
- **测试步骤**：
  1. 从同一 IP 快速发送 201 个请求（耗尽桶）
  2. 等待 1 秒（补充 100 个令牌）
  3. 再从同一 IP 发送 100 个请求
- **预期结果**：
  - 第 1 轮：前 200 个成功，第 201 个触发 429
  - 等待 1 秒后，第 2 轮 100 个请求全部成功

### TC-E-007 Content 评论 parent_id 默认值 0

- **需求来源**：gateway-v1.2-prd.md / Story 1
- **优先级**：高
- **前置条件**：已登录并绑定学校的用户
- **测试步骤**：
  1. 发送 `POST /api/v1/content/posts/123/comments`，请求体 `{"content": "测试"}`（完全不传 `parent_id` 字段）
  2. 检查 gRPC 请求中的 `ParentId`
- **预期结果**：
  - gRPC 请求中 `ParentId=0`（默认一级评论）
  - 评论创建成功

### TC-E-008 ListCommentReplies page_size 超过 50

- **需求来源**：gateway-v1.2-prd.md / FR-2
- **优先级**：中
- **前置条件**：评论 ID 456 下存在回复
- **测试步骤**：
  1. 发送 `GET /api/v1/content/comments/456/replies?page_size=100`，携带有效 Token
  2. 检查返回的回复数量
- **预期结果**：
  - Content Service 截断为最多 50 条
  - Gateway 透传截断后的结果

### TC-E-009 多个 IP 独立限流

- **需求来源**：gateway-service-prd.md / Story 7、功能 5
- **优先级**：中
- **前置条件**：可模拟两个不同 IP 的请求
- **测试步骤**：
  1. 从 IP-A 快速发送 201 个请求（触发限流）
  2. 从 IP-B 同时发送 100 个请求
- **预期结果**：
  - IP-A 的第 201 个请求触发 429
  - IP-B 的 100 个请求全部成功（各自独立计数）

### TC-E-010 gRPC Code 到 HTTP Status 映射

- **需求来源**：gateway-service-prd.md / 附录 HTTP 状态码映射
- **优先级**：中
- **前置条件**：可模拟各 gRPC 错误码的下游响应
- **测试步骤**：
  1. 模拟下游返回 gRPC `InvalidArgument`，检查网关 HTTP 状态码
  2. 模拟下游返回 gRPC `NotFound`，检查网关 HTTP 状态码
  3. 模拟下游返回 gRPC `Internal`，检查网关 HTTP 状态码
  4. 模拟下游返回 gRPC `Unavailable`，检查网关 HTTP 状态码
  5. 模拟下游返回 gRPC `DeadlineExceeded`，检查网关 HTTP 状态码
- **预期结果**：
  - `InvalidArgument` → HTTP 400，错误码 4xxxx 段
  - `NotFound` → HTTP 404，错误码 4xxxx 段
  - `Internal` → HTTP 500，错误码 5xxxx/9xxxx 段
  - `Unavailable` → HTTP 503，错误码 5xxxx 段
  - `DeadlineExceeded` → HTTP 504，错误码 5xxxx 段

---

## 3. 异常测试（TC-ERR）

### TC-ERR-001 缺失 Token 访问受保护接口

- **需求来源**：gateway-service-prd.md / Story 2、功能 2
- **优先级**：高
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`，不携带 `Authorization` 头
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 401
  - 响应体 `{code: 20001, message: "missing token", trace_id: "..."}`

### TC-ERR-002 Token 签名无效

- **需求来源**：gateway-service-prd.md / Story 2、功能 2
- **优先级**：高
- **前置条件**：构造一个使用错误密钥签名的 Token
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`，请求头携带 `Authorization: Bearer <invalid_signature_token>`
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 401
  - 响应体 `{code: 20003, message: "invalid token", trace_id: "..."}`

### TC-ERR-003 Token 已过期

- **需求来源**：gateway-service-prd.md / Story 2、功能 2
- **优先级**：高
- **前置条件**：构造一个已过期的 Token
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`，请求头携带 `Authorization: Bearer <expired_token>`
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 401
  - 响应体 `{code: 20002, message: "token expired", trace_id: "..."}`

### TC-ERR-004 Token 格式错误（不带 Bearer 前缀）

- **需求来源**：gateway-service-prd.md / Story 2
- **优先级**：高
- **前置条件**：持有有效 Token
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`，请求头携带 `Authorization: <token>`（缺少 Bearer 前缀）
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 401
  - 响应体包含 `code: 20001` 或 `code: 20003`

### TC-ERR-005 Token 被篡改（Payload 被修改）

- **需求来源**：gateway-service-prd.md / Story 2
- **优先级**：高
- **前置条件**：持有有效 Token，手动修改 Payload 部分（Base64 解码后修改再编码）
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`，请求头携带篡改后的 Token
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 401
  - 响应体 `{code: 20003, message: "invalid token", trace_id: "..."}`

### TC-ERR-006 微信登录 code 无效

- **需求来源**：gateway-service-prd.md / Story 1
- **优先级**：高
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `POST /api/v1/user/login`，请求体 `{"code": "invalid_code"}`
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 401 或 400
  - 响应体 `{code: 10001, message: "invalid wechat code", trace_id: "..."}`

### TC-ERR-007 微信服务不可用

- **需求来源**：gateway-service-prd.md / Story 1
- **优先级**：高
- **前置条件**：模拟微信 API 不可用（超时或连接失败）
- **测试步骤**：
  1. 发送 `POST /api/v1/user/login`，请求体 `{"code": "some_code"}`
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 503
  - 响应体 `{code: 10002, message: "wechat service unavailable", trace_id: "..."}`

### TC-ERR-008 Refresh Token 过期

- **需求来源**：gateway-service-prd.md / Story 3、功能 3
- **优先级**：高
- **前置条件**：持有已过期的 Refresh Token
- **测试步骤**：
  1. 发送 `POST /api/v1/user/refresh`，请求体 `{"refresh_token": "<expired_refresh_token>"}`
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 401
  - 响应体 `{code: 20004, message: "refresh token expired", trace_id: "..."}`

### TC-ERR-009 Refresh Token 无效（伪造）

- **需求来源**：gateway-service-prd.md / Story 3、功能 3
- **优先级**：高
- **前置条件**：持有伪造的 Refresh Token
- **测试步骤**：
  1. 发送 `POST /api/v1/user/refresh`，请求体 `{"refresh_token": "fake_token_string"}`
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 401
  - 响应体 `{code: 20005, message: "invalid refresh token", trace_id: "..."}`

### TC-ERR-010 下游 User Service 不可用

- **需求来源**：gateway-service-prd.md / 错误码规范、gRPC Code 映射
- **优先级**：高
- **前置条件**：模拟 User Service 停机（etcd 注册但服务不响应）
- **测试步骤**：
  1. 调用 `GET /api/v1/user/me`（携带有效 Token）
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 503
  - 响应体包含 `code: 5xxxx` 范围的错误码
  - 响应体包含 `trace_id`

### TC-ERR-011 下游 Content Service 不可用

- **需求来源**：gateway-service-prd.md / 错误码规范、Content Service 路由表
- **优先级**：高
- **前置条件**：模拟 Content Service 停机
- **测试步骤**：
  1. 调用 `GET /api/v1/content/posts`（不携带 Token）
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 503
  - 响应体包含 `code: 5xxxx` 范围的错误码
  - 响应体包含 `trace_id`

### TC-ERR-012 etcd 服务发现失败

- **需求来源**：gateway-service-prd.md / 集成依赖
- **优先级**：高
- **前置条件**：etcd 不可用或下游服务未注册
- **测试步骤**：
  1. 启动 Gateway 但 etcd 不可达
  2. 观察启动行为
- **预期结果**：
  - Gateway 启动失败或进入降级模式（根据实现策略）
  - 记录错误日志

### TC-ERR-013 请求体格式错误（非 JSON）

- **需求来源**：gateway-service-prd.md / 功能 1 协议转换
- **优先级**：中
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `POST /api/v1/user/login`，Content-Type 为 `application/json`，但请求体为非 JSON 格式（如 `plain text`）
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 400
  - 响应体包含错误提示和 `trace_id`

### TC-ERR-014 请求体缺少必填字段

- **需求来源**：gateway-service-prd.md / Story 1
- **优先级**：中
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `POST /api/v1/user/login`，请求体 `{}`（缺少 `code` 字段）
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 400
  - 响应体包含参数错误码（4xxxx 段）

### TC-ERR-015 访问不存在的路由

- **需求来源**：gateway-service-prd.md / 功能 1 路由聚合
- **优先级**：低
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `GET /api/v1/nonexistent/endpoint`
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 404
  - 响应体包含错误提示

### TC-ERR-016 不支持的 HTTP 方法

- **需求来源**：gateway-service-prd.md / 功能 1 路由聚合
- **优先级**：低
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `PATCH /api/v1/user/login`（login 仅支持 POST）
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 405 或 404

### TC-ERR-017 ListCommentReplies 父评论不存在

- **需求来源**：gateway-v1.2-prd.md / FR-2
- **优先级**：中
- **前置条件**：评论 ID 99999 不存在
- **测试步骤**：
  1. 发送 `GET /api/v1/content/comments/99999/replies`，携带有效 Token
  2. 检查响应状态码和响应体
- **预期结果**：
  - HTTP 状态码 404
  - 响应体透传 Content Service 的 NOT_FOUND 错误
  - 响应体包含 `trace_id`

### TC-ERR-018 ListCommentReplies :id 非法值（非数字）

- **需求来源**：gateway-v1.2-prd.md / FR-2
- **优先级**：中
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `GET /api/v1/content/comments/abc/replies`，携带有效 Token
  2. 检查响应状态码
- **预期结果**：
  - HTTP 状态码 400
  - 响应体包含参数错误码

### TC-ERR-019 ListCommentReplies :id 为 0 或负数

- **需求来源**：gateway-v1.2-prd.md / FR-2
- **优先级**：中
- **前置条件**：Gateway 服务已启动
- **测试步骤**：
  1. 发送 `GET /api/v1/content/comments/0/replies`，携带有效 Token
  2. 发送 `GET /api/v1/content/comments/-1/replies`，携带有效 Token
  3. 检查响应状态码
- **预期结果**：
  - 两个请求均返回 HTTP 400
  - 响应体包含参数错误码

### TC-ERR-020 刷新 Token 时 Token 被撤销（双花防护）

- **需求来源**：gateway-service-prd.md / Story 3、功能 3
- **优先级**：中
- **前置条件**：持有有效 Refresh Token
- **测试步骤**：
  1. 使用 Refresh Token A 成功刷新一次，获取新 Token
  2. 再次使用 Refresh Token A 尝试刷新
  3. 检查第二次请求的响应
- **预期结果**：
  - 第二次请求返回 401
  - 响应体 `{code: 20005, message: "invalid refresh token"}`（Refresh Token 已轮换/失效）

---

## 4. 状态转换测试（TC-ST）

### TC-ST-001 完整登录到访问受保护资源流程

- **需求来源**：gateway-service-prd.md / Story 1、Story 2
- **优先级**：高
- **前置条件**：Gateway、User Service、微信 API 均可用
- **测试步骤**：
  1. 发送 `POST /api/v1/user/login`，请求体 `{"code": "valid_wx_code"}`
  2. 记录返回的 `access_token`
  3. 使用 `access_token` 调用 `GET /api/v1/user/me`
  4. 检查用户信息返回
- **预期结果**：
  - 步骤 1 返回 `access_token`
  - 步骤 3 返回用户信息，包含 `user_id`、`school_id` 等

### TC-ST-002 登录 → 绑定学校 → 多租户隔离验证

- **需求来源**：gateway-service-prd.md / Story 1、Story 4
- **优先级**：高
- **前置条件**：新用户未绑定学校
- **测试步骤**：
  1. 使用新用户登录获取 Token
  2. 使用该 Token 访问 `POST /api/v1/content/posts`（写接口）
  3. 检查被拒绝（403 campus not bound）
  4. 调用 `PUT /api/v1/user/campus` 绑定学校
  5. 再次访问 `POST /api/v1/content/posts`
  6. 检查 gRPC metadata 中的 `school_id`
- **预期结果**：
  - 步骤 3：返回 403 `{code: 20006}`
  - 步骤 6：写接口正常访问，gRPC metadata 中包含绑定的 `school_id`

### TC-ST-003 Access Token 过期 → Refresh Token 换新 → 继续访问

- **需求来源**：gateway-service-prd.md / Story 3
- **优先级**：高
- **前置条件**：已登录获取 Access Token 和 Refresh Token
- **测试步骤**：
  1. 使用有效 Access Token 调用受保护接口（成功）
  2. 等待 Access Token 过期
  3. 使用过期 Access Token 调用受保护接口（失败 401）
  4. 使用 Refresh Token 调用 `POST /api/v1/user/refresh`，获取新 Access Token
  5. 使用新 Access Token 调用受保护接口（成功）
- **预期结果**：
  - 步骤 1：200 OK
  - 步骤 3：401 `{code: 20002, message: "token expired"}`
  - 步骤 4：200 OK，返回新 `access_token`
  - 步骤 5：200 OK

### TC-ST-004 Refresh Token 轮换状态

- **需求来源**：gateway-service-prd.md / Story 3
- **优先级**：高
- **前置条件**：已登录获取 Access Token 和 Refresh Token
- **测试步骤**：
  1. 使用 Refresh Token A 调用 `/api/v1/user/refresh`，获取新 Token
  2. 使用 Refresh Token A 再次调用 `/api/v1/user/refresh`
  3. 检查第二次请求结果
- **预期结果**：
  - 步骤 1：成功
  - 步骤 2：失败（Refresh Token A 已失效），返回 401

### TC-ST-005 限流触发 → 等待恢复 → 请求恢复

- **需求来源**：gateway-service-prd.md / Story 7
- **优先级**：高
- **前置条件**：限流器默认配置
- **测试步骤**：
  1. 从同一 IP 快速发送 250 个请求（触发限流）
  2. 等待 1 秒
  3. 再从同一 IP 发送 100 个请求
- **预期结果**：
  - 步骤 1：部分请求返回 429
  - 步骤 3：全部成功（令牌桶已恢复）

### TC-ST-006 帖子发布 → 评论 → 二级回复 → 查询回复列表

- **需求来源**：gateway-service-prd.md / Content Service 路由表、gateway-v1.2-prd.md
- **优先级**：高
- **前置条件**：已登录并绑定学校的用户
- **测试步骤**：
  1. 调用 `POST /api/v1/content/posts` 发布帖子，记录 `post_id`
  2. 调用 `POST /api/v1/content/posts/{post_id}/comments`，内容 `{"content": "一级评论"}`，记录 `comment_id`
  3. 调用 `POST /api/v1/content/posts/{post_id}/comments`，内容 `{"content": "二级回复", "parent_id": comment_id}`
  4. 调用 `GET /api/v1/content/comments/{comment_id}/replies`
- **预期结果**：
  - 步骤 1：帖子创建成功
  - 步骤 2：一级评论创建成功
  - 步骤 3：二级回复创建成功
  - 步骤 4：返回的回复列表包含步骤 3 创建的回复

### TC-ST-007 点赞 → 取消点赞 → 再次点赞

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：中
- **前置条件**：已登录并绑定学校的用户，帖子存在
- **测试步骤**：
  1. 调用 `POST /api/v1/content/posts/123/like` 点赞
  2. 调用 `DELETE /api/v1/content/posts/123/like` 取消点赞
  3. 调用 `POST /api/v1/content/posts/123/like` 再次点赞
- **预期结果**：
  - 步骤 1：点赞成功
  - 步骤 2：取消点赞成功
  - 步骤 3：再次点赞成功

### TC-ST-008 帖子发布 → 编辑 → 删除

- **需求来源**：gateway-service-prd.md / Content Service 路由表
- **优先级**：中
- **前置条件**：已登录并绑定学校的用户，是帖子的作者
- **测试步骤**：
  1. 调用 `POST /api/v1/content/posts` 发布帖子，记录 `post_id`
  2. 调用 `GET /api/v1/content/posts/{post_id}` 查看帖子详情
  3. 调用 `PUT /api/v1/content/posts/{post_id}` 修改标题
  4. 调用 `DELETE /api/v1/content/posts/{post_id}` 删除帖子
  5. 再次调用 `GET /api/v1/content/posts/{post_id}` 验证已删除
- **预期结果**：
  - 步骤 2：返回原帖子信息
  - 步骤 3：修改成功
  - 步骤 4：删除成功
  - 步骤 5：返回 404（帖子不存在）

### TC-ST-009 多租户数据隔离验证

- **需求来源**：gateway-service-prd.md / Story 4、功能 4
- **优先级**：高
- **前置条件**：两个不同学校的用户 A（学校 X）和 B（学校 Y）
- **测试步骤**：
  1. 用户 A 调用 `POST /api/v1/content/posts` 发布帖子
  2. 用户 A 调用 `GET /api/v1/content/posts` 查看帖子列表
  3. 用户 B 调用 `GET /api/v1/content/posts` 查看帖子列表
  4. 检查两个列表的内容差异
- **预期结果**：
  - 步骤 2：用户 A 的列表包含自己发布的帖子
  - 步骤 3：用户 B 的列表不包含用户 A 的帖子（数据隔离）

### TC-ST-010 Token 签名密钥变更后旧 Token 全部失效

- **需求来源**：gateway-service-prd.md / 功能 2、技术约束
- **优先级**：低
- **前置条件**：使用密钥 K1 签发 Token，然后将 Gateway 配置切换为密钥 K2
- **测试步骤**：
  1. 使用密钥 K1 签发的旧 Token 调用受保护接口
  2. 使用密钥 K2 签发的新 Token 调用受保护接口
- **预期结果**：
  - 步骤 1：返回 401 `{code: 20003, message: "invalid token"}`
  - 步骤 2：正常访问

---

## 5. 需求-测试用例覆盖矩阵

| 需求编号 | 需求描述 | 来源 | 覆盖的测试用例 |
|---------|---------|------|---------------|
| 功能 1 | 路由聚合与协议转换 | PRD v1.1 | TC-F-001, TC-F-008, TC-F-009, TC-ERR-015, TC-ERR-016 |
| 功能 2 | 白名单路由 + JWT 鉴权 | PRD v1.1 | TC-F-003, TC-F-004, TC-F-005, TC-F-006, TC-E-001, TC-E-002, TC-ERR-001, TC-ERR-002, TC-ERR-003, TC-ERR-004, TC-ERR-005, TC-E-003, TC-ST-010 |
| 功能 3 | Refresh Token 机制 | PRD v1.1 | TC-F-004, TC-F-033, TC-E-004, TC-ERR-008, TC-ERR-009, TC-ERR-020, TC-ST-003, TC-ST-004 |
| 功能 4 | 多租户隔离 school_id 注入 | PRD v1.1 | TC-F-007, TC-F-034, TC-F-035, TC-F-036, TC-ST-002, TC-ST-009 |
| 功能 5 | IP 级限流 | PRD v1.1 | TC-F-019, TC-F-020, TC-F-031, TC-E-005, TC-E-006, TC-E-009, TC-ST-005 |
| 功能 6 | 全链路追踪 OTel | PRD v1.1 | TC-F-021, TC-F-022, TC-F-023, TC-F-024, TC-F-025, TC-F-026 |
| 功能 7 | 统一错误响应格式 | PRD v1.1 | TC-F-029, TC-F-030, TC-F-031, TC-F-032, TC-E-010 |
| 功能 8 | CORS 处理 | PRD v1.1 | TC-F-027, TC-F-028 |
| 功能 9 | 优雅停机 | PRD v1.1 | TC-F-037, TC-F-038, TC-F-039 |
| Story 1 | 微信登录 | PRD v1.1 | TC-F-002, TC-F-003, TC-ERR-006, TC-ERR-007, TC-ERR-014, TC-ST-001 |
| Story 2 | JWT 鉴权访问受保护资源 | PRD v1.1 | TC-F-005, TC-F-006, TC-ERR-001, TC-ERR-002, TC-ERR-003, TC-ERR-004, TC-ERR-005, TC-ST-001 |
| Story 3 | Refresh Token 无感续期 | PRD v1.1 | TC-F-004, TC-F-033, TC-ERR-008, TC-ERR-009, TC-ERR-020, TC-ST-003, TC-ST-004 |
| Story 4 | 多租户隔离 school_id | PRD v1.1 | TC-F-007, TC-F-034, TC-F-035, TC-F-036, TC-ST-002, TC-ST-009 |
| Story 5 | 跨域访问 CORS | PRD v1.1 | TC-F-027, TC-F-028 |
| Story 6 | 全链路追踪 | PRD v1.1 | TC-F-021, TC-F-022, TC-F-023, TC-F-024, TC-F-025, TC-F-026 |
| Story 7 | IP 级限流 | PRD v1.1 | TC-F-019, TC-F-020, TC-E-005, TC-E-006, TC-E-009, TC-ST-005 |
| Story 8 | 优雅停机 | PRD v1.1 | TC-F-037, TC-F-038, TC-F-039 |
| User 路由 | User Service 5 个路由 | PRD v1.1 | TC-F-002, TC-F-004, TC-F-005, TC-F-040, TC-F-041 |
| Content 路由 | Content Service 11 个路由 | PRD v1.1 | TC-F-008 ~ TC-F-018, TC-ST-006 ~ TC-ST-009 |
| FR-1 | 透传 parent_id | PRD v1.2 | TC-F-042, TC-F-043, TC-E-007 |
| FR-2 | ListCommentReplies 路由 | PRD v1.2 | TC-F-044, TC-F-045, TC-E-008, TC-ERR-017, TC-ERR-018, TC-ERR-019 |
| 错误码规范 | 统一错误码分段 | PRD v1.1 | TC-F-029, TC-F-030, TC-F-031, TC-F-032, TC-E-010 |
| gRPC Code 映射 | gRPC → HTTP 状态码 | PRD v1.1 | TC-E-010, TC-ERR-010, TC-ERR-011 |
| Metadata 透传 | user-id/user-role/school-id | PRD v1.1 | TC-F-006, TC-F-007 |

---

## 测试用例统计

| 类别 | 编号前缀 | 用例数量 |
|------|---------|---------|
| 功能测试 | TC-F | 45 |
| 边界测试 | TC-E | 10 |
| 异常测试 | TC-ERR | 20 |
| 状态转换测试 | TC-ST | 10 |
| **合计** | — | **85** |

## 优先级分布

| 优先级 | 用例数量 | 占比 |
|-------|---------|------|
| 高 | 55 | 64.7% |
| 中 | 26 | 30.6% |
| 低 | 4 | 4.7% |

## 需求覆盖率

- PRD v1.1：9 个核心功能 + 8 个用户故事 + 5 个 User 路由 + 11 个 Content 路由 + 错误码规范 + gRPC 映射 + Metadata 透传 = **全部覆盖**
- PRD v1.2：FR-1（parent_id 透传）+ FR-2（ListCommentReplies）= **全部覆盖**
