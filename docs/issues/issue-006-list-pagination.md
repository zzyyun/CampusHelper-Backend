# Issue #006: 帖子列表 + 游标分页

> **Label**: `epic:content-service` `P1` `feature`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 2-3 天

---

## 任务描述

实现帖子列表查询接口，支持按类型/分类/状态筛选，支持按发布时间倒序或点赞数倒序排序，采用**游标分页（Cursor-based Pagination）**适配小程序下拉翻页体验。强制 school_id 隔离。

---

## 技术方案

### 游标分页设计

**为什么用游标分页而非传统 OFFSET/LIMIT？**
- 性能稳定：无论翻到第几页，查询速度一致
- 避免重复/漏数据：新增/删除帖子不影响已加载的列表
- 适配小程序：用户下拉时用"上一页最后一条 ID"作为游标

**游标编码格式：**
```json
// 客户端只需存储上次的 cursor 字符串（Base64 编码）
{"last_id": 12345, "sort_value": "2026-06-08T12:00:00Z"}
```

### 接口设计

```protobuf
message ListPostsRequest {
  int64 school_id = 1;          // 强制注入
  PostType type = 2;            // 可选：通用/失物/二手
  ItemCategory category = 3;    // 可选：物品分类
  PostStatus status = 4;        // 默认 published
  string cursor = 5;            // 上次响应的 next_cursor
  int32 page_size = 6;          // 默认 20，最大 100
  SortType sort = 7;            // time_desc / likes_desc
}

message ListPostsResponse {
  repeated Post posts = 1;
  string next_cursor = 2;       // 下次请求的 cursor
  bool has_more = 3;
}

enum SortType {
  SORT_TYPE_UNSPECIFIED = 0;
  TIME_DESC = 1;     // 发布时间倒序（默认）
  LIKES_DESC = 2;    // 点赞数倒序
}
```

### 核心实现

**1. Service 层（service/post_service.go）：**

```go
func (s *PostService) ListPosts(ctx context.Context, req *ListPostsRequest) (*ListPostsResponse, error) {
    // 1. 解析游标
    lastID, err := decodeCursor(req.Cursor)
    if err != nil && req.Cursor != "" {
        return nil, ErrInvalidCursor
    }
    
    // 2. 构造查询条件
    query := s.db.WithContext(ctx).
        Scopes(SchoolScope(req.SchoolID)).
        Where("status = ?", PostStatusPublished)
    
    if req.Type != PostTypeUnspecified {
        query = query.Where("type = ?", req.Type)
    }
    if req.Category != ItemCategoryUnspecified {
        query = query.Where("item_category = ?", req.Category)
    }
    
    // 3. 游标分页 + 排序
    pageSize := req.PageSize
    if pageSize <= 0 || pageSize > 100 {
        pageSize = 20
    }
    
    var posts []model.Post
    switch req.Sort {
    case SortTimeDesc:
        query = query.Order("created_at DESC, id DESC")
        if lastID > 0 {
            // 联合索引优化：WHERE (created_at, id) < (last_created_at, last_id)
            lastPost, _ := s.postRepo.GetByID(ctx, req.SchoolID, lastID)
            if lastPost != nil {
                query = query.Where(
                    "(created_at, id) < (?, ?)",
                    lastPost.CreatedAt, lastID,
                )
            }
        }
    case SortLikesDesc:
        query = query.Order("likes_count DESC, id DESC")
        if lastID > 0 {
            query = query.Where(
                "(likes_count, id) < (?, ?)",
                lastLikesCount, lastID,
            )
        }
    }
    
    // 4. 多查一条用于判断 has_more
    if err := query.Limit(pageSize + 1).Find(&posts).Error; err != nil {
        return nil, err
    }
    
    hasMore := len(posts) > pageSize
    if hasMore {
        posts = posts[:pageSize]
    }
    
    // 5. 生成下次游标
    var nextCursor string
    if hasMore && len(posts) > 0 {
        nextCursor = encodeCursor(posts[len(posts)-1].ID)
    }
    
    return &ListPostsResponse{
        Posts:      posts,
        NextCursor: nextCursor,
        HasMore:    hasMore,
    }, nil
}
```

**2. 索引设计（性能关键）：**

```sql
-- 列表查询主索引
CREATE INDEX idx_list_time ON posts(school_id, status, type, created_at DESC, id DESC);
CREATE INDEX idx_list_likes ON posts(school_id, status, type, likes_count DESC, id DESC);

-- 分类筛选索引
CREATE INDEX idx_category ON posts(school_id, status, item_category, created_at DESC);
```

**3. Redis 缓存优化（可选，Phase 2）：**

```go
// 热帖缓存（首页第一页）
func (s *PostService) ListHotPosts(ctx context.Context, schoolID int64) ([]Post, error) {
    cacheKey := fmt.Sprintf("hot_posts:%d", schoolID)
    if cached, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
        return deserializePosts(cached), nil
    }
    
    posts, err := s.repo.ListHot(ctx, schoolID, 20)
    if err == nil {
        s.redis.Set(ctx, cacheKey, serialize(posts), 5*time.Minute)
    }
    return posts, nil
}
```

### 游标编解码工具

```go
// encodeCursor 编码游标（Base64）
func encodeCursor(lastID int64) string {
    payload := fmt.Sprintf(`{"last_id":%d}`, lastID)
    return base64.StdEncoding.EncodeToString([]byte(payload))
}

// decodeCursor 解码游标
func decodeCursor(cursor string) (int64, error) {
    if cursor == "" {
        return 0, nil
    }
    data, err := base64.StdEncoding.DecodeString(cursor)
    if err != nil {
        return 0, err
    }
    var p struct {
        LastID int64 `json:"last_id"`
    }
    if err := json.Unmarshal(data, &p); err != nil {
        return 0, err
    }
    return p.LastID, nil
}
```

---

## 检查清单

- [ ] 在 Protobuf 中添加 `ListPostsRequest` / `ListPostsResponse` / `SortType`
- [ ] 添加数据库索引（`idx_list_time`、`idx_list_likes`、`idx_category`）
- [ ] 实现 `service.ListPosts` 业务逻辑
- [ ] 实现游标编解码工具
- [ ] 实现时间倒序和点赞倒序两种排序
- [ ] 实现类型/分类/状态筛选
- [ ] 强制 SchoolScope 隔离
- [ ] 实现 `has_more` 判断逻辑
- [ ] 编写单元测试（覆盖空游标、非空游标、边界）
- [ ] 编写性能测试（10000 条数据下响应时间 < 200ms）

---

## 验收标准

- [ ] 默认按发布时间倒序
- [ ] 支持切换为点赞数倒序
- [ ] 支持按 `type`、`item_category` 筛选
- [ ] 游标分页稳定（无重复/漏数据）
- [ ] school_id 强制隔离（错误 school_id 返回空）
- [ ] 响应时间 < 200ms（含 Redis 缓存）
- [ ] 单元测试覆盖率 > 85%
- [ ] 性能测试通过（10000 条数据下 < 200ms）

---

## 依赖关系

- **被阻塞**: 
  - #011 Protobuf 接口定义
  - #001 通用帖子基础层
- **阻塞**: 无（但 #010 内容搜索 会复用部分查询逻辑）

---

## 备注

- 严格遵循 PRD "Story 3：搜索和筛选帖子" 和 "功能 1：通用帖子基础层" 定义
- 游标编码建议使用 JSON + Base64，便于后续扩展字段
- 性能关键：复合索引 `(school_id, status, type, created_at, id)` 必须存在
- Redis 缓存策略放在 Phase 2，本期直接走 DB 查询