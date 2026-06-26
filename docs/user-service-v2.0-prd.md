# Product Requirements Document: User Service v2.0 管理员升级

**Version**: 2.0
**Date**: 2026-06-26
**Author**: Sarah (Product Owner)
**Quality Score**: 92/100

---

## Executive Summary

User Service v2.0 在现有学生端能力基础上，新增管理员体系。通过复用已有的 Role/Permission 基础设施，为学生会志愿者提供**内容审核**和**用户管理**两大核心能力。本版本不新建独立 Admin Service，所有管理员能力直接嵌入 User Service，通过 Gateway 层的 `RequireRole` 中间件实现权限保护。

目标用户是各高校学生会志愿者——非技术背景、低频操作、但需要明确的权限边界。第一版聚焦"能用"，后续迭代再补充数据看板等运营工具。

---

## Problem Statement

**Current Situation**: 当前平台拥有 6 个微服务、完整的社区/任务/消息功能，但**没有任何管理后台**。违规内容（广告、敏感信息）无法被及时处理，违规用户无法被约束。平台面临被学校禁用、社区秩序失控的风险。

**Proposed Solution**: 在 User Service 现有架构上新增 4 个管理 RPC + 1 个内容审核回调，利用已有的 `RoleAdmin`/`RoleSuperAdmin` 角色体系和 `Can()` 权限引擎，快速交付管理后台能力。审核流程采用"关键词自动过滤 + 人工复审"模式，平衡效率与安全。

**Business Impact**: 
- 内容安全风险从"零管控"提升为"自动过滤+人工复审"
- 为每个学校提供自助管理能力，降低运营团队人工干预成本
- 为后续全面向学校推广扫清合规障碍

---

## Success Metrics

**Primary KPIs:**
- **审核效率**: 管理员从收到审核通知到完成操作的平均时间 < 2 小时
- **违规覆盖率**: 关键词过滤拦截率 ≥ 80%（减少人工审核量）
- **误封率**: 被封禁用户的申诉成功率 < 5%（封禁准确度）

**Validation**: 上线的第一个月，通过操作审计日志统计各学校管理员的操作频率和审核时效。

---

## User Personas

### Primary: 学生志愿者管理员（school_admin）
- **Role**: 每校 1-2 名，学生会成员兼任
- **Goals**: 维护本校社区秩序，及时处理违规内容和用户
- **Pain Points**: 非技术背景，操作界面需要简单直观；权限边界必须清晰（不能越权操作其他学校）
- **Technical Level**: Novice — 需要简洁的列表+操作按钮式交互

### Secondary: 超级管理员（super_admin）
- **Role**: 开发团队/平台运营方
- **Goals**: 跨校管理、任命/撤销 school_admin、处理重大违规
- **Pain Points**: 需要全局视角，不受 school_id 隔离限制
- **Technical Level**: Advanced

---

## User Stories & Acceptance Criteria

### Story 1: 封禁违规用户

**As a** school_admin
**I want to** 封禁发布违规内容的用户
**So that** 该用户无法继续登录和使用平台

**Acceptance Criteria:**
- [ ] admin 可以封禁本校 role=student 的用户（status 从 normal 改为 banned）
- [ ] admin **不能**封禁本校其他 admin 或 super_admin
- [ ] 封禁后用户无法通过 WxLogin/RefreshToken 登录（返回"账号已被封禁"错误）
- [ ] 封禁后用户已有的帖子/任务**保留可见**（不联动下架）
- [ ] 支持解封操作（banned → normal）
- [ ] super_admin 可以封禁任意学校、任意角色（除 super_admin 自身）

### Story 2: 用户列表查询 + 角色管理

**As a** school_admin
**I want to** 查看本校用户列表并按条件筛选
**So that** 快速定位需要处理的用户

**Acceptance Criteria:**
- [ ] 按 school_id 筛选（admin 只能看本校，super_admin 可跨校）
- [ ] 按 role 筛选（student / admin / super_admin）
- [ ] 按 status 筛选（normal / banned / deleted）
- [ ] 支持关键词搜索（nickname 模糊匹配）
- [ ] 支持游标分页（cursor-based，复用 ListSchools 的 Base64+JSON 模式）
- [ ] super_admin 可以提升学生为 admin（SetUserRole）
- [ ] super_admin 可以撤销 admin 为 student
- [ ] role 变更记录到审计日志

