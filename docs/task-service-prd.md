# 产品需求文档：Task Service（任务悬赏服务）

**版本**：1.0
**日期**：2026-06-26
**作者**：Sarah（产品负责人）
**质量评分**：91/100

---

## 执行摘要

校园内有大量零散的跑腿、拼车、悬赏需求——代取快递、拼车回家、求人帮忙修电脑——但目前平台没有专门承接这类需求的微服务。Content Service 虽然支持帖子发布，但其审核流程（pending → published）和物品分类（失物招领/二手交易）无法覆盖任务场景的快节奏和强时效性。

Task Service 作为独立微服务填补这个空白：支持跑腿、拼车、悬赏三类任务的发布与接单，采用「不审核直接上架 + 先到先得接单 + 自动过期」的轻量模式。报酬由用户线下协商，平台仅提供信息匹配。

与 Content Service 的关系：任务是独立业务域，不共用帖子数据模型。但复用同一套微服务基础设施（etcd / gRPC / RabbitMQ / MySQL），以及 User Service 的用户鉴权与学校隔离。

---

## 问题陈述

**当前状态**：
- 校园跑腿/拼车需求真实存在，但平台无对应功能
- 强行用 Content Service 帖子发布跑腿任务，缺少接单状态管理和时效机制
- 任务发布与接单者之间缺乏匹配桥梁

**解决方案**：

独立 Task Service（`cmd/task/`），独立数据库（`campus_task`）：
- 三类任务：跑腿（delivery）/ 拼车（carpool）/ 悬赏（bounty）
- 不审核，创建即上架（open 状态）
- 先到先得接单（open → in_progress）
- 任务到期自动过期 goroutine
- 状态变更发布 `task.*` MQ 事件
- 无评论、无支付、无搜索

**业务价值**：
- 满足高频校园任务需求，提升 DAU
- 轻量模式降低运营成本（无需审核团队）
- MQ 事件为后续 Message Service 通知提供数据源

---

## 成功指标

| 指标 | 目标 | 验证方式 |
|------|------|---------|
| 任务创建到上架延迟 | < 1s | 日志监控 |
| 接单 API P95 | < 200ms | 性能测试 |
| 自动过期准确率 | 100% | 定时任务日志 |
| MQ 事件发布覆盖率 | 100% 状态变更必发 | 日志匹配 |

---

## 用户画像

### 主用户 A：任务发布者（雇主）

- **角色**：需要别人帮忙跑腿/拼车的在校学生
- **目标**：快速发布一个跑腿任务，找到愿意接单的人
- **痛点**：发到群里没人理，朋友圈时效短
- **技术层度**：普通（仅使用小程序）

### 主用户 B：任务接单者

- **角色**：愿意顺路帮别人跑腿/拼车来赚点外快的学生
- **目标**：浏览附近待接单任务，选择想做的
- **痛点**：不知道有什么任务可接，接单流程要简单
- **技术层度**：普通

---

## 用户故事与验收标准

### Story 1：发布跑腿任务

**作为** 一名学生
**我想要** 发布一个跑腿任务（如"代取快递 3 号楼"）
**以便于** 有人接单帮我完成

**验收标准：**
- [ ] `POST /api/v1/tasks` 创建任务，必填字段：title、task_type
- [ ] 可选字段：description、location、reward_desc（报酬说明，文本）、**contact**（联系方式，如微信号/手机号）、**note**（留言/备注）
- [ ] 创建时系统提示「请填写联系方式，接单者接单后才能看到」
- [ ] contact 和 note 字段在接单前**完全隐藏**（列表和详情均不可见）
- [ ] 创建后状态为 open（立即上架，无需审核）
- [ ] 同 post_id 的点赞数量
- [ ] 同 school_id 隔离
- [ ] 同 user 的 DFA 敏感词扫描（复用 pkg/sensitive）

### Story 2：浏览待接单任务列表

**作为** 一名潜在接单者
**我想要** 查看当前待接单的任务列表
**以便于** 选择我想做的任务

**验收标准：**
- [ ] `GET /api/v1/tasks` 返回当前 school 的任务列表（status=open）
- [ ] 支持按 task_type 筛选
- [ ] 支持游标分页（与 Content Service 相同的 Base64+JSON 模式）
- [ ] 默认按 created_at DESC 排序，即将过期的靠前

### Story 3：查看任务详情

**作为** 任意用户
**我想要** 查看单个任务的完整信息
**以便于** 了解任务地点、报酬详情后决定是否接单

