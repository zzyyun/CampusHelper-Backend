# Product Requirements Document: 高并发性能优化

**Version**: 1.0
**Date**: 2026-07-08
**Author**: Sarah (Product Owner)
**Quality Score**: 93/100

---

## Executive Summary

CampusHelper 后端系统当前部署在单台 4C8G ECS 上，尚未经过系统性压测和高并发优化。随着用户量增长和功能迭代（AI 审核、任务悬赏等），系统在开学季、活动推广、热门任务抢购等高并发场景下面临响应变慢甚至服务不可用的风险。

本 PRD 定义了以"压测先行、数据驱动、分阶段优化"为核心的高并发改造方案，目标是在 1-2 周内完成基线摸底和首轮优化，使系统在 4C8G 单机环境下具备可靠的高并发处理能力。

**核心策略**：先压测找瓶颈 → 再针对性优化 → 最后验证效果。

---

## Problem Statement

**Current Situation**:
- 系统未经高并发压测，不知道当前 QPS 上限和瓶颈在哪
- 6 个微服务共用 4C8G 单机，资源竞争风险高
- 数据库查询缺少缓存层，热点数据直接打 MySQL
- 无 API 限流机制，突发流量可能导致雪崩
- 无连接池调优配置，高并发下可能耗尽连接

**Proposed Solution**:
分三个阶段实施：① 性能压测摸底，② Redis 缓存 + 限流降级，③ 数据库优化 + 验证

**Business Impact**:
- 系统可用性从"未知"提升到"可控"
- 核心 API 响应时间 < 200ms（P95）
- 单机承载能力量化，为后续扩容提供数据依据

---

## Success Metrics

**Primary KPIs:**

| 指标 | 基线值（压测后） | 优化目标 | 测量方式 |
|------|-----------------|---------|---------|
| API P95 延迟 | 压测摸底 | < 200ms | go test -bench / wrk |
| 错误率 | 压测摸底 | < 0.1% | Prometheus metrics |
| 单机 QPS 上限 | 压测摸底 | 提升 50%+ | wrk 压测对比 |
| MySQL 慢查询数 | 压测摸底 | < 5/min | 慢查询日志 |
| 内存占用峰值 | 压测摸底 | < 70% | docker stats / top |

**Validation**: 每阶段结束后运行压测对比，用数据验证优化效果。

---

## User Personas

### Primary: 校园学生用户
- **Role**: 高校在校学生（目标：单校 5000+ 用户）
- **Goals**: 快速浏览帖子、秒抢悬赏任务、即时消息通知
- **Pain Points**: 高峰期页面加载慢、提交任务无响应、消息延迟
- **Technical Level**: 初级（手机用户，非技术用户）

### Secondary: 运维管理员
- **Role**: 系统管理员
- **Goals**: 监控系统健康状态、快速定位瓶颈、按需扩容
- **Pain Points**: 缺少性能基线数据、出问题难定位根因
- **Technical Level**: 中级（熟悉 Docker、Linux 基本运维）

---

## User Stories & Acceptance Criteria

### Story 1: 性能压测摸底

**As a** 运维管理员
**I want to** 在优化前获得系统性能基线数据
**So that** 后续优化有数据对比依据

**Acceptance Criteria:**
- [ ] 编写 wrk 压测脚本，覆盖 6 个服务的核心 API
- [ ] 压测场景：50/100/200/500 并发梯度
- [ ] 输出报告包含：QPS、P50/P95/P99 延迟、错误率、CPU/内存曲线
- [ ] 识别 Top 3 瓶颈点并给出优先级排序

### Story 2: Redis 多级缓存

**As a** 学生用户
**I want to** 在高峰期浏览帖子和任务列表时依然快速加载
**So that** 不会因为同时访问的人多而变卡

**Acceptance Criteria:**
- [ ] 帖子列表/详情、任务列表/详情增加 Redis 缓存
- [ ] 缓存策略：热点数据 TTL 5min，写操作主动失效
- [ ] 缓存穿透防护：空值缓存（TTL 30s）
- [ ] 缓存击穿防护：singleflight 防止并发回源
- [ ] 缓存命中率 > 80%（可通过 metrics 监控）

