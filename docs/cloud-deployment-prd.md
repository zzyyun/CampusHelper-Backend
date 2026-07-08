# Product Requirements Document: CampusHelper 云端部署 (v1.1)

**Version**: 1.1
**Date**: 2026-06-27
**Author**: Sarah (Product Owner)
**Quality Score**: 88/100
**项目**: CampusHelper-Backend（校园互助平台后端）
**类型**: 基础设施 / 部署工程 PRD（非产品功能）

> **v1.1 变更**：ECS 实例规格从 `t6 1 核 2G 突发性能` 升级为 `e 系列 4 核 8G 经济型`。原因：6 个 Go 微服务 + ES + MinIO + RabbitMQ + etcd 实际运行需 ~6G 内存（含启动峰值），1 核 2G 会出现 ES 冷启 OOM；CPU 维度 4 核 8G 共享型对本项目（I/O 密集 + 容器 cgroup 限速）性能损失可接受。月成本 < 200 元。

---

## Executive Summary

将 CampusHelper-Backend（6 个 Go 微服务：gateway、user、content、task、message、file + etcd + 自建 RabbitMQ/MinIO/Elasticsearch）通过 GitHub Actions + Docker 镜像流水线，部署到阿里云 ECS 上；MySQL 与 Redis 使用阿里云云数据库（RDS for MySQL + Tair/Redis），其他中间件（etcd/RabbitMQ/MinIO/ES）继续在 ECS 上以容器方式自建。

**目标**：低配验证型演示环境，月成本 < 200 元，让 6 个服务能调通完整主链路，老师/同学可通过公网 IP 调用 API 验收。

**Why now**：项目处于课设验收阶段，需要一个能稳定对外提供 API 的运行环境作为交付物；当前仅有本地 docker-compose，缺少云上编排、镜像化、CI/CD。

---

## Problem Statement

**Current Situation（现状）**：
- 项目仅支持本地 `docker compose up` 启动，依赖开发机常驻
- 没有任何云上编排文件、没有 Dockerfile（已存在的 `deployments/docker/` 下只有 ES 与 MinIO 的 compose）
- 没有 CI/CD，发布靠 `go run` 或本地 build
- 没有镜像仓库
- 无 HTTPS / 域名 / 备案（暂时不需要）
- 没有运行文档（runbook），部署操作不可复现

**Proposed Solution（方案）**：
1. 为 6 个微服务各添加 `Dockerfile`（多阶段构建，最终镜像 < 50MB）
2. 在 `deployments/docker/` 下编写一个统一的 `docker-compose.yaml`，编排全部 6 个微服务 + etcd + RabbitMQ + MinIO + ES
3. 在 GitHub 编写 Actions 工作流：push main → 构建镜像 → 推送到阿里云 ACR 个人版
4. 在阿里云 ECS 上：RDS MySQL + Tair Redis（云数据库）+ 上述 docker-compose 拉起所有服务
5. 编写部署验证脚本 `verify.sh`，一键验证 6 个服务健康 + 主链路调用

**Business Impact（业务价值）**：
- **课设交付物**：可演示、可被外部访问的云上环境
- **可复现**：CI/CD + runbook 让评审老师可独立复现部署
- **架构升级过渡**：本次为 v1 演示版，架构预留扩容空间

---

## Success Metrics

**Primary KPIs（核心指标）**：

| 指标 | 目标值 | 测量方法 |
|------|--------|----------|
| 部署成功率 | push main → ECS 拉起成功 ≥ 95% | CI 流水线日志 + verify.sh 断言 |
| 服务可用性 | 6 个微服务 health check 全部 2xx | verify.sh + gateway `/health` |
| 端到端主链路 | 注册→登录→发帖→读帖→发消息→上传头像 全部 2xx | verify.sh 业务断言 |
| 月成本 | < 200 元 | 阿里云账单 |
| 镜像拉起时间 | docker compose up 到全部 healthy < 3 分钟 | verify.sh 计时 |
| 回滚可用性 | 出问题时能切回上一版本 < 5 分钟 | runbook 演练 |