**验收标准：**
- [ ] `GET /api/v1/tasks/:id` 返回任务完整信息
- [ ] 含发布者 nickname / avatar_url（跨服务调用 User Service）
- [ ] 含当前状态和执行者信息（如有）

### Story 4：接单 + 联系方式交换

**作为** 一名学生
**我想要** 接下一个待接单的任务，并看到发布者的联系方式
**以便于** 联系发布者完成跑腿

**验收标准：**
- [ ] `POST /api/v1/tasks/:id/claim` 接单，请求体含：
  - `contact`（必填）：接单者自己的联系方式（微信号/手机号）
  - `message`（可选）：接单者留言
- [ ] 接单成功后，系统自动将双方的 contact + message/note **互相展示**：
  - 对接单者：返回发布者的 contact + note
  - 对发布者（通过任务详情）：接单者的 contact + message 填入 claimant_contact / claimant_msg
- [ ] 仅 open 状态可接单，in_progress/completed/cancelled/expired 拒绝
- [ ] 发布者本人不可接自己的任务
- [ ] 先到先得：第一个调用的用户锁定，后续请求返回已接单
- [ ] 必须使用数据库乐观锁或事务防止超卖
- [ ] 发布 `task.claimed` MQ 事件（含 claimant_id）

### Story 5：完成任务

**作为** 接单者
**我想要** 将任务标记为已完成
**以便于** 关闭任务

**验收标准：**
- [ ] `PUT /api/v1/tasks/:id/complete` 仅接单者可操作
- [ ] 仅 in_progress 状态可完成
- [ ] 发布 `task.completed` MQ 事件

### Story 6：取消任务

**作为** 发布者或接单者
**我想要** 取消一个任务
**以便于** 任务不再需要时能关闭

**验收标准：**
- [ ] `PUT /api/v1/tasks/:id/cancel` 发布者和接单者均可操作
- [ ] 发布者取消：open / in_progress 均可 → cancelled
- [ ] 接单者取消：仅 in_progress → cancelled（释放任务）
- [ ] 发布 `task.cancelled` MQ 事件

### Story 7：自动过期

**作为** 系统
**我想要** 未在过期时间内被接单的任务自动关闭
**以便于** 列表不出现僵尸任务

**验收标准：**
- [ ] 创建时若未指定 expired_at，默认当前时间 + 24h
- [ ] 每隔 15 分钟运行一次定时任务，扫描 expired_at < NOW() 且 status=open 的任务
- [ ] 将其状态更新为 expired
- [ ] 发布 `task.expired` MQ 事件
- [ ] 启动时立即执行一次

### Story 8：删除任务

**作为** 发布者
**我想要** 删除我的任务
**以便于** 清理不想要的草稿或重复发布

**验收标准：**
- [ ] `DELETE /api/v1/tasks/:id` 仅发布者可操作
- [ ] 仅 open 状态可删除（已有接单者不可删）
- [ ] 软删除

---

## 功能需求

### FR-1：任务数据模型

```go
type TaskStatus int8

const (
    TaskStatusOpen       TaskStatus = 1 // 待接单
    TaskStatusInProgress TaskStatus = 2 // 进行中
    TaskStatusCompleted  TaskStatus = 3 // 已完成
    TaskStatusCancelled  TaskStatus = 4 // 已取消
    TaskStatusExpired    TaskStatus = 5 // 已过期
)

type TaskType string

const (
    TaskTypeDelivery TaskType = "delivery" // 跑腿
    TaskTypeCarpool  TaskType = "carpool"  // 拼车
    TaskTypeBounty   TaskType = "bounty"   // 悬赏
)

type Task struct {
    ID              int64          `gorm:"primaryKey;autoIncrement:false"`
    SchoolID        int64          `gorm:"index;not null"`
    UserID          int64          `gorm:"index;not null"`          // 发布者
    ClaimantID      int64          `gorm:"default:0"`               // 接单者（0=未接单）
    TaskType        string         `gorm:"size:32;not null"`
    Title           string         `gorm:"size:128;not null"`
    Description     string         `gorm:"size:2000;default:''"`
    Location        string         `gorm:"size:256;default:''"`     // 地点
    RewardDesc      string         `gorm:"size:256;default:''"`     // 报酬说明（文字）

    // ── 联系方式与留言（接单前隐藏，接单后对双方可见） ──────────
    Contact         string         `gorm:"size:256;default:''"`     // 发布者联系方式（微信号/手机号）
    Note            string         `gorm:"size:500;default:''"`     // 发布者留言/备注
    ClaimantContact string         `gorm:"size:256;default:''"`     // 接单者联系方式
    ClaimantMsg     string         `gorm:"size:500;default:''"`     // 接单者留言

    Status          TaskStatus     `gorm:"default:1"`
    ExpiredAt       time.Time      // 过期时间（默认创建后 24h）
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       gorm.DeletedAt `gorm:"index"`
}
```