### Story 3: 内容审核入口

**As a** school_admin
**I want to** 查看待审核内容列表并执行审核操作
**So that** 违规内容被及时处理

**Acceptance Criteria:**
- [ ] 查看本校状态为 `pending_review` 的内容列表（按时间倒序）
- [ ] 支持分页（cursor-based）
- [ ] 审核操作：通过（pending_review → published）或 驳回（pending_review → rejected）
- [ ] 驳回时记录驳回原因
- [ ] 审核完成后内容状态变更通过 MQ 事件通知 Content Service
- [ ] 操作记录写入审计日志

### Story 4: 操作审计日志

**As a** super_admin
**I want to** 查看管理员操作记录
**So that** 追溯所有管理操作，防止权限滥用

**Acceptance Criteria:**
- [ ] 记录字段：操作人 ID、目标 ID、操作类型、操作时间、详情
- [ ] 操作类型：`ban_user`, `unban_user`, `set_role`, `audit_content`
- [ ] 存储于 MySQL（admin_audit_logs 表）
- [ ] 支持按操作人、操作类型、时间范围筛选查询
- [ ] 自动保留 90 天，超期可清理

---

## Functional Requirements

### Core Features

**Feature 1: BanUser / UnbanUser**
- Description: 管理员封禁/解封用户。封禁后用户无法通过登录接口获取 token，RefreshToken 也拒绝被封禁用户。
- User flow:
  1. 管理员在用户列表中选择目标用户
  2. 点击"封禁"按钮，确认操作
  3. 系统调用 `BanUser` RPC，DB 更新 status=banned
  4. 清除该用户 Redis 缓存
  5. 写入审计日志
- Edge cases:
  - admin 封禁同校 admin → 拒绝，返回 20007 "权限不足"
  - 封禁已封禁用户 → 幂等，返回成功
  - 封禁不存在的用户 → 返回 "用户不存在"
  - 被封禁用户调用 WxLogin → 返回 20008 "账号已被封禁"
- Error handling:
  - 参数不合法（user_id ≤ 0）→ `InvalidArgument`
  - 权限不足 → `PermissionDenied`

**Feature 2: ListUsers + SetUserRole**
- Description: 管理员按条件查询用户列表，super_admin 可以设置用户角色。
- User flow:
  1. 管理员进入用户管理页面
  2. 系统默认加载本校用户列表（最新注册在前）
  3. 可选筛选：按状态/角色/关键词搜索
  4. super_admin 可在列表中直接切换用户角色
- Edge cases:
  - admin 跨校查询 → 强制覆写 school_id 为本校（Gateway 层注入）
  - super_admin 跨校查询 → 可传任意 school_id 或 0（查全部）
  - super_admin 不能修改自己的角色
  - 角色提升后自动清除 Redis 缓存
- Error handling:
  - 无效的 role 值 → `InvalidArgument`
  - 权限不足 → `PermissionDenied`

**Feature 3: ListContentForAudit + AuditContent**
- Description: 管理员获取待审核内容并执行审核。审核流程：关键词自动过滤（Content Service 发布时）→ 被拦截的进入 pending_review → 管理员调用 AuditContent 通过或驳回。
- User flow:
  1. 用户发布内容 → Content Service 检查关键词
  2. 命中敏感词 → 状态设为 pending_review（不可见）
  3. 管理员查看待审核列表
  4. 审核通过 → Content 状态变为 published
  5. 审核驳回 → Content 状态变为 rejected，记录驳回原因
- Edge cases:
  - 审核已被他人审核的内容 → Content Service 检查状态，拒绝重复操作
  - 审核不存在的 content → 返回 "内容不存在"
  - Content Service 不可用 → gRPC 返回 Unavailable，前端提示稍后重试
- Error handling:
  - 参数不合法 → `InvalidArgument`
  - 权限不足 → `PermissionDenied`

