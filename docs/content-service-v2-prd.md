# 产品需求文档：Content Service v2.1 — 异步链路激活 & 二级评论

**版本**：2.1
**日期**：2026-06-25
**作者**：Sarah（产品负责人）
**质量评分**：90/100
**前置版本**：v1.0（docs/content-service-prd.md，2026-06-08）

---

## 执行摘要

Content Service v1.0 已完成审核流、MQ 发布、ES 客户端封装等基础能力，但**异步链路未实际贯通**：审核通过事件已发出，但 ES 同步消费者从未启动，导致搜索功能停留在"代码完整但搜不到任何东西"的空架子状态。同时，一级评论已交付，但**互动深度不足**——用户无法对某条评论回复，也无 @ 提及机制。

v2.1 聚焦两个目标：

1. **激活异步链路**：在 `cmd/content/main.go` 中启动 `ESSyncConsumer`，订阅 `content.events` 队列，让"审核通过 → ES 索引 → 可被搜索"形成完整闭环。
2. **扩展二级评论能力**：在现有 `CreateComment` 上扩展 `parent_id`（父评论 ID）和 `mentioned_user_ids`（被 @ 用户列表），新增 `comment_mentions` 表记录 @ 关系，发布 `content.replied` MQ 事件**为下一期独立 Message Service 做好契约预留**。

本期**不包含**通知消费者实现（独立 `cmd/message/` 服务留待下一期）。但通过清晰的 MQ 事件 schema 设计，未来消费方无需改动即可对接。

---

## 问题陈述

### 当前痛点

| 痛点 | 影响 |
|------|------|
| ES 同步消费者已实现但未启动 | 审核通过 1 万篇帖子，但 ES 索引为 0，搜索功能完全失效 |
| 评论只支持一级，无回复入口 | 用户无法对评论互动，互动深度不足，社交粘性低 |
| 无 @ 提及机制 | 用户希望联系特定人需手动拼接昵称，无法精准通知 |
| 通知链路完全缺失 | `content.liked` / `content.review_result` 事件有发布但无消费者 |

### 解决方案

- 在 `cmd/content/main.go` 主进程启动后开启 goroutine 运行 `ESSyncConsumer.Start(ctx)`，订阅 `content.events` 队列，复用已有的 `pkg/mq/consumer.go` 自动重连能力。
- 扩展 `CreateCommentRequest` 字段（`parent_id` + `mentioned_user_ids`），新增 `comment_mentions` 表 + 级联软删除逻辑。
- 设计并发布 `content.replied` MQ 事件（轻量、forward-compat），等待下一期 `cmd/message/` 服务消费。

### 业务价值

- **搜索可用**：修复后用户可基于关键词/分类筛选搜索校园内帖子，是内容发现的核心入口。
- **互动深化**：二级评论 + @ 提及让帖子从"单向发布"转为"多人讨论"，提升 DAU 与停留时长。
- **架构可演进**：MQ 事件 schema 提前设计，下一期消息服务可零成本接入。

---

## 成功指标

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| ES 同步延迟 P95 | 审核通过后 < 5 秒内可被搜索到 | 单元测试 + 集成测试 + Jaeger trace 比对 |
| ES 同步成功率 | ≥ 99.9% | MQ 队列监控 + DLQ 重投率 |
| 二级评论创建成功率 | ≥ 99.5% | 接口日志统计 |
| @mention 校验通过率 | 100% 校验 mentioned_user_id 存在性 + 同学校 | 单元测试覆盖 |
| MQ 事件发布成功率 | `content.replied` 100% 入队（best-effort 降级日志） | RabbitMQ 管理后台监控 |
| `cmd/content/main.go` 启动时间 | 引入 ES Consumer 后启动时间增长 < 1s | 本地启动计时 |

---

## 用户画像

### 主用户：发帖者/浏览者（沿用 v1.0）

- **角色**：在校大学生
- **目标**：发布/浏览失物招领、二手交易、通用帖子
- **痛点**：搜不到内容、互动深度不够
- **本期新增需求**：能搜到、能对评论回复并 @ 特定人

### 新增关注方：被 @ 用户（虚拟角色，本期无感知）

- **角色**：被评论者 @ 的校内用户
- **目标**：未来收到"某人在帖子 X 的评论中 @ 了你"的通知
- **本期状态**：事件已发布但无消费者，本期不感知
- **下期上线**：`cmd/message/` 服务消费 `content.replied` 事件，落到 `notifications` 表，提供通知列表 API

---

## 用户故事与验收标准

### Story 1：审核通过后用户能搜到帖子

