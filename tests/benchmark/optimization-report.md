# CampusHelper 高并发优化效果对比报告

> 更新日期：2026-07-08
> 对比基准：task-139 基线报告

---

## 1. 优化项汇总

| 任务 | 优化内容 | 文件变更 |
|------|----------|----------|
| task-140 | Redis 多级缓存 | `pkg/cache/cache.go`, `cmd/content/database/post_cache.go`, `cmd/task/repo/task_cache.go` |
| task-141 | API 限流降级 | `cmd/gateway/middleware/ratelimit.go`, `cmd/gateway/middleware/degradation.go` |
| task-143 | 数据库连接池优化 | `pkg/db/mysql.go`, `config/config.go` |

---

## 2. 预期性能提升（理论分析）

### 2.1 task-140：Redis 多级缓存

| 接口 | 优化前 QPS | 预期提升 | 优化后 QPS | 依据 |
|------|-----------|----------|-----------|------|
| 帖子详情 `GET /content/posts/:id` | ~300 (gRPC+MySQL) | **5~10x** | 1,500~3,000 | Cache-Aside 命中时绕过 gRPC+MySQL，延迟从 ~50ms 降至 ~5ms |
| 任务详情 `GET /tasks/:id` | ~300 | **5~10x** | 1,500~3,000 | 同上 |
| P99 延迟（读接口） | ~200ms | **降低 60~80%** | ~40ms | Redis GET <2ms + singleflight 合并并发请求 |
| 缓存穿透 | 无防护 | **完全消除** | — | 空值哨兵阻止不存在 ID 的反复 DB 查询 |
| 缓存击穿 | 无防护 | **完全消除** | — | singleflight 确保同 key 并发只放一个请求到 DB |

**Cache 命中率预估（冷启动后稳态）**：
- 帖子详情：85%~95%（读多写少，10min TTL）
- 任务详情：80%~90%（任务状态变更频率低）

### 2.2 task-141：API 限流降级

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 恶意流量防护 | 无（直接穿透到后端） | IP 级令牌桶，超限返回429 |
| HTTP 状态码 | 200（错误码在 body 内） | **429 + Retry-After 头** |
| 内存泄漏 | `buckets` map 无限增长 | 后台 goroutine 5分钟清理过期桶 |
| 服务故障处理 | 全部502/504 | **熔断降级503**（5次失败触发30s降级窗口） |
| 用户体验 | 无提示 | `Retry-After` 头告知客户端重试时间 |

### 2.3 task-143：数据库连接池优化

| 参数 | 优化前 | 优化后 | 影响 |
|------|--------|--------|------|
| MaxIdleConns | 10 | **25** | 减少建连开销，空闲连接预热 |
| MaxOpenConns | 100 | **200** | 提升并发承载能力 |
| ConnMaxIdleTime | 未设置 | **600s** | 及时释放空闲连接，降低 RDS 负载 |
| 慢查询监控 | 无 | **>200ms 自动记录** | 及时发现性能退化 |
| 配置方式 | 硬编码 | **yaml 可配置** | 运行时无需改代码 |

---

## 3. 实测对比数据（待填充）

> ⚠️ 需在本地/测试环境启动全部服务后，运行以下命令填充真实数据：

```bash
# 优化后压测（需启动服务）
./tests/benchmark/run-benchmark.sh \
  --url http://localhost:8080 \
  --token "your-jwt-token" \
  --out tests/benchmark/optimization-post-report.md
```

### 3.1 帖子详情对比（预期）

| 指标 | 优化前基线 | 优化后（预期） | 提升 |
|------|-----------|-------------|------|
| QPS (C=100) | - | - | - |
| P50 (ms) | - | - | - |
| P90 (ms) | - | - | - |
| P99 (ms) | - | - | - |

### 3.2 任务详情对比（预期）

| 指标 | 优化前基线 | 优化后（预期） | 提升 |
|------|-----------|-------------|------|
| QPS (C=100) | - | - | - |
| P50 (ms) | - | - | - |
| P90 (ms) | - | - | - |
| P99 (ms) | - | - | - |

### 3.3 限流效果验证

```bash
# 快速发100请求验证429触发
for i in $(seq 1 100); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)
  [ "$code" = "429" ] && echo "429 triggered at request #$i" && break
done
```

---

## 4. 运维要点

| 场景 | 操作 |
|------|------|
| Redis 不可用 | 服务自动降级到纯 MySQL（无缓存层，日志告警） |
| 缓存数据不一致 | 写操作自动失效缓存（InvalidatePostCache/InvalidateTaskCache） |
| 限流误触发 | 调整 `config/my_config.yaml` 中 `gateway.rateLimit` 和 `gateway.rateBurst` |
| 熔断器恢复 | 30s 后自动进入半开态探测，连续3次成功恢复正常 |
| 慢查询排查 | 查看日志中 `[SLOW_QUERY]` 前缀，参照 docs/db-performance-report.md 优化索引 |

---

## 5. 关联文档

| 文档 | 路径 | 用途 |
|------|------|------|
| 基线报告 | `tests/benchmark/baseline-report.md` | 优化前性能基线 |
| DB 性能报告 | `docs/db-performance-report.md` | 连接池调优 + 慢查询 Top5 + 建议索引 |
| 本报告 | `tests/benchmark/optimization-report.md` | 优化效果对比 |
