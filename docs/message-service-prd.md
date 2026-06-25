# 产品需求文档：Message Service（消息通知服务）

**版本**：1.0
**日期**：2026-06-25
**作者**：Sarah（产品负责人）
**质量评分**：92/100
**前置依赖**：Content Service v2.1（依赖 MQ 事件生产端）

---

## 执行摘要

校园互助平台当前存在一个**通知真空**：用户点赞帖子、评论被回复、帖子审核结果、违规下架等事件已被 Content Service 发布到 RabbitMQ，但没有任何服务消费这些事件——用户完全不知道自己收到了点赞或回复，审核结果也无法触达发帖人。

Message Service 填补这个空白：作为独立微服务消费通知事件、持久化到 MySQL、通过 Gateway 对外提供 RESTful 通知列表 API。同时，本期在 Content Service 中补齐 `content.replied` 事件的发布逻辑，形成"事件发生 → MQ 投递 → 通知持久化 → 用户查看"的完整闭环。

本期不包含微信订阅消息推送，所有通知为站内信形式。

---

## 问题陈述

**当前状态**：

| 痛点 | 影响 |
|------|------|
| 点赞事件（`content.liked`）有发布无消费 | 帖子作者不知道谁赞了自己的帖子 |
| 审核结果事件有发布无消费 | 发帖人审核被拒后不会收到任何提示，一直以为还在审核中 |
| 违规下架事件仅有 ES 消费者 | 发帖人不知道帖子被下架及原因 |
| 评论回复事件（`content.replied`）代码未实现 | 用户回复评论后，被回复者没有任何感知 |
| 无通知存储和查询能力 | 用户无法在任何位置查看历史通知 |

**解决方案**：

1. **Content Service 侧**：新增 `notification.events` 队列 Publisher，通知类事件（liked / review_result / taken_down / replied）双投递到 `content.events`（ES 同步保留）和 `notification.events`（本服务消费）；补齐 `content.replied` 事件发布逻辑
2. **Message Service 侧**：独立微服务（`cmd/message/`），独立 MySQL 库（`campus_message`），消费 `notification.events` 队列，持久化到 `notifications` 表
3. **Gateway 侧**：新增通知类路由（JWT 鉴权，不需强制 school 绑定）

**业务价值**：
- 消除"通知真空"，让用户感知到社交互动（点赞/回复），提升 DAU 与留存
- 审核结果可触达，降低用户困惑与客服压力
- 违规下架有通知，提升平台治理透明度和合规性
- 统一的站内通知中心为后续私信、系统公告等扩展奠定数据基础

---

## 成功指标

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| 事件到通知入库延迟 P95 | < 3 秒 | MQ 事件时间 vs 通知 created_at 比对 |
| 通知列表 API P95 | < 100ms | 性能测试 |
| 通知创建成功率 | ≥ 99.9% | 事件投递量与通知创建量比对 |
| `content.replied` 事件发布覆盖 | 100% 二级评论创建时发布 | 单元测试 + 断言 |
| 优雅停止不丢消息 | 0 丢失 | SIGTERM 时等待 inflight 消息确认 |

---

## 用户画像

### 主用户：校园普通用户（A 角）

- **角色**：在校大学生，发帖/评论/点赞的活跃用户
- **目标**：及时知道有人点赞、回复自己的内容；知道帖子审核结果
- **痛点**：发帖后石沉大海，不知道审核过了没；不知道有人回复了自己
- **技术层度**：普通（仅使用小程序，不关心后端实现）

### 次用户：管理员/审核员（B 角）

- **角色**：学校管理员或平台审核团队
- **目标**：确认违规下架通知已送达发帖人
- **技术层度**：普通

---

## 用户故事与验收标准

### Story 1：帖子被点赞后收到通知

**作为** 一名发帖用户
**我想要** 在有人点赞我的帖子后收到通知
**以便于** 知道谁对我的内容感兴趣

**验收标准：**
- [ ] Content Service 发布 `content.liked` 事件时同时投递到 `notification.events` 队列
- [ ] Message Service 消费后创建通知记录，标题格式：「{nickname} 赞了你的帖子「{title}」」
- [ ] 通知包含 `ref_type=post` + `ref_id=post_id`，点击可跳转到帖子详情页
- [ ] 自己赞自己的帖子不生成通知（`user_id == target_user_id` 跳过）

### Story 2：帖子审核结果通知

