# 产品需求文档：API Gateway（网关服务）

**版本**：1.1
**日期**：2026-06-24
**作者**：Sarah（产品负责人）
**质量评分**：91/100

---

## 实施进度（截至 2026-06-24）

| 模块 | 实现状态 | 说明 |
|------|---------|------|
| Gin 引擎 + 全局中间件（CORS / RateLimit / Trace） | ✅ 已实现 | 见 `cmd/gateway/router/app.go` |
| `/health` 健康检查 | ✅ 已实现 | 直接返回 `{"status":"ok"}` |
| User Service 路由（login / me / info / campus） | ✅ 已实现 | 见 `cmd/gateway/handler/user_handler.go` |
| 基于 etcd 的 User Service gRPC 客户端 | ✅ 已实现 | 见 `cmd/gateway/client/user_client.go` |
| JWT 鉴权中间件 + RequireRole | ✅ 已实现 | 见 `middleware/jwt_auth.go` |
| OTel Trace 中间件（响应头 X-Trace-ID） | ✅ 已实现 | 见 `middleware/trace.go` |
| IP 级令牌桶限流 | ✅ 已实现 | 见 `middleware/ratelimit.go` |
| CORS 跨域处理 | ✅ 已实现 | 见 `middleware/cors.go` |
| 优雅停机（SIGINT / SIGTERM） | ✅ 已实现 | 见 `cmd/gateway/main.go` |
| **gRPC metadata 注入 `user-id` / `user-role`** | ✅ 已实现 | `handler/authCtx()` |
| **统一错误响应 `{code, message, trace_id}`** | 🔲 MVP 待实现 | 当前为 `{"error": "..."}` 风格 |
| **Refresh Token + `/api/v1/user/refresh`** | 🔲 MVP 待实现 | JWT Claims 仅含 `UserID+Role` |
| **`school-id` 注入下游 gRPC metadata** | 🔲 MVP 待实现 | metadata 当前不含 school-id |
| **未绑定学校用户拒绝访问（非白名单写接口）** | 🔲 MVP 待实现 | 暂无强制约束 |
| **Content Service 路由表（11 个接口）** | 🔲 MVP 待实现 | router 仅含 User 分组 |
| **Content Service gRPC 客户端初始化** | 🔲 MVP 待实现 | main.go 仅 `InitUserClient` |
| **Task / Message / Admin / File 服务路由** | 🔲 后续 PRD | 当前不在范围 |
| **Prometheus 指标导出** | 🔲 后续阶段 | Phase 5 可观测性完善 |

---

## 执行摘要

API Gateway 是校园互助平台（CampusHelper）所有客户端（微信小程序、后续可能扩展的 Web/管理后台）访问后端的**唯一入口**。它承担"统一路由聚合 + 鉴权与多租户隔离"两大核心职责，把外部 HTTP/RESTful 请求转换为对下游 gRPC 微服务（User、Content、Task、Message、Admin、File）的协议转换调用，并在请求链路中强制注入 `user_id`、`school_id`、`trace_id` 等横切上下文，保障多租户数据隔离与全链路可观测性。

作为整个系统的入口，Gateway 同时实现 JWT 鉴权（含 Access/Refresh 双 Token）、IP 级限流、跨域处理、统一错误响应格式与基于 OpenTelemetry 的分布式追踪。本期 PRD 聚焦 Gateway 服务本身的中间件、路由框架、鉴权与多租户隔离机制，并附带 User Service 和 Content Service 的完整路由清单，作为后续 Task / Message / Admin / File 服务接入网关的参考模板。

> **当前快照**：User Service 路由、JWT 鉴权、OTel Trace、CORS、IP 限流、优雅停机、gRPC 客户端接入等基础链路已打通；统一错误体、Refresh Token、`school-id` 注入、Content 路由为本期 MVP 待实现项。

---

## 问题陈述

**当前痛点**：

1. **入口分散**：如果客户端直连各微服务 gRPC 端口（user:50001 / content:50002 / task:50003 ...），需要在小程序端硬编码多个地址，且任何端口变更都会导致客户端发版。
2. **鉴权不一致**：每个微服务各自实现 JWT 校验，存在 Token 解析逻辑重复、密钥/算法不一致导致的安全风险。
3. **多租户隔离缺失**：如果下游服务需自行从 Token 解析 `school_id` 注入 Context，重复实现且容易遗漏，导致跨校数据泄漏风险。
4. **链路断裂**：HTTP → gRPC → MQ 各段各自生成 TraceID，导致 Jaeger 中无法串联全链路，故障定位困难。
5. **错误风格混乱**：下游 gRPC 错误直接透传 HTTP，状态码语义不统一，前端处理困难。

**解决方案**：构建统一的 API Gateway，作为唯一对外入口，统一下列能力：