### FR-2：状态机

```
                      ┌─────────┐
        创建          │         │
  ─────────────────▶ │  开  放  │
                      │  (open)  │
                      └────┬────┘
                           │
                    ┌──────┴──────┐
                    │  接单        │
                    ▼              │
              ┌──────────┐        │
              │  进行中   │        │
              │(in_progress)      │
              └─────┬────┘        │
                    │             │
           ┌────────┼────────┐   │
           ▼        ▼        ▼   ▼
      ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐
      │已完成 │ │已取消│ │已过期│ │已取消│
      │      │ │      │ │      │ │      │
      └──────┘ └──────┘ └──────┘ └──────┘
```

**状态转移规则：**
| 当前状态 | 可转移到 | 操作 | 操作者 |
|---------|---------|------|-------|
| open | in_progress | 接单 | 其他用户 |
| open | cancelled | 取消 | 发布者 |
| open | expired | 自动过期 | 系统 |
| open | deleted | 删除 | 发布者 |
| in_progress | completed | 完成 | 接单者 |
| in_progress | cancelled | 取消 | 发布者/接单者 |

### FR-3：RESTful API

所有路由注册在 **Gateway**，JWT 鉴权 + school 绑定（写操作）。

| 方法 | 路由 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/tasks` | CreateTask | 发布任务（需 school 绑定） |
| GET | `/api/v1/tasks` | ListTasks | 任务列表（游标分页，按类型筛选） |
| GET | `/api/v1/tasks/:id` | GetTask | 任务详情（含发布者信息） |
| PUT | `/api/v1/tasks/:id` | UpdateTask | 更新任务（仅发布者，仅 open 状态） |
| DELETE | `/api/v1/tasks/:id` | DeleteTask | 删除任务（仅发布者，仅 open 状态） |
| POST | `/api/v1/tasks/:id/claim` | ClaimTask | 接单 |
| PUT | `/api/v1/tasks/:id/complete` | CompleteTask | 完成任务 |
| PUT | `/api/v1/tasks/:id/cancel` | CancelTask | 取消任务 |

### FR-4：联系方式可见性规则

联系方式和留言字段遵循**接单后可见**原则：

| 字段 | 接单前（open） | 接单后（in_progress/completed） |
|------|---------------|-------------------------------|
| `contact`（发布者联系方式） | ❌ 隐藏 | ✅ 对接单者可见 |
| `note`（发布者留言） | ❌ 隐藏 | ✅ 对接单者可见 |
| `claimant_contact`（接单者联系方式） | —（无值） | ✅ 对发布者可见 |
| `claimant_msg`（接单者留言） | —（无值） | ✅ 对发布者可见 |
| `title` / `description` / `location` | ✅ 所有人可见 | ✅ 所有人可见 |

**实现要点**：
- 列表中（ListTasks）始终不返回 contact/note 字段（即使已接单，列表不展示私密信息）
- 详情中（GetTask）：根据当前请求用户判断——若是发布者或接单者，展示对方的联系方式；否则隐藏
- Claim 响应中直接返回发布者的 contact + note

### FR-6：MQ 事件

| 事件类型 | 触发时机 | Data |
|---------|---------|------|
| `task.created` | 任务创建 | title, task_type |
| `task.claimed` | 接单成功 | claimant_id |
| `task.completed` | 任务完成 | claimant_id |
| `task.cancelled` | 任务取消 | reason |
| `task.expired` | 自动过期 | — |

事件投递到 `task.events` 队列（预留），同时通知类事件投递到 `notification.events`（供 Message Service 消费）。

### FR-7：自动过期

后台 goroutine 每 15 分钟执行一次：
```go
db.Model(&Task{}).Where("status = ? AND expired_at < NOW()", TaskStatusOpen).
    Update("status", TaskStatusExpired)
