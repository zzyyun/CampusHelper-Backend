# Issue #002: 失物招领模板

> **Label**: `epic:content-service` `P2` `feature`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 2 天

---

## 任务描述

在通用帖子基础上实现**失物招领**业务模板，扩展字段（地点、物品分类、联系方式、丢失/拾取标记），添加特有状态"已当领"(`retrieved`)，实现 30 天自动过期规则。

---

## 技术方案

### 数据模型扩展

**MySQL 表（lost_found_posts）：**

```sql
CREATE TABLE lost_found_posts (
    post_id BIGINT PRIMARY KEY COMMENT '关联 posts.id',
    location VARCHAR(200) NOT NULL COMMENT '丢失/拾取地点文字描述',
    item_category TINYINT NOT NULL COMMENT '物品分类(枚举)',
    contact VARCHAR(100) NOT NULL COMMENT '联系方式(手机/微信)',
    lost_or_found TINYINT NOT NULL COMMENT '1=丢失 2=拾到',
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    INDEX idx_post (post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='失物招领扩展表';
```

### 扩展字段（Protobuf）

```protobuf
message LostFoundExtra {
  string location = 1;           // 丢失/拾取地点
  ItemCategory item_category = 2;
  string contact = 3;            // 手机/微信
  LostOrFoundType lost_or_found = 4;
}

enum LostOrFoundType {
  LOST_OR_FOUND_UNSPECIFIED = 0;
  LOST = 1;    // 我丢失了
  FOUND = 2;   // 我拾到了
}
```

### 业务规则

1. **创建校验**：
   - `location` 必填，长度 ≤ 200
   - `contact` 必填，必须是手机号或微信号格式
   - `lost_or_found` 必须为 `LOST` 或 `FOUND`

2. **状态机扩展**：
   - 在通用帖子状态基础上，新增 `RETRIEVED`（已当领）
   - 用户可手动将 `PUBLISHED → RETRIEVED`

3. **过期规则**：
   - 发布后 30 天自动过期
   - 过期前 3 天通过 Message Service 提醒用户
   - 由 RabbitMQ 延迟队列实现（Phase 2）

### 核心代码（service 层示例）

```go
// CreateLostFoundPost 创建失物招领帖
func (s *PostService) CreateLostFoundPost(ctx context.Context, req *CreatePostRequest) (*Post, error) {
    // 1. 校验失物招领特有字段
    if req.LostFound.Location == "" {
        return nil, errors.New("地点不能为空")
    }
    if !validateContact(req.LostFound.Contact) {
        return nil, errors.New("联系方式格式错误")
    }
    
    // 2. DFA 敏感词扫描（#004）
    if hits := s.dfaScan(req.Title + req.Content); len(hits) > 0 {
        return nil, ErrSensitiveWord
    }
    
    // 3. 创建帖子（status=pending）
    post := &model.Post{
        SchoolID: req.SchoolID,
        UserID:   req.UserID,
        Type:     PostTypeLostFound,
        Title:    req.Title,
        Content:  req.Content,
        Images:   req.Images,
        Status:   PostStatusPending,
        ExpiredAt: ptr(time.Now().Add(30 * 24 * time.Hour)),
    }
    
    // 4. 保存扩展字段
    return s.repo.CreateLostFound(ctx, post, req.LostFound)
}
```

---

## 检查清单

- [ ] 创建 `lost_found_posts` 表
- [ ] 在 Protobuf 中添加 `LostFoundExtra` 消息和 `LostOrFoundType` 枚举
- [ ] 重新生成 `.pb.go` 代码
- [ ] 实现 `model.LostFound` 模型
- [ ] 实现 `repo.CreateLostFound` / `GetLostFoundByPostID`
- [ ] 实现 `service.CreateLostFoundPost` 业务逻辑
- [ ] 实现特有状态 `RETRIEVED` 的流转
- [ ] 设置 `expired_at = now() + 30 days`
- [ ] 编写单元测试（覆盖字段校验、状态流转）

---

## 验收标准

- [ ] 可以成功创建失物招领帖（含特有字段）
- [ ] 字段缺失或格式错误返回明确错误信息
- [ ] 过期时间自动设置为 30 天后
- [ ] 状态可流转到 `RETRIEVED`（已当领）
- [ ] 失物招领帖可被通用帖子列表查询（type 过滤）
- [ ] 单元测试覆盖核心场景

---

## 依赖关系

- **被阻塞**: 
  - #011 Protobuf 接口定义
  - #001 通用帖子基础层
- **阻塞**: 无

---

## 备注

- 严格遵循 PRD "功能 2：失物招领模板" 定义
- 联系人信息（手机/微信）属于敏感数据，**禁止在 ES 中索引**
- 30 天过期由 RabbitMQ 延迟队列实现，详见 Phase 2 计划