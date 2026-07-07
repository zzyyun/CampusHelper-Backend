# Test Cases: AI 智能审核 + Content Service v3.0

## Overview
- **Feature**: AI 智能审核 (ai-moderation 微服务) + Content Service v3.0 集成
- **Requirements Source**: `docs/ai-moderation-content-service-v3.0-prd.md`
- **Test Coverage**: ai-moderation 服务、Content Service 同步集成、异步补判 Consumer、熔断降级、AI 审计日志、可观测性
- **Last Updated**: 2026-06-27
- **关联 Epic**: #89 — AI 智能审核 + Content Service v3.0

---

## 1. Functional Tests (功能测试)

### TC-F-001: 正常帖子 AI 同步通过 → 直接 published
- **Requirement**: Story 1 — 正常帖子 AI 秒级通过
- **Priority**: High
- **Preconditions**:
  - ai-moderation 服务正常运行（健康检查 OK）
  - Content Service 已连接 ai-moderation gRPC
  - 数据库 ai_audit_logs 表已创建
- **Test Steps**:
  1. 用户通过 POST /api/v1/posts 提交纯文本帖子（"校园里有人捡到钥匙"）
  2. DFA 敏感词扫描未命中
  3. Content Service 同步调用 ai-moderation.ModerateText（800ms timeout）
  4. AI 返回 result=PASS, confidence=0.95, categories=[]
- **Expected Results**:
  - 帖子状态 = `published`
  - 用户收到 200 响应，body 含 `ai_result: "pass"`, `ai_confidence: 0.95`
  - ai_audit_logs 新增 1 条记录（ai_result=0, fallback_used=false）
  - MQ 事件 `content.published` 发布
  - ES 索引在 5s 内可被搜索到
- **Postconditions**: 帖子立即可见，无人工干预

### TC-F-002: AI 同步拦截违规内容 → rejected
- **Requirement**: Story 2 — 违规内容 AI 同步拦截
- **Priority**: High
- **Preconditions**:
  - ai-moderation 服务正常
  - 测试数据集含明显违规样本（涉政/色情/广告）
- **Test Steps**:
  1. 用户提交含变体违规词的帖子（"加薇❤详谈"）
  2. DFA 未命中（变体字绕过）
  3. AI 推理返回 result=BLOCK, confidence=0.92, categories=["广告引流"]
- **Expected Results**:
  - 帖子状态 = `rejected`
  - 用户收到 400 响应，错误码 40001，含命中原因
  - ai_audit_logs 记录 ai_result=2
  - 发帖者收到通知"内容违规，已被拒绝"
- **Postconditions**: 帖子不入 ES 索引，不可见

### TC-F-003: AI 返回 review → 进入 pending_review 走人工
- **Requirement**: Story 3 — AI 不确定 → 进人工池
- **Priority**: High
- **Preconditions**:
  - ai-moderation 服务正常
  - 提交含疑似违规但 AI 置信度中等的内容
- **Test Steps**:
  1. 用户提交含政治敏感边缘词的帖子
  2. AI 返回 result=REVIEW, confidence=0.62, categories=["涉政疑似"]
  3. 管理员通过 User v2.0 `ListContentForAudit` 查看
- **Expected Results**:
  - 帖子状态 = `pending_review`
  - 用户看到"内容审核中"提示
  - 管理员后台可见此条，标记原因为"涉政疑似，置信度 0.62"
  - ai_audit_logs 记录 ai_result=1, ai_categories=["涉政疑似"]
- **Postconditions**: 帖子待人工裁决，期间不可见

### TC-F-004: 异步补判下架已发布帖子
- **Requirement**: Story 5 — 已发布帖子异步补判下架
- **Priority**: High
- **Preconditions**:
  - 帖子已 published 状态
  - 异步 Consumer 已启动并订阅 `ai.moderation.async_review`
- **Test Steps**:
  1. 定时调度器扫描近 7 天 published 帖子 → 发送 `ai.moderation.async_review` 消息
  2. Consumer 拉取消息，调 ai-moderation.ModerateText
  3. AI 返回 result=BLOCK, confidence=0.88
- **Expected Results**:
  - 帖子状态 = `taken_down`
  - MQ 事件 `content.taken_down` 发布
  - 发帖者收到通知"您的帖子因违规被下架"
  - ES 索引同步删除
  - ai_audit_logs 记录异步补判操作
- **Postconditions**: 帖子不可见，触发用户侧通知

### TC-F-005: ai-moderation 健康检查通过
- **Requirement**: Feature 1 — ai-moderation 微服务骨架
- **Priority**: High
- **Preconditions**:
  - ai-moderation 服务已启动
  - ONNX 模型文件存在且 SHA256 校验通过
- **Test Steps**:
  1. `grpcurl -plaintext localhost:50061 grpc.health.v1.Health/Check`
- **Expected Results**:
  - 返回 status=SERVING
  - 响应体含 model_loaded=true, model_version="v1.0"
- **Postconditions**: Content Service 可正常调用 AI 服务

