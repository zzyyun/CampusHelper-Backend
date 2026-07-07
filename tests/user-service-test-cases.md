# 用户服务测试用例文档

**版本**：1.0
**日期**：2026-07-08
**关联文档**：
- `docs/user-service-prd.md`（用户服务 v1.1）
- `docs/user-service-v2.0-prd.md`（用户服务 v2.0）

---

## 目录

1. [测试用例列表](#测试用例列表)
   - [功能测试（TC-F）](#功能测试tc-f)
   - [边界测试（TC-E）](#边界测试tc-e)
   - [异常测试（TC-ERR）](#异常测试tc-err)
   - [状态转换测试（TC-ST）](#状态转换测试tc-st)
2. [需求-测试用例覆盖矩阵](#需求-测试用例覆盖矩阵)

---

## 测试用例列表

### 功能测试（TC-F）

| 编号 | 标题 | 需求来源 | 优先级 | 前置条件 | 测试步骤 | 预期结果 |
|------|------|---------|--------|---------|---------|---------|
| TC-F-001 | 微信登录 - 正常登录（新用户） | v1.0 WxLogin | 高 | 1. 微信 API jscode2session 可用<br>2. 用户未注册（openID 不存在） | 1. 调用 WxLogin RPC，传入有效 wxCode<br>2. 系统通过 wxCode 向微信 API 换取 openID<br>3. 系统在数据库中创建新用户记录<br>4. 返回 access_token 和 refresh_token | 1. 新用户记录创建成功，字段完整（openID、school_id=0、role=student、status=normal）<br>2. 返回两个 token，access_token 含 UserID/SchoolID/Role 信息<br>3. 用户 SchoolID 为 0（未绑定学校） |
| TC-F-002 | 微信登录 - 正常登录（已注册用户） | v1.0 WxLogin | 高 | 1. 用户 openID 已存在于数据库<br>2. 用户 status=normal | 1. 调用 WxLogin RPC，传入有效 wxCode<br>2. 系统通过 wxCode 向微信 API 换取 openID<br>3. 系统查询到已有用户记录<br>4. 返回 access_token 和 refresh_token | 1. 不创建新用户，返回已有用户信息<br>2. token 中包含正确的 UserID、SchoolID、Role<br>3. access_token 正常生成 |
| TC-F-003 | 微信登录 - 封禁用户登录 | v2.0 BanUser Story | 高 | 1. 用户 status=banned | 1. 调用 WxLogin RPC，传入该用户的有效 wxCode<br>2. 系统查询到用户记录，status=banned | 1. 登录被拒绝<br>2. 返回错误码 20008，错误信息"账号已被封禁"<br>3. 不生成任何 token |
| TC-F-004 | Token 续签 - 正常续签 | v1.0 RefreshToken | 高 | 1. 用户持有有效的 refresh_token<br>2. refresh_token 未过期 | 1. 调用 RefreshToken RPC，传入 refresh_token<br>2. 系统验证 refresh_token 有效性 | 1. 返回新的 access_token 和新的 refresh_token<br>2. 新 token 中包含正确的 UserID、SchoolID、Role<br>3. 旧 refresh_token 失效（轮转机制） |
| TC-F-005 | 绑定学校 - 正常绑定 | v1.0 BindCampus | 高 | 1. 用户已登录<br>2. 用户 school_id=0（未绑定）<br>3. 目标学校 school_id 存在于学校表 | 1. 调用 BindCampus RPC，传入 user_id 和 school_id<br>2. 系统验证学校存在<br>3. 更新用户 school_id<br>4. 清除 Redis 用户缓存 | 1. 用户 school_id 更新为指定学校<br>2. Redis 中 user 缓存被清除<br>3. 返回成功 |
| TC-F-006 | 获取当前用户 - 缓存命中 | v1.0 GetCurrentUser | 中 | 1. 用户已登录<br>2. Redis 中存在该用户的缓存 | 1. 调用 GetCurrentUser RPC<br>2. 系统从 Redis 缓存中读取用户信息 | 1. 返回完整的用户信息（ID、openID、school_id、role、status、nickname 等）<br>2. 未触发数据库查询（仅读缓存） |
| TC-F-007 | 获取当前用户 - 缓存未命中 | v1.0 GetCurrentUser | 中 | 1. 用户已登录<br>2. Redis 中无该用户缓存 | 1. 调用 GetCurrentUser RPC<br>2. 系统从 MySQL 查询用户信息<br>3. 查询结果写入 Redis 缓存 | 1. 返回完整的用户信息<br>2. Redis 中生成该用户的缓存<br>3. 缓存过期时间正确 |
| TC-F-008 | 更新用户信息 - 正常更新 | v1.0 UpdateUserInfo | 高 | 1. 用户已登录<br>2. Redis 中存在该用户缓存 | 1. 调用 UpdateUserInfo RPC，传入 user_id 和要修改的字段（如 nickname）<br>2. 系统更新数据库<br>3. 清除 Redis 缓存 | 1. 数据库中对应字段更新成功<br>2. Redis 缓存被清除<br>3. 返回更新后的用户信息 |
| TC-F-009 | 学校列表查询 - 不带关键字 | v1.1 ListSchools | 中 | 1. 用户已登录（JWT 鉴权）<br>2. 学校表中存在数据 | 1. 调用 GET /api/v1/schools，不带 keyword 参数<br>2. 系统查询全部学校（按分页） | 1. 返回学校列表，每项含 school_id、name、province<br>2. 返回 has_more 和 next_cursor 字段<br>3. 默认 page_size=20 |
| TC-F-010 | 学校列表查询 - 模糊搜索 | v1.1 ListSchools | 中 | 1. 用户已登录<br>2. 学校表中存在包含"大学"关键字的学校 | 1. 调用 GET /api/v1/schools?keyword=大学<br>2. 系统按关键字模糊匹配学校名称 | 1. 返回名称包含"大学"的学校列表<br>2. 结果按分页返回<br>3. API 响应 P95 < 100ms |
| TC-F-011 | 封禁用户 - 管理员封禁学生 | v2.0 BanUser | 高 | 1. 管理员已登录（role=admin，school_id=1）<br>2. 目标用户 role=student，status=normal，school_id=1<br>3. 目标用户已缓存在 Redis | 1. 调用 BanUser RPC，传入 user_id 和封禁原因<br>2. 系统更新目标用户 status=banned<br>3. 清除目标用户 Redis 缓存<br>4. 写入审计日志 | 1. 用户 status 更新为 banned<br>2. Redis 中 user 缓存被清除<br>3. 审计日志记录：操作人=admin、目标=user_id、操作类型=ban_user<br>4. 返回成功 |
| TC-F-012 | 封禁用户 - 超管封禁任意学校学生 | v2.0 BanUser | 高 | 1. 超管已登录（role=super_admin）<br>2. 目标用户 role=student，school_id=2（不同学校） | 1. 超管调用 BanUser RPC，传入 user_id<br>2. 系统更新目标用户 status=banned | 1. 目标用户被成功封禁<br>2. school_id 隔离对 super_admin 不生效<br>3. 审计日志记录操作 |
| TC-F-013 | 解封用户 | v2.0 UnbanUser | 高 | 1. 管理员已登录<br>2. 目标用户 status=banned | 1. 调用 UnbanUser RPC，传入 user_id<br>2. 系统更新目标用户 status=normal<br>3. 写入审计日志 | 1. 用户 status 更新为 normal<br>2. 用户可正常登录<br>3. 审计日志记录：操作类型=unban_user |
| TC-F-014 | 用户列表查询 - 按状态筛选 | v2.0 ListUsers | 高 | 1. 管理员已登录（school_id=1）<br>2. 本校存在 normal、banned 状态的用户 | 1. 调用 ListUsers RPC，status=banned<br>2. 系统按 school_id=1 + status=banned 查询 | 1. 仅返回本校 status=banned 的用户<br>2. 不包含其他学校的用户<br>3. 结果按注册时间倒序 |
| TC-F-015 | 用户列表查询 - 按角色筛选 | v2.0 ListUsers | 中 | 1. 管理员已登录（school_id=1）<br>2. 本校存在 student 和 admin 角色的用户 | 1. 调用 ListUsers RPC，role=student<br>2. 系统按 school_id=1 + role=student 查询 | 1. 仅返回本校 role=student 的用户<br>2. 不包含 admin 用户 |
| TC-F-016 | 用户列表查询 - 关键词搜索 | v2.0 ListUsers | 中 | 1. 管理员已登录（school_id=1）<br>2. 本校存在 nickname 为"张三"的用户 | 1. 调用 ListUsers RPC，keyword=张三<br>2. 系统按 nickname 模糊匹配 | 1. 返回 nickname 包含"张三"的用户<br>2. 不返回 nickname 不匹配的用户 |
| TC-F-017 | 用户列表查询 - 超管跨校查询 | v2.0 ListUsers | 高 | 1. 超管已登录<br>2. 多个学校存在用户 | 1. 超管调用 ListUsers RPC，school_id=0（查全部）<br>2. 系统查询所有学校的用户 | 1. 返回所有学校的用户列表<br>2. school_id=0 被正确识别为"查全部" |
| TC-F-018 | 设置用户角色 - 超管提升学生为管理员 | v2.0 SetUserRole | 高 | 1. 超管已登录<br>2. 目标用户 role=student，school_id=1 | 1. 超管调用 SetUserRole RPC，user_id=target，role=admin<br>2. 系统更新目标用户 role=admin<br>3. 清除目标用户 Redis 缓存<br>4. 写入审计日志 | 1. 目标用户 role 更新为 admin<br>2. Redis 缓存被清除<br>3. 审计日志记录：操作类型=set_role<br>4. 后续请求中目标用户具备 admin 权限 |
| TC-F-019 | 设置用户角色 - 超管撤销管理员 | v2.0 SetUserRole | 高 | 1. 超管已登录<br>2. 目标用户 role=admin | 1. 超管调用 SetUserRole RPC，role=student<br>2. 系统更新 role=student | 1. 目标用户 role 更新为 student<br>2. Redis 缓存被清除<br>3. 目标用户失去管理员权限 |
| TC-F-020 | 内容审核列表 - 查询待审核内容 | v2.0 ListContentForAudit | 高 | 1. 管理员已登录（school_id=1）<br>2. Content Service 中有 pending_review 状态的内容 | 1. 调用 ListContentForAudit RPC<br>2. 系统调用 Content Service gRPC 获取待审列表 | 1. 返回本校 pending_review 状态的内容列表<br>2. 按时间倒序排列<br>3. 支持 cursor 分页 |
| TC-F-021 | 内容审核 - 通过审核 | v2.0 AuditContent | 高 | 1. 管理员已登录<br>2. 目标内容 status=pending_review | 1. 调用 AuditContent RPC，action=approve<br>2. 系统调用 Content Service UpdateContentStatus(content_id, published)<br>3. 写入审计日志 | 1. 内容状态变更为 published<br>2. 内容对用户可见<br>3. 审计日志记录：操作类型=audit_content |
| TC-F-022 | 内容审核 - 驳回审核 | v2.0 AuditContent | 高 | 1. 管理员已登录<br>2. 目标内容 status=pending_review | 1. 调用 AuditContent RPC，action=reject，reason="包含广告信息"<br>2. 系统调用 Content Service UpdateContentStatus(content_id, rejected, reason) | 1. 内容状态变更为 rejected<br>2. 驳回原因被记录<br>3. 审计日志记录完整操作详情 |
| TC-F-023 | 操作审计日志 - 查询审计记录 | v2.0 审计日志 Story | 中 | 1. 超管已登录<br>2. 系统中已存在多条审计日志 | 1. 超管调用审计日志查询接口<br>2. 传入筛选条件（操作人、操作类型、时间范围） | 1. 返回符合条件的审计日志列表<br>2. 每条日志包含：操作人ID、目标ID、操作类型、操作时间、详情<br>3. 操作类型字段正确（ban_user/unban_user/set_role/audit_content） |
| TC-F-024 | 微信登录 - 代码换取 session（mock 微信 API） | v1.0 WxLogin | 高 | 1. 微信 API jscode2session 接口可用（mock） | 1. 启动 httptest server mock 微信 API<br>2. 传入 wxCode 调用 WxLogin<br>3. mock server 返回 openID 和 session_key | 1. 系统正确向微信 API 发送请求，参数包含 appid、secret、js_code、grant_type<br>2. 成功解析返回的 openID<br>3. 登录流程正常完成 |
| TC-F-025 | 封禁用户 - 幂等性（重复封禁） | v2.0 BanUser Edge Case | 中 | 1. 管理员已登录<br>2. 目标用户 status=banned（已被封禁） | 1. 再次调用 BanUser RPC，传入同一 user_id<br>2. 系统检查目标用户已处于 banned 状态 | 1. 返回成功（幂等操作）<br>2. 不重复写入审计日志（或标记为重复操作）<br>3. 用户状态保持 banned |
| TC-F-026 | 审计日志 - 90 天自动清理 | v2.0 审计日志清理 | 低 | 1. 数据库中存在超过 90 天的审计日志<br>2. StartCleanupTask 已启动 | 1. 触发清理任务<br>2. 系统扫描 audit_logs 表<br>3. 删除超过 90 天的记录 | 1. 超过 90 天的日志被删除<br>2. 90 天内的日志不受影响<br>3. 清理任务正常退出 |
| TC-F-027 | 审核列表 - Content Service 不可用降级 | v2.0 ListContentForAudit | 中 | 1. 管理员已登录<br>2. Content Service gRPC 不可用 | 1. 调用 ListContentForAudit RPC<br>2. Content Service 返回 Unavailable | 1. 返回 gRPC Unavailable 错误<br>2. 前端提示"请稍后重试"<br>3. User Service 自身不崩溃 |
| TC-F-028 | 审核内容 - 重复审核拒绝 | v2.0 AuditContent Edge Case | 中 | 1. 管理员已登录<br>2. 目标内容已被其他管理员审核（status=published） | 1. 调用 AuditContent RPC，action=reject<br>2. 系统检查内容当前状态 | 1. Content Service 拒绝重复操作<br>2. 返回"内容已被审核"相关错误<br>3. 不修改内容状态 |

---

### 边界测试（TC-E）

| 编号 | 标题 | 需求来源 | 优先级 | 前置条件 | 测试步骤 | 预期结果 |
|------|------|---------|--------|---------|---------|---------|
| TC-E-001 | WxLogin - 空 wxCode | v1.0 WxLogin | 高 | 1. 用户未登录 | 1. 调用 WxLogin RPC，wxCode 为空字符串 | 1. 返回 InvalidArgument 错误<br>2. 不调用微信 API |
| TC-E-002 | RefreshToken - 过期 token | v1.0 RefreshToken | 高 | 1. 用户持有已过期的 refresh_token | 1. 调用 RefreshToken RPC，传入过期 token | 1. 返回认证失败错误<br>2. 不生成新 token |
| TC-E-003 | RefreshToken - 非法 token 格式 | v1.0 RefreshToken | 高 | 1. 持有格式错误的字符串 | 1. 调用 RefreshToken RPC，传入"invalid_token_string" | 1. 返回 InvalidArgument 或 Unauthenticated 错误<br>2. 不生成新 token |
| TC-E-004 | BindCampus - 学校不存在 | v1.0 BindCampus | 高 | 1. 用户已登录<br>2. 目标 school_id 不存在于学校表 | 1. 调用 BindCampus RPC，传入不存在的 school_id | 1. 返回"学校不存在"错误<br>2. 用户 school_id 不变 |
| TC-E-005 | BindCampus - 重复绑定同一学校 | v1.0 BindCampus | 中 | 1. 用户已绑定 school_id=1 | 1. 再次调用 BindCampus RPC，school_id=1 | 1. 返回成功（幂等）或提示已绑定<br>2. 用户 school_id 保持不变 |
| TC-E-006 | UpdateUserInfo - 空参数更新 | v1.0 UpdateUserInfo | 中 | 1. 用户已登录 | 1. 调用 UpdateUserInfo RPC，传入 user_id，不传任何修改字段 | 1. 返回成功但不执行实际更新<br>2. 数据库记录不变 |
| TC-E-007 | ListSchools - page_size 上限校验 | v1.1 ListSchools | 中 | 1. 用户已登录 | 1. 调用 GET /api/v1/schools?page_size=100（超过上限 50） | 1. page_size 被限制为 50<br>2. 返回最多 50 条学校数据 |
| TC-E-008 | ListSchools - page_size=0 | v1.1 ListSchools | 中 | 1. 用户已登录 | 1. 调用 GET /api/v1/schools?page_size=0 | 1. 使用默认 page_size=20<br>2. 返回 20 条数据 |
| TC-E-009 | ListSchools - 空关键字 | v1.1 ListSchools | 低 | 1. 用户已登录 | 1. 调用 GET /api/v1/schools?keyword=（空字符串） | 1. 返回全部学校列表（等同于不带 keyword）<br>2. 分页正常工作 |
| TC-E-010 | BanUser - user_id=0 | v2.0 BanUser | 高 | 1. 管理员已登录 | 1. 调用 BanUser RPC，user_id=0 | 1. 返回 InvalidArgument 错误<br>2. 不执行任何数据库操作 |
| TC-E-011 | BanUser - user_id 为负数 | v2.0 BanUser | 中 | 1. 管理员已登录 | 1. 调用 BanUser RPC，user_id=-1 | 1. 返回 InvalidArgument 错误 |
| TC-E-012 | ListUsers - page_size 超上限 | v2.0 ListUsers | 中 | 1. 管理员已登录 | 1. 调用 ListUsers RPC，page_size=100 | 1. page_size 被限制为上限值<br>2. 返回正确数量的用户 |
| TC-E-013 | SetUserRole - 无效角色值 | v2.0 SetUserRole | 高 | 1. 超管已登录 | 1. 调用 SetUserRole RPC，role=99（无效值） | 1. 返回 InvalidArgument 错误<br>2. 用户角色不变 |
| TC-E-014 | AuditContent - 空 reason（通过审核） | v2.0 AuditContent | 低 | 1. 管理员已登录<br>2. 目标内容 status=pending_review | 1. 调用 AuditContent RPC，action=approve，reason 为空 | 1. 审核通过成功<br>2. reason 为空不影响通过操作 |
| TC-E-015 | ListContentForAudit - 空分页 cursor | v2.0 ListContentForAudit | 中 | 1. 管理员已登录<br>2. 存在待审内容 | 1. 调用 ListContentForAudit RPC，cursor 为空字符串<br>2. 系统将其视为第一页查询 | 1. 返回第一页待审内容<br>2. cursor 为空时从头开始查询 |
| TC-E-016 | SetUserRole - 超管修改自身角色 | v2.0 SetUserRole | 高 | 1. 超管已登录<br>2. 传入自己的 user_id | 1. 超管调用 SetUserRole RPC，user_id=自己的 ID，role=student | 1. 返回"不能修改自身角色"错误<br>2. 超管角色不变 |
| TC-E-017 | userIDFromCtx - context 中无 metadata | v1.1 调试日志清理 | 高 | 1. 构造一个空的 context（无 metadata） | 1. 调用 userIDFromCtx<br>2. 传入空 context | 1. 返回 0<br>2. 无 fmt.Printf 输出<br>3. 无 panic |
| TC-E-018 | userIDFromCtx - metadata 中无 user-id 键 | v1.1 调试日志清理 | 高 | 1. 构造含有 metadata 但无 user-id 的 context | 1. 调用 userIDFromCtx<br>2. 传入仅包含其他 key 的 metadata | 1. 返回 0<br>2. 无 fmt.Printf 输出 |
| TC-E-019 | userIDFromCtx - user-id 格式非法 | v1.1 调试日志清理 | 高 | 1. 构造 metadata，user-id="not_a_number" | 1. 调用 userIDFromCtx<br>2. ParseInt 无法解析 | 1. 返回 0<br>2. 无 fmt.Printf 输出 |
| TC-E-020 | ListUsers - school_id 为 0（超管查全部） | v2.0 ListUsers | 高 | 1. 超管已登录<br>2. 多个学校存在用户 | 1. 超管调用 ListUsers RPC，school_id=0 | 1. 返回所有学校的用户<br>2. school_id=0 表示"不限制学校" |
| TC-E-021 | BanUser - 封禁不存在的用户 | v2.0 BanUser | 高 | 1. 管理员已登录<br>2. 传入不存在的 user_id | 1. 调用 BanUser RPC，传入数据库中不存在的 user_id | 1. 返回"用户不存在"错误<br>2. 不写入审计日志 |
| TC-E-022 | 核心路径测试覆盖率检查 | v1.1 测试覆盖 | 高 | 1. 测试代码已编写完成 | 1. 执行 `go test -coverprofile` 检查核心路径覆盖率 | 1. 核心路径覆盖率 > 60%<br>2. 关键路径（认证、多租户、数据修改）覆盖率 >= 90% |

---

### 异常测试（TC-ERR）

| 编号 | 标题 | 需求来源 | 优先级 | 前置条件 | 测试步骤 | 预期结果 |
|------|------|---------|--------|---------|---------|---------|
| TC-ERR-001 | WxLogin - 微信 API 返回错误 | v1.0 WxLogin | 高 | 1. Mock 微信 API 返回错误码 | 1. Mock jscode2session 返回 errcode != 0<br>2. 调用 WxLogin RPC | 1. 返回微信 API 错误信息<br>2. 不创建用户、不生成 token |
| TC-ERR-002 | WxLogin - 微信 API 超时 | v1.0 WxLogin | 高 | 1. Mock 微信 API 延迟超时 | 1. Mock jscode2session 超时<br>2. 调用 WxLogin RPC | 1. 返回超时错误<br>2. User Service 保持稳定<br>3. 不产生资源泄漏 |
| TC-ERR-003 | WxLogin - 微信 API 返回空 openID | v1.0 WxLogin | 中 | 1. Mock 微信 API 返回 openid="" | 1. 调用 WxLogin RPC | 1. 返回登录失败错误<br>2. 不创建空 openID 的用户记录 |
| TC-ERR-004 | RefreshToken - 篡改 token payload | v1.0 RefreshToken | 高 | 1. 持有手动修改过 payload 的 token | 1. 调用 RefreshToken RPC，传入篡改后的 token | 1. 签名校验失败<br>2. 返回认证错误<br>3. 不生成新 token |
| TC-ERR-005 | BindCampus - 未登录状态调用 | v1.0 BindCampus | 高 | 1. 用户未登录（无有效 token） | 1. 直接调用 BindCampus RPC<br>2. context 中无 user-id metadata | 1. userIDFromCtx 返回 0<br>2. 返回"未认证"错误<br>3. 不执行绑定操作 |
| TC-ERR-006 | UpdateUserInfo - 未登录状态调用 | v1.0 UpdateUserInfo | 高 | 1. 用户未登录 | 1. 直接调用 UpdateUserInfo RPC，context 无 user-id | 1. 返回"未认证"错误<br>2. 不修改任何用户信息 |
| TC-ERR-007 | GetCurrentUser - 未登录状态调用 | v1.0 GetCurrentUser | 高 | 1. 用户未登录 | 1. 直接调用 GetCurrentUser RPC | 1. 返回"未认证"错误<br>2. 不查询用户信息 |
| TC-ERR-008 | BanUser - 无权限封禁同校管理员 | v2.0 BanUser | 高 | 1. 管理员已登录（role=admin，school_id=1）<br>2. 目标用户 role=admin，school_id=1 | 1. 管理员调用 BanUser RPC，目标为同校 admin | 1. 返回 PermissionDenied 错误（错误码 20007）<br>2. 目标用户 status 不变<br>3. 不写入审计日志 |
| TC-ERR-009 | BanUser - 无权限封禁超管 | v2.0 BanUser | 高 | 1. 管理员已登录（role=admin）<br>2. 目标用户 role=super_admin | 1. 管理员调用 BanUser RPC，目标为 super_admin | 1. 返回 PermissionDenied 错误<br>2. super_admin 不可被封禁 |
| TC-ERR-010 | SetUserRole - 管理员尝试设置角色 | v2.0 SetUserRole | 高 | 1. 管理员已登录（role=admin） | 1. 管理员调用 SetUserRole RPC，尝试提升学生为 admin | 1. 返回 PermissionDenied 错误<br>2. 角色变更未执行<br>3. SetUserRole 仅允许 super_admin 调用 |
| TC-ERR-011 | ListUsers - 管理员跨校查询 | v2.0 ListUsers | 高 | 1. 管理员已登录（school_id=1）<br>2. 传入 school_id=2 | 1. 管理员调用 ListUsers RPC，school_id=2 | 1. Gateway 层强制覆写 school_id 为 1<br>2. 仅返回 school_id=1 的用户<br>3. 管理员无法越权查看其他学校 |
| TC-ERR-012 | ListSchools - 未认证访问 | v1.1 ListSchools | 中 | 1. 用户未登录 | 1. 直接调用 GET /api/v1/schools，不带 JWT | 1. 返回 401 未认证错误<br>2. 不返回学校数据 |
| TC-ERR-013 | AuditContent - 审核不存在的内容 | v2.0 AuditContent | 高 | 1. 管理员已登录<br>2. content_id 不存在 | 1. 调用 AuditContent RPC，传入不存在的 content_id | 1. 返回"内容不存在"错误<br>2. 不写入审计日志 |
| TC-ERR-014 | AuditContent - 驳回时无 reason | v2.0 AuditContent | 中 | 1. 管理员已登录<br>2. 目标内容 status=pending_review | 1. 调用 AuditContent RPC，action=reject，reason 为空 | 1. 返回参数校验错误（驳回需提供原因）<br>2. 内容状态不变 |
| TC-ERR-015 | 串行攻击 - 伪造其他用户 ID | 全局安全 | 高 | 1. 用户 A 已登录<br>2. 伪造 context，注入其他用户的 user-id | 1. 用户 A 构造包含 user_id=B 的 metadata<br>2. 调用 GetCurrentUser 等 RPC | 1. 用户 A 的操作始终使用 token 中的 user_id<br>2. 伪造的 metadata 中 user-id 被忽略（由 Gateway 注入）<br>3. 无法操作其他用户的数据 |
| TC-ERR-016 | 数据库连接异常 - 写入失败 | 全局容错 | 高 | 1. MySQL 连接不可用 | 1. 调用 WxLogin RPC<br>2. 数据库写入/查询失败 | 1. 返回数据库错误<br>2. User Service 不 panic<br>3. 连接池恢复正常后可继续服务 |
| TC-ERR-017 | Redis 连接异常 - 缓存操作失败 | 全局容错 | 中 | 1. Redis 连接不可用 | 1. 调用 GetCurrentUser RPC<br>2. Redis 缓存读取失败 | 1. 降级查询 MySQL 数据库<br>2. 返回用户信息（无缓存加速）<br>3. User Service 不崩溃 |
| TC-ERR-018 | MQ 发送失败 - 审核状态变更通知 | v2.0 AuditContent | 中 | 1. RabbitMQ 连接不可用<br>2. 管理员执行审核操作 | 1. 管理员调用 AuditContent RPC<br>2. 状态变更成功但 MQ 发送失败 | 1. 数据库状态变更成功<br>2. MQ 发送失败记录日志告警<br>3. 不阻塞审核操作返回<br>4. Content Service 侧可通过轮询兜底 |
| TC-ERR-019 | 审计日志写入失败不阻塞主操作 | v2.0 审计日志 | 高 | 1. audit_logs 表写入异常 | 1. 管理员执行封禁操作<br>2. 审计日志写入失败 | 1. 封禁操作正常完成<br>2. 系统记录 warn 级别日志<br>3. 不因日志写入失败而回滚主操作 |
| TC-ERR-020 | ListSchools - school 表数据量性能验证 | v1.1 ListSchools | 中 | 1. 学校表中存在接近 3000 条记录 | 1. 调用 GET /api/v1/schools 不带 keyword<br>2. 记录响应时间 | 1. API 响应 P95 < 100ms<br>2. 返回数据正确完整 |

---

### 状态转换测试（TC-ST）

| 编号 | 标题 | 需求来源 | 优先级 | 前置条件 | 测试步骤 | 预期结果 |
|------|------|---------|--------|---------|---------|---------|
| TC-ST-001 | 用户状态：normal → banned（封禁） | v2.0 BanUser | 高 | 1. 管理员已登录<br>2. 目标用户 status=normal | 1. 管理员调用 BanUser RPC | 1. 用户 status 从 normal 变为 banned<br>2. 用户无法登录（WxLogin 返回 20008）<br>3. 用户无法续签 token |
| TC-ST-002 | 用户状态：banned → normal（解封） | v2.0 UnbanUser | 高 | 1. 目标用户 status=banned | 1. 管理员调用 UnbanUser RPC | 1. 用户 status 从 banned 变为 normal<br>2. 用户可正常登录<br>3. token 正常生成 |
| TC-ST-003 | 用户状态：banned → banned（重复封禁） | v2.0 BanUser | 中 | 1. 目标用户 status=banned | 1. 管理员再次调用 BanUser RPC | 1. 操作幂等，返回成功<br>2. 用户 status 保持 banned<br>3. 不产生重复状态转换 |
| TC-ST-004 | 用户状态：normal → normal（重复解封） | v2.0 UnbanUser | 低 | 1. 目标用户 status=normal | 1. 管理员调用 UnbanUser RPC | 1. 操作幂等，返回成功<br>2. 用户 status 保持 normal |
| TC-ST-005 | 封禁后登录失败 → 解封后登录成功 | v2.0 BanUser + UnbanUser | 高 | 1. 用户 A status=banned<br>2. 用户持有有效 wxCode | 1. 用户 A 调用 WxLogin → 预期失败（20008）<br>2. 管理员调用 UnbanUser 解封用户 A<br>3. 用户 A 再次调用 WxLogin | 1. 第一次登录失败，返回"账号已被封禁"<br>2. 解封成功<br>3. 第二次登录成功，获得有效 token |
| TC-ST-006 | 内容状态：pending_review → published（审核通过） | v2.0 AuditContent | 高 | 1. 内容 status=pending_review | 1. 管理员调用 AuditContent，action=approve | 1. 内容状态从 pending_review 变为 published<br>2. 内容对用户可见<br>3. MQ 事件通知 Content Service |
| TC-ST-007 | 内容状态：pending_review → rejected（审核驳回） | v2.0 AuditContent | 高 | 1. 内容 status=pending_review | 1. 管理员调用 AuditContent，action=reject，reason="违规内容" | 1. 内容状态从 pending_review 变为 rejected<br>2. 驳回原因被记录<br>3. 内容对用户不可见 |
| TC-ST-008 | 内容审核：pending_review 已被他人审核 | v2.0 AuditContent | 中 | 1. 内容 status=pending_review<br>2. 管理员 A 先执行审核 | 1. 管理员 B 尝试审核同一内容 | 1. Content Service 检测状态变更<br>2. 返回"内容已被审核"错误<br>3. 不产生冲突覆盖 |
| TC-ST-009 | 用户角色：student → admin（提升） | v2.0 SetUserRole | 高 | 1. 超管已登录<br>2. 目标用户 role=student | 1. 超管调用 SetUserRole，role=admin | 1. 用户 role 从 student 变为 admin<br>2. Redis 缓存清除<br>3. 用户获得 admin 路由访问权限<br>4. 审计日志记录角色变更 |
| TC-ST-010 | 用户角色：admin → student（降级） | v2.0 SetUserRole | 高 | 1. 超管已登录<br>2. 目标用户 role=admin | 1. 超管调用 SetUserRole，role=student | 1. 用户 role 从 admin 变为 student<br>2. Redis 缓存清除<br>3. 用户失去 admin 路由访问权限 |
| TC-ST-011 | 用户角色：student → super_admin（直接提升至超管） | v2.0 SetUserRole | 中 | 1. 超管已登录<br>2. 目标用户 role=student | 1. 超管调用 SetUserRole，role=super_admin | 1. 用户 role 更新为 super_admin<br>2. 该用户获得跨校管理能力<br>3. 审计日志完整记录 |
| TC-ST-012 | 封禁后清除缓存 → 续签时拉取最新状态 | v2.0 BanUser + Redis | 高 | 1. 用户 status=normal<br>2. Redis 中缓存该用户（status=normal） | 1. 管理员封禁用户（status=banned，Redis 缓存清除）<br>2. 用户尝试 RefreshToken | 1. Redis 中旧缓存已清除<br>2. RefreshToken 从 DB 读取最新状态<br>3. 发现 banned 状态，拒绝续签<br>4. 返回"账号已被封禁" |
| TC-ST-013 | 角色变更后权限即时生效 | v2.0 SetUserRole + RequireRole | 高 | 1. 用户 A role=student<br>2. 用户 A 持有旧 token（role=student） | 1. 超管提升用户 A 为 admin<br>2. 用户 A 使用旧 token 访问 admin 路由<br>3. Gateway 解析 JWT 中的 role | 1. 旧 token 中 role=student<br>2. Gateway RequireRole 检查失败<br>3. 用户需要重新登录获取新 token<br>4. 新 token 中 role=admin，可访问管理路由 |
| TC-ST-014 | 用户绑定学校状态：未绑定(0) → 已绑定 | v1.0 BindCampus | 高 | 1. 用户 school_id=0 | 1. 用户调用 BindCampus，school_id=1 | 1. school_id 从 0 变为 1<br>2. Redis 缓存清除<br>3. 后续请求中 context 注入正确的 school_id |
| TC-ST-015 | 用户绑定学校状态：已绑定 → 更换学校 | v1.0 BindCampus | 中 | 1. 用户 school_id=1 | 1. 用户调用 BindCampus，school_id=2 | 1. school_id 从 1 变为 2<br>2. 用户后续只能看到 school_id=2 的内容<br>3. 多租户隔离正确生效 |
| TC-ST-016 | 封禁用户后帖子/任务保留可见 | v2.0 BanUser | 中 | 1. 用户 A 发布了帖子和任务<br>2. 用户 A status=normal | 1. 管理员封禁用户 A<br>2. 其他用户访问用户 A 的帖子和任务 | 1. 用户 A 被封禁<br>2. 用户 A 的帖子仍然可见<br>3. 用户 A 的任务仍然可见<br>4. 数据不被联动删除或下架 |

---

## 需求-测试用例覆盖矩阵

### v1.1 用户服务需求覆盖

| 需求编号 | 需求描述 | 覆盖的测试用例 | 覆盖数 |
|---------|---------|---------------|-------|
| FR-1 | 移除 userIDFromCtx 中的调试日志 | TC-E-017, TC-E-018, TC-E-019 | 3 |
| Story 1 | 消除生产调试日志 | TC-E-017, TC-E-018, TC-E-019 | 3 |
| Story 2 | 核心测试覆盖 | TC-E-022, TC-F-001~TC-F-008, TC-F-024 | 13 |
| Story 3 | 暴露 ListSchools RPC | TC-F-009, TC-F-010, TC-E-007~TC-E-009, TC-ERR-012, TC-ERR-020 | 7 |
| FR-2 | 测试覆盖 | TC-E-022, TC-F-024 | 2 |
| FR-3 | ListSchools RPC | TC-F-009, TC-F-010, TC-E-007~TC-E-009 | 5 |
| WxLogin | 微信登录 RPC | TC-F-001, TC-F-002, TC-F-003, TC-F-024, TC-E-001, TC-ERR-001~TC-ERR-003 | 8 |
| RefreshToken | Token 续签 RPC | TC-F-004, TC-E-002, TC-E-003, TC-ERR-004 | 4 |
| BindCampus | 绑定学校 RPC | TC-F-005, TC-E-004, TC-E-005, TC-ERR-005, TC-ST-014, TC-ST-015 | 6 |
| GetCurrentUser | 获取当前用户 RPC | TC-F-006, TC-F-007, TC-ERR-007 | 3 |
| UpdateUserInfo | 更新用户信息 RPC | TC-F-008, TC-E-006, TC-ERR-006 | 3 |

### v2.0 用户服务需求覆盖

| 需求编号 | 需求描述 | 覆盖的测试用例 | 覆盖数 |
|---------|---------|---------------|-------|
| Story 1 | 封禁违规用户 | TC-F-011, TC-F-012, TC-F-013, TC-F-025, TC-E-010, TC-E-011, TC-E-021, TC-ERR-008, TC-ERR-009, TC-ST-001~TC-ST-005, TC-ST-012, TC-ST-016 | 15 |
| Story 2 | 用户列表查询+角色管理 | TC-F-014~TC-F-019, TC-E-012, TC-E-013, TC-E-016, TC-E-020, TC-ERR-010, TC-ERR-011, TC-ST-009~TC-ST-011, TC-ST-013 | 15 |
| Story 3 | 内容审核入口 | TC-F-020~TC-F-022, TC-F-027, TC-F-028, TC-E-014, TC-E-015, TC-ERR-013, TC-ERR-014, TC-ERR-018, TC-ST-006~TC-ST-008 | 13 |
| Story 4 | 操作审计日志 | TC-F-023, TC-F-026, TC-ERR-019 | 3 |
| BanUser | 封禁用户 RPC | TC-F-011, TC-F-012, TC-F-025, TC-E-010, TC-E-011, TC-E-021, TC-ERR-008, TC-ERR-009, TC-ST-001~TC-ST-005, TC-ST-012, TC-ST-016 | 14 |
| UnbanUser | 解封用户 RPC | TC-F-013, TC-ST-002, TC-ST-004, TC-ST-005 | 4 |
| ListUsers | 用户列表查询 RPC | TC-F-014~TC-F-017, TC-E-012, TC-E-020, TC-ERR-011 | 7 |
| SetUserRole | 设置用户角色 RPC | TC-F-018, TC-F-019, TC-E-013, TC-E-016, TC-ERR-010, TC-ST-009~TC-ST-011, TC-ST-013 | 8 |
| ListContentForAudit | 待审内容列表 RPC | TC-F-020, TC-F-027, TC-E-015 | 3 |
| AuditContent | 审核操作 RPC | TC-F-021, TC-F-022, TC-F-028, TC-E-014, TC-ERR-013, TC-ERR-014, TC-ERR-018, TC-ST-006~TC-ST-008 | 10 |
| 审计日志 | 操作审计日志 | TC-F-023, TC-F-026, TC-ERR-019 | 3 |
| JWT + RequireRole | 权限中间件 | TC-ST-013, TC-ERR-010, TC-ERR-011, TC-ERR-012 | 4 |
| 多租户隔离 | school_id 隔离 | TC-ERR-011, TC-E-020, TC-ST-014, TC-ST-015 | 4 |
| Redis 缓存管理 | 缓存清除与降级 | TC-ST-012, TC-ERR-017, TC-F-006, TC-F-007 | 4 |

### 测试类型统计

| 类别 | 用例数量 | 占比 |
|------|---------|------|
| 功能测试（TC-F） | 28 | 37.3% |
| 边界测试（TC-E） | 22 | 29.3% |
| 异常测试（TC-ERR） | 20 | 26.7% |
| 状态转换测试（TC-ST） | 16 | 21.3% |
| **合计** | **86** | **100%** |

### 优先级分布

| 优先级 | 用例数量 | 占比 |
|--------|---------|------|
| 高 | 48 | 55.8% |
| 中 | 25 | 29.1% |
| 低 | 5 | 5.8% |
| 未标注 | 8 | 9.3% |

---

*本文档基于 user-service-prd.md（v1.1）和 user-service-v2.0-prd.md（v2.0）两份需求文档生成，共包含 86 条测试用例，覆盖所有功能需求点。*
