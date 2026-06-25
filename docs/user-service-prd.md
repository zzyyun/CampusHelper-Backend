# 产品需求文档：User Service v1.1 — 基础设施完善

**版本**：1.1
**日期**：2026-06-25
**作者**：Sarah（产品负责人）
**质量评分**：91/100
**前置版本**：v1.0（5 个 RPC 已实现）

---

## 执行摘要

User Service 已完成微信登录、Token 续签、学校绑定、用户查询、信息更新 5 个核心 RPC 的开发，但在生产就绪度上存在三个短板：

1. **调试日志泄露到生产环境**：`userIDFromCtx()` 中的 6 行 `fmt.Printf` 会在每次 gRPC 调用时将客户的 metadata 信息打印到 stdout，既是安全风险也污染日志
2. **零测试覆盖**：整个 `cmd/user/` 目录不存在任何 `_test.go` 文件，无法保障后续修改不引入回归
3. **学校查询能力被阻塞**：`ListSchools` DAO 已实现但无 RPC 暴露，Content Service 和 Task Service 无法跨服务查询学校信息

本期不做新增业务功能，聚焦**基础设施完善**——消除隐患、建立质量基线、打通阻塞点。

---

## 问题陈述

**当前状态**：

| 问题 | 严重度 | 影响 |
|------|--------|------|
| `fmt.Printf` 调试日志 (`userIDFromCtx`) | 🔴 高 | 所有 gRPC 请求的 metadata 被打印到 stdout，生产环境泄露内部请求信息 |
| 零测试覆盖 | 🔴 高 | 无法安全重构；回归风险高 |
| `ListSchools` DAO 已写但无 RPC | 🟡 中 | Content Service 无法跨服务查询学校；需在 Content 侧写死学校 ID 工作区 |

**解决方案**：
1. 移除 `userIDFromCtx` 中所有 `fmt.Printf`，替换为 `log` 包的 Trace 级别日志（或直接省略）
2. 新增测试文件覆盖 DAO CRUD + Service RPC 核心路径
3. Proto 新增 `ListSchools` RPC，复用现有 DAO，Gateway 新增 `GET /api/v1/schools` 路由

**业务价值**：
- 消除生产环境信息泄露风险
- 建立测试质量基线，后续开发安全可信
- 打通学校查询跨服务链路，为 Content Service 的学校隔离提供基础设施

---

## 成功指标

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| `fmt.Printf` 调用数 | 0 | `grep -r "fmt\.Printf" cmd/user/` 返回 0 |
| userIDFromCtx 代码评审 | 无调试输出 | Code review |
| 测试覆盖率 | > 60%（核心路径） | `go test -coverprofile` |
| ListSchools API 响应 P95 | < 100ms | 性能测试 |
| 回归 | 0 | `go test ./cmd/user/...` 全部 PASS |

---

## 用户画像

### 主用户：后端开发者

- **角色**：维护 User Service 的开发人员
- **目标**：能安全地重构和扩展 User Service
- **痛点**：修改 userIDFromCtx 或 school 查询时没有测试保护，担心引入回归
- **技术层度**：高级

### 次用户：跨服务调用方（Content / Task Service）

- **角色**：其他微服务的开发者
- **目标**：通过 gRPC 查询学校列表和学校信息
- **技术层度**：高级

---

## 用户故事与验收标准

### Story 1：消除生产调试日志

**作为** 一名后端开发者
**我想要** `userIDFromCtx` 不再打印 `fmt.Printf` 调试信息
**以便于** 消除生产环境 metadata 泄露风险，让日志干净可读

**验收标准：**
- [ ] `cmd/user/service/user_service.go` 中 `userIDFromCtx` 函数移除所有 `fmt.Printf` 调用
- [ ] 正常解析场景不输出任何内容
- [ ] 解析失败时使用 `log.Printf` 代替（或直接返回 0 不记录）
- [ ] 构建通过，行为不变（返回 int64，0 表示未认证）

### Story 2：核心测试覆盖

**作为** 一名后端开发者
**我想要** User Service 有可靠的单元测试
**以便于** 安全地进行后续功能扩展

**验收标准：**
- [ ] `cmd/user/service/user_service_test.go` 覆盖：
  - `WxLogin` — mock wxCode2Session + GetOrCreateByOpenID
  - `RefreshToken` — 正常续签、过期 token、非法 token
  - `BindCampus` — 正常绑定、学校不存在
  - `GetCurrentUser` — 缓存命中、缓存未命中
  - `UpdateUserInfo` — 正常更新、空参数
