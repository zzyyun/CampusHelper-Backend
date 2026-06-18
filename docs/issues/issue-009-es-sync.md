# Issue #009: ES 异步同步

> **Label**: `epic:content-service` `P1` `feature` `infra`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 3-4 天

---

## 任务描述

实现 Elasticsearch 8 异步同步机制。当帖子状态变为 `published` 时，通过 RabbitMQ 发送 `content.published` 事件，消费者异步写入 ES 索引。保证审核通过后 < 5s 可被搜索到。

---

## 技术方案

### 架构流程

```
Content Service ──(审核通过)──→ RabbitMQ(content.events)
                                        ↓
                              ES Sync Consumer
                                        ↓
                                  Elasticsearch Index
```

### ES 索引设计

**索引名**：`posts_v1`（按版本号管理，便于索引重建）

**Mapping（IK 分词器）：**

```json
{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "analysis": {
      "analyzer": {
        "ik_smart_pinyin": {
          "type": "custom",
          "tokenizer": "ik_smart"
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "id":            { "type": "long" },
      "school_id":     { "type": "long" },
      "user_id":       { "type": "long" },
      "type":          { "type": "integer" },
      "title":         { "type": "text", "analyzer": "ik_max_word", "search_analyzer": "ik_smart" },
      "content":       { "type": "text", "analyzer": "ik_max_word", "search_analyzer": "ik_smart" },
      "status":        { "type": "integer" },
      "item_category": { "type": "integer" },
      "price":         { "type": "double" },
      "condition":     { "type": "integer" },
      "location":      { "type": "text", "analyzer": "ik_smart" },
      "lost_or_found": { "type": "integer" },
      "likes_count":   { "type": "integer" },
      "comment_count": { "type": "integer" },
      "created_at":    { "type": "date" },
      "updated_at":    { "type": "date" }
    }
  }
}
```

**关键点：**
- `school_id` 字段用于多租户过滤
- 失物招领的 `contact`（联系方式）**禁止索引**，保护隐私
- `title` 和 `content` 使用 IK 分词器（中文友好）

### MQ 事件消费

**消费者（cmd/consumer/main.go 或 internal/content/consumer/es_sync.go）：**

```go
// ESSyncConsumer ES 同步消费者
type ESSyncConsumer struct {
    esClient *elastic.Client
    rabbit   *mq.Consumer
}

// HandleContentPublished 处理 content.published 事件
func (c *ESSyncConsumer) HandleContentPublished(ctx context.Context, event *ContentEvent) error {
    // 1. 从 MQ 头提取 TraceID（保证链路追踪）
    traceID := event.TraceID
    ctx = trace.ContextWithSpanContext(ctx, ...)
    
    // 2. 查询完整帖子数据（含扩展字段）
    post, err := c.postRepo.GetPostFull(ctx, event.SchoolID, event.PostID)
    if err != nil {
        return fmt.Errorf("查询帖子失败: %w", err)
    }
    
    // 3. 转换为 ES 文档
    doc := postToESDoc(post)
    
    // 4. 写入 ES（upsert）
    req := esapi.IndexRequest{
        Index:      "posts_v1",
        DocumentID: strconv.FormatInt(post.ID, 10),
        Body:       bytes.NewReader(mustJSON(doc)),
        Refresh:    "false",  // 异步刷新，提升吞吐
    }
    res, err := req.Do(ctx, c.esClient)
    if err != nil {
        return fmt.Errorf("ES 写入失败: %w", err)
    }
    defer res.Body.Close()
    
    if res.IsError() {
        return fmt.Errorf("ES 返回错误: %s", res.String())
    }
    
    log.Printf("[ES Sync] 帖子 %d 已同步 (school=%d, trace=%s)", post.ID, post.SchoolID, traceID)
    return nil
}

// HandleContentDeleted 处理帖子删除（下架）
func (c *ESSyncConsumer) HandleContentDeleted(ctx context.Context, event *ContentEvent) error {
    req := esapi.DeleteRequest{
        Index:      "posts_v1",
        DocumentID: strconv.FormatInt(event.PostID, 10),
    }
    res, err := req.Do(ctx, c.esClient)
    if err != nil { return err }
    defer res.Body.Close()
    return nil
}

// postToESDoc 转换为 ES 文档（敏感字段过滤）
func postToESDoc(post *model.Post) map[string]interface{} {
    doc := map[string]interface{}{
        "id":            post.ID,
        "school_id":     post.SchoolID,
        "user_id":       post.UserID,
        "type":          int(post.Type),
        "title":         post.Title,
        "content":       post.Content,
        "status":        int(post.Status),
        "likes_count":   post.LikesCount,
        "comment_count": post.CommentCount,
        "created_at":    post.CreatedAt,
        "updated_at":    post.UpdatedAt,
    }
    
    // 失物招领扩展（不含 contact）
    if post.LostFound != nil {
        doc["location"]      = post.LostFound.Location
        doc["item_category"] = int(post.LostFound.ItemCategory)
        doc["lost_or_found"] = int(post.LostFound.LostOrFound)
    }
    
    // 二手交易扩展
    if post.SecondHand != nil {
        doc["price"]         = post.SecondHand.Price
        doc["condition"]     = int(post.SecondHand.Condition)
        doc["item_category"] = int(post.SecondHand.ItemCategory)
    }
    
    return doc
}
```

