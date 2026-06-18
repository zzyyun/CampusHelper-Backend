# Issue #010: 内容搜索

> **Label**: `epic:content-service` `P1` `feature`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 2-3 天

---

## 任务描述

实现 Content Service 的全文搜索接口，代理 ES 查询。支持关键词搜索（标题 + 正文）+ 分类筛选 + 帖子类型筛选，强制 school_id 多租户隔离。

---

## 技术方案

### 接口设计

```protobuf
rpc SearchContent(SearchContentRequest) returns (SearchContentResponse);

message SearchContentRequest {
  int64 school_id = 1;          // 强制注入
  string keyword = 2;           // 搜索关键词(可选)
  PostType type = 3;            // 帖子类型筛选
  ItemCategory category = 4;    // 物品分类筛选
  PostStatus status = 5;        // 默认 published
  int32 page = 6;               // 页码（从 1 开始）
  int32 page_size = 7;          // 默认 20
  SortType sort = 8;            // relevance / time_desc / likes_desc
}

message SearchContentResponse {
  repeated Post posts = 1;
  int64 total = 2;              // 总命中数
  int32 page = 3;
  int32 page_size = 4;
  int64 took_ms = 5;            // ES 查询耗时
}

enum SortType {
  SORT_TYPE_UNSPECIFIED = 0;
  RELEVANCE = 1;     // 相关度排序（默认）
  TIME_DESC = 2;
  LIKES_DESC = 3;
}
```

### 核心实现

**1. ES 查询构造（service/search_service.go）：**

```go
func (s *SearchService) SearchContent(ctx context.Context, req *SearchContentRequest) (*SearchContentResponse, error) {
    startTime := time.Now()
    
    // 1. 构造 ES 查询（Bool Query）
    must := []map[string]interface{}{}
    filter := []map[string]interface{}{
        // 强制 school_id 隔离（绝对不能漏！）
        {"term": map[string]interface{}{"school_id": req.SchoolID}},
        {"term": map[string]interface{}{"status": int(PostStatusPublished)}},
    }
    
    // 关键词搜索（标题 + 正文）
    if req.Keyword != "" {
        must = append(must, map[string]interface{}{
            "multi_match": map[string]interface{}{
                "query":     req.Keyword,
                "fields":    []string{"title^3", "content"},  // 标题权重更高
                "type":      "best_fields",
                "fuzziness": "AUTO",                         // 模糊匹配
            },
        })
    } else {
        must = append(must, map[string]interface{}{
            "match_all": map[string]interface{}{},
        })
    }
    
    // 帖子类型筛选
    if req.Type != PostTypeUnspecified {
        filter = append(filter, map[string]interface{}{
            "term": map[string]interface{}{"type": int(req.Type)},
        })
    }
    
    // 物品分类筛选
    if req.Category != ItemCategoryUnspecified {
        filter = append(filter, map[string]interface{}{
            "term": map[string]interface{}{"item_category": int(req.Category)},
        })
    }
    
    // 2. 构造排序
    sort := []map[string]interface{}{}
    switch req.Sort {
    case SortRelevance:
        sort = append(sort, map[string]interface{}{"_score": "desc"})
    case SortTimeDesc:
        sort = append(sort, map[string]interface{}{"created_at": "desc"})
    case SortLikesDesc:
        sort = append(sort, map[string]interface{}{"likes_count": "desc"})
    }
    sort = append(sort, map[string]interface{}{"id": "desc"})  // 二级排序
    
    // 3. 分页
    page := req.Page
    if page < 1 { page = 1 }
    pageSize := req.PageSize
    if pageSize <= 0 || pageSize > 100 { pageSize = 20 }
    from := (page - 1) * pageSize
    
    // 4. 执行 ES 查询
    query := map[string]interface{}{
        "from":  from,
        "size":  pageSize,
        "query": map[string]interface{}{
            "bool": map[string]interface{}{
                "must":   must,
                "filter": filter,
            },
        },
        "sort":             sort,
        "track_total_hits": true,
        "_source":          []string{"id", "school_id", "user_id", "type", "title", "content", "status", "item_category", "price", "likes_count", "comment_count", "created_at"},
    }
    
    res, err := s.esClient.Search(
        s.esClient.Search.WithContext(ctx),
        s.esClient.Search.WithIndex("posts_v1"),
        s.esClient.Search.WithBody(strings.NewReader(mustJSON(query))),
        s.esClient.Search.WithTrackTotalHits(true),
    )
    if err != nil {
        return nil, fmt.Errorf("ES 查询失败: %w", err)
    }
    defer res.Body.Close()
    
    // 5. 解析结果
    var esResp struct {
        Took int64 `json:"took"`
        Hits struct {
            Total struct {
                Value int64 `json:"value"`
            } `json:"total"`
            Hits []struct {
                ID     string                 `json:"_id"`
                Source map[string]interface{} `json:"_source"`
                Score  float64                `json:"_score"`
            } `json:"hits"`
        } `json:"hits"`
    }
    if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
        return nil, err
    }
    
    // 6. 转换为 Post 列表
    posts := make([]*model.Post, 0, len(esResp.Hits.Hits))
    for _, hit := range esResp.Hits.Hits {
        posts = append(posts, esDocToPost(hit.Source))
    }
    
    return &SearchContentResponse{
        Posts:    posts,
        Total:    esResp.Hits.Total.Value,
        Page:     page,
        PageSize: pageSize,
        TookMs:   esResp.Took,
    }, nil
}
```