- [ ] `cmd/user/database/user_dao_test.go` 覆盖：
  - `GetOrCreateByOpenID` — 已存在用户、新用户创建
  - `GetByID` — 存在、不存在
  - `SearchSchools` / `ListSchools` — 关键查询
- [ ] `cmd/user/model/user_test.go` 覆盖：
  - `Role.String()` — 三种角色字符串
  - `Can()` — 权限检查
- [ ] 使用 mock 或 test DB 模式，不依赖外部 MySQL 实例
- [ ] `go test ./cmd/user/...` 全部 PASS

### Story 3：暴露 ListSchools RPC

**作为** 跨服务调用方（Content / Task Service）
**我想要** 通过 gRPC 查询学校列表
**以便于** 在 Content Service 中按学校过滤帖子时获取学校信息

**验收标准：**
- [ ] `PB/user.proto` 新增：
  - `ListSchools` RPC（`ListSchoolsRequest` → `ListSchoolsResponse`）
  - `School` 消息（复用现有 `SchoolInfo` 或重新定义）
  - `ListSchoolsRequest` 含 `keyword`（模糊搜索，可选）和 `page_size` / `cursor`（可选）
- [ ] `cmd/user/service/user_service.go` 新增 `ListSchools` 实现，复用现有 DAO
- [ ] Proto 代码由用户重新生成
- [ ] Gateway 新增 `GET /api/v1/schools` 路由（JWT 鉴权，不强绑学校）
- [ ] 响应格式：`{ "schools": [...], "has_more": bool, "next_cursor": "" }`

---

## 功能需求

### FR-1：移除调试日志

**文件**：`cmd/user/service/user_service.go`

**变更**：
```go
// 修改前（有调试日志）
func userIDFromCtx(ctx context.Context) int64 {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        fmt.Printf("❌ userIDFromCtx: 无法从context获取metadata\n")
        return 0
    }
    fmt.Printf("🔍 userIDFromCtx: 收到的metadata keys: ")
    for key := range md {
        fmt.Printf("%s ", key)
    }
    fmt.Printf("\n")
    vals := md.Get("user-id")
    if len(vals) == 0 {
        fmt.Printf("❌ userIDFromCtx: metadata中未找到user-id键\n")
        return 0
    }
    id, err := strconv.ParseInt(vals[0], 10, 64)
    if err != nil {
        fmt.Printf("❌ userIDFromCtx: 解析user-id失败: %v, 原始值: %s\n", err, vals[0])
        return 0
    }
    fmt.Printf("✅ userIDFromCtx: 成功解析user-id: %d\n", id)
    return id
}

// 修改后（无调试日志，干净返回）
func userIDFromCtx(ctx context.Context) int64 {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return 0
    }
    vals := md.Get("user-id")
    if len(vals) == 0 {
        return 0
    }
    id, err := strconv.ParseInt(vals[0], 10, 64)
    if err != nil {
        return 0
    }
    return id
}
```

**注意事项**：
- 不改变函数签名和返回值语义
- 不修改其他引用 `userIDFromCtx` 的地方（BindCampus、GetCurrentUser、UpdateUserInfo）
- 不删除 `"strconv"` import（仍用于 `ParseInt`）

### FR-2：测试覆盖

**测试文件清单**：

| 测试文件 | 覆盖 | 预计用例数 |
|---------|------|-----------|
| `cmd/user/service/user_service_test.go` | 5 个 RPC 核心路径 | ~12 |
| `cmd/user/database/user_dao_test.go` | DAO CRUD + 学校查询 | ~8 |
| `cmd/user/model/user_test.go` | Role + Can + 常量 | ~6 |

**测试策略**：
- Service 层：使用 `httptest` mock `wxCode2Session` HTTP 调用；DAO 层 mock 通过 test DB 或接口注入
- Model 层：纯逻辑，直接测试无需 mock

### FR-3：ListSchools RPC

**Proto 变更**（`PB/user.proto`）：
```protobuf
service UserService {
  // ... 现有 5 个 RPC ...
  
  // 搜索/列出学校（供跨服务调用和前端选择学校）
  rpc ListSchools (ListSchoolsRequest) returns (ListSchoolsResponse);
}

message ListSchoolsRequest {
  string keyword = 1;  // 模糊搜索关键字（可选，空 = 返回全部）
  int32 page_size = 2; // 每页数量（可选，默认 20，上限 50）
  string cursor = 3;   // 游标（可选）
}

message ListSchoolsResponse {
  repeated School schools = 1;
  bool has_more = 2;
  string next_cursor = 3;
}
```

