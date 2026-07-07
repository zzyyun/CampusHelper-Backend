# AI 审核服务完善上线 — 测试用例文档

**文档版本**: 1.0
**关联 PRD**: [AI 审核服务完善上线 PRD](../docs/ai-service-completion-prd.md)
**生成日期**: 2026-07-08
**状态**: 初版

---

## 目录

1. [测试范围概述](#1-测试范围概述)
2. [测试策略](#2-测试策略)
3. [功能测试用例（TC-F）](#3-功能测试用例tc-f)
4. [边界测试用例（TC-E）](#4-边界测试用例tc-e)
5. [异常测试用例（TC-ERR）](#5-异常测试用例tc-err)
6. [状态转换测试用例（TC-ST）](#6-状态转换测试用例tc-st)
7. [需求-测试用例覆盖矩阵](#7-需求-测试用例覆盖矩阵)

---

## 1. 测试范围概述

本测试文档覆盖 AI 审核服务（ai-moderation）从"可编译不可运行"到"生产可用"的全部完善工作，共六大功能模块：

| 编号 | 模块 | 描述 |
|------|------|------|
| F1 | Content Service AI 客户端初始化 | 修复 AI 客户端静默失效根因 |
| F2 | ai-moderation 模式自动推导 | 修复 mode 硬编码，支持 Mock/ONNX 双模式 |
| F3 | 异步补判调度器完整实现 | 补全异步链路入口，实现真实 DAO 查询 |
| F4 | 宽限期终止器完整实现 | 补全异步链路出口，实现帖子终态流转 |
| F5 | ai-moderation 容器化与编排 | Dockerfile + docker-compose 接入 |
| F6 | CI/CD Pipeline 集成 | GitHub Actions 自动构建与推送 |

---

## 2. 测试策略

- **单元测试**：覆盖核心业务逻辑（模式推导、DAO 查询、客户端初始化）
- **集成测试**：验证跨模块交互（Content Service → AI 客户端、调度器 → MQ）
- **容器化测试**：验证 Docker 构建、编排启动、健康检查
- **测试工具**：`go test -race -v`、`docker compose`、`grpcurl`
- **外部依赖处理**：数据库/MQ/etcd 使用 mock 或内存替代

---

## 3. 功能测试用例（TC-F）

### TC-F-001

- **标题**: Content Service 启动时成功初始化 AI 客户端
- **需求来源**: Story 1 / Feature 1
- **优先级**: 高
- **前置条件**:
  1. `config/my_config.yaml` 中配置 `service.ai-moderation.address` 为有效的 gRPC 地址
  2. ai-moderation 服务已启动或地址可达
- **测试步骤**:
  1. 执行 `go run ./cmd/content` 启动 Content Service
  2. 观察启动日志
- **预期结果**:
  1. 日志包含 `AI client initialized (addr=xxx)` 且 addr 与配置一致
  2. Content Service 正常启动，不报错退出
  3. 后续发帖时 AI 审核调用正常返回结果（非降级 PASS）

---

### TC-F-002

- **标题**: AI 客户端地址从配置文件正确读取
- **需求来源**: Story 1 / Feature 1
- **优先级**: 高
- **前置条件**:
  1. 配置文件中 `service.ai-moderation.address` 设置为 `localhost:50061`
- **测试步骤**:
  1. 启动 Content Service
  2. 检查日志中输出的地址
- **预期结果**:
  1. 日志显示 `AI client initialized (addr=localhost:50061)`
  2. 客户端实例内部地址字段为 `localhost:50061`

---

### TC-F-003

- **标题**: ai-moderation Mock 模式正确激活
- **需求来源**: Story 2 / Feature 2
- **优先级**: 高
- **前置条件**:
  1. 配置文件中 `aiModeration.enabled=false`
  2. 无 `onnx_enabled` build tag
- **测试步骤**:
  1. 执行 `go run ./cmd/ai-moderation` 启动服务
  2. 观察启动日志
  3. 使用 `grpcurl` 调用 HealthCheck 接口
- **预期结果**:
  1. 启动日志包含 mode=mock 相关信息
  2. HealthCheck 响应中 `mode` 字段为 `mock`
  3. 服务正常监听 gRPC 端口 50061

---

### TC-F-004

- **标题**: ai-moderation ONNX 模式正确激活
- **需求来源**: Story 2 / Feature 2
- **优先级**: 高
- **前置条件**:
  1. 配置文件中 `aiModeration.enabled=true`
  2. 使用 `-tags onnx_enabled` 编译
  3. ONNX 模型文件已放置于配置路径
- **测试步骤**:
  1. 执行 `go run -tags onnx_enabled ./cmd/ai-moderation`
  2. 观察启动日志
  3. 使用 `grpcurl` 调用 HealthCheck 接口
- **预期结果**:
  1. 启动日志包含 mode=onnxruntime 相关信息
  2. HealthCheck 响应中 `mode` 字段为 `onnxruntime`
  3. 模型加载成功日志输出

---

### TC-F-005

- **标题**: Mock/ONNX 模式切换验证
- **需求来源**: Story 2 / Feature 2
- **优先级**: 高
- **前置条件**:
  1. 同一份代码，可分别配置 `enabled=true` 和 `enabled=false`
- **测试步骤**:
  1. 设置 `enabled=false`，启动服务，记录 HealthCheck 的 mode
  2. 停止服务，设置 `enabled=true`（含 build tag），重新启动
  3. 记录 HealthCheck 的 mode
- **预期结果**:
  1. `enabled=false` 时 mode 为 `mock`
  2. `enabled=true` 时 mode 为 `onnxruntime`
  3. 两种模式切换无需修改代码

---

### TC-F-006

- **标题**: HealthCheck 接口返回准确的服务模式
- **需求来源**: Story 2 / Feature 2 / Success Metrics
- **优先级**: 高
- **前置条件**:
  1. ai-moderation 服务已启动
- **测试步骤**:
  1. 使用 `grpcurl` 调用 `grpc.health.v1.Health/Check`
  2. 解析响应中的 mode 字段
- **预期结果**:
  1. 响应 status 为 SERVING
  2. mode 字段与实际运行模式一致（mock 或 onnxruntime）

---

### TC-F-007

- **标题**: 异步调度器正常扫描并入队已发布帖子
- **需求来源**: Story 3 / Feature 3
- **优先级**: 高
- **前置条件**:
  1. 数据库 `posts` 表中存在 `status=2` 且 `created_at` 在近 7 天内的帖子
  2. MQ 服务正常运行
  3. AsyncReviewScheduler 已在 main.go 中启动
- **测试步骤**:
  1. 手动触发异步调度器扫描（或等待定时触发）
  2. 检查 MQ 中是否收到对应消息
- **预期结果**:
  1. 每条帖子生成一条 MQ 消息
  2. 消息包含 `school_id` 和 `post_id` 字段
  3. 日志记录扫描结果（扫描到 N 条帖子）

---

### TC-F-008

- **标题**: 异步调度器按 school_id 分组入队
- **需求来源**: Story 3 / Feature 3 / Acceptance Criteria
- **优先级**: 高
- **前置条件**:
  1. 数据库中存在多个不同 school_id 的待扫描帖子
- **测试步骤**:
  1. 触发调度器扫描
  2. 检查 MQ 消息内容
- **预期结果**:
  1. 每条消息的 `school_id` 与对应帖子的 `school_id` 一致
  2. 同一 school_id 的帖子各自独立入队（非合并为一条消息）

---

### TC-F-009

- **标题**: 异步调度器查询条件正确过滤帖子
- **需求来源**: Story 3 / Feature 3 / SQL 查询
- **优先级**: 高
- **前置条件**:
  1. 数据库中存在以下帖子：
     - 帖子 A: status=2, created_at=昨天, deleted_at=NULL
     - 帖子 B: status=2, created_at=8天前, deleted_at=NULL
     - 帖子 C: status=1 (draft), created_at=昨天, deleted_at=NULL
     - 帖子 D: status=2, created_at=昨天, deleted_at=昨天
- **测试步骤**:
  1. 触发调度器扫描
  2. 检查入队消息列表
- **预期结果**:
  1. 仅帖子 A 被入队
  2. 帖子 B（超过7天）不入队
  3. 帖子 C（非 published 状态）不入队
  4. 帖子 D（已软删除）不入队

---

### TC-F-010

- **标题**: 异步调度器分页限制验证（LIMIT 1000）
- **需求来源**: Story 3 / Feature 3 / Acceptance Criteria
- **优先级**: 中
- **前置条件**:
  1. 数据库中存在超过 1000 条符合条件的帖子
- **测试步骤**:
  1. 触发一次调度器扫描
  2. 统计入队消息数量
- **预期结果**:
  1. 单次扫描入队消息不超过 1000 条
  2. 日志记录扫描批次信息

---

### TC-F-011

- **标题**: 宽限期终止器正常关闭超时帖子
- **需求来源**: Story 4 / Feature 4
- **优先级**: 高
- **前置条件**:
  1. 数据库中存在帖子：status=8 (taken_down_pending), updated_at 早于 24h 前
  2. 对应帖子在 `ai_audit_logs` 中无 `ai_status=3` 的记录
  3. MQ 服务正常运行
- **测试步骤**:
  1. 手动触发宽限期终止器
  2. 检查帖子状态
  3. 检查 MQ 消息
- **预期结果**:
  1. 帖子 status 更新为 `closed`
  2. MQ 发布 `content.taken_down` 事件，包含 post_id 和 school_id
  3. 日志记录终止操作

---

### TC-F-012

- **标题**: 宽限期终止器跳过有申诉的帖子
- **需求来源**: Story 4 / Feature 4 / Acceptance Criteria
- **优先级**: 高
- **前置条件**:
  1. 帖子 status=8, updated_at 早于 24h 前
  2. `ai_audit_logs` 表中存在该帖子的 `ai_status=3` 记录
- **测试步骤**:
  1. 触发宽限期终止器
  2. 检查帖子状态
- **预期结果**:
  1. 帖子 status 仍为 `taken_down_pending`，未变更
  2. 不发布 MQ 事件
  3. 日志记录该帖子因有申诉而跳过

---

### TC-F-013

- **标题**: 宽限期终止器扫描条件正确过滤帖子
- **需求来源**: Story 4 / Feature 4 / SQL 查询
- **优先级**: 高
- **前置条件**:
  1. 帖子 A: status=8, updated_at=25h前, deleted_at=NULL
  2. 帖子 B: status=8, updated_at=23h前, deleted_at=NULL（未超宽限期）
  3. 帖子 C: status=2, updated_at=25h前, deleted_at=NULL（非 pending 状态）
  4. 帖子 D: status=8, updated_at=25h前, deleted_at=昨天（已软删除）
- **测试步骤**:
  1. 触发宽限期终止器
  2. 检查各帖子状态变化
- **预期结果**:
  1. 仅帖子 A 被终止（status 变为 closed）
  2. 帖子 B、C、D 均保持原状态不变

---

### TC-F-014

- **标题**: ai-moderation Dockerfile Mock 模式构建成功
- **需求来源**: Story 5 / Feature 5
- **优先级**: 高
- **前置条件**:
  1. Docker 环境可用
  2. `build/docker/ai-moderation.Dockerfile` 已创建
- **测试步骤**:
  1. 执行 `docker build -f build/docker/ai-moderation.Dockerfile -t ai-moderation:mock .`
  2. 检查构建输出
- **预期结果**:
  1. 构建成功，无错误
  2. 镜像标签 `ai-moderation:mock` 创建成功
  3. Dockerfile 使用 `CGO_ENABLED=0` 编译
  4. 基础镜像为 distroless

---

### TC-F-015

- **标题**: ai-moderation Dockerfile ONNX 模式构建成功
- **需求来源**: Story 5 / Feature 5
- **优先级**: 中
- **前置条件**:
  1. Docker 环境可用
  2. `libonnxruntime` 运行时可在构建阶段安装
- **测试步骤**:
  1. 执行带 ONNX 参数的 `docker build`
  2. 检查构建输出
- **预期结果**:
  1. 多阶段构建成功
  2. 最终镜像包含 `libonnxruntime.so`
  3. 二进制使用 `-tags onnx_enabled` 编译

---

### TC-F-016

- **标题**: docker-compose 中 ai-moderation 服务定义正确
- **需求来源**: Story 5 / Feature 5
- **优先级**: 高
- **前置条件**:
  1. `campus-docker-compose.yaml` 已添加 ai-moderation 服务定义
- **测试步骤**:
  1. 查看 compose 文件中 ai-moderation 服务定义
  2. 检查端口映射、依赖关系、健康检查配置
- **预期结果**:
  1. 端口映射包含 `50061:50061`（gRPC）和 `9091:9091`（metrics）
  2. depends_on 包含 `etcd` 和 `rabbitmq`
  3. 健康检查配置为 gRPC health check
  4. volumes 包含模型文件挂载路径 `/models:/models`
  5. restart 策略为 `unless-stopped`

---

### TC-F-017

- **标题**: docker-compose up 启动 ai-moderation 服务
- **需求来源**: Story 5 / Feature 5
- **优先级**: 高
- **前置条件**:
  1. etcd 和 rabbitmq 服务已启动
  2. 配置文件已正确挂载
- **测试步骤**:
  1. 执行 `docker compose up -d ai-moderation`
  2. 等待服务启动
  3. 执行 `docker compose ps ai-moderation` 检查状态
- **预期结果**:
  1. 容器状态为 running/healthy
  2. 日志无启动报错
  3. gRPC 端口可连通

---

### TC-F-018

- **标题**: CI/CD build matrix 包含 ai-moderation
- **需求来源**: Story 6 / Feature 6
- **优先级**: 高
- **前置条件**:
  1. `.github/workflows/deploy.yaml` 已更新
- **测试步骤**:
  1. 查看 deploy.yaml 中的 build matrix 定义
  2. 检查 service 列表
- **预期结果**:
  1. build matrix 中包含 `ai-moderation` 选项
  2. 构建命令使用 `CGO_ENABLED=0 go build ./cmd/ai-moderation`

---

### TC-F-019

- **标题**: workflow_dispatch 下拉菜单包含 ai-moderation 选项
- **需求来源**: Story 6 / Feature 6
- **优先级**: 中
- **前置条件**:
  1. `.github/workflows/deploy.yaml` 已更新
- **测试步骤**:
  1. 在 GitHub Actions 页面查看 workflow_dispatch 触发器
  2. 检查 service 输入参数的 options 列表
- **预期结果**:
  1. 下拉菜单中包含 `ai-moderation` 选项

---

### TC-F-020

- **标题**: CI/CD 构建并推送 ai-moderation 镜像
- **需求来源**: Story 6 / Feature 6 / Success Metrics
- **优先级**: 高
- **前置条件**:
  1. push 到 main 分支
  2. GitHub Actions workflow 正常运行
- **测试步骤**:
  1. 向 main 分支 push 代码
  2. 查看 GitHub Actions 运行记录
  3. 检查 ACR 中的镜像列表
- **预期结果**:
  1. GitHub Actions 中 ai-moderation 构建步骤成功
  2. ACR 中出现新镜像 tag 格式：`v1.0-ai-moderation-{短SHA}`
  3. 镜像可正常拉取

---

### TC-F-021

- **标题**: Content Service 启动时 ai-moderation 地址为空自动跳过初始化
- **需求来源**: Story 1 / Feature 1 / Edge Cases
- **优先级**: 高
- **前置条件**:
  1. 配置文件中 `service.ai-moderation.address` 为空或未配置
- **测试步骤**:
  1. 启动 Content Service
  2. 观察启动日志
- **预期结果**:
  1. 日志包含 WARN 级别日志，提示 AI 客户端地址为空，跳过初始化
  2. Content Service 正常启动，不报错退出
  3. AI 审核调用降级为 PASS（nil client fallback）

---

### TC-F-022

- **标题**: 异步调度器 main.go 中正确启动调度器
- **需求来源**: Story 3 / Feature 3 / Acceptance Criteria
- **优先级**: 高
- **前置条件**:
  1. ai-moderation 服务代码已更新
- **测试步骤**:
  1. 启动 ai-moderation 服务
  2. 观察启动日志
- **预期结果**:
  1. 日志包含异步调度器启动信息
  2. 调度器持有正确的 ctx 和 mqAddr 参数

---

### TC-F-023

- **标题**: 宽限期终止器 main.go 中正确启动终止器
- **需求来源**: Story 4 / Feature 4 / Acceptance Criteria
- **优先级**: 高
- **前置条件**:
  1. ai-moderation 服务代码已更新
- **测试步骤**:
  1. 启动 ai-moderation 服务
  2. 观察启动日志
- **预期结果**:
  1. 日志包含宽限期终止器启动信息
  2. 终止器持有正确的 ctx 和 mqAddr 参数

---

### TC-F-024

- **标题**: AI 审核端到端延迟 P95 <= 800ms
- **需求来源**: Success Metrics / Performance
- **优先级**: 中
- **前置条件**:
  1. ai-moderation 服务正常运行
  2. Jaeger 可观测
- **测试步骤**:
  1. 通过 Content Service 发起 50 次 AI 审核请求
  2. 记录每次请求的耗时
  3. 计算 P95 延迟
- **预期结果**:
  1. P95 延迟 <= 800ms
  2. Jaeger 中可观察到完整 span 链路

---

## 4. 边界测试用例（TC-E）

### TC-E-001

- **标题**: 异步调度器扫描结果恰好为 1000 条
- **需求来源**: Story 3 / Feature 3 / LIMIT 1000
- **优先级**: 中
- **前置条件**:
  1. 数据库中恰好存在 1000 条符合查询条件的帖子
- **测试步骤**:
  1. 触发调度器扫描
  2. 统计入队消息数量
- **预期结果**:
  1. 入队消息恰好 1000 条
  2. 无截断或遗漏

---

### TC-E-002

- **标题**: 异步调度器扫描结果恰好为 1001 条（超限边界）
- **需求来源**: Story 3 / Feature 3 / LIMIT 1000
- **优先级**: 中
- **前置条件**:
  1. 数据库中存在 1001 条符合查询条件的帖子
- **测试步骤**:
  1. 触发调度器扫描
  2. 统计入队消息数量
- **预期结果**:
  1. 单次扫描仅入队 1000 条
  2. 第 1001 条未被包含在本次扫描中
  3. 日志记录本次扫描总数

---

### TC-E-003

- **标题**: 异步调度器扫描结果为空
- **需求来源**: Story 3 / Feature 3 / Edge Cases
- **优先级**: 中
- **前置条件**:
  1. 数据库中无符合条件的帖子
- **测试步骤**:
  1. 触发调度器扫描
  2. 观察日志
- **预期结果**:
  1. 不发送任何 MQ 消息
  2. 日志记录扫描到 0 条帖子
  3. 服务正常运行，不报错

---

### TC-E-004

- **标题**: 宽限期终止器单次处理恰好达到 200 条上限
- **需求来源**: Story 4 / Feature 4 / Performance
- **优先级**: 中
- **前置条件**:
  1. 数据库中恰好存在 200 条 status=8 且超时的帖子
- **测试步骤**:
  1. 触发宽限期终止器
  2. 检查帖子状态变化
- **预期结果**:
  1. 全部 200 条帖子被处理
  2. 状态正确更新为 closed
  3. MQ 事件正常发送

---

### TC-E-005

- **标题**: 帖子宽限期恰好满 24 小时（时间边界）
- **需求来源**: Story 4 / Feature 4
- **优先级**: 中
- **前置条件**:
  1. 帖子 status=8, updated_at 恰好为 24 小时前
  2. 无申诉记录
- **测试步骤**:
  1. 在 updated_at 恰好等于 now-24h 的时间点触发终止器
  2. 检查帖子状态
- **预期结果**:
  1. 根据查询条件 `updated_at < now-24h`，精确 24h 的帖子不被处理（< 而非 <=）
  2. 仅超 24h 的帖子被终止

---

### TC-E-006

- **标题**: 帖子宽限期差 1 秒未满 24 小时
- **需求来源**: Story 4 / Feature 4
- **优先级**: 低
- **前置条件**:
  1. 帖子 status=8, updated_at 为 23h59m59s 前
- **测试步骤**:
  1. 触发宽限期终止器
  2. 检查帖子状态
- **预期结果**:
  1. 帖子状态保持 `taken_down_pending`，不被终止

---

### TC-E-007

- **标题**: 异步调度器扫描 7 天边界帖子（第 7 天与第 8 天）
- **需求来源**: Story 3 / Feature 3 / created_at >= now-7d
- **优先级**: 中
- **前置条件**:
  1. 帖子 A: created_at 恰好为 7 天前
  2. 帖子 B: created_at 为 7 天 + 1 秒前
- **测试步骤**:
  1. 触发调度器扫描
  2. 检查入队结果
- **预期结果**:
  1. 帖子 A（第 7 天）被入队
  2. 帖子 B（超过 7 天）不被入队

---

### TC-E-008

- **标题**: gRPC 服务端口 50061 已被占用时的启动行为
- **需求来源**: Story 5 / Feature 5 / Technical Constraints
- **优先级**: 中
- **前置条件**:
  1. 端口 50061 已被其他进程占用
- **测试步骤**:
  1. 启动 ai-moderation 服务
  2. 观察错误日志
- **预期结果**:
  1. 服务启动失败，日志明确提示端口 50061 已被占用
  2. 进程退出，返回非零退出码

---

### TC-E-009

- **标题**: ONNX 模式下模型文件恰好为最小有效文件
- **需求来源**: Story 2 / Feature 2 / Edge Cases
- **优先级**: 低
- **前置条件**:
  1. 模型文件为合法但极小的 ONNX 模型
- **测试步骤**:
  1. 使用最小有效模型文件启动 ONNX 模式
  2. 执行一次审核请求
- **预期结果**:
  1. 服务正常启动
  2. 推理结果返回（即使结果可能不准确，但流程正常）

---

## 5. 异常测试用例（TC-ERR）

### TC-ERR-001

- **标题**: Content Service 初始化时 gRPC 连接失败
- **需求来源**: Story 1 / Feature 1 / Error Handling
- **优先级**: 高
- **前置条件**:
  1. `service.ai-moderation.address` 配置为不可达地址（如 `localhost:99999`）
  2. ai-moderation 服务未启动
- **测试步骤**:
  1. 启动 Content Service
  2. 观察启动日志
- **预期结果**:
  1. 日志包含 WARN 级别错误，提示 AI 客户端连接失败
  2. Content Service 正常启动，不报错退出
  3. AI 审核调用降级为 PASS（nil client fallback）

---

### TC-ERR-002

- **标题**: ai-moderation enabled=true 但无 onnx_enabled build tag 时启动失败
- **需求来源**: Story 2 / Feature 2 / Edge Cases
- **优先级**: 高
- **前置条件**:
  1. 配置 `aiModeration.enabled=true`
  2. 编译时不使用 `-tags onnx_enabled`
- **测试步骤**:
  1. 执行 `go run ./cmd/ai-moderation`（无 build tag）
  2. 观察错误输出
- **预期结果**:
  1. 启动失败，输出明确错误信息：ONNX 模式需要 `onnx_enabled` build tag
  2. 进程退出，返回非零退出码

---

### TC-ERR-003

- **标题**: ONNX 模式下模型文件不存在时启动失败
- **需求来源**: Story 2 / Feature 2 / Edge Cases
- **优先级**: 高
- **前置条件**:
  1. 配置 `enabled=true`，使用 `onnx_enabled` tag 编译
  2. 模型文件路径配置为不存在的路径
- **测试步骤**:
  1. 启动 ai-moderation 服务
  2. 观察错误日志
- **预期结果**:
  1. 启动失败（fatal），日志明确提示模型文件不存在
  2. 进程退出，返回非零退出码

---

### TC-ERR-004

- **标题**: ONNX 模式下模型 hash 不匹配时启动失败
- **需求来源**: Story 2 / Feature 2 / Edge Cases
- **优先级**: 高
- **前置条件**:
  1. 模型文件存在但内容不匹配预期 hash
- **测试步骤**:
  1. 启动 ai-moderation 服务
  2. 观察错误日志
- **预期结果**:
  1. 启动失败（fatal），日志明确提示模型 hash 不匹配
  2. 进程退出，返回非零退出码

---

### TC-ERR-005

- **标题**: 异步调度器 MQ 发送失败时的服务行为
- **需求来源**: Story 3 / Feature 3 / Error Handling
- **优先级**: 高
- **前置条件**:
  1. MQ 服务不可用或连接断开
  2. 数据库中有符合条件的帖子
- **测试步骤**:
  1. 断开 MQ 连接
  2. 触发调度器扫描
  3. 观察日志和服务状态
- **预期结果**:
  1. 日志记录 MQ 发送失败错误
  2. 调度器不崩溃，服务继续运行
  3. 帖子数据不丢失（数据库不受影响）

---

### TC-ERR-006

- **标题**: 异步调度器数据库查询超时
- **需求来源**: Story 3 / Feature 3 / Technical Constraints
- **优先级**: 中
- **前置条件**:
  1. 数据库响应极慢（可模拟网络延迟或连接池耗尽）
- **测试步骤**:
  1. 模拟数据库慢查询
  2. 触发调度器扫描
  3. 观察日志和服务状态
- **预期结果**:
  1. 查询超时后记录错误日志
  2. 调度器不崩溃，等待下一个执行周期重试
  3. 服务保持正常运行

---

### TC-ERR-007

- **标题**: 宽限期终止器数据库状态更新失败
- **需求来源**: Story 4 / Feature 4 / Error Handling
- **优先级**: 高
- **前置条件**:
  1. 帖子满足终止条件
  2. 数据库写入失败（模拟约束冲突或连接中断）
- **测试步骤**:
  1. 模拟 UpdateStatus 失败
  2. 触发宽限期终止器
  3. 检查帖子状态和日志
- **预期结果**:
  1. 日志记录状态更新失败
  2. 帖子状态保持不变（status=8）
  3. 终止器不崩溃，继续处理下一条帖子
  4. MQ 事件不发送（因为状态未成功更新）

---

### TC-ERR-008

- **标题**: 宽限期终止器 MQ 发送失败但状态已更新
- **需求来源**: Story 4 / Feature 4 / Error Handling
- **优先级**: 中
- **前置条件**:
  1. 帖子满足终止条件
  2. 数据库更新成功但 MQ 发送失败
- **测试步骤**:
  1. 模拟 UpdateStatus 成功但 MQ 发送失败
  2. 触发宽限期终止器
  3. 检查帖子状态
- **预期结果**:
  1. 帖子状态已更新为 closed
  2. 日志记录 MQ 发送失败
  3. MQ 事件可通过后续重试机制补偿（或记录待补偿日志）

---

### TC-ERR-009

- **标题**: AI 审核服务不可用时 Content Service 的降级行为
- **需求来源**: Story 1 / Feature 1 / Error Handling
- **优先级**: 高
- **前置条件**:
  1. AI 客户端初始化成功
  2. 运行期间 ai-moderation 服务宕机
- **测试步骤**:
  1. 启动 Content Service（AI 客户端初始化成功）
  2. 停止 ai-moderation 服务
  3. 通过 Content Service 发帖触发 AI 审核
  4. 观察行为
- **预期结果**:
  1. `callAIModeration` 内部熔断器触发降级
  2. 发帖请求不被阻塞，内容正常发布（AI 审核结果标记为 DEGRADED）
  3. 日志记录 AI 调用降级信息

---

### TC-ERR-010

- **标题**: 熔断器连续失败后自动恢复
- **需求来源**: Story 1 / Feature 1 / Technical Constraints
- **优先级**: 中
- **前置条件**:
  1. AI 客户端已初始化并连接 ai-moderation
  2. ai-moderation 服务曾宕机后恢复
- **测试步骤**:
  1. 停止 ai-moderation，发送 5 次审核请求触发熔断
  2. 等待 30s 熔断恢复窗口
  3. 重启 ai-moderation
  4. 再次发送审核请求
- **预期结果**:
  1. 熔断期间请求直接降级
  2. 恢复窗口后尝试半开状态
  3. ai-moderation 恢复后，审核请求恢复正常

---

### TC-ERR-011

- **标题**: docker-compose 中 ai-moderation 容器异常退出后自动重启
- **需求来源**: Story 5 / Feature 5
- **优先级**: 中
- **前置条件**:
  1. ai-moderation 容器已通过 docker compose 启动
  2. restart 策略为 unless-stopped
- **测试步骤**:
  1. 执行 `docker compose exec ai-moderation kill -SIGTERM 1` 模拟进程退出
  2. 等待 10 秒
  3. 检查容器状态
- **预期结果**:
  1. 容器自动重启
  2. 重启后服务恢复正常
  3. 日志记录重启事件

---

### TC-ERR-012

- **标题**: docker-compose 缺少 etcd 或 rabbitmq 依赖时的服务启动行为
- **需求来源**: Story 5 / Feature 5
- **优先级**: 中
- **前置条件**:
  1. etcd 或 rabbitmq 未启动
- **测试步骤**:
  1. 执行 `docker compose up ai-moderation`（不启动依赖服务）
  2. 观察容器状态
- **预期结果**:
  1. 取决于 depends_on 配置（若使用 health check 依赖则启动被阻止）
  2. 或服务启动但依赖检查失败，日志记录连接错误
  3. 不出现未处理的 panic

---

### TC-ERR-013

- **标题**: CI/CD 构建 ai-moderation 测试阶段失败阻止镜像推送
- **需求来源**: Story 6 / Feature 6
- **优先级**: 高
- **前置条件**:
  1. ai-moderation 包中存在测试失败的代码
- **测试步骤**:
  1. 向 main 分支 push 包含测试失败的代码
  2. 观察 GitHub Actions 运行结果
- **预期结果**:
  1. CI test 阶段失败
  2. 后续 build 和 push 阶段不执行
  3. ACR 中不出现新镜像 tag

---

## 6. 状态转换测试用例（TC-ST）

### TC-ST-001

- **标题**: AI 客户端状态转换：未初始化 → 已初始化
- **需求来源**: Story 1 / Feature 1
- **优先级**: 高
- **前置条件**:
  1. Content Service 启动，配置有效
- **测试步骤**:
  1. 在 `InitAIClient` 调用前后分别检查 AI 客户端状态
  2. 验证全局变量变化
- **预期结果**:
  1. 调用前 AI 客户端为 nil
  2. 调用后 AI 客户端为有效实例
  3. 后续审核调用不再走 nil client fallback

---

### TC-ST-002

- **标题**: AI 客户端状态转换：已初始化 → 降级（DEGRADED）
- **需求来源**: Story 1 / Feature 1 / Error Handling
- **优先级**: 高
- **前置条件**:
  1. AI 客户端已成功初始化
  2. ai-moderation 服务运行中
- **测试步骤**:
  1. 停止 ai-moderation 服务
  2. 通过 Content Service 发帖触发 AI 审核
  3. 检查审核结果状态
- **预期结果**:
  1. AI 审核调用失败
  2. 返回降级结果（DEGRADED），内容正常发布
  3. 熔断器计数递增

---

### TC-ST-003

- **标题**: AI 审核模式转换：Mock → ONNX（配置变更）
- **需求来源**: Story 2 / Feature 2
- **优先级**: 中
- **前置条件**:
  1. 服务当前以 Mock 模式运行
  2. ONNX 模型文件和 build tag 已准备
- **测试步骤**:
  1. 停止服务
  2. 修改配置 `enabled=true`，使用 `onnx_enabled` tag 重新编译
  3. 重启服务
  4. 调用 HealthCheck
- **预期结果**:
  1. 重启后 HealthCheck mode 从 `mock` 变为 `onnxruntime`
  2. 审核请求使用真实推理

---

### TC-ST-004

- **标题**: 帖子状态转换：published → taken_down_pending → closed
- **需求来源**: Story 4 / Feature 4
- **优先级**: 高
- **前置条件**:
  1. 帖子 status=2 (published)
  2. AI 异步审核判定违规，触发下架
  3. 进入宽限期（status=8）
- **测试步骤**:
  1. 确认帖子状态为 taken_down_pending
  2. 等待 24h 宽限期（或模拟时间推进）
  3. 触发宽限期终止器
  4. 检查帖子最终状态
- **预期结果**:
  1. 24h 后终止器将 status 从 `taken_down_pending` 更新为 `closed`
  2. 发布 `content.taken_down` MQ 事件
  3. 帖子不再展示在平台中

---

### TC-ST-005

- **标题**: 帖子状态转换：taken_down_pending + 申诉 → 保持 pending
- **需求来源**: Story 4 / Feature 4 / Acceptance Criteria
- **优先级**: 高
- **前置条件**:
  1. 帖子 status=8 (taken_down_pending)
  2. `ai_audit_logs` 中有 `ai_status=3` 记录（申诉中）
  3. 已超 24h 宽限期
- **测试步骤**:
  1. 触发宽限期终止器
  2. 检查帖子状态
- **预期结果**:
  1. 帖子状态保持 `taken_down_pending`
  2. 不发布 MQ 事件
  3. 日志记录因申诉跳过该帖子
  4. 等待后续申诉处理流程

---

### TC-ST-006

- **标题**: AI 服务健康状态转换：SERVING → NOT_SERVING → SERVING
- **需求来源**: Story 2 / Feature 2 / Success Metrics
- **优先级**: 中
- **前置条件**:
  1. ai-moderation 服务正常运行
- **测试步骤**:
  1. 正常运行时调用 HealthCheck，确认 SERVING
  2. 模拟异常（如内存溢出或资源耗尽）
  3. 恢复正常后再次调用 HealthCheck
- **预期结果**:
  1. 正常时返回 SERVING
  2. 异常时返回 NOT_SERVING 或连接失败
  3. 恢复后重新返回 SERVING

---

### TC-ST-007

- **标题**: 异步调度器执行状态：空闲 → 扫描中 → 空闲
- **需求来源**: Story 3 / Feature 3
- **优先级**: 低
- **前置条件**:
  1. AsyncReviewScheduler 已启动
- **测试步骤**:
  1. 观察调度器日志中的状态变更
  2. 在扫描执行期间检查状态
- **预期结果**:
  1. 非执行期间日志显示空闲/等待状态
  2. 触发扫描时日志显示扫描开始
  3. 扫描完成后日志显示扫描结束及结果统计
  4. 服务恢复正常空闲状态

---

## 7. 需求-测试用例覆盖矩阵

| 需求编号 | 需求描述 | 测试用例 | 状态 |
|----------|----------|----------|------|
| **Story 1**: Content Service AI 客户端初始化 | | | |
| AC1-1 | cmd/content/main.go 调用 InitAIClient | TC-F-001, TC-ST-001 | 已覆盖 |
| AC1-2 | AI 客户端地址从配置读取 | TC-F-002 | 已覆盖 |
| AC1-3 | 初始化失败不阻止启动 | TC-F-021, TC-ERR-001 | 已覆盖 |
| AC1-4 | 启动日志包含初始化信息 | TC-F-001, TC-F-002 | 已覆盖 |
| AC1-5 | 单元测试覆盖三种场景 | TC-F-021, TC-ERR-001, TC-ST-001 | 已覆盖 |
| 运行时降级 | AI 服务不可用时降级 | TC-ST-002, TC-ERR-009, TC-ERR-010 | 已覆盖 |
| **Story 2**: ONNX 模式正确激活 | | | |
| AC2-1 | 根据 enabled 配置选择模式 | TC-F-003, TC-F-004, TC-F-005 | 已覆盖 |
| AC2-2 | mode 参数从配置动态获取 | TC-F-005, TC-ST-003 | 已覆盖 |
| AC2-3 | HealthCheck mode 字段准确 | TC-F-006, TC-ST-006 | 已覆盖 |
| AC2-4 | 配置文件添加 mode 字段 | TC-F-003, TC-F-004 | 已覆盖 |
| AC2-5 | Mock/ONNX 均通过测试 | TC-F-003, TC-F-004 | 已覆盖 |
| 配置缺失 | enabled 配置缺失默认 false | TC-F-003 | 已覆盖 |
| 模型文件不存在 | 启动失败 | TC-ERR-003 | 已覆盖 |
| 模型 hash 不匹配 | 启动失败 | TC-ERR-004 | 已覆盖 |
| 无 build tag | enabled=true 无 tag 启动失败 | TC-ERR-002 | 已覆盖 |
| **Story 3**: 异步补判调度器完整实现 | | | |
| AC3-1 | scanRecentPublishedPosts 实现真实 DAO | TC-F-007, TC-F-009 | 已覆盖 |
| AC3-2 | 查询条件正确 | TC-F-009, TC-E-007 | 已覆盖 |
| AC3-3 | 按 school_id 分组入队 | TC-F-008 | 已覆盖 |
| AC3-4 | LIMIT 防止全量扫描 | TC-F-010, TC-E-001, TC-E-002 | 已覆盖 |
| AC3-5 | main.go 启动调度器 | TC-F-022 | 已覆盖 |
| AC3-6 | 单元测试覆盖三种场景 | TC-F-007, TC-E-003, TC-ERR-005 | 已覆盖 |
| 空结果 | 扫描结果为空 | TC-E-003 | 已覆盖 |
| MQ 发送失败 | MQ 不可用时行为 | TC-ERR-005 | 已覆盖 |
| DB 查询超时 | 数据库超时处理 | TC-ERR-006 | 已覆盖 |
| **Story 4**: 宽限期终止器完整实现 | | | |
| AC4-1 | scanTakenDownPendingPosts 实现真实 DAO | TC-F-011, TC-F-013 | 已覆盖 |
| AC4-2 | 查询条件正确 | TC-F-013, TC-E-005, TC-E-006 | 已覆盖 |
| AC4-3 | hasAppeal 查询申诉标记 | TC-F-012, TC-ST-005 | 已覆盖 |
| AC4-4 | 无申诉 → closed + MQ 事件 | TC-F-011, TC-ST-004 | 已覆盖 |
| AC4-5 | 有申诉 → 跳过 | TC-F-012, TC-ST-005 | 已覆盖 |
| AC4-6 | main.go 启动终止器 | TC-F-023 | 已覆盖 |
| AC4-7 | 单元测试覆盖三种场景 | TC-F-011, TC-F-012, TC-ERR-007 | 已覆盖 |
| 状态更新失败 | UpdateStatus 失败 | TC-ERR-007 | 已覆盖 |
| MQ 发送失败 | 状态已更新但 MQ 失败 | TC-ERR-008 | 已覆盖 |
| **Story 5**: ai-moderation 容器化与编排 | | | |
| AC5-1 | Dockerfile Mock 模式构建 | TC-F-014 | 已覆盖 |
| AC5-2 | CGO_ENABLED=0 静态构建 | TC-F-014 | 已覆盖 |
| AC5-3 | ONNX 模式多阶段构建 | TC-F-015 | 已覆盖 |
| AC5-4 | docker-compose 服务定义 | TC-F-016 | 已覆盖 |
| AC5-5 | 端口映射 50061/9091 | TC-F-016, TC-E-008 | 已覆盖 |
| AC5-6 | 依赖 etcd/rabbitmq | TC-F-016, TC-ERR-012 | 已覆盖 |
| AC5-7 | gRPC 健康检查 | TC-F-016 | 已覆盖 |
| AC5-8 | 模型文件 volume 挂载 | TC-F-016 | 已覆盖 |
| 启动成功 | docker compose up 正常启动 | TC-F-017 | 已覆盖 |
| 异常重启 | 容器退出后自动重启 | TC-ERR-011 | 已覆盖 |
| **Story 6**: CI/CD Pipeline 集成 | | | |
| AC6-1 | build matrix 添加 ai-moderation | TC-F-018 | 已覆盖 |
| AC6-2 | workflow_dispatch 下拉菜单 | TC-F-019 | 已覆盖 |
| AC6-3 | 镜像 tag 格式 | TC-F-020 | 已覆盖 |
| AC6-4 | Mock 模式不依赖 CGO | TC-F-018 | 已覆盖 |
| AC6-5 | CI test 覆盖 ai-moderation | TC-ERR-013 | 已覆盖 |
| **Success Metrics** | | | |
| M1 | AI 客户端初始化成功率 100% | TC-F-001, TC-F-002, TC-F-021 | 已覆盖 |
| M2 | 同步审核延迟 P95 <= 800ms | TC-F-024 | 已覆盖 |
| M3 | 健康检查通过 SERVING | TC-F-006, TC-ST-006 | 已覆盖 |
| M4 | Mock/ONNX 双模式可切换 | TC-F-005, TC-ST-003 | 已覆盖 |
| M5 | 异步调度器每日执行 | TC-F-007, TC-F-022 | 已覆盖 |
| M6 | 宽限期终止器每小时执行 | TC-F-011, TC-F-023 | 已覆盖 |
| M7 | Docker 构建成功 | TC-F-014, TC-F-015 | 已覆盖 |
| M8 | docker-compose 编排成功 | TC-F-017 | 已覆盖 |
| M9 | CI/CD 构建推送镜像 | TC-F-020 | 已覆盖 |

---

## 测试统计

| 类别 | 数量 |
|------|------|
| 功能测试（TC-F） | 24 |
| 边界测试（TC-E） | 9 |
| 异常测试（TC-ERR） | 13 |
| 状态转换测试（TC-ST） | 7 |
| **合计** | **53** |

---

*本文档基于 AI 审核服务完善上线 PRD v1.0 生成，确保 6 个 User Story、9 项 Success Metrics 的全面覆盖。*
