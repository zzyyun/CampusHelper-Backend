# Issue #003: 二手交易模板

> **Label**: `epic:content-service` `P2` `feature`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 2 天

---

## 任务描述

在通用帖子基础上实现**二手交易**业务模板，扩展字段（价格、原价、成色、交易方式、商品类别），添加特有状态"已售出"(`sold`)，实现 60 天自动过期规则。

---

## 技术方案

### 数据模型扩展

**MySQL 表（second_hand_posts）：**

```sql
CREATE TABLE second_hand_posts (
    post_id BIGINT PRIMARY KEY COMMENT '关联 posts.id',
    price DECIMAL(10,2) NOT NULL COMMENT '期望售价(元)',
    original_price DECIMAL(10,2) NULL COMMENT '原价(元,可选)',
    `condition` TINYINT NOT NULL COMMENT '成色: 1=全新 2=几乎全新 3=良好 4=一般',
    trade_method TINYINT NOT NULL COMMENT '交易方式: 1=面交 2=快递',
    item_category TINYINT NOT NULL COMMENT '物品分类',
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    INDEX idx_post (post_id),
    INDEX idx_price (price)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='二手交易扩展表';
```

### 扩展字段（Protobuf）

```protobuf
message SecondHandExtra {
  double price = 1;              // 期望售价
  double original_price = 2;     // 原价（可选）
  Condition condition = 3;       // 成色
  TradeMethod trade_method = 4;  // 交易方式
  ItemCategory item_category = 5;
}

enum TradeMethod {
  TRADE_METHOD_UNSPECIFIED = 0;
  FACE_TO_FACE = 1;   // 面交
  DELIVERY = 2;       // 快递
}
```

### 业务规则

1. **创建校验**：
   - `price > 0`，最多两位小数
   - `original_price >= price`（如有）
   - `condition` 必须为合法枚举值
   - `trade_method` 必须为 `FACE_TO_FACE` 或 `DELIVERY`

2. **状态机扩展**：
   - 在通用帖子状态基础上，新增 `SOLD`（已售出）
   - 用户可手动将 `PUBLISHED → SOLD`

3. **过期规则**：
   - 发布后 60 天自动过期
   - 支持用户手动续期（每次 +30 天，最多 3 次）

4. **平台规则**：
   - **平台不介入资金流转**，仅提供信息展示
   - 无担保交易、无意向金

### 核心代码示例

```go
// CreateSecondHandPost 创建二手交易帖
func (s *PostService) CreateSecondHandPost(ctx context.Context, req *CreatePostRequest) (*Post, error) {
    extra := req.SecondHand
    
    // 价格校验
    if extra.Price <= 0 {
        return nil, errors.New("价格必须大于0")
    }
    if extra.OriginalPrice > 0 && extra.OriginalPrice < extra.Price {
        return nil, errors.New("原价不能低于售价")
    }
    
    // 创建帖子（status=pending, expired_at = now + 60 days）
    post := &model.Post{
        SchoolID:  req.SchoolID,
        UserID:    req.UserID,
        Type:      PostTypeSecondHand,
        Title:     req.Title,
        Content:   req.Content,
        Images:    req.Images,
        Status:    PostStatusPending,
        ExpiredAt: ptr(time.Now().Add(60 * 24 * time.Hour)),
    }
    
    return s.repo.CreateSecondHand(ctx, post, extra)
}
```

---

## 检查清单

- [ ] 创建 `second_hand_posts` 表
- [ ] 在 Protobuf 中添加 `SecondHandExtra` 消息和 `TradeMethod` 枚举
- [ ] 重新生成 `.pb.go` 代码
- [ ] 实现 `model.SecondHand` 模型
- [ ] 实现 `repo.CreateSecondHand` / `GetSecondHandByPostID`
- [ ] 实现 `service.CreateSecondHandPost` 业务逻辑
- [ ] 实现特有状态 `SOLD` 的流转
- [ ] 设置 `expired_at = now() + 60 days`
- [ ] 实现手动续期接口（每次 +30 天）
- [ ] 编写单元测试

---

## 验收标准

- [ ] 可以成功创建二手交易帖（含特有字段）
- [ ] 价格校验有效（负数、原价低于售价被拒绝）
- [ ] 过期时间自动设置为 60 天后
- [ ] 状态可流转到 `SOLD`（已售出）
- [ ] 手动续期功能正常工作（最多 3 次）
- [ ] 单元测试覆盖核心场景

---

## 依赖关系

- **被阻塞**: 
  - #011 Protobuf 接口定义
  - #001 通用帖子基础层
- **阻塞**: 无

---

## 备注

- 严格遵循 PRD "功能 3：二手交易模板" 定义
- 平台**不介入资金流转**，仅做信息撮合
- 价格字段使用 `DECIMAL(10,2)` 保证精度
- 续期次数限制可在 `users` 表中加字段 `extension_count`（或单独记录表）