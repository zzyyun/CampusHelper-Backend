# Product Requirements Document: AI 审核服务完善上线

**Version**: 1.0
**Date**: 2026-07-08
**Author**: Sarah (Product Owner)
**Quality Score**: 91/100

**前置依赖**：
- [AI 审核 + Content Service v3.0](docs/ai-moderation-content-service-v3.0-prd.md) — 架构设计与业务流程
- [Content Service v2.1](docs/content-service-v2-prd.md) — 异步链路与 MQ 基础设施

---

## Executive Summary

AI 审核服务（ai-moderation）的架构设计已完成（v3.0 PRD），代码骨架已实现（gRPC 服务、熔断客户端、双模式推理、审计日志、异步消费者），但处于 **"可编译不可运行"** 状态。本 PRD 定义将 ai-moderation 从预部署状态推进到 **生产可用** 所需的全部工作。

核心缺口：Content Service 未初始化 AI 客户端（功能静默失效）、无 Dockerfile（无法容器化）、未接入 docker-compose/CI/CD（无法编排部署）、异步调度器和宽限期终止器为桩函数（异步链路断裂）、AI 服务启动时硬编码 mock 模式（ONNX 无法激活）。

**业务价值**：
- **打通 AI 审核全链路**：从"架构存在"到"生产可用"，释放 v3.0 的全部业务价值
- **降低人工审核量 ≥ 80%**（v3.0 PRD 承诺的指标，因当前缺口尚未兑现）
- **验证微服务部署架构**：为后续服务接入 Docker/CI/CD 提供模板

---

## Problem Statement

### Current Situation

| 缺口 | 影响 | 严重度 |
|------|------|--------|
| Content Service `main.go` 未调用 `InitAIClient` | AI 客户端永远为 nil，所有调用静默降级为 DEGRADED | **阻塞** |
| 无 ai-moderation Dockerfile | 无法容器化部署 | **阻塞** |
| 未接入 `campus-docker-compose.yaml` | 无法编排到服务拓扑中 | **阻塞** |
| 未纳入 `.github/workflows/deploy.yaml` | CI/CD 不构建不部署 | **阻塞** |
| `main.go` 硬编码 `mode="mock"` | ONNX 加载后仍报 mock，HealthCheck 信息不准确 | 高 |
| `async_review_scheduler` 桩函数 | 每日兜底扫描不工作，仅依赖实时入队 | 高 |
| `taken_down_finalizer` 桩函数 | 24h 宽限期终止器不工作，帖子永久停留在 pending | 高 |
| 配置默认 `enabled: false` | 默认无法启用真实推理 | 中 |

### Proposed Solution

分两阶段（功能优先 → 部署完善）修复全部缺口，使 AI 审核服务达到生产就绪状态。

---

## Success Metrics

| 指标 | 目标值 | 验证方式 |
|------|--------|----------|
| AI 客户端初始化成功率 | 100%（Content Service 启动时） | `content-service` 启动日志含 `AI client initialized` |
| AI 同步审核端到端延迟 | P95 ≤ 800ms | Jaeger span 监控 |
| AI 服务健康检查通过 | `grpc_health_v1` 返回 SERVING | `grpcurl` 验证 |
| Mock/ONNX 双模式可切换 | 配置 `enabled` 控制 | 单元测试 + 集成测试 |
| 异步调度器扫描执行 | 每日 02:00 触发，日志含扫描结果 | 启动日志 + MQ 消息 |
| 宽限期终止器执行 | 每小时触发，超 24h 无申诉帖子 → closed | MQ `content.taken_down` 事件 |
| Docker 构建成功 | `docker build` 通过 | CI build 阶段 |
| docker-compose 编排成功 | `docker compose up ai-moderation` 启动正常 | 本地验证 |
| CI/CD 构建 ai-moderation 镜像 | push main 后 ACR 出现新镜像 tag | GitHub Actions 运行记录 |

---

## User Personas