### TC-F-006: ai-moderation 注册到 etcd
- **Requirement**: Feature 1 — 服务发现
- **Priority**: Medium
- **Preconditions**:
  - etcd 服务可用
  - ai-moderation 服务启动
- **Test Steps**:
  1. 启动 ai-moderation 服务
  2. `etcdctl get /services/ai-moderation --prefix`
- **Expected Results**:
  - etcd 中可见 `ai-moderation` 节点（含 IP:Port）
  - TTL 自动续期正常
- **Postconditions**: Content Service 可通过 etcd 发现 ai-moderation

### TC-F-007: ai_audit_logs 写入每条 AI 调用
- **Requirement**: Feature 5 — AI 审计日志表
- **Priority**: High
- **Preconditions**:
  - ai_audit_logs 表已创建
  - Content Service 已集成 AI 调用
- **Test Steps**:
  1. 发起 100 次发帖（50 正常 + 30 违规 + 20 边界）
  2. 检查 ai_audit_logs 表行数
- **Expected Results**:
  - 共 100 条记录
  - ai_result 分布：0(pass)=50, 1(review)=X, 2(block)=Y
  - latency_ms 字段均 > 0
  - fallback_used 字段均 = false（AI 服务正常）
- **Postconditions**: 所有 AI 调用可追溯

### TC-F-008: Prometheus metrics 暴露
- **Requirement**: Story 6 — AI 审核可观测性
- **Priority**: Medium
- **Preconditions**:
  - ai-moderation 服务启动
  - 端口 9091 已开放
- **Test Steps**:
  1. `curl http://localhost:9091/metrics`
  2. 发起 AI 调用后再查询
- **Expected Results**:
  - 可见 `ai_moderation_calls_total{result="pass"}` 等指标
  - 可见 `ai_moderation_latency_seconds` histogram
  - 可见 `ai_moderation_circuit_state` gauge
  - 计数器随调用次数增长
- **Postconditions**: 监控可抓取 metrics

### TC-F-009: Jaeger trace 全链路关联
- **Requirement**: Story 6 — 可观测性
- **Priority**: Medium
- **Preconditions**:
  - Jaeger 已部署
  - Content Service 与 ai-moderation 均开启 OTLP 上报
- **Test Steps**:
  1. 用户发起 1 次发帖请求（含 trace_id）
  2. 在 Jaeger UI 按 trace_id 查询
- **Expected Results**:
  - 可见完整 trace：`gateway → content.CreatePost → ai-moderation.ModerateText`
  - 父子 span 关系正确
  - AI span 显示 ai_result, latency_ms 标签
- **Postconditions**: 全链路追踪打通

### TC-F-010: 异步补判 pass 不动状态
- **Requirement**: Feature 3 — 异步补判 Consumer
- **Priority**: Medium
- **Preconditions**:
  - 帖子已 published
  - 异步 Consumer 运行中
- **Test Steps**:
  1. 异步 Consumer 处理 published 帖子
  2. AI 重新判断返回 result=PASS
- **Expected Results**:
  - 帖子状态保持 `published` 不变
  - ai_audit_logs 新增 1 条记录（标记为 async_review）
  - 不发送任何 MQ 事件
- **Postconditions**: 帖子正常可见

### TC-F-011: 异步补判 review 不动状态
- **Requirement**: Feature 3 — 异步补判 Consumer
- **Priority**: Medium
- **Preconditions**:
  - 帖子已 published
- **Test Steps**:
  1. AI 异步返回 result=REVIEW
- **Expected Results**:
  - 帖子状态保持 `published`（保守策略）
  - ai_audit_logs 记录 ai_result=1
  - 不动状态、不发 MQ
- **Postconditions**: 避免异步误杀已发布内容

### TC-F-012: User v2.0 audit_content_ai 操作类型记录
- **Requirement**: Feature 5 + User v2.0 集成
- **Priority**: Low
- **Preconditions**:
  - User v2.0 已上线
- **Test Steps**:
  1. 管理员调用 admin 接口触发 AI 操作
  2. 查询 admin_audit_logs
- **Expected Results**:
  - 可见 action="audit_content_ai" 的记录
  - operator_id, target_id, detail 字段正确填充
- **Postconditions**: AI 操作可追溯到具体管理员

---

## 2. Edge Case Tests (边界测试)

### TC-E-001: 输入超长文本截断到 512 token
- **Requirement**: Feature 1 — 模型输入限制
- **Priority**: High
- **Preconditions**:
  - ai-moderation 服务正常
- **Test Steps**:
  1. 提交 1500 字的纯文本帖子
  2. 观察 AI 处理结果
- **Expected Results**:
  - 文本被截断到 512 token 处理
  - ai_audit_logs 记录 actual_token_count=512
  - AI 决策基于截断后内容
  - 异步补判时重新评估完整文本（异步不限制）
- **Postconditions**: 长文本可正常处理

### TC-E-002: 空文本提交
- **Requirement**: Feature 2 — 边界处理
- **Priority**: Medium
- **Preconditions**:
  - 业务允许空文本帖子
- **Test Steps**:
  1. 用户提交 text="" 的帖子
