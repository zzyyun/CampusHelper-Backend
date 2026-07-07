# 消息服务（Message Service）测试用例文档

**版本**：1.0  
**日期**：2026-07-08  
**对应 PRD**：`docs/message-service-prd.md` v1.0  
**服务范围**：消息通知服务（站内信通知中心）  

---

## 目录

1. [测试用例总览](#1-测试用例总览)
2. [功能测试（TC-F）](#2-功能测试tc-f)
3. [边界测试（TC-E）](#3-边界测试tc-e)
4. [异常测试（TC-ERR）](#4-异常测试tc-err)
5. [状态转换测试（TC-ST）](#5-状态转换测试tc-st)
6. [需求-测试用例覆盖矩阵](#6-需求-测试用例覆盖矩阵)

---

## 1. 测试用例总览

| 类别 | 编号前缀 | 数量 | 说明 |
|------|---------|------|------|
| 功能测试 | TC-F | 45 | 正常业务流程、API 功能验证 |
| 边界测试 | TC-E | 15 | 字段长度、分页、时间边界 |
| 异常测试 | TC-ERR | 18 | 故障场景、非法输入、服务异常 |
| 状态转换测试 | TC-ST | 10 | 通知生命周期状态流转 |
| **合计** | — | **88** | — |

---

## 2. 功能测试（TC-F）

### 2.1 事件消费 — 点赞通知（Story 1）

**TC-F-001**

| 字段 | 内容 |
|------|------|
| **标题** | content.liked 事件消费后创建点赞通知 |
| **需求来源** | Story 1 / FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行，MQ Consumer 已订阅 `notification.events` 队列；2. User Service、Content Service gRPC 端点可达；3. 数据库中存在用户 A（帖子作者）和用户 B（点赞者）；4. 帖子 post_id=1001、post_title="求购自行车" 存在于 Content Service |
| **测试步骤** | 1. 通过 MQ 客户端向 `notification.events` 队列发布 `content.liked` 事件：`{"type":"content.liked","post_id":"1001","school_id":"100","user_id":"2002","data":{"actor_nickname":"李四"}}`；2. 等待 Message Service 消费事件；3. 查询 `campus_message.notifications` 表 |
| **预期结果** | 1. `notifications` 表新增一条记录；2. `type` = `liked`；3. `title` = `「李四 赞了你的帖子「求购自行车」」`；4. `content` = `""`（空）；5. `user_id` = 帖子作者 ID；6. `from_user_id` = 点赞者 user_id；7. `ref_type` = `post`；8. `ref_id` = 1001；9. `is_read` = 0；10. `school_id` = 100 |

---

**TC-F-002**

| 字段 | 内容 |
|------|------|
| **标题** | content.liked 事件中 actor_nickname 正确传入通知标题 |
| **需求来源** | Story 1 / FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 同 TC-F-001，用户 B 的昵称为"小明同学" |
| **测试步骤** | 1. 发布 `content.liked` 事件，`data.actor_nickname` = "小明同学"；2. 消费完成后查询通知记录 |
| **预期结果** | 1. `title` = `「小明同学 赞了你的帖子「求购自行车」」`；2. 标题中包含完整的 nickname 和帖子标题快照 |

---

### 2.2 事件消费 — 审核结果通知（Story 2）

**TC-F-003**

| 字段 | 内容 |
|------|------|
| **标题** | content.published 事件消费后创建审核通过通知 |
| **需求来源** | Story 2 / FR-3 / R12 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. 帖子 post_id=1002、post_title="出一台iPad"，作者 user_id=3001 |
| **测试步骤** | 1. 发布 `content.published` 事件：`{"type":"content.published","post_id":"1002","school_id":"100","user_id":"3001","data":{}}`；2. 等待消费完成；3. 查询通知记录 |
| **预期结果** | 1. `type` = `published`；2. `title` = `「你的帖子「出一台iPad」审核已通过」`；3. `content` = `""`（空）；4. `user_id` = 3001；5. `ref_type` = `post`；6. `ref_id` = 1002 |

---

**TC-F-004**

| 字段 | 内容 |
|------|------|
| **标题** | content.review_result 事件消费后创建审核拒绝通知（含拒绝原因） |
| **需求来源** | Story 2 / FR-3 / R13 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. 帖子 post_id=1003、post_title="代写作业"，作者 user_id=3002；3. 拒绝原因为"内容违规，禁止发布代写信息" |
| **测试步骤** | 1. 发布 `content.review_result` 事件：`{"type":"content.review_result","post_id":"1003","school_id":"100","user_id":"3002","data":{"reason":"内容违规，禁止发布代写信息"}}`；2. 等待消费完成；3. 查询通知记录 |
| **预期结果** | 1. `type` = `review_result`；2. `title` = `「你的帖子「代写作业」审核未通过」`；3. `content` = `内容违规，禁止发布代写信息`（包含拒绝原因）；4. `user_id` = 3002；5. `ref_type` = `post`；6. `ref_id` = 1003 |

---

### 2.3 事件消费 — 下架通知（Story 3）

**TC-F-005**

| 字段 | 内容 |
|------|------|
| **标题** | content.taken_down 事件消费后创建下架通知 |
| **需求来源** | Story 3 / FR-3 / R14 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. 帖子 post_id=1004、post_title="出售违禁品"，作者 user_id=3003；3. 下架原因为"违反平台规定" |
| **测试步骤** | 1. 发布 `content.taken_down` 事件：`{"type":"content.taken_down","post_id":"1004","school_id":"100","user_id":"3003","data":{"reason":"违反平台规定"}}`；2. 等待消费完成；3. 查询通知记录 |
| **预期结果** | 1. `type` = `taken_down`；2. `title` = `「你的帖子「出售违禁品」因违规已下架」`；3. `content` = `违反平台规定`（包含下架原因）；4. `user_id` = 3003；5. `ref_type` = `post`；6. `ref_id` = 1004 |

---

### 2.4 事件消费 — 回复通知（Story 4）

**TC-F-006**

| 字段 | 内容 |
|------|------|
| **标题** | content.replied 事件消费后创建回复通知 |
| **需求来源** | Story 4 / FR-2 / FR-3 / R15 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. 帖子 post_id=1005；3. 父评论 comment_id=500，父评论作者 user_id=4001；4. 回复者 user_id=4002，昵称"王五"，回复内容"我也觉得，太贵了" |
| **测试步骤** | 1. 发布 `content.replied` 事件：`{"type":"content.replied","post_id":"1005","school_id":"100","user_id":"4002","data":{"parent_comment_id":"500","parent_comment_user_id":"4001","content_preview":"我也觉得，太贵了"}}`；2. 等待消费完成；3. 查询通知记录 |
| **预期结果** | 1. `type` = `replied`；2. `title` = `「王五 回复了你的评论：「我也觉得，太贵了」」`；3. `content` = 完整回复内容；4. `user_id` = 4001（父评论作者，而非回复者）；5. `from_user_id` = 4002；6. `ref_type` = `post`；7. `ref_id` = 1005 |

---

**TC-F-007**

| 字段 | 内容 |
|------|------|
| **标题** | content.replied 事件的 content_preview 截取前 50 个字符 |
| **需求来源** | Story 4 / FR-2 / R15 |
| **优先级** | 中 |
| **前置条件** | 1. Message Service 正常运行；2. 回复内容为 100 个字符的长文本 |
| **测试步骤** | 1. 构造 `content.replied` 事件，`content_preview` 设为 100 个字符的字符串；2. 等待消费完成；3. 查询通知记录的 `title` 字段 |
| **预期结果** | 1. `title` 中的预览部分仅包含前 50 个字符；2. 第 50 个字符后的内容被截断 |

---

**TC-F-008**

| 字段 | 内容 |
|------|------|
| **标题** | content.replied 事件的 content_preview 不足 50 字符时原样保留 |
| **需求来源** | Story 4 / FR-2 |
| **优先级** | 中 |
| **前置条件** | 1. Message Service 正常运行；2. 回复内容为 20 个字符的短文本 |
| **测试步骤** | 1. 构造 `content.replied` 事件，`content_preview` 设为 20 个字符的字符串；2. 等待消费完成；3. 查询通知记录 |
| **预期结果** | 1. `title` 中的预览部分完整保留 20 个字符；2. 不做任何截断 |

---

### 2.5 事件消费 — 跳过规则

**TC-F-009**

| 字段 | 内容 |
|------|------|
| **标题** | 自己赞自己的帖子不生成通知 |
| **需求来源** | Story 1 / FR-3 / R19 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. 用户 A（user_id=3001）是帖子 post_id=1001 的作者；3. 用户 A 对自己的帖子点赞 |
| **测试步骤** | 1. 发布 `content.liked` 事件，`user_id` = 3001，`post_id` = 1001（同一用户）；2. 等待消费完成；3. 查询 `notifications` 表，条件：`user_id=3001 AND type='liked' AND ref_id=1001` |
| **预期结果** | 1. 未生成新的通知记录；2. `notifications` 表中无该条通知 |

---

### 2.6 双队列投递（FR-1）

**TC-F-010**

| 字段 | 内容 |
|------|------|
| **标题** | content.liked 事件仅投递 notification.events 队列 |
| **需求来源** | FR-1 / R16 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 正常运行；2. 两个队列 `content.events` 和 `notification.events` 均存在 |
| **测试步骤** | 1. Content Service 处理点赞事件；2. 监控 `content.events` 队列消息数；3. 监控 `notification.events` 队列消息数 |
| **预期结果** | 1. `content.events` 队列无 `content.liked` 消息；2. `notification.events` 队列收到 1 条 `content.liked` 消息 |

---

**TC-F-011**

| 字段 | 内容 |
|------|------|
| **标题** | content.published 事件同时投递两个队列 |
| **需求来源** | FR-1 / R16 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 正常运行；2. 两个队列均存在 |
| **测试步骤** | 1. Content Service 处理帖子审核通过事件；2. 监控两个队列 |
| **预期结果** | 1. `content.events` 队列收到 `content.published` 消息（ES 索引消费）；2. `notification.events` 队列收到 `content.published` 消息（通知消费） |

---

**TC-F-012**

| 字段 | 内容 |
|------|------|
| **标题** | content.taken_down 事件同时投递两个队列 |
| **需求来源** | FR-1 / R16 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 正常运行；2. 两个队列均存在 |
| **测试步骤** | 1. Content Service 处理帖子下架事件；2. 监控两个队列 |
| **预期结果** | 1. `content.events` 队列收到 `content.taken_down` 消息（ES 删除消费）；2. `notification.events` 队列收到 `content.taken_down` 消息（通知消费） |

---

**TC-F-013**

| 字段 | 内容 |
|------|------|
| **标题** | content.review_result 事件仅投递 notification.events 队列 |
| **需求来源** | FR-1 / R16 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 正常运行 |
| **测试步骤** | 1. Content Service 处理帖子审核拒绝事件；2. 监控两个队列 |
| **预期结果** | 1. `content.events` 队列无 `content.review_result` 消息；2. `notification.events` 队列收到 1 条 `content.review_result` 消息 |

---

### 2.7 content.replied 事件发布（FR-2）

**TC-F-014**

| 字段 | 内容 |
|------|------|
| **标题** | parent_id != 0 时发布 content.replied 事件 |
| **需求来源** | FR-2 / R17 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 正常运行；2. `notification.events` Publisher 已初始化 |
| **测试步骤** | 1. 调用 `CreateComment` 接口创建一条 `parent_id = 500` 的评论；2. 检查 `notification.events` 队列 |
| **预期结果** | 1. 评论创建成功；2. `notification.events` 队列收到 `content.replied` 事件；3. 事件包含 `parent_comment_id`、`parent_comment_user_id`、`content_preview` |

---

**TC-F-015**

| 字段 | 内容 |
|------|------|
| **标题** | parent_id = 0（一级评论）时不发布 content.replied 事件 |
| **需求来源** | FR-2 / R17 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 正常运行 |
| **测试步骤** | 1. 调用 `CreateComment` 接口创建一条 `parent_id = 0` 的一级评论；2. 检查 `notification.events` 队列 |
| **预期结果** | 1. 评论创建成功；2. `notification.events` 队列未收到 `content.replied` 事件 |

---

**TC-F-016**

| 字段 | 内容 |
|------|------|
| **标题** | content.replied 事件在事务提交成功后才发布 |
| **需求来源** | FR-2 / R17 / 风险评估 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 正常运行；2. 模拟事务提交失败场景（如数据库连接异常） |
| **测试步骤** | 1. 构造 `CreateComment` 调用，使事务提交失败；2. 检查 `notification.events` 队列 |
| **预期结果** | 1. 评论创建失败（事务回滚）；2. `notification.events` 队列未收到 `content.replied` 事件 |

---

### 2.8 EventContentLiked 常量定义

**TC-F-017**

| 字段 | 内容 |
|------|------|
| **标题** | EventContentLiked 常量值正确且被引用 |
| **需求来源** | FR-1 / 附录 |
| **优先级** | 中 |
| **前置条件** | 代码已编译通过 |
| **测试步骤** | 1. 检查 `pkg/mq/publisher.go` 中 `EventContentLiked` 常量定义；2. 检查 Content Service 代码中 `content.liked` 字面量已替换为该常量 |
| **预期结果** | 1. `EventContentLiked = "content.liked"` 存在于常量定义中；2. 所有 `content.liked` 字面量引用均使用该常量 |

---

**TC-F-018**

| 字段 | 内容 |
|------|------|
| **标题** | EventContentReplied 常量值正确 |
| **需求来源** | FR-2 / 附录 |
| **优先级** | 中 |
| **前置条件** | 代码已编译通过 |
| **测试步骤** | 1. 检查 `pkg/mq/publisher.go` 中 `EventContentReplied` 常量定义 |
| **预期结果** | 1. `EventContentReplied = "content.replied"` 存在于常量定义中 |

---

### 2.9 通知列表 API（Story 5）

**TC-F-019**

| 字段 | 内容 |
|------|------|
| **标题** | GET /api/v1/notifications 分页返回当前用户通知列表 |
| **需求来源** | Story 5 / FR-5 / R18 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录，JWT token 有效；2. 用户 A 有 10 条通知 |
| **测试步骤** | 1. 携带 JWT token 请求 `GET /api/v1/notifications?limit=5`；2. 检查响应体 |
| **预期结果** | 1. HTTP 200；2. `notifications` 数组长度 ≤ 5；3. 响应包含 `unread_count`、`has_more`、`next_cursor` 字段；4. 每条通知包含 `id`、`type`、`title`、`content`、`is_read`、`created_at`、`ref_type`、`ref_id` |

---

**TC-F-020**

| 字段 | 内容 |
|------|------|
| **标题** | GET /api/v1/notifications 按 type 筛选通知 |
| **需求来源** | Story 5 / FR-5 / R18 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 A 有 liked、review_result、replied 类型通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications?type=liked`；2. 检查响应体 |
| **预期结果** | 1. HTTP 200；2. 返回的所有通知 `type` 均为 `liked`；3. 不包含其他类型通知 |

---

**TC-F-021**

| 字段 | 内容 |
|------|------|
| **标题** | GET /api/v1/notifications 通知按 created_at DESC 排序 |
| **需求来源** | Story 5 / FR-5 / R18 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 A 有 3 条不同时间创建的通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications`；2. 检查返回通知的 `created_at` 顺序 |
| **预期结果** | 1. HTTP 200；2. 第一条通知的 `created_at` 最新（最晚）；3. 最后一条通知的 `created_at` 最早 |

---

**TC-F-022**

| 字段 | 内容 |
|------|------|
| **标题** | GET /api/v1/notifications 响应包含正确的 unread_count |
| **需求来源** | Story 5 / FR-5 / R18 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 A 有 5 条未读通知、3 条已读通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications`；2. 检查响应体中的 `unread_count` |
| **预期结果** | 1. HTTP 200；2. `unread_count` = 5 |

---

**TC-F-023**

| 字段 | 内容 |
|------|------|
| **标题** | GET /api/v1/notifications 无通知时返回空列表 |
| **需求来源** | Story 5 / FR-5 / R18 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 A 无任何通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications`；2. 检查响应体 |
| **预期结果** | 1. HTTP 200；2. `notifications` 为空数组 `[]`；3. `unread_count` = 0；4. `has_more` = false |

---

### 2.10 未读数 API（Story 6）

**TC-F-024**

| 字段 | 内容 |
|------|------|
| **标题** | GET /api/v1/notifications/unread-count 返回正确未读数 |
| **需求来源** | Story 6 / FR-5 / R19 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 A 有 3 条未读通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications/unread-count`；2. 检查响应体 |
| **预期结果** | 1. HTTP 200；2. 响应为 `{"count": 3}` |

---

**TC-F-025**

| 字段 | 内容 |
|------|------|
| **标题** | GET /api/v1/notifications/unread-count 无未读时返回 0 |
| **需求来源** | Story 6 / FR-5 / R19 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 A 所有通知均已读或无通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications/unread-count`；2. 检查响应体 |
| **预期结果** | 1. HTTP 200；2. 响应为 `{"count": 0}` |

---

**TC-F-026**

| 字段 | 内容 |
|------|------|
| **标题** | unread-count 为实时 COUNT 查询（不缓存） |
| **需求来源** | Story 6 / FR-5 / R19 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 A 当前有 2 条未读通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications/unread-count`，确认返回 `count=2`；2. 通过 MQ 发布一条新的点赞事件为用户 A 创建通知；3. 再次请求 `GET /api/v1/notifications/unread-count` |
| **预期结果** | 1. 第二次请求返回 `count=3`（实时反映新增未读数） |

---

### 2.11 标记已读 API（Story 7）

**TC-F-027**

| 字段 | 内容 |
|------|------|
| **标题** | PUT /api/v1/notifications/:id/read 标记单条通知为已读 |
| **需求来源** | Story 7 / FR-5 / R20 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 通知 id=10001 属于用户 A，`is_read`=0 |
| **测试步骤** | 1. 请求 `PUT /api/v1/notifications/10001/read`；2. 查询数据库该通知的 `is_read` 字段 |
| **预期结果** | 1. HTTP 200；2. 该通知 `is_read` = 1；3. `unread_count` 减少 1 |

---

**TC-F-028**

| 字段 | 内容 |
|------|------|
| **标题** | PUT /api/v1/notifications/:id/read 重复标记为幂等操作 |
| **需求来源** | Story 7 / FR-5 / R20 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 通知 id=10001 已标记为已读（`is_read`=1） |
| **测试步骤** | 1. 请求 `PUT /api/v1/notifications/10001/read`；2. 检查响应和数据库状态 |
| **预期结果** | 1. HTTP 200（不报错）；2. `is_read` 保持为 1；3. `unread_count` 不变 |

---

**TC-F-029**

| 字段 | 内容 |
|------|------|
| **标题** | PUT /api/v1/notifications/read-all 批量标记当前用户所有通知为已读 |
| **需求来源** | Story 7 / FR-5 / R20 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 A 有 5 条未读通知 |
| **测试步骤** | 1. 请求 `PUT /api/v1/notifications/read-all`；2. 请求 `GET /api/v1/notifications/unread-count` |
| **预期结果** | 1. HTTP 200；2. 未读数返回 `{"count": 0}`；3. 数据库中用户 A 所有通知 `is_read` = 1 |

---

### 2.12 删除通知 API（Story 8）

**TC-F-030**

| 字段 | 内容 |
|------|------|
| **标题** | DELETE /api/v1/notifications/:id 软删除单条通知 |
| **需求来源** | Story 8 / FR-5 / R21 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 通知 id=10001 属于用户 A |
| **测试步骤** | 1. 请求 `DELETE /api/v1/notifications/10001`；2. 查询数据库该通知记录 |
| **预期结果** | 1. HTTP 200；2. 该通知 `deleted_at` 不为 NULL（软删除）；3. `GET /notifications` 不再返回该通知 |

---

**TC-F-031**

| 字段 | 内容 |
|------|------|
| **标题** | DELETE /api/v1/notifications/:id 仅能删除自己的通知 |
| **需求来源** | Story 8 / FR-5 / 安全校验 / R22 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 通知 id=10002 属于用户 B |
| **测试步骤** | 1. 用户 A 请求 `DELETE /api/v1/notifications/10002` |
| **预期结果** | 1. HTTP 403 或 404；2. 通知未被删除 |

---

### 2.13 鉴权与安全

**TC-F-032**

| 字段 | 内容 |
|------|------|
| **标题** | 所有通知 API 强制 JWT 鉴权 |
| **需求来源** | FR-5 / 安全 / R22 |
| **优先级** | 高 |
| **前置条件** | 1. 无有效 JWT token |
| **测试步骤** | 1. 不携带 Authorization header 请求 `GET /api/v1/notifications`；2. 不携带 Authorization header 请求 `GET /api/v1/notifications/unread-count`；3. 不携带 Authorization header 请求 `PUT /api/v1/notifications/1/read`；4. 不携带 Authorization header 请求 `PUT /api/v1/notifications/read-all`；5. 不携带 Authorization header 请求 `DELETE /api/v1/notifications/1` |
| **预期结果** | 1. 所有 5 个请求均返回 HTTP 401 |

---

**TC-F-033**

| 字段 | 内容 |
|------|------|
| **标题** | 用户只能查看自己的通知列表 |
| **需求来源** | FR-5 / 安全 / R22 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 用户 B 有通知；3. 用户 A 无通知 |
| **测试步骤** | 1. 用户 A 请求 `GET /api/v1/notifications` |
| **预期结果** | 1. HTTP 200；2. 返回空列表（不包含用户 B 的通知） |

---

### 2.14 school_id 隔离

**TC-F-034**

| 字段 | 内容 |
|------|------|
| **标题** | school_id 隔离 — 不同学校的通知互不可见 |
| **需求来源** | FR-4 / FR-5 / R22 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A（school_id=100）和用户 C（school_id=200）有相同 user_id 的不同用户；2. 用户 A 和 C 各自有通知 |
| **测试步骤** | 1. 用户 A（school_id=100）请求 `GET /api/v1/notifications`；2. 用户 C（school_id=200）请求 `GET /api/v1/notifications` |
| **预期结果** | 1. 用户 A 仅看到 school_id=100 的通知；2. 用户 C 仅看到 school_id=200 的通知；3. 两者的 `unread_count` 仅统计各自学校的通知 |

---

### 2.15 通知数据模型验证

**TC-F-035**

| 字段 | 内容 |
|------|------|
| **标题** | 通知记录 Snowflake ID 生成正确 |
| **需求来源** | FR-4 / R23 |
| **优先级** | 中 |
| **前置条件** | 1. Message Service 正常运行 |
| **测试步骤** | 1. 消费一个事件创建通知；2. 查询数据库记录的 `id` 字段 |
| **预期结果** | 1. `id` 为非零的 64 位整数；2. `id` 符合雪花算法格式（时间戳+机器ID+序列号） |

---

**TC-F-036**

| 字段 | 内容 |
|------|------|
| **标题** | 通知数据库 `campus_message` 独立于其他服务 |
| **需求来源** | FR-4 / 架构约束 / R23 |
| **优先级** | 高 |
| **前置条件** | 1. 服务已部署 |
| **测试步骤** | 1. 检查 Message Service 配置文件中 `messageDatabase` 字段；2. 检查 Message Service 连接的 MySQL 数据库名 |
| **预期结果** | 1. 配置 `messageDatabase: "campus_message"`；2. Message Service 仅连接 `campus_message` 数据库 |

---

**TC-F-037**

| 字段 | 内容 |
|------|------|
| **标题** | 通知表索引 `idx_user_read` 存在 |
| **需求来源** | FR-4 / R23 |
| **优先级** | 中 |
| **前置条件** | 1. `campus_message` 数据库已初始化 |
| **测试步骤** | 1. 执行 `SHOW INDEX FROM notifications WHERE Key_name = 'idx_user_read'` |
| **预期结果** | 1. 索引存在；2. 包含列 `user_id`、`is_read`、`created_at` |

---

### 2.16 跨服务回调

**TC-F-038**

| 字段 | 内容 |
|------|------|
| **标题** | 事件消费时正确回调 User Service 获取 actor_nickname |
| **需求来源** | FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. User Service gRPC 端点可达；2. 点赞者 user_id=2002，昵称"小红" |
| **测试步骤** | 1. 发布 `content.liked` 事件，user_id=2002；2. 监控 User Service gRPC 调用日志；3. 检查通知记录 |
| **预期结果** | 1. Message Service 调用了 User Service 的 `GetCurrentUser` 接口；2. 通知 `title` 中使用了正确的昵称"小红" |

---

**TC-F-039**

| 字段 | 内容 |
|------|------|
| **标题** | 事件消费时正确回调 Content Service 获取 post_title |
| **需求来源** | FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service gRPC 端点可达；2. 帖子 post_id=1001，标题"求购自行车" |
| **测试步骤** | 1. 发布 `content.liked` 事件，post_id=1001；2. 监控 Content Service gRPC 调用日志；3. 检查通知记录 |
| **预期结果** | 1. Message Service 调用了 Content Service 的帖子查询接口；2. 通知 `title` 中使用了正确的帖子标题"求购自行车" |

---

### 2.17 通知标题快照策略

**TC-F-040**

| 字段 | 内容 |
|------|------|
| **标题** | 通知标题快照 — 帖子删除后通知标题不变 |
| **需求来源** | FR-4 / R24 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 A 已收到点赞通知（title 包含帖子标题快照）；2. 帖子随后被删除 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications` 查看该通知；2. 检查通知 `title` 字段 |
| **预期结果** | 1. 通知 `title` 仍完整包含帖子标题快照；2. 标题内容不受帖子删除影响 |

---

**TC-F-041**

| 字段 | 内容 |
|------|------|
| **标题** | 通知标题快照 — 用户修改昵称后通知标题不变 |
| **需求来源** | FR-4 / R24 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 B（昵称"李四"）赞了用户 A 的帖子；2. 用户 A 已收到点赞通知；3. 用户 B 将昵称改为"小明" |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications` 查看该通知；2. 检查通知 `title` 字段 |
| **预期结果** | 1. 通知 `title` 仍为`「李四 赞了你的帖子...」`（使用旧昵称快照） |

---

### 2.18 消息 ACK

**TC-F-042**

| 字段 | 内容 |
|------|------|
| **标题** | 事件消费成功后正确 Ack 消息 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. MQ 队列有待消费消息 |
| **测试步骤** | 1. 发布一条 `content.liked` 事件；2. 消费成功后检查 RabbitMQ Management UI 或队列状态 |
| **预期结果** | 1. 消息从队列中移除（已 Ack）；2. 通知记录正确创建 |

---

### 2.19 多学校多用户并发场景

**TC-F-043**

| 字段 | 内容 |
|------|------|
| **标题** | 同一事件仅通知帖子作者，不通知其他用户 |
| **需求来源** | Story 1 / FR-3 |
| **优先级** | 高 |
| **前置条件** | 1. 帖子 post_id=1001 的作者为用户 A；2. 用户 B 和用户 C 都在该校 |
| **测试步骤** | 1. 用户 B 点赞帖子 1001；2. 等待通知创建；3. 查询 `notifications` 表中 post_id=1001 相关的通知 |
| **预期结果** | 1. 仅生成 1 条通知，`user_id` = 用户 A（帖子作者）；2. 不为用户 B 或用户 C 生成通知 |

---

**TC-F-044**

| 字段 | 内容 |
|------|------|
| **标题** | 同时消费多条不同类型事件，通知正确分类 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 中 |
| **前置条件** | 1. Message Service 正常运行；2. 用户 A 有多种事件待处理 |
| **测试步骤** | 1. 连续发布 `content.liked`、`content.review_result`、`content.replied` 三条事件；2. 等待全部消费完成；3. 查询用户 A 的通知列表 |
| **预期结果** | 1. 生成 3 条通知；2. 各通知 `type` 分别为 `liked`、`review_result`、`replied`；3. 各通知标题格式正确 |

---

### 2.20 系统配置与服务注册

**TC-F-045**

| 字段 | 内容 |
|------|------|
| **标题** | Message Service 通过 etcd 注册并被 Gateway 发现 |
| **需求来源** | 技术约束 / 集成 |
| **优先级** | 高 |
| **前置条件** | 1. etcd 服务运行正常；2. Message Service 启动 |
| **测试步骤** | 1. 检查 etcd 中 Message Service 注册信息；2. Gateway 通过 gRPC 调用 Message Service |
| **预期结果** | 1. etcd 中存在 `message-service` 注册节点；2. Gateway 能通过服务发现调用 Message Service 的 gRPC 接口 |

---

## 3. 边界测试（TC-E）

**TC-E-001**

| 字段 | 内容 |
|------|------|
| **标题** | title 字段长度达到最大值 255 字符 |
| **需求来源** | FR-4 / R23 |
| **优先级** | 中 |
| **前置条件** | 1. 构造超长帖子标题（200+字符）和超长昵称（50+字符），组合后 title > 255 字符 |
| **测试步骤** | 1. 发布事件，使格式化后的 title 超过 255 字符；2. 观察消费结果 |
| **预期结果** | 1. 系统对 title 做截断处理（截断至 255 字符）或拒绝写入并记录错误日志；2. 不导致服务崩溃 |

---

**TC-E-002**

| 字段 | 内容 |
|------|------|
| **标题** | content 字段长度达到最大值 500 字符 |
| **需求来源** | FR-4 / R23 |
| **优先级** | 中 |
| **前置条件** | 1. 构造审核拒绝原因或下架原因，长度为 500 字符 |
| **测试步骤** | 1. 发布 `content.review_result` 事件，reason 为 500 字符；2. 查询通知记录 |
| **预期结果** | 1. `content` 字段完整保存 500 字符；2. 不截断、不报错 |

---

**TC-E-003**

| 字段 | 内容 |
|------|------|
| **标题** | content 字段超过 500 字符时截断 |
| **需求来源** | FR-4 / R23 |
| **优先级** | 中 |
| **前置条件** | 1. 构造审核拒绝原因，长度为 600 字符 |
| **测试步骤** | 1. 发布 `content.review_result` 事件，reason 为 600 字符；2. 查询通知记录 |
| **预期结果** | 1. `content` 字段截断为 500 字符；2. 服务不崩溃 |

---

**TC-E-004**

| 字段 | 内容 |
|------|------|
| **标题** | content_preview 正好 50 个字符时不做截断 |
| **需求来源** | FR-2 / Story 4 |
| **优先级** | 中 |
| **前置条件** | 1. 回复内容正好 50 个字符 |
| **测试步骤** | 1. 发布 `content.replied` 事件，`content_preview` 为 50 字符；2. 检查通知 title |
| **预期结果** | 1. title 中的预览部分完整包含 50 个字符 |

---

**TC-E-005**

| 字段 | 内容 |
|------|------|
| **标题** | content_preview 为 51 个字符时截取前 50 个 |
| **需求来源** | FR-2 / Story 4 |
| **优先级** | 中 |
| **前置条件** | 1. 回复内容为 51 个字符 |
| **测试步骤** | 1. 发布 `content.replied` 事件，`content_preview` 为 51 字符；2. 检查通知 title |
| **预期结果** | 1. title 中的预览部分仅包含前 50 个字符 |

---

**TC-E-006**

| 字段 | 内容 |
|------|------|
| **标题** | 通知列表分页 — limit=1 返回恰好一条记录 |
| **需求来源** | Story 5 / FR-5 / R18 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 A 有 10 条通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications?limit=1`；2. 检查返回数量 |
| **预期结果** | 1. `notifications` 数组长度 = 1；2. `has_more` = true |

---

**TC-E-007**

| 字段 | 内容 |
|------|------|
| **标题** | 通知列表分页 — limit 超过总记录数 |
| **需求来源** | Story 5 / FR-5 |
| **优先级** | 低 |
| **前置条件** | 1. 用户 A 有 3 条通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications?limit=100` |
| **预期结果** | 1. `notifications` 数组长度 = 3；2. `has_more` = false |

---

**TC-E-008**

| 字段 | 内容 |
|------|------|
| **标题** | 通知列表分页 — 使用 cursor 翻页 |
| **需求来源** | Story 5 / FR-5 / R18 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 A 有 20 条通知；2. 第一页返回 `next_cursor` |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications?limit=10`；2. 取返回的 `next_cursor`；3. 请求 `GET /api/v1/notifications?limit=10&cursor={next_cursor}` |
| **预期结果** | 1. 第一页返回 10 条，`has_more` = true，`next_cursor` 非空；2. 第二页返回 10 条，`has_more` = false |

---

**TC-E-009**

| 字段 | 内容 |
|------|------|
| **标题** | 30 天边界 — 正好 30 天的通知仍存在 |
| **需求来源** | Story 8 / 定时清理 / R25 |
| **优先级** | 中 |
| **前置条件** | 1. 通知记录 `created_at` = 当前时间 - 30 天（恰好 30 天） |
| **测试步骤** | 1. 查询 `notifications` 表中 `created_at` 为 30 天前的记录 |
| **预期结果** | 1. 该记录仍存在（尚未被清理） |

---

**TC-E-010**

| 字段 | 内容 |
|------|------|
| **标题** | 30 天边界 — 超过 30 天的软删除通知被物理清理 |
| **需求来源** | Story 8 / 定时清理 / R25 |
| **优先级** | 中 |
| **前置条件** | 1. 通知记录 `created_at` = 当前时间 - 31 天；2. `deleted_at` 不为 NULL（已软删除） |
| **测试步骤** | 1. 触发定时清理任务；2. 查询 `notifications` 表 |
| **预期结果** | 1. 该记录已被物理删除（不存在于数据库中） |

---

**TC-E-011**

| 字段 | 内容 |
|------|------|
| **标题** | type 字段边界 — 5 种合法类型值 |
| **需求来源** | FR-3 / FR-4 |
| **优先级** | 中 |
| **前置条件** | 1. 准备 5 种不同类型的事件 |
| **测试步骤** | 1. 分别消费 `liked`、`published`、`review_result`、`taken_down`、`replied` 事件；2. 查询通知记录的 `type` 字段 |
| **预期结果** | 1. 5 条通知的 `type` 字段分别对应上述 5 种值 |

---

**TC-E-012**

| 字段 | 内容 |
|------|------|
| **标题** | ref_type 字段 — post 和 comment 两种关联类型 |
| **需求来源** | FR-4 / R23 |
| **优先级** | 低 |
| **前置条件** | 1. 消费点赞事件（关联帖子）和回复事件（关联评论） |
| **测试步骤** | 1. 分别消费 `content.liked` 和 `content.replied` 事件；2. 查询通知记录的 `ref_type` 字段 |
| **预期结果** | 1. 点赞通知 `ref_type` = `post`；2. 回复通知 `ref_type` = `post`（关联帖子详情页跳转） |

---

**TC-E-013**

| 字段 | 内容 |
|------|------|
| **标题** | from_user_id 为 0 — 系统通知场景 |
| **需求来源** | FR-4 / R23 |
| **优先级** | 低 |
| **前置条件** | 1. 审核结果事件（系统触发，无具体操作者） |
| **测试步骤** | 1. 消费 `content.review_result` 事件；2. 查询 `from_user_id` 字段 |
| **预期结果** | 1. `from_user_id` = 0（表示系统通知） |

---

**TC-E-014**

| 字段 | 内容 |
|------|------|
| **标题** | 通知列表默认 limit 参数 |
| **需求来源** | Story 5 / FR-5 / R18 |
| **优先级** | 低 |
| **前置条件** | 1. 用户 A 有 50 条通知 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications`（不传 limit 参数）；2. 检查返回数量 |
| **预期结果** | 1. 使用默认分页大小（如 20 条）；2. `has_more` = true |

---

**TC-E-015**

| 字段 | 内容 |
|------|------|
| **标题** | 帖子标题包含特殊字符时通知标题正确显示 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 低 |
| **前置条件** | 1. 帖子标题包含中文引号、emoji 等特殊字符 |
| **测试步骤** | 1. 发布 `content.liked` 事件，帖子标题包含特殊字符；2. 查询通知 title |
| **预期结果** | 1. 通知 title 正确包含特殊字符，无乱码 |

---

## 4. 异常测试（TC-ERR）

**TC-ERR-001**

| 字段 | 内容 |
|------|------|
| **标题** | MQ 消费异常 — 消息 JSON 格式错误 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行 |
| **测试步骤** | 1. 向 `notification.events` 队列发送非法 JSON 字符串（如 `not-json`）；2. 观察消费者行为和日志 |
| **预期结果** | 1. 消费者不崩溃；2. 该消息被 NACK 或记录错误日志后丢弃；3. 其他正常消息继续消费 |

---

**TC-ERR-002**

| 字段 | 内容 |
|------|------|
| **标题** | MQ 消费异常 — 消息缺少必要字段 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行 |
| **测试步骤** | 1. 发送缺少 `type` 字段的消息：`{"post_id":"1001","school_id":"100"}`；2. 发送缺少 `post_id` 字段的消息；3. 观察消费者行为 |
| **预期结果** | 1. 消费者不崩溃；2. 缺少必要字段的消息被 NACK 或记录错误日志；3. 正常消息继续消费 |

---

**TC-ERR-003**

| 字段 | 内容 |
|------|------|
| **标题** | MQ 消费异常 — 未知事件类型 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 中 |
| **前置条件** | 1. Message Service 正常运行 |
| **测试步骤** | 1. 发送 `type` = `"unknown.event"` 的消息；2. 观察消费者行为 |
| **预期结果** | 1. 消费者不崩溃；2. 记录警告日志，消息被 NACK 或丢弃 |

---

**TC-ERR-004**

| 字段 | 内容 |
|------|------|
| **标题** | User Service 回调超时（3s） |
| **需求来源** | FR-3 / 风险评估 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. User Service 响应延迟 > 3 秒或不可达 |
| **测试步骤** | 1. 发布 `content.liked` 事件；2. 观察消费者行为和日志 |
| **预期结果** | 1. 消费者在超时后降级处理（记录日志，使用默认值如"未知用户"）；2. 通知仍被创建（title 中 nickname 使用降级值）；3. 不阻塞后续消息消费 |

---

**TC-ERR-005**

| 字段 | 内容 |
|------|------|
| **标题** | Content Service 回调超时（3s） |
| **需求来源** | FR-3 / 风险评估 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 响应延迟 > 3 秒或不可达 |
| **测试步骤** | 1. 发布 `content.liked` 事件；2. 观察消费者行为和日志 |
| **预期结果** | 1. 消费者在超时后降级处理（记录日志，使用默认值如"未知帖子"）；2. 通知仍被创建（title 中 post_title 使用降级值）；3. 不阻塞后续消息消费 |

---

**TC-ERR-006**

| 字段 | 内容 |
|------|------|
| **标题** | User Service 回调失败 — 用户不存在 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 中 |
| **前置条件** | 1. User Service 可达但 user_id 对应的用户已被删除 |
| **测试步骤** | 1. 发布 `content.liked` 事件，user_id 指向不存在的用户；2. 观察消费者行为 |
| **预期结果** | 1. 消费者降级处理；2. 通知 title 中 nickname 使用默认值；3. 服务不崩溃 |

---

**TC-ERR-007**

| 字段 | 内容 |
|------|------|
| **标题** | Content Service 回调失败 — 帖子不存在 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 中 |
| **前置条件** | 1. Content Service 可达但 post_id 对应的帖子已被删除 |
| **测试步骤** | 1. 发布 `content.liked` 事件，post_id 指向不存在的帖子；2. 观察消费者行为 |
| **预期结果** | 1. 消费者降级处理；2. 通知 title 中 post_title 使用默认值；3. 服务不崩溃 |

---

**TC-ERR-008**

| 字段 | 内容 |
|------|------|
| **标题** | MySQL 写入失败时消息不被 Ack |
| **需求来源** | FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. 模拟 MySQL 写入异常（如磁盘满） |
| **测试步骤** | 1. 发布 `content.liked` 事件；2. 模拟数据库写入失败；3. 检查 MQ 消息状态 |
| **预期结果** | 1. 消息未被 Ack；2. 消息重新入队或进入死信队列；3. 消费者不崩溃 |

---

**TC-ERR-009**

| 字段 | 内容 |
|------|------|
| **标题** | RabbitMQ 连接断开后自动重连 |
| **需求来源** | FR-3 / 集成 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行并连接 RabbitMQ |
| **测试步骤** | 1. 重启 RabbitMQ 服务（模拟连接断开）；2. 恢复 RabbitMQ 服务；3. 发布新消息；4. 检查是否被消费 |
| **预期结果** | 1. Message Service 自动重连 RabbitMQ；2. 新消息被正常消费；3. 不需要手动重启服务 |

---

**TC-ERR-010**

| 字段 | 内容 |
|------|------|
| **标题** | 双队列投递 — notification.events 投递失败不影响 content.events |
| **需求来源** | FR-1 / 风险评估 / R16 |
| **优先级** | 中 |
| **前置条件** | 1. Content Service 正常运行；2. 模拟 notification.events Publisher 异常 |
| **测试步骤** | 1. 使 notification.events Publisher 不可用；2. 触发 content.published 事件；3. 检查 content.events 队列 |
| **预期结果** | 1. content.events 队列正常收到消息（ES 同步不受影响）；2. notification.events 投递失败记录错误日志 |

---

**TC-ERR-011**

| 字段 | 内容 |
|------|------|
| **标题** | 双队列投递 — content.events 投递失败不影响 notification.events |
| **需求来源** | FR-1 / 风险评估 / R16 |
| **优先级** | 中 |
| **前置条件** | 1. Content Service 正常运行；2. 模拟 content.events Publisher 异常 |
| **测试步骤** | 1. 使 content.events Publisher 不可用；2. 触发 content.published 事件；3. 检查 notification.events 队列 |
| **预期结果** | 1. notification.events 队列正常收到消息（通知消费不受影响）；2. content.events 投递失败记录错误日志 |

---

**TC-ERR-012**

| 字段 | 内容 |
|------|------|
| **标题** | JWT token 无效（签名错误） |
| **需求来源** | FR-5 / 安全 / R22 |
| **优先级** | 高 |
| **前置条件** | 1. 构造一个签名错误的 JWT token |
| **测试步骤** | 1. 使用无效 token 请求 `GET /api/v1/notifications` |
| **预期结果** | 1. HTTP 401 Unauthorized |

---

**TC-ERR-013**

| 字段 | 内容 |
|------|------|
| **标题** | JWT token 过期 |
| **需求来源** | FR-5 / 安全 / R22 |
| **优先级** | 高 |
| **前置条件** | 1. 构造一个已过期的 JWT token |
| **测试步骤** | 1. 使用过期 token 请求 `GET /api/v1/notifications` |
| **预期结果** | 1. HTTP 401 Unauthorized |

---

**TC-ERR-014**

| 字段 | 内容 |
|------|------|
| **标题** | 标记不存在的通知为已读 |
| **需求来源** | Story 7 / FR-5 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 A 已登录；2. 通知 id=999999 不存在 |
| **测试步骤** | 1. 请求 `PUT /api/v1/notifications/999999/read` |
| **预期结果** | 1. HTTP 404 Not Found；2. 响应包含明确错误信息 |

---

**TC-ERR-015**

| 字段 | 内容 |
|------|------|
| **标题** | 删除不存在的通知 |
| **需求来源** | Story 8 / FR-5 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 A 已登录；2. 通知 id=999999 不存在 |
| **测试步骤** | 1. 请求 `DELETE /api/v1/notifications/999999` |
| **预期结果** | 1. HTTP 404 Not Found |

---

**TC-ERR-016**

| 字段 | 内容 |
|------|------|
| **标题** | 尝试删除他人的通知（权限校验） |
| **需求来源** | Story 8 / FR-5 / 安全 / R22 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 已登录；2. 通知 id=10002 属于用户 B |
| **测试步骤** | 1. 用户 A 请求 `DELETE /api/v1/notifications/10002` |
| **预期结果** | 1. HTTP 403 Forbidden 或 404 Not Found；2. 通知未被删除 |

---

**TC-ERR-017**

| 字段 | 内容 |
|------|------|
| **标题** | GET /notifications 传入非法分页参数 |
| **需求来源** | Story 5 / FR-5 |
| **优先级** | 低 |
| **前置条件** | 1. 用户 A 已登录 |
| **测试步骤** | 1. 请求 `GET /api/v1/notifications?limit=abc`；2. 请求 `GET /api/v1/notifications?limit=-1` |
| **预期结果** | 1. 使用默认分页大小或返回 400 Bad Request；2. 不崩溃 |

---

**TC-ERR-018**

| 字段 | 内容 |
|------|------|
| **标题** | 跨服务回调 context 被取消 |
| **需求来源** | FR-3 / 风险评估 / R11 |
| **优先级** | 中 |
| **前置条件** | 1. Message Service 正常运行；2. 模拟 gRPC 调用 context 被提前取消 |
| **测试步骤** | 1. 发布事件触发消费；2. 在回调过程中模拟 context cancelled |
| **预期结果** | 1. 消费者优雅处理 context 取消错误；2. 消息未被 Ack（允许重试）；3. 不影响其他消息消费 |

---

## 5. 状态转换测试（TC-ST）

**TC-ST-001**

| 字段 | 内容 |
|------|------|
| **标题** | 通知状态转换：未读 → 已读（单条标记） |
| **需求来源** | Story 7 / FR-5 / R20 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 有 1 条未读通知（`is_read`=0）；2. `unread_count` = 1 |
| **测试步骤** | 1. 调用 `PUT /api/v1/notifications/:id/read`；2. 查询 `is_read` 字段；3. 查询 `unread-count` API |
| **预期结果** | 1. `is_read` 从 0 变为 1；2. `unread_count` 从 1 变为 0 |

---

**TC-ST-002**

| 字段 | 内容 |
|------|------|
| **标题** | 通知状态转换：多条未读 → 批量已读 |
| **需求来源** | Story 7 / FR-5 / R20 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 有 5 条未读通知；2. `unread_count` = 5 |
| **测试步骤** | 1. 调用 `PUT /api/v1/notifications/read-all`；2. 查询所有通知的 `is_read` 字段；3. 查询 `unread-count` API |
| **预期结果** | 1. 所有 5 条通知 `is_read` 变为 1；2. `unread_count` 变为 0 |

---

**TC-ST-003**

| 字段 | 内容 |
|------|------|
| **标题** | 通知状态转换：存在 → 软删除 |
| **需求来源** | Story 8 / FR-5 / R21 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 有 1 条通知（id=10001），`deleted_at` = NULL |
| **测试步骤** | 1. 调用 `DELETE /api/v1/notifications/10001`；2. 查询数据库 `deleted_at` 字段；3. 调用 `GET /notifications` |
| **预期结果** | 1. `deleted_at` 从 NULL 变为当前时间戳；2. 通知列表不再包含该通知；3. 数据库记录仍存在 |

---

**TC-ST-004**

| 字段 | 内容 |
|------|------|
| **标题** | 通知生命周期：软删除 → 30 天后物理删除 |
| **需求来源** | Story 8 / 定时清理 / R25 |
| **优先级** | 高 |
| **前置条件** | 1. 一条通知的 `deleted_at` 设为 30 天前的时间；2. `created_at` 也超过 30 天 |
| **测试步骤** | 1. 触发定时清理任务；2. 查询数据库该记录 |
| **预期结果** | 1. 记录已被物理删除；2. 数据库中不存在该记录 |

---

**TC-ST-005**

| 字段 | 内容 |
|------|------|
| **标题** | 事件消费状态：content.replied 事务提交后事件发布 → 通知创建成功 |
| **需求来源** | FR-2 / FR-3 / 风险评估 / R17 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 和 Message Service 均正常运行 |
| **测试步骤** | 1. 调用 `CreateComment` 创建二级评论；2. 等待事务提交；3. 检查 notification.events 队列；4. 等待 Message Service 消费；5. 查询 notifications 表 |
| **预期结果** | 1. 评论创建成功（事务提交）；2. notification.events 队列收到 content.replied 事件；3. notifications 表新增回复通知；4. 全链路状态一致 |

---

**TC-ST-006**

| 字段 | 内容 |
|------|------|
| **标题** | 事件消费状态：content.replied 事务回滚 → 事件不发布 |
| **需求来源** | FR-2 / 风险评估 / R17 |
| **优先级** | 高 |
| **前置条件** | 1. Content Service 正常运行；2. 构造事务提交失败场景 |
| **测试步骤** | 1. 调用 `CreateComment` 创建评论（触发事务回滚）；2. 检查 notification.events 队列 |
| **预期结果** | 1. 评论创建失败（事务回滚）；2. notification.events 队列未收到 content.replied 事件；3. 不存在"有事件无评论"的不一致状态 |

---

**TC-ST-007**

| 字段 | 内容 |
|------|------|
| **标题** | 优雅停止：SIGTERM 时等待 inflight 消息确认后退出 |
| **需求来源** | 成功指标 / R26 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. 有一条消息正在消费中（inflight） |
| **测试步骤** | 1. 向队列发布一条消息；2. 在消息消费过程中发送 SIGTERM 信号；3. 检查消息是否被正确消费 |
| **预期结果** | 1. 消费者不立即退出；2. 等待当前 inflight 消息处理完成并 Ack；3. 服务正常退出；4. 消息不丢失 |

---

**TC-ST-008**

| 字段 | 内容 |
|------|------|
| **标题** | MQ 消费状态：消费成功 → Ack → 消息从队列移除 |
| **需求来源** | FR-3 / R11 |
| **优先级** | 高 |
| **前置条件** | 1. Message Service 正常运行；2. 队列中有待消费消息 |
| **测试步骤** | 1. 发布一条 `content.liked` 事件；2. 等待消费完成；3. 检查 MQ 队列和数据库 |
| **预期结果** | 1. 数据库新增通知记录；2. 消息从 MQ 队列中移除（已 Ack）；3. 队列消息数减少 1 |

---

**TC-ST-009**

| 字段 | 内容 |
|------|------|
| **标题** | 通知创建后修改昵称不影响通知标题快照 |
| **需求来源** | FR-4 / 快照策略 / R24 |
| **优先级** | 中 |
| **前置条件** | 1. 用户 B（昵称"李四"）赞了用户 A 的帖子；2. 用户 A 收到通知，title 含"李四" |
| **测试步骤** | 1. 用户 B 将昵称改为"小明"；2. 查询用户 A 的该条通知；3. 发布新的点赞事件（用户 C 赞同一帖子） |
| **预期结果** | 1. 用户 A 的旧通知 title 仍含"李四"（快照不变）；2. 新通知 title 中使用用户 C 的昵称 |

---

**TC-ST-010**

| 字段 | 内容 |
|------|------|
| **标题** | 已读标记后 unread_count 实时减少 |
| **需求来源** | Story 7 / Story 6 / FR-5 |
| **优先级** | 高 |
| **前置条件** | 1. 用户 A 有 3 条未读通知；2. `unread_count` = 3 |
| **测试步骤** | 1. 调用 `PUT /notifications/:id/read` 标记 1 条为已读；2. 立即调用 `GET /notifications/unread-count`；3. 再标记 1 条已读；4. 再次调用 `GET /notifications/unread-count` |
| **预期结果** | 1. 第一次查询 `count` = 2；2. 第二次查询 `count` = 1 |

---

## 6. 需求-测试用例覆盖矩阵

下表列出 PRD 中每个需求点及其对应的测试用例编号，确保所有需求被完整覆盖。

| 需求编号 | 需求描述 | 对应测试用例 |
|---------|---------|------------|
| **Story 1** | 帖子被点赞后收到通知 | TC-F-001, TC-F-002, TC-F-009, TC-F-043 |
| **Story 2** | 帖子审核结果通知 | TC-F-003, TC-F-004 |
| **Story 3** | 帖子被下架后收到通知 | TC-F-005 |
| **Story 4** | 评论被回复后收到通知 | TC-F-006, TC-F-007, TC-F-008 |
| **Story 5** | 查看通知列表 | TC-F-019, TC-F-020, TC-F-021, TC-F-022, TC-F-023, TC-E-006, TC-E-007, TC-E-008, TC-E-014 |
| **Story 6** | 获取未读数 | TC-F-024, TC-F-025, TC-F-026 |
| **Story 7** | 标记已读 | TC-F-027, TC-F-028, TC-F-029, TC-ST-001, TC-ST-002, TC-ST-010 |
| **Story 8** | 删除通知 | TC-F-030, TC-F-031, TC-E-009, TC-E-010, TC-ST-003, TC-ST-004 |
| **FR-1** | Content Service 双队列投递 | TC-F-010, TC-F-011, TC-F-012, TC-F-013, TC-ERR-010, TC-ERR-011 |
| **FR-2** | content.replied 事件发布 | TC-F-014, TC-F-015, TC-F-016, TC-F-017, TC-F-018, TC-ST-005, TC-ST-006 |
| **FR-3** | Message Service 事件消费 | TC-F-001, TC-F-003, TC-F-005, TC-F-006, TC-F-038, TC-F-039, TC-F-042, TC-F-044, TC-ST-008 |
| **FR-4** | 通知数据模型 | TC-F-035, TC-F-036, TC-F-037, TC-F-040, TC-F-041, TC-E-001, TC-E-002, TC-E-003, TC-E-011, TC-E-012, TC-E-013 |
| **FR-5** | RESTful API | TC-F-019 ~ TC-F-034, TC-E-006 ~ TC-E-008, TC-E-014, TC-E-015, TC-ERR-014 ~ TC-ERR-017 |
| **安全** | JWT 鉴权 + user_id 校验 | TC-F-032, TC-F-033, TC-ERR-012, TC-ERR-013, TC-ERR-016 |
| **school_id 隔离** | 多租户数据隔离 | TC-F-034 |
| **性能** | 事件消费 P95 < 3s、API P95 < 100ms、500 event/s | TC-F-026（实时性验证）、TC-F-044（并发场景） |
| **优雅停止** | SIGTERM 时不丢失 inflight 消息 | TC-ST-007 |
| **30天清理** | 软删除通知 30 天后物理清理 | TC-E-009, TC-E-010, TC-ST-004 |
| **标题快照** | 创建时格式化标题，后续不变 | TC-F-040, TC-F-041, TC-ST-009 |
| **自赞跳过** | user_id == target_user_id 不生成通知 | TC-F-009 |
| **服务注册** | etcd 注册 + Gateway 发现 | TC-F-045 |
| **独立数据库** | 每服务独立 MySQL | TC-F-036 |
| **跨服务回调降级** | 超时/失败时降级处理 | TC-ERR-004, TC-ERR-005, TC-ERR-006, TC-ERR-007, TC-ERR-018 |

### 覆盖率统计

| 类别 | 需求数 | 已覆盖 | 覆盖率 |
|------|-------|-------|-------|
| Story 1 ~ 8 | 8 | 8 | 100% |
| FR-1 ~ FR-5 | 5 | 5 | 100% |
| 非功能需求（安全/性能/架构） | 6 | 6 | 100% |
| **合计** | **19** | **19** | **100%** |

---

*文档生成日期：2026-07-08 | 对应 PRD 版本：message-service-prd.md v1.0*