### Primary: 平台运维人员
- **角色**：负责 CampusHelper 后端服务的部署与运维
- **目标**：ai-moderation 服务可容器化部署、可观测、可重启
- **痛点**：当前 AI 审核只是"代码存在但跑不起来"，无法在服务器上运行

### Secondary: 内容审核管理员
- **角色**：负责校园内容审核的运营人员
- **目标**：AI 自动处理大部分正常/明显违规内容，只需关注 AI 不确定的内容
- **痛点**：当前所有未命中 DFA 的内容都进人工池，审核效率低

### Tertiary: 发帖用户（学生）
- **角色**：在平台发布帖子的在校学生
- **目标**：发帖不被无故拦截，违规帖子被及时处理
- **痛点**：N/A（当前 AI 未实际运行，无体感）

---

## User Stories & Acceptance Criteria

### Story 1: Content Service 启动时初始化 AI 客户端

**As a** 平台运维人员
**I want** Content Service 启动时自动连接 ai-moderation 服务
**So that** 发帖时的 AI 审核调用不再静默降级

**Acceptance Criteria:**
- [ ] `cmd/content/main.go` 调用 `service.InitAIClient(aiclient.NewClient(...))`
- [ ] AI 客户端地址从配置 `service.ai-moderation.address` 读取
- [ ] 初始化失败时记录 WARN 日志但不阻止 Content Service 启动（降级模式）
- [ ] 启动日志包含 `AI client initialized (addr=xxx)`
- [ ] 单元测试覆盖：初始化成功、地址为空、连接失败

### Story 2: ONNX 模式正确激活

**As a** 平台运维人员
**I want** 配置 `enabled: true` 后 ai-moderation 使用 ONNX 真实推理
**So that** AI 审核不再只是返回固定 PASS

**Acceptance Criteria:**
- [ ] `cmd/ai-moderation/main.go` 根据 `aiModeration.enabled` 自动选择 mock/onnx 模式
- [ ] `NewServiceWithMode` 的 mode 参数从配置动态获取（不再硬编码 "mock"）
- [ ] HealthCheck 响应的 `mode` 字段准确反映实际模式
- [ ] 配置文件中添加 `aiModeration.mode` 字段（或从 `enabled` 自动推导）
- [ ] Mock 模式与 ONNX 模式均通过现有单元测试

### Story 3: 异步补判调度器完整实现

**As a** 内容审核管理员
**I want** 每日凌晨自动扫描近 7 天已发布帖子并入队异步复审
**So that** 实时入队遗漏的帖子也能被 AI 复审

**Acceptance Criteria:**
- [ ] `async_review_scheduler.go` 的 `scanRecentPublishedPosts` 实现真实 DAO 查询
- [ ] 查询条件：`status=2 (published) AND created_at >= now-7d AND deleted_at IS NULL`
- [ ] 按 `school_id` 分组入队，每条消息携带 `school_id` + `post_id`
- [ ] 查询结果带 `LIMIT`（防止首次全量扫描造成 MQ 积压，建议单次 ≤ 1000 条）
- [ ] `main.go` 中启动 AsyncReviewScheduler（传入 ctx + mqAddr）
- [ ] 单元测试覆盖：正常扫描、空结果、MQ 发送失败

### Story 4: 宽限期终止器完整实现

**As a** 内容审核管理员
**I want** 超过 24h 宽限期且无申诉的帖子自动转为 closed 状态
**So that** 违规帖子被最终下架，不再占用平台资源

**Acceptance Criteria:**
- [ ] `taken_down_finalizer.go` 的 `scanTakenDownPendingPosts` 实现真实 DAO 查询
- [ ] 查询条件：`status=8 (taken_down_pending) AND updated_at < now-24h AND deleted_at IS NULL`
- [ ] `hasAppeal` 查询 `ai_audit_logs` 表检查是否有申诉标记（本期简化：查 `ai_status=3` 标记）
- [ ] 无申诉 → `UpdateStatus(taken_down_pending → closed)` + 发布 `content.taken_down` MQ 事件
- [ ] 有申诉 → 跳过（日志记录）
- [ ] `main.go` 中启动 TakenDownFinalizer（传入 ctx + mqAddr）
- [ ] 单元测试覆盖：正常终止、有申诉跳过、状态更新失败

