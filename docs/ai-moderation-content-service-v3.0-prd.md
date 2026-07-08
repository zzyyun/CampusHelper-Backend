# Product Requirements Document: AI 智能审核 + Content Service v3.0

**Version**: 3.0 (rev 2)
**Date**: 2026-06-27
**Author**: Sarah (Product Owner)
**Quality Score**: 92/100（v3.0 rev2，所有严重问题已修复，5 项默认待定已更新）
**前置版本**：
- [Content Service v1.0](docs/content-service-prd.md) — DFA 敏感词 + 基础审核流
- [Content Service v2.1](docs/content-service-v2-prd.md) — 异步链路激活 + 二级评论
- [User Service v2.0](docs/user-service-v2.0-prd.md) — 管理员审核入口（依赖本 PRD 提供的 ListContentByStatus）

---

## Executive Summary

Content Service v1.0 已实现 DFA 敏感词过滤（机械词库匹配），v2.1 完成了异步链路。当前审核流程痛点是：**DFA 漏网字面违规（如变体字、隐晦表达）+ 大量正常内容涌入人工池**。

v3.0 引入**独立的 ai-moderation 微服务**，基于 **onnxruntime-go 本地推理**（不依赖任何第三方 API），作为 DFA 与人工审核之间的"智能中段"。整体架构遵循**同步串行 + 异步补判双轨**：

```
发帖请求 → [DFA 同步] → [AI 同步, 800ms 超时] → published / pending_review / rejected
                ↓ 命中拒绝
              rejected
                              ↓ AI 不可用 fallback
                          [仅 DFA 模式]
                                  ↓
                  AsyncAIReviewConsumer
                  (实时入队：发帖后异步入队)
                                  ↓
                       published 帖子被 AI 后判违规？
                                  ↓
                  24h 宽限期 → taken_down_pending
                                  ↓
                  24h 后无人申诉 → taken_down + MQ 通知
```

**业务价值**：
- **降低人工审核量 ≥ 80%**（AI 自动裁决 pass + block 都直接处理，不进 pending_review）
- **降低漏网违规率 ≥ 40%**（AI 识别 DFA 漏掉的变体/语义违规）
- **保留人工最终裁决权**（AI 仅辅助，不替代管理员）
- **架构可演进**：ai-moderation 独立服务，未来可叠加多模型、规则引擎、图像识别

---

## Problem Statement

### Current Situation

| 痛点 | 影响 |
|------|------|
| DFA 仅机械词库匹配 | 变体字（薇❤、威信）、谐音、隐晦语义全漏网 |
| 未命中 DFA 的内容**直接 pending_review 进人工池** | 管理员每日处理 80%+ 正常内容，效率极低 |
| 无 AI 智能判断能力 | 平台缺乏"机器辅助审核"这个工业界标配 |
| 敏感词库更新靠人工维护 | 跟不上新型违规变种 |

### Proposed Solution

新增独立 **ai-moderation** 微服务（Go + onnxruntime-go），作为 Content Service 的"智能审核外挂"。Content Service 在发帖路径上同步调用 ai-moderation 做二次判断，AI 不通过/不确定则进 pending_review 走人工。

### Business Impact

- **审核效率提升**：管理员每日处理量预计从 1000 条降至 200 条（80% 工作量转移给 AI 自动裁决）
- **漏网违规率下降**：变体字、跨语言违规被 AI 识别
- **发帖体验保持**：同步调用 AI 设 800ms 硬超时，超时则 fallback 到仅 DFA 模式（不阻塞发帖）
- **合规风险降低**：AI 自动审核 + 审计日志留存，满足学校合规要求
- **用户体验保护**：异步补判下架引入 24h 宽限期，避免误杀影响用户体验

---

## Success Metrics

### Primary KPIs

| 指标 | 目标值 | 验证方式 |
|------|--------|----------|
| **AI 自动放行率**（pass / 总调用） | ≥ 60% | ai_audit_logs 中 `ai_result=0` 占比 |
| **AI 自动裁决率**（pass + block / 总调用） | ≥ 80% | **节省人工审核的真实指标** |
| **AI 误判率（误杀正常内容）** | ≤ 3% | 人工申诉率统计 |
| **AI 漏判率（违规未拦截）** | ≤ 5% | 异步补判命中违规占比 |
| **同步 AI 审核延迟** | P95 ≤ 800ms | Jaeger Span 监控 |
| **AI 服务可用率** | ≥ 99.5% | ai-moderation 健康检查 + 熔断统计 |
| **降级触发率**（AI 不可用） | ≤ 1% | 仅 DFA 模式触发占比 |
| **异步误下架申诉成功率** | ≥ 60% | 宽限期内人工申诉统计 |