**作为** 一名发帖用户
**我想要** 帖子审核通过或被拒绝时收到通知
**以便于** 及时知道帖子是否上架，被拒时了解原因

**验收标准：**
- [ ] Content Service `ApprovePost` 方法发布事件时同时投递到 `notification.events`
- [ ] Content Service `RejectPost` 方法同上
- [ ] 审核通过通知标题：「你的帖子「{title}」审核已通过」
- [ ] 审核拒绝通知标题：「你的帖子「{title}」审核未通过」，内容包含拒绝原因
- [ ] 点击通知跳转到帖子详情页

### Story 3：帖子被下架后收到通知

**作为** 一名发帖用户
**我想要** 我的帖子因违规被下架时收到通知
**以便于** 了解违规原因并及时整改

**验收标准：**
- [ ] Content Service `TakedownPost` 发布事件时同时投递到 `notification.events`
- [ ] 通知标题：「你的帖子「{title}」因违规已下架」
- [ ] 通知内容包含下架原因
- [ ] 点击通知跳转到帖子详情页

### Story 4：评论被回复后收到通知

**作为** 一名评论用户
**我想要** 有人回复我的评论时收到通知
**以便于** 参与讨论互动

**验收标准：**
- [ ] Content Service `CreateComment` 中 `parent_id != 0` 时发布 `content.replied` 事件
- [ ] 事件投递到 `notification.events` 队列
- [ ] 通知标题：「{nickname} 回复了你的评论：「{preview}」」
- [ ] 预览截取回复内容前 50 个字符
- [ ] 点击通知跳转到帖子详情页

### Story 5：查看通知列表

**作为** 一名普通用户
**我想要** 查看我的所有通知
**以便于** 回顾点赞、回复等互动记录

**验收标准：**
- [ ] `GET /api/v1/notifications` 分页返回当前用户的通知列表（cursor 分页或 offset 分页）
- [ ] 支持按 `type` 筛选（liked / review_result / taken_down / replied）
- [ ] 每条通知包含：id、type、title、content、is_read、created_at、ref_type、ref_id
- [ ] 默认按 created_at DESC 排序
- [ ] 返回值包含 `unread_count`，供前端展示小红点

### Story 6：获取未读数

**作为** 一名普通用户
**我想要** 快速看到未读通知数量
**以便于** 在首页/导航栏展示小红点

**验收标准：**
- [ ] `GET /api/v1/notifications/unread-count` 返回 `{"count": N}`
- [ ] 实时 COUNT 查询（不缓存），保证准确性

### Story 7：标记已读

**作为** 一名普通用户
**我想要** 将通知标记为已读
**以便于** 消除未读提示

**验收标准：**
- [ ] `PUT /api/v1/notifications/:id/read` 标记单条为已读
- [ ] `PUT /api/v1/notifications/read-all` 批量标记当前用户所有通知为已读
- [ ] 已读操作后 `unread_count` 相应减少
- [ ] 重复标记已读为幂等操作

### Story 8：删除通知

**作为** 一名普通用户
**我想要** 删除不需要的通知
**以便于** 保持通知列表整洁

**验收标准：**
- [ ] `DELETE /api/v1/notifications/:id` 软删除单条通知
- [ ] 仅本人可删除自己的通知（校验 user_id）
- [ ] 系统定时清理 30 天前的通知（物理删除，可配置）

---

## 功能需求

### FR-1：Content Service 双队列投递

**功能描述**：Content Service 现有的 `publishEvent` 在向 `content.events` 投递的同时，通知类事件额外投递到 `notification.events`。

**受影响的事件类型**：

| 事件 | 投递 content.events | 投递 notification.events |
|------|:---:|:---:|
| `content.published` | ✅（ES 索引） | ✅（通知用户审核通过） |
| `content.review_result` | — | ✅（通知用户审核被拒） |
| `content.taken_down` | ✅（ES 删除） | ✅（通知用户帖子下架） |
| `content.liked` | — | ✅（通知帖子作者被点赞） |

> `content.published` 和 `content.taken_down` 已在 `content.events` 中有 ES Consumer，双投递后 ES 消费者不变，Message Service 从 `notification.events` 消费新增的投递。

**实现要点**：
- `cmd/content/service/post_service.go` 中新增 `notificationPublisher`（队列名 `notification.events`）
- `publishEvent` 或新的辅助函数中，判断事件类型是否属于通知类，若是则同时调用 `notificationPublisher.Publish()`
- `InitMQ` 中初始化两个 Publisher
- **`content.liked` 事件**：当前使用字符串字面量 `"content.liked"`，需要定义为常量（`EventContentLiked = "content.liked"`），纳入 `mq` 包常量管理

