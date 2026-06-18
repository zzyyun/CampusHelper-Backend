# Project: CampusHelper-Backend (校园互助平台后端)

## 1. 项目概述
本项目是一个面向高校大学生的微信小程序后端系统，采用 Go 微服务架构。核心业务包括校园社区（失物招领/二手）、任务悬赏（跑腿/拼车）和深度消息互动。
**核心架构特性**：
- **多租户隔离**：不同学校展示不同内容，所有业务数据强制按 `school_id` 隔离。
- **全链路追踪**：TraceID 贯穿 HTTP网关 -> gRPC内部调用 -> RabbitMQ异步消息 -> 消费者。

## 2. 技术栈
- **Language**: Go 1.22+
- **Gateway**: Gin (HTTP/RESTful API)
- **Internal RPC**: gRPC + Protobuf
- **Service Discovery**: etcd
- **Database**: MySQL 8.0 (GORM), Redis 7 (go-redis/v9)
- **Message Queue**: RabbitMQ (amqp091-go)
- **Search Engine**: Elasticsearch 8 (go-elasticsearch)
- **Tracing**: OpenTelemetry + Jaeger
- **Logging**: log (结构化日志，结合 TraceID和rabbitmq)
- **Config**: Viper

## 3. 目录结构 (Monorepo)
```text
/
├── cmd/                   # 微服务启动入口 (main.go)
│   ├── gateway/           # API 网关,统一处理路由
│   ├── user/              # 用户服务
│   ├── content/           # 内容服务
│   ├── task/              # 任务服务
│   ├── message/           # 消息服务
│   ├── admin/             # 管理服务
│   └── file/              # 文件服务
├── PB/                   # Protobuf 定义 (.proto) 及生成的代码
│   └── pb/
├── internal/              # 各服务内部业务逻辑 (按服务名划分目录)
│   ├── gateway/           # 路由、中间件、gRPC 客户端调用
│   ├── user/              #  service, model,repo,handler
│   └── ...
├── pkg/                   # 全局公共组件 (严禁包含具体业务逻辑)
│   ├── errcode/           # 统一错误码定义与 gRPC/HTTP 转换
│   ├── contextx/          # Context 扩展 (提取/注入 school_id, user_id, trace_id)
│   ├── middleware/        # Gin 和 gRPC 拦截器 (Auth, Tracing, Logging)
│   ├── mq/                # RabbitMQ 封装 (带 Trace 透传的 Producer/Consumer)
│   ├── es/                # ES 客户端封装
│   └── db/                # GORM 初始化与通用 Scopes (如 SchoolScope)
├── deployments/           # Dockerfile, docker-compose, k8s yaml
├── go.mod
└── Makefile