### Story 5: ai-moderation 容器化与编排

**As a** 平台运维人员
**I want** ai-moderation 服务可通过 Docker 和 docker-compose 部署
**So that** 与其他微服务统一管理

**Acceptance Criteria:**
- [ ] 创建 `build/docker/ai-moderation.Dockerfile`（遵循现有 Dockerfile 模式）
- [ ] Mock 模式：`CGO_ENABLED=0` 静态构建（与现有服务一致）
- [ ] ONNX 模式：多阶段构建，含 `libonnxruntime` 运行时依赖（build tag `onnx_enabled`）
- [ ] `campus-docker-compose.yaml` 添加 ai-moderation 服务定义
  - 端口映射：`50061:50061`（gRPC）、`9091:9091`（metrics）
  - 依赖：etcd、rabbitmq
  - 健康检查：gRPC health check
- [ ] 模型文件通过 volume 挂载（`/models/`）

### Story 6: CI/CD 集成

**As a** 平台运维人员
**I want** push 到 main 时自动构建 ai-moderation 镜像并推送到 ACR
**So that** 部署流程与其他服务一致

**Acceptance Criteria:**
- [ ] `.github/workflows/deploy.yaml` build matrix 添加 `ai-moderation`
- [ ] `workflow_dispatch` service 下拉菜单添加 `ai-moderation` 选项
- [ ] 镜像 tag 格式：`v1.0-ai-moderation-{短SHA}`
- [ ] Mock 模式 Dockerfile 不依赖 CGO（CI runner 无需安装 onnxruntime）
- [ ] CI test 阶段覆盖 ai-moderation 包的测试

---

## Functional Requirements

### Feature 1: Content Service AI 客户端初始化

**Description**: 在 `cmd/content/main.go` 的启动流程中，读取 `service.ai-moderation.address` 配置，创建 `aiclient.NewClient` 实例，调用 `service.InitAIClient` 注入。

**User flow**:
1. Content Service 启动
2. 读取 ai-moderation 地址配置
3. 创建 gRPC 客户端（含熔断器）
4. 调用 `InitAIClient` 注入全局变量
5. 日志记录初始化结果

**Edge cases**:
- 地址为空 → 跳过初始化，日志 WARN
- gRPC 连接失败 → 跳过初始化，日志 WARN（不阻塞启动）
- ai-moderation 服务尚未启动 → 客户端创建成功（gRPC 连接是懒建立）

**Error handling**: 初始化失败不阻止 Content Service 启动，`callAIModeration` 内部已有 nil client fallback 逻辑。

### Feature 2: ai-moderation 模式自动推导

**Description**: `cmd/ai-moderation/main.go` 根据 `aiModeration.enabled` 配置自动推导模式字符串，传递给 `NewServiceWithMode`。

**User flow**:
1. 读取 `aiModeration.enabled` 配置
2. `enabled=false` → MockLoader + mode="mock"
3. `enabled=true` + `onnx_enabled` build tag → OnnxLoader + mode="onnxruntime"
4. `enabled=true` + 无 `onnx_enabled` tag → 启动失败（明确错误信息）
5. HealthCheck 响应 `mode` 字段准确反映

**Edge cases**:
- 配置缺失 `enabled` → 默认 false（mock 模式）
- 模型文件不存在 → 启动失败（fatal）
- 模型 hash 不匹配 → 启动失败（fatal）

### Feature 3: 异步调度器 DAO 实现

**Description**: 替换 `async_review_scheduler.go` 中的桩函数 `scanRecentPublishedPosts`，实现真实数据库查询。

**SQL 查询**:
```sql
SELECT id, school_id FROM posts
WHERE status = 2
  AND created_at >= ?
  AND deleted_at IS NULL
ORDER BY id ASC
LIMIT 1000
```