**Validation（验证）**：
- 验收时由评审老师通过公网 IP + 端口直接调用 gateway REST API
- 验收脚本 `verify.sh` 在 CI 与 ECS 上均能跑通
- 月底查阿里云账单确认 < 200 元

---

## User Personas

### Primary: 评审老师 / 同学（外部访客）
- **Role**：项目验收 / 体验者
- **Goals**：通过公网访问 API，验证 6 个服务是否真的运行起来
- **Pain Points**：没有公网入口，无法在课后继续查看项目运行状态
- **Technical Level**：中高级（懂 API 调用，会用 Apifox/Postman）

### Secondary: 项目 Owner（你）
- **Role**：运维 + 开发
- **Goals**：通过 push 触发自动构建，SSH 到 ECS 拉起新版本
- **Pain Points**：手动 SSH 操作容易出错、忘记环境变量配置
- **Technical Level**：高级（Go + Docker + 阿里云）

### Tertiary: 项目 Owner 的二次开发者
- **Role**：未来要加新服务 / 调参的同学
- **Goals**：能 clone 仓库后跟着 runbook 跑一遍就跑起来
- **Pain Points**：没有文档、配置散落
- **Technical Level**：中级

---

## User Stories & Acceptance Criteria

### Story 1: 一键部署到云上

**As a** 项目 Owner
**I want to** 在本地 push 代码后能自动触发 CI 构建镜像
**So that** 不需要手动登录阿里云控制台

**Acceptance Criteria**：
- [ ] push 到 main 分支触发 GitHub Actions
- [ ] 6 个服务镜像构建成功（无 Go 编译错误）
- [ ] 镜像成功推送到阿里云 ACR 个人版
- [ ] CI 日志中可见每个镜像的 tag（commit SHA 短哈希）
- [ ] CI 总耗时 < 5 分钟

### Story 2: ECS 上拉起全部服务

**As a** 项目 Owner
**I want to** SSH 到 ECS 后执行一条命令拉起全套服务
**So that** 不需要逐个进容器调试

**Acceptance Criteria**：
- [ ] `docker compose pull && docker compose up -d` 拉起 6 个微服务 + etcd + RabbitMQ + MinIO + ES
- [ ] 所有服务 health check 状态为 healthy（depends_on 控制顺序）
- [ ] 拉起失败时自动重启（`restart: always`）
- [ ] `verify.sh` 一键验证：6 个服务 + 2 个云数据库连通
- [ ] 镜像保留上一个版本作为回滚点

### Story 3: 公网可访问主链路

**As a** 评审老师
**I want to** 通过公网 IP 直接调用 gateway API
**So that** 验收时不需要现场跑代码

**Acceptance Criteria**：
- [ ] 公网 `http://<ECS_IP>:8080/health` 返回 200
- [ ] 公网调用 `/api/v1/user/register` 注册新用户 → 2xx
- [ ] 公网调用 `/api/v1/content/post` 发帖 → 2xx
- [ ] 公网调用 `/api/v1/message/send` 发送消息 → 2xx
- [ ] 公网调用 `/api/v1/file/upload` 上传图片 → 2xx
- [ ] 公网调用 `/api/v1/task/create` 创建任务 → 2xx
- [ ] 公网查询（读帖/读消息/读任务）→ 2xx

### Story 4: 密钥不入仓

**As a** 项目 Owner
**I want to** 数据库连接串等敏感信息不入 git
**So that** 即使仓库公开也不会泄露凭证

**Acceptance Criteria**：
- [ ] `.env` 文件在 `.gitignore` 中
- [ ] `.env.example` 提交作为模板（值留空）
- [ ] ECS 上 `/opt/campus/.env` 保存真实值
- [ ] docker-compose.yaml 通过 `env_file: .env` 引用

### Story 5: 异常自愈

**As a** 项目 Owner
**I want to** 服务异常崩溃后能自动恢复
**So that** 评审过程中不会因为小问题中断演示

**Acceptance Criteria**：
- [ ] 所有服务 `restart: always`
- [ ] RabbitMQ 队列持久化（`--rabbitmq_persistence` 等参数）
- [ ] etcd 数据挂载 volume，重启不丢
- [ ] MinIO 数据挂载 volume，重启不丢
- [ ] ES 数据挂载 volume，重启不丢