### FR-2：Content Service — 发布 content.replied 事件

**功能描述**：`CreateComment` 中当 `parent_id != 0`（即二级回复创建成功）时，发布 `content.replied` 事件。

**事件 Schema**：
```json
{
  "type": "content.replied",
  "post_id": 12345,
  "school_id": 100,
  "user_id": 678,
  "trace_id": "abc-def-...",
  "time": "2026-06-25T10:30:00Z",
  "data": {
    "parent_comment_id": "999",
    "parent_comment_user_id": "555",
    "content_preview": "我也觉得..."
  }
}
```

**实现要点**：
- 在 `CreateComment` 中判断 `parent_id != 0` 且事务提交成功后发布
- 从父评论查询 `parent_comment_user_id`（父评论作者的 user_id，即通知接收方）
- `content_preview` 截取 `content` 前 50 个字符
- `data` 中 string 值为 string 类型（RabbitMQ 序列化要求，所有 `map[string]string`）

### FR-3：Message Service 事件消费

**功能描述**：Message Service 启动时初始化 MQ Consumer，订阅 `notification.events` 队列，注册 4 种事件处理器。

**事件 → 通知映射**：

| 事件 | 处理器 | title 格式 | 通知对象 |
|------|--------|-----------|---------|
| `content.liked` | `handleLiked` |「{actor_nickname} 赞了你的帖子「{post_title}」」 | 帖子作者 |
| `content.published` | `handleReviewResult` |「你的帖子「{post_title}」审核已通过」 | 帖子作者 |
| `content.review_result` | `handleReviewResult` |「你的帖子「{post_title}」审核未通过」+ 原因 | 帖子作者 |
| `content.taken_down` | `handleTakenDown` |「你的帖子「{post_title}」因违规已下架」+ 原因 | 帖子作者 |
| `content.replied` | `handleReplied` |「{actor_nickname} 回复了你的评论：「{preview}」」 | 父评论作者 |

**消费者注册**：
```go
consumer.RegisterHandler("content.liked", s.handleLiked)
consumer.RegisterHandler("content.published", s.handleReviewResult)
consumer.RegisterHandler("content.review_result", s.handleReviewResult)
consumer.RegisterHandler("content.taken_down", s.handleTakenDown)
consumer.RegisterHandler("content.replied", s.handleReplied)
```

**通知创建流程**：
1. 收到事件 → 解析 event type + data
2. 从 event `data` 或回调 User Service 获取 `actor_nickname`
3. 从 event `post_id` 回调 Content Service 获取 `post_title`
4. 格式化 title 字符串
5. INSERT INTO notifications（同一事务不跨服务，每个通知独立 INSERT）
6. Ack 消息

> **性能要点**：步骤 2-3 中回调 User/Content Service 获取昵称和标题属于**同步调用**。高频场景下（如批量点赞），可使用 Redis 缓存 user nickname 减少跨服务调用。本期 MVP 不做缓存优化，下期按需引入。

### FR-4：通知数据模型