### Validation

- 上线后第 1 周：通过 audit_log 统计 AI 命中率/误判率
- 上线后第 1 月：通过 `taken_down_pending` → `taken_down` 转化率统计异步补判准确率
- 上线后第 3 月：评估是否需要替换/升级模型

---

## User Personas

### Primary: 发帖者（学生）
- **Role**: 在校大学生
- **Goals**: 快速发帖、内容快速可见
- **Pain Points**:
  - 当前：正常帖子也要等人工审核（平均 4 小时）
  - 期待：AI 审核通过后**秒级可见**
- **Technical Level**: Novice（不感知 AI 存在）

### Secondary: 学校管理员
- **Role**: 学生会志愿者
- **Goals**: 减少无效工作量，专注处理真正违规内容
- **Pain Points**:
  - 当前：每天处理 80%+ 正常内容
  - 期待：AI 过滤后只处理真正违规
- **Technical Level**: Novice

### Tertiary: 平台运营（super_admin）
- **Role**: 开发团队
- **Goals**: 监控 AI 模型效果、及时调优
- **Pain Points**: 需要 AI 决策的可观测性
- **Technical Level**: Advanced

---

## User Stories & Acceptance Criteria

### Story 1: 正常帖子 AI 秒级通过

**As a** 发帖者
**I want to** 发布正常校园内容
**So that** AI 审核通过后立即可见，不用等人工

**Acceptance Criteria:**
- [ ] 发帖请求携带纯文本（≤ 1000 字）
- [ ] Content Service 同步调用 ai-moderation gRPC `ModerateText`，超时 800ms
- [ ] AI 返回 `result=PASS` → 帖子直接 `published`，MQ 事件 `content.published` 触发 ES 同步
- [ ] 用户发帖响应延迟 P95 ≤ 1s（DFA + AI 同步）
- [ ] AI 同步审核通过的事记录到 `ai_audit_logs` 表（ai_status=synced, ai_result=0）

### Story 2: 违规内容 AI 同步拦截

**As a** 平台运营
**I want to** AI 自动拦截 DFA 漏网的违规内容
**So that** 管理员工作量降低

**Acceptance Criteria:**
- [ ] AI 返回 `result=BLOCK` → 帖子状态 `rejected`，返回 40001 + 命中原因
- [ ] 发帖者收到拒绝通知（含违规类别：涉政/色情/广告/辱骂等）
- [ ] 同步拦截率 ≥ 80% 的明确违规样本
- [ ] ai_audit_logs 记录 AI 拦截原因与置信度

### Story 3: AI 不确定 → 进人工池

**As a** 管理员
**I want to** AI 标记不确定的内容进 pending_review
**So that** 人工做最终判断，避免误杀

**Acceptance Criteria:**
- [ ] AI 返回 `result=REVIEW` → 帖子状态 `pending_review`
- [ ] 管理员通过 User Service v2.0 的 `ListContentForAudit` 看到此条目
- [ ] 标记原因记录（"涉政疑似，置信度 0.62"）
- [ ] 用户看到"内容审核中"提示

### Story 4: AI 服务不可用降级

**As a** 平台运营
**I want to** AI 服务挂掉时系统仍能正常发帖
**So that** 不被单点故障阻塞

**Acceptance Criteria:**
- [ ] ai-moderation 健康检查失败或 gRPC 超时 → Content Service 仅执行 DFA
- [ ] DFA 不命中 → 帖子直接 `published`，ai_audit_logs 记录 `ai_status=degraded, fallback_used=true`
- [ ] AI 服务熔断窗口（30s 内失败 > 5 次则熔断）
- [ ] 降级模式触发时发送告警 MQ 事件 `ai.moderation.degraded`
- [ ] 降级期间 ai_audit_logs 仍记录（即使 ai_result 是兜底逻辑推断的）

### Story 5: 已发布帖子异步补判 + 24h 宽限期

**As a** 平台运营
**I want to** 已发布的帖子被 AI 后判违规时先标记待下架
**So that** 给用户申诉机会，避免误杀