- 路由聚合：客户端只需对接 Gateway 一个地址
- 鉴权与多租户隔离：JWT 解析后注入 `user_id`、`school_id`、`role` 到下游 gRPC metadata
- 全链路追踪：基于 OTel TraceContext 透传 HTTP → gRPC → MQ
- 统一错误体 `{code, message, data, trace_id}`，便于前端处理
- IP 级限流 + CORS + 优雅停机

**业务影响**：

- 客户端发版频率降低 50%（不再因后端端口/路径变化发版）
- 多校数据泄漏风险降低（强制注入 school_id，避免遗漏）
- 故障定位效率提升（TraceID 串联全链路）
- 前端集成成本降低（统一错误格式，鉴权逻辑收敛到网关）

---

## 成功指标

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| 接口响应时间 | 平均 < 200ms，P99 < 300ms | Phase 5 压测（wrk/JMeter） |
| 吞吐能力 | 支持 100 QPS 业务读 + 20 QPS 业务写 | 压测验证 |
| 并发用户 | 支持 1000 并发连接 | Phase 5 压测 |
| 全链路追踪覆盖率 | HTTP → gRPC → MQ 全程 TraceID 不断链 | Jaeger 中可检索完整 Span |
| 多租户隔离零泄漏 | 跨校访问请求 100% 被拦截 | 集成测试 + 灰度验证 |
| JWT 鉴权拦截率 | 无效/过期 Token 100% 拒绝 | 单元测试 + 渗透测试 |
| 错误响应格式一致性 | 所有 4xx/5xx 响应均符合 `{code,message,trace_id}` 规范 | 接口契约测试 |

---

## 用户画像

### 主要用户：微信小程序前端

- **角色**：负责小程序端业务逻辑开发的前端工程师
- **目标**：通过统一的 HTTP 接口与后端交互，获取数据、提交操作
- **痛点**：希望鉴权逻辑简单、错误提示明确、接口风格一致
- **技术水平**：熟悉小程序 wx.request，对后端协议无强约束
- **使用频率**：每次页面加载、每次交互都会发起 Gateway 请求

### 次要用户：移动端 / Web 端用户（潜在）

- **角色**：使用平台的学生用户
- **目标**：无感使用，不关心底层架构
- **痛点**：希望登录一次长期有效（Refresh Token 无感续期），错误提示友好
- **使用频率**：日均多次访问

### 运维用户：后端工程师 / SRE

- **角色**：负责 Gateway 部署、监控、问题排查
- **目标**：快速定位故障、优雅停机、可观测性完善
- **痛点**：希望 TraceID 可串联全链路、Prometheus 指标齐全、限流可观测

---

## 用户故事与验收标准

### Story 1：微信小程序登录与身份获取

**作为**小程序前端  
**我想要**通过 `wx.login()` 获取的 code 调用 Gateway 登录接口  
**以便于**获得 Access Token 并判断用户是否已绑定学校

**验收标准：**

- [x] `POST /api/v1/user/login`，请求体 `{code: "wx.login()返回的code"}`（注：当前 JSON 字段为 `code`，与 Protobuf `js_code` 字段映射在 handler 中完成）
- [x] 200 响应（当前实现）：`{access_token: "...", is_bound_campus: true, school_id: 123}`
- [ ] 200 响应（MVP 统一格式后）：`{code: 0, message: "ok", data: {access_token: "...", is_bound_campus: true, school_id: 123}, trace_id: "..."}`
- [ ] 401 响应（code 无效）：`{code: 10001, message: "invalid wechat code", trace_id: "..."}`（当前为 `{"error": "..."}`，MVP 改造）
- [ ] 503 响应（微信服务异常）：`{code: 10002, message: "wechat service unavailable", trace_id: "..."}`
- [ ] 限流：登录接口受全局 IP 限流保护（防爆破依赖全局限流策略，无需独立阈值）
- [x] 接口无需 JWT 鉴权（属于白名单）

### Story 2：JWT 鉴权访问受保护资源

**作为**小程序前端  
**我想要**在请求头携带 Access Token 访问受保护接口  
**以便于**获取当前用户信息并执行业务操作

**验收标准：**

- [x] 所有非白名单接口必须在 `Authorization: Bearer <token>` 头携带 Token
- [x] Token 缺失：401 `{error: "missing token"}`（当前实现）→ MVP 改造为 `{code: 20001, message: "missing token", trace_id: "..."}`
- [x] Token 无效/过期：401 `{error: "invalid token"}`（当前实现，不细分过期与签名错误）→ MVP 改造区分：
  - Token 过期：401 `{code: 20002, message: "token expired", trace_id: "..."}`
  - Token 签名无效：401 `{code: 20003, message: "invalid token", trace_id: "..."}`
