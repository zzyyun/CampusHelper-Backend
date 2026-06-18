# Issue #008: 点赞功能

> **Label**: `epic:content-service` `P2` `feature`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 1-2 天

---

## 任务描述

实现帖子点赞/取消点赞功能。用户对已发布的帖子点赞或取消点赞，操作通过 Redis 原子计数防止并发问题，并通过 MQ 事件通知 Message Service 推送互动通知。

---

## 技术方案

### 数据模型

**点赞关系表（post_likes）：**

```sql
CREATE TABLE post_likes (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    school_id BIGINT NOT NULL,
    post_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_post (school_id, post_id, user_id),  -- 唯一约束防重复点赞
    INDEX idx_post (post_id),
    INDEX idx_user (user_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子点赞关系表';
```

### 接口设计

```protobuf
rpc LikePost(LikePostRequest) returns (LikePostResponse);
rpc UnlikePost(UnlikePostRequest) returns (UnlikePostResponse);

message LikePostRequest {
  int64 school_id = 1;
  int64 post_id = 2;
  int64 user_id = 3;
}

message LikePostResponse {
  bool liked = 1;          // 当前点赞状态
  int32 likes_count = 2;   // 当前点赞总数
}
```

### 核心实现

**1. 点赞（service/like_service.go）：**

```go
func (s *LikeService) LikePost(ctx context.Context, req *LikePostRequest) (*LikePostResponse, error) {
    // 1. 检查帖子是否存在（SchoolScope 隔离）
    post, err := s.postRepo.GetPost(ctx, req.SchoolID, req.PostID)
    if err != nil {
        return nil, ErrPostNotFound
    }
    
    // 2. Redis 去重检查（防止并发重复点赞）
    likeKey := fmt.Sprintf("like:%d:%d", req.PostID, req.UserID)
    exists, _ := s.redis.Exists(ctx, likeKey).Result()
    if exists > 0 {
        // 已点赞，直接返回当前状态
        count, _ := s.getLikesCount(ctx, req.PostID)
        return &LikePostResponse{Liked: true, LikesCount: count}, nil
    }
    
    // 3. 写入点赞关系（DB 唯一约束保证幂等性）
    like := &model.PostLike{
        SchoolID: req.SchoolID,
        PostID:   req.PostID,
        UserID:   req.UserID,
    }
    if err := s.repo.CreateLike(ctx, like); err != nil {
        // 唯一约束冲突 = 已点赞
        if isDuplicateKeyError(err) {
            return &LikePostResponse{Liked: true}, nil
        }
        return nil, err
    }
    
    // 4. Redis 原子计数 +1
    s.redis.Incr(ctx, fmt.Sprintf("post:%d:likes_count", req.PostID))
    s.redis.Set(ctx, likeKey, "1", 0)  // 标记已点赞
    
    // 5. 发送 MQ 事件（通知帖子作者）
    s.mq.Publish(ctx, "content.events", &ContentEvent{
        Type:     "content.liked",
        PostID:   req.PostID,
        SchoolID: req.SchoolID,
        UserID:   req.UserID,
        Data: map[string]string{
            "post_author": strconv.FormatInt(post.UserID, 10),
        },
        TraceID: traceIDFromContext(ctx),
    })
    
    return &LikePostResponse{
        Liked:      true,
        LikesCount: s.getCurrentCount(ctx, req.PostID),
    }, nil
}
```

**2. 取消点赞：**

```go
func (s *LikeService) UnlikePost(ctx context.Context, req *UnlikePostRequest) (*UnlikePostResponse, error) {
    likeKey := fmt.Sprintf("like:%d:%d", req.PostID, req.UserID)
    
    // 1. 删除点赞关系
    rows, err := s.repo.DeleteLike(ctx, req.SchoolID, req.PostID, req.UserID)
    if err != nil { return nil, err }
    
    if rows == 0 {
        // 未点赞，直接返回
        count, _ := s.getLikesCount(ctx, req.PostID)
        return &UnlikePostResponse{Liked: false, LikesCount: count}, nil
    }
    
    // 2. Redis 计数 -1
    s.redis.Decr(ctx, fmt.Sprintf("post:%d:likes_count", req.PostID))
    s.redis.Del(ctx, likeKey)
    
    return &UnlikePostResponse{
        Liked:      false,
        LikesCount: s.getCurrentCount(ctx, req.PostID),
    }, nil
}
```

**3. 计数一致性保障（异步刷回 MySQL）：**

```go
// 定期任务：每 5 分钟将 Redis 计数刷回 MySQL
func (s *LikeService) SyncLikesCountToDB(ctx context.Context) {
    keys, _ := s.redis.Keys(ctx, "post:*:likes_count").Result()
    for _, key := range keys {
        postID, _ := strconv.ParseInt(strings.Split(key, ":")[1], 10, 64)
        count, _ := s.redis.Get(ctx, key).Int()
        
        // 仅在差异较大时更新（避免频繁写）
        post, _ := s.postRepo.GetPostByID(ctx, postID)
        if post != nil && abs(int(post.LikesCount)-count) > 10 {
            s.postRepo.UpdateLikesCount(ctx, postID, int32(count))
        }
    }
}
```

### 并发安全保障

| 风险 | 解决方案 |
|------|---------|
| 重复点赞 | DB 唯一约束 `(school_id, post_id, user_id)` |
| 并发计数错误 | Redis `INCR/DECR` 原子操作 |
| Redis 与 DB 不一致 | 定期任务异步刷回 + 启动时重建 |
| MQ 事件重复发送 | 业务侧幂等（点赞前先 EXISTS 检查） |

---

## 检查清单

- [ ] 创建 `post_likes` 表（含唯一约束）
- [ ] 在 Protobuf 中添加 `LikePost` / `UnlikePost` 接口
- [ ] 实现 `model.PostLike` 模型
- [ ] 实现 `repo.LikeRepo`（含幂等处理）
- [ ] 实现 `service.LikePost` 业务逻辑
- [ ] 实现 `service.UnlikePost` 业务逻辑
- [ ] 实现 Redis 原子计数（INCR/DECR）
- [ ] 实现 Redis 防重标记
- [ ] 实现 MQ 事件发布（`content.liked`）
- [ ] 实现定期任务：Redis → MySQL 计数同步
- [ ] 编写单元测试（覆盖并发场景）

---

## 验收标准

- [ ] 用户对帖子点赞成功
- [ ] 同一用户重复点赞返回幂等成功（无副作用）
- [ ] 取消点赞正确递减计数
- [ ] 点赞后 MQ 发送 `content.liked` 事件
- [ ] Redis 与 MySQL 计数最终一致（允许 5 分钟延迟）
- [ ] school_id 强制隔离
- [ ] 单元测试覆盖幂等和并发场景

---

## 依赖关系

- **被阻塞**: 
  - #011 Protobuf 接口定义
  - #001 通用帖子基础层
- **阻塞**: 无

---

## 备注

- 严格遵循 PRD "Story 4：互动" 中的点赞需求
- 点赞计数**不要求强一致性**，允许最终一致
- 并发安全由 Redis 原子操作 + DB 唯一约束双重保障
- MQ 事件 `content.liked` 由 Message Service 消费，推送通知
- 点赞数同步策略：每 5 分钟一次，差异 > 10 才更新 MySQL（避免写热点）