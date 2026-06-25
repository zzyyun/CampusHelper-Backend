# 产品需求文档：Gateway v1.2 — Content Service v2.1 对齐

**版本**：1.2
**日期**：2026-06-25
**作者**：Sarah（产品负责人）
**质量评分**：91/100
**前置版本**：v1.1（Content Service 11 个接口路由，#22）

---

## 执行摘要

Content Service v2.1 新增了 `CreateCommentRequest.parent_id` 字段和 `ListCommentReplies` RPC。但 Gateway 层尚未同步——当前 `createCommentReq` DTO 缺少 `parent_id` 字段（客户端无法创建二级回复），且无 `ListCommentReplies` HTTP 路由（客户端无法查询某条评论下的回复列表）。

本 PRD 定义 Gateway 层的最低改动：透传 `parent_id` + 新增 1 条读路由，彻底打通"二级评论"的客户端访问链路。

---

## 问题陈述

**当前状态**：
| 问题 | 影响 |
|------|------|
| `createCommentReq` 无 `parent_id` | 客户端无法创建二级回复（`parent_id` 永远为 0） |
| 无 `ListCommentReplies` 路由 | 客户端无法查询一级评论下的回复列表 |

**解决方案**：
- `createCommentReq` 新增可选字段 `parent_id`（默认 0 = 一级评论）
- `CreateComment` handler 将该字段透传到 gRPC 请求
- 新增 `GET /api/v1/content/comments/:id/replies` 路由 + `ListCommentReplies` handler

**改动量**：2 个文件、~40 行代码、0 新增依赖。

---

## 成功指标

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| 客户端可创建二级回复 | `POST /comments` 传 `parent_id` → 成功返回 `comment_id` | 手动测试 + 单测 |
| 客户端可查询回复列表 | `GET /comments/:id/replies` → 返回 replies 数组 | 手动测试 + 单测 |
| 旧客户端无回归 | `POST /comments` 不传 `parent_id` → 行为不变（一级评论） | 单测 |

---

## 用户故事

### Story 1：客户端创建二级回复

**作为** 小程序前端开发者
**我想要** 在 `POST /api/v1/content/comments` 请求体中传 `parent_id` 字段
**以便于** 用户可以对某条一级评论进行回复

**验收标准：**
- [ ] `createCommentReq` 新增 `parent_id` 字段（JSON: `parent_id`, int64, 可选）
- [ ] 不传 `parent_id` 时默认为 0 → gRPC `ParentId=0`（一级评论，向后兼容）
- [ ] 传 `parent_id=123` 时 → gRPC `ParentId=123`
- [ ] handler 单测覆盖：parent_id=0、parent_id>0

### Story 2：客户端查询某条评论的回复列表

**作为** 小程序前端开发者
**我想要** 调用 `GET /api/v1/content/comments/:id/replies?cursor=&page_size=`
**以便于** 展示一级评论下的所有二级回复

**验收标准：**
- [ ] 新路由 `GET /api/v1/content/comments/:id/replies`（JWT 鉴权，不强制 school 绑定）
- [ ] 响应格式：`{ "replies": [...], "next_cursor": "", "has_more": false }`
- [ ] 支持游标分页（`cursor` / `page_size` query 参数）
- [ ] `:id` 必须 > 0，否则返回 400
- [ ] handler 单测覆盖：正常返回、游标翻页、父评论不存在

---

## 功能需求

### FR-1：透传 parent_id

- **文件**：`cmd/gateway/handler/content_handler.go`
- **DTO 变更**：
  ```go
  type createCommentReq struct {
      PostID   int64  `json:"post_id" binding:"required,min=1"`
      Content  string `json:"content" binding:"required,min=1,max=500"`
      ParentID int64  `json:"parent_id"` // 新增：0=一级，>0=二级回复
  }
  ```
- **Handler 变更**：在 `CreateCommentRequest` 中增加 `ParentId: req.ParentID`
- **错误处理**：`parent_id` 由 Content Service 做业务校验（存在/未删除/必须一级/同帖子），Gateway 不做额外校验

### FR-2：ListCommentReplies 路由

- **路由**：`GET /api/v1/content/comments/:id/replies`
- **中间件**：`JWTAuth`（读路由，不强制 `RequireSchoolBound`）
- **Handler**：
  ```go
  func ListCommentReplies(c *gin.Context) {
      // 解析 :id → parentCommentID
      // 构造 ListCommentRepliesRequest{SchoolId, ParentCommentId, Pagination}
      // 调用 client.ContentClient.ListCommentReplies(ctx, req)
      // 返回 {replies, next_cursor, has_more}
  }
  ```
- **边缘场景**：
  - `:id` 非法（≤0、非数字） → 400
  - `page_size` > 50 → 服务端截断（Content Service 已实现）
  - 父评论不存在 → 服务端返回 NOT_FOUND，Gateway 透传 GRPCErrorResponse

### Out of Scope

- ❌ 不新增中间件
- ❌ 不修改 Content Service proto（已在 v2.1 完成）
- ❌ 不修改 User Service 路由

---

## 技术约束

### 性能
- `ListCommentReplies` handler 与现有 `ListComments` 同量级（≤ 150ms P95）

### 安全
- JWT 鉴权（复用现有中间件）
- `parent_id` 不做 Gateway 层校验（透传，由 Content Service 校验）

### 集成
- 复用 `client.ContentClient`（gRPC 客户端已含 `ListCommentReplies` 方法）
- 路由注册在现有 `/api/v1/content` 路由组

---

## MVP 范围与分期

### Phase 1（本 PRD）

- `createCommentReq` 新增 `parent_id`
- `CreateComment` handler 透传 `parent_id`
- `GET /api/v1/content/comments/:id/replies` 路由 + handler
- 单测覆盖

### Out of Scope for Later
- 批量查询回复（一次拉取多条一级评论的回复）
- WebSocket 实时评论通知

---

## 风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 旧客户端未更新 → parent_id 不传 | 高 | 无 | 默认 0，向后兼容 |
| `ListCommentReplies` gRPC 调用超时 | 低 | 低 | 复用现有 timeout + gRPC 中间件 |

---

## 附录

### 路由变更对比

```
v1.1 (当前)                          v1.2 (本 PRD)
─────────────────────────────────    ─────────────────────────────────
POST /content/comments               POST /content/comments          ← req 新增 parent_id
DELETE /content/comments/:id         DELETE /content/comments/:id    ← 不变
GET /posts/:id/comments              GET /posts/:id/comments         ← 不变
                                     GET /comments/:id/replies       ← 新增
```

### 参考

- Content Service v2.1 PRD：`docs/content-service-v2-prd.md`
- Content Service v2.1 Changelog：`docs/content-service-v2-changelog.md`
- Gateway v1.1 PR：#22（11 个 Content 路由）

---

*本 PRD 通过 1 轮迭代式需求对话生成，质量评分 91/100。改动量极小（2 文件、~30 行），无风险、无阻塞依赖。*