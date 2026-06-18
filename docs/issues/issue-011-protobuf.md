# Issue #011: Protobuf 接口定义

> **Label**: `epic:content-service` `P0` `infra`  
> **所属 Epic**: [epic-content-service](epic-content-service.md)  
> **状态**: 待开发  
> **预估工期**: 1-2 天

---

## 任务描述

定义 Content Service 的 gRPC 服务接口（Protobuf），包括所有 RPC 方法、消息类型、枚举定义。这是整个 Content Service 的**起点任务**，所有后续开发都依赖此接口。

---

## 技术方案

### 文件位置

- `PB/content.proto` — Content Service 接口定义
- `PB/common.proto` — 公共消息类型（如 `Pagination`、`SchoolContext`）

### 必须实现的 RPC 方法

**帖子 CRUD（供 Gateway 调用）：**
- `CreatePost(CreatePostRequest) returns (CreatePostResponse)`
- `GetPost(GetPostRequest) returns (GetPostResponse)`
- `UpdatePost(UpdatePostRequest) returns (UpdatePostResponse)`
- `DeletePost(DeletePostRequest) returns (DeletePostResponse)`
- `ListPosts(ListPostsRequest) returns (ListPostsResponse)` — 游标分页

**评论与点赞（供 Gateway 调用）：**
- `CreateComment(CreateCommentRequest) returns (CreateCommentResponse)`
- `DeleteComment(DeleteCommentRequest) returns (DeleteCommentResponse)`
- `LikePost(LikePostRequest) returns (LikePostResponse)`
- `UnlikePost(UnlikePostRequest) returns (UnlikePostResponse)`

**内容搜索（代理 ES 查询，供 Gateway 调用）：**
- `SearchContent(SearchContentRequest) returns (SearchContentResponse)`

**审核操作（供 Admin Service 调用）：**
- `ApprovePost(ApprovePostRequest) returns (ApprovePostResponse)`
- `RejectPost(RejectPostRequest) returns (RejectPostResponse)`
- `TakedownPost(TakedownPostRequest) returns (TakedownPostResponse)`

### 核心消息类型

```protobuf
// 帖子类型枚举
enum PostType {
  POST_TYPE_UNSPECIFIED = 0;
  POST_TYPE_GENERAL = 1;        // 通用
  POST_TYPE_LOST_FOUND = 2;     // 失物招领
  POST_TYPE_SECOND_HAND = 3;    // 二手交易
}

// 帖子状态枚举
enum PostStatus {
  POST_STATUS_UNSPECIFIED = 0;
  POST_STATUS_PENDING = 1;      // 审核中
  POST_STATUS_PUBLISHED = 2;    // 已发布
  POST_STATUS_EXPIRED = 3;      // 已过期
  POST_STATUS_CLOSED = 4;       // 已关闭
  POST_STATUS_REJECTED = 5;     // 已拒绝
  POST_STATUS_RETRIEVED = 6;    // 失物已当领
  POST_STATUS_SOLD = 7;         // 二手已售出
}

// 物品分类枚举
enum ItemCategory {
  ITEM_CATEGORY_UNSPECIFIED = 0;
  ITEM_CATEGORY_DIGITAL = 1;       // 手机/数码
  ITEM_CATEGORY_CERTIFICATE = 2;   // 证件/卡类
  ITEM_CATEGORY_KEY = 3;           // 钥匙/门卡
  ITEM_CATEGORY_BOOK = 4;          // 书籍/资料
  ITEM_CATEGORY_CLOTHING = 5;      // 服装/饰品
  ITEM_CATEGORY_CHARGER = 6;       // 充电器/配件
  ITEM_CATEGORY_DAILY = 7;         // 生活用品
  ITEM_CATEGORY_OTHER = 8;         // 其他
}

// 成色枚举（二手交易专用）
enum Condition {
  CONDITION_UNSPECIFIED = 0;
  CONDITION_BRAND_NEW = 1;     // 全新
  CONDITION_LIKE_NEW = 2;      // 几乎全新
  CONDITION_GOOD = 3;          // 良好
  CONDITION_FAIR = 4;          // 一般
}

// 帖子消息
message Post {
  int64 id = 1;
  int64 school_id = 2;
  int64 user_id = 3;
  PostType type = 4;
  string title = 5;
  string content = 6;
  repeated string images = 7;
  PostStatus status = 8;
  int32 likes_count = 9;
  int32 comment_count = 10;
  int64 created_at = 11;
  int64 expired_at = 12;
  
  // 失物招领扩展字段
  LostFoundExtra lost_found = 20;
  
  // 二手交易扩展字段
  SecondHandExtra second_hand = 21;
}
```

### 游标分页结构

```protobuf
message CursorPagination {
  string cursor = 1;       // 上次响应最后一条记录的 ID
  int32 page_size = 2;    // 每页条数（默认 20，最大 100）
}

message ListPostsRequest {
  int64 school_id = 1;    // 强制注入
  PostType type = 2;      // 可选筛选
  ItemCategory category = 3;
  PostStatus status = 4;
  CursorPagination pagination = 5;
  SortType sort = 6;       // time_desc / likes_desc
}
```

---

## 检查清单

- [ ] 创建 `PB/content.proto` 文件
- [ ] 定义 `ContentService` 服务及所有 14 个 RPC 方法
- [ ] 定义所有枚举（PostType、PostStatus、ItemCategory、Condition 等）
- [ ] 定义 `Post`、`LostFoundExtra`、`SecondHandExtra` 等消息类型
- [ ] 定义 `CursorPagination` 通用分页结构
- [ ] 定义 `SchoolContext` 公共上下文类型
- [ ] 在 `PB/common.proto` 中提取公共类型
- [ ] 配置 `Makefile` 或脚本生成 `.pb.go` 和 `_grpc.pb.go`
- [ ] 提交并验证生成代码无编译错误

---

## 验收标准

- [ ] `protoc` 命令可成功生成 Go 代码
- [ ] 所有 14 个 RPC 方法在生成的接口中可见
- [ ] 枚举值与 PRD 定义一致
- [ ] 消息字段命名遵循 Google Protobuf 风格（下划线）
- [ ] 生成代码通过 `go build ./...`
- [ ] 文件包含必要的中文注释

---

## 依赖关系

- **被阻塞**: 无（起点任务）
- **阻塞**: 
  - #001 通用帖子基础层
  - #004 DFA 敏感词过滤
  - #005 内容审核流程
  - #006 帖子列表 + 游标分页
  - #007 一级评论系统
  - #008 点赞功能
  - #009 ES 异步同步
  - #010 内容搜索

---

## 备注

- 严格遵循 PRD 第 221-244 行的 gRPC 服务接口定义
- Protobuf 字段命名规范：使用 `snake_case`
- Go 代码生成工具：`protoc-gen-go` + `protoc-gen-go-grpc`
- 所有 Protobuf 注释必须使用简体中文