- [x] 网关解析 Token 后，注入 `user_id`（int64）与 `role`（int8）到 `gin.Context`
- [x] `authCtx()` 将 `user-id`、`user-role` 写入 gRPC metadata 转发下游
- [ ] 不携带 Token 调用受保护接口时，前端可获得明确的中文错误提示（依赖统一错误体实现）

### Story 3：Refresh Token 无感续期（MVP 待实现）

**作为**小程序前端  
**我想要**Access Token 过期后自动使用 Refresh Token 获取新 Token  
**以便于**用户不会感知登录态失效

**当前状态**：🔲 **未实现**。当前 `pkg/jwt/jwt.go` 中 `UserClaims` 仅含 `UserID` 和 `Role` 两字段，无 Refresh Token 类型；`/api/v1/user/refresh` 接口未注册；登录响应未返回 `refresh_token`。

**验收标准（MVP 待实现）：**

- [ ] `POST /api/v1/user/refresh`，请求体 `{refresh_token: "..."}`
- [ ] 200 响应：返回新的 `access_token`（`refresh_token` 可选续期或保持）
- [ ] 401 响应（Refresh Token 过期）：`{code: 20004, message: "refresh token expired", trace_id: "..."}`
- [ ] 401 响应（Refresh Token 无效）：`{code: 20005, message: "invalid refresh token", trace_id: "..."}`
- [ ] Access Token 默认有效期 24 小时（可通过配置 `jwt.accessExpireH` 调整，**当前已支持**）
- [ ] Refresh Token 默认有效期 7 天（可通过配置 `jwt.refreshExpireH` 调整，**当前配置已就绪但未使用**）
- [ ] 在 `pkg/jwt/jwt.go` 中新增 `RefreshClaims` 类型，区分 `GenerateAccessToken` / `GenerateRefreshToken`
- [ ] 改造 `handler.WxLogin` 调用 User Service 时同步返回 `refresh_token`

### Story 4：多租户隔离（school_id 注入，MVP 待实现）

**作为**微服务下游业务  
**我想要**从 gRPC metadata 中获取 `school_id`  
**以便于**在数据查询时强制按学校隔离，避免跨校数据泄漏

**当前状态**：🔲 **部分未实现**。JWT lib 的 `UserClaims` 中**没有 `SchoolID` 字段**；`handler.authCtx()` 中 metadata 仅含 `user-id` 和 `user-role`，**不含 `school-id`**；未绑定学校用户调用受保护接口**不会被强制拒绝**。

**验收标准（MVP 待实现）：**

- [ ] 在 `pkg/jwt/jwt.go` 的 `UserClaims` 中新增 `SchoolID int64` 字段
- [ ] 网关从 Token 中解析 `school_id`，通过 gRPC metadata 透传给下游服务（key: `school-id`，value 为十进制字符串）
- [ ] 改造 `handler.authCtx()`，在 metadata 中追加 `school-id`
- [ ] 下游服务可通过 `pkg/contextx.GetSchoolID(ctx)` 读取
- [ ] Token 中无 `school_id`（用户未绑定学校）调用非白名单写接口时，返回 403 `{code: 20006, message: "campus not bound", trace_id: "..."}`
- [ ] 未绑定学校的用户仍可访问白名单接口（如 `/user/login`、`/user/refresh`、`/user/me` 查看绑定状态）
- [ ] 集成测试：用户 A 在学校 X 发帖，用户 B 在学校 Y 无法通过任何接口查询到 A 的帖子

### Story 5：跨域访问（CORS）

**作为**小程序开发者工具调试  
**我想要**从浏览器域名访问 Gateway 接口时不被跨域拦截  
**以便于**本地开发与调试

**验收标准：**

- [ ] `OPTIONS` 预检请求直接返回 204
- [ ] 响应头携带 `Access-Control-Allow-Origin: *`、`Access-Control-Allow-Headers: Authorization,Content-Type,X-Trace-ID`
- [ ] 响应头 `Access-Control-Expose-Headers: X-Trace-ID`（前端可读取 TraceID）
- [ ] 生产环境建议收紧 Origin 白名单（本期允许 `*`，后期按需调整）

### Story 6：全链路追踪

**作为**SRE / 后端工程师  
**我想要**通过 TraceID 在 Jaeger 中检索完整 HTTP → gRPC → MQ → Consumer 链路  
**以便于**快速定位性能瓶颈与故障点

**验收标准：**