**实现要点**:
- 使用 `content_repo` 包的 `mustContentDB()` 获取 GORM 实例
- 分页查询（LIMIT 1000），避免单次加载过多数据
- `main.go` 中启动调度器，传入 `ctx` + `mqAddr`

### Feature 4: 宽限期终止器 DAO 实现

**Description**: 替换 `taken_down_finalizer.go` 中的桩函数，实现真实数据库查询和申诉检查。

**SQL 查询**:
```sql
-- 扫描超宽限期帖子
SELECT id, school_id, user_id FROM posts
WHERE status = 8
  AND updated_at < ?
  AND deleted_at IS NULL

-- 检查申诉记录（简化：查 ai_audit_logs 是否有申诉标记）
SELECT COUNT(*) FROM ai_audit_logs
WHERE post_id = ? AND ai_status = 3
```

**实现要点**:
- `scanTakenDownPendingPosts` 使用 GORM 查询
- `hasAppeal` 查询 ai_audit_logs 表（本期简化：ai_status=3 表示申诉中）
- `main.go` 中启动 finalizer，传入 `ctx` + `mqAddr`

### Feature 5: Dockerfile 与 docker-compose

**Description**: 创建 ai-moderation 的 Dockerfile 并接入 docker-compose 编排。

**Dockerfile 设计**:
- Mock 模式（默认）：`CGO_ENABLED=0`，基础镜像 `distroless/static`，与现有服务一致
- ONNX 模式：需在 Dockerfile 中安装 `libonnxruntime`，使用 `-tags onnx_enabled` 构建
- 端口：`50061`（gRPC）、`9091`（metrics）

**docker-compose 服务定义**:
```yaml
ai-moderation:
  build:
    context: .
    dockerfile: build/docker/ai-moderation.Dockerfile
  container_name: ai-moderation
  ports:
    - "50061:50061"
    - "9091:9091"
  volumes:
    - ./config/my_config.yaml:/app/config/my_config.yaml
    - /models:/models  # ONNX 模型文件
  depends_on:
    - etcd
    - rabbitmq
  restart: unless-stopped
```

### Feature 6: CI/CD Pipeline 集成

**Description**: 将 ai-moderation 加入 GitHub Actions 构建和部署流程。

**修改点**:
- build matrix: 添加 `ai-moderation` 服务
- workflow_dispatch service 下拉菜单: 添加 `ai-moderation` 选项
- 构建命令: `CGO_ENABLED=0 go build ./cmd/ai-moderation`（mock 模式）

### Out of Scope

- ONNX 模型文件本身的训练与提供（需单独任务）
- 评论区 AI 审核（v3.x 后续）
- 图片 AI 审核（v3.x 后续）
- 用户申诉工作流（v3.x 后续，本期仅做简化 `hasAppeal`）
- AI 管理后台 Dashboard（v4.0）
- mTLS 内网加密（安全增强，非本期）
- Tokenizer 真实 vocab.txt 升级（模型适配任务）
- 多实例负载均衡（水平扩展任务）

---

## Technical Constraints

### Performance
- AI 同步审核延迟 P95 ≤ 800ms（已有超时保障）
- 异步调度器单次扫描 ≤ 1000 条（分页查询）
- 宽限期终止器每小时执行，单次处理 ≤ 200 条

### Security
- gRPC 接口仅限内网调用（已有服务发现隔离）
- 内容存储为 SHA256 哈希（已有隐私保护）
- 不暴露额外端口到公网

### Integration
- **etcd**: 服务注册与发现（已有基础设施）
- **RabbitMQ**: 异步消息（已有 `notification.events` 队列）
- **MySQL (campus_content)**: `posts` + `ai_audit_logs` 表（已有 schema）
- **Jaeger**: 分布式追踪（已有 OpenTelemetry 集成）

### Technology Stack
- Go 1.22+（与现有服务一致）
- gRPC + Protobuf（已有 proto 定义）
- onnxruntime-go v1.31.0（已有依赖）
- sony/gobreaker（已有熔断器）
- Docker + distroless（与现有 Dockerfile 一致）

---

## MVP Scope & Phasing