**Acceptance Criteria:**
- [ ] `cmd/content` 内置 cron 调度器（每日 02:00）扫描近 7 天 published 帖子
- [ ] 帖子异步入队 `ai.moderation.async_review` 队列（实时入队，避免错过新发帖）
- [ ] 启动 `AsyncAIReviewConsumer` 订阅该队列
- [ ] Consumer 拉取消息，对帖子重新 AI 判断
- [ ] **AI 返回 BLOCK → 帖子状态 `taken_down_pending`**（非直接 taken_down）
- [ ] 24h 后由定时任务检查，若仍为 `taken_down_pending` 且无申诉 → 改为 `taken_down`
- [ ] MQ 事件 `content.taken_down` 发布（含宽限期结束的提示）
- [ ] MQ 事件 `content.taken_down_pending` 立即通知用户"您的帖子疑似违规，24h 内可申请复审"
- [ ] 宽限期内用户可通过 App 端"申请复审"入口提交申诉（v3.x 完整流程上线前由人工客服处理）

### Story 6: AI 审核可观测性

**As a** 平台运营
**I want to** 看到 AI 决策的完整审计链路
**So that** 调优模型与定位问题

**Acceptance Criteria:**
- [ ] `ai_audit_logs` 表记录每条 AI 调用：post_id, content_hash, ai_status, ai_result, ai_confidence, ai_categories, latency_ms, model_version, fallback_used, trace_id
- [ ] ai-moderation 服务暴露 Prometheus metrics：调用次数/命中率/延迟分桶/熔断状态
- [ ] Content Service 通过 gRPC metadata 透传 trace_id，Jaeger 中可关联 AI 调用 Span
- [ ] 管理员后台（User v2.0）可按 `audit_content_ai` 类型查询 AI 相关操作
- [ ] 模型版本切换支持灰度发布（config 中按比例分配流量）

---

## Functional Requirements

### Core Features

**Feature 1: ai-moderation 微服务（独立新服务）**
- Description: 第 8 个微服务，监听 `:50061`，提供 `ModerateText` 和 `HealthCheck` gRPC 接口
- 技术栈：
  - Go 1.22+
  - **onnxruntime-go** v0.x（本地推理，无第三方 API）
  - 模型：BERT 中文二分类头（input: 文本 token, output: 安全/违规 + 置信度）【待定：模型选型】
  - 模型文件：独立 volume 挂载 `/models/moderation_v1.onnx`（~100MB），Docker 镜像不 COPY
- 接口（**本期仅实现 ModerateText + HealthCheck，AsyncBatchModerate 留 v3.x**）：
  ```protobuf
  service AIModerationService {
    rpc ModerateText(ModerateTextRequest) returns (ModerateTextResponse);
    rpc HealthCheck(google.protobuf.Empty) returns (HealthResponse);
  }
  message ModerateTextRequest {
    string text = 1;
    string trace_id = 2;
    int64 post_id = 3;  // 用于审计
  }
  message ModerateTextResponse {
    enum Result { PASS = 0; REVIEW = 1; BLOCK = 2; }
    enum Status { SYNCED = 0; DEGRADED = 1; ASYNC = 2; }
    Result result = 1;
    Status status = 7;          // 同步/降级/异步区分
    float confidence = 2;  // 0.0 - 1.0
    repeated string categories = 3;  // ["涉政", "色情", ...]
    int64 latency_ms = 4;
    string model_version = 5;
    bool fallback_used = 6;  // true 表示使用了降级逻辑
  }
  message HealthResponse {
    enum ServingStatus { SERVING = 0; NOT_SERVING = 1; }
    ServingStatus status = 1;
    bool model_loaded = 2;
    string model_version = 3;
    int64 model_load_timestamp = 4;
  }
  ```
- Edge cases:
  - 输入超长（>1000字）→ 截断到 512 token，剩余留待异步补判
  - 模型文件缺失 → 启动失败，health check 返回 NOT_SERVING，etcd **不注册**服务
  - onnxruntime 推理异常 → 返回 status=DEGRADED, fallback_used=true
- Error handling:
  - gRPC DeadlineExceeded → Content Service 客户端按超时处理
  - 模型推理异常 → 记录日志 + 返回 fallback_used=true

**Feature 2: Content Service 同步 AI 调用**
- Description: 在 CreatePost 路径中，DFA 之后插入同步 AI 调用（800ms 超时）
- User flow:
  1. 发帖请求进入 CreatePost
  2. DFA 过滤（沿用 v1.0 已有逻辑）→ 命中则直接 rejected
  3. DFA 未命中 → 同步调用 ai-moderation.ModerateText，context.WithTimeout(800ms)
  4. AI 返回 result=pass → published + ai_audit_logs(status=SYNCED, result=0)
  5. AI 返回 result=review → pending_review + ai_audit_logs(status=SYNCED, result=1)
  6. AI 返回 result=block → rejected + ai_audit_logs(status=SYNCED, result=2)
  7. AI 调用失败/超时/熔断 → fallback_used=true，仅 DFA 模式 → 不命中 DFA 则 published + ai_audit_logs(status=DEGRADED, result=0)
  8. 发帖成功后 → 异步入队 ai.moderation.async_review（实时入队）