**Feature 4: 操作审计日志**
- Description: 所有管理员关键操作自动记录。日志表与 User Service 同库。
- Edge cases:
  - 日志表写入失败 → 不应阻塞主操作（log 警告 + 继续）
  - 日志保留 90 天，`StartCleanupTask` 定期清理

### Out of Scope
- **数据看板/统计报表** — 后续 v2.1 迭代
- **内容关键词管理** — 由 Content Service v3.0 负责
- **申诉/工单系统** — 后续独立 PRD
- **批量操作** — 第一版仅支持单用户操作
- **RBAC 权限引擎全量接入** — model.Can() 保留但仅在 Gateway 中间件层使用 RequireRole，不启用细粒度 Permission 校验
- **学校 CRUD 管理** — 后续迭代

---

## Technical Constraints

### Performance
- 用户列表查询: 单次 ≤ 200ms (索引 `school_id + status`)
- 审核列表查询: 单次 ≤ 200ms (走 Content Service gRPC)
- 封禁/解封: 单次 ≤ 100ms
- 所有管理接口支持 100 QPS（管理端并发量低）

### Security
- Gateway 层新增 `/api/v1/admin/*` 路由组
- 路由组统一使用 `JWT + RequireRole(RoleAdmin)` 中间件
- 角色提升接口 `SetUserRole` 使用 `RequireRole(RoleSuperAdmin)`
- 所有管理接口的 school_id 由 Gateway 注入（admin 不可伪造）
- super_admin 跨校查询时，school_id=0 表示"全部"

### Integration
- **[Content Service]**: 调用 `ListContentByStatus(status=pending_review, school_id)` 和 `UpdateContentStatus(content_id, status, reason)` — 需 Content Service 新增对应 RPC
- **[Gateway]**: 新增 admin 路由组 + RequireRole 中间件接入
- **[MySQL]**: User Service 现有 DB，新增 `admin_audit_logs` 表
- **[Redis]**: 封禁用户时清除 `user:id:{id}` 缓存，使 token 刷新时拉取最新状态

### Technology Stack
- Go 1.22+, GORM, gRPC + Protobuf
- 审计日志：MySQL (与 User Service 同库)
- 认证：现有 JWT Claims 中已包含 `role` 字段，**无需修改**

### 现有 JWT Role 基础设施（已就绪，仅需接线）

```
┌─ 签发侧 (User Service) ─────────────────────────────────────────┐
│  GenerateAccessToken(userID, schoolID, role, secret, expireH)    │
│  → UserClaims { UserID, SchoolID, Role }  ← role 已经在 Token 里 │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─ 解析侧 (Gateway JWTAuth) ──────────────────────────────────────┐
│  ParseAccessToken(token) → claims                                │
│  c.Set("user_id",   claims.UserID)                               │
│  c.Set("user_role", claims.Role)       ← 注入 gin.Context        │
│  c.Set("user_school_id", claims.SchoolID)                        │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─ 鉴权侧 (Gateway RequireRole) ───────────────────────────────────┐
│  RequireRole(minRole int8)  ← 中间件已定义但未接入路由              │
│  → 读取 c.Get("user_role") 与 minRole 比较                        │
│  → 不足返回 20007 "权限不足"                                      │
│  v2.0 工作: 接入 /api/v1/admin/* 路由组即可                       │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─ 透传侧 (Gateway authCtx) ───────────────────────────────────────┐
│  authCtx(c) → grpc metadata:                                     │
│    "user-id"   ← c.Get("user_id")                                │
│    "user-role" ← c.Get("user_role")   ← 下游服务可读取            │
│    "school-id" ← c.Get("user_school_id")                         │
└──────────────────────────────────────────────────────────────────┘
```

**结论**: Role 字段的签发 → 解析 → 鉴权 → 透传四环已全部打通。v2.0 的核心工程工作是**新增 6 个管理 RPC + Gateway Admin 路由组接线**，而非基础认证改造。

---

## MVP Scope & Phasing