**作为** 一名浏览者
**我想要** 在搜索框输入关键词查找帖子
**以便于** 找到失物招领、二手交易等相关内容

**验收标准：**
- [ ] `cmd/content/main.go` 启动时开启 goroutine 调用 `ESSyncConsumer.Start(ctx)`
- [ ] Consumer 订阅 `content.events` 队列（队列名硬编码在 `pkg/mq`）
- [ ] 处理 `content.published` 事件：从 MySQL 读取帖子完整数据，调用 `esClient.IndexPost` 索引
- [ ] 处理 `content.taken_down` 事件：调用 `esClient.DeletePost` 从 ES 删除
- [ ] 处理失败时消息 Nack+requeue，由 `pkg/mq/consumer.go` 自带的指数退避重试
- [ ] 优雅停止：收到 SIGTERM 时调用 `ESSyncConsumer.Stop()`，确保 in-flight 消息处理完成
- [ ] 单测覆盖：消费者注册、handler 选择、context 透传
- [ ] 端到端验证脚本：mock 一个 `content.published` 事件 → 验证 ES 索引中可搜到该帖子

---

### Story 2：用户对某条评论进行回复

**作为** 一名评论者
**我想要** 对帖子下的某条一级评论回复
**以便于** 与该评论作者或其他讨论者互动

**验收标准：**
- [ ] `CreateCommentRequest` 新增可选字段：`parent_id int64`（0 = 一级评论）、`mentioned_user_ids []int64`
- [ ] 业务约束：`parent_id != 0` 时，其指向的评论必须是**一级评论**（即父评论 `parent_id == 0`），禁止二级评论下再嵌套
- [ ] 校验：父评论必须存在、未被删除、属于同一 `school_id`
- [ ] 写入时 `parent_id` 持久化到 `post_comments.parent_id`
- [ ] 单测覆盖：parent_id=0、parent_id=有效一级评论、parent_id=二级评论（应被拒绝）、parent_id 不存在、跨 school_id

---

### Story 3：评论中 @ 某用户（事件级预留）

**作为** 一名评论者
**我想要** 在回复中 @ 一位校内同学
**以便于** 让 ta 注意到我的回复（未来通过通知服务实现）

**验收标准：**
- [ ] 新增数据表 `comment_mentions`：
  - `id` (PK, snowflake)
  - `school_id` (index)
  - `comment_id` (index)
  - `mentioned_user_id` (index)
  - `created_at`
  - 唯一索引：`(comment_id, mentioned_user_id)`
- [ ] 业务校验：
  - `mentioned_user_ids` 数量上限 5（防滥用）
  - 所有 ID 必须 > 0 且互不相同
  - 每个 ID 必须存在于 `users` 表（跨服务校验：调用 user-service gRPC `GetUserExists(user_id, school_id)` 或本期降级为只校验 `school_id` 一致性，下一期接入 user-service 后做强校验）
- [ ] 写入：评论创建成功后，在同一事务内批量插入 `comment_mentions` 记录
- [ ] 单测覆盖：mentioned_user_ids 数量超限、含重复 ID、含不存在 ID

---

### Story 4：发布 `content.replied` MQ 事件（forward-compat）

**作为** 系统
**我想要** 当一条回复被创建时发布 `content.replied` 事件
**以便于** 下一期独立 Message Service 可零成本消费并通知被 @ 用户

**验收标准：**
- [ ] 新增常量 `mq.EventContentReplied = "content.replied"`
- [ ] 事件 schema（JSON）：
  ```json
  {
    "type": "content.replied",
    "post_id": 12345,
    "school_id": 100,
    "user_id": 678,                 // 回复者
    "trace_id": "abc-def-...",
    "time": "2026-06-25T10:30:00Z",
    "data": {
      "parent_comment_id": 999,
      "parent_comment_user_id": 555, // 父评论作者（潜在通知目标）
      "mentioned_user_ids": [555, 666],
      "content_preview": "我也觉得..."  // 前 50 字，避免重事件体
    }
  }
  ```
- [ ] 发布时机：`CreateComment` 中 `parent_id != 0` 且成功后，best-effort 发布，失败仅记日志
- [ ] 兼容旧事件类型：`pkg/mq/publisher.go` 常量保持向后兼容（不删不改既有 `content.published` 等）

---

### Story 5：删除一级评论时级联软删除其下回复

**作为** 一名评论作者
**我想要** 删除自己的一级评论
**以便于** 同时清除所有针对它的回复

