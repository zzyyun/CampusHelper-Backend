# CampusHelper 高并发运维手册

> 适用版本：v1.0+ | 更新日期：2026-07-08

---

## 1. 架构概览

```
客户端 → Gin Gateway → gRPC 内部调用 → [User/Content/Task/Message/File] 服务
                                    ↘ MySQL (GORM) + Redis (缓存) + ES (搜索)
                                    ↘ RabbitMQ (异步事件)
```

---

## 2. 配置说明

### 2.1 连接池配置（`config/my_config.yaml`）

```yaml
mysql:
  # 数据库连接（各服务独立库）
  host: "127.0.0.1"
  port: "3306"
  # 连接池参数（4C8G 推荐值，高配机器可适当放大）
  pool:
    maxIdleConns: 25      # 空闲连接数（保持预热，减少建连开销）
    maxOpenConns: 200     # 最大连接数（RDS max_connections/服务数）
    connMaxLifetime: 3600 # 连接最大存活时间（秒）
    connMaxIdleTime: 600  # 空闲连接最大存活时间（秒，及时释放）

gateway:
  rateLimit: 100   # 每秒每个 IP 的请求数
  rateBurst: 200   # 令牌桶突发容量
```

### 2.2 Redis 配置

```yaml
redis:
  address: "127.0.0.1:6379"
  password: ""  # 生产环境务必设置密码
```

**Redis 不可用时降级策略**：各服务自动跳过缓存层，直接查询 MySQL（日志输出 `[WARN] Redis 连接失败（缓存降级）`）

---

## 3. 运行时监控

### 3.1 日志关键字速查

| 日志前缀 | 含义 | 响应动作 |
|----------|------|----------|
| `[SLOW_QUERY]` | 执行超过 200ms 的 SQL | 分析 SQL，检查索引是否命中 |
| `[ratelimit] 清理过期桶` | IP 桶清理正常运行 | 无需处理（信息性日志） |
| `[degradation] 熔断器打开` | 某服务连续失败触发降级 | 检查对应服务健康状态 |
| `[degradation] 熔断器恢复` | 服务恢复正常 | 确认服务稳定，无需处理 |
| `[db] xxx 连接池` | 服务启动时输出连接池参数 | 确认参数符合预期 |

### 3.2 关键指标

**连接池健康（GORM sql.DB.Stats）**：

| 指标 | 正常范围 | 告警阈值 | 说明 |
|------|----------|----------|------|
| InUse / MaxOpen | < 50% | > 80% | 连接池即将耗尽 |
| WaitCount 增速 | < 1/s | > 10/s | 请求排队严重 |
| Idle | > 5 | = 0 | 无空闲连接 |

**缓存命中率（Redis INFO keyspace）**：

```bash
# 查看 Redis 命中率
redis-cli INFO stats | grep keyspace
# keyspace_hits / (keyspace_hits + keyspace_misses) 应 > 0.8
```

---

## 4. 常见故障处理

### 4.1 响应变慢（P99 > 500ms）

```bash
# 1. 检查慢查询日志
grep "SLOW_QUERY" /var/log/campus-helper/*.log | tail -20

# 2. 检查连接池使用率
grep "连接池" /var/log/campus-helper/*.log | tail -5

# 3. 检查 Redis 是否可达
redis-cli PING

# 4. 检查 ES 是否正常
curl -s http://localhost:9200/_cluster/health
```

### 4.2 大量 429 响应

```bash
# 查看限流触发频率
grep "429\|ErrRateLimited" /var/log/campus-helper/*.log | wc -l

# 调整限流参数（临时放宽）
# 修改 config/my_config.yaml:
#   gateway.rateLimit: 200    # 从100提到200
#   gateway.rateBurst: 400    # 从200提到400
```

### 4.3 服务降级（503 响应）

```bash
# 检查降级中间件日志
grep "degradation" /var/log/campus-helper/*.log

# 手动重置熔断器（需调用管理接口或重启 Gateway）
```

### 4.4 Redis 缓存数据不一致

**症状**：更新数据后，读取仍返回旧数据

**原因**：写操作后缓存失效失败（Redis 暂时不可用）

**处理**：
```bash
# 手动清除指定服务的缓存（紧急）
redis-cli KEYS "post:school:*" | xargs redis-cli DEL
redis-cli KEYS "task:school:*" | xargs redis-cli DEL
```

> 正常情况下，写操作已自动调用 `InvalidatePostCache` / `InvalidateTaskCache` 失效缓存。

---

## 5. 性能压测

### 5.1 基准压测

```bash
# 无认证接口
./tests/benchmark/run-benchmark.sh --url http://localhost:8080

# 全接口（需 JWT）
./tests/benchmark/run-benchmark.sh \
  --url http://localhost:8080 \
  --token "eyJhbGciOiJIUzI1NiIs..."
```

### 5.2 建议压测频率

| 场景 | 频率 | 输出 |
|------|------|------|
| 代码变更（连接池/缓存相关） | 每次 PR | 对比基线 |
| 季度性能回顾 | 每季度 | 更新基线 |
| 疑似性能退化 | 随时 | 问题定位报告 |

---

## 6. 索引维护

### 6.1 建议索引（参照 db-performance-report.md）

```sql
-- Content 服务：帖子列表查询
CREATE INDEX idx_posts_list ON posts (school_id, status, type, id DESC);

-- Content 服务：评论列表查询
CREATE INDEX idx_comments_list ON post_comments (post_id, parent_id, status);

-- Task 服务：任务过期扫描
CREATE INDEX idx_tasks_expire ON tasks (status, expired_at);

-- Task 服务：列表查询
CREATE INDEX idx_tasks_list ON tasks (school_id, status, expired_at, created_at);
```

### 6.2 索引生效验证

```sql
EXPLAIN SELECT * FROM posts WHERE school_id = 1 AND status = 2 AND type = 1 ORDER BY id DESC LIMIT 20;
-- 确认 type=ref 或 range，Extra 中无 Using filesort
```

---

## 7. 相关文档索引

| 文档 | 路径 | 用途 |
|------|------|------|
| 性能基线报告 | `tests/benchmark/baseline-report.md` | 优化前基线数据 |
| 优化对比报告 | `tests/benchmark/optimization-report.md` | 优化前后对比 |
| DB 性能报告 | `docs/db-performance-report.md` | 连接池调优 + 慢查询 Top5 |
| 压测脚本 | `tests/benchmark/run-benchmark.sh` | 一键压测工具 |
