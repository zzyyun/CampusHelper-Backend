# 产品需求文档：Content Service（内容服务）

**版本**：1.0  
**日期**：2026-06-08  
**作者**：Sarah（产品负责人）  
**质量评分**：91/100  

---

## 执行摘要

Content Service 是校园互助平台（CampusHelper）的核心业务服务，为微信小程序提供帖子发布、内容发现和社区互动能力。该服务以"通用帖子"为基础架构，向上扩展出**失物招领**和**二手交易**两种业务模板，覆盖大学生在校园内最高频的信息共享场景。

作为学习项目，Content Service 的设计目标是在 Go 微服务架构下完整实践：多租户数据隔离、DFA 内容审核、基于 RabbitMQ 的异步 ES 同步、gRPC 服务间调用，以及 OpenTelemetry 全链路追踪。通过本服务的实现，验证整体微服务架构的可行性和各组件的集成方式。

---

## 问题陈述

**当前痛点**：大学生在校园内有强烈的失物找回、二手物品流通和信息共享需求，但现有通用社交平台缺乏针对校园场景的业务字段和多校区隔离能力。

**解决方案**：构建以学校为隔离单元的内容发布平台，通过业务模板化设计，为不同场景提供定制化字段和状态流转，同时确保不同学校的学生只能看到本校内容。

**技术价值**：完整实现微服务架构中的内容服务层，作为整个平台技术架构的核心验证点。

---

## 成功指标

鉴于本项目为学习/练手项目，成功标准以技术实现完整性为核心：

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| 全链路追踪覆盖 | HTTP → gRPC → MQ 全程有 TraceID | Jaeger 中可检索完整 Span |
| ES 同步延迟 | 审核通过后 < 5s 可搜索 | 功能测试验证 |
| 接口响应时间 | 帖子列表接口 < 200ms | 压测工具验证（wrk/JMeter） |
| 并发支持 | 支持 < 1000 并发用户 | Phase 5 压测验证 |
| 内容违规率 | DFA 过滤 + 人工抽查控制违规内容 | 审核日志统计 |

---

## 用户画像

### 主要用户：普通学生（发帖者）

- **角色**：在校大学生，有信息发布需求
- **目标**：快速发布失物招领、出售闲置物品、求助信息
- **痛点**：现有平台无法针对性填写失物特征；找到有意向的买家后沟通成本高
- **使用频率**：低频发布，中高频浏览
- **技术水平**：普通微信小程序用户

### 次要用户：普通学生（浏览者）

- **角色**：在校大学生，有信息获取需求
- **目标**：搜索丢失物品信息、发现合适的二手商品、获取校园动态
- **痛点**：无法精确筛选同校区内的帖子；搜索结果不准确
- **使用频率**：高频浏览

### 运营用户：管理员（审核者）

- **角色**：学校管理员或平台运营人员
- **目标**：高效审核内容、维护平台内容质量
- **痛点**：需要逐条审核效率低；缺乏辅助信息判断违规内容
- **技术水平**：内部系统操作员

---

## 用户故事与验收标准

### Story 1：发布失物招领帖

**作为**一名丢失物品的学生  
**我想要**发布一条包含丢失地点和物品特征的失物招领帖  
**以便于**让拾到物品的人能联系我

**验收标准：**
- [ ] 帖子必须填写：标题、描述、丢失/拾取地点（文字）、物品分类（枚举）、联系方式（手机/微信）
- [ ] 支持上传多张物品图片
- [ ] 提交时 DFA 扫描：命中敏感词则拒绝并高亮标出敏感词，返回错误提示
- [ ] 未命中敏感词：帖子进入"审核中"状态，管理员人工抽查
- [ ] 审核通过后帖子状态变为"已发布"，同时通过 MQ 异步同步到 ES
- [ ] 发帖者收到审核结果通知（调用 Message Service）
- [ ] 帖子默认 30 天后自动过期，过期前通知用户可选续期或关闭
- [ ] 发帖者可手动将帖子标记为"已当领"并关闭

### Story 2：发布二手交易帖

**作为**想出售闲置物品的学生  
**我想要**发布二手商品信息  
**以便于**找到有意向的买家进行线下成交

