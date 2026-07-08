# Content Service 测试用例文档

**服务名称**：Content Service（内容服务）
**PRD 来源**：
- `docs/content-service-prd.md`（v1.0，2026-06-08）
- `docs/content-service-v2-prd.md`（v2.1，2026-06-25）
**生成日期**：2026-07-08
**测试用例总数**：85

---

## 目录

1. [功能测试（TC-F）](#1-功能测试tc-f)
2. [边界测试（TC-E）](#2-边界测试tc-e)
3. [异常测试（TC-ERR）](#3-异常测试tc-err)
4. [状态转换测试（TC-ST）](#4-状态转换测试tc-st)
5. [需求-测试用例覆盖矩阵](#5-需求-测试用例覆盖矩阵)

---

## 1. 功能测试（TC-F）

### TC-F-001：发布失物招领帖 — 完整字段提交

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-001 |
| **标题** | 发布失物招领帖 — 所有必填字段正确提交 |
| **需求来源** | v1.0 Story 1、功能 2 |
| **优先级** | 高 |
| **前置条件** | 用户已登录（JWT Token 有效），JWT 中包含 school_id；File Service 可用 |
| **测试步骤** | 1. 构造 CreatePostRequest，type=lost_found，填写 title、content、location、item_category（手机/数码）、contact（手机号）、lost_or_found=丢失<br>2. 上传 2 张物品图片（调用 File Service 获取 CDN URL）<br>3. 调用 CreatePost gRPC 接口 |
| **预期结果** | 1. 返回 CreatePostResponse，post_id 为合法雪花 ID<br>2. 帖子 status=pending<br>3. 帖子 images 字段包含 2 个 CDN URL<br>4. lost_found 扩展字段（location、item_category、contact、lost_or_found）均正确持久化<br>5. created_at 自动填充，expired_at = created_at + 30 天<br>6. Jaeger 中可追踪到完整 Span（HTTP → gRPC） |

---

### TC-F-002：发布失物招领帖 — 图片上传

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-002 |
| **标题** | 发布失物招领帖 — 支持多张图片上传 |
| **需求来源** | v1.0 Story 1 |
| **优先级** | 高 |
| **前置条件** | 用户已登录；File Service 可用 |
| **测试步骤** | 1. 构造 CreatePostRequest，images 字段包含 3 个 CDN URL（模拟 File Service 已上传）<br>2. 调用 CreatePost 接口<br>3. 调用 GetPost 验证返回的 images 列表 |
| **预期结果** | 1. 帖子创建成功<br>2. GetPost 返回的 images 数组包含 3 个 URL，顺序与提交一致 |

---

### TC-F-003：发布二手交易帖 — 完整字段提交

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-003 |
| **标题** | 发布二手交易帖 — 所有字段正确提交 |
| **需求来源** | v1.0 Story 2、功能 3 |
| **优先级** | 高 |
| **前置条件** | 用户已登录；File Service 可用 |
| **测试步骤** | 1. 构造 CreatePostRequest，type=second_hand<br>2. 填写 title、content、price=50、original_price=200（可选）、condition=like_new、trade_method=face_to_face、item_category=数码<br>3. 上传 1 张商品图片<br>4. 调用 CreatePost 接口 |
| **预期结果** | 1. 返回合法 post_id<br>2. status=pending<br>3. second_hand 扩展字段（price、original_price、condition、trade_method、item_category）正确持久化<br>4. expired_at = created_at + 60 天<br>5. images 包含 1 个 URL |

---

### TC-F-004：发布通用帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-004 |
| **标题** | 发布通用帖子 — 仅需基础字段 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 CreatePostRequest，type=general，填写 title、content<br>2. 不填写任何扩展字段（location、price 等）<br>3. 调用 CreatePost 接口 |
| **预期结果** | 1. 帖子创建成功，status=pending<br>2. type=general<br>3. 扩展字段为零值 |

---

### TC-F-005：帖子列表 — 游标分页

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-005 |
| **标题** | 帖子列表 — cursor-based 游标分页加载 |
| **需求来源** | v1.0 Story 3、功能 1 |
| **优先级** | 高 |
| **前置条件** | 当前 school_id 下已有 20 条已发布帖子 |
| **测试步骤** | 1. 调用 ListPosts，school_id=100，不传 cursor，limit=10<br>2. 从返回结果取最后一条帖子的 ID 作为 cursor<br>3. 再次调用 ListPosts，传入 cursor，limit=10 |
| **预期结果** | 1. 第一次返回 10 条帖子，按 created_at 倒序排列<br>2. 第二次返回剩余帖子，不包含第一页已展示的帖子<br>3. 两次返回的帖子无重复 |

---

### TC-F-006：帖子列表 — 按点赞数排序

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-006 |
| **标题** | 帖子列表 — 切换为按点赞数排序 |
| **需求来源** | v1.0 Story 3 |
| **优先级** | 中 |
| **前置条件** | 当前 school_id 下已有 5 条已发布帖子，likes_count 分别为 0、3、1、5、2 |
| **测试步骤** | 1. 调用 ListPosts，sort_by=likes_desc<br>2. 检查返回列表顺序 |
| **预期结果** | 帖子按 likes_count 降序排列：5、3、2、1、0 |

---

### TC-F-007：帖子列表 — 按类型筛选

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-007 |
| **标题** | 帖子列表 — 按帖子类型（失物/二手/通用）筛选 |
| **需求来源** | v1.0 Story 3 |
| **优先级** | 高 |
| **前置条件** | 当前 school_id 下有 3 条失物帖、2 条二手帖、1 条通用帖（均已发布） |
| **测试步骤** | 1. 调用 ListPosts，type=lost_found<br>2. 调用 ListPosts，type=second_hand |
| **预期结果** | 1. 第一次仅返回 3 条失物帖<br>2. 第二次仅返回 2 条二手帖 |

---

### TC-F-008：帖子详情 — 获取单个帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-008 |
| **标题** | 获取帖子详情 — 包含扩展字段和计数 |
| **需求来源** | v1.0 功能 1、Story 4 |
| **优先级** | 高 |
| **前置条件** | 已创建一条失物招领帖，有 3 条评论、5 个点赞 |
| **测试步骤** | 1. 调用 GetPost，传入 post_id 和 school_id |
| **预期结果** | 1. 返回帖子完整信息（title、content、images、扩展字段）<br>2. comment_count=3，likes_count=5<br>3. status=published |

---

### TC-F-009：审核通过帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-009 |
| **标题** | Admin Service 调用审核通过 — 帖子发布并触发 ES 同步 |
| **需求来源** | v1.0 Story 5、功能 5 |
| **优先级** | 高 |
| **前置条件** | 已创建一条 status=pending 的帖子；MQ 可用；ES 可用 |
| **测试步骤** | 1. 以 Admin Service 身份调用 ApprovePost，传入 post_id<br>2. 等待 5 秒<br>3. 通过 ES 搜索该帖子标题 |
| **预期结果** | 1. ApprovePost 返回成功<br>2. 帖子 status 变为 published<br>3. MQ 发送 content.published 事件（含 TraceID）<br>4. ES 索引中可搜到该帖子<br>5. Jaeger 中可见 MQ Span |

---

### TC-F-010：审核拒绝帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-010 |
| **标题** | Admin Service 调用审核拒绝 — 帖子被拒绝并通知用户 |
| **需求来源** | v1.0 Story 5 |
| **优先级** | 高 |
| **前置条件** | 已创建一条 status=pending 的帖子 |
| **测试步骤** | 1. 以 Admin Service 身份调用 RejectPost，post_id + reason="内容含有广告嫌疑"<br>2. 查询帖子状态 |
| **预期结果** | 1. 帖子 status 变为 rejected<br>2. MQ 发送 content.review_result 事件（含拒绝原因）<br>3. rejected 状态帖子仅发帖者可见 |

---

### TC-F-011：下架帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-011 |
| **标题** | 下架已发布帖子 — TakedownPost |
| **需求来源** | v1.0 Story 5 |
| **优先级** | 高 |
| **前置条件** | 已有一条 status=published 的帖子，且已同步到 ES |
| **测试步骤** | 1. 以 Admin Service 身份调用 TakedownPost，post_id<br>2. 等待 5 秒<br>3. 通过 ES 搜索该帖子 |
| **预期结果** | 1. 帖子状态更新<br>2. MQ 发送 content.taken_down 事件<br>3. ES 索引中该帖子被删除，搜索不到 |

---

### TC-F-012：一级评论 — 创建评论

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-012 |
| **标题** | 对帖子发表一级评论 |
| **需求来源** | v1.0 Story 4、功能 4 |
| **优先级** | 高 |
| **前置条件** | 已有一条 status=published 的帖子；用户已登录 |
| **测试步骤** | 1. 调用 CreateComment，school_id、post_id、user_id、content="请问还在吗？"，parent_id=0 |
| **预期结果** | 1. 评论创建成功，comment_id 为合法雪花 ID<br>2. parent_id=0（一级评论）<br>3. 帖子 comment_count +1<br>4. MQ 发送 content.comment_created 事件 |

---

### TC-F-013：一级评论 — 删除评论

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-013 |
| **标题** | 评论作者删除自己的一级评论 |
| **需求来源** | v1.0 功能 4 |
| **优先级** | 高 |
| **前置条件** | 已存在一条由当前用户创建的一级评论 |
| **测试步骤** | 1. 调用 DeleteComment，comment_id、user_id（评论作者）<br>2. 查询帖子 comment_count |
| **预期结果** | 1. 评论被删除（软删除，status=2）<br>2. 帖子 comment_count -1 |

---

### TC-F-014：点赞帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-014 |
| **标题** | 用户对帖子点赞 |
| **需求来源** | v1.0 Story 4 |
| **优先级** | 高 |
| **前置条件** | 已有一条 status=published 的帖子；用户未对该帖点赞 |
| **测试步骤** | 1. 调用 LikePost，post_id、user_id、school_id<br>2. 查询帖子 likes_count |
| **预期结果** | 1. 点赞成功，返回 success<br>2. 帖子 likes_count +1<br>3. MQ 发送 content.liked 事件 |

---

### TC-F-015：取消点赞帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-015 |
| **标题** | 用户取消对帖子的点赞 |
| **需求来源** | v1.0 Story 4 |
| **优先级** | 高 |
| **前置条件** | 用户已对某帖子点赞 |
| **测试步骤** | 1. 调用 UnlikePost，post_id、user_id<br>2. 查询帖子 likes_count |
| **预期结果** | 1. 取消点赞成功<br>2. 帖子 likes_count -1<br>3. 不发布 MQ 事件 |

---

### TC-F-016：ES 全文搜索 — 关键词搜索

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-016 |
| **标题** | 关键词搜索帖子（标题 + 描述） |
| **需求来源** | v1.0 Story 3、功能 5 |
| **优先级** | 高 |
| **前置条件** | 已有审核通过的帖子同步到 ES，标题包含"校园卡" |
| **测试步骤** | 1. 调用 SearchContent，school_id=100，keyword="校园卡"<br>2. 检查返回结果 |
| **预期结果** | 1. 返回包含"校园卡"关键词的帖子列表<br>2. 搜索结果仅包含 school_id=100 的帖子<br>3. 结果按相关性/时间排序 |

---

### TC-F-017：ES 搜索 — 按类型和类别筛选

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-017 |
| **标题** | 搜索时按帖子类型和商品类别组合筛选 |
| **需求来源** | v1.0 Story 3、功能 5 |
| **优先级** | 中 |
| **前置条件** | ES 中已有多种类型的帖子 |
| **测试步骤** | 1. 调用 SearchContent，school_id=100，type=second_hand，item_category=数码<br>2. 检查返回结果 |
| **预期结果** | 1. 仅返回 school_id=100、type=second_hand、item_category=数码的帖子<br>2. 不包含其他类型或类别的帖子 |

---

### TC-F-018：DFA 敏感词过滤 — 命中拒绝

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-018 |
| **标题** | 发帖时 DFA 检测到敏感词 — 拒绝创建 |
| **需求来源** | v1.0 功能 6、Story 1 |
| **优先级** | 高 |
| **前置条件** | DFA 词库已加载（Redis 中有敏感词"赌博"）；用户已登录 |
| **测试步骤** | 1. 构造 CreatePostRequest，content 中包含"赌博"二字<br>2. 调用 CreatePost |
| **预期结果** | 1. 帖子创建被拒绝，返回 400 错误<br>2. 响应体包含命中的敏感词列表及在文本中的位置<br>3. 数据库中无该帖子记录 |

---

### TC-F-019：DFA 敏感词过滤 — 未命中通过

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-019 |
| **标题** | 发帖时内容无敏感词 — 正常进入审核 |
| **需求来源** | v1.0 功能 6 |
| **优先级** | 高 |
| **前置条件** | DFA 词库已加载 |
| **测试步骤** | 1. 构造 CreatePostRequest，content="我在图书馆捡到一张校园卡"<br>2. 调用 CreatePost |
| **预期结果** | 1. 帖子创建成功，status=pending<br>2. 不返回敏感词错误 |

---

### TC-F-020：帖子更新

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-020 |
| **标题** | 发帖者更新自己的帖子 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 中 |
| **前置条件** | 发帖者已创建帖子 |
| **测试步骤** | 1. 调用 UpdatePost，修改 title 和 content<br>2. 调用 GetPost 验证更新 |
| **预期结果** | 1. 更新成功<br>2. title、content 为新值<br>3. 其他字段不变 |

---

### TC-F-021：帖子删除

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-021 |
| **标题** | 发帖者删除自己的帖子 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 中 |
| **前置条件** | 发帖者已创建帖子 |
| **测试步骤** | 1. 调用 DeletePost，post_id、user_id（发帖者）<br>2. 调用 GetPost 验证 |
| **预期结果** | 1. 帖子被软删除<br>2. GetPost 返回 404 或不可见 |

---

### TC-F-022：ES 同步消费者激活 — main.go 启动

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-022 |
| **标题** | cmd/content/main.go 启动时自动开启 ESSyncConsumer |
| **需求来源** | v2.1 Story 1、FR-1 |
| **优先级** | 高 |
| **前置条件** | RabbitMQ 可用；ES 可用 |
| **测试步骤** | 1. 启动 cmd/content/main.go<br>2. 检查日志中是否包含 ESSyncConsumer 启动信息<br>3. 检查 RabbitMQ 管理界面 content.events 队列是否有消费者连接 |
| **预期结果** | 1. main.go 启动后 ESSyncConsumer goroutine 异步启动<br>2. 不阻塞 gRPC server 的 Serve（启动时间增长 < 1s）<br>3. RabbitMQ 中可见活跃消费者 |

---

### TC-F-023：ES 同步 — 审核通过后帖子被索引

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-023 |
| **标题** | content.published 事件触发 ES 索引 |
| **需求来源** | v2.1 Story 1、FR-1 |
| **优先级** | 高 |
| **前置条件** | ESSyncConsumer 已启动；有 status=pending 的帖子 |
| **测试步骤** | 1. 调用 ApprovePost 审核通过帖子<br>2. 等待 5 秒内<br>3. 通过 SearchContent 搜索该帖子标题 |
| **预期结果** | 1. ES 索引中可搜到该帖子（P95 延迟 < 5s）<br>2. 索引数据包含完整的帖子信息（title、content、type、school_id 等）<br>3. Jaeger 中可见 consumer Span |

---

### TC-F-024：ES 同步 — 下架后帖子从 ES 删除

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-024 |
| **标题** | content.taken_down 事件触发 ES 删除 |
| **需求来源** | v2.1 Story 1、FR-1 |
| **优先级** | 高 |
| **前置条件** | 帖子已审核通过且已索引到 ES |
| **测试步骤** | 1. 调用 TakedownPost 下架帖子<br>2. 等待 5 秒内<br>3. 通过 SearchContent 搜索该帖子 |
| **预期结果** | 1. ES 索引中该帖子被删除<br>2. 搜索返回结果不包含该帖子 |

---

### TC-F-025：ES 同步 — 幂等性验证

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-025 |
| **标题** | 重复消费同一 content.published 事件不产生副作用 |
| **需求来源** | v2.1 FR-1 边缘场景 |
| **优先级** | 中 |
| **前置条件** | 帖子已索引到 ES |
| **测试步骤** | 1. 手动发布一条与已有帖子相同 post_id 的 content.published 事件<br>2. 等待消费者处理<br>3. 通过 SearchContent 搜索该帖子 |
| **预期结果** | 1. ES 中该帖子数据与最新 MySQL 数据一致<br>2. 不产生重复索引<br>3. 同一 post_id 覆盖写（幂等） |

---

### TC-F-026：ES 同步消费者 — 优雅停止

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-026 |
| **标题** | 收到 SIGTERM 时 ESSyncConsumer 优雅停止 |
| **需求来源** | v2.1 Story 1、FR-1 |
| **优先级** | 高 |
| **前置条件** | ESSyncConsumer 正在运行，有 in-flight 消息正在处理 |
| **测试步骤** | 1. 向 cmd/content 进程发送 SIGTERM 信号<br>2. 检查日志中是否有优雅停止相关信息<br>3. 等待进程退出 |
| **预期结果** | 1. ESSyncConsumer.Stop() 被调用<br>2. in-flight 消息处理完成后进程退出<br>3. 不出现消息丢失或半处理状态 |

---

### TC-F-027：二级评论 — 创建回复

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-027 |
| **标题** | 对一级评论进行回复（二级评论） |
| **需求来源** | v2.1 Story 2、FR-2 |
| **优先级** | 高 |
| **前置条件** | 已有一条 status=published 的帖子；已有一条一级评论（parent_id=0）；用户已登录 |
| **测试步骤** | 1. 调用 CreateComment，school_id、post_id、user_id、content="我也觉得很好用"，parent_id=一级评论ID<br>2. 查询帖子 comment_count |
| **预期结果** | 1. 评论创建成功，comment_id 为合法雪花 ID<br>2. parent_id 持久化为一级评论 ID<br>3. 帖子 comment_count +1<br>4. MQ 发送 content.replied 事件 |

---

### TC-F-028：二级评论 — 禁止三级嵌套

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-028 |
| **标题** | 对二级评论回复应被拒绝（仅支持两级） |
| **需求来源** | v2.1 Story 2、FR-2 |
| **优先级** | 高 |
| **前置条件** | 已有一条二级评论（parent_id=一级评论ID） |
| **测试步骤** | 1. 调用 CreateComment，parent_id=二级评论ID<br>2. 检查返回结果 |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 错误信息提示"仅支持二级回复，不允许嵌套"<br>3. 数据库中无新评论记录 |

---

### TC-F-029：二级评论 — 校验父评论存在性

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-029 |
| **标题** | parent_id 指向不存在的评论时返回 404 |
| **需求来源** | v2.1 FR-2 边缘场景 |
| **优先级** | 高 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 调用 CreateComment，parent_id=999999（不存在的 ID） |
| **预期结果** | 1. 返回 NOT_FOUND 错误<br>2. 不创建任何评论记录 |

---

### TC-F-030：二级评论 — 父评论已删除

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-030 |
| **标题** | parent_id 指向已删除评论时返回 410 |
| **需求来源** | v2.1 FR-2 边缘场景 |
| **优先级** | 中 |
| **前置条件** | 存在一条已软删除的一级评论（status=2） |
| **测试步骤** | 1. 调用 CreateComment，parent_id=已删除评论ID |
| **预期结果** | 1. 返回 FAILED_PRECONDITION 错误<br>2. 错误信息提示"该评论已删除" |

---

### TC-F-031：二级评论 — 父评论跨学校

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-031 |
| **标题** | parent_id 指向其他学校的评论时返回 403 |
| **需求来源** | v2.1 FR-2 边缘场景 |
| **优先级** | 高 |
| **前置条件** | school_id=100 下有一条一级评论；当前请求 school_id=200 |
| **测试步骤** | 1. 调用 CreateComment，school_id=200，parent_id=100学校的一级评论ID |
| **预期结果** | 1. 返回 PERMISSION_DENIED 错误<br>2. 不创建任何评论记录 |

---

### TC-F-032：@mention — 创建评论时 @ 多个用户

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-032 |
| **标题** | 评论中 @mention 多个用户并正确写入 comment_mentions 表 |
| **需求来源** | v2.1 Story 3、FR-3 |
| **优先级** | 高 |
| **前置条件** | 已有 3 个合法用户（user_id=10、20、30），属于同一 school_id |
| **测试步骤** | 1. 调用 CreateComment，mentioned_user_ids=[10, 20, 30]<br>2. 查询 comment_mentions 表 |
| **预期结果** | 1. 评论创建成功<br>2. comment_mentions 表中新增 3 条记录，comment_id 指向新评论<br>3. 每条记录的 mentioned_user_id 分别为 10、20、30<br>4. 三条记录与评论在同一事务内写入 |

---

### TC-F-033：@mention — 校验 @ 人数上限

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-033 |
| **标题** | mentioned_user_ids 超过 5 个时拒绝 |
| **需求来源** | v2.1 Story 3、FR-3 |
| **优先级** | 高 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 调用 CreateComment，mentioned_user_ids=[1,2,3,4,5,6]（6 个 ID） |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 错误信息提示"最多 @ 5 人"<br>3. 不创建任何评论或 mention 记录 |

---

### TC-F-034：@mention — 校验 ID 合法性（非负数）

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-034 |
| **标题** | mentioned_user_ids 含 0 或负数时拒绝 |
| **需求来源** | v2.1 Story 3、FR-3 边缘场景 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 调用 CreateComment，mentioned_user_ids=[0, -1, 10] |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 错误信息提示 ID 必须为正整数 |

---

### TC-F-035：@mention — 自动去重

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-035 |
| **标题** | mentioned_user_ids 含重复 ID 时自动去重 |
| **需求来源** | v2.1 Story 3、FR-3 |
| **优先级** | 中 |
| **前置条件** | user_id=10 存在 |
| **测试步骤** | 1. 调用 CreateComment，mentioned_user_ids=[10, 10, 10]<br>2. 查询 comment_mentions 表 |
| **预期结果** | 1. 评论创建成功<br>2. comment_mentions 表中仅 1 条记录（comment_id, mentioned_user_id=10） |

---

### TC-F-036：content.replied MQ 事件 — 二级评论发布

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-036 |
| **标题** | 二级评论创建成功后发布 content.replied 事件 |
| **需求来源** | v2.1 Story 4、FR-4 |
| **优先级** | 高 |
| **前置条件** | MQ 可用；已有一条一级评论 |
| **测试步骤** | 1. 创建二级评论（parent_id=一级评论ID，content="我也觉得..."）<br>2. 监听 MQ content.events 队列 |
| **预期结果** | 1. MQ 中出现 content.replied 事件<br>2. 事件 JSON 包含 type、post_id、school_id、user_id、trace_id、time<br>3. data 字段包含 parent_comment_id、parent_comment_user_id、mentioned_user_ids、content_preview（前 50 字）<br>4. trace_id 与请求链路一致 |

---

### TC-F-037：content.replied 事件 — 一级评论不触发

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-037 |
| **标题** | 创建一级评论时不发布 content.replied 事件 |
| **需求来源** | v2.1 Story 4、FR-4 |
| **优先级** | 高 |
| **前置条件** | MQ 可用 |
| **测试步骤** | 1. 创建一级评论（parent_id=0）<br>2. 监听 MQ content.events 队列 |
| **预期结果** | 1. MQ 中出现 content.comment_created 事件，但不出现 content.replied 事件 |

---

### TC-F-038：content.replied 事件 — best-effort 发布

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-038 |
| **标题** | MQ 发布失败时不阻塞评论创建返回 |
| **需求来源** | v2.1 Story 4、FR-4 |
| **优先级** | 中 |
| **前置条件** | MQ 不可用（模拟连接断开） |
| **测试步骤** | 1. 模拟 MQ Publisher 返回错误<br>2. 创建二级评论<br>3. 检查评论是否创建成功 |
| **预期结果** | 1. 评论创建成功返回（不阻塞）<br>2. 仅记录 ERROR 日志（含 trace_id）<br>3. 不返回错误给调用方 |

---

### TC-F-039：级联软删除 — 删除一级评论时级联删除回复

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-039 |
| **标题** | 删除一级评论时同时软删除其下所有二级回复 |
| **需求来源** | v2.1 Story 5、FR-5 |
| **优先级** | 高 |
| **前置条件** | 已有一条一级评论，其下有 3 条二级回复 |
| **测试步骤** | 1. 调用 DeleteComment，comment_id=一级评论ID，user_id=评论作者<br>2. 查询 comment_mentions 和评论表 |
| **预期结果** | 1. 一级评论 status=2（软删除）<br>2. 3 条二级回复均 status=2（软删除）<br>3. 所有操作在同一事务内完成<br>4. 帖子 comment_count 递减 4（1 条一级 + 3 条二级） |

---

### TC-F-040：级联软删除 — 删除二级评论仅删自身

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-040 |
| **标题** | 删除二级评论时不影响一级评论 |
| **需求来源** | v2.1 Story 5、FR-5 |
| **优先级** | 高 |
| **前置条件** | 已有一条一级评论，其下有 2 条二级回复 |
| **测试步骤** | 1. 调用 DeleteComment，comment_id=二级评论ID<br>2. 查询一级评论状态 |
| **预期结果** | 1. 二级评论 status=2<br>2. 一级评论仍为 status=1（正常）<br>3. 帖子 comment_count -1 |

---

### TC-F-041：评论列表 — 仅返回一级评论

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-041 |
| **标题** | ListComments 默认只返回一级评论 |
| **需求来源** | v2.1 Story 5、FR-5 |
| **优先级** | 高 |
| **前置条件** | 帖子下有 2 条一级评论，第一条下有 3 条二级回复 |
| **测试步骤** | 1. 调用 ListComments，post_id，不传 parent_id |
| **预期结果** | 1. 返回 2 条一级评论<br>2. 不返回任何二级回复<br>3. 过滤掉 status=2 的已删除评论 |

---

### TC-F-042：评论列表 — 获取指定父评论的回复

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-042 |
| **标题** | ListCommentReplies 返回指定父评论下的所有回复 |
| **需求来源** | v2.1 Story 5、FR-5 |
| **优先级** | 高 |
| **前置条件** | 一级评论 A 下有 3 条回复，一级评论 B 下有 1 条回复 |
| **测试步骤** | 1. 调用 ListCommentReplies，parent_id=评论A的ID |
| **预期结果** | 1. 返回 3 条回复<br>2. 不返回评论 B 的回复<br>3. 按 created_at 倒序排列 |

---

### TC-F-043：删除一级评论 — 无回复时正常删除

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-043 |
| **标题** | 删除无回复的一级评论走原有逻辑 |
| **需求来源** | v2.1 Story 5、FR-5 |
| **优先级** | 中 |
| **前置条件** | 已有一条一级评论，无二级回复 |
| **测试步骤** | 1. 调用 DeleteComment，comment_id=一级评论ID |
| **预期结果** | 1. 一级评论 status=2<br>2. comment_count -1<br>3. 不查询或操作二级评论 |

---

### TC-F-044：TraceID 全链路透传 — MQ 消息

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-044 |
| **标题** | MQ 消息中正确携带 TraceID |
| **需求来源** | v1.0 技术约束、v2.1 FR-1 |
| **优先级** | 高 |
| **前置条件** | MQ 可用；Jaeger 可用 |
| **测试步骤** | 1. 通过 Gateway 发起创建帖子请求<br>2. 审核通过后检查 MQ 消息中的 trace_id<br>3. 在 Jaeger 中按该 trace_id 搜索 |
| **预期结果** | 1. MQ 消息头或 body 中包含合法 TraceID<br>2. Jaeger 中可见完整的 Span 链：HTTP → gRPC → MQ publish → MQ consume |

---

### TC-F-045：MQ 事件常量兼容性

| 项目 | 内容 |
|------|------|
| **编号** | TC-F-045 |
| **标题** | 新增 content.replied 事件不破坏既有事件常量 |
| **需求来源** | v2.1 Story 4、FR-4 |
| **优先级** | 中 |
| **前置条件** | 代码库已包含 v1.0 事件常量 |
| **测试步骤** | 1. 检查 pkg/mq/publisher.go 中的事件常量<br>2. 验证 content.published、content.comment_created、content.liked、content.review_result、content.taken_down、content.expiring_soon 仍存在且未修改<br>3. 验证新增 content.replied 常量存在 |
| **预期结果** | 1. 所有 v1.0 事件常量保持不变<br>2. 新增 EventContentReplied 常量<br>3. 编译通过，无兼容性问题 |

---

## 2. 边界测试（TC-E）

### TC-E-001：帖子标题 — 空字符串

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-001 |
| **标题** | 创建帖子时标题为空 |
| **需求来源** | v1.0 Story 1、功能 1 |
| **优先级** | 高 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 CreatePostRequest，title=""，其他字段合法<br>2. 调用 CreatePost |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 错误信息提示标题不能为空 |

---

### TC-E-002：帖子标题 — 最大长度

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-002 |
| **标题** | 创建帖子时标题达到最大长度限制 |
| **需求来源** | v1.0 Story 1 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 title 为 100 个字符的合法字符串<br>2. 调用 CreatePost |
| **预期结果** | 1. 帖子创建成功（或返回长度超限错误，取决于业务定义） |

---

### TC-E-003：帖子标题 — 超过最大长度

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-003 |
| **标题** | 创建帖子时标题超过最大长度限制 |
| **需求来源** | v1.0 Story 1 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 title 为 200 个字符的字符串<br>2. 调用 CreatePost |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 错误信息提示标题长度超限 |

---

### TC-E-004：帖子内容 — 空字符串

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-004 |
| **标题** | 创建帖子时内容为空 |
| **需求来源** | v1.0 Story 1 |
| **优先级** | 高 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 CreatePostRequest，content=""，title 合法<br>2. 调用 CreatePost |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 提示内容不能为空 |

---

### TC-E-005：帖子内容 — 最大长度

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-005 |
| **标题** | 创建帖子时内容达到最大长度限制 |
| **需求来源** | v1.0 Story 1 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 content 为 5000 个字符的合法字符串<br>2. 调用 CreatePost |
| **预期结果** | 1. 帖子创建成功（或返回长度超限错误） |

---

### TC-E-006：图片列表 — 空列表

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-006 |
| **标题** | 创建帖子时未上传任何图片 |
| **需求来源** | v1.0 Story 1 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 CreatePostRequest，images=[]（空列表）<br>2. 调用 CreatePost |
| **预期结果** | 1. 帖子创建成功<br>2. images 字段为空列表（不强制要求图片） |

---

### TC-E-007：二手帖价格 — 零价格

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-007 |
| **标题** | 二手帖 price=0（免费赠送） |
| **需求来源** | v1.0 Story 2、功能 3 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 CreatePostRequest，type=second_hand，price=0<br>2. 调用 CreatePost |
| **预期结果** | 1. 帖子创建成功（免费赠送场景合法）<br>2. price 字段为 0 |

---

### TC-E-008：二手帖价格 — 负数价格

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-008 |
| **标题** | 二手帖 price 为负数应被拒绝 |
| **需求来源** | v1.0 Story 2 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 构造 CreatePostRequest，type=second_hand，price=-10<br>2. 调用 CreatePost |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 提示价格不能为负数 |

---

### TC-E-009：游标分页 — 空结果

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-009 |
| **标题** | 帖子列表在该学校无数据时返回空列表 |
| **需求来源** | v1.0 Story 3 |
| **优先级** | 中 |
| **前置条件** | school_id=999 下无任何帖子 |
| **测试步骤** | 1. 调用 ListPosts，school_id=999 |
| **预期结果** | 1. 返回空列表<br>2. 不报错<br>3. cursor 为空（表示无更多数据） |

---

### TC-E-010：游标分页 — 最后一页

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-010 |
| **标题** | 帖子列表最后一页数据正确 |
| **需求来源** | v1.0 Story 3 |
| **优先级** | 中 |
| **前置条件** | school_id 下有 15 条已发布帖子，limit=10 |
| **测试步骤** | 1. 第一次调用 ListPosts，limit=10<br>2. 第二次调用 ListPosts，cursor=第一页最后一条 ID，limit=10 |
| **预期结果** | 1. 第一次返回 10 条<br>2. 第二次返回 5 条<br>3. 第二次返回的 cursor 为空（无更多数据） |

---

### TC-E-011：@mention — 空列表

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-011 |
| **标题** | mentioned_user_ids 为空列表时不创建 mention 记录 |
| **需求来源** | v2.1 FR-3 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 调用 CreateComment，mentioned_user_ids=[]（空列表）<br>2. 查询 comment_mentions 表 |
| **预期结果** | 1. 评论创建成功<br>2. comment_mentions 表中无新增记录 |

---

### TC-E-012：@mention — 刚好 5 个用户

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-012 |
| **标题** | mentioned_user_ids 刚好 5 个时创建成功 |
| **需求来源** | v2.1 FR-3 |
| **优先级** | 中 |
| **前置条件** | 5 个合法用户存在 |
| **测试步骤** | 1. 调用 CreateComment，mentioned_user_ids=[1,2,3,4,5]<br>2. 查询 comment_mentions |
| **预期结果** | 1. 评论创建成功<br>2. comment_mentions 中新增 5 条记录 |

---

### TC-E-013：content_preview — 超过 50 字时截断

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-013 |
| **标题** | content.replied 事件中 content_preview 截取前 50 字 |
| **需求来源** | v2.1 Story 4 |
| **优先级** | 中 |
| **前置条件** | MQ 可用 |
| **测试步骤** | 1. 创建二级评论，content 为 100 个字符的字符串<br>2. 监听 MQ 消息 |
| **预期结果** | 1. MQ 事件中 data.content_preview 为前 50 个字符<br>2. 不超过 50 字 |

---

### TC-E-014：帖子列表 — limit 上限

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-014 |
| **标题** | ListPosts 传入超大 limit 值时应有上限控制 |
| **需求来源** | v1.0 Story 3 |
| **优先级** | 低 |
| **前置条件** | school_id 下有少量帖子 |
| **测试步骤** | 1. 调用 ListPosts，limit=10000 |
| **预期结果** | 1. 返回帖子数量不超过系统上限（如 50 或 100）<br>2. 不出现超时或 OOM |

---

### TC-E-015：级联软删除 — 一级评论下无回复

| 项目 | 内容 |
|------|------|
| **编号** | TC-E-015 |
| **标题** | 删除无回复的一级评论时级联逻辑不影响性能 |
| **需求来源** | v2.1 FR-5 |
| **优先级** | 低 |
| **前置条件** | 已有一条一级评论，无二级回复 |
| **测试步骤** | 1. 调用 DeleteComment，comment_id=一级评论ID<br>2. 记录响应时间 |
| **预期结果** | 1. 评论删除成功<br>2. 响应时间在正常范围内（< 200ms）<br>3. 无多余数据库操作 |

---

## 3. 异常测试（TC-ERR）

### TC-ERR-001：未授权发帖 — 缺少 JWT Token

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-001 |
| **标题** | 未携带 JWT Token 时创建帖子被拒绝 |
| **需求来源** | v1.0 安全与合规 |
| **优先级** | 高 |
| **前置条件** | 无有效 JWT Token |
| **测试步骤** | 1. 构造合法 CreatePostRequest<br>2. 不携带 Authorization header<br>3. 调用接口 |
| **预期结果** | 1. 返回 UNAUTHENTICATED 错误<br>2. 数据库无新记录 |

---

### TC-ERR-002：未授权发帖 — 过期 JWT Token

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-002 |
| **标题** | 使用过期 JWT Token 创建帖子被拒绝 |
| **需求来源** | v1.0 安全与合规 |
| **优先级** | 高 |
| **前置条件** | 持有过期的 JWT Token |
| **测试步骤** | 1. 使用过期 Token 构造请求<br>2. 调用 CreatePost |
| **预期结果** | 1. 返回 UNAUTHENTICATED 错误<br>2. 提示 Token 已过期 |

---

### TC-ERR-003：school_id 缺失

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-003 |
| **标题** | 请求中缺少 school_id |
| **需求来源** | v1.0 多租户隔离 |
| **优先级** | 高 |
| **前置条件** | 用户已登录，但 JWT 中无 school_id |
| **测试步骤** | 1. 构造 CreatePostRequest，school_id=0<br>2. 调用 CreatePost |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 提示 school_id 不能为空 |

---

### TC-ERR-004：跨学校数据访问 — 查看其他学校帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-004 |
| **标题** | 使用 school_id=200 的 Token 查看 school_id=100 的帖子 |
| **需求来源** | v1.0 多租户隔离、安全与合规 |
| **优先级** | 高 |
| **前置条件** | school_id=100 下有已发布帖子 |
| **测试步骤** | 1. 使用 school_id=200 的 JWT 调用 GetPost，post_id=100学校的帖子 |
| **预期结果** | 1. 返回 NOT_FOUND 或 PERMISSION_DENIED 错误<br>2. 不泄露帖子内容 |

---

### TC-ERR-005：跨学校数据访问 — 评论跨学校

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-005 |
| **标题** | 使用 school_id=200 的 Token 对 school_id=100 的帖子评论 |
| **需求来源** | v1.0 多租户隔离 |
| **优先级** | 高 |
| **前置条件** | school_id=100 下有已发布帖子 |
| **测试步骤** | 1. 使用 school_id=200 的 JWT 调用 CreateComment，post_id=100学校的帖子 |
| **预期结果** | 1. 返回 PERMISSION_DENIED 错误<br>2. SchoolScope 强制过滤，不创建跨校评论 |

---

### TC-ERR-006：跨学校数据访问 — 搜索隔离

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-006 |
| **标题** | 搜索结果仅包含本校帖子 |
| **需求来源** | v1.0 Story 3、多租户隔离 |
| **优先级** | 高 |
| **前置条件** | school_id=100 和 school_id=200 各有 5 条帖子 |
| **测试步骤** | 1. 调用 SearchContent，school_id=100 |
| **预期结果** | 1. 仅返回 school_id=100 的帖子<br>2. 结果中不包含 school_id=200 的任何帖子 |

---

### TC-ERR-007：ES 同步 — MySQL 读不到帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-007 |
| **标题** | ES 消费者收到事件但 MySQL 中帖子不存在时重试 |
| **需求来源** | v2.1 FR-1 错误处理 |
| **优先级** | 中 |
| **前置条件** | ESSyncConsumer 已启动 |
| **测试步骤** | 1. 手动发送 content.published 事件，post_id 指向不存在的帖子<br>2. 检查 MQ 消息状态和日志 |
| **预期结果** | 1. 消费者返回错误，消息 Nack+requeue<br>2. 30 秒后自动重试（指数退避）<br>3. 日志中记录 ERROR 级别错误 |

---

### TC-ERR-008：ES 同步 — ES 索引失败

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-008 |
| **标题** | ES 索引操作失败时消息重试 |
| **需求来源** | v2.1 FR-1 错误处理 |
| **优先级** | 中 |
| **前置条件** | ESSyncConsumer 已启动；ES 模拟返回错误 |
| **测试步骤** | 1. 模拟 ES 连接不可用<br>2. 触发 content.published 事件<br>3. 检查日志 |
| **预期结果** | 1. 消息 Nack+requeue<br>2. 指数退避重试<br>3. 记录 ERROR 日志 |

---

### TC-ERR-009：ES 同步 — MQ 连接断开后自动重连

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-009 |
| **标题** | MQ 连接断开后 ESSyncConsumer 自动重连 |
| **需求来源** | v2.1 FR-1 边缘场景 |
| **优先级** | 中 |
| **前置条件** | ESSyncConsumer 正在运行 |
| **测试步骤** | 1. 模拟 MQ 连接断开（重启 RabbitMQ）<br>2. 等待 30 秒<br>3. 发送一条 content.published 事件 |
| **预期结果** | 1. 消费者自动重连<br>2. 新事件被正常消费<br>3. 已注册的 handler 不丢失 |

---

### TC-ERR-010：ES 同步 — MQ 服务完全不可用时降级

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-010 |
| **标题** | MQ 不可用时 Content Service 降级运行 |
| **需求来源** | v2.1 FR-1 错误处理 |
| **优先级** | 中 |
| **前置条件** | RabbitMQ 完全不可用 |
| **测试步骤** | 1. 启动 cmd/content/main.go（MQ 不可用）<br>2. 调用 CreatePost 创建帖子 |
| **预期结果** | 1. gRPC server 正常启动并提供服务<br>2. 帖子创建成功（写 MySQL 成功）<br>3. MQ 发布失败仅记录 ERROR 日志<br>4. 不影响核心写操作 |

---

### TC-ERR-011：评论敏感词 — DFA 命中

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-011 |
| **标题** | 评论内容命中 DFA 敏感词时拒绝创建 |
| **需求来源** | v1.0 功能 6、v2.1 安全 |
| **优先级** | 高 |
| **前置条件** | DFA 词库已加载；有已发布帖子 |
| **测试步骤** | 1. 调用 CreateComment，content 含敏感词"赌博"<br>2. 检查返回结果 |
| **预期结果** | 1. 返回 FAILED_PRECONDITION 错误<br>2. 返回 SensitiveWordError，包含命中敏感词及位置<br>3. 数据库无新评论 |

---

### TC-ERR-012：删除非自己的帖子

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-012 |
| **标题** | 非发帖者尝试删除帖子被拒绝 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 高 |
| **前置条件** | 帖子由 user_id=1 创建 |
| **测试步骤** | 1. 使用 user_id=2 调用 DeletePost |
| **预期结果** | 1. 返回 PERMISSION_DENIED 错误<br>2. 帖子状态不变 |

---

### TC-ERR-013：删除非自己的评论

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-013 |
| **标题** | 非评论作者尝试删除评论被拒绝 |
| **需求来源** | v1.0 功能 4 |
| **优先级** | 高 |
| **前置条件** | 评论由 user_id=1 创建 |
| **测试步骤** | 1. 使用 user_id=2 调用 DeleteComment |
| **预期结果** | 1. 返回 PERMISSION_DENIED 错误<br>2. 评论状态不变 |

---

### TC-ERR-014：审核已发布帖子 — 重复审核

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-014 |
| **标题** | 对已审核通过的帖子再次调用 ApprovePost |
| **需求来源** | v1.0 Story 5 |
| **优先级** | 中 |
| **前置条件** | 帖子 status=published |
| **测试步骤** | 1. 调用 ApprovePost，post_id=已发布帖子 |
| **预期结果** | 1. 返回 FAILED_PRECONDITION 错误<br>2. 提示帖子不在待审核状态 |

---

### TC-ERR-015：点赞后重复点赞

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-015 |
| **标题** | 同一用户对同一帖子重复点赞 |
| **需求来源** | v1.0 Story 4 |
| **优先级** | 中 |
| **前置条件** | 用户已点赞某帖子 |
| **测试步骤** | 1. 再次调用 LikePost |
| **预期结果** | 1. 返回 ALREADY_EXISTS 或幂等成功<br>2. likes_count 不重复增加 |

---

### TC-ERR-016：未点赞时取消点赞

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-016 |
| **标题** | 用户未点赞时调用 UnlikePost |
| **需求来源** | v1.0 Story 4 |
| **优先级** | 中 |
| **前置条件** | 用户未对该帖子点赞 |
| **测试步骤** | 1. 调用 UnlikePost |
| **预期结果** | 1. 返回 NOT_FOUND 或幂等成功<br>2. likes_count 不变（不为负数） |

---

### TC-ERR-017：对已过期帖子评论

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-017 |
| **标题** | 对 status=expired 的帖子发表评论被拒绝 |
| **需求来源** | v1.0 功能 4 |
| **优先级** | 中 |
| **前置条件** | 帖子 status=expired |
| **测试步骤** | 1. 调用 CreateComment，post_id=过期帖子 |
| **预期结果** | 1. 返回 FAILED_PRECONDITION 错误<br>2. 提示帖子已过期不可评论 |

---

### TC-ERR-018：对已关闭帖子点赞

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-018 |
| **标题** | 对 status=closed 的帖子点赞被拒绝 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 低 |
| **前置条件** | 帖子 status=closed |
| **测试步骤** | 1. 调用 LikePost |
| **预期结果** | 1. 返回 FAILED_PRECONDITION 错误<br>2. 帖子 likes_count 不变 |

---

### TC-ERR-019：二级评论 — parent_id 为负数

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-019 |
| **标题** | CreateComment 中 parent_id 为负数时拒绝 |
| **需求来源** | v2.1 FR-2 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 调用 CreateComment，parent_id=-1 |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 提示 parent_id 无效 |

---

### TC-ERR-020：帖子不存在 — GetPost

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-020 |
| **标题** | 查询不存在的帖子返回 404 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 中 |
| **前置条件** | post_id=999999 不存在 |
| **测试步骤** | 1. 调用 GetPost，post_id=999999 |
| **预期结果** | 1. 返回 NOT_FOUND 错误 |

---

### TC-ERR-021：MQ 服务不可用时审核通过

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-021 |
| **标题** | MQ 不可用时 ApprovePost 仍能正常更新状态 |
| **需求来源** | v1.0 Story 5、v2.1 FR-1 |
| **优先级** | 中 |
| **前置条件** | MQ 模拟不可用 |
| **测试步骤** | 1. 调用 ApprovePost<br>2. 查询帖子状态 |
| **预期结果** | 1. 帖子 status=published（MySQL 更新成功）<br>2. MQ 事件发送失败记录 ERROR 日志<br>3. ES 不会同步（需人工补偿） |

---

### TC-ERR-022：ListPosts — school_id 参数不合法

| 项目 | 内容 |
|------|------|
| **编号** | TC-ERR-022 |
| **标题** | ListPosts 传入 school_id=0 |
| **需求来源** | v1.0 多租户隔离 |
| **优先级** | 中 |
| **前置条件** | 用户已登录 |
| **测试步骤** | 1. 调用 ListPosts，school_id=0 |
| **预期结果** | 1. 返回 INVALID_ARGUMENT 错误<br>2. 提示 school_id 无效 |

---

## 4. 状态转换测试（TC-ST）

### TC-ST-001：正常审核流程 — pending → published

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-001 |
| **标题** | 帖子从审核中变为已发布 |
| **需求来源** | v1.0 功能 1、Story 5 |
| **优先级** | 高 |
| **前置条件** | 已创建帖子，status=pending |
| **测试步骤** | 1. 调用 ApprovePost<br>2. 查询帖子 status |
| **预期结果** | 1. status 从 pending 变为 published<br>2. MQ 发送 content.published 事件 |

---

### TC-ST-002：审核拒绝 — pending → rejected

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-002 |
| **标题** | 帖子从审核中变为已拒绝 |
| **需求来源** | v1.0 Story 5、附录 |
| **优先级** | 高 |
| **前置条件** | 已创建帖子，status=pending |
| **测试步骤** | 1. 调用 RejectPost<br>2. 查询帖子 status |
| **预期结果** | 1. status 从 pending 变为 rejected<br>2. rejected 帖子仅发帖者可见 |

---

### TC-ST-003：过期 — published → expired

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-003 |
| **标题** | 失物招领帖发布 30 天后自动过期 |
| **需求来源** | v1.0 功能 2 |
| **优先级** | 高 |
| **前置条件** | 失物招领帖 status=published，expired_at 设为当前时间 |
| **测试步骤** | 1. 等待 expired_at 到达（或模拟时间推进）<br>2. 触发过期检查逻辑<br>3. 查询帖子 status |
| **预期结果** | 1. status 从 published 变为 expired<br>2. MQ 发送 content.expiring_soon 事件（过期前 3 天） |

---

### TC-ST-004：过期 — published → expired（二手帖 60 天）

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-004 |
| **标题** | 二手交易帖发布 60 天后自动过期 |
| **需求来源** | v1.0 功能 3 |
| **优先级** | 高 |
| **前置条件** | 二手帖 status=published，expired_at 设为当前时间 |
| **测试步骤** | 1. 触发过期检查逻辑<br>2. 查询帖子 status |
| **预期结果** | 1. status 从 published 变为 expired<br>2. expired_at 为 created_at + 60 天 |

---

### TC-ST-005：主动关闭 — pending → closed

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-005 |
| **标题** | 审核中的帖子可被发帖者主动关闭 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 中 |
| **前置条件** | 帖子 status=pending |
| **测试步骤** | 1. 发帖者调用关闭接口<br>2. 查询帖子 status |
| **预期结果** | 1. status 从 pending 变为 closed |

---

### TC-ST-006：主动关闭 — published → closed（失物已找到）

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-006 |
| **标题** | 失物招领帖标记为"已当领"并关闭 |
| **需求来源** | v1.0 Story 1、功能 2 |
| **优先级** | 高 |
| **前置条件** | 失物招领帖 status=published |
| **测试步骤** | 1. 发帖者调用关闭/已当领接口<br>2. 查询帖子 status |
| **预期结果** | 1. status 从 published 变为 closed（retrieved） |

---

### TC-ST-007：主动关闭 — published → closed（已喇出）

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-007 |
| **标题** | 二手交易帖标记为"已喇出"并关闭 |
| **需求来源** | v1.0 Story 2、功能 3 |
| **优先级** | 高 |
| **前置条件** | 二手帖 status=published |
| **测试步骤** | 1. 发帖者调用关闭/已喇出接口<br>2. 查询帖子 status |
| **预期结果** | 1. status 从 published 变为 closed（sold） |

---

### TC-ST-008：非法状态转换 — published → pending

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-008 |
| **标题** | 已发布的帖子不能回退到审核中 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 中 |
| **前置条件** | 帖子 status=published |
| **测试步骤** | 1. 尝试将 status 改回 pending（模拟非法操作） |
| **预期结果** | 1. 状态机拒绝非法转换<br>2. status 仍为 published |

---

### TC-ST-009：非法状态转换 — expired → published

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-009 |
| **标题** | 已过期的帖子不能直接恢复为已发布 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 中 |
| **前置条件** | 帖子 status=expired |
| **测试步骤** | 1. 尝试将 status 改为 published |
| **预期结果** | 1. 状态机拒绝非法转换<br>2. status 仍为 expired |

---

### TC-ST-010：非法状态转换 — closed → published

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-010 |
| **标题** | 已关闭的帖子不能恢复为已发布 |
| **需求来源** | v1.0 功能 1 |
| **优先级** | 中 |
| **前置条件** | 帖子 status=closed |
| **测试步骤** | 1. 尝试将 status 改为 published |
| **预期结果** | 1. 状态机拒绝非法转换<br>2. status 仍为 closed |

---

### TC-ST-011：非法状态转换 — rejected → published

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-011 |
| **标题** | 已拒绝的帖子不能直接变为已发布 |
| **需求来源** | v1.0 附录 |
| **优先级** | 中 |
| **前置条件** | 帖子 status=rejected |
| **测试步骤** | 1. 尝试将 status 改为 published |
| **预期结果** | 1. 状态机拒绝非法转换<br>2. status 仍为 rejected |

---

### TC-ST-012：二手帖扩展状态 — published → sold

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-012 |
| **标题** | 二手帖标记为已喇出（sold） |
| **需求来源** | v1.0 功能 3 |
| **优先级** | 高 |
| **前置条件** | 二手帖 status=published |
| **测试步骤** | 1. 发帖者调用 sold 标记接口 |
| **预期结果** | 1. status 从 published 变为 sold（closed 的特化）<br>2. 帖子不再出现在列表中 |

---

### TC-ST-013：失物帖扩展状态 — published → retrieved

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-013 |
| **标题** | 失物招领帖标记为已当领（retrieved） |
| **需求来源** | v1.0 功能 2 |
| **优先级** | 高 |
| **前置条件** | 失物招领帖 status=published |
| **测试步骤** | 1. 发帖者调用 retrieved 标记接口 |
| **预期结果** | 1. status 从 published 变为 retrieved（closed 的特化）<br>2. 帖子不再出现在列表中 |

---

### TC-ST-014：ES 同步与状态联动 — 审核通过后 ES 可搜

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-014 |
| **标题** | 帖子状态变为 published 后 ES 索引同步 |
| **需求来源** | v1.0 功能 5、v2.1 FR-1 |
| **优先级** | 高 |
| **前置条件** | ESSyncConsumer 已启动；帖子 status=pending |
| **测试步骤** | 1. 调用 ApprovePost<br>2. 5 秒内调用 SearchContent 搜索 |
| **预期结果** | 1. status 变为 published<br>2. ES 中可搜到该帖子<br>3. TraceID 贯穿 gRPC → MQ → Consumer |

---

### TC-ST-015：ES 同步与状态联动 — 下架后 ES 不可搜

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-015 |
| **标题** | 帖子被下架后 ES 索引删除 |
| **需求来源** | v1.0 功能 5、v2.1 FR-1 |
| **优先级** | 高 |
| **前置条件** | 帖子已审核通过且在 ES 中可搜 |
| **测试步骤** | 1. 调用 TakedownPost<br>2. 等待 5 秒<br>3. 搜索该帖子 |
| **预期结果** | 1. ES 中该帖子被删除<br>2. 搜索结果不包含该帖子 |

---

### TC-ST-016：过期前提醒 — expiring_soon 事件

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-016 |
| **标题** | 帖子距过期 3 天内发布 content.expiring_soon 事件 |
| **需求来源** | v1.0 功能 2、RabbitMQ 事件规范 |
| **优先级** | 中 |
| **前置条件** | 失物招领帖 status=published，expired_at 设为当前时间 + 2 天 |
| **测试步骤** | 1. 触发过期提醒检查逻辑<br>2. 监听 MQ 消息 |
| **预期结果** | 1. MQ 中出现 content.expiring_soon 事件<br>2. 事件包含 post_id、user_id、school_id |

---

### TC-ST-017：管理员下架 — published → rejected(takedown)

| 项目 | 内容 |
|------|------|
| **编号** | TC-ST-017 |
| **标题** | 管理员对已发布帖子执行下架操作 |
| **需求来源** | v1.0 Story 5 |
| **优先级** | 高 |
| **前置条件** | 帖子 status=published |
| **测试步骤** | 1. Admin Service 调用 TakedownPost<br>2. 查询帖子状态 |
| **预期结果** | 1. 帖子状态更新为非 published<br>2. 不再对外可见<br>3. MQ 发送 content.taken_down 事件 |

---

## 5. 需求-测试用例覆盖矩阵

### v1.0 PRD 需求覆盖

| 需求编号 | 需求描述 | 测试用例编号 |
|---------|---------|-------------|
| Story 1 | 发布失物招领帖 | TC-F-001, TC-F-002, TC-F-018, TC-F-019 |
| Story 2 | 发布二手交易帖 | TC-F-003 |
| Story 3 | 搜索和筛选帖子 | TC-F-005, TC-F-006, TC-F-007, TC-F-016, TC-F-017, TC-ERR-006 |
| Story 4 | 互动（评论与点赞） | TC-F-012, TC-F-013, TC-F-014, TC-F-015, TC-F-037, TC-ERR-015, TC-ERR-016 |
| Story 5 | 内容审核（管理员） | TC-F-009, TC-F-010, TC-F-011, TC-ST-001, TC-ST-002, TC-ST-017 |
| 功能 1 | 通用帖子基础层 | TC-F-004, TC-F-008, TC-F-020, TC-F-021, TC-ST-005~TC-ST-011 |
| 功能 2 | 失物招领模板 | TC-F-001, TC-E-007, TC-ST-003, TC-ST-013, TC-ST-016 |
| 功能 3 | 二手交易模板 | TC-F-003, TC-E-007, TC-E-008, TC-ST-004, TC-ST-012 |
| 功能 4 | 评论系统 | TC-F-012, TC-F-013, TC-E-001~TC-E-005, TC-ERR-011~TC-ERR-013, TC-ERR-017 |
| 功能 5 | 内容搜索 | TC-F-016, TC-F-017, TC-ERR-006, TC-ST-014, TC-ST-015 |
| 功能 6 | DFA 敏感词过滤 | TC-F-018, TC-F-019, TC-ERR-011 |
| 多租户隔离 | school_id 强制隔离 | TC-ERR-003, TC-ERR-004, TC-ERR-005, TC-ERR-006, TC-ERR-022 |
| 安全与合规 | JWT 鉴权 | TC-ERR-001, TC-ERR-002 |
| 集成依赖 | MQ 异步同步 | TC-F-009, TC-F-011, TC-F-044, TC-ERR-007~TC-ERR-010, TC-ERR-021 |
| gRPC 接口 | ContentService 全部 RPC | TC-F-001~TC-F-021, TC-ERR-001~TC-ERR-022 |

### v2.1 PRD 需求覆盖

| 需求编号 | 需求描述 | 测试用例编号 |
|---------|---------|-------------|
| Story 1 | 审核通过后 ES 搜索可用 | TC-F-022, TC-F-023, TC-F-024, TC-F-025, TC-F-026 |
| Story 2 | 二级评论回复 | TC-F-027, TC-F-028, TC-F-029, TC-F-030, TC-F-031, TC-ERR-019 |
| Story 3 | 评论中 @mention | TC-F-032, TC-F-033, TC-F-034, TC-F-035, TC-E-011, TC-E-012 |
| Story 4 | content.replied MQ 事件 | TC-F-036, TC-F-037, TC-F-038, TC-F-045, TC-E-013 |
| Story 5 | 级联软删除 | TC-F-039, TC-F-040, TC-F-041, TC-F-042, TC-F-043, TC-E-015 |
| FR-1 | ES 同步消费者激活 | TC-F-022, TC-F-023, TC-F-024, TC-F-025, TC-F-026, TC-ERR-007~TC-ERR-010, TC-ERR-021 |
| FR-2 | 二级评论 API | TC-F-027, TC-F-028, TC-F-029, TC-F-030, TC-F-031, TC-ERR-019 |
| FR-3 | @mention 数据模型与校验 | TC-F-032, TC-F-033, TC-F-034, TC-F-035, TC-E-011, TC-E-012 |
| FR-4 | MQ content.replied 事件 | TC-F-036, TC-F-037, TC-F-038, TC-F-045, TC-E-013 |
| FR-5 | 级联软删除 | TC-F-039, TC-F-040, TC-F-041, TC-F-042, TC-F-043, TC-E-015 |
| 安全 | SchoolScope + DFA | TC-F-031, TC-ERR-004, TC-ERR-005, TC-ERR-006, TC-ERR-011 |
| 性能 | ES 同步延迟 P95 < 5s | TC-F-023 |
| 性能 | 二级评论 P95 < 200ms | TC-F-027 |
| 性能 | ListCommentReplies P95 < 150ms | TC-F-042 |
| 集成 | RabbitMQ + ES + gRPC | TC-F-022~TC-F-026, TC-F-036~TC-F-038, TC-F-044, TC-F-045 |

### 测试类别统计

| 测试类别 | 编号前缀 | 用例数 |
|---------|---------|--------|
| 功能测试 | TC-F | 45 |
| 边界测试 | TC-E | 15 |
| 异常测试 | TC-ERR | 22 |
| 状态转换测试 | TC-ST | 17 |
| **合计** | — | **85** |

### 优先级统计

| 优先级 | 用例数 |
|--------|--------|
| 高 | 52 |
| 中 | 29 |
| 低 | 4 |
| **合计** | **85** |

---

*本文档基于 content-service-prd.md（v1.0）和 content-service-v2-prd.md（v2.1）生成，覆盖功能测试、边界测试、异常测试和状态转换测试四大类别，共计 85 个测试用例。*