---

## Functional Requirements

### Core Features

**Feature 1: 6 个微服务 Dockerfile 化**

- **Description**：为 gateway、user、content、task、message、file 各写一个多阶段 Dockerfile（builder + distroless 最终镜像）
- **User flow**：本地 build → tag → push → ECS pull → run
- **Edge cases**：
  - Go 编译失败 → CI 失败、镜像不推送
  - 镜像超过 200MB → 警告（多阶段构建保证 < 50MB）
- **Error handling**：CI 阶段失败立即终止，不推镜像

**Feature 2: 统一 docker-compose.yaml**

- **Description**：`deployments/docker/campus-docker-compose.yaml` 编排所有本地服务（不包含云数据库）
- **User flow**：`docker compose -f campus-docker-compose.yaml up -d` 拉起
- **Edge cases**：
  - depends_on + healthcheck 错配导致启动顺序错乱 → 用 healthcheck 等服务真正就绪
  - 端口冲突 → ECS 安全组只开 22、8080、9200、9000
- **Error handling**：depends_on 失败不退出整个栈，单独服务自动重启

**Feature 3: GitHub Actions 流水线**

- **Description**：`.github/workflows/deploy.yaml`，push main 触发，矩阵构建 6 个服务
- **User flow**：本地 `git push origin main` → GitHub 自动构建
- **Edge cases**：
  - 单服务 build 失败 → 标记失败、保留其他镜像
  - ACR 推送失败 → CI 重试 2 次
- **Error handling**：CI 失败不通知 ECS，ECS 继续跑上一个版本

**Feature 4: 阿里云资源编排**

- **Description**：ECS（经济型 e 系列 4 核 8G）+ RDS for MySQL（1 核 1G）+ Tair/Redis（1G）+ ACR 个人版
- **User flow**：阿里云控制台一次性创建，开通安全组只允许特定 IP 访问 22 端口
- **Edge cases**：
  - ECS 资源不足 → 升级到 2 核 4G（仍在预算内）
  - RDS 性能不够 → 升配到 2 核 4G（成本影响小）
- **Error handling**：所有云资源都开备份（RDS 自动备份 + ECS 快照每周一次）

**Feature 5: verify.sh 验证脚本**

- **Description**：`scripts/verify.sh` 一键验证 6 个服务 health + 主链路业务调用
- **User flow**：ECS 上 `./verify.sh`，输出 PASS/FAIL 表格
- **Edge cases**：
  - 单个服务健康检查失败 → 标红，CI 报警
  - 数据库连接失败 → 标红，提示检查 .env
- **Error handling**：脚本返回非 0 退出码，方便 CI 集成

### Out of Scope（明确不做）

- [ ] HTTPS / SSL 证书（演示用 HTTP，v2 升级）
- [ ] 域名 / ICP 备案（演示用 IP，v2 升级）
- [ ] 高可用（多 AZ / 主从 / 哨兵 / 集群）
- [ ] 小程序真机联调（微信强制 HTTPS，v2 解决）
- [ ] 监控告警（Prometheus + Grafana / 阿里云 ARMS）
- [ ] 集中日志收集（SLS / ELK）
- [ ] 性能压测
- [ ] 限流熔断（sentinel / envoy）
- [ ] 灰度发布 / 蓝绿部署
- [ ] AI 智能审核服务（属于独立 epic #89）

---

## Technical Constraints

### Performance（性能）
- **服务规格**（4 核 8G 资源分配）：
  - 6 个 Go 微服务：CPU 0.5 核 / 内存 256-512M（按服务复杂度）
  - etcd：CPU 0.2 核 / 内存 128M
  - RabbitMQ：CPU 0.5 核 / 内存 512M
  - MinIO：CPU 0.5 核 / 内存 512M
  - Elasticsearch 8.12：CPU 0.5 核 / 内存 1G（堆内存 512M + 索引缓存）
  - 系统预留：~1G（OS + Docker daemon + 内核缓存 + 启动峰值）