### Story 3: API 限流降级

**As a** 运维管理员
**I want to** 在突发流量时保护后端服务不被打垮
**So that** 系统可用性不因外部流量而崩溃

**Acceptance Criteria:**
- [ ] Gateway 层实现令牌桶限流，按 API 路径配置 QPS 上限
- [ ] 全局限流：单 IP 每分钟最多 60 次请求（可配置）
- [ ] 超限返回 429 Too Many Requests，不打到后端服务
- [ ] 降级策略：非核心 API（搜索/ES）在高负载时返回兜底数据
- [ ] 限流状态通过 Prometheus metrics 暴露

### Story 4: 数据库连接池 + 慢查询优化

**As a** 运维管理员
**I want to** MySQL 连接池参数合理配置，慢查询被识别和优化
**So that** 数据库不会成为并发瓶颈

**Acceptance Criteria:**
- [ ] GORM 连接池参数优化：MaxOpenConns / MaxIdleConns / ConnMaxLifetime 合理配置
- [ ] 启用 MySQL 慢查询日志（阈值 200ms）
- [ ] 压测中发现的 Top 5 慢查询增加索引或优化 SQL
- [ ] 优化后慢查询数量 < 5/min

### Story 5: 优化效果验证

**As a** 运维管理员
**I want to** 看到优化前后的性能对比数据
**So that** 确认优化确实有效

**Acceptance Criteria:**
- [ ] 优化后重跑相同压测场景
- [ ] 输出优化前后对比报告（QPS / 延迟 / 错误率 / 资源占用）
- [ ] 所有核心 API P95 延迟 < 200ms
- [ ] 错误率 < 0.1%

---

## Functional Requirements

### Feature 1: 性能压测基线

- **描述**: 使用 wrk 对 6 个微服务核心 API 进行梯度压测
- **工具**: wrk (HTTP) + 自定义 Go 压测脚本 (gRPC)
- **压测场景**:
  - 场景 A：帖子列表查询（GET /api/v1/posts）— 读密集
  - 场景 B：发帖提交（POST /api/v1/posts）— 写密集
  - 场景 C：任务列表 + 抢任务（GET + POST /api/v1/tasks）— 混合读写
  - 场景 D：用户登录（POST /api/v1/auth/login）— 认证密集
  - 场景 E：全服务混合压测 — 模拟真实流量
- **输出物**: `tests/benchmark/baseline-report.md`

### Feature 2: Redis 缓存层

- **描述**: 在 Gateway / Content / Task 服务引入 Redis 缓存
- **缓存架构**: 应用层缓存（GORM 查询结果 → Redis）
- **关键设计**:
  - 缓存 Key 规范：`{service}:{school_id}:{resource}:{id}`
  - 序列化：JSON（便于调试）
  - 失效策略：写操作主动失效 + TTL 兜底
  - 容错：Redis 不可用时降级直接查 DB，不阻塞请求
- **影响服务**: gateway, content, task

### Feature 3: Gateway 限流

- **描述**: 在 Gin 中间件层实现 API 限流
- **限流算法**: 令牌桶（golang.org/x/time/rate 或自实现）
- **配置维度**: 全局限流 + 按 API 路径限流 + 按 IP 限流
- **响应格式**: HTTP 429 + Retry-After 头
- **降级逻辑**: 当系统 CPU > 80% 时，非核心 API 返回缓存兜底数据
- **影响服务**: gateway

### Feature 4: 数据库优化

- **描述**: 连接池调优 + 慢查询治理
- **连接池配置**:
  - MaxOpenConns: 25（单服务）
  - MaxIdleConns: 10
  - ConnMaxLifetime: 5min
  - ConnMaxIdleTime: 3min
- **慢查询治理**: 按压测结果针对性加索引或重写 SQL
- **影响服务**: 所有有 DB 的服务（user, content, task, message, file）

---

## Technical Constraints

