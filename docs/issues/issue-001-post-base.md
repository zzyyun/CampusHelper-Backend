# Issue #001: 通用帖子基础层

> **Label**: `epic:content-service` `P0` `feature`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 3-4 天

---

## 任务描述

实现通用帖子基础层（Post Base Layer），这是 Content Service 的核心数据模型。包括帖子表设计、CRUD 操作、状态机、学校隔离 Scope。所有业务模板（失物招领、二手交易）都基于此层扩展。

---

## 技术方案

### 数据模型设计

**MySQL 表结构（posts）：**

```sql
CREATE TABLE posts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    school_id BIGINT NOT NULL COMMENT '学校ID,多租户隔离键',
    user_id BIGINT NOT NULL COMMENT '发帖用户ID',
    type TINYINT NOT NULL COMMENT '帖子类型: 1=通用 2=失物招领 3=二手',
    title VARCHAR(200) NOT NULL COMMENT '标题',
    content TEXT NOT NULL COMMENT '正文',
    images JSON COMMENT '图片URL数组',
    status TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=审核中 2=已发布 3=已过期 4=已关闭 5=已拒绝',
    likes_count INT NOT NULL DEFAULT 0 COMMENT '点赞数',
    comment_count INT NOT NULL DEFAULT 0 COMMENT '评论数',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    expired_at TIMESTAMP NULL COMMENT '过期时间',
    INDEX idx_school_status_created (school_id, status, created_at DESC),
    INDEX idx_school_type_status (school_id, type, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子表';
```

### 目录结构

```
internal/content/
├── model/
│   └── post.go              # Post 数据模型
├── repo/
│   └── post_repo.go         # 数据访问层（含 SchoolScope）
├── service/
│   └── post_service.go      # 业务逻辑
└── handler/
    └── content_handler.go   # gRPC 接口实现
```

### 核心代码

**1. GORM Model（model/post.go）：**

```go
// Post 通用帖子数据模型
type Post struct {
    ID          int64     `gorm:"primaryKey" json:"id"`
    SchoolID    int64     `gorm:"not null;index" json:"school_id"`
    UserID      int64     `gorm:"not null;index" json:"user_id"`
    Type        PostType  `gorm:"not null" json:"type"`
    Title       string    `gorm:"size:200;not null" json:"title"`
    Content     string    `gorm:"type:text;not null" json:"content"`
    Images      StringArray `gorm:"type:json" json:"images"`
    Status      PostStatus `gorm:"not null;default:1" json:"status"`
    LikesCount  int32     `gorm:"default:0" json:"likes_count"`
    CommentCount int32    `gorm:"default:0" json:"comment_count"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    ExpiredAt   *time.Time `json:"expired_at,omitempty"`
}

// TableName 指定表名
func (Post) TableName() string { return "posts" }
```

**2. SchoolScope（pkg/db/scope.go）：**

```go
// SchoolScope 自动注入 school_id 隔离条件
func SchoolScope(schoolID int64) func(db *gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("school_id = ?", schoolID)
    }
}
```

**3. Repo 层（repo/post_repo.go）：**

```go
// CreatePost 创建帖子（自动应用 SchoolScope）
func (r *PostRepo) CreatePost(ctx context.Context, post *model.Post) error {
    return r.db.WithContext(ctx).Create(post).Error
}

// GetPost 根据ID和school_id获取帖子（强制隔离）
func (r *PostRepo) GetPost(ctx context.Context, schoolID, postID int64) (*model.Post, error) {
    var post model.Post
    err := r.db.WithContext(ctx).
        Scopes(SchoolScope(schoolID)).
        First(&post, postID).Error
    if err != nil { return nil, err }
    return &post, nil
}
```

### 状态机

```
pending (1) ──→ published (2) ──→ expired (3)
   │                │
   │                └──→ closed (4)
   └──→ rejected (5)
```

**状态流转规则：**
- `pending → published`：审核通过（#005）
- `pending → rejected`：审核拒绝（#005）
- `published → expired`：超过过期时间
- `published → closed`：用户主动关闭
- `published → retrieved`：失物已当领（#002 扩展）
- `published → sold`：二手已售出（#003 扩展）

---

## 检查清单

- [ ] 创建 `posts` 表（包含所有索引）
- [ ] 实现 `model.Post` GORM 模型
- [ ] 实现 `repo.PostRepo`（含 `SchoolScope` 强制注入）
- [ ] 实现 `service.PostService` 业务逻辑
- [ ] 实现 `CreatePost` / `GetPost` / `UpdatePost` / `DeletePost` gRPC 接口
- [ ] 实现状态机校验（非法状态流转返回错误）
- [ ] 实现单元测试（覆盖率 > 80%）
- [ ] 所有查询强制携带 school_id
- [ ] 所有代码注释使用简体中文

---

## 验收标准

- [ ] 可以成功创建/查询/更新/删除帖子
- [ ] 不携带 school_id 的查询会被 SchoolScope 自动过滤
- [ ] 跨学校查询（school_id 错误）返回空结果而非错误
- [ ] 状态机非法流转返回明确错误信息
- [ ] 单元测试覆盖核心场景
- [ ] `go test ./internal/content/...` 全部通过
- [ ] gRPC 接口在 Postman/grpcui 中可正常调用

---

## 依赖关系

- **被阻塞**: #011 Protobuf 接口定义
- **阻塞**: 
  - #002 失物招领模板
  - #003 二手交易模板
  - #006 帖子列表 + 游标分页
  - #007 一级评论系统

---

## 备注

- 严格遵循 PRD "功能 1：通用帖子基础层" 的定义
- 所有查询必须携带 school_id，这是项目的**强制安全约束**
- 图片 URL 由 File Service 返回（详见 #009 中提到的集成）
- 单元测试使用 SQLite in-memory 或 testcontainers MySQL