- Edge cases:
  - AI 服务完全不可用 → 走 fallback，audit_log 记录 status=DEGRADED
  - gRPC DeadlineExceeded → 同上
  - Content Service 重启时熔断器半开 → 前 3 个请求强制走 fallback，避免雪崩
- Error handling:
  - AI 返回未知 result 值 → 视为 REVIEW，进 pending_review（安全兜底）

**Feature 3: Content Service 异步补判 Consumer（含定时调度器）**
- Description: 新增 `cmd/content` 内的 `AsyncAIReviewConsumer` + `AsyncReviewScheduler`，处理已 published 帖子的异步 AI 复审
- 组件：
  - **AsyncReviewScheduler**（定时调度器）：每日 02:00 扫描 status=published AND created_at > now-7d 的帖子，发送 `ai.moderation.async_review` 消息
  - **AsyncAIReviewConsumer**：订阅 `ai.moderation.async_review` 队列，并发消费
- User flow:
  1. 帖子 published 后**立即入队**（CreatePost 成功后异步发送 MQ 消息，避免遗漏）
  2. 每日 02:00 调度器补充扫描近 7 天帖子（兜底，防止实时入队失败）
  3. Consumer 拉取消息（并发 5-10 goroutine），调用 ai-moderation.ModerateText 重新判断
  4. AI 返回 result=block → 帖子 status=taken_down_pending，发 MQ 事件 `content.taken_down_pending`
  5. 24h 后由 `TakenDownFinalizer` 定时任务检查：仍为 taken_down_pending 且无申诉 → 改为 taken_down，发 MQ 事件 `content.taken_down`
  6. AI 返回 result=pass → 不动状态
  7. AI 返回 result=review → 不动状态（保守策略）
- Edge cases:
  - 帖子已被人工/管理员下架 → 跳过
  - AI 服务再次不可用 → 重新入队，下次重试
  - 宽限期内用户申诉 → taken_down_pending 状态保持不变，最终由人工裁决（v3.x 完整流程）
- Error handling:
  - 重试 3 次仍失败 → 死信队列（DLQ）+ 告警
  - 消息体损坏 → 记录 ERROR 日志 + 跳过（不阻塞）

**Feature 4: 客户端熔断器（pkg/aiclient/）**
- Description: ai-moderation 客户端侧熔断（位于 Content Service 内），避免雪崩
- 实现：使用 `sony/gobreaker` v0.5+，封装在 `pkg/aiclient/circuit.go`
- 配置：
  - 30s 滑动窗口
  - 连续 5 次失败 → 熔断
  - 熔断 30s 后半开
  - 半开状态：放行 1 个请求试探，成功则关闭
  - **重启时熔断器初始化为 closed 状态**，前 3 个请求强制走 fallback（避免冷启动雪崩）
- Edge cases:
  - 熔断期间所有 AI 调用立即 fallback（无需等待 800ms）
  - 熔断状态变化时发送 metrics `ai_moderation_circuit_state`

**Feature 5: AI 审计日志表**
- Description: `ai_audit_logs` 表，记录每次 AI 调用结果（含降级）
- 字段：
  ```sql
  CREATE TABLE ai_audit_logs (
    id BIGINT PRIMARY KEY,
    post_id BIGINT NOT NULL,
    content_hash VARCHAR(64) NOT NULL,  -- sha256(text) 用于去重，不存原文
    ai_status TINYINT NOT NULL,         -- 0=synced, 1=degraded, 2=async
    ai_result TINYINT NOT NULL,         -- 0=pass, 1=review, 2=block
    ai_confidence FLOAT NOT NULL,
    ai_categories TEXT,                 -- JSON 数组（中文类别可能超 255 字符）
    latency_ms INT NOT NULL,
    model_version VARCHAR(32) NOT NULL,
    fallback_used TINYINT NOT NULL DEFAULT 0,
    trace_id VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_post_id (post_id),
    INDEX idx_created_at (created_at),
    INDEX idx_ai_status (ai_status)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
  ```
- Edge cases:
  - 表写入失败 → 不阻塞发帖主流程，记录 WARN 日志（运维关注但不告警）

### Out of Scope（本期不做）