### Performance
- 核心 API P95 延迟 < 200ms
- 单机 QPS 提升 50%+（相对优化前基线）
- MySQL 慢查询 < 5/min
- 内存占用 < 70%（4C8G 环境）

### Security
- 限流配置不暴露内部服务细节
- 缓存数据不包含敏感信息（密码、token）
- Redis 仅监听内网，设置访问密码

### Integration
- **Redis 7**: 升级或新部署，用于缓存 + 限流计数
- **MySQL 8.0**: 已有，仅需配置调优
- **Prometheus + Grafana**: 指标采集和可视化（如有），否则通过日志输出
- **wrk**: 压测工具，需安装到 ECS 或本地开发机

### Technology Stack
- Go 1.25 + Gin + GORM（现有）
- go-redis/v9（已引入，需实际使用）
- golang.org/x/time/rate（限流）
- wrk（压测）
- singleflight（防缓存击穿，标准库 sync 单包）

---

## MVP Scope & Phasing

### Phase 1: 压测摸底（第 1-3 天）— MVP 核心

**交付物**: `tests/benchmark/baseline-report.md`

- 编写 wrk 压测脚本
- 对 6 个服务核心 API 进行 50/100/200 并发梯度压测
- 识别 Top 3 瓶颈点
- 输出基线报告（QPS / 延迟 / 错误率 / 资源占用）

### Phase 2: 缓存 + 限流（第 4-8 天）

**交付物**: 代码改动 + 配置

- Redis 缓存层接入（content/task 服务）
- Gateway 限流中间件
- 缓存穿透/击穿防护
- 降级兜底逻辑

### Phase 3: 数据库优化 + 效果验证（第 9-12 天）

**交付物**: 优化后压测报告 + 操作手册

- 连接池参数调优
- Top 5 慢查询优化
- 重跑 Phase 1 相同场景压测
- 输出优化前后对比报告
- 编写运维操作手册

### Future Considerations
- 多机水平扩展（ECS 集群）
- 数据库读写分离
- CDN 静态资源加速
- API 网关（如 APISIX）替代自建限流

---

## Risk Assessment

| Risk | 概率 | 影响 | 缓解策略 |
|------|------|------|---------|
| 压测发现严重瓶颈需大规模重构 | 中 | 高 | Phase 1 用最小脚本快速摸底，不投入过多时间 |
| Redis 升级影响现有功能 | 低 | 高 | 本地先验证，灰度切换，保留降级开关 |
| 优化引入新 bug | 中 | 中 | 每阶段跑全量测试 + 压测回归 |
| 1-2 周内四个措施全部完成有风险 | 高 | 中 | 严格按 Phase 执行，Phase 2 可裁剪降级逻辑 |

---

## Dependencies & Blockers

**Dependencies:**
- Redis 7 实例：需要部署或升级（可升级中间件预算）
- wrk 安装：`apt install wrk` 或编译安装
- 项目代码中已引入 go-redis/v9（需确认是否已初始化客户端）

**Known Blockers:**
- 当前无 Redis 实例运行（需先部署）
- 部分服务可能未暴露 /health 端点（压测前需补充）

---

## Appendix

### Glossary
- **QPS**: Queries Per Second，每秒查询数
- **P95/P99**: 95%/99% 请求的响应时间上限
- **缓存穿透**: 查询一个不存在的数据，每次都穿透到 DB
- **缓存击穿**: 热点 Key 过期瞬间，大量请求同时打到 DB
- **singleflight**: Go 标准库工具，并发请求同一 Key 时只允许一个回源
- **令牌桶**: 限流算法，以固定速率向桶中放令牌，请求消耗令牌

### References
- 已有 PRD: `docs/cloud-deployment-prd.md`（部署架构）
- 已有 PRD: `docs/ai-moderation-content-service-v3.0-prd.md`（AI 审核）
- CI/CD: `.github/workflows/deploy.yaml`（构建部署流水线）

---

*This PRD was created through interactive requirements gathering with quality scoring to ensure comprehensive coverage of business, functional, UX, and technical dimensions.*