```sql
CREATE TABLE notifications (
    id              BIGINT       PRIMARY KEY,              -- 雪花算法 ID
    school_id       BIGINT       NOT NULL,                 -- 学校隔离
    user_id         BIGINT       NOT NULL,                 -- 接收通知的用户
    type            VARCHAR(32)  NOT NULL,                 -- liked / published / review_result / taken_down / replied
    title           VARCHAR(255) NOT NULL,                 -- 通知标题（含快照，如「张三 赞了你的帖子「求购自行车」」）
    content         VARCHAR(500) DEFAULT '',               -- 通知正文/预览（审核原因、回复预览等）
    from_user_id    BIGINT       DEFAULT 0,                -- 触发者 user_id（可为 0 表示系统通知）
    ref_type        VARCHAR(32)  DEFAULT '',               -- 关联类型：post / comment
    ref_id          BIGINT       DEFAULT 0,                -- 关联 ID：post_id / comment_id
    is_read         TINYINT(1)   DEFAULT 0,                -- 0=未读，1=已读
    created_at      DATETIME     NOT NULL,
    deleted_at      DATETIME     DEFAULT NULL,              -- 软删除（用户手动删除）
    INDEX idx_user_read (user_id, is_read, created_at DESC),
    INDEX idx_cleanup (created_at)                         -- 用于 30 天定时清理
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**标题快照策略**：title 字段在创建时即格式化完成并持久化，包含用户昵称和帖子标题的"快照"。即使后续昵称修改或帖子删除，通知内容不受影响。

### FR-5：RESTful API

所有路由注册在 **Gateway**，JWT 鉴权，不需强制 school 绑定。

| 方法 | 路由 | Handler | 说明 |
|------|------|---------|------|
| GET | `/api/v1/notifications` | ListNotifications | 分页通知列表，可选 type 筛选 |
| GET | `/api/v1/notifications/unread-count` | UnreadCount | 未读通知数 |
| PUT | `/api/v1/notifications/:id/read` | MarkRead | 标记单条已读 |
| PUT | `/api/v1/notifications/read-all` | MarkAllRead | 标记全部已读 |
| DELETE | `/api/v1/notifications/:id` | DeleteNotification | 软删除单条通知 |

**ListNotifications 响应格式示例**：
```json
{
  "notifications": [
    {
      "id": 10001,
      "type": "liked",
      "title": "张三 赞了你的帖子「求购自行车」",
      "content": "",
      "is_read": false,
      "ref_type": "post",
      "ref_id": 20001,
      "created_at": "2026-06-25T10:30:00Z"
    }
  ],
  "unread_count": 3,
  "has_more": false,
  "next_cursor": ""
}
```

### Out of Scope（本期不包含）

- ❌ 微信订阅消息推送
- ❌ WebSocket 实时通知推送（前端轮询即可）
- ❌ 通知按类型分组/聚合（如「N 人赞了你的帖子」）
- ❌ 私信/聊天功能
- ❌ 系统公告（全量推送）
- ❌ 通知铃声/震动设置
- ❌ 消息已读回执

---

## 技术约束

### 性能

- 事件消费到通知入库 P95 < 3s（含跨服务回调 User/Content Service）
- 通知列表 API P95 < 100ms（MySQL 索引覆盖）
- 通知创建吞吐量：支持 500 event/s（单个实例）
- `unread-count` API P95 < 50ms

### 安全

- 所有通知 API 强制 `user_id` = JWT 身份（用户只能看自己的通知）
- `school_id` 隔离：通知写入时从事件中带 school_id，查询时通过 Gateway 注入的 school_id 过滤
- 软删除：用户删除后保留 DB 记录 30 天，到期物理清理

### 集成

| 系统 | 集成方式 | 说明 |
|------|---------|------|
| **RabbitMQ** | 消费者（`pkg/mq/consumer.go`） | 订阅 `notification.events` |
| **MySQL** | GORM（独立库 `campus_message`） | `my_config.yaml` 新增 `messageDatabase` |
| **Gateway** | gRPC 客户端 + HTTP 路由 | 复用 JWT 中间件，新增 5 个路由 |
| **Content Service** | MQ 双投递 + 新增 `content.replied` | 改动 `post_service.go` + `mq` 包常量 |
| **User Service** | gRPC 回调获取 nickname | 回调 User Service `GetCurrentUser` |

### 技术栈

- Go 1.22+
- GORM v2（AutoMigrate `notifications` 表）
- RabbitMQ（amqp091-go，复用 `pkg/mq`）
- Snowflake ID（复用 `pkg/snowflake`）
- etcd 服务注册 + gRPC 通信（与 Gateway 交互模式同 Content Service）

### 队列架构

```
                content.published
                content.taken_down        ┌─────────────────┐
                content.liked        ──▶  │ content.events   │ ──▶ ES Sync Consumer（已有）
                content.review_result     └─────────────────┘
                
                content.published
                content.taken_down        ┌──────────────────────┐
                content.liked        ──▶  │ notification.events  │ ──▶ Message Service（本期新增）
                content.review_result     └──────────────────────┘
                content.replied（新增）