### Phase 1: 功能完善（优先级最高）

**目标**：让 AI 审核在本地/开发环境可正常工作

1. **Content Service AI 客户端初始化** — 修复"功能静默失效"的根因
2. **ai-moderation 模式自动推导** — 修复 mode 硬编码
3. **异步调度器 DAO 实现** — 补全异步链路入口
4. **宽限期终止器 DAO 实现** — 补全异步链路出口
5. **配置文件更新** — 添加必要配置项

**MVP 验证标准**：
- `go run ./cmd/content` 启动日志含 `AI client initialized`
- `go run ./cmd/ai-moderation` 启动日志含正确 mode
- Mock 模式下 `go test ./...` 全部通过
- 异步调度器/终止器日志可观察到扫描执行

### Phase 2: 容器化与部署

**目标**：ai-moderation 可容器化部署到生产环境

1. **Dockerfile** — Mock 模式 + ONNX 模式双版本
2. **docker-compose 接入** — 服务编排与依赖
3. **CI/CD 集成** — 自动构建与推送镜像

**验证标准**：
- `docker compose build ai-moderation` 成功
- `docker compose up ai-moderation` 服务正常启动
- push main 后 GitHub Actions 构建 ai-moderation 镜像

---

## Risk Assessment

| 风险 | 概率 | 影响 | 缓解策略 |
|------|------|------|----------|
| ONNX 模型文件未提供，无法测试真实推理 | 高 | 中 | Mock 模式已完整，可先验证全链路；ONNX 测试后续补充 |
| `libonnxruntime` 在 Docker 中安装失败 | 中 | 中 | 提供 Mock Dockerfile（默认）和 ONNX Dockerfile（可选） |
| 异步调度器 DAO 引入 GORM 查询性能问题 | 低 | 低 | LIMIT 1000 分页 + 索引已存在（idx_status, idx_created_at） |
| 宽限期终止器 hasAppeal 简化逻辑误判 | 中 | 中 | 本期为简化实现，完整申诉机制留 v3.x；误判方向为保守（跳过下架） |
| Content Service 重启期间 AI 调用丢失 | 低 | 低 | 熔断器自动恢复 + gRPC 懒连接 |

---

## Dependencies & Blockers

**Dependencies:**
- `pkg/aiclient`（已完成）— 提供客户端接口与熔断器
- `cmd/content/repo`（已完成）— 提供 GORM 查询基础
- `cmd/content/model`（已完成）— 提供 PostStatus 常量与 AIAuditLog 模型
- `internal/ai_moderation`（已完成）— 提供 Service + MockLoader + OnnxLoader
- `config/my_config.yaml`（需更新）— 添加 AI 客户端地址配置

**Known Blockers:**
- ONNX 模型文件（`.onnx`）需单独训练/获取，不在本 PRD 范围内
- ONNX Runtime 共享库（`libonnxruntime.so`）在目标部署环境的安装方式需确认

---

## Appendix

### 术语表
- **ai-moderation**: 独立微服务，提供 AI 文本审核能力（gRPC 接口）
- **Mock 模式**: 固定返回 PASS 的模拟实现，零 cgo 依赖
- **ONNX 模式**: 基于 onnxruntime-go 的真实推理实现，需 build tag `onnx_enabled`
- **异步补判**: 已 published 帖子被 AI 重新审核的机制（MQ 驱动）
- **宽限期**: 帖子被异步下架后的 24h 申诉窗口期
- **熔断器**: 客户端侧保护机制，连续 5 次失败后 30s 内直接降级

### 参考文档
- [AI 审核 + Content Service v3.0 PRD](docs/ai-moderation-content-service-v3.0-prd.md)
- [AI 审核测试用例](tests/ai-moderation-test-cases.md)
- `PB/ai_moderation.proto` — gRPC 接口定义
- `internal/ai_moderation/` — 服务端实现
- `pkg/aiclient/` — 客户端封装

---

*本 PRD 通过交互式需求收集与质量评分（91/100）生成，确保业务、功能、UX、技术维度的全面覆盖。*