**验收标准：**
- [ ] `DeleteComment` 中：若被删除评论为一级评论（`parent_id == 0`），事务内同时软删除所有 `parent_id = 该评论ID` 的回复
- [ ] 二级评论的删除逻辑不变（仅作者本人，事务内递减 `comment_count`）
- [ ] 列表接口过滤：`ListComments` 默认只返回一级评论 + status=1；新增 `ListCommentReplies(parent_id)` 返回指定父评论下的所有回复
- [ ] 单测覆盖：一级评论无回复、单条回复、多条回复；删除二级评论不影响一级

---

## 功能需求

### FR-1：ES 同步消费者激活

**功能描述**：
- 在 `cmd/content/main.go` 主进程启动 gRPC server 后，开启独立 goroutine 运行 `ESSyncConsumer`
- Consumer 订阅 `content.events` 队列，复用 `pkg/mq/consumer.go` 的连接管理/重连/重试机制
- 处理两类事件：`content.published`（索引） / `content.taken_down`（删除）

**用户流程**（系统视角）：
1. 管理员在 admin-service 调用 `ApprovePost` → Content Service 更新 MySQL → 发布 `content.published` 事件
2. ESSyncConsumer 接收事件 → 从 MySQL 读完整帖子 → 索引到 ES
3. 用户通过 search API 可搜到该帖子

**边缘场景**：
- **MySQL 读不到帖子**：返回错误 → Nack+requeue → 30 秒后重试（指数退避）
- **ES 索引失败**：同上重试
- **MQ 连接断开**：`pkg/mq/consumer.go` 自动重连，注册 handler 不丢失
- **重复消费**：ES 索引幂等（同一 post_id 覆盖写），无需额外去重

**错误处理**：
- 失败消息通过 Nack+requeue 重试
- 长期失败（>5 次）：本期不引入 DLQ，记录 ERROR 日志供运维介入
- MQ Publisher/Consumer 任一不可用：服务降级运行，仅记录日志，不影响 gRPC 主流程

---

### FR-2：二级评论 API

**功能描述**：
- 扩展 `pb.CreateCommentRequest`：
  ```protobuf
  message CreateCommentRequest {
    int64 school_id = 1;
    int64 post_id = 2;
    int64 user_id = 3;
    string content = 4;
    int64 parent_id = 5;              // 新增：0=一级评论，>0=二级回复
    repeated int64 mentioned_user_ids = 6;  // 新增：被 @ 用户列表
  }
  ```
- 限制：`parent_id` 只能指向一级评论，**不允许二级评论下再嵌套回复**

**用户流程**：
1. 用户在帖子详情页点击某条评论的"回复"按钮 → 客户端携带 `parent_id` 调用 `CreateComment`
2. Content Service 校验父评论存在性 + 合法性 → DFA 敏感词扫描 → 雪花 ID → 事务写入评论 + mentions
3. 发布 `content.replied` MQ 事件

**边缘场景**：
- `parent_id` 指向二级评论 → 拒绝（"仅支持二级回复，不允许嵌套"）
- `parent_id` 指向不存在评论 → 404
- `parent_id` 跨 `school_id` → 403
- `parent_id` 指向已删除评论 → 410 Gone（提示用户"该评论已删除"）

**错误处理**：
- 参数非法 → `INVALID_ARGUMENT`
- 父评论不存在 → `NOT_FOUND`
- 父评论被删除 → `FAILED_PRECONDITION`
- 跨学校 → `PERMISSION_DENIED`
- 敏感词命中 → `FAILED_PRECONDITION` + `SensitiveWordError`（沿用 v1.0）

---

### FR-3：@ mention 数据模型与校验

**功能描述**：
- 新表 `comment_mentions` 持久化 @ 关系
- 创建评论时同步插入 mentions 记录（同事务）

**校验规则**：
- `mentioned_user_ids` 长度 0 ≤ N ≤ 5
- 去重后逐个校验：
  - 每个 ID > 0
  - 同 `school_id`（通过调用 user-service `GetUserSchool(user_id)` 或本期降级为**信任客户端传入**，仅校验非空非负）
- 本期降级说明：`User Service` 尚未实现 → 本期 mentions 仅做存在性 + 数量校验，**不下游调用**；下期 User Service 上线后，在 `service` 层加 gRPC 校验

**边缘场景**：
- mentioned_user_ids 含 0 或负数 → 422
- 重复 ID → 自动去重
- 超 5 个 → 422 "最多 @ 5 人"

**错误处理**：校验失败 → `INVALID_ARGUMENT` + 详细错误消息

---

### FR-4：MQ `content.replied` 事件