### MQ 消费者注册

```go
// cmd/consumer/main.go
func main() {
    // 初始化 ES 客户端
    esClient := es.NewClient(viper.GetString("es.url"))
    
    // 初始化 RabbitMQ 消费者
    rabbit := mq.NewConsumer("content.events")
    
    // 注册事件处理器
    consumer := &ESSyncConsumer{
        esClient: esClient,
        rabbit:   rabbit,
    }
    
    rabbit.On("content.published", consumer.HandleContentPublished)
    rabbit.On("content.deleted", consumer.HandleContentDeleted)
    
    // 启动消费
    rabbit.Start()
    
    // 优雅退出
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    log.Println("[Consumer] 收到退出信号，正在关闭...")
    rabbit.Stop()
}
```

### 错误处理与重试

```go
// MQ 消息处理失败时的重试策略
func (c *ESSyncConsumer) processWithRetry(ctx context.Context, handler func() error) error {
    maxRetries := 3
    backoff := []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second}
    
    for i := 0; i < maxRetries; i++ {
        err := handler()
        if err == nil { return nil }
        
        log.Printf("[ES Sync] 第 %d 次重试: %v", i+1, err)
        time.Sleep(backoff[i])
    }
    
    // 重试耗尽，进入死信队列
    return ErrRetryExhausted
}
```

---

## 检查清单

- [ ] 创建 ES 索引 `posts_v1`（含 IK 分词器 Mapping）
- [ ] 实现 `pkg/es/client.go` ES 客户端封装
- [ ] 实现 `internal/content/consumer/es_sync.go` 消费者
- [ ] 实现 `postToESDoc` 转换函数（敏感字段过滤）
- [ ] 实现 MQ 消息消费（含 TraceID 提取）
- [ ] 实现错误重试机制（指数退避）
- [ ] 实现死信队列（DLQ）
- [ ] 实现 ES 索引重建脚本（用于数据修复）
- [ ] 编写集成测试（MQ → ES 端到端）
- [ ] 部署消费者服务到 `cmd/consumer/`

---

## 验收标准

- [ ] 帖子审核通过后 < 5s 可在 ES 中搜索到
- [ ] ES 文档包含完整字段（敏感字段 `contact` 被过滤）
- [ ] MQ 消息失败时自动重试 3 次
- [ ] 重试耗尽的消息进入死信队列
- [ ] 帖子删除（下架）时 ES 文档同步删除
- [ ] TraceID 贯穿 HTTP → gRPC → MQ → Consumer 全程
- [ ] 集成测试通过：审核通过 → ES 同步成功

---

## 依赖关系

- **被阻塞**: 
  - #005 内容审核流程
  - #011 Protobuf 接口定义
- **阻塞**: 
  - #010 内容搜索（依赖 ES 数据）

---

## 备注

- 严格遵循 PRD "功能 5：内容搜索" + "附录：RabbitMQ 事件规范" 定义
- **敏感字段（contact）禁止写入 ES**，避免隐私泄露
- 消费者是独立部署的服务（`cmd/consumer/`），与 Content Service 解耦
- ES 索引使用 IK 分词器，需要提前在 ES 集群中安装
- 死信队列配置：`content.events.dlq`