- **Expected Results**:
  - Content Service 在 DFA 之前返回 40001"内容为空"
  - 不调用 AI 服务
- **Postconditions**: 无 ai_audit_logs 记录

### TC-E-003: AI 阈值边界 confidence=0.9 正好 pass
- **Requirement**: Feature 1 — 决策阈值
- **Priority**: High
- **Preconditions**:
  - AI 模型返回 confidence=0.90 边界值
- **Test Steps**:
  1. Mock AI 返回 confidence=0.90
  2. 提交帖子触发该判断
- **Expected Results**:
  - 决策为 PASS（≥ 0.9 阈值）
  - 帖子 status=published
  - ai_audit_logs 记录 confidence=0.90, ai_result=0
- **Postconditions**: 边界值准确处理

### TC-E-004: AI 阈值边界 confidence=0.5 正好 review
- **Requirement**: Feature 1 — 决策阈值
- **Priority**: High
- **Preconditions**:
  - Mock AI 返回 confidence=0.50
- **Test Steps**:
  1. 提交帖子触发该判断
- **Expected Results**:
  - 决策为 REVIEW（0.5-0.9 区间）
  - 帖子 status=pending_review
  - ai_audit_logs 记录 confidence=0.50, ai_result=1
- **Postconditions**: 边界值准确处理

### TC-E-005: AI 阈值边界 confidence=0.49 触发 block
- **Requirement**: Feature 1 — 决策阈值
- **Priority**: High
- **Preconditions**:
  - Mock AI 返回 confidence=0.49（明确违规但模型不太确定）
- **Test Steps**:
  1. 提交帖子触发该判断
- **Expected Results**:
  - 决策为 BLOCK（< 0.5 阈值）
  - 帖子 status=rejected
  - 用户收到违规通知
- **Postconditions**: 边界值准确处理

### TC-E-006: 文本含特殊字符（emoji + 控制字符）
- **Requirement**: Feature 1 — 文本预处理
- **Priority**: Medium
- **Preconditions**:
  - 提交含 emoji 🎉 和 \n\t 控制字符的文本
- **Test Steps**:
  1. 提交帖子："今天天气真好🎉\n\t心情不错"
  2. AI 处理
- **Expected Results**:
  - 文本预处理清洗控制字符
  - emoji 保留或按 tokenizer 规则处理
  - AI 返回正常 result
  - 不出现 tokenize 异常
- **Postconditions**: 特殊字符不导致 AI 调用失败

### TC-E-007: 极短文本（< 10 字）
- **Requirement**: Feature 1 — 短文本处理
- **Priority**: Medium
- **Test Steps**:
  1. 提交文本"在吗"
- **Expected Results**:
  - AI 正常处理
  - ai_audit_logs 记录 token_count 较小
  - 不出现模型异常
- **Postconditions**: 短文本可正常审核

### TC-E-008: 异步补判空队列
- **Requirement**: Feature 3 — Consumer 空闲态
- **Priority**: Low
- **Test Steps**:
  1. 启动 Consumer 但无消息
  2. 等待 60s
- **Expected Results**:
  - Consumer 持续运行不退出
  - metrics 显示 idle 状态
  - 无错误日志
- **Postconditions**: Consumer 长期运行稳定

### TC-E-009: 异步补判时帖子已被人工下架
- **Requirement**: Feature 3 — 跳过已处理帖子
- **Priority**: Medium
- **Test Steps**:
  1. 帖子状态从 published → rejected（人工操作）
  2. 异步 Consumer 拉到该帖子消息
- **Expected Results**:
  - Consumer 跳过该帖子（status != published）
  - 记录 DEBUG 日志
  - 不调用 AI 服务
  - 不重复发 MQ 事件
- **Postconditions**: 避免重复处理

### TC-E-010: 单实例 AI 并发 50 QPS
- **Requirement**: 性能约束 — 单实例 50 QPS
- **Priority**: High
- **Preconditions**:
  - ai-moderation 单实例
  - 压测工具 wrk / JMeter
- **Test Steps**:
  1. 用 wrk 发起 50 并发持续 60s
  2. 观察 P95 延迟与错误率
- **Expected Results**:
  - P95 ≤ 600ms
  - 错误率 < 0.1%
  - CPU 不超 80%
  - 无熔断触发
- **Postconditions**: 性能符合预期

### TC-E-011: 内容恰好 512 token 边界
- **Requirement**: Feature 1 — token 截断
- **Priority**: Medium
- **Test Steps**:
  1. 提交恰好 512 token 的文本
- **Expected Results**:
  - 完整处理，无截断
  - ai_audit_logs 记录 token_count=512
- **Postconditions**: 边界值无异常

### TC-E-012: AI 返回空 categories 列表
- **Requirement**: Feature 1 — 响应字段
- **Priority**: Low
- **Test Steps**:
  1. Mock AI 返回 categories=[]
- **Expected Results**:
  - 正常处理
  - ai_audit_logs 中 ai_categories=""
- **Postconditions**: 空数组不导致解析错误

### TC-E-013: trace_id 为空
- **Requirement**: Feature 1 — 透传字段
- **Priority**: Low
- **Test Steps**:
  1. 提交 trace_id="" 的请求