**功能描述**：
- 二级评论（`parent_id != 0`）创建成功后，发布 `content.replied` 事件
- 事件 payload 轻量化，仅包含**身份信息** + `content_preview`（前 50 字），消费者按需查 DB 补全
- best-effort 发布：失败仅记 ERROR 日志，**不阻塞评论创建返回**

**轻量化理由**：
- 通知服务消费时可主动查 MySQL 拿最新数据，避免事件与 DB 不一致
- 减小 MQ 带宽占用，支持未来高吞吐场景

**forward-compat 约束**：
- 不修改既有事件常量（`content.published` / `content.review_result` / `content.taken_down` / `content.liked`）
- 新事件类型仅追加，不影响旧消费者

**错误处理**：发布失败 → ERROR 日志（trace_id 关联），主流程不感知

---

### FR-5：级联软删除

**功能描述**：
- 删除一级评论时，同事务内软删除所有 `parent_id = 该评论ID` 的回复
- 不物理删除（保留 audit trail）

**用户流程**：
1. 用户点击某条一级评论的"删除"
2. 后端事务：
   - 校验 ownership
   - UPDATE `post_comments` SET `status=2`, `deleted_at=NOW()` WHERE `id = ?` AND `school_id = ?`
   - 同事务：UPDATE `post_comments` SET `status=2`, `deleted_at=NOW()` WHERE `parent_id = ?` AND `school_id = ?`
   - 递减帖子 `comment_count`（仅递减一级评论的 1 条，二级评论的 count 已包含在父评论的计数中——确认：v1.0 的 `CreateComment` 是每次都 +1，所以级联删除二级时也要 -N）
- 列表接口自动过滤 `status=2`

**边缘场景**：
- 删除非一级评论 → 不级联（仅删自己）
- 一级评论下无回复 → 走原有逻辑
- 删除过程中新回复进来 → 锁表或乐观锁避免 race（本期接受小概率 race，下期优化）

**错误处理**：
- ownership 校验失败 → 403
- 评论不存在 → 404

---

## 技术约束

### 性能

- ES 同步延迟 P95 < 5s（从 ApprovePost 返回到 ES 可搜）
- 二级评论创建接口 P95 < 200ms（含 mentions 写入）
- `ListCommentReplies` 接口 P95 < 150ms（带索引）

### 安全

- mentioned_user_ids 本期降级为只校验数量 + 非负数，下期接入 user-service 后做强校验（同学校 + 存在性）
- 所有接口强制 `school_id` 隔离，跨学校访问返回 403
- DFA 敏感词扫描沿用 v1.0 实现，命中则拒绝（评论 + 帖子一致策略）

### 集成

- **RabbitMQ**：复用 `pkg/mq`（publisher/consumer）
- **Elasticsearch**：复用 `pkg/es/client.go`，索引 `campus_posts`
- **gRPC**：Content Service 现有接口不变，仅扩展字段
- **未来 user-service**：下期接入，本期 mentions 校验降级

### 技术栈

- Go 1.22+
- GORM v2（自动迁移 `comment_mentions` 表）
- amqp091-go（MQ）
- go-elasticsearch v8（ES）

---

## MVP 范围与分期

### Phase 1（本 PRD，必交付）

| 模块 | 范围 |
|------|------|
| **ES 同步消费者激活** | main.go 启动 goroutine + ESSyncConsumer；处理 2 类事件；优雅停止 |
| **二级评论 API** | CreateComment 扩展 parent_id + mentioned_user_ids；业务校验；级联软删除 |
| **@mention 数据模型** | comment_mentions 表 + 索引 + 事务写入 |
| **MQ content.replied 事件** | 常量定义 + 轻量 payload + best-effort 发布 |
| **数据迁移** | AutoMigrate comment_mentions 表 |
| **测试** | 单元测试覆盖：cursor/mention/cascade/MQ event |

### Phase 2（下一期，不在本 PRD）

- 独立 `cmd/message/` 服务（含独立数据库 `campus_message`）
- 消费 `content.replied` / `content.liked` / `content.review_result` 事件
- 通知列表 API（站内消息中心）
- @mention 强校验（调用 user-service）
- 微信订阅消息推送（可选）

### 显式 Out of Scope

- ❌ 多层嵌套回复（仅支持二级）
- ❌ 通知消费者实现（独立 Message Service 下一期）
- ❌ 微信订阅消息推送
- ❌ @ 自动补全前端 UI（前后端解耦，前端自行实现）
- ❌ 通知 WebSocket 实时推送（轮询即可）
- ❌ ES 索引结构升级（沿用 v1.0 `PostDocument`）
- ❌ 全量 reindex 工具（手动从 MySQL 重建 ES 留待运维自助）