```
启动时立即执行一次。每次执行后记录日志（过期任务数、耗时）。

### Out of Scope

- ❌ 任务评论/Chat
- ❌ 支付/积分系统
- ❌ ES 搜索（使用 MySQL 列表 + 筛选）
- ❌ 审核流程（直接上架）
- ❌ 任务申诉/仲裁
- ❌ 接单者选择（先到先得）
- ❌ 任务收藏/关注
- ❌ 共享 Content Service 的评论和点赞

---

## 技术约束

### 性能

- 任务列表 P95 < 150ms（MySQL 索引覆盖）
- 接单操作 P95 < 200ms（需事务 + 乐观锁）
- 自动过期每次 < 5s（任务表数据量 < 100 万）

### 安全

- 所有写操作强制 school_id 隔离
- 接单校验：发布者不可接自己的单
- 软删除：保留审计轨迹

### 集成

| 系统 | 集成方式 | 说明 |
|------|---------|------|
| **MySQL** | GORM（独立库 `campus_task`） | 新数据库 |
| **Gateway** | gRPC 客户端 + HTTP 路由 | 8 个端点 |
| **User Service** | gRPC 回调 | 获取发布者 nickname/avatar |
| **RabbitMQ** | 发布 `task.*` 事件 | 通知队列 |
| **Message Service** | 消费 `task.*` 通知事件 | 下期对接 |

### 技术栈

- Go 1.22+
- GORM v2（AutoMigrate `tasks` 表）
- Snowflake ID（复用 `pkg/snowflake`）
- RabbitMQ（复用 `pkg/mq`）
- etcd 服务注册
- DFA 敏感词扫描（复用 `pkg/sensitive`）

---

## MVP 范围与分期

### Phase 1：MVP（本 PRD）

- 任务 CRUD（创建/列表/详情/更新/删除）
- 接单/完成/取消 操作
- 状态机（open → in_progress → completed / cancelled）
- 自动过期 goroutine
- MQ 事件发布（created / claimed / completed / cancelled / expired）
- Gateway 8 个路由
- 单元测试（核心路径）

### 显式 Out of Scope

- ❌ 任务评论（已确认移除）
- ❌ 支付/积分
- ❌ ES 搜索
- ❌ 审核流程
- ❌ 任务申诉

---

## 风险评估

| 风险 | 概率 | 影响 | 缓解策略 |
|------|------|------|---------|
| 接单竞态条件（两人同时接同一任务） | 中 | 高 | 数据库行锁或乐观锁（CAS: status=open AND claimant_id=0） |
| 过期任务清理不及时 | 低 | 低 | 15 分钟扫描间隔，容忍 15 分钟延迟 |
| MQ 事件丢失 | 低 | 中 | 同 Content Service 的 best-effort 模式，降级记录日志 |
| DFA 敏感词扫描漏过 | 低 | 中 | 复用 Content Service 已验证的 DFA 算法 |

---

## 依赖与阻塞

### 依赖

| 依赖项 | 描述 | 状态 |
|--------|------|------|
| MySQL | `campus_task` 数据库（需创建） | 需配置 |
| Snowflake | 任务 ID 生成 | ✅ 已有 |
| RabbitMQ | `task.events` + `notification.events` 队列 | ✅ 已有 |
| User Service | 获取用户 nickname/avatar | ✅ 已有 |
| Gateway | 复用 JWT + RequireSchoolBound 中间件 | ✅ 已有 |

### 已知阻塞

无。所有依赖均已就绪。

---

## 附录

### 配置变更

`my_config.yaml` 新增：

```yaml
mysql:
  databases:
    task: "campus_task"

service:
  task:
    name: "task-service"
    address: "127.0.0.1:50003"
    loadBalance: false
```

### Proto 概要

```protobuf
service TaskService {
  rpc CreateTask (CreateTaskRequest) returns (CreateTaskResponse);
  rpc GetTask (GetTaskRequest) returns (Task);
  rpc ListTasks (ListTasksRequest) returns (ListTasksResponse);
  rpc UpdateTask (UpdateTaskRequest) returns (Task);
  rpc DeleteTask (DeleteTaskRequest) returns (pb.BaseResponse);
  rpc ClaimTask (ClaimTaskRequest) returns (pb.BaseResponse);
  rpc CompleteTask (CompleteTaskRequest) returns (pb.BaseResponse);
  rpc CancelTask (CancelTaskRequest) returns (pb.BaseResponse);
}
```

### 参考文档

- **Gateway Service PRD**：`docs/gateway-service-prd.md`（路由注册模式）
- **Content Service PRD**：`docs/content-service-prd.md`（CRUD + 游标分页模式参考）
- **Message Service PRD**：`docs/message-service-prd.md`（MQ 事件消费对接）

---

*本 PRD 通过 3 轮迭代式需求对话生成，质量评分 91/100。Task Service 采用轻量模式，无审核/无支付/无评论，聚焦任务匹配核心价值。*