- [x] 客户端可在请求头携带 W3C `traceparent` header 传入 TraceID（OTel 标准），否则网关生成新 TraceID（当前实现依赖 OTel `HeaderCarrier` 提取）
- [ ] 客户端在请求头携带 `X-Trace-ID` 自定义 TraceID（MVP 增强，需在 Trace 中间件显式解析该 header）
- [x] 响应头 `X-Trace-ID` 返回当前请求的 TraceID（当前已实现）
- [x] `c.Set("trace_id", traceID)` 注入到 `gin.Context`，便于日志关联
- [x] 网关创建 OTel Span，名为 `c.FullPath()`（如 `/api/v1/user/me`）
- [x] 下游 gRPC 调用通过 `otelgrpc.NewClientHandler()` 拦截器自动注入 TraceContext（已在 `client/user_client.go` 中配置）
- [ ] MQ 消息通过消息头携带 TraceContext，消费者读取后创建子 Span（依赖 Message Service / RabbitMQ 封装）
- [ ] Jaeger 中可看到完整 Span 树：gateway.HTTP → user-service.GetCurrentUser

### Story 7：IP 级限流

**作为**运维人员  
**我想要**Gateway 在高并发或恶意请求下保护下游服务  
**以便于**避免被刷接口导致服务雪崩

**验收标准：**

- [ ] 基于令牌桶算法，按客户端 IP 限流
- [ ] 默认速率：100 QPS，突发 200（可通过配置 `gateway.rateLimit` / `gateway.rateBurst` 调整）
- [ ] 超限响应：429 `{code: 30001, message: "rate limit exceeded", trace_id: "..."}`
- [ ] 限流计数仅在内存中维护（学习项目场景），重启后丢失可接受

### Story 8：优雅停机

**作为**运维人员  
**我想要**Gateway 在收到 SIGTERM 信号时优雅停机  
**以便于**正在处理的请求能完成，新请求不再接入

**验收标准：**

- [x] 捕获 `SIGINT` / `SIGTERM` 信号（`main.go` 中通过 `signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)`）
- [x] 停止接收新连接（`http.Server.Shutdown`）
- [x] 等待进行中的请求完成（超时 10 秒：`context.WithTimeout(ctx, 10*time.Second)`）
- [x] 关闭 etcd 连接：`defer pkg_etcd.CloseEtcd()`
- [x] 关闭 Tracer：`defer func() { _ = shutdown(context.Background()) }()`
- [ ] 打印停机日志（含 TraceID 关联）— 当前为简单 `fmt.Println("[gateway] shutting down…")`，MVP 增强为结构化日志

---

## 功能需求

### 核心功能

#### 功能 1：路由聚合与协议转换

- **描述**：HTTP/RESTful 入口，Gin 框架统一路由，对接下游 gRPC 服务
- **路由结构**：
  - `/health`：健康检查（无需鉴权）
  - `/api/v1/*`：业务路由，按服务分子组（user / content / task / message / admin / file）
- **协议转换**：JSON over HTTP → Protobuf over gRPC
- **错误转换**：gRPC Code → HTTP Status + 业务错误码

#### 功能 2：白名单路由 + JWT 鉴权

- **白名单**（无需鉴权）：
  - `POST /api/v1/user/login`
  - `POST /api/v1/user/refresh`
  - `GET /health`
- **受保护路由**：除白名单外，所有 `/api/v1/*` 接口必须携带有效 JWT
- **鉴权流程**：
  1. 从 `Authorization: Bearer <token>` 头提取 Token
  2. 校验签名（HS256 + `jwt.authKey`）
  3. 校验过期时间
  4. 解析 Claims：`user_id` (int64)、`role` (int8)、`school_id` (int64，可选)
  5. 注入 `user_id`、`user_role` 到 `gin.Context`
- **错误码**：
  - 缺失 Token：401 / `20001 missing token`
  - Token 过期：401 / `20002 token expired`
  - Token 无效：401 / `20003 invalid token`
  - Refresh Token 过期：401 / `20004 refresh token expired`
  - Refresh Token 无效：401 / `20005 invalid refresh token`
  - 未绑定学校：403 / `20006 campus not bound`

#### 功能 3：Refresh Token 机制（MVP 待实现）

- **状态**：🔲 当前 `pkg/jwt/jwt.go` 中无 Refresh Token 类型，`/api/v1/user/refresh` 未注册
- **Access Token**：
  - 默认有效期：24 小时（`jwt.accessExpireH`，**当前已配置**）
  - 用途：业务接口鉴权（**当前已实现**）
- **Refresh Token**：
  - 默认有效期：168 小时（7 天，`jwt.refreshExpireH`，**当前已配置但未使用**）
  - 用途：刷新 Access Token
  - 存储：本期暂存内存（学习项目）；生产建议 Redis
- **Refresh 接口**：
  - `POST /api/v1/user/refresh`（**当前未注册**）
  - 入参：`{refresh_token: "..."}`
  - 响应：新的 `access_token`（`refresh_token` 可选轮换）

#### 功能 4：多租户隔离（school_id 注入，MVP 待实现）