- **Expected Results**:
  - AI 服务仍正常返回
  - ai_audit_logs 中 trace_id 字段为空字符串
  - 不影响发帖主流程
- **Postconditions**: 缺失 trace_id 不阻塞

---

## 3. Error Handling Tests (错误处理)

### TC-ERR-001: AI 服务不可用 → fallback 到仅 DFA
- **Requirement**: Story 4 — AI 服务不可用降级
- **Priority**: Critical
- **Preconditions**:
  - ai-moderation 服务**停止运行**
- **Test Steps**:
  1. 停止 ai-moderation 服务
  2. 用户提交正常帖子
- **Expected Results**:
  - gRPC 连接失败 / Unavailable
  - Content Service 立即 fallback，不等待 800ms
  - DFA 未命中 → 帖子 status=published
  - ai_audit_logs 记录 fallback_used=true, ai_status=degraded
  - 用户发帖体验正常
  - 触发告警 MQ 事件 `ai.moderation.degraded`
- **Postconditions**: 系统仍能正常发帖

### TC-ERR-002: AI 调用超时（>800ms）
- **Requirement**: Story 4 — 同步超时
- **Priority**: Critical
- **Preconditions**:
  - ai-moderation 服务正常但推理慢
- **Test Steps**:
  1. Mock AI 推理耗时 1200ms
  2. 用户提交帖子
- **Expected Results**:
  - 800ms 后 gRPC DeadlineExceeded
  - Content Service 立即 fallback（不阻塞）
  - DFA 未命中 → 帖子 published
  - ai_audit_logs 记录 fallback_used=true
  - 用户响应延迟约 800ms（而非 1200ms）
- **Postconditions**: 同步超时不影响发帖

### TC-ERR-003: 连续 5 次失败触发熔断
- **Requirement**: Feature 4 — 熔断器
- **Priority**: Critical
- **Preconditions**:
  - ai-moderation 服务返回错误
- **Test Steps**:
  1. 连续发起 5 次 AI 调用，每次都失败
  2. 第 6 次发起调用
- **Expected Results**:
  - 前 5 次：gRPC 错误，fallback_used=true
  - 第 6 次：熔断器开启，**立即返回**（<10ms）fallback
  - Prometheus gauge `ai_moderation_circuit_state=1`
  - 日志记录熔断事件
- **Postconditions**: 熔断保护生效，避免雪崩

### TC-ERR-004: 熔断期间所有请求立即 fallback
- **Requirement**: Feature 4 — 熔断保护
- **Priority**: High
- **Test Steps**:
  1. 触发熔断（同 TC-ERR-003）
  2. 熔断期间发起 100 次 AI 调用
- **Expected Results**:
  - 所有 100 次调用均在 <10ms 内 fallback
  - 不调用 ai-moderation gRPC
  - 全部 fallback_used=true
- **Postconditions**: 熔断期间零依赖 AI 服务

### TC-ERR-005: 熔断半开试探成功 → 关闭
- **Requirement**: Feature 4 — 熔断恢复
- **Priority**: High
- **Preconditions**:
  - 熔断已开启
  - 等待 30s
  - 此时 AI 服务已恢复
- **Test Steps**:
  1. 等待熔断 Timeout(30s)
  2. 发起 1 次试探请求（Mock 返回成功）
- **Expected Results**:
  - 半开状态放行 1 个请求
  - AI 返回成功 → 熔断关闭
  - 后续调用正常调 AI
  - metrics 显示 circuit_state=0
- **Postconditions**: 自动恢复正常

### TC-ERR-006: 熔断半开试探失败 → 继续熔断
- **Requirement**: Feature 4 — 熔断恢复
- **Priority**: Medium
- **Test Steps**:
  1. 熔断开启 → 等待 30s
  2. 试探请求仍失败
- **Expected Results**:
  - 熔断继续开启（重新进入 30s 冷却）
  - 不影响后续 fallback 行为
- **Postconditions**: 熔断器正确处理失败探测

### TC-ERR-007: 模型文件缺失 → 启动失败
- **Requirement**: Feature 1 — 模型加载校验
- **Priority**: Critical
- **Preconditions**:
  - /models/moderation_v1.onnx 文件被删除
- **Test Steps**:
  1. 启动 ai-moderation 服务
- **Expected Results**:
  - 启动失败，退出码非 0
  - 日志记录"model file not found"
  - 健康检查返回 NOT_SERVING
  - etcd 中**不注册**服务（避免下游调用）
- **Postconditions**: 启动失败保护，避免不可用服务被调用

### TC-ERR-008: ONNX 推理异常 → 返回 fallback
- **Requirement**: Feature 1 — 模型容错
- **Priority**: High
- **Test Steps**:
  1. Mock ONNX runtime 返回错误
  2. 调用 AI
- **Expected Results**:
  - ai-moderation 捕获异常
  - 返回 response.fallback_used=true
  - Content Service 据此走 fallback 逻辑
  - ai_audit_logs 记录 model_error
- **Postconditions**: 推理异常不导致发帖失败