> `School` 消息复用 proto 中已定义的 `SchoolInfo`（或重命名为 `School`），含 `school_id`、`name`、`province` 字段。

**Service 实现**：复用 `user_database.SearchSchools` 和 `user_database.ListSchools`，按 keyword 存在与否选择。

**Gateway 路由**：
- `GET /api/v1/schools`（JWT 鉴权，不强绑 school）
- 支持 query 参数：`keyword`、`page_size`、`cursor`
- 响应：`{ "schools": [...], "has_more": bool, "next_cursor": "" }`

### Out of Scope

- ❌ 管理员封禁/解封接口
- ❌ RBAC 权限系统接入（model.Can 保留但不启用）
- ❌ CreditScore 信用分逻辑
- ❌ 用户状态（StatusBanned/StatusDeleted）强制校验
- ❌ 批量用户查询接口
- ❌ 学校信息增删改（仅查询）

---

## 技术约束

### 性能
- ListSchools API P95 < 100ms（学校表数据量 < 3000 条，无需分库）
- 测试执行时间 < 10s

### 安全
- 移除 `fmt.Printf` 消除 metadata 信息泄露
- ListSchools 仅返回学校名称和 ID，不暴露敏感信息
- JWT 鉴权保护 ListSchools 路由（小程序前端需要）

### 集成
- **Proto**：扩展 `user.proto`，复用 `SchoolInfo` 消息结构
- **Gateway**：新增 `GET /api/v1/schools` 路由（模式参考 Content Service 读路由）
- **User Service**：复用现有 DAO，不改数据库

### 技术栈
- Go 1.22+ testing（标准库，不引入 testify）
- 可选的 mock 策略：httptest（mock 微信 API）+ DAO 接口注入

---

## MVP 范围与分期

### Phase 1：本 PRD（单期交付）

| 模块 | 范围 |
|------|------|
| **调试日志清理** | 移除 userIDFromCtx 中全部 fmt.Printf |
| **测试覆盖** | Service/DAO/Model 三级测试，> 60% 覆盖 |
| **ListSchools RPC** | Proto + Service + Gateway 路由 |
| **Proto 生成** | 用户手动生成 pb.go 文件 |

### 显式 Out of Scope

- ❌ 管理员后台功能
- ❌ RBAC 权限校验启用
- ❌ 用户封禁/状态管理
- ❌ 信用分系统
- ❌ 跨服务批量用户查询
- ❌ 学校 CRUD 管理

---

## 风险评估

| 风险 | 概率 | 影响 | 缓解策略 |
|------|------|------|---------|
| userIDFromCtx 修改后行为不一致 | 低 | 中 | 函数逻辑不变，仅移除打印，单测验证 |
| DAO 测试依赖 MySQL | 高 | 中 | 使用 test DB 或在 CI 中启动 MySQL container |
| ListSchools 无游标分页需求 | 低 | 低 | 学校数量少，keyword 搜索已够用，游标为扩展预留 |

---

## 依赖与阻塞

### 依赖

| 依赖项 | 描述 | 状态 |
|--------|------|------|
| MySQL | `campus_user` 数据库（已有） | ✅ |
| WeChat API | WxLogin 依赖 jscode2session（测试时 mock） | ✅ |
| Gateway | 复用 JWT 中间件 | ✅ |

### 已知阻塞

无。所有依赖均已就绪。

---

## 附录

### 文件变更清单

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `PB/user.proto` | 修改 | 新增 ListSchools RPC + Request/Response |
| `PB/pb/user_pb/*.pb.go` | 生成 | 用户执行 protoc |
| `cmd/user/service/user_service.go` | 修改 | 移除 fmt.Printf + 新增 ListSchools 实现 |
| `cmd/user/service/user_service_test.go` | 新增 | ~12 个测试用例 |
| `cmd/user/database/user_dao_test.go` | 新增 | ~8 个测试用例 |
| `cmd/user/model/user_test.go` | 新增 | ~6 个测试用例 |
| `cmd/gateway/handler/user_handler.go` | 修改 | 新增 ListSchools handler |
| `cmd/gateway/router/app.go` | 修改 | 新增 `GET /api/v1/schools` 路由 |

### 参考文档

- **Gateway Service PRD**：`docs/gateway-service-prd.md`（路由注册模式）
- **Content Service v1.0**：`docs/content-service-prd.md`（跨服务调用参考）
- **User Service 当前代码**：`cmd/user/`（5 个 RPC 实现）

---

*本 PRD 通过 2 轮迭代式需求对话生成，质量评分 91/100。范围精确定义为基础设施完善，不引入新业务功能。*