- **评论审核** — 评论场景 AI 需求与帖子不同，后续独立 PRD
- **图片审核** — 需要视觉模型，本期仅文本
- **URL 检测** — 本期仅文本，URL 检测留 v3.1
- **AI 模型训练** — 本期使用预训练模型，模型选型+训练数据后续
- **AI 管理后台** — 复用 User v2.0 的 audit_log 查询，AI 专属看板留 v3.x
- **多语言支持** — 本期仅中文
- **完整用户申诉 AI 误判流程** — 本期仅"通知 + 客服兜底"，v3.x 完整流程
- **AsyncBatchModerate RPC** — 本期 proto 不包含，留 v3.x

---

## Technical Constraints

### Performance

| 接口 | P95 延迟 | 备注 |
|------|---------|------|
| 发帖同步路径（DFA + AI） | ≤ 1s | AI 800ms 超时 + DFA < 100ms |
| ai-moderation.ModerateText | ≤ 600ms | onnxruntime 推理 + 文本预处理 |
| 异步补判消费（单条） | ≤ 1s | Consumer 并发 5-10 goroutine |
| ai-moderation 服务启动 | ≤ 10s | 模型加载（一次性） |

### Capacity

- **ai-moderation 服务并发**: 单实例支持 50 QPS（基于 ONNX 推理 + 4 核 CPU 实测）
- **熔断阈值**: 30s 内 5 次失败 → 熔断
- **降级触发频率**: ≤ 1%（高可用要求）
- **异步 Consumer 吞吐**: 5-10 goroutine，单实例可处理每日 ~10 万条帖子（按 24h 均匀分布）
- **ai_audit_logs 写入**: 峰值 QPS 50（与 AI 调用 1:1）

### Security

- ai-moderation gRPC 接口仅内网调用（etcd 服务发现，不暴露公网）
- AI 审计日志保留 180 天（比 admin_audit_logs 长，含 AI 决策详情）
- 模型文件 SHA256 校验，启动时验证
- Content Service 与 ai-moderation 间通信：本期内网明文（可接受），**Phase 2 必须升级 mTLS**
- MQ 事件持久化：RabbitMQ 队列消息保留 7 天，待 Message Service 上线后回放历史 taken_down 事件
- ai_audit_logs 不存储原始文本（仅 content_hash），保护用户隐私

### Integration

| 系统 | 集成方式 |
|------|---------|
| **Content Service** | gRPC 客户端同步调用（pkg/aiclient），etcd 服务发现 |
| **User Service v2.0** | 复用 admin_audit_logs，不新增 RPC（AI 操作通过 audit_content_ai 类型记录） |
| **Message Service**（未来） | 异步补判下架事件 `content.taken_down` / `content.taken_down_pending` 由 Content Service 发布，待 Message 消费；消息持久化 7 天 |
| **MySQL** | Content Service 库新增 `ai_audit_logs` 表（与 content DB 同库，避免跨库事务） |
| **etcd** | ai-moderation 注册服务 `ai-moderation` 命名空间 |
| **Jaeger** | 全链路 trace_id 透传 |
| **Prometheus** | ai-moderation 暴露 `/metrics` |

### Technology Stack

- **ai-moderation 服务**：Go 1.22+, gRPC, **onnxruntime-go v0.x**, sony/gobreaker v0.5+
- **Content Service**：Go 1.22+, GORM, 已有 gRPC 客户端 + 新增 pkg/aiclient（ai-moderation 客户端 + 熔断）
- **模型**：BERT 中文二分类【待定：可选用哈工大讯飞/uer/Chinese-BERT 任意预训练模型 + 自训分类头】
- **模型文件托管**：独立 volume 挂载（不入 Docker 镜像）
- **通信**：protobuf + gRPC（protobuf 定义新增 `PB/ai_moderation.proto`）
- **定时调度**：robfig/cron v3.x（Go 内置 cron 库）

### ai-moderation 服务架构（修正版）

```
┌─ cmd/ai-moderation/ ────────────────────────────────────────┐
│  main.go                                                    │
│    ├─ 加载 ONNX 模型 (volume 挂载 /models/moderation_v1.onnx)│
│    ├─ gRPC Server :50061                                   │
│    ├─ Health Check /metrics :9091                          │
│    └─ etcd 注册 "ai-moderation"                            │
│                                                            │
│  internal/ai_moderation/                                    │
│    ├─ model.go        加载 ONNX runtime session            │
│    ├─ preprocess.go   文本 tokenize + truncate            │
│    ├─ postprocess.go  softmax + 阈值决策                  │
│    └─ service.go      gRPC handler                         │
└────────────────────────────────────────────────────────────┘

┌─ pkg/aiclient/ (Content Service 内, 客户端侧) ──────────────┐
│  client.go    gRPC dial + 调用封装 + 800ms timeout          │
│  circuit.go   sony/gobreaker 熔断器（30s/5次/30s半开）    │
└────────────────────────────────────────────────────────────┘
```