### TC-ERR-009: 异步补判重试 3 次进 DLQ
- **Requirement**: Feature 3 — 重试机制
- **Priority**: High
- **Test Steps**:
  1. 异步 Consumer 拉取消息
  2. AI 调用连续失败 3 次
- **Expected Results**:
  - 消息重试 3 次（指数退避）
  - 第 3 次失败后消息进 DLQ
  - 告警通知运维
  - 不阻塞其他消息处理
- **Postconditions**: 失败消息可追溯

### TC-ERR-010: ai_audit_logs 写入失败不阻塞发帖
- **Requirement**: Feature 5 — 审计容错
- **Priority**: High
- **Preconditions**:
  - 数据库临时不可用（如 kill 进程）
- **Test Steps**:
  1. AI 调用成功后，ai_audit_logs 写入失败
- **Expected Results**:
  - 帖子仍正常 published
  - WARN 日志记录"audit log write failed"
  - 帖子状态不受影响
  - 用户无感知
- **Postconditions**: 审计失败不影响业务

### TC-ERR-011: gRPC DeadlineExceeded 处理
- **Requirement**: Feature 1 — gRPC 错误处理
- **Priority**: Medium
- **Test Steps**:
  1. AI 调用超过 800ms
- **Expected Results**:
  - 捕获 codes.DeadlineExceeded
  - 走 fallback 路径
  - 不向上层返回 error
- **Postconditions**: 超时错误被优雅处理

### TC-ERR-012: AI 返回未知 result 值 → REVIEW 兜底
- **Requirement**: Feature 1 — 未知值容错
- **Priority**: High
- **Test Steps**:
  1. Mock AI 返回 result=99（未知枚举）
- **Expected Results**:
  - Content Service 视为 REVIEW（安全兜底）
  - 帖子 status=pending_review
  - ai_audit_logs 记录 ai_result=99 (raw)
  - WARN 日志"unknown ai result"
- **Postconditions**: 未知值不会直接放行

### TC-ERR-013: post_id 不存在（异常传参）
- **Requirement**: Feature 2 — 入参校验
- **Priority**: Medium
- **Test Steps**:
  1. 调用 ModerateText(post_id=-1)
- **Expected Results**:
  - 返回 InvalidArgument
  - 不写入 ai_audit_logs（无 post_id 关联）
- **Postconditions**: 异常入参被拒绝

### TC-ERR-014: 异步补判时 AI 服务再次不可用
- **Requirement**: Feature 3 — 重入队
- **Priority**: Medium
- **Test Steps**:
  1. 异步 Consumer 拉取消息
  2. AI 服务不可用
- **Expected Results**:
  - 消息重新入队，下次重试
  - 不丢失消息
  - 累计重试到 3 次进 DLQ
- **Postconditions**: 消息可靠性保证

### TC-ERR-015: 并发同 trace_id 写入 ai_audit_logs
- **Requirement**: Feature 5 — 并发安全
- **Priority**: Medium
- **Test Steps**:
  1. 同 trace_id 并发发起 10 次 AI 调用
- **Expected Results**:
  - ai_audit_logs 中 10 条独立记录
  - trace_id 字段一致
  - 主键 id 不冲突
- **Postconditions**: 并发写入安全

### TC-ERR-016: ai-moderation 重启导致熔断半开
- **Requirement**: Feature 4 — 重启场景
- **Priority**: Medium
- **Test Steps**:
  1. AI 服务重启
  2. Content Service 熔断器半开状态
- **Expected Results**:
  - 前 3 个请求强制走 fallback（避免雪崩）
  - 之后熔断器关闭，恢复正常
  - 日志记录"circuit half-open force fallback"
- **Postconditions**: 重启后快速恢复

---

## 4. State Transition Tests (状态转换)

### TC-ST-001: 发帖全流程 DFA → AI pass → published
- **Requirement**: Feature 2 — 同步审核流
- **Priority**: High
- **Preconditions**:
  - DFA 词库不含测试文本敏感词
  - AI 服务正常返回 PASS
- **Test Steps**:
  1. 用户提交正常帖子
  2. 观察状态机变化
- **Expected Results**:
  - 状态序列：draft (瞬态) → DFA pass → AI pass → **published**
  - MQ 事件 `content.published`
  - ai_audit_logs 记录 ai_result=0
  - ES 索引同步
- **Postconditions**: 帖子可被搜索

### TC-ST-002: 发帖全流程 DFA → AI review → pending_review
- **Requirement**: Feature 3 — 进入人工池
- **Priority**: High
- **Preconditions**:
  - AI 返回 REVIEW
- **Test Steps**:
  1. 用户提交疑似违规帖子
- **Expected Results**:
  - 状态序列：draft → DFA pass → AI review → **pending_review**
  - ai_audit_logs 记录 ai_result=1
  - 用户看到"审核中"
  - 管理员后台可见
- **Postconditions**: 等待人工裁决

### TC-ST-003: 发帖全流程 DFA → AI block → rejected
- **Requirement**: Feature 2 — 同步拦截
- **Priority**: High
- **Test Steps**:
  1. 用户提交明确违规帖子