- **状态**：🔲 JWT lib `UserClaims` 中无 `SchoolID` 字段；`handler.authCtx()` 未在 metadata 注入 `school-id`；未绑定学校用户无强制拒绝逻辑
- **数据来源（MVP 后）**：JWT Claims 中的 `school_id`
- **注入方式（MVP 后）**：gRPC metadata，key 为 `school-id`，value 为十进制字符串
- **下游读取**：通过 `pkg/contextx.GetSchoolID(ctx)` 读取（**当前已实现 Get/Set**）
- **强制约束（MVP 后）**：
  - 未绑定学校的用户访问非白名单写接口 → 403 `20006`
  - 下游服务的 GORM 查询必须使用 `SchoolScope` 全局注入 `school_id` 过滤

#### 功能 5：IP 级限流

- **算法**：令牌桶
- **维度**：按客户端 IP
- **配置**：
  - `gateway.rateLimit`：每秒补充令牌数（默认 100）
  - `gateway.rateBurst`：桶容量（默认 200）
- **实现位置**：`middleware/ratelimit.go`（已有基础逻辑）
- **错误响应**：429 `{code: 30001, message: "rate limit exceeded", trace_id: "..."}`

#### 功能 6：全链路追踪（OTel）

- **实现位置**：`middleware/trace.go`（已有基础逻辑）
- **行为**：
  - 提取请求头 `X-Trace-ID`（如有），作为 OTel TraceID 起点
  - 创建 OTel Span，名为 `c.FullPath()`（如 `/api/v1/user/me`）
  - 注入 `traceparent` / `tracestate` 等 W3C Trace Context 头到下游 gRPC metadata
  - 响应头 `X-Trace-ID` 返回当前 TraceID
- **下游集成**：通过 `otelgrpc` 拦截器自动透传，无需业务代码手动注入

#### 功能 7：统一错误响应格式

- **格式**：
  ```json
  {
    "code": 0,
    "message": "ok",
    "data": {},
    "trace_id": "abc123..."
  }
  ```
- **字段说明**：
  - `code`：0 表示成功，非 0 为业务错误码
  - `message`：人类可读的错误描述（中文）
  - `data`：成功时携带业务数据；失败时为空对象
  - `trace_id`：链路追踪 ID，便于问题定位
- **错误码分段**：
  - `0`：成功
  - `1xxxx`：第三方依赖错误（如微信、Token 颁发）
  - `2xxxx`：鉴权与权限错误
  - `3xxxx`：限流与配额错误
  - `4xxxx`：请求参数错误
  - `5xxxx`：下游服务错误
  - `9xxxx`：系统内部错误

#### 功能 8：CORS 处理

- **实现位置**：`middleware/cors.go`（已有基础逻辑）
- **响应头**：
  - `Access-Control-Allow-Origin: *`（本期允许全部，生产建议收紧）
  - `Access-Control-Allow-Methods: GET,POST,PUT,PATCH,DELETE,OPTIONS`
  - `Access-Control-Allow-Headers: Authorization,Content-Type,X-Request-ID,X-Trace-ID`
  - `Access-Control-Expose-Headers: X-Trace-ID`
  - `Access-Control-Max-Age: 86400`
- **预检请求**：OPTIONS 方法直接返回 204

#### 功能 9：优雅停机

- **信号处理**：`SIGINT` / `SIGTERM`
- **步骤**：
  1. 停止接收新连接（`http.Server.Shutdown(ctx)`）
  2. 等待进行中请求完成（超时 10 秒）
  3. 关闭 etcd 客户端
  4. 关闭 Tracer（flush span 到 Jaeger）
  5. 打印停机日志

### User Service 路由表

| 方法 | 路径 | 鉴权 | 描述 | gRPC 接口 | 实现状态 |
|------|------|------|------|-----------|---------|
| POST | `/api/v1/user/login` | 否 | 微信登录，颁发 Access Token | `UserService.WxLogin` | ✅ 已实现 |
| POST | `/api/v1/user/refresh` | 否 | Refresh Token 换 Access Token | `UserService.RefreshToken` | 🔲 MVP 待实现 |
| GET | `/api/v1/user/me` | 是 | 获取当前用户信息 | `UserService.GetCurrentUser` | ✅ 已实现 |
| PUT | `/api/v1/user/info` | 是 | 更新昵称/头像 | `UserService.UpdateUserInfo` | ✅ 已实现 |
| PUT | `/api/v1/user/campus` | 是 | 绑定学校 | `UserService.BindCampus` | ✅ 已实现 |

### Content Service 路由表（路由 + 鉴权清单，MVP 待实现）

