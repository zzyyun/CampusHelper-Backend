# Issue #005: 内容审核流程

> **Label**: `epic:content-service` `P1` `feature`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 3 天

---

## 任务描述

实现帖子审核工作流：管理员可对待审核（pending）帖子执行"通过"或"拒绝"操作。审核通过后帖子状态变为 `published`，同时通过 RabbitMQ 发送 `content.published` 事件触发 ES 同步。审核拒绝时填写原因并通知用户。

---

## 技术方案

### 状态流转

```
[发帖] → pending ──→ published ──→ [对外可见 + ES同步]
            │
            └──→ rejected ──→ [通知用户 + 仅作者可见]
```

### 审核接口（Admin Service 调用）

**ApprovePost（审核通过）：**
```protobuf
rpc ApprovePost(ApprovePostRequest) returns (ApprovePostResponse);

message ApprovePostRequest {
  int64 post_id = 1;
  int64 reviewer_id = 2;  // 审核员ID
  string comment = 3;     // 审核备注（可选）
}
```

**RejectPost（审核拒绝）：**
```protobuf
rpc RejectPost(RejectPostRequest) returns (RejectPostResponse);

message RejectPostRequest {
  int64 post_id = 1;
  int64 reviewer_id = 2;
  string reason = 3;      // 拒绝原因（必填）
}
```

**TakedownPost（违规下架）：**
```protobuf
rpc TakedownPost(TakedownPostRequest) returns (TakedownPostResponse);

// 已发布的帖子被举报后强制下架
```

### 核心流程

**1. 审核通过流程：**

```go
func (s *ReviewService) ApprovePost(ctx context.Context, req *ApprovePostRequest) error {
    // 1. 查询帖子（SchoolScope 隔离）
    post, err := s.postRepo.GetPost(ctx, schoolID, req.PostID)
    if err != nil { return err }
    
    // 2. 状态校验（只有 pending 可审核通过）
    if post.Status != PostStatusPending {
        return ErrInvalidStatusTransition
    }
    
    // 3. 更新状态为 published
    post.Status = PostStatusPublished
    if err := s.postRepo.UpdatePost(ctx, post); err != nil {
        return err
    }
    
    // 4. 发送 MQ 事件（ES 同步 + 通知用户）
    return s.mq.Publish(ctx, "content.events", &ContentEvent{
        Type:     "content.published",
        PostID:   post.ID,
        SchoolID: post.SchoolID,
        UserID:   post.UserID,
        TraceID:  traceIDFromContext(ctx),
    })
}
```

**2. 审核拒绝流程：**

```go
func (s *ReviewService) RejectPost(ctx context.Context, req *RejectPostRequest) error {
    if req.Reason == "" {
        return errors.New("拒绝原因不能为空")
    }
    
    post, err := s.postRepo.GetPost(ctx, schoolID, req.PostID)
    if err != nil { return err }
    
    post.Status = PostStatusRejected
    post.RejectReason = req.Reason
    if err := s.postRepo.UpdatePost(ctx, post); err != nil {
        return err
    }
    
    // 发送 MQ 事件（通知用户审核未通过）
    return s.mq.Publish(ctx, "content.events", &ContentEvent{
        Type:   "content.review_result",
        PostID: post.ID,
        UserID: post.UserID,
        Data: map[string]string{
            "result": "rejected",
            "reason": req.Reason,
        },
        TraceID: traceIDFromContext(ctx),
    })
}
```

### MQ 事件定义

```go
// ContentEvent 内容事件
type ContentEvent struct {
    Type     string            `json:"type"`      // content.published / content.review_result
    PostID   int64             `json:"post_id"`
    SchoolID int64             `json:"school_id"`
    UserID   int64             `json:"user_id"`
    Data     map[string]string `json:"data,omitempty"`
    TraceID  string            `json:"trace_id"`  // 全链路追踪
    Time     time.Time         `json:"time"`
}
```

### 数据模型扩展

在 `posts` 表中新增字段：

```sql
ALTER TABLE posts ADD COLUMN reject_reason VARCHAR(500) NULL COMMENT '审核拒绝原因';
ALTER TABLE posts ADD COLUMN reviewer_id BIGINT NULL COMMENT '审核员ID';
ALTER TABLE posts ADD COLUMN reviewed_at TIMESTAMP NULL COMMENT '审核时间';
```

---

## 检查清单

- [ ] 在 `posts` 表添加审核相关字段
- [ ] 实现 `model.ReviewRecord`（如需要单独审核记录表）
- [ ] 实现 `service.ApprovePost` 业务逻辑
- [ ] 实现 `service.RejectPost` 业务逻辑（含原因校验）
- [ ] 实现 `service.TakedownPost` 业务逻辑
- [ ] 实现 MQ 事件发布（带 TraceID）
- [ ] 在 Protobuf 中添加 `ApprovePost` / `RejectPost` / `TakedownPost` 接口
- [ ] 实现 `posts.likes_count` 和 `comment_count` 字段（后续 Issue 使用）
- [ ] 编写单元测试
- [ ] 集成测试：审核通过 → ES 收到事件

---

## 验收标准

- [ ] 只有 `pending` 状态可被审核通过
- [ ] 只有 `pending` 状态可被审核拒绝
- [ ] `published` 状态可被 `TakedownPost` 下架（→ `closed`）
- [ ] 审核通过后 MQ 发送 `content.published` 事件（ES 消费者可接收）
- [ ] 审核拒绝时拒绝原因为空则返回错误
- [ ] MQ 事件消息头携带 TraceID
- [ ] 所有操作记录 `reviewer_id` 和 `reviewed_at`
- [ ] 单元测试 + 集成测试通过

---

## 依赖关系

- **被阻塞**: 
  - #011 Protobuf 接口定义
  - #001 通用帖子基础层
  - #004 DFA 敏感词过滤
- **阻塞**: 
  - #009 ES 异步同步
  - #010 内容搜索（间接依赖 ES 数据）

---

## 备注

- 严格遵循 PRD "Story 5：内容审核" 定义
- 审核操作通过 Admin Service gRPC 调用 Content Service
- 审核记录的详细日志（操作人、时间、原因）应保留至少 90 天
- MQ 事件格式需与 Message Service 团队对齐（RabbitMQ 事件规范见 PRD 附录）