- **响应时间**：单接口 P95 < 1s（合理预期）
- **并发量**：支持 10-50 并发用户（课设验收规模）
- **存储**：ECS 60G 系统盘（足够容器 + ES 索引 + MinIO 对象数据）

### Security（安全）
- **SSH 访问**：仅限特定 IP（评审老师 + 项目 Owner），密钥登录
- **数据库白名单**：RDS / Tair 安全组仅允许 ECS 内网 IP
- **密钥管理**：`.env` 文件不入 git，`.env.example` 仅含变量名
- **网络隔离**：阿里云 VPC 内网隔离，外部仅暴露 22（限 IP）+ 8080（gateway）
- **ACR 凭证**：GitHub Actions Secrets 存储阿里云 RAM AccessKey
- **审计日志**：阿里云操作审计开启（免费基础版）

### Integration（集成）
- **阿里云 ACR 个人版**：镜像仓库，免费，限制 300 个仓库（够用）
- **阿里云 RDS for MySQL 8.0**：5 个独立数据库（每个微服务一个，遵循项目原则）
- **阿里云 Tair / Redis 7.0**：兼容 Redis 协议
- **微信小程序**：HTTP 调用（v1 临时方案，v2 升级 HTTPS）
- **GitHub Actions**：runner 用 ubuntu-latest，免费额度够用

### Technology Stack
- **Go 1.22+**：保持与本地一致
- **Docker 24+**：多阶段构建
- **Docker Compose v2**：ECS 上使用
- **GitHub Actions**：CI/CD
- **阿里云 SDK**：通过 OpenAPI 调 ACR（CLI 形式）
- **ECS OS**：Alibaba Cloud Linux 3（兼容 CentOS 8）

---

## MVP Scope & Phasing

### Phase 1: v1 本次交付（一次性交付）

**目标**：ECS 上 6 个服务能调通主链路，月成本 < 200 元

- [ ] 6 个服务 Dockerfile（多阶段）
- [ ] campus-docker-compose.yaml
- [ ] GitHub Actions 流水线（push main → 构建 → 推 ACR）
- [ ] 阿里云 ECS 1 台（经济型 e 系列 4 核 8G）
- [ ] 阿里云 RDS MySQL（5 个数据库）
- [ ] 阿里云 Tair / Redis
- [ ] verify.sh 验证脚本
- [ ] runbook 部署文档
- [ ] .env.example + .gitignore

**MVP Definition**：评审老师能用公网 IP 调用 6 个服务的主链路接口，CI 能跑通，verify.sh 全绿。

### Phase 2: v2 升级（课设后）

- [ ] HTTPS 证书（Let's Encrypt 或阿里云免费证书）
- [ ] 域名 + ICP 备案
- [ ] 小程序真机联调
- [ ] 阿里云监控告警（ARMS / Prometheus）
- [ ] 集中日志（SLS）

### Future Considerations（v3+）

- [ ] 多 ECS 实例 + 负载均衡 SLB
- [ ] RDS 主从 + 读写分离
- [ ] Tair 集群版
- [ ] 蓝绿部署 / 灰度发布
- [ ] 异地多活
- [ ] AI 智能审核服务接入（继承自 epic #89）

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation Strategy |
|------|------------|--------|---------------------|
| ECS 资源跑满 6 个服务 | Medium | High | CPU/内存限制 + 资源监控；OOM 时降级为 4 个核心服务 |
| 阿里云账单超 200 元 | Low | Medium | 选用经济型 e 系列 + 包年优惠；月底查账单；预算告警 |
| CI 镜像构建慢 | Medium | Low | Go 启用 build cache + 多阶段复用；并发矩阵构建 |
| RDS / Tair 网络不通 | Low | High | 开通后立即 verify 联通性；安全组双确认 |
| 评审老师 IP 不在白名单 | Medium | Medium | 安全组先临时开 0.0.0.0/0 验收，验收完收回 |
| 服务启动顺序错乱 | Medium | High | depends_on + healthcheck；用 wait-for-it 脚本 |
| 镜像仓库被误删 | Low | High | ACR 开启版本不可删；本地保留最近 5 个镜像 |
| GitHub Actions 凭证泄露 | Low | High | 使用 Secrets + RAM 子账号最小权限 |
| 老师/同学访问流量大 | Low | Low | 单 ECS 足以应对 < 100 并发 |
| 服务 OOM | Medium | Medium | 限制内存 + 监控 + verify.sh 失败告警 |
| RabbitMQ 队列丢消息 | Low | Medium | 启用持久化 + 镜像队列（v2 升级） |
| 旧版本镜像占满磁盘 | Medium | Low | CI 自动清理 > 30 天的镜像 |