```

> Content Service 对同一事件同时投递两个队列（若事件同时需要 ES 同步 + 通知）。

---

## MVP 范围与分期

### Phase 1：MVP（本 PRD）

**Content Service 侧**：
- 新增 `notification.events` Publisher 双投递
- 新增 `EventContentLiked` 常量
- 为 `CreateComment` 补齐 `content.replied` 事件发布

**Message Service 侧**：
- `cmd/message/main.go` 启动入口（gRPC + MQ Consumer）
- `internal/message/service/` 5 种事件处理器
- `internal/message/model/notification.go` 数据模型
- `internal/message/repo/notification_repo.go` CRUD 操作
- MQ Consumer 订阅 `notification.events`

**Gateway 侧**：
- 新增 `cmd/gateway/handler/notification_handler.go`（5 个 handler）
- `cmd/gateway/router/app.go` 注册通知路由组
- `cmd/gateway/client/message_client.go` gRPC 客户端

**测试**：
- 单元测试覆盖：5 种事件处理器（mock MQ 事件）
- 单元测试覆盖：5 个通知 API handler
- 单元测试覆盖：Content Service `content.replied` 事件发布
- 端到端测试：发布 MQ 事件 → 确认通知入库 → API 查询到

### Phase 2：增强（下一期）

- 通知类型筛选/搜索增强
- 通知批量删除
- 点赞通知聚合（「张三等 5 人赞了你的帖子」）
- 通知设置（接收哪些类型的通知）

### 显式 Out of Scope

- ❌ 微信/邮件/短信推送
- ❌ WebSocket 实时推送
- ❌ 系统公告全量推送
- ❌ 私信聊天
- ❌ 通知分组聚合

---

## 风险评估

| 风险 | 概率 | 影响 | 缓解策略 |
|------|------|------|---------|
| MQ 队列名称变更导致消息丢失 | 低 | 高 | 事件类型常量为硬编码字符串，通过单测保证不变 |
| 跨服务回调（User/Content）导致消费延迟 | 中 | 中 | 回调使用超时 context（3s），超时降级只记录不阻塞 |
| 通知表数据量增长过快 | 低 | 中 | 30 天定时清理 + 索引 `idx_cleanup` |
| content.replied 事件在事务前发布 | 中 | 高 | 必须在事务提交成功后发布，避免回滚后有事件无评论 |
| 双队列投递中一个失败 | 中 | 低 | 两个 Publisher 独立运行，失败各自降级记录日志 |

---

## 依赖与阻塞

### 依赖

| 依赖项 | 描述 | 状态 |
|--------|------|------|
| RabbitMQ 服务 | `localhost:5672`，队列 `notification.events` | ✅ 已有 |
| MySQL | `campus_message` 数据库（需创建） | 需配置 |
| Snowflake | 通知 ID 生成复用现有雪花算法 | ✅ 已有 |
| User Service | 回调获取 `nickname`（需要 gRPC 客户端） | ✅ 已有 RPC |
| Content Service | 需发布 `content.replied` 事件 + 双队列投递 | 本期变更 |
| Gateway | 新增 message_client.go + 5 个路由 | 本期新增 |

### 已知阻塞

无。基础设施均已就绪。

---

## 附录

### 事件常量变更

`pkg/mq/publisher.go` 新增常量：

```go
const (
    EventContentLiked    = "content.liked"     // 新增加常量
    EventContentReplied  = "content.replied"   // 新增事件类型
)
```

### 配置变更

`my_config.yaml` 新增：

```yaml
mysql:
  messageDatabase: "campus_message"    # 新增 Message Service 数据库

service:
  message:
    name: "message-service"
    address: "127.0.0.1:50004"
    loadBalance: false
```

### 通知标题格式汇总

| 类型 | title 格式 | content |
|------|-----------|---------|
| `liked` | `{nickname} 赞了你的帖子「{title}」` | 空 |
| `published` | `你的帖子「{title}」审核已通过` | 空 |
| `review_result` | `你的帖子「{title}」审核未通过` | 拒绝原因 |
| `taken_down` | `你的帖子「{title}」因违规已下架` | 下架原因 |
| `replied` | `{nickname} 回复了你的评论：「{preview}」` | 完整回复内容 |

### 参考文档

- **Content Service v2.1 PRD**：`docs/content-service-v2-prd.md`（事件定义基准）
- **Gateway Service PRD**：`docs/gateway-service-prd.md`（路由注册模板）
- **MQ 消费者封装**：`pkg/mq/consumer.go`（复用自动重连机制）
- **Content Service 当前事件发布**：`cmd/content/service/post_service.go`（`publishEvent` 函数）

---

*本 PRD 通过 4 轮迭代式需求对话生成，质量评分 92/100，覆盖业务价值、功能需求、用户体验、技术约束、范围优先级五大维度。所有关键决策（队列隔离策略、已读模型、通知快照、保留期限）均已与产品负责人确认。*