| 方法 | 路径 | 鉴权 | 描述 | gRPC 接口 | 实现状态 |
|------|------|------|------|-----------|---------|
| POST | `/api/v1/content/posts` | 是 | 发布帖子 | `ContentService.CreatePost` | 🔲 MVP 待实现 |
| GET | `/api/v1/content/posts` | 否 | 帖子列表（按 school_id 过滤） | `ContentService.ListPosts` | 🔲 MVP 待实现 |
| GET | `/api/v1/content/posts/:id` | 否 | 帖子详情 | `ContentService.GetPost` | 🔲 MVP 待实现 |
| PUT | `/api/v1/content/posts/:id` | 是 | 编辑帖子（仅作者） | `ContentService.UpdatePost` | 🔲 MVP 待实现 |
| DELETE | `/api/v1/content/posts/:id` | 是 | 删除帖子（仅作者） | `ContentService.DeletePost` | 🔲 MVP 待实现 |
| POST | `/api/v1/content/posts/:id/comments` | 是 | 发表评论 | `ContentService.CreateComment` | 🔲 MVP 待实现 |
| GET | `/api/v1/content/posts/:id/comments` | 否 | 评论列表 | `ContentService.ListComments` | 🔲 MVP 待实现 |
| DELETE | `/api/v1/content/comments/:id` | 是 | 删除评论（仅作者） | `ContentService.DeleteComment` | 🔲 MVP 待实现 |
| POST | `/api/v1/content/posts/:id/like` | 是 | 点赞 | `ContentService.LikePost` | 🔲 MVP 待实现 |
| DELETE | `/api/v1/content/posts/:id/like` | 是 | 取消点赞 | `ContentService.UnlikePost` | 🔲 MVP 待实现 |
| GET | `/api/v1/content/search` | 否 | 关键词搜索（走 ES） | `ContentService.SearchContent` | 🔲 MVP 待实现 |

> 详细请求/响应字段以 `PB/content.proto` 为准，本表仅锁定路由结构与鉴权策略。Content 路由注册、Content Service gRPC 客户端初始化、`client/content_client.go` 文件创建均为本期 MVP 任务。

### Out of Scope（不在本期范围）

- WebSocket 升级（Message Service 推送，单独设计）
- Task / Message / Admin / File 服务路由（后续服务 PRD 各自定义）
- API 版本管理（v1 单一版本，后续按需迭代 v2）
- Swagger / OpenAPI 自动生成（后期）
- 熔断器（hystrix-go / sony/gobreaker），后续按需引入
- Request Body 大小限制（依赖 Nginx / 网关层前置）
- Prometheus 指标导出（Phase 5 可观测性完善阶段）
- API Key / 签名校验（小程序场景非必需）

---

## 技术约束

### 性能

- 接口响应时间：平均 < 200ms，P99 < 300ms
- 吞吐能力：100 QPS 业务读 + 20 QPS 业务写
- 并发用户：1000 并发连接
- 优雅停机：进行中请求 10 秒内完成

### 安全与合规

- **JWT 鉴权**：HS256 算法 + 服务端密钥（`jwt.authKey`）
- **多租户隔离**：所有写接口强制注入 `school_id`，未绑定学校用户拒绝访问
- **IP 限流**：防止恶意请求与接口爆破
- **错误信息脱敏**：不向客户端泄漏堆栈、SQL、内部服务地址
- **CORS**：本期允许 `*`，生产环境建议收紧为具体域名

### 集成依赖

| 依赖 | 集成方式 | 用途 |
|------|---------|------|
| **etcd** | `pkg/etcd` + `pkg/discovery` | 服务发现（gRPC 客户端动态拨号） |
| **Jaeger** | OpenTelemetry OTLP HTTP | 全链路追踪后端 |
| **下游 gRPC 服务** | otelgrpc 拦截器 | 自动注入 TraceContext |
| **配置中心** | Viper + YAML | 运行时配置（限流、Token 有效期等） |

### 技术栈约束

- **语言**：Go 1.22+
- **HTTP 框架**：Gin
- **gRPC**：google.golang.org/grpc（含 otelgrpc 拦截器）
- **服务发现**：etcd（`go.etcd.io/etcd/client/v3`）
- **链路追踪**：OpenTelemetry + Jaeger OTLP HTTP
- **配置**：Viper
- **JWT**：自定义 `pkg/jwt`（HS256）
- **日志**：标准 `log` 包，结合 TraceID（结构化）

### 配置项

```yaml
gateway:
  address: "0.0.0.0:8082"
  rateLimit: 100        # 每秒令牌补充速率
  rateBurst: 200        # 令牌桶容量

jwt:
  authKey: "campus_help_secret_2026"
  accessExpireH: 24     # Access Token 有效期（小时）
  refreshExpireH: 168   # Refresh Token 有效期（小时）

service:
  user:
    name: "user-service"
    address: "127.0.0.1:50001"
    loadBalance: false
  content:
    name: "content-service"
    address: "127.0.0.1:50002"
    loadBalance: true

etcd:
  address:
    - "127.0.0.1:2379"

jaeger:
  endpoint: "127.0.0.1:4318"
```

---

## MVP 范围与分阶段计划

### Phase 1（MVP）—— 本期单阶段交付