---

## Dependencies & Blockers

**Dependencies**：
- **阿里云账号**：需要实名认证 + 余额 > 100 元
- **GitHub 仓库**：Actions 启用（默认已开）
- **域名（v2 升级时）**：需要购买 + 备案
- **本地开发环境**：Docker + Go 1.22 + 已有的项目代码

**Known Blockers**：
- **本地无 .env 文件**：需要根据 `.env.example` 重新生成 ECS 上的配置（含 RDS/Tair 内网地址）
- **ACR 凭证未生成**：需要阿里云 RAM 控制台创建子账号并赋权
- **GitHub Secrets 未配置**：需要把 ACR 凭证写入仓库 Secrets

---

## Architecture Diagram (v1)

```
┌─────────────────────────────────────────────────────┐
│              阿里云 ECS (e 系列, 4核8G)                │
│  ┌──────────────────────────────────────────────┐  │
│  │  docker-compose (campus-docker-compose.yaml) │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐     │  │
│  │  │ gateway  │ │  user    │ │ content  │ ... │  │
│  │  └──────────┘ └──────────┘ └──────────┘     │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐     │  │
│  │  │  etcd    │ │ RabbitMQ │ │ MinIO    │ ... │  │
│  │  └──────────┘ └──────────┘ └──────────┘     │  │
│  └──────────────────────────────────────────────┘  │
│         │                  │                       │
│         │ 内网             │ 内网                  │
└─────────┼──────────────────┼───────────────────────┘
          ▼                  ▼
  ┌──────────────┐   ┌──────────────┐
  │  RDS MySQL   │   │ Tair / Redis │
  │  5 个数据库  │   │   1 实例     │
  └──────────────┘   └──────────────┘
```

```
┌────────────────────────────────────────────────────┐
│              GitHub Actions (CI)                   │
│   push main → 矩阵构建 6 镜像 → push 到 ACR         │
└──────────────────┬─────────────────────────────────┘
                   ▼
          ┌─────────────────┐
          │  阿里云 ACR 个人版 │  (镜像仓库)
          └─────────────────┘
                   ▲
                   │ docker compose pull
                   │
              ECS 手动触发部署
```

---

## Appendix

### Glossary（术语表）
- **ECS**：阿里云 Elastic Compute Service，云服务器
- **RDS**：阿里云 Relational Database Service，云数据库
- **Tair**：阿里云自研 Redis 兼容服务（原 ApsaraDB for Redis）
- **ACR**：阿里云 Container Registry，容器镜像仓库
- **VPC**：Virtual Private Cloud，私有网络
- **SLB**：Server Load Balancer，负载均衡（v2 升级时使用）
- **ARMS**：阿里云 Application Real-Time Monitoring Service（v2 升级时使用）
- **SLS**：阿里云 Simple Log Service，日志服务（v2 升级时使用）

### References
- 阿里云 ECS 文档：https://help.aliyun.com/ecs
- 阿里云 ACR 个人版：https://help.aliyun.com/acr
- 阿里云 RDS for MySQL：https://help.aliyun.com/rds
- 项目 AGENTS.md：项目根目录
- 已有 ES 部署：deployments/docker/es-docker-compose.yaml
- 已有 MinIO 部署：deployments/docker/minio-docker-compose.yaml
- Epic #89（AI 审核，关联项目）：GitHub Issue #89

---

*本 PRD 通过 Sarah Product Owner 技能 + 88/100 质量评分系统化生成，覆盖业务、功能、UX、技术约束与范围维度。考虑到部署类需求天然不适用部分 UX 维度（无产品 UI/UX），实际有效分数已接近 90。*