- **Expected Results**:
  - 状态序列：draft → DFA pass → AI block → **rejected**
  - 用户收到违规通知
  - ai_audit_logs 记录 ai_result=2
- **Postconditions**: 帖子不可见

### TC-ST-004: 发帖全流程 DFA → AI 不可用 → fallback → published
- **Requirement**: Story 4 — 降级
- **Priority**: Critical
- **Preconditions**:
  - AI 服务完全不可用
  - DFA 未命中
- **Test Steps**:
  1. 停止 AI 服务
  2. 用户提交帖子
- **Expected Results**:
  - 状态序列：draft → DFA pass → AI fallback → **published**
  - ai_audit_logs 记录 fallback_used=true, ai_status=degraded
  - 用户发帖成功（无 AI 审核）
  - 告警事件触发
- **Postconditions**: 系统降级运行

### TC-ST-005: DFA 直接命中 → rejected（不调 AI）
- **Requirement**: Feature 2 — DFA 优先级
- **Priority**: High
- **Preconditions**:
  - 帖子含 DFA 词库敏感词
- **Test Steps**:
  1. 用户提交含敏感词的帖子
- **Expected Results**:
  - 状态序列：draft → DFA block → **rejected**（不进入 AI）
  - 用户收到 DFA 命中词列表
  - ai_audit_logs **无新增记录**（未调 AI）
  - 性能: < 100ms 响应（不调 AI）
- **Postconditions**: DFA 拦截优先

### TC-ST-006: 异步补判 published → taken_down
- **Requirement**: Feature 3 — 异步下架
- **Priority**: High
- **Preconditions**:
  - 帖子 status=published
- **Test Steps**:
  1. 异步 Consumer 处理
  2. AI 返回 BLOCK
- **Expected Results**:
  - 状态序列：**published → taken_down**
  - MQ 事件 `content.taken_down`
  - ES 索引删除
  - 用户收到下架通知
  - ai_audit_logs 新增 1 条（标记 async_review）
- **Postconditions**: 帖子不可见

### TC-ST-007: 异步补判 pass 状态保持 published
- **Requirement**: Feature 3 — 保守策略
- **Priority**: Medium
- **Test Steps**:
  1. AI 返回 PASS
- **Expected Results**:
  - 状态序列：published → **published**（不变）
  - ai_audit_logs 记录 ai_result=0
  - 无 MQ 事件
- **Postconditions**: 帖子正常

### TC-ST-008: 异步补判 review 状态保持 published
- **Requirement**: Feature 3 — 保守策略
- **Priority**: Medium
- **Test Steps**:
  1. AI 返回 REVIEW（异步）
- **Expected Results**:
  - 状态序列：published → **published**（保守不动）
  - ai_audit_logs 记录 ai_result=1
  - 无 MQ 事件
- **Postconditions**: 避免误杀

### TC-ST-009: 异步补判时帖子已被 taken_down → 跳过
- **Requirement**: Feature 3 — 跳过已处理
- **Priority**: Medium
- **Test Steps**:
  1. 帖子状态已被管理员改为 taken_down
  2. Consumer 处理
- **Expected Results**:
  - 不调用 AI
  - 不改状态
  - DEBUG 日志记录"already taken_down"
- **Postconditions**: 幂等处理

### TC-ST-010: pending_review → published/rejected（人工操作）
- **Requirement**: User v2.0 集成 — 管理员裁决
- **Priority**: Medium
- **Preconditions**:
  - 帖子 status=pending_review（AI review 后）
- **Test Steps**:
  1. 管理员通过 User v2.0 AuditContent RPC 操作
  2. action=approve 或 reject
- **Expected Results**:
  - approve: pending_review → **published** + MQ + ES
  - reject: pending_review → **rejected** + reason 记录
  - ai_audit_logs 记录人工裁决（关联 operator_id）
  - admin_audit_logs 记录操作
- **Postconditions**: 人工裁决完成

### TC-ST-011: 熔断状态机 closed → open → half-open → closed
- **Requirement**: Feature 4 — 熔断状态机
- **Priority**: High
- **Preconditions**:
  - AI 服务从正常到故障到恢复
- **Test Steps**:
  1. 阶段1: AI 正常，状态 closed
  2. 阶段2: AI 连续 5 次失败，状态 open
  3. 阶段3: 等待 30s，状态 half-open
  4. 阶段4: 试探成功，状态 closed
- **Expected Results**:
  - 状态转换准确
  - 各状态行为符合预期
  - Prometheus gauge 正确反映状态
- **Postconditions**: 熔断器正常状态机

### TC-ST-012: AI 决策状态机 PASS / REVIEW / BLOCK 完整路径
- **Requirement**: Feature 1 — 决策状态机
- **Priority**: High
- **Test Steps**:
  1. 对同一帖子在不同置信度下测试
- **Expected Results**:
  | confidence | 决策 | 帖子状态 |
  |-----------|------|---------|
  | ≥ 0.9 | PASS | published |
  | 0.5 ≤ x < 0.9 | REVIEW | pending_review |
  | < 0.5 | BLOCK | rejected |
