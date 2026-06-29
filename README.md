# CampusHelper-Backend（校园互助平台后端）

> 面向高校大学生的微信小程序后端 · Go 微服务架构 · 已部署阿里云 ECS

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![Services](https://img.shields.io/badge/微服务-6-blueviolet)](#系统架构)
[![Deploy](https://img.shields.io/badge/部署-阿里云%20ECS-FF6A00?logo=alibabacloud)](#部署)
[![Status](https://img.shields.io/badge/状态-生产可用-brightgreen)](#状态)

---

## 项目简介

**CampusHelper** 是一款面向高校大学生的微信小程序后端系统，提供校园社区（失物招领、二手交易、组队/拼车）、任务悬赏（跑腿代拿）、深度消息互动等核心业务。

**核心架构特性**：
- **多租户隔离**：不同学校展示不同内容，所有业务数据强制按 `school_id` 隔离
- **全链路追踪**：TraceID 贯穿 HTTP 网关 → gRPC 内部调用 → RabbitMQ 异步消息 → 消费者
- **每服务独立数据库**：5 个 MySQL 库（user/content/task/message/file），符合微服务架构原则
- **AI 智能审核**：集成 v3.0 AI 文本审核（DFA + ONNX 模型双通道）
- **HTTPS 生产部署**：Nginx 反代 + 阿里云免费个人测试证书（域名 `rithupc.cn`）

## 核心特性

| 业务 | 描述 |
|------|------|
| 👤 **用户系统** | 微信小程序登录、RBAC 权限、学校绑定、用户信息管理 |
| 📝 **内容社区** | 通用帖子、失物招领、二手交易、ES 关键词搜索、点赞/收藏/评论 |
| 🏃 **任务悬赏** | 跑腿代拿、组队/拼车、悬赏机制、状态机流转、超时自动取消 |
| 🔔 **消息中心** | 站内信、系统/互动/业务通知（WebSocket 实时推送规划中） |
| 🖼️ **文件服务** | 图片上传（MinIO 对象存储）、图片压缩、文件元数据 |
| 🛡️ **管理后台** | 内容审核、用户封禁/解封、用户列表、运营数据 |
| 🤖 **AI 审核** | DFA 敏感词 + ONNX 文本分类，800ms 超时，失败降级 DFA-only |

## 技术栈

| 类别 | 选型 |
|------|------|
| **语言** | Go 1.25+ |
| **API 网关** | [Gin](https://github.com/gin-gonic/gin)（HTTP/RESTful） |
| **内部 RPC** | gRPC + Protobuf |
| **服务发现** | [etcd](https://etcd.io/) v3.5 |
| **数据库** | MySQL 8.0（GORM）+ Redis 7（go-redis/v9） |
| **消息队列** | [RabbitMQ](https://www.rabbitmq.com/) 3.12（amqp091-go） |
| **搜索引擎** | [Elasticsearch](https://www.elastic.co/) 8（go-elasticsearch + ik 分词） |
| **对象存储** | [MinIO](https://min.io/)（S3 兼容） |
| **配置中心** | [Viper](https://github.com/spf13/viper) |
| **链路追踪** | OpenTelemetry + [Jaeger](https://www.jaegertracing.io/) |
| **日志** | 结构化日志（带 TraceID 与 RabbitMQ） |
| **容器化** | Docker + Docker Compose v2 |
| **反向代理** | [Nginx](https://www.nginx.com/) 1.25（alpine） |
| **CI/CD** | GitHub Actions（6 服务矩阵构建 + 推阿里云 ACR） |
| **云服务** | 阿里云 ECS + RDS MySQL + Tair Redis + ACR 容器镜像 |

## 系统架构

```
                                  ┌─────────────────────────────────────┐
                                  │   小程序 (WeChat / 微信)               │
                                  │   AppID: wxa782f10bddd49b38            │
                                  └────────────────┬────────────────────┘
                                                   │ HTTPS
                                                   ▼
                          ┌────────────────────────────────────────────┐
                          │  Nginx (campus-nginx) — TLS 终止            │
                          │  443 → 50000 (HTTP/2 + 安全头)             │
                          └────────────────┬───────────────────────────┘
                                           │
                                           ▼
                          ┌────────────────────────────────────────────┐
                          │  API Gateway (campus-gateway :50000)        │
                          │  · JWT 鉴权  · 限流  · 跨域  · 协议转换      │
                          │  · TraceID 注入  · School-ID 隔离           │
                          └────┬──────┬──────┬──────┬──────┬─────────────┘
                               │      │      │      │      │      gRPC
              ┌────────────────┘      │      │      │      │
              ▼                ▼       ▼      ▼      ▼      ▼
        ┌────────┐      ┌────────┐ ┌────────┐ ┌──────┐ ┌──────┐ ┌────────────┐
        │  User  │      │Content │ │  Task  │ │  Msg │ │ File │ │ AI-Moder   │
        │  :50001│      │ :50002 │ │ :50003 │ │:50004│ │:50005│ │   :50061   │
        └───┬────┘      └───┬────┘ └───┬────┘ └──┬───┘ └──┬───┘ └─────┬──────┘
            │               │          │        │       │            │
            ▼               ▼          ▼        ▼       ▼            ▼
       ┌────────┐      ┌────────┐ ┌────────┐ ┌──────┐ ┌──────┐ ┌────────────┐
       │campus_ │      │campus_ │ │campus_ │ │campus│ │campus│ │  ONNX     │
       │  user  │      │content │ │  task  │ │ _msg │ │_file │ │  Model    │
       └────────┘      └────────┘ └────────┘ └──────┘ └──────┘ └────────────┘

       MySQL 5 库   ← 阿里云 RDS MySQL (rm-bp1w9fi56lv1b7348)
       Redis         ← 阿里云 Tair (r-bp1ywvgc1xxbc7b2x2)

       etcd / RabbitMQ / MinIO / Elasticsearch ← ECS 自建（10 容器）
```

### 6 个微服务

| 服务 | 端口 | 数据库 | 职责 |
|------|------|--------|------|
| **gateway** | 50000 | - | HTTP 入口、JWT、限流、路由 |
| **user** | 50001 | campus_user | 微信登录、学校绑定、用户管理 |
| **content** | 50002 | campus_content | 帖子、ES 搜索、AI 审核 |
| **task** | 50003 | campus_task | 跑腿/拼车、状态机、抢单 |
| **message** | 50004 | campus_message | 站内信、通知（消费 MQ 事件） |
| **file** | 50005 | campus_file | 图片上传、MinIO 集成 |
| **ai-moderation** | 50061 | - | ONNX Runtime 推理（独立进程） |

## 目录结构

```
CampusHelper-Backend/
├── cmd/                          # 微服务启动入口（main.go）
│   ├── gateway/                  # API 网关
│   ├── user/                     # 用户服务
│   ├── content/                  # 内容服务（含 AI 审核集成）
│   ├── task/                     # 任务服务
│   ├── message/                  # 消息服务
│   ├── file/                     # 文件服务
│   └── ai-moderation/            # AI 审核独立进程（ONNX 推理）
├── internal/                     # 各服务内部业务逻辑
├── PB/                           # Protobuf 定义 + 生成代码
├── pkg/                          # 全局公共组件
│   ├── errcode/                  # 统一错误码
│   ├── contextx/                 # Context 扩展
│   ├── middleware/               # Gin + gRPC 拦截器
│   ├── mq/                       # RabbitMQ 封装
│   ├── es/                       # ES 客户端
│   ├── db/                       # GORM 初始化
│   ├── tracer/                   # OpenTelemetry
│   ├── snowflake/                # 雪花 ID
│   ├── etcd/                     # 服务发现
│   ├── jwt/                      # JWT
│   ├── sensitive/                # DFA 敏感词
│   └── aiclient/                 # AI 审核客户端
├── deployments/                  # 部署配置
│   ├── docker/
│   │   ├── campus-docker-compose.yaml    # 10 容器编排
│   │   ├── nginx/                        # Nginx 配置 + SSL
│   │   └── es-with-ik/                   # ES + ik 分词
│   └── es/
├── build/docker/                 # 6 个微服务 Dockerfile + build.sh
├── config/                       # 配置模板
│   └── my_config.ecs.yaml.template
├── docs/                         # PRD + 文档 + ApiFox 接口
├── scripts/                      # 部署 + 验证脚本
│   ├── verify.sh
│   ├── verify-https.sh
│   ├── validate-campus-deploy.sh
│   └── miniapp/                  # 微信小程序源码
├── .github/workflows/deploy.yaml # CI
├── CLAUDE.md                     # Claude Code 指令
├── AGENTS.md                     # AI 代理开发指南
└── Makefile
```

## 快速开始

### 前置依赖

- Go 1.25+
- Docker + Docker Compose v2
- 阿里云账号（RDS / Tair / ACR）
- 微信小程序 AppID

### 本地开发

```bash
# 1. 克隆仓库
git clone https://github.com/zzyyun/CampusHelper-Backend.git
cd CampusHelper-Backend

# 2. 准备配置
cp config/my_config.ecs.yaml.template config/my_config.yaml
# 编辑 config/my_config.yaml，配置 MySQL / Redis / 微信 AppID 等

# 3. 拉起基础设施 + 6 服务
cd deployments/docker
docker compose -f campus-docker-compose.yaml up -d

# 4. 验证
bash scripts/verify.sh
```

### ECS 部署

详见 [docs/cloud-deployment-prd.md](docs/cloud-deployment-prd.md)。

```bash
# 1. ECS 上克隆 + 配置
ssh root@<ECS_IP> "cd /opt/campus && git clone https://github.com/zzyyun/CampusHelper-Backend.git ."
scp config/my_config.yaml root@<ECS_IP>:/opt/campus/config/

# 2. 拉起 10 容器
ssh root@<ECS_IP> "cd /opt/campus && \
  docker compose --env-file .env -f deployments/docker/campus-docker-compose.yaml up -d"

# 3. 验证
ssh root@<ECS_IP> "bash scripts/verify.sh && bash scripts/validate-campus-deploy.sh"
```

### HTTPS 部署（生产）

详见 [docs/wechat-miniapp-launch-prd.md](docs/wechat-miniapp-launch-prd.md)。

```bash
# 1. 申请阿里云免费个人测试证书
# 2. 上传证书到 ECS
scp fullchain.pem privkey.pem root@<ECS_IP>:/opt/campus/deployments/nginx/certs/

# 3. 启动 Nginx 容器
ssh root@<ECS_IP> "cd /opt/campus && \
  docker compose --env-file .env -f deployments/docker/campus-docker-compose.yaml up -d nginx"

# 4. 验证
bash scripts/verify-https.sh
```

## 配置说明

### 关键配置项（`config/my_config.yaml`）

| 字段 | 说明 | 示例 |
|------|------|------|
| `mysql.host` | 阿里云 RDS 内网域名 | `rm-bp1w9fi56lv1b7348.mysql.rds.aliyuncs.com` |
| `mysql.password` | RDS 密码 | `Yasuo1228` |
| `redis.address` | Tair Redis 内网域名 | `r-bp1ywvgc1xxbc7b2x2.redis.rds.aliyuncs.com:6379` |
| `rabbitmq.address` | 容器内地址 | `campus-rabbitmq:5672` |
| `elasticsearch.index` | 帖子 ES 索引名 | `campus_posts` |
| `etcd.address` | 容器内地址 | `campus-etcd:2379` |
| `jwt.authKey` | JWT 签名密钥 | `<your-jwt-secret>` |
| `wechat.appId` | 微信小程序 AppID | `wxa782f10bddd49b38` |
| `wechat.appSecret` | 微信小程序 Secret | （敏感） |

> ⚠️ `config/my_config.yaml` 不入仓（含敏感凭证）。模板见 `config/my_config.ecs.yaml.template`。

## API 文档概览

完整接口定义见 `docs/apifox-*-api.yaml`（可直接导入 ApiFox）。核心路由（`/api/v1`）：

### 公开接口
| Method | Path | 说明 |
|--------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/schools` | 学校列表 |
| POST | `/user/login` | 微信登录（双 Token） |
| POST | `/user/refresh` | 刷新 Token |

### 用户
| Method | Path | 说明 |
|--------|------|------|
| GET | `/user/me` | 当前用户信息 |
| PUT | `/user/campus` | 绑定学校 |
| PUT | `/user/info` | 更新昵称/头像 |

### 内容
| Method | Path | 说明 |
|--------|------|------|
| GET | `/content/posts` | 帖子列表（游标分页） |
| GET | `/content/posts/:id` | 帖子详情 |
| POST | `/content/posts` | 发帖（含 AI 审核） |
| PUT | `/content/posts/:id` | 编辑 |
| DELETE | `/content/posts/:id` | 删除 |
| POST | `/content/posts/:id/like` | 点赞 |
| POST | `/content/search` | ES 搜索 |

### 任务
| Method | Path | 说明 |
|--------|------|------|
| GET | `/tasks` | 任务列表 |
| GET | `/tasks/:id` | 任务详情 |
| POST | `/tasks` | 发任务 |
| POST | `/tasks/:id/claim` | 抢单 |
| PUT | `/tasks/:id/complete` | 完成 |
| PUT | `/tasks/:id/cancel` | 取消 |

### 通知
| Method | Path | 说明 |
|--------|------|------|
| GET | `/notifications` | 通知列表 |
| GET | `/notifications/unread-count` | 未读数 |
| PUT | `/notifications/:id/read` | 标已读 |
| PUT | `/notifications/read-all` | 全部已读 |

### 文件
| Method | Path | 说明 |
|--------|------|------|
| POST | `/files/upload` | 上传图片（multipart） |
| GET | `/files/:id` | 文件元数据 |
| DELETE | `/files/:id` | 删除文件 |

### 管理后台（需管理员权限）
| Method | Path | 说明 |
|--------|------|------|
| POST | `/admin/users/ban` | 封禁用户 |
| POST | `/admin/users/unban` | 解封 |
| GET | `/admin/users/list` | 用户列表 |
| GET | `/admin/content/audit-list` | 审核队列 |
| POST | `/admin/content/audit` | 审核动作 |

## 测试

### 端到端业务验证

```bash
# 5 阶段业务验证（推荐：每次部署后跑）
bash scripts/verify.sh

# 4 阶段 ECS 部署验证
bash scripts/validate-campus-deploy.sh

# 7 项 HTTPS 验证
bash scripts/verify-https.sh
```

### 单元测试

```bash
go test ./...
```

## 部署

### GitHub Actions CI

`.github/workflows/deploy.yaml`：push main 触发

1. 矩阵构建 6 个服务镜像（fail-fast: false）
2. 缓存：Go modules + Docker layers（GHA cache）
3. tag 策略：`:v1.0-{service}-{sha}` + `:v1.0-{service}-latest`
4. 推阿里云 ACR（单 repo 多 tag 模式：`campus_sends/campus`）

需要在 GitHub 仓库 Settings → Secrets 配置：
- `ACR_REGISTRY`：阿里云 Registry 域名
- `ACR_USERNAME`：阿里云账号
- `ACR_PASSWORD`：AccessKey Secret

### ECS 资源分配（4 核 8G 经济型 e 系列）

| 服务 | CPU | 内存 |
|------|-----|------|
| 6 Go 服务 | 0.2-0.5 | 256-512M |
| etcd | 0.2 | 256M |
| RabbitMQ | 0.5 | 512M |
| MinIO | 0.5 | 512M |
| Elasticsearch | 0.5 | 1G |
| Nginx | 0.2 | 128M |

**月成本 < 150 元**（ECS + RDS + Tair + ACR + 流量）。

## 文档导航

| 文档 | 用途 |
|------|------|
| [docs/cloud-deployment-prd.md](docs/cloud-deployment-prd.md) | v1.1 云端部署 PRD |
| [docs/wechat-miniapp-launch-prd.md](docs/wechat-miniapp-launch-prd.md) | v2.0 微信小程序上线 PRD |
| [docs/ai-moderation-content-service-v3.0-prd.md](docs/ai-moderation-content-service-v3.0-prd.md) | v3.0 AI 智能审核 PRD |
| [docs/gateway-v1.2-prd.md](docs/gateway-v1.2-prd.md) | 网关 v1.2 设计 |
| [docs/content-service-v2-prd.md](docs/content-service-v2-prd.md) | 内容服务 v2 PRD |
| [docs/task-service-prd.md](docs/task-service-prd.md) | 任务服务 PRD |
| [docs/user-service-v2.0-prd.md](docs/user-service-v2.0-prd.md) | 用户服务 v2.0 PRD |
| [CLAUDE.md](CLAUDE.md) | Claude Code 项目指令 |
| [AGENTS.md](AGENTS.md) | AI 代理开发指南 |

## 路线图

### ✅ 已完成
- v1.0 基础设施：etcd + RabbitMQ + MinIO + ES + 6 服务
- v1.1 云端部署：阿里云 ECS + RDS + Tair
- v1.2 网关升级：JWT + 限流 + 跨域
- v2.0 内容服务：失物招领 + 二手交易 + 跑腿/拼车
- v2.0 微信小程序上线：HTTPS + 阿里云证书
- v3.0 AI 智能审核：DFA + ONNX 双通道

### 🔄 进行中
- Phase B 微信小程序真机自测
- Phase C 体验版 + 正式版

### 📋 未来
- v3.1 实时消息（WebSocket）
- v3.2 管理后台 UI
- v3.3 推送通知（订阅消息）
- v3.4 数据统计与运营报表

## 贡献

- 主仓库：<https://github.com/zzyyun/CampusHelper-Backend>
- Issues / PRs：欢迎提交
- 开发规范：所有注释必须使用简体中文（项目约定）

## 许可证

仅供学习与教学使用，未经授权不得用于商业用途。

---

**作者**：[yun](https://github.com/zzyyun) · **项目状态**：✅ 生产可用（阿里云 ECS 部署中）
