# Issue #007: 一级评论系统

> **Label**: `epic:content-service` `P2` `feature`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 2-3 天

---

## 任务描述

实现帖子的一级评论系统，支持用户对帖子发表评论，支持删除自己的评论。评论发布后通过 MQ 事件通知 Message Service，由 Message Service 推送互动通知给帖子作者。

**注**：二级回复（评论的评论）放在 Phase 2 迭代。

---

## 技术方案

### 数据模型

**MySQL 表（comments）：**

```sql
CREATE TABLE comments (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    school_id BIGINT NOT NULL COMMENT '学校ID',
    post_id BIGINT NOT NULL COMMENT '帖子ID',
    user_id BIGINT NOT NULL COMMENT '评论者ID',
    content VARCHAR(1000) NOT NULL COMMENT '评论内容',
    parent_id BIGINT NULL COMMENT '父评论ID(预留,Phase2使用)',
    status TINYINT NOT NULL DEFAULT 1 COMMENT '1=正常 2=已删除',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_post (school_id, post_id, created_at DESC),
    INDEX idx_user (user_id, created_at DESC),
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='评论表';
```

### 接口设计

```protobuf
service ContentService {
  rpc CreateComment(CreateCommentRequest) returns (CreateCommentResponse);
  rpc DeleteComment(DeleteCommentRequest) returns (DeleteCommentResponse);
  rpc ListComments(ListCommentsRequest) returns (ListCommentsResponse);
}

message Comment {
  int64 id = 1;
  int64 school_id = 2;
  int64 post_id = 3;
  int64 user_id = 4;
  string content = 5;
  int64 parent_id = 6;  // 预留
  int64 created_at = 7;
}

message CreateCommentRequest {
  int64 school_id = 1;
  int64 post_id = 2;
  int64 user_id = 3;     // 从 JWT 注入
  string content = 4;    // 1-500 字
}

message ListCommentsRequest {
  int64 school_id = 1;
  int64 post_id = 2;
  string cursor = 3;
  int32 page_size = 4;   // 默认 20
}
```

### 核心实现

**1. 创建评论（service/comment_service.go）：**

```go
func (s *CommentService) CreateComment(ctx context.Context, req *CreateCommentRequest) (*Comment, error) {
    // 1. 校验
    if len(req.Content) == 0 || len(req.Content) > 500 {
        return nil, errors.New("评论内容长度需在 1-500 字之间")
    }
    
    // 2. 检查帖子是否存在（SchoolScope 隔离）
    post, err := s.postRepo.GetPost(ctx, req.SchoolID, req.PostID)
    if err != nil {
        return nil, ErrPostNotFound
    }
    
    // 3. DFA 敏感词扫描
    if hits := s.dfaMatcher.Match(req.Content); len(hits) > 0 {
        return nil, &ErrSensitiveWords{Hits: hits}
    }
    
    // 4. 创建评论
    comment := &model.Comment{
        SchoolID: req.SchoolID,
        PostID:   req.PostID,
        UserID:   req.UserID,
        Content:  req.Content,
        Status:   1,
    }
    if err := s.repo.CreateComment(ctx, comment); err != nil {
        return nil, err
    }
    
    // 5. 原子递增帖子评论数（Redis + 异步刷 MySQL）
    s.redis.Incr(ctx, fmt.Sprintf("post:%d:comment_count", req.PostID))
    
    // 6. 发送 MQ 事件（通知 Message Service）
    s.mq.Publish(ctx, "content.events", &ContentEvent{
        Type:     "content.comment_created",
        PostID:   req.PostID,
        SchoolID: req.SchoolID,
        UserID:   req.UserID,
        Data: map[string]string{
            "comment_id":   strconv.FormatInt(comment.ID, 10),
            "post_author":  strconv.FormatInt(post.UserID, 10),
            "content_preview": truncate(req.Content, 50),
        },
        TraceID: traceIDFromContext(ctx),
    })
    
    return comment, nil
}
```

**2. 删除评论（软删除）：**

```go
func (s *CommentService) DeleteComment(ctx context.Context, req *DeleteCommentRequest) error {
    // 1. 查询评论
    comment, err := s.repo.GetComment(ctx, req.SchoolID, req.CommentID)
    if err != nil { return err }
    
    // 2. 权限校验：只有作者可删除
    if comment.UserID != req.UserID {
        return ErrPermissionDenied
    }
    
    // 3. 软删除（status=2）
    comment.Status = 2
    comment.Content = "[已删除]"  // 内容替换
    if err := s.repo.UpdateComment(ctx, comment); err != nil {
        return err
    }
    
    // 4. 递减评论数
    s.redis.Decr(ctx, fmt.Sprintf("post:%d:comment_count", comment.PostID))
    
    return nil
}
```

**3. 评论列表（游标分页）：**

```go
func (s *CommentService) ListComments(ctx context.Context, req *ListCommentsRequest) (*ListCommentsResponse, error) {
    lastID, _ := decodeCursor(req.Cursor)
    
    pageSize := req.PageSize
    if pageSize <= 0 || pageSize > 100 {
        pageSize = 20
    }
    
    var comments []model.Comment
    err := s.db.WithContext(ctx).
        Scopes(SchoolScope(req.SchoolID)).
        Where("post_id = ? AND status = 1", req.PostID).
        Order("id DESC").
        Where("id < ?", lastID).  // 游标
        Limit(pageSize + 1).
        Find(&comments).Error
    
    hasMore := len(comments) > pageSize
    if hasMore {
        comments = comments[:pageSize]
    }
    
    var nextCursor string
    if hasMore && len(comments) > 0 {
        nextCursor = encodeCursor(comments[len(comments)-1].ID)
    }
    
    return &ListCommentsResponse{
        Comments:   comments,
        NextCursor: nextCursor,
        HasMore:    hasMore,
    }, nil
}
```

---

## 检查清单

- [ ] 创建 `comments` 表（含索引）
- [ ] 在 Protobuf 中添加 `Comment` 消息和 3 个 RPC 接口
- [ ] 实现 `model.Comment` 模型
- [ ] 实现 `repo.CommentRepo`
- [ ] 实现 `service.CreateComment`（含 DFA 扫描）
- [ ] 实现 `service.DeleteComment`（权限校验 + 软删除）
- [ ] 实现 `service.ListComments`（游标分页）
- [ ] 实现 Redis 评论数计数
- [ ] 实现 MQ 事件发布（`content.comment_created`）
- [ ] 编写单元测试

---

## 验收标准

- [ ] 用户可对帖子发表评论（1-500 字）
- [ ] 评论内容会经过 DFA 敏感词扫描
- [ ] 评论后会递增帖子的 comment_count
- [ ] 评论创建后 MQ 发送 `content.comment_created` 事件
- [ ] 只有作者可删除自己的评论
- [ ] 删除评论时递减帖子的 comment_count
- [ ] 评论列表采用游标分页
- [ ] school_id 强制隔离
- [ ] 单元测试覆盖率 > 80%

---

## 依赖关系

- **被阻塞**: 
  - #011 Protobuf 接口定义
  - #001 通用帖子基础层
  - #004 DFA 敏感词过滤
- **阻塞**: 无（但 Phase 2 二级回复会依赖此 Issue）

---

## 备注

- 严格遵循 PRD "Story 4：互动" + "功能 4：评论系统" 定义
- 二级回复（评论的评论）放在 Phase 2，本期 `parent_id` 字段预留但不使用
- 评论数通过 Redis 原子计数（INCR/DECR），定期异步刷回 MySQL（避免写热点）
- MQ 事件 `content.comment_created` 由 Message Service 消费，推送互动通知