- **Postconditions**: 三种状态正确流转

---

## 5. Integration Tests (集成测试)

### TC-I-001: 端到端发帖 → AI → ES 同步全链路
- **Requirement**: MVP 完整链路
- **Priority**: High
- **Preconditions**:
  - 完整环境：gateway + content + ai-moderation + MySQL + Redis + ES + RabbitMQ + Jaeger
- **Test Steps**:
  1. 用户通过小程序发起发帖
  2. Gateway 鉴权 → Content Service → AI Service → MQ → ES
- **Expected Results**:
  - 全链路 trace_id 一致
  - Jaeger 显示完整 span 树
  - 帖子最终可被 ES 搜索
  - 各环节无错误
- **Postconditions**: 链路打通

### TC-I-002: 端到端异步补判 → 下架 → MQ 通知
- **Requirement**: 异步流
- **Priority**: High
- **Test Steps**:
  1. 帖子已 published
  2. 异步 Consumer 检测到违规
  3. taken_down + MQ → Message Service（mock）通知用户
- **Expected Results**:
  - 帖子下架
  - 用户收到通知
  - ES 删除
- **Postconditions**: 异步链路打通

### TC-I-003: Content Service 重启后熔断器行为
- **Requirement**: 重启场景
- **Priority**: Medium
- **Test Steps**:
  1. Content Service 熔断半开状态时重启
  2. 重启后前几个请求
- **Expected Results**:
  - 熔断器状态重置为 closed
  - 前 3 个请求强制 fallback（避免雪崩）
  - 之后恢复正常
- **Postconditions**: 重启后稳定

### TC-I-004: ai-moderation 多实例负载均衡
- **Requirement**: 未来扩展（v3.x）
- **Priority**: Low
- **Preconditions**:
  - 启动 2 个 ai-moderation 实例
  - etcd 注册多节点
- **Test Steps**:
  1. 发起 100 次 AI 调用
- **Expected Results**:
  - 2 实例负载均衡
  - 调用均匀分布
- **Postconditions**: 支持水平扩展

---

## 6. Security Tests (安全测试)

### TC-SEC-001: 模型文件 SHA256 校验失败 → 启动失败
- **Requirement**: 安全启动
- **Priority**: High
- **Preconditions**:
  - 模型文件 hash 与配置不符
- **Test Steps**:
  1. 修改模型文件一个字节
  2. 启动 ai-moderation
- **Expected Results**:
  - 启动失败
  - 日志记录"model hash mismatch"
  - 退出码非 0
- **Postconditions**: 防止模型被篡改

### TC-SEC-002: trace_id 端到端透传
- **Requirement**: 可观测性 + 安全审计
- **Priority**: High
- **Preconditions**:
  - Jaeger 已部署
- **Test Steps**:
  1. Gateway 注入 trace_id
  2. 检查 ai_audit_logs 中 trace_id 字段
- **Expected Results**:
  - trace_id 从 gateway → content → ai-moderation 全程一致
  - ai_audit_logs.trace_id 正确
  - Jaeger 中可关联所有 span
- **Postconditions**: 完整审计链路

### TC-SEC-003: ai_audit_logs 不暴露原始敏感文本
- **Requirement**: 数据隐私
- **Priority**: High
- **Preconditions**:
  - ai_audit_logs 表设计
- **Test Steps**:
  1. 检查 ai_audit_logs 表结构
  2. 验证是否有 text 列
- **Expected Results**:
  - 仅记录 content_hash (SHA256)，不存原始文本
  - 隐私符合 GDPR / 国内法规
- **Postconditions**: 隐私保护

### TC-SEC-004: gRPC 接口仅内网调用
- **Requirement**: 网络安全
- **Priority**: High
- **Preconditions**:
  - ai-moderation 监听 :50061
- **Test Steps**:
  1. 从公网 IP 尝试连接
- **Expected Results**:
  - 连接被防火墙拒绝
  - 仅内网/etcd 发现的服务可调用
- **Postconditions**: 防止外部滥用

### TC-SEC-005: AI 返回 categories 不含可执行内容
- **Requirement**: 响应安全
- **Priority**: Low
- **Test Steps**:
  1. Mock AI 返回 categories 含 HTML/JS 字符串
- **Expected Results**:
  - Content Service 视 categories 为纯文本
  - 不解析 HTML/JS
  - 前端展示时转义
- **Postconditions**: 防 XSS

---

## 7. Performance Tests (性能测试)

### TC-PERF-001: 发帖同步路径 P95 ≤ 1s
- **Requirement**: 性能约束
- **Priority**: High
- **Preconditions**:
  - AI 服务正常
  - DFA 词库加载完毕
- **Test Steps**:
  1. wrk 压测 50 并发持续 60s
  2. 收集 P95 延迟
- **Expected Results**:
  - P95 ≤ 1000ms（DFA < 100ms + AI < 800ms + 业务开销）
- **Postconditions**: 满足性能指标

### TC-PERF-002: ai-moderation 单次推理 P95 ≤ 600ms
- **Requirement**: 性能约束
- **Priority**: High
- **Test Steps**:
  1. 单次调用 1000 次取 P95
