# Issue #004: DFA 敏感词过滤

> **Label**: `epic:content-service` `P1` `feature` `security`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 2-3 天

---

## 任务描述

实现 DFA（Deterministic Finite Automaton，确定有限自动机）敏感词过滤器。在发帖时（写入数据库前）扫描标题和正文，命中敏感词则拒绝创建并返回敏感词位置。词库存储于 Redis 支持热更新。

---

## 技术方案

### 算法原理

**DFA 多模式匹配算法**：
- 构建敏感词 Trie 树（前缀树）
- 一次扫描输入文本，O(n) 时间复杂度匹配所有敏感词
- 支持中英文混合、最大匹配、跳词检测

### 词库管理

**Redis 数据结构**：
- `Key`: `dfa:sensitive_words`
- `Type**: Set（存储所有敏感词）
- 支持热更新：词库变更时无需重启服务

### 目录结构

```
pkg/sensitive/
├── dfa.go              # DFA 算法核心实现
├── word_manager.go     # 词库加载与热更新
└── dfa_test.go         # 单元测试

internal/content/service/
└── post_service.go     # 集成 DFA 扫描
```

### 核心代码

**1. DFA Trie 树节点（pkg/sensitive/dfa.go）：**

```go
// TrieNode DFA 树节点
type TrieNode struct {
    Children map[rune]*TrieNode
    IsEnd    bool         // 是否是敏感词结尾
    Word     string       // 完整敏感词（用于回显）
}

// DFA 敏感词匹配器
type DFAMatcher struct {
    root *TrieNode
    mu   sync.RWMutex
}

// Build 构建 Trie 树
func (d *DFAMatcher) Build(words []string) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.root = &TrieNode{Children: make(map[rune]*TrieNode)}
    for _, word := range words {
        d.insert(word)
    }
}

// Match 在文本中匹配所有敏感词，返回命中列表
func (d *DFAMatcher) Match(text string) []MatchResult {
    d.mu.RLock()
    defer d.mu.RUnlock()
    
    var results []MatchResult
    runes := []rune(text)
    
    for i := 0; i < len(runes); i++ {
        node := d.root
        for j := i; j < len(runes); j++ {
            child, ok := node.Children[runes[j]]
            if !ok { break }
            if child.IsEnd {
                results = append(results, MatchResult{
                    Word:   child.Word,
                    Start:  i,
                    End:    j + 1,
                    Length: j - i + 1,
                })
            }
            node = child
        }
    }
    return results
}

type MatchResult struct {
    Word   string `json:"word"`     // 命中的敏感词
    Start  int    `json:"start"`    // 起始位置（字符索引）
    End    int    `json:"end"`      // 结束位置
    Length int    `json:"length"`   // 长度
}
```

**2. 词库热更新（pkg/sensitive/word_manager.go）：**

```go
// LoadWords 从 Redis 加载敏感词库
func (m *WordManager) LoadWords(ctx context.Context) error {
    words, err := m.redis.SMembers(ctx, "dfa:sensitive_words").Result()
    if err != nil {
        return fmt.Errorf("加载敏感词失败: %w", err)
    }
    m.matcher.Build(words)
    log.Printf("[DFA] 加载敏感词 %d 个", len(words))
    return nil
}

// WatchUpdates 监听词库变更（Redis Keyspace Notifications）
func (m *WordManager) WatchUpdates(ctx context.Context) {
    pubsub := m.redis.PSubscribe(ctx, "__keyspace@0__:dfa:sensitive_words")
    for msg := range pubsub.Channel() {
        log.Printf("[DFA] 检测到词库变更: %s", msg.Payload)
        if err := m.LoadWords(ctx); err != nil {
            log.Printf("[DFA] 重载词库失败: %v", err)
        }
    }
}
```

**3. Service 集成（internal/content/service/post_service.go）：**

```go
// CreatePost 创建帖子（含 DFA 扫描）
func (s *PostService) CreatePost(ctx context.Context, req *CreatePostRequest) (*Post, error) {
    // 1. DFA 敏感词扫描
    fullText := req.Title + "\n" + req.Content
    hits := s.dfaMatcher.Match(fullText)
    if len(hits) > 0 {
        return nil, &ErrSensitiveWords{
            Message: "内容包含敏感词",
            Hits:    hits,  // 返回敏感词列表及位置
        }
    }
    
    // 2. 正常创建流程...
}
```

### 错误响应格式

```json
{
  "code": 400,
  "message": "内容包含敏感词",
  "data": {
    "hits": [
      {"word": "敏感词A", "start": 12, "end": 15, "length": 3},
      {"word": "敏感词B", "start": 30, "end": 33, "length": 3}
    ]
  }
}
```

---

## 检查清单

- [ ] 实现 DFA Trie 树构建算法
- [ ] 实现 `Match` 多模式匹配算法
- [ ] 实现 Redis 词库加载（SMembers）
- [ ] 实现 Redis Keyspace Notifications 监听
- [ ] 编写初始化脚本，导入示例敏感词到 Redis
- [ ] 在 `PostService.CreatePost` 集成 DFA 扫描
- [ ] 实现 `ErrSensitiveWords` 错误类型（含命中详情）
- [ ] 实现管理员手动触发词库重载接口
- [ ] 编写单元测试（性能、正确性、边界）
- [ ] 性能基准测试（10000 字文本扫描 < 10ms）

---

## 验收标准

- [ ] 命中敏感词时拒绝创建帖子，返回 400
- [ ] 响应体包含所有命中的敏感词及位置
- [ ] 未命中敏感词时正常进入 pending 状态
- [ ] 词库变更后无需重启服务即可生效（热更新）
- [ ] 10000 字文本扫描耗时 < 10ms
- [ ] 单元测试覆盖率 > 90%
- [ ] 性能基准测试通过

---

## 依赖关系

- **被阻塞**: #011 Protobuf 接口定义
- **阻塞**: #005 内容审核流程

---

## 备注

- 严格遵循 PRD "功能 6：DFA 敏感词过滤" 定义
- DFA 算法时间复杂度 O(n)，适合高并发场景
- 敏感词词库建议初始导入 1000+ 常见词
- Redis Keyspace Notifications 需要在 redis.conf 中配置 `notify-keyspace-events Egx`