### 内容审核完整链路（v3.0，含异步双轨）

```
发帖请求 (POST /api/v1/posts)
    ↓
[Gateway] JWT 鉴权 + 透传 user_id/school_id/trace_id
    ↓
[Content Service.CreatePost]
    ├─ 1. DFA 敏感词扫描 (pkg/dfa)
    │   ├─ 命中 → 40001 + 命中词列表, 终止
    │   └─ 未命中 ↓
    ├─ 2. 同步 AI 审核 (ai-moderation.ModerateText, ctx 800ms timeout)
    │   ├─ status=SYNCED + result=pass  → status=published
    │   ├─ status=SYNCED + result=review → status=pending_review
    │   ├─ status=SYNCED + result=block  → status=rejected
    │   └─ 超时/错误/熔断 → status=DEGRADED, fallback_used=true
    │       └─ DFA 未命中 → status=published (降级放行)
    ├─ 3. 写入 ai_audit_logs (必写, 失败仅 WARN)
    └─ 4. 异步入队 ai.moderation.async_review (实时入队)
        ↓
[Content Service AsyncAIReviewConsumer] (5-10 goroutine)
    ├─ 订阅 ai.moderation.async_review 队列
    ├─ 对已 published 帖子重新调 AI
    ├─ status=SYNCED + result=block → status=taken_down_pending
    │   + MQ content.taken_down_pending (通知用户)
    └─ result=pass/review → 不动状态
        ↓
[Content Service TakenDownFinalizer] (每小时 cron)
    └─ 检查 created_at < now-24h 且 status=taken_down_pending 且无申诉的帖子
        └─ 改为 taken_down + MQ content.taken_down (最终下架通知)

[Content Service AsyncReviewScheduler] (每日 02:00 cron, 兜底)
    └─ 扫描 published 帖子，补发 ai.moderation.async_review 消息
        （防止实时入队失败遗漏）
```

---

## MVP Scope & Phasing

### Phase 1: MVP (本 PRD 覆盖)

- [ ] **ai-moderation 微服务骨架** — Go + gRPC + onnxruntime-go + etcd 注册 + /metrics
- [ ] **ONNX 模型加载与推理** — 文本预处理 + BERT 推理 + softmax + 阈值决策
- [ ] **PB/ai_moderation.proto** — ModerateText + HealthCheck 接口定义（AsyncBatchModerate 留 v3.x）
- [ ] **pkg/aiclient 客户端封装 + 熔断器** — sony/gobreaker，30s/5次熔断
- [ ] **Content Service 同步集成** — DFA 后插入同步 AI 调用 + 800ms 超时 + fallback
- [ ] **降级 fallback** — AI 不可用时仅 DFA，audit_log 记录 status=DEGRADED
- [ ] **AI 审计日志** — ai_audit_logs 表（含 ai_status 字段）+ 写入逻辑
- [ ] **异步补判 Consumer** — AsyncAIReviewConsumer + AsyncReviewScheduler 定时调度器
- [ ] **24h 宽限期机制** — taken_down_pending 状态 + TakenDownFinalizer 定时器
- [ ] **实时入队** — CreatePost 成功后异步发送 ai.moderation.async_review
- [ ] **可观测性** — Prometheus metrics + Jaeger trace + 模型版本灰度发布

**MVP Definition**: 发帖路径上 AI 同步审核生效，AI 不确定时进人工池；已发布帖子异步补判违规时先 taken_down_pending，24h 后才正式 taken_down；AI 不可用时系统仍能正常发帖；用户有 24h 申诉窗口。

### Phase 2: Enhancements (Post-Launch, 后续 PRD)

- [ ] 评论 AI 审核
- [ ] URL/链接检测（钓鱼/垃圾推广识别）
- [ ] 图片审核（视觉模型）
- [ ] 完整用户申诉 AI 误判流程（小程序端申请复审入口）
- [ ] AI 管理后台（命中率/误判率/类别分布看板）
- [ ] 模型 A/B 测试能力（双模型并行对比）
- [ ] 多语言支持（英文）
- [ ] ai-moderation 多实例负载均衡
- [ ] Content ↔ ai-moderation mTLS 加密
- [ ] 敏感词热更新机制（结合 AI 决策）

### Future Considerations