**2. 高亮支持（可选）：**

```go
// 在查询中添加 highlight 配置
query["highlight"] = map[string]interface{}{
    "fields": map[string]interface{}{
        "title":   map[string]interface{}{},
        "content": map[string]interface{}{
            "fragment_size":       150,
            "number_of_fragments": 3,
        },
    },
    "pre_tags":  []string{"<em>"},
    "post_tags": []string{"</em>"},
}
```

### 性能优化

**1. 查询缓存（Redis）：**

```go
func (s *SearchService) SearchWithCache(ctx context.Context, req *SearchContentRequest) (*SearchContentResponse, error) {
    // 仅对热门搜索词缓存
    if req.Keyword == "" {
        return s.SearchContent(ctx, req)  // 无关键词走原始查询
    }
    
    cacheKey := fmt.Sprintf("search:%d:%s:%d:%d:%d",
        req.SchoolID, req.Keyword, req.Type, req.Category, req.Page)
    
    if cached, err := s.redis.Get(ctx, cacheKey).Result(); err == nil {
        var resp SearchContentResponse
        json.Unmarshal([]byte(cached), &resp)
        return &resp, nil
    }
    
    resp, err := s.SearchContent(ctx, req)
    if err == nil && resp.Total > 0 {
        s.redis.Set(ctx, cacheKey, mustJSONString(resp), 5*time.Minute)
    }
    return resp, nil
}
```

**2. ES 索引优化（已在 #009 完成）：**

- 使用 IK 分词器（中文友好）
- `school_id` 字段类型 `long`（便于 term filter）
- `title^3` 标题权重提升

---

## 检查清单

- [ ] 在 Protobuf 中添加 `SearchContentRequest` / `SearchContentResponse`
- [ ] 实现 `service.SearchContent` 业务逻辑
- [ ] 实现 ES Bool Query 构造（must + filter）
- [ ] 实现 school_id 强制过滤（filter 中）
- [ ] 实现关键词 multi_match 搜索（title^3 + content）
- [ ] 实现类型和分类筛选
- [ ] 实现三种排序（相关度/时间/点赞）
- [ ] 实现分页（from/size）
- [ ] 实现 Redis 缓存层（可选）
- [ ] 编写单元测试
- [ ] 编写性能测试（响应时间 < 500ms）

---

## 验收标准

- [ ] 关键词搜索返回标题或正文匹配的帖子
- [ ] 标题匹配权重高于正文（搜索结果排序合理）
- [ ] 支持按 type 和 category 筛选
- [ ] 强制 school_id 隔离（错误 school_id 返回空）
- [ ] 支持 3 种排序方式
- [ ] ES 查询响应时间 < 500ms（含 Redis 缓存 < 200ms）
- [ ] 单元测试覆盖率 > 80%
- [ ] 性能测试通过

---

## 依赖关系

- **被阻塞**: 
  - #009 ES 异步同步（ES 中必须有数据）
  - #011 Protobuf 接口定义
- **阻塞**: 无

---

## 备注

- 严格遵循 PRD "Story 3：搜索和筛选帖子" + "功能 5：内容搜索" 定义
- `school_id` 必须在 filter 中（不是 must），利用 ES filter cache 提升性能
- 敏感字段（联系方式）已在 #009 中过滤，不写入 ES
- 模糊匹配使用 `fuzziness: AUTO`，避免误匹配
- 高亮功能建议开启，小程序前端展示更友好