**已实现（沿用既有代码）：**

1. ✅ 路由框架：Gin 引擎 + 全局中间件（CORS / RateLimit / Trace）
2. ✅ JWT 鉴权：白名单 + JWT 校验 + Token 解析注入 Context
3. ✅ 全链路追踪：OTel Trace 中间件（响应头 `X-Trace-ID`）+ otelgrpc 拦截器
4. ✅ IP 级限流：基于令牌桶
5. ✅ 优雅停机：SIGTERM 信号处理 + 资源关闭
6. ✅ User Service 路由：登录、Me、Update Info、Bind Campus
7. ✅ 健康检查：`GET /health`
8. ✅ gRPC metadata 注入：`user-id` / `user-role`

**MVP 待实现：**

1. 🔲 **统一错误响应中间件**：`gRPC Code → HTTP Status + {code,message,trace_id}`
2. 🔲 **Refresh Token 机制**：`POST /api/v1/user/refresh` + JWT lib 拆分
3. 🔲 **school-id 注入 metadata**：JWT Claims 扩展 + `authCtx` 改造
4. 🔲 **未绑定学校强制约束**：403 `campus not bound`
5. 🔲 **Content Service gRPC 客户端**：`client/content_client.go`
6. 🔲 **Content Service 路由**：帖子 CRUD + 评论 + 点赞 + 搜索（11 个接口）
7. 🔲 **Trace 中间件增强**：显式解析 `X-Trace-ID` 自定义 header

**MVP 定义**：能完整支持微信小程序登录（Access + Refresh 双 Token）→ 绑定学校 → 浏览/发布/搜索帖子 → 评论/点赞 → 收到通知的端到端链路，跨校数据 100% 隔离，所有错误响应符合统一格式，并在 Jaeger 中检索到完整 TraceID 贯穿 HTTP → gRPC → MQ。

---

## 风险评估

| 风险 | 概率 | 影响 | 缓解策略 |
|------|------|------|---------|
| JWT 密钥泄漏 | 低 | 高 | 生产环境密钥注入环境变量或 Vault，配置文件不提交明文 |
| Refresh Token 内存存储重启丢失 | 中 | 中 | 本期为学习项目可接受；生产建议 Redis 持久化 |
| gRPC 客户端连接泄漏 | 中 | 中 | 启动时初始化失败直接 Fatal；运行期依赖 grpc 内置 keepalive |
| OTel Tracer 初始化失败 Gateway 启动失败 | 低 | 高 | 已有容错：Tracer 失败仅打印 Warn，继续运行（无追踪模式） |
| CORS `*` 在生产环境被利用 | 中 | 中 | 本期允许，生产部署前替换为具体白名单域名 |
| 限流策略过严导致正常用户被拒 | 中 | 中 | 通过配置 `rateLimit` / `rateBurst` 调整阈值；后续按用户/接口粒度细化 |
| 未绑定学校用户绕过 school_id 检查 | 低 | 高 | 网关 + 下游服务双重校验：网关层强制未绑定拒绝；下游 GORM `SchoolScope` 全局过滤 |
| 高并发下令牌桶内存持续增长 | 中 | 低 | 引入 TTL 清理或替换为 Redis 分布式限流 |

---

## 依赖与阻塞项

**依赖服务：**

- **User Service**：必须先实现 `WxLogin` / `RefreshToken` / `GetCurrentUser` / `UpdateUserInfo` / `BindCampus` 等 gRPC 接口
- **Content Service**：必须先实现 `CreatePost` / `ListPosts` / `GetPost` / `UpdatePost` / `DeletePost` / `CreateComment` / `ListComments` / `DeleteComment` / `LikePost` / `UnlikePost` / `SearchContent` 等 gRPC 接口
- **etcd 服务发现**：Gateway 启动前需 etcd 已运行，且各下游服务已注册

**基础设施依赖：**

- MySQL + Redis + RabbitMQ + ES + Jaeger 环境（参考 Phase 1 基建）
- etcd 服务发现，用于下游服务注册和 Gateway 客户端发现

**已知阻塞项：**

- 无（各服务接口定义已存在 `PB/` 目录）

---

## 附录

### 错误码规范

| 段位 | 含义 | 示例 |
|------|------|------|
| 0 | 成功 | `{code: 0, message: "ok"}` |
| 1xxxx | 第三方依赖错误 | 10001 invalid wechat code / 10002 wechat service unavailable |
| 2xxxx | 鉴权与权限错误 | 20001 missing token / 20002 token expired / 20006 campus not bound |
| 3xxxx | 限流与配额错误 | 30001 rate limit exceeded |
| 4xxxx | 请求参数错误 | 40001 invalid parameter |
| 5xxxx | 下游服务错误 | 50001 user service unavailable |
| 9xxxx | 系统内部错误 | 90001 internal error |