**验收标准：**
- [ ] 帖子必须填写：标题、描述、价格、成色等级（全新/几乎全新/良好/一般）、交易方式（面交/快递）、商品类别
- [ ] 支持填写原价（可选）
- [ ] 支持上传多张商品图片
- [ ] 帖子状态：出售中 / 已喇出
- [ ] 平台不介入资金流转，仅提供信息展示
- [ ] 帖子默认 60 天后自动过期，支持续期

### Story 3：搜索和筛选帖子

**作为**浏览平台的学生  
**我想要**通过关键词或分类快速找到相关帖子  
**以便于**高效获取所需信息

**验收标准：**
- [ ] 默认只展示本学校（school_id）的帖子，强制隔离
- [ ] 支持关键词全文搜索（标题 + 描述，走 ES）
- [ ] 支持按帖子类型（失物/二手/通用）筛选
- [ ] 支持按商品类别标签筛选
- [ ] 帖子列表默认按发布时间倒序，支持切换为按点赞数排序
- [ ] 列表采用游标加载（cursor-based），适配小程序下拉翻页体验

### Story 4：互动（评论与点赞）

**作为**浏览帖子的学生  
**我想要**对帖子进行评论和点赞  
**以便于**与发帖者或其他用户互动

**验收标准：**
- [ ] 支持对帖子发表一级评论
- [ ] 支持对评论发表二级回复（仅限两层，不支持无限嵌套）
- [ ] 支持对帖子点赞/取消点赞
- [ ] 评论/点赞操作后，通过 MQ 事件触发 Message Service 向帖子作者推送互动通知
- [ ] 帖子详情页展示评论总数和点赞总数

### Story 5：内容审核（管理员）

**作为**内容审核员  
**我想要**高效审核待审核的帖子  
**以便于**保障平台内容质量

**验收标准：**
- [ ] 审核列表展示所有"审核中"状态的帖子
- [ ] 审核详情页展示：帖子原文和图片、发帖用户信息和历史违规记录、DFA 命中的敏感词详情
- [ ] 支持一键"审核通过"（帖子发布 + ES 同步触发）
- [ ] 支持"审核拒绝"并填写拒绝原因（原因同步推送给用户）
- [ ] 审核操作通过 Admin Service gRPC 调用 Content Service 的审核接口

---

## 功能需求

### 核心功能

#### 功能 1：通用帖子基础层

- **描述**：所有内容类型的公共属性和行为，包括标题、正文、图片列表、状态机、评论、点赞
- **数据字段**：`id`, `school_id`, `user_id`, `type`（枚举：general/lost_found/second_hand）, `title`, `content`, `images[]`, `status`（pending/published/expired/closed）, `likes_count`, `comment_count`, `created_at`, `expired_at`
- **状态流转**：`pending` → `published` → `expired`；任意状态可 → `closed`
- **边界**：所有查询必须携带 school_id，GORM 通过 `SchoolScope` 全局注入

#### 功能 2：失物招领模板

- **扩展字段**：`location`（丢失/拾取地点文字）, `item_category`（枚举）, `contact`（手机/微信）, `lost_or_found`（丢失/拾到）
- **特有状态**：帖子状态追加"已当领"(retrieved)
- **过期规则**：发布后 30 天自动过期，过期前 3 天通过 Message Service 提醒用户

#### 功能 3：二手交易模板

- **扩展字段**：`price`（期望售价）, `original_price`（原价，可选）, `condition`（成色枚举：brand_new/like_new/good/fair）, `trade_method`（face_to_face/delivery）, `item_category`
- **特有状态**：帖子状态追加"已喇出"(sold)
- **过期规则**：发布后 60 天自动过期，支持用户手动续期

#### 功能 4：评论系统

- **结构**：两层评论树（一级评论 + 二级回复）
- **接口**：创建/删除评论，创建/删除回复
- **分页**：评论列表采用游标加载
- **通知**：发布评论/回复后，发送 MQ 事件给 Message Service

#### 功能 5：内容搜索

- **全文搜索**：通过 ES 对 `title`、`content` 进行全文检索
- **筛选**：支持按 `type`、`item_category`、`status` 过滤
- **强制条件**：所有搜索请求必须包含 `school_id` 条件
- **ES 同步**：帖子审核通过后，向 MQ 发送 `content.published` 事件，消费者异步写入 ES

#### 功能 6：DFA 敏感词过滤

- **时机**：发帖时在写入数据库前执行
- **命中处理**：拒绝创建，返回 400 错误，响应体包含命中的敏感词列表及位置
- **未命中**：帖子进入 `pending` 状态，等待人工抽查

### 不在本期范围内

- 收藏功能（后期迭代）
- 平台内担保交易/意向金（后期迭代）
- 校园地图 API 地理位置标记（后期迭代）
- 帖子举报功能（后期迭代）
- 帖子分享卡片生成（后期迭代）

---

## 技术约束

### 性能

- 帖子列表接口（含 Redis 缓存热数据）响应时间 < 200ms
- ES 搜索接口响应时间 < 500ms
- 支持并发用户数 < 1000（Phase 5 压测验证）

### 安全与合规

- **多租户隔离**：所有数据查询通过 GORM `SchoolScope` 强制注入 `school_id` 条件，网关层从 JWT 解析 school_id 注入 Context
- **DFA 敏感词**：全量拦截违规内容，命中词列表存储于 Redis 缓存
- **接口鉴权**：写操作（发帖、评论、点赞）需携带有效 JWT Token，读操作（列表、搜索）可匿名访问

### 集成依赖

| 依赖服务 | 集成方式 | 用途 |
|---------|---------|------|
| **File Service** | gRPC | 帖子图片上传，返回 CDN URL |
| **Message Service** | RabbitMQ 事件发布 | 互动通知、审核结果通知、过期提醒 |
| **Admin Service** | 暴露 gRPC 接口供调用 | 接收审核通过/拒绝指令 |
| **Elasticsearch** | RabbitMQ 消费者异步写入 | 全文搜索索引维护 |
| **Redis** | go-redis/v9 直接调用 | 热帖缓存、点赞计数、DFA 词库 |

### gRPC 服务接口

```protobuf
service ContentService {
  // 帖子 CRUD（供 Gateway 调用）
  rpc CreatePost(CreatePostRequest) returns (CreatePostResponse);
  rpc GetPost(GetPostRequest) returns (GetPostResponse);
  rpc UpdatePost(UpdatePostRequest) returns (UpdatePostResponse);
  rpc DeletePost(DeletePostRequest) returns (DeletePostResponse);
  rpc ListPosts(ListPostsRequest) returns (ListPostsResponse);    // 游标分页

  // 评论与点赞（供 Gateway 调用）
  rpc CreateComment(CreateCommentRequest) returns (CreateCommentResponse);
  rpc DeleteComment(DeleteCommentRequest) returns (DeleteCommentResponse);
  rpc LikePost(LikePostRequest) returns (LikePostResponse);
  rpc UnlikePost(UnlikePostRequest) returns (UnlikePostResponse);

  // 内容搜索（代理 ES 查询，供 Gateway 调用）
  rpc SearchContent(SearchContentRequest) returns (SearchContentResponse);

  // 审核操作（供 Admin Service 调用）
  rpc ApprovePost(ApprovePostRequest) returns (ApprovePostResponse);
  rpc RejectPost(RejectPostRequest) returns (RejectPostResponse);
  rpc TakedownPost(TakedownPostRequest) returns (TakedownPostResponse);
}
```

### 技术栈约束

- **语言**：Go 1.22+
- **框架**：gRPC + Protobuf（内部通信）、Gin（仅 Gateway 层）
- **存储**：MySQL 8.0（GORM）+ Redis 7 + Elasticsearch 8
- **消息队列**：RabbitMQ（amqp091-go），消息需携带 TraceID 以保证链路完整性
- **链路追踪**：所有 gRPC 调用和 MQ 消息通过 OpenTelemetry 注入/提取 TraceID

---

## MVP 范围与分阶段计划

### Phase 1（MVP）—— Week 2 交付

必须实现：
- 通用帖子 CRUD + 学校隔离
- 失物招领模板（含特有字段）
- 二手交易模板（含特有字段）
- DFA 敏感词过滤（命中拒绝）
- 帖子内容审核流程（pending → published）
- 帖子列表 + 游标分页 + 时间/点赞数排序
- 一级评论（发布/删除）
- 点赞/取消点赞
- ES 异步同步（RabbitMQ）
- 关键词 + 分类筛选搜索

**MVP 定义**：能完整走通"发帖 → 审核 → 发布 → 搜索"链路，并在 Jaeger 中看到完整 TraceID 贯穿 HTTP → gRPC → MQ → 消费者。

### Phase 2（增强）—— 后期迭代

- 二级评论回复（评论的评论）
- 帖子自动过期 + 续期提醒（RabbitMQ 延迟队列）
- 互动通知完整集成（Message Service）
- Redis 热帖缓存优化
- 收藏功能

### 未来考虑

- 校园地图 API 对接（失物位置可视化）
- 担保交易 / 意向金体系
- 帖子举报与自动下架机制

---

## 风险评估

| 风险 | 概率 | 影响 | 缓解策略 |
|------|------|------|---------|
| ES 同步延迟导致搜索不及时 | 中 | 中 | 采用最终一致性设计，搜索结果延迟 < 5s 可接受；监控 MQ 消费延迟 |
| DFA 敏感词词库维护成本 | 低 | 中 | 词库存 Redis，支持热更新，无需重启服务 |
| gRPC 跨服务 TraceID 断链 | 中 | 高 | 使用 otelgrpc 拦截器自动注入，MQ 消息头携带 Trace 上下文 |
| 高并发抢占评论/点赞计数 | 低 | 低 | Redis 原子计数（INCR/DECR），定期异步刷回 MySQL |
| 二手帖状态不一致（已喇出但仍被联系） | 低 | 低 | 状态变更通过单一入口，状态可见即为最新；纯信息展示，无资金风险 |

---

## 依赖与阻塞项

**服务依赖：**
- **File Service**：图片上传接口必须在 Content Service 之前完成，或同步开发
- **Message Service**：互动通知功能依赖 Message Service 消费 MQ 事件
- **Admin Service**：审核接口供 Admin Service 调用，需协同定义 Protobuf 接口

**基础设施依赖（Phase 1 前需就绪）：**
- MySQL + Redis + RabbitMQ + ES + Jaeger 环境（参考 Phase 1 基建）
- etcd 服务发现，用于 Content Service 注册和 Gateway 发现

---

## 附录

### 帖子状态枚举

| 状态 | 说明 |
|------|------|
| `pending` | 审核中（DFA 通过，等待人工抽查） |
| `published` | 已发布（审核通过，对外可见） |
| `expired` | 已过期（超过有效期自动转换） |
| `closed` | 已关闭（用户主动关闭，如失物已找到/商品已售出） |
| `rejected` | 已拒绝（审核未通过，仅发帖者可见） |

### 物品分类枚举（失物 & 二手共用部分）

手机/数码、证件/卡类、钥匙/门卡、书籍/资料、服装/饰品、充电器/配件、生活用品、其他

### RabbitMQ 事件规范

| 事件名 | 触发时机 | 消费方 |
|--------|---------|-------|
| `content.published` | 帖子审核通过 | ES 消费者（写入索引） |
| `content.comment_created` | 新评论/回复发布 | Message Service（互动通知） |
| `content.liked` | 帖子被点赞 | Message Service（互动通知） |
| `content.review_result` | 审核通过/拒绝 | Message Service（系统通知） |
| `content.expiring_soon` | 帖子距过期 ≤ 3 天 | Message Service（过期提醒） |

### 术语表

- **school_id**：多租户隔离键，标识帖子所属学校，来源于 JWT Token，由网关注入 Context
- **DFA**：Deterministic Finite Automaton，确定有限自动机，用于高性能敏感词多模式匹配
- **游标加载（Cursor-based）**：以上一次响应最后一条记录的 ID 作为下次请求的游标，适合小程序无限下拉场景
- **最终一致性**：帖子发布后通过异步 MQ 同步至 ES，搜索存在秒级延迟但最终数据一致

---

*本 PRD 通过与产品负责人的交互式需求澄清生成，质量评分 91/100，覆盖业务目标、功能需求、用户体验和技术约束四个维度。*