---

## 风险评估

| 风险 | 概率 | 影响 | 缓解策略 |
|------|------|------|---------|
| ES 同步消息积压 | 中 | 中 | 复用 `pkg/mq/consumer.go` 自带重试 + 队列深度监控告警 |
| 级联软删除性能差 | 低 | 低 | 单事务内完成，少量回复场景足够；下期按 `parent_id` 索引优化 |
| mentioned_user_id 伪造/越权 | 中 | 中 | 本期仅做格式校验；下期 user-service 上线后做强校验 |
| MQ 事件丢失导致未来消息服务无数据可消费 | 低 | 高 | Phase 2 消费者加入定期从 MySQL `comment_mentions` 表扫描补偿机制 |
| parent_id 校验不严导致非法嵌套 | 低 | 中 | 强校验 `parent_id` 指向的评论必须是 `parent_id=0`；单测覆盖 |
| 启动时间增加 ES Consumer | 低 | 低 | goroutine 异步启动，不阻塞 gRPC server `Serve` |
| AutoMigrate 在已部署环境失败 | 低 | 高 | 本期 `comment_mentions` 表为新增，部署文档说明需提前备份；下期考虑独立 migration 工具 |

---

## 依赖与阻塞

### 依赖

| 依赖项 | 描述 | 负责人 |
|--------|------|--------|
| RabbitMQ 服务 | `localhost:15672`（配置文件 `my_config.yaml`） | 运维 |
| Elasticsearch 集群 | `pkg/es/client.go` 已封装 | 运维 |
| Snowflake ID 生成器 | 复用 `pkg/snowflake/snowflake.go`（comment + mention 共用节点） | — |
| MySQL 数据库 | `campus_content`（新增 `comment_mentions` 表） | 运维 |
| Jaeger | TraceID 全链路透传（MQ Header + ContentEvent.TraceID） | 运维 |

### 已知阻塞

无。当前 Content Service v1.0 的所有依赖均已就绪，本期仅做扩展，不引入新外部依赖。

---

## 附录

### 术语表

| 术语 | 定义 |
|------|------|
| **ES Sync Consumer** | 监听 `content.events` 队列的消费者，将帖子变更同步到 Elasticsearch |
| **parent_id** | 评论的父评论 ID；0 = 一级评论，>0 = 二级回复 |
| **@mention** | 评论中 @ 用户的机制，持久化到 `comment_mentions` 表 |
| **级联软删除** | 删除一级评论时同时软删除其下所有二级回复 |
| **content.replied** | 二级评论创建时发布的 MQ 事件，forward-compat 预留 |
| **forward-compat** | 当前设计兼顾未来扩展（如下一期 Message Service） |

### 参考资料

- **v1.0 PRD**：`docs/content-service-prd.md`（2026-06-08）
- **MQ 事件定义**：`pkg/mq/publisher.go`（ContentEvent 结构 + 事件常量）
- **MQ 消费者**：`pkg/mq/consumer.go`（自动重连 + 指数退避）
- **ES 客户端封装**：`pkg/es/client.go`（IndexPost / DeletePost / Search）
- **ES 同步消费者**：`cmd/content/service/es_sync.go`（已实现但未启动）
- **状态机**：`cmd/content/model/post.go`（`allowedTransitions`）
- **DFA 敏感词**：`pkg/sensitive/dfa.go`
- **游标分页**：`cmd/content/service/post_service.go`（`encodeCursor` / `parseCursor`）

### 数据迁移 SQL（参考）

```sql
-- comment_mentions 表 AutoMigrate 会自动创建，以下为参考 DDL
CREATE TABLE IF NOT EXISTS comment_mentions (
    id BIGINT PRIMARY KEY,
    school_id BIGINT NOT NULL,
    comment_id BIGINT NOT NULL,
    mentioned_user_id BIGINT NOT NULL,
    created_at DATETIME NOT NULL,
    UNIQUE KEY uk_comment_user (comment_id, mentioned_user_id),
    KEY idx_school (school_id),
    KEY idx_mentioned (mentioned_user_id)
);
```

---

*本 PRD 通过 5 轮迭代式需求对话生成，质量评分 90/100，覆盖业务、功能、UX、技术、范围五大维度。所有关键决策（异步链路激活范围、二级评论形态、@mention 设计、级联删除策略、API 扩展 vs 独立接口、MQ 事件 schema）均已与产品负责人确认。*