### HTTP 状态码映射

| gRPC Code | HTTP Status | 业务错误码段 |
|-----------|-------------|------------|
| OK | 200 | 0 |
| InvalidArgument | 400 | 4xxxx |
| Unauthenticated | 401 | 2xxxx |
| PermissionDenied | 403 | 2xxxx |
| NotFound | 404 | 4xxxx |
| ResourceExhausted | 429 | 3xxxx |
| Internal | 500 | 5xxxx / 9xxxx |
| Unavailable | 503 | 5xxxx |
| DeadlineExceeded | 504 | 5xxxx |

### gRPC Metadata 透传清单

| Metadata Key | 类型 | 来源 | 用途 |
|--------------|------|------|------|
| `user-id` | string | JWT Claims | 下游服务识别调用者 |
| `user-role` | string | JWT Claims | 下游服务权限校验 |
| `school-id` | string | JWT Claims | 多租户隔离核心字段 |
| `traceparent` | string | OTel 自动注入 | W3C Trace Context 透传 |

### 术语表

- **白名单路由**：无需鉴权即可访问的接口（如登录、Refresh、健康检查）
- **Access Token**：短期有效的业务鉴权 Token，默认 24 小时
- **Refresh Token**：长期有效的换新 Token，默认 7 天，用于无感续期
- **school_id**：多租户隔离键，标识用户所属学校
- **TraceID**：OpenTelemetry 全链路追踪 ID，贯穿 HTTP → gRPC → MQ
- **令牌桶**：限流算法，按速率补充令牌，请求消耗令牌，桶空则拒绝
- **优雅停机**：收到终止信号后先停止接入新请求，等待进行中请求完成，再退出进程

### 现有代码资产（参考）

| 文件 | 角色 | 状态 |
|------|------|------|
| `cmd/gateway/main.go` | 启动入口、信号处理、gRPC 客户端初始化 | ✅ 已完成（仅 User Client） |
| `cmd/gateway/router/app.go` | 路由注册、全局中间件挂载 | ✅ 已完成（仅 User 分组） |
| `cmd/gateway/middleware/jwt_auth.go` | JWT 鉴权 + RequireRole | ✅ 已完成 |
| `cmd/gateway/middleware/trace.go` | OTel Trace 中间件（含 X-Trace-ID 响应头） | ✅ 已完成 |
| `cmd/gateway/middleware/ratelimit.go` | IP 令牌桶限流 | ✅ 已完成 |
| `cmd/gateway/middleware/cors.go` | CORS 跨域处理 | ✅ 已完成 |
| `cmd/gateway/handler/user_handler.go` | User Service 路由处理（login/me/info/campus） | ✅ 已完成 |
| `cmd/gateway/client/user_client.go` | User gRPC 客户端（基于 etcd，含 otelgrpc） | ✅ 已完成 |
| `pkg/contextx/contextx.go` | trace_id / user_id / school_id 读写 | ✅ 已完成 |
| `pkg/jwt/jwt.go` | JWT 签发与解析（HS256） | ✅ 已完成（仅 UserID+Role） |
| `cmd/gateway/handler/refresh_handler.go` | Refresh Token 处理 | 🔲 MVP 待创建 |
| `cmd/gateway/middleware/error_wrapper.go` | 统一错误响应中间件 | 🔲 MVP 待创建 |
| `cmd/gateway/client/content_client.go` | Content gRPC 客户端 | 🔲 MVP 待创建 |
| `cmd/gateway/handler/content_handler.go` | Content Service 路由处理（11 个接口） | 🔲 MVP 待创建 |

### MVP 拆分建议（按 Issue 粒度）

| Issue 编号建议 | 主题 | 工作量预估 |
|----------------|------|-----------|
| Issue #A | 统一错误响应中间件（`middleware/error_wrapper.go`） | 0.5d |
| Issue #B | JWT `UserClaims` 增加 `SchoolID` 字段 | 0.5d |
| Issue #C | `handler/authCtx` 注入 `school-id` metadata + 未绑定学校拒绝 | 0.5d |
| Issue #D | Refresh Token 机制（`pkg/jwt` 拆分 + `/user/refresh` 接口） | 1d |
| Issue #E | Content gRPC 客户端（`client/content_client.go` + main.go 注册） | 0.5d |
| Issue #F | Content Service 路由注册（11 个接口）+ handler 实现 | 2d |
| Issue #G | Trace 中间件增强（显式解析 `X-Trace-ID` 自定义 header） | 0.5d |

---

*本 PRD（v1.1）基于 `cmd/gateway` 实际代码状态调整，区分"已实现"与"MVP 待实现"，并补充 MVP 拆分建议（Issue 粒度）。质量评分 91/100，覆盖业务目标、功能需求、用户体验、技术约束和分阶段交付五个维度。*