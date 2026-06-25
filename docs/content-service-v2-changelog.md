# Content Service v2.1 变更日志

**版本**：2.1
**日期**：2026-06-25
**作者**：Sarah（产品负责人）+ zzyyun
**关联 Epic**：[#34 Content Service v2.1](https://github.com/zzyyun/CampusHelper-Backend/issues/34)

---

## 📋 概述

Content Service v2.1 修复异步链路（ES 同步消费者上线），扩展二级评论（扁平回复 + 级联软删除），并为下一期独立 Message Service 预留事件契约。

完整需求文档：[docs/content-service-v2-prd.md](./content-service-v2-prd.md)

---

## ✨ 新增功能

### 1. 异步链路激活（Issue #35）

- **`cmd/content/main.go` 启动 ES Sync Consumer goroutine**
  - 订阅 `content.events` 队列
  - 处理 `content.published` 事件 → `esClient.IndexPost`
  - 处理 `content.taken_down` 事件 → `esClient.DeletePost`
  - 优雅停止：收到 SIGTERM 时调用 `Stop()` 等待 in-flight 消息完成

- **新增 ES 配置项**
  - `config/my_config.yaml` 新增 `elasticsearch.addresses` + `elasticsearch.index`
  - `config/config.go` 新增 `ElasticsearchConfig` 结构

- **bug 修复**
  - `ESSyncConsumer.Stop()` 改为 nil-safe，未初始化时调用不 panic

- **链路贯通**
  - 审核通过 → MQ 事件 → ES 索引 → 可被搜索
  - 违规下架 → MQ 事件 → ES 删除

### 2. 二级评论 API（Issue #36）

- **proto 扩展**
  - `CreateCommentRequest` 新增 `parent_id` 字段（0=一级评论，>0=二级回复）

- **service 业务校验**
  - 父评论必须存在
  - 父评论必须未被删除
  - 父评论必须是一级评论（不允许二级再嵌套）
  - 父评论所属帖子必须与请求一致

- **repo 新增方法**
  - `GetCommentByID` — 按 ID 查询单条评论（带 school_id 隔离）

### 3. 评论级联软删除（Issue #37）

- **proto 新增**
  - `ListCommentRepliesRequest/Response` + `ListCommentReplies` RPC

- **service 升级**
  - `DeleteComment` 改为级联模式：删除一级评论时同事务软删除其下所有回复
  - `comment_count` 累加递减（一级 1 + N 条回复）

- **service 新增**
  - `ListCommentReplies` — 查询某条一级评论下的所有回复（游标分页）

- **repo 新增方法**
  - `ListRepliesByParent` — 按父评论 ID 查询回复（游标分页）
  - `CascadeSoftDeleteReplies` — 软删除父评论下所有回复
  - `DecCommentCountBy` — 按指定数量原子递减 comment_count（GREATEST 保护）

---

## 🧪 测试

| 测试文件 | 测试数 | 覆盖 |
|---------|--------|------|
| `cmd/content/service/es_sync_test.go` | 5 | ES Sync Consumer 构造、停止、事件路由 |
| `cmd/content/service/comment_reply_test.go` | 7 | parent_id 校验、嵌套拒绝、错误消息 |
| `cmd/content/service/cascade_delete_test.go` | 6 | 级联删除触发、count 递减、ListReplies 校验 |

合计 **18 个新单测** + 原有测试全部通过。

---

## 📊 提交历史

| Commit | 描述 |
|--------|------|
| `d1f7f2b` | feat(content): 激活 ES 同步消费者（#35） |
| `258e4ae` | feat(content): 扩展二级评论 API（#36） |
| `1c36243` | feat(content): 评论级联软删除 + ListCommentReplies（#37） |
| `<TBD>` | docs(content): v2.1 端到端验证 + 变更日志（#38） |

---

## 🚫 显式未实现（Out of Scope）

- ❌ @ 提及机制（**产品决策移除**）
- ❌ 多层嵌套回复（仅支持二级）
- ❌ 通知消费者实现（独立 `cmd/message/` 服务留待下一期）
- ❌ 微信订阅消息推送
- ❌ 通知 WebSocket 实时推送
- ❌ ES 全量 reindex 工具（手动重建留待运维自助）

---

## 🔜 下一期（Phase 2）

- 独立 `cmd/message/` 服务（含独立数据库 `campus_message`）
- 消费 `content.liked` / `content.review_result` 事件
- 通知列表 API（站内消息中心）
- @mention 强校验（调用 user-service）
- 微信订阅消息推送（可选）

---

*本变更日志为 Content Service v2.1 发版记录，所有功能均经过单元测试验证。端到端验证详见 `cmd/content/service/v2_e2e_test.go`（需 `go test -tags=e2e` 启用）。*