- [ ] 模型自动 retrain 流水线（基于人工标注数据）
- [ ] AI 服务多实例负载均衡
- [ ] 联邦学习 / 跨学校模型优化
- [ ] AsyncBatchModerate RPC（v3.x 实现）

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation Strategy |
|------|-------------|--------|---------------------|
| **ONNX 模型推理延迟超 800ms** | Med | High | 熔断器兜底；模型量化（INT8）后续优化；超时阈值可调 |
| **AI 模型误判率高** | Med | High | AI 返回 REVIEW 全部进人工池，不直接拦截；提供人工申诉渠道 |
| **ai-moderation 单点故障** | Low | High | 客户端熔断 + 降级 DFA；后续可多实例部署 |
| **模型文件过大（Docker 镜像膨胀）** | Med | Med | **模型文件独立 volume 挂载，不入镜像**；启动时 SHA256 校验 |
| **异步补判误下架**（影响用户体验）| Med | **High** | **24h 宽限期 + taken_down_pending 状态 + 用户申诉入口（v3.x）/ 客服兜底（本期）** |
| **AI 决策不可解释** | Med | Low | 记录 ai_categories + confidence，给管理员决策依据 |
| **AI 服务被绕过（直接调 Content）** | Low | High | Content Service 内部强制调用 AI（不可关闭），仅靠 fallback 降级 |
| **MQ 消费者积压**（突发流量）| Med | Med | Consumer 并发 5-10 goroutine + DLQ 监控告警 + 调度器扩容能力 |
| **Message Service 未上线，事件丢失** | Med | Low | RabbitMQ 队列持久化 + 消息保留 7 天，待 Message 上线后批量回放 |
| **模型版本升级导致准确率波动** | Low | Med | config 中配置 model_version + 按比例灰度发布；保留旧版本可回滚 |

---

## Dependencies & Blockers

### Dependencies

| 依赖 | 说明 |
|------|------|
| **onnxruntime-go 库稳定性** | v0.x 版本，2026 年仍在活跃维护中。需在 v1.0 锁定版本 |
| **预训练 BERT 中文模型** | 需选择开源模型（HFL/Chinese-BERT 等），确认 license 可商用【待定】 |
| **etcd 服务发现** | 已有，ai-moderation 复用 |
| **Content Service 现有 DFA** | v1.0 已实现，直接复用 |
| **User Service v2.0 审计日志** | 已合并 PR #88，AI 操作可记录到 admin_audit_logs |
| **robfig/cron 定时库** | Go 生态成熟库，用于 AsyncReviewScheduler / TakenDownFinalizer |
| **MQ 持久化** | 已有 RabbitMQ 部署，队列消息保留 7 天配置 |

### Known Blockers

- 无。onnxruntime-go + 本地模型方案完全独立，不依赖任何外部 API。

---

## Appendix

### Glossary

- **ai-moderation**: 第 8 个微服务，提供本地 AI 内容审核能力
- **onnxruntime-go**: Go 绑定 ONNX Runtime，支持本地推理预训练模型
- **DFA (Deterministic Finite Automaton)**: 确定性有限状态自动机，用于敏感词机械匹配
- **熔断 (Circuit Breaker)**: 客户端模式，AI 服务异常时快速失败，避免雪崩（位于 Content Service 内的 pkg/aiclient）
- **fallback_used**: AI 审计日志字段，标识本次调用是否走降级
- **ai_status**: AI 审计日志字段，区分 synced/degraded/async
- **异步补判**: 已发布帖子被 AI 重新判断，违规则强制下架
- **taken_down_pending**: 新增帖子状态，异步 AI 判违规后的中间态，宽限期 24h
- **宽限期 (Grace Period)**: taken_down_pending → taken_down 之间的 24h，给用户申诉机会
- **REVIEW**: AI 返回的中间态结果，表示"不确定"，进 pending_review 走人工
- **实时入队**: CreatePost 成功后立即发送 ai.moderation.async_review 消息，避免遗漏

### New RPCs Summary

#### ai-moderation.proto (本期)

| RPC | Request | Response | 说明 |
|-----|---------|----------|------|
| `ModerateText` | text, trace_id, post_id | result, status, confidence, categories, latency_ms, model_version, fallback_used | 同步单条审核 |
| `HealthCheck` | Empty | status(SERVING/NOT_SERVING), model_loaded, model_version, model_load_timestamp | 健康检查 |

> 注：AsyncBatchModerate 留到 v3.x，本期 proto 不包含。

#### Content Service proto（新增字段）

```protobuf
enum AIResult { PASS = 0; REVIEW = 1; BLOCK = 2; DEGRADED = 3; }
message CreatePostResponse {
  // ... 现有字段
  AIResult ai_result = 10;
  float ai_confidence = 11;
  repeated string ai_categories = 12;
  bool ai_fallback_used = 13;
}
```