- **Expected Results**:
  - P95 ≤ 600ms
  - 平均 ≤ 300ms
- **Postconditions**: 模型推理性能达标

### TC-PERF-003: ai-moderation 启动时间 ≤ 10s
- **Requirement**: 性能约束
- **Priority**: Medium
- **Test Steps**:
  1. 冷启动（首次）计时
- **Expected Results**:
  - 启动时间 ≤ 10s
  - 模型加载是主要耗时
- **Postconditions**: 启动性能满足运维要求

---

## Test Coverage Matrix (需求覆盖矩阵)

| 需求 ID | 需求描述 | 测试用例 | 覆盖率 |
|---------|---------|---------|--------|
| REQ-F-1 | ai-moderation 微服务骨架 | TC-F-005, TC-F-006 | ✅ Complete |
| REQ-F-2 | ONNX 模型加载与推理 | TC-F-007, TC-E-001, TC-E-006, TC-E-007, TC-ERR-007, TC-ERR-008, TC-SEC-001 | ✅ Complete |
| REQ-F-3 | Content 同步 AI 集成 | TC-F-001, TC-F-002, TC-F-003, TC-ST-001~005, TC-ERR-012 | ✅ Complete |
| REQ-F-4 | 客户端熔断器 | TC-ERR-003, TC-ERR-004, TC-ERR-005, TC-ERR-006, TC-ERR-016, TC-ST-011 | ✅ Complete |
| REQ-F-5 | 异步补判 Consumer | TC-F-004, TC-F-010, TC-F-011, TC-E-009, TC-ERR-009, TC-ERR-014, TC-ST-006~009 | ✅ Complete |
| REQ-F-6 | AI 审计日志 | TC-F-007, TC-F-012, TC-ERR-010, TC-ERR-015, TC-SEC-003 | ✅ Complete |
| REQ-F-7 | 可观测性 | TC-F-008, TC-F-009, TC-SEC-002 | ✅ Complete |
| REQ-S-1 | Story 1: AI 秒级通过 | TC-F-001, TC-ST-001, TC-PERF-001 | ✅ Complete |
| REQ-S-2 | Story 2: AI 拦截违规 | TC-F-002, TC-ST-003 | ✅ Complete |
| REQ-S-3 | Story 3: AI 不确定进人工池 | TC-F-003, TC-ST-002 | ✅ Complete |
| REQ-S-4 | Story 4: AI 不可用降级 | TC-ERR-001, TC-ERR-002, TC-ST-004 | ✅ Complete |
| REQ-S-5 | Story 5: 异步补判下架 | TC-F-004, TC-ST-006, TC-I-002 | ✅ Complete |
| REQ-S-6 | Story 6: 可观测性 | TC-F-007, TC-F-008, TC-F-009, TC-SEC-002 | ✅ Complete |

---

## Notes

### 关键假设
1. **AI 模型选型**：本期使用 HFL Chinese-BERT-wwm + 自训分类头，具体模型在 issue #93 中确认
2. **模型文件大小**：~100MB，独立 volume 挂载（不入 Docker 镜像）
3. **AI 决策阈值**：≥0.9 pass / 0.5-0.9 review / <0.5 block（可在 config.yaml 调整）
4. **同步超时**：800ms 硬超时
5. **熔断配置**：30s 窗口，连续 5 次失败触发
6. **异步补判触发频率**：每天扫描近 7 天 published 帖子

### 已知限制
- 本期**不包含**：评论 AI 审核、图片 AI 审核、链接检测、用户申诉流程
- 训练数据 / 模型微调不在本期范围
- AI 管理后台（命中率/误判率看板）留 v3.x

### 待定项（实现前需 review）
- ONNX 模型具体选型与 license
- 模型文件存储位置（volume vs 镜像 COPY）
- AI 通信加密（内网明文 vs mTLS）
- ai_audit_logs 保留天数（180 天默认）
- 异步补判调度策略（每日定时 vs 实时队列）

### 测试执行优先级建议
1. **P0**（必须）：TC-F-001/002/003, TC-ERR-001/002/003, TC-ST-001/002/003/004, TC-I-001
2. **P1**（重要）：TC-F-004/005/007, TC-E-001/003/004/005, TC-ERR-004/005/007/008/009/012, TC-ST-005/006, TC-PERF-001/002
3. **P2**（建议）：其余用例

### 关联 Issue 列表
- Epic: #89 — AI 智能审核 + Content Service v3.0
- #90 — ai-moderation 微服务骨架
- #91 — PB/ai_moderation.proto 定义
- #92 — ai_audit_logs 表 + GORM Model + DAO
- #93 — ONNX 模型加载与本地推理
- #94 — Content Service 同步 AI 调用集成
- #95 — 客户端熔断器
- #96 — Content Service 异步补判 Consumer
- #97 — AI 审计日志写入 + 可观测性

---

*This test suite provides comprehensive coverage across functional, edge case, error handling, state transition, integration, security, and performance dimensions. Total: 70+ test cases mapped to PRD requirements and Story-level acceptance criteria.*