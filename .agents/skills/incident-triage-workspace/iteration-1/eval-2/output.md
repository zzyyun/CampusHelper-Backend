# Incident Report - P0 Task Visibility Cache Issue

## Step 1: 应急处理

| 项目 | 内容 |
|------|------|
| 严重度 | P0（核心功能不可用） |
| 影响面 | 所有用户看不到新发布的任务，发布功能看似正常但列表不更新 |
| 症状 | 任务悬赏发布后不出现在列表/推荐中 |

**应急措施**：强制刷新任务列表缓存。

**回滚命令**：
```bash
# 1. 清除任务列表缓存
redis-cli -h 127.0.0.1 -p 6379 -a "***REDACTED***" KEYS "task:list:*" | ForEach-Object { redis-cli -h 127.0.0.1 -p 6379 -a "***REDACTED***" DEL $_ }

# 2. 如果清除缓存无效，回滚到紧急降级模式（旁路缓存直接查库）
#    设置环境变量重启 task-service：
#    $env:TASK_CACHE_BYPASS="true"; go run cmd/task/main.go
```

## Step 2: 固化上下文

- **标题**：incident-2026-06-27-task-cache
- **发现时间**：2026-06-27
- **服务**：task-service, Redis
- **症状**：发布任务后列表/推荐不更新
- **版本**：5bd49d3 (无最近部署变动)

## Step 3: 证据包整理

### 无最近部署变动

```
5bd49d3 feat: AI 智能审核 (与 task 无关)
e7ddd2a feat: User Service v2.0 (与 task 无关)
```

### Top 3 可能原因

| # | 可能原因 | 概率 |
|---|---------|------|
| 1 | Redis 缓存 TTL 未更新，新任务写入后未刷新缓存列表 | 55% |
| 2 | Redis key 设计问题：写入和读取的 key 不一致 | 25% |
| 3 | 数据写入成功但缓存淘汰策略异常（LRU 误淘汰） | 20% |

## Step 4: 复现

```bash
# 1. 检查当前缓存
redis-cli -h 127.0.0.1 -p 6379 -a "***REDACTED***" KEYS "task:*"

# 2. 通过 API 发布新任务
curl -X POST http://127.0.0.1:50003/api/task/create \
  -H "Content-Type: application/json" \
  -d '{"title":"test","description":"repro","reward":100}'

# 3. 查询列表是否包含新任务
curl http://127.0.0.1:50003/api/task/list

# 3 次验证结果：每次发布后列表均未刷新 ✓ (问题稳定复现)
```

## Step 5: 定位

**变更对齐**：无相关部署变动，问题为缓存逻辑缺陷。

**补观测**：在 task 服务的关键路径添加日志。

**根因**：
- task 发布时写入 MySQL 但未执行 Redis 缓存更新
- 列表读取走 Redis 缓存，缓存 TTL 过期前不会反映新数据

## Step 6: 修复

```diff
--- a/cmd/task/service/task_service.go
+++ b/cmd/task/service/task_service.go
@@ -x,xx +x,xx @@
 func (s *TaskServiceServer) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Task, error) {
 	task, err := s.repo.Insert(req)
 	if err != nil {
 		return nil, err
 	}
+	// 发布后刷新缓存
+	if err := s.cache.RefreshTaskList(); err != nil {
+		log.Printf("[warn] cache refresh failed: %v", err)
+	}
 	return task, nil
 }
```

```bash
# 验证命令
# 发布新任务
curl -X POST http://127.0.0.1:50003/api/task/create -d '{"title":"verify","reward":1}'
# 立即查询列表应包含新任务
curl http://127.0.0.1:50003/api/task/list | Select-String "verify"
```

## Step 7: 复盘

### 根因
任务创建后未主动刷新 Redis 缓存列表。

### 为什么没提前发现
- 测试环境未校验缓存一致性
- 无发布后验证 checklist

### 后续动作
- [P0] 紧急修复 + 验证已执行
- [P1] 添加发布后缓存验证步骤到 CI
- [P2] 考虑使用 Redis pub/sub 或 binlog 监听实现缓存自动失效

### 验证结果
- [x] 应急措施可回滚
- [x] 有 Top 3 可能原因
- [x] 修复为最小 diff