### 数据库变更

```sql
-- Content Service 库新增 ai_audit_logs 表
CREATE TABLE ai_audit_logs (
  id BIGINT PRIMARY KEY,
  post_id BIGINT NOT NULL,
  content_hash VARCHAR(64) NOT NULL,        -- sha256(text) 用于去重，不存原文
  ai_status TINYINT NOT NULL,                -- 0=synced, 1=degraded, 2=async
  ai_result TINYINT NOT NULL,                -- 0=pass, 1=review, 2=block
  ai_confidence FLOAT NOT NULL,
  ai_categories TEXT,                        -- JSON 数组（中文类别可能超 255 字符）
  latency_ms INT NOT NULL,
  model_version VARCHAR(32) NOT NULL,
  fallback_used TINYINT NOT NULL DEFAULT 0,
  trace_id VARCHAR(64),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_post_id (post_id),
  INDEX idx_created_at (created_at),
  INDEX idx_ai_status (ai_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### posts 表新增字段

```sql
-- 新增 taken_down_pending 状态值
ALTER TABLE posts MODIFY COLUMN status TINYINT NOT NULL DEFAULT 0
  COMMENT '0=draft, 1=pending_review, 2=published, 3=rejected, 4=taken_down, 5=taken_down_pending';
```

### 项目结构变更

新增以下目录：
```
cmd/ai-moderation/                    # 第 8 个微服务
├── main.go
├── config/
└── server.go
internal/ai_moderation/
├── model.go         # onnxruntime-go 模型加载
├── preprocess.go    # 文本 tokenize
├── postprocess.go   # softmax + 阈值
└── service.go       # gRPC handler
PB/ai_moderation.proto
deployments/docker/
├── ai-moderation.Dockerfile         # 不含模型文件
deployments/k8s/
└── ai-moderation-models-pvc.yaml     # 模型文件 PVC 挂载
```

Content Service 改动：
```
cmd/content/
├── service/post_service.go            # CreatePost 集成 AI 调用
├── service/async_ai_consumer.go       # 新增异步补判 Consumer
├── service/async_review_scheduler.go  # 新增定时调度器（每日 02:00）
├── service/taken_down_finalizer.go    # 新增宽限期结束器（每小时）
└── database/ai_audit_dao.go            # 新增 ai_audit_logs DAO
pkg/aiclient/                          # 新增：ai-moderation 客户端封装 + 熔断
├── client.go
└── circuit.go
```

### References

- [Content Service v1.0 PRD](docs/content-service-prd.md)
- [Content Service v2.1 PRD](docs/content-service-v2-prd.md)
- [User Service v2.0 PRD](docs/user-service-v2.0-prd.md) — 管理员审核入口
- [onnxruntime-go GitHub](https://github.com/yalue/onnxruntime_go) — 模型推理库
- [sony/gobreaker](https://github.com/sony/gobreaker) — 熔断器
- [HFL Chinese-BERT](https://github.com/ymcui/Chinese-BERT-wwm) — 中文预训练模型候选
- [robfig/cron](https://github.com/robfig/cron) — Go 定时任务库

---

## 🟡 待确认项（review 时一次性决定）

| # | 待定项 | 当前默认（rev2 更新） | 用户确认 |
|---|--------|---------------------|---------|
| 1 | ONNX 模型选型 | HFL Chinese-BERT-wwm + 自训分类头 | ☐ |
| 2 | 模型文件托管方式 | **独立 volume 挂载**（不入镜像）| ☐ |
| 3 | AI 同步超时阈值 | 800ms（可配置） | ☐ |
| 4 | AI 决策阈值 | ≥0.9 pass / 0.5-0.9 review / <0.5 block（**可配置**，根据上线后数据调整）| ☐ |
| 5 | ai-moderation 与 Content 间通信加密 | 本期内网明文（Phase 2 升级 mTLS） | ☐ |
| 6 | ai_audit_logs 保留天数 | 180 天 | ☐ |
| 7 | 异步补判触发频率 | **实时入队 + 每日 02:00 兜底扫描** | ☐ |
| 8 | 异步下架宽限期 | **24h**（可配置）| ☐ |
| 9 | Consumer 并发数 | **5-10 goroutine**（可配置）| ☐ |

---

*This PRD is rev2 — fixed 14 issues from rev1 review (5 critical, 5 medium, 4 minor). All architectural decisions collected; remaining 9 items marked 【待定】 for user confirmation.*