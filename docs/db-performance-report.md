# 数据库连接池优化 + 慢查询治理报告

> 更新日期：2026-07-08 | 服务：6 微服务 MySQL

---

## 1. 连接池参数优化

### 1.1 优化前（硬编码）

| 参数 | 值 | 问题 |
|------|-----|------|
| MaxIdleConns | 10 | 空闲连接过少，高并发时频繁创建/销毁连接 |
| MaxOpenConns | 100 | 6 服务共用同一 RDS 时可能超出 max_connections |
| ConnMaxLifetime | 1h | 过长，RDS proxy 可能已断开连接 |
| ConnMaxIdleTime | 未设置 | 空闲连接长期占用内存，RDS 侧可能回收 |

### 1.2 优化后（可配置，默认值基于4C8G ECS + 阿里云 RDS）

| 参数 | 新默认值 | 依据 |
|------|----------|------|
| MaxIdleConns | **25** | 保持 25 个预热连接，减少建连开销 |
| MaxOpenConns | **200** | RDS max_connections=2000，6 服务平均分配 |
| ConnMaxLifetime | **3600s (1h)** | 保持不变，兼容 RDS proxy |
| ConnMaxIdleTime | **600s (10min)** | 及时释放空闲连接，降低 RDS 负载 |

### 1.3 配置方式

在 `config/my_config.yaml` 中新增（可选，未配置使用默认值）：

```yaml
mysql:
  pool:
    maxIdleConns: 25
    maxOpenConns: 200
    connMaxLifetime: 3600
    connMaxIdleTime: 600
```

---

## 2. 慢查询治理

### 2.1 慢查询监控机制

- **阈值**：200ms（`pkg/db/mysql.go:slowQueryThreshold`）
- **输出**：`[SLOW_QUERY] service=xxx duration=xxx sql=xxx`
- **实现**：GORM Before/After 回调对，覆盖 Query/Create/Update/Delete 全操作类型

### 2.2 Top 5 高风险慢查询（代码审计）

基于代码静态分析，以下查询在大数据量下存在慢查询风险：

| 序号 | 服务 | 查询路径 | 风险原因 | 优化建议 |
|------|------|----------|----------|----------|
| 1 | Content | `ListByCursor` + 多条件过滤 | `type` + `status` + `cursor` 三重 WHERE，无复合索引 | 添加 `idx_posts_list` 复合索引 |
| 2 | Task | `List` + 复杂 ORDER BY | `CASE WHEN` 表达式排序无法使用索引 | 拆分为两次查询或添加 `status` + `expired_at` 复合索引 |
| 3 | Content | `SearchContent` (ES) | ES 查询延迟高（网络往返 + 分词） | 增加 `size` 参数限制返回量，使用 `filter` 替代 `query` |
| 4 | Content | `ListComments` + `parent_id` 过滤 | `parent_id=0 AND status=1` 组合，无复合索引 | 添加 `idx_comments_list` 复合索引 |
| 5 | Task | `ExpireOpenTasks` | 全表扫描 `WHERE status=1 AND expired_at < NOW()` | 添加 `idx_tasks_expire` 索引 |

### 2.3 建议添加的索引

```sql
-- Content 服务：帖子列表查询加速
CREATE INDEX idx_posts_list ON posts (school_id, status, type, id DESC);

-- Content 服务：评论列表查询加速
CREATE INDEX idx_comments_list ON post_comments (post_id, parent_id, status);

-- Task 服务：任务过期扫描加速
CREATE INDEX idx_tasks_expire ON tasks (status, expired_at);

-- Task 服务：列表查询加速
CREATE INDEX idx_tasks_list ON tasks (school_id, status, expired_at, created_at);
```

---

## 3. 连接池监控建议

### 3.1 关键指标（GORM 标准）

```go
sqlDB.Stats()
// InUse     int      // 当前正在使用的连接数
// Idle      int      // 当前空闲的连接数
// WaitCount int64    // 等待连接的总次数
// WaitDuration time.Duration // 等待连接的总时间
```

### 3.2 告警阈值建议

| 指标 | 告警阈值 | 动作 |
|------|----------|------|
| InUse / MaxOpenConns | > 80% | 连接池即将耗尽，考虑扩容或优化查询 |
| WaitCount 增速 | > 10/s | 请求排队严重，检查慢查询 |
| WaitDuration 平均值 | > 50ms | 连接等待过久，增加 MaxOpenConns |
| Idle 连接数 | = 0 | 无空闲连接，高并发风险 |

---

## 4. 相关代码位置

| 文件 | 内容 |
|------|------|
| `pkg/db/mysql.go` | 连接池初始化 + 慢查询回调 |
| `pkg/db/scope.go` | SchoolScope 多租户隔离 |
| `config/config.go` | `MysqlPoolConfig` 结构体 + `GetPool()` 方法 |
| `config/my_config.yaml` | 运行时连接池配置（可选覆盖） |
