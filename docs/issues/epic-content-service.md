# Epic: Content Service（内容服务）

> **Label**: `epic:content-service`  
> **优先级**: P1  
> **里程碑**: Phase 1 (MVP) - Week 2 交付  
> **状态**: 待开发  
> **依赖 PRD**: [docs/content-service-prd.md](../content-service-prd.md)

---

## 📋 概述

Content Service 是校园互助平台（CampusHelper）的核心业务服务，为微信小程序提供**帖子发布、内容发现和社区互动**能力。该服务以"通用帖子"为基础架构，向上扩展出**失物招领**和**二手交易**两种业务模板，覆盖大学生在校园内最高频的信息共享场景。

**设计目标**：在 Go 微服务架构下完整实践多租户数据隔离、DFA 内容审核、基于 RabbitMQ 的异步 ES 同步、gRPC 服务间调用，以及 OpenTelemetry 全链路追踪。

---

## 🎯 核心业务目标

| 业务场景 | 价值 | 优先级 |
|---------|------|--------|
| 失物招领发布 | 解决学生丢失物品找回问题 | P1 |
| 二手交易发布 | 解决闲置物品流通需求 | P1 |
| 内容审核 | 保障平台内容合规 | P0 |
| 全文搜索 | 提升内容发现效率 | P1 |
| 多租户隔离 | 保证不同学校数据隔离 | P0 |
| 全链路追踪 | 实现可观测性 | P0 |

---

## 📦 子任务清单

### 基础设施（P0 - 必须先完成）

- [ ] **#011** [Protobuf 接口定义](issue-011-protobuf.md) (P0) - **起点**
- [ ] **#001** [通用帖子基础层](issue-001-post-base.md) (P0)

### 核心功能（P1 - MVP 必交付）

- [ ] **#004** [DFA 敏感词过滤](issue-004-dfa-filter.md) (P1)
- [ ] **#005** [内容审核流程](issue-005-review-flow.md) (P1)
- [ ] **#006** [帖子列表 + 游标分页](issue-006-list-pagination.md) (P1)
- [ ] **#009** [ES 异步同步](issue-009-es-sync.md) (P1)
- [ ] **#010** [内容搜索](issue-010-search.md) (P1)

### 业务扩展（P2 - MVP 可选）

- [ ] **#002** [失物招领模板](issue-002-lost-found.md) (P2)
- [ ] **#003** [二手交易模板](issue-003-second-hand.md) (P2)
- [ ] **#007** [一级评论系统](issue-007-comment-level1.md) (P2)
- [ ] **#008** [点赞功能](issue-008-like-feature.md) (P2)

---

## 🔗 依赖关系图

```text
#011 (Protobuf) ───┬───→ #001 (帖子基础层) ───┬───→ #002 (失物招领)
                   │                          ├───→ #003 (二手交易)
                   │                          ├───→ #006 (列表分页)
                   │                          └───→ #007 (评论)
                   │                                      │
                   └───→ #004 (DFA) ──→ #005 (审核) ─────┴───→ #008 (点赞)
                                              │
                                              ├───→ #009 (ES同步)
                                              └───→ #010 (搜索)
```

**关键路径**：`#011 → #001 → #004 → #005 → #009 → #010`

---

## ✅ 验收标准（MVP 整体）

### 功能验收

- [ ] 完整走通"发帖 → 审核 → 发布 → 搜索"链路
- [ ] 通用帖子、失物招领、二手交易三种模板均可发布
- [ ] DFA 敏感词命中时拒绝并返回敏感词位置
- [ ] 帖子列表支持游标分页 + 时间/点赞排序
- [ ] 审核通过后帖子自动同步到 ES，< 5s 可搜索
- [ ] 一级评论和点赞功能可用
- [ ] 所有读操作强制 school_id 隔离

### 技术验收

- [ ] HTTP → gRPC → MQ 全程有 TraceID，Jaeger 中可检索完整 Span
- [ ] gRPC 服务注册到 etcd，Gateway 自动发现
- [ ] MQ 消息头携带 Trace 上下文
- [ ] 所有查询通过 GORM `SchoolScope` 全局注入 school_id
- [ ] 写操作（发帖/评论/点赞）需 JWT Token，读操作可匿名
- [ ] 单元测试覆盖核心业务逻辑
- [ ] 接口响应时间：帖子列表 < 200ms，ES 搜索 < 500ms

### 集成验收

- [ ] File Service gRPC 集成（图片上传）
- [ ] Message Service MQ 事件发布（互动通知）
- [ ] Admin Service gRPC 接口暴露（审核操作）
- [ ] Elasticsearch 8 异步同步消费者
- [ ] Redis 热数据缓存 + DFA 词库缓存

---

## 📊 成功指标

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| 全链路追踪覆盖 | HTTP → gRPC → MQ 全程有 TraceID | Jaeger 中可检索完整 Span |
| ES 同步延迟 | 审核通过后 < 5s 可搜索 | 功能测试验证 |
| 接口响应时间 | 帖子列表 < 200ms | 压测工具验证（wrk/JMeter） |
| 并发支持 | < 1000 并发用户 | Phase 5 压测验证 |
| 内容违规率 | DFA 过滤 + 人工抽查 | 审核日志统计 |

---

## 📁 涉及的服务/组件

| 组件 | 角色 |
|------|------|
| `cmd/content/` | Content Service 启动入口 |
| `internal/content/` | 业务逻辑（service、model、repo、handler） |
| `internal/gateway/` | Gateway 路由 + gRPC 客户端调用 |
| `pkg/db/` | GORM 初始化 + `SchoolScope` |
| `pkg/mq/` | RabbitMQ Producer/Consumer（带 Trace 透传） |
| `pkg/es/` | ES 客户端封装 |
| `pkg/contextx/` | Context 扩展（school_id、user_id、trace_id） |
| `PB/pb/content.proto` | Protobuf 定义 |

---

## 🚧 不在本期范围

- 二级评论回复（Phase 2）
- 帖子自动过期 + 续期提醒（Phase 2）
- Redis 热帖缓存优化（Phase 2）
- 收藏功能（Phase 2）
- 校园地图 API 对接（未来）
- 担保交易/意向金（未来）
- 帖子举报与自动下架（未来）

---

## 🔌 外部依赖

| 依赖服务 | 集成方式 | 状态 |
|---------|---------|------|
| File Service | gRPC | 需同步开发或前期就绪 |
| Message Service | RabbitMQ 事件 | 需协同定义事件 schema |
| Admin Service | 暴露 gRPC 接口 | 需协同定义接口 |
| MySQL + Redis + RabbitMQ + ES + Jaeger | 基础设施 | Phase 1 前需就绪 |
| etcd | 服务发现 | 需就绪 |

---

## 📝 备注

- 本 Epic 严格遵循 PRD（[content-service-prd.md](../content-service-prd.md)）的定义
- 所有代码注释必须使用简体中文（参见 CLAUDE.md 全局规范）
- 多租户隔离是**强制约束**，任何数据查询都必须携带 school_id
- TraceID 必须贯穿所有调用链，这是项目的核心技术价值之一