### Phase 1: MVP (Required for Initial Launch)
- [x] **BanUser / UnbanUser** — 封禁/解封用户 RPC + 登录拒绝逻辑
- [x] **ListUsers** — 用户列表查询（筛选 + 分页 + 搜索）
- [x] **SetUserRole** — super_admin 角色管理
- [x] **ListContentForAudit + AuditContent** — 内容审核入口（需 Content Service 配合）
- [x] **操作审计日志** — 基础记录 + 90 天保留
- [x] **Gateway Admin 路由组** — RequireRole 中间件接入

**MVP Definition**: 学生会志愿者可以查看本校用户列表、封禁违规学生、审核待复审内容，所有操作记录可追溯。

### Phase 2: Enhancements (Post-Launch)
- [ ] **简易数据看板** — 各学校用户数、发帖量、任务数总览
- [ ] **批量操作** — 批量封禁、批量审核
- [ ] **申诉处理** — 用户申诉 → 管理员复审

### Future Considerations
- [ ] 关键词配置管理界面
- [ ] 违规用户等级（警告/禁言/永久封禁）
- [ ] RBAC 细粒度权限（按 Permission 拆分，非仅 Role 级别）
- [ ] 学校 CRUD 管理
- [ ] ES 全文搜索替代 MySQL LIKE

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation Strategy |
|------|------------|--------|---------------------|
| **Content Service 审核接口未实现** | Med | High | Phase 1 需同步完成 Content Service 的 `ListContentByStatus` 和 `UpdateContentStatus` RPC；若阻塞可先上线用户管理功能 |
| **关键词过滤准确率不足** | Med | Med | 先使用简单关键词匹配，允许管理员人工复审兜底；后续接入 NLP 敏感词模型 |
| **学生志愿者误操作** | High | Low | 审计日志可追溯；封禁支持解封；封禁仅影响登录不发帖，不删除数据 |
| **super_admin 权限过大** | Low | High | super_admin 数量极小（1-2人），操作全部有审计日志；后续可增加操作二次确认 |
| **审核效率不达标** | Med | Med | 关键词过滤减少人工量；后续可增加审核通知（MQ → 小程序模板消息） |

---

## Dependencies & Blockers

**Dependencies:**
- **Content Service**: 需要新增 `ListContentByStatus` 和 `UpdateContentStatus` 两个 RPC，用于管理员审核内容。配合新增 `pending_review` 状态。
- **JWT Middleware**: 已有 `RequireRole` 中间件（`cmd/gateway/middleware/jwt_auth.go:51`），无需修改，直接接线即可。

**Known Blockers:**
- 无已知技术阻塞。Role/Permission 模型、RequireRole 中间件、UserStatus 枚举均已实现，只是未接入生产。

---

## Appendix

### Glossary
- **school_admin**: 学校级管理员，role=admin(2)，只能管理本校学生
- **super_admin**: 超级管理员，role=super_admin(3)，可跨校管理所有用户和角色
- **pending_review**: 内容状态，表示被关键词过滤拦截，等待人工审核
- **审计日志**: admin_audit_logs 表记录的所有管理员操作

### New RPCs Summary (User Service)

| RPC | Request | Response | Min Role | 说明 |
|-----|---------|----------|----------|------|
| `BanUser` | user_id, reason | BaseResponse | admin | 封禁用户 |
| `UnbanUser` | user_id | BaseResponse | admin | 解封用户 |
| `ListUsers` | school_id, role, status, keyword, page_size, cursor | ListUsersResponse | admin | 用户列表 |
| `SetUserRole` | user_id, role | BaseResponse | super_admin | 设置角色 |
| `ListContentForAudit` | school_id, page_size, cursor | ListContentResponse | admin | 待审内容 |
| `AuditContent` | content_id, action, reason | BaseResponse | admin | 审核操作 |

### References
- [用户服务 v1.1 PRD](docs/user-service-prd.md)
- [User Service 当前代码](cmd/user/)
- [Gateway 路由注册](cmd/gateway/router/app.go)
- [JWT 认证中间件](cmd/gateway/middleware/jwt_auth.go)
- [现存 RBAC 模型](cmd/user/model/user.go)

---

*This PRD was created through interactive requirements gathering with quality scoring to ensure comprehensive coverage of business, functional, UX, and technical dimensions.*