# CampusHelper 云端部署方案 - 测试用例文档

**版本**: 1.0
**生成日期**: 2026-07-08
**需求来源**: cloud-deployment-prd.md v1.1
**测试范围**: 云部署全流程（Dockerfile、docker-compose 编排、CI/CD、安全、验证脚本、异常自愈）

---

## 目录

1. [测试用例](#测试用例)
   - [功能测试 (TC-F)](#功能测试-tc-f)
   - [边界测试 (TC-E)](#边界测试-tc-e)
   - [异常测试 (TC-ERR)](#异常测试-tc-err)
   - [状态转换测试 (TC-ST)](#状态转换测试-tc-st)
2. [需求-测试用例覆盖矩阵](#需求-测试用例覆盖矩阵)

---

## 测试用例

### 功能测试 (TC-F)

#### TC-F-001: push main 分支触发 GitHub Actions 流水线

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-001 |
| **标题** | push main 分支自动触发 CI 流水线 |
| **需求来源** | Story 1 (AC1: push 到 main 分支触发 GitHub Actions) / Feature 3 |
| **优先级** | 高 |
| **前置条件** | 1. GitHub 仓库已启用 Actions<br>2. `.github/workflows/deploy.yaml` 已配置<br>3. 代码仓库有 main 分支的 push 权限 |
| **测试步骤** | 1. 在本地修改任意文件<br>2. 执行 `git add . && git commit -m "test: 触发 CI 流水线"`<br>3. 执行 `git push origin main`<br>4. 登录 GitHub 查看 Actions 页面 |
| **预期结果** | 1. Actions 页面出现新的 workflow run<br>2. 触发事件显示为 `push` 到 `main`<br>3. 流水线状态为 running 或 completed |

---

#### TC-F-002: 矩阵构建 6 个微服务镜像

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-002 |
| **标题** | CI 矩阵构建 6 个微服务的 Docker 镜像 |
| **需求来源** | Story 1 (AC2: 6 个服务镜像构建成功) / Feature 1 |
| **优先级** | 高 |
| **前置条件** | 1. CI 流水线已触发<br>2. 6 个服务的 Dockerfile 均存在于 `cmd/{service}/Dockerfile`<br>3. Go 源码无编译错误 |
| **测试步骤** | 1. 触发 CI 流水线（push main 或手动触发）<br>2. 查看 Actions 日志中矩阵构建阶段<br>3. 确认 gateway、user、content、task、message、file 6 个服务的构建 job<br>4. 检查每个 job 的日志确认 `docker build` 成功 |
| **预期结果** | 1. 6 个服务均出现独立的构建 job<br>2. 每个 job 的 exit code 为 0<br>3. 无 Go 编译错误<br>4. 日志中可见 `Successfully built` 和 `Successfully tagged` |

---

#### TC-F-003: 镜像推送至阿里云 ACR 个人版

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-003 |
| **标题** | 构建成功的镜像推送至阿里云 ACR |
| **需求来源** | Story 1 (AC3: 镜像成功推送到阿里云 ACR 个人版) / Feature 3 |
| **优先级** | 高 |
| **前置条件** | 1. CI 流水线镜像构建阶段成功<br>2. GitHub Secrets 已配置 ACR 凭证（ACR_REGISTRY、ACR_NAMESPACE、ACR_USERNAME、ACR_PASSWORD）<br>3. 阿里云 ACR 个人版已创建 |
| **测试步骤** | 1. 等待 CI 流水线进入 push 阶段<br>2. 查看 Actions 日志中 `docker push` 部分<br>3. 登录阿里云 ACR 控制台，检查镜像仓库列表 |
| **预期结果** | 1. 6 个镜像均推送成功，无 `unauthorized` 或 `denied` 错误<br>2. ACR 控制台可见 6 个仓库及对应镜像 tag<br>3. 每个镜像 tag 包含 commit SHA 短哈希 |

---

#### TC-F-004: 镜像 tag 包含 commit SHA 短哈希

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-004 |
| **标题** | 镜像 tag 包含 commit SHA 短哈希标识 |
| **需求来源** | Story 1 (AC4: CI 日志中可见每个镜像的 tag) / Feature 3 |
| **优先级** | 中 |
| **前置条件** | 1. CI 流水线已推送镜像<br>2. 阿里云 ACR 有镜像 |
| **测试步骤** | 1. 查看 Actions 日志中镜像 tag 信息<br>2. 在 ACR 控制台查看各仓库的 tag 列表<br>3. 比对 tag 中的 SHA 与 GitHub commit SHA |
| **预期结果** | 1. 每个镜像 tag 格式为 `{服务名}:{commit_sha_短哈希}` 或类似格式<br>2. tag 中的 SHA 与触发构建的 commit SHA 一致<br>3. 同一次 push 构建的 6 个镜像使用相同的 commit SHA |

---

#### TC-F-005: campus-docker-compose.yaml 编排全部服务

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-005 |
| **标题** | docker-compose 一键拉起 6 个微服务 + 中间件 |
| **需求来源** | Story 2 (AC1: docker compose up 拉起全部服务) / Feature 2 |
| **优先级** | 高 |
| **前置条件** | 1. ECS 上已安装 Docker + Docker Compose v2<br>2. `campus-docker-compose.yaml` 已部署至 `/opt/campus/`<br>3. `.env` 文件已配置真实值<br>4. 镜像已推送至 ACR |
| **测试步骤** | 1. SSH 登录 ECS<br>2. 进入 `/opt/campus/` 目录<br>3. 执行 `docker compose -f campus-docker-compose.yaml pull`<br>4. 执行 `docker compose -f campus-docker-compose.yaml up -d`<br>5. 执行 `docker compose -f campus-docker-compose.yaml ps` |
| **预期结果** | 1. pull 阶段 6 个微服务 + etcd + RabbitMQ + MinIO + ES 镜像拉取成功<br>2. `up -d` 无报错<br>3. `docker compose ps` 显示所有服务状态为 `Up` 或 `running`<br>4. 服务数量 = 10（6 微服务 + 4 中间件） |

---

#### TC-F-006: 所有服务 health check 状态为 healthy

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-006 |
| **标题** | 服务启动后 health check 通过 |
| **需求来源** | Story 2 (AC2: 所有服务 health check 状态为 healthy) / Feature 2 |
| **优先级** | 高 |
| **前置条件** | 1. `docker compose up -d` 已执行<br>2. 所有服务已启动<br>3. 等待 1-2 分钟让 health check 生效 |
| **测试步骤** | 1. 执行 `docker compose -f campus-docker-compose.yaml ps`<br>2. 检查每个服务的 STATUS 列是否显示 `(healthy)`<br>3. 对 gateway 执行 `curl http://localhost:8080/health`<br>4. 依次对 6 个微服务的 health 端口执行健康检查 |
| **预期结果** | 1. 所有服务 STATUS 列显示 `Up (healthy)` 或等效状态<br>2. gateway `/health` 返回 HTTP 200<br>3. 无服务处于 `unhealthy` 或 `starting` 状态（等待超时后） |

---

#### TC-F-007: depends_on 控制服务启动顺序

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-007 |
| **标题** | 服务启动顺序由 depends_on + healthcheck 正确编排 |
| **需求来源** | Story 2 (AC2: depends_on 控制顺序) / Feature 2 (Edge cases) |
| **优先级** | 高 |
| **前置条件** | 1. docker-compose.yaml 中配置了 depends_on 和 healthcheck<br>2. 所有服务的 Dockerfile 中包含 HEALTHCHECK 指令 |
| **测试步骤** | 1. 执行 `docker compose -f campus-docker-compose.yaml down`<br>2. 执行 `docker compose -f campus-docker-compose.yaml up -d`<br>3. 使用 `docker compose logs -f` 观察启动顺序<br>4. 记录每个服务的 Started 时间戳 |
| **预期结果** | 1. 基础设施服务（etcd、RabbitMQ、MinIO、ES）先于微服务启动<br>2. 微服务等待依赖的中间件 healthy 后再启动<br>3. 无微服务因依赖未就绪而反复重启<br>4. gateway 最后启动（依赖所有后端服务） |

---

#### TC-F-008: verify.sh 一键验证服务健康

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-008 |
| **标题** | verify.sh 脚本验证 6 个服务健康状态 |
| **需求来源** | Story 2 (AC4: verify.sh 一键验证) / Feature 5 |
| **优先级** | 高 |
| **前置条件** | 1. 所有服务已启动并 healthy<br>2. `scripts/verify.sh` 已部署至 ECS 并赋予执行权限 |
| **测试步骤** | 1. SSH 登录 ECS<br>2. 执行 `cd /opt/campus && ./scripts/verify.sh`<br>3. 查看脚本输出 |
| **预期结果** | 1. 脚本输出 6 个服务的 health check 结果表格<br>2. 所有服务显示 PASS<br>3. 2 个云数据库（RDS MySQL、Tair Redis）连通性显示 PASS<br>4. 脚本退出码为 0 |

---

#### TC-F-009: verify.sh 主链路业务调用验证

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-009 |
| **标题** | verify.sh 端到端主链路业务断言通过 |
| **需求来源** | Story 2 (AC4) / Story 3 / Success Metrics (端到端主链路) |
| **优先级** | 高 |
| **前置条件** | 1. 所有服务已启动且 healthy<br>2. gateway 可访问<br>3. RDS MySQL 和 Tair Redis 连通 |
| **测试步骤** | 1. 执行 `./scripts/verify.sh`<br>2. 观察脚本中业务断言部分（注册→登录→发帖→读帖→发消息→上传头像）<br>3. 检查每一步的 HTTP 状态码 |
| **预期结果** | 1. 注册接口返回 2xx<br>2. 登录接口返回 2xx 并返回 token<br>3. 发帖接口返回 2xx<br>4. 读帖接口返回 2xx<br>5. 发消息接口返回 2xx<br>6. 上传头像接口返回 2xx<br>7. 所有断言 PASS，脚本退出码为 0 |

---

#### TC-F-010: 公网 health 接口可访问

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-010 |
| **标题** | 公网 IP 访问 gateway health 接口返回 200 |
| **需求来源** | Story 3 (AC1: 公网 health 返回 200) |
| **优先级** | 高 |
| **前置条件** | 1. ECS 安全组已开放 8080 端口<br>2. gateway 服务已启动<br>3. 知道 ECS 公网 IP |
| **测试步骤** | 1. 在本地浏览器或 curl 执行 `curl http://<ECS_IP>:8080/health`<br>2. 检查 HTTP 状态码和响应体 |
| **预期结果** | 1. HTTP 状态码为 200<br>2. 响应体包含服务健康信息（如 `{"status":"ok"}`）<br>3. 响应时间 < 5s |

---

#### TC-F-011: 公网用户注册接口调用

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-011 |
| **标题** | 公网调用用户注册接口返回 2xx |
| **需求来源** | Story 3 (AC2: 公网调用 register 返回 2xx) |
| **优先级** | 高 |
| **前置条件** | 1. gateway 公网可访问<br>2. user 服务已启动<br>3. RDS MySQL 用户库连通 |
| **测试步骤** | 1. 构造注册请求体（包含用户名、密码、school_id 等必填字段）<br>2. 执行 `curl -X POST http://<ECS_IP>:8080/api/v1/user/register -H "Content-Type: application/json" -d '<请求体>'`<br>3. 检查 HTTP 状态码 |
| **预期结果** | 1. HTTP 状态码为 2xx（200 或 201）<br>2. 响应体包含注册成功信息或用户 ID<br>3. 数据库中出现对应用户记录 |

---

#### TC-F-012: 公网发帖接口调用

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-012 |
| **标题** | 公网调用发帖接口返回 2xx |
| **需求来源** | Story 3 (AC3: 公网调用 content/post 返回 2xx) |
| **优先级** | 高 |
| **前置条件** | 1. gateway 公网可访问<br>2. content 服务已启动<br>3. 已有登录 token（通过注册/登录获取） |
| **测试步骤** | 1. 使用已注册账号登录获取 token<br>2. 构造发帖请求体（标题、内容、类型等）<br>3. 执行 `curl -X POST http://<ECS_IP>:8080/api/v1/content/post -H "Content-Type: application/json" -H "Authorization: Bearer <token>" -d '<请求体>'`<br>4. 检查 HTTP 状态码 |
| **预期结果** | 1. HTTP 状态码为 2xx<br>2. 响应体包含帖子 ID 或成功信息<br>3. 数据库中出现对应帖子记录 |

---

#### TC-F-013: 公网发消息接口调用

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-013 |
| **标题** | 公网调用发送消息接口返回 2xx |
| **需求来源** | Story 3 (AC4: 公网调用 message/send 返回 2xx) |
| **优先级** | 高 |
| **前置条件** | 1. gateway 公网可访问<br>2. message 服务已启动<br>3. 已有登录 token |
| **测试步骤** | 1. 使用已注册账号登录获取 token<br>2. 构造消息请求体（接收者、内容等）<br>3. 执行 `curl -X POST http://<ECS_IP>:8080/api/v1/message/send -H "Content-Type: application/json" -H "Authorization: Bearer <token>" -d '<请求体>'`<br>4. 检查 HTTP 状态码 |
| **预期结果** | 1. HTTP 状态码为 2xx<br>2. 响应体包含消息 ID 或成功信息 |

---

#### TC-F-014: 公网文件上传接口调用

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-014 |
| **标题** | 公网调用文件上传接口返回 2xx |
| **需求来源** | Story 3 (AC5: 公网调用 file/upload 返回 2xx) |
| **优先级** | 高 |
| **前置条件** | 1. gateway 公网可访问<br>2. file 服务已启动<br>3. MinIO 已启动<br>4. 已有登录 token |
| **测试步骤** | 1. 准备一个测试图片文件（< 5MB）<br>2. 使用已注册账号登录获取 token<br>3. 执行 `curl -X POST http://<ECS_IP>:8080/api/v1/file/upload -H "Authorization: Bearer <token>" -F "file=@test.png"`<br>4. 检查 HTTP 状态码 |
| **预期结果** | 1. HTTP 状态码为 2xx<br>2. 响应体包含文件 URL 或文件 ID<br>3. MinIO 中出现对应文件 |

---

#### TC-F-015: 公网创建任务接口调用

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-015 |
| **标题** | 公网调用创建任务接口返回 2xx |
| **需求来源** | Story 3 (AC6: 公网调用 task/create 返回 2xx) |
| **优先级** | 高 |
| **前置条件** | 1. gateway 公网可访问<br>2. task 服务已启动<br>3. 已有登录 token |
| **测试步骤** | 1. 使用已注册账号登录获取 token<br>2. 构造任务请求体（任务类型、描述、赏金等）<br>3. 执行 `curl -X POST http://<ECS_IP>:8080/api/v1/task/create -H "Content-Type: application/json" -H "Authorization: Bearer <token>" -d '<请求体>'`<br>4. 检查 HTTP 状态码 |
| **预期结果** | 1. HTTP 状态码为 2xx<br>2. 响应体包含任务 ID 或成功信息 |

---

#### TC-F-016: 公网查询接口调用（读帖/读消息/读任务）

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-016 |
| **标题** | 公网查询类接口（读帖/读消息/读任务）全部返回 2xx |
| **需求来源** | Story 3 (AC7: 公网查询返回 2xx) |
| **优先级** | 高 |
| **前置条件** | 1. gateway 公网可访问<br>2. 已有登录 token<br>3. 之前已创建过帖子、消息和任务 |
| **测试步骤** | 1. 执行 `curl http://<ECS_IP>:8080/api/v1/content/post/<帖子ID> -H "Authorization: Bearer <token>"`<br>2. 执行 `curl http://<ECS_IP>:8080/api/v1/message/list -H "Authorization: Bearer <token>"`<br>3. 执行 `curl http://<ECS_IP>:8080/api/v1/task/<任务ID> -H "Authorization: Bearer <token>"`<br>4. 分别检查 HTTP 状态码 |
| **预期结果** | 1. 读帖接口返回 2xx 和帖子详情<br>2. 读消息列表返回 2xx 和消息列表<br>3. 读任务返回 2xx 和任务详情 |

---

#### TC-F-017: .env 文件不入 git 仓库

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-017 |
| **标题** | .env 文件被 .gitignore 排除，不提交到仓库 |
| **需求来源** | Story 4 (AC1: .env 在 .gitignore 中) |
| **优先级** | 高 |
| **前置条件** | 1. 项目根目录存在 `.gitignore` 文件<br>2. 项目根目录存在 `.env` 文件 |
| **测试步骤** | 1. 检查 `.gitignore` 文件内容是否包含 `.env`<br>2. 执行 `git status` 查看 .env 是否在未跟踪列表<br>3. 执行 `git ls-files .env` 确认未被追踪 |
| **预期结果** | 1. `.gitignore` 中包含 `.env` 规则<br>2. `git status` 不显示 `.env` 为已跟踪文件<br>3. `git ls-files .env` 无输出 |

---

#### TC-F-018: .env.example 提交作为模板

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-018 |
| **标题** | .env.example 模板文件已提交且值留空 |
| **需求来源** | Story 4 (AC2: .env.example 提交作为模板，值留空) |
| **优先级** | 中 |
| **前置条件** | 1. 项目中存在 `.env.example` 文件<br>2. 文件已被 git 追踪 |
| **测试步骤** | 1. 执行 `git ls-files .env.example` 确认已追踪<br>2. 读取 `.env.example` 文件内容<br>3. 检查所有变量的值是否为空或占位符 |
| **预期结果** | 1. `.env.example` 已被 git 追踪<br>2. 文件包含所有必需的环境变量名（如 DB_HOST、DB_PASSWORD、REDIS_HOST 等）<br>3. 所有变量值为空或示例占位符，不含真实凭证 |

---

#### TC-F-019: ECS 上 .env 保存真实配置值

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-019 |
| **标题** | ECS 上 /opt/campus/.env 包含真实配置值 |
| **需求来源** | Story 4 (AC3: ECS 上 /opt/campus/.env 保存真实值) |
| **优先级** | 高 |
| **前置条件** | 1. ECS 已部署<br>2. RDS MySQL 和 Tair Redis 已创建 |
| **测试步骤** | 1. SSH 登录 ECS<br>2. 执行 `cat /opt/campus/.env` 查看配置<br>3. 检查所有必需变量是否有值<br>4. 确认 DB_HOST 指向 RDS 内网地址<br>5. 确认 REDIS_HOST 指向 Tair 内网地址 |
| **预期结果** | 1. `.env` 文件存在于 `/opt/campus/`<br>2. 所有环境变量均有非空值<br>3. 数据库连接串指向 RDS 内网地址（非 localhost）<br>4. Redis 连接串指向 Tair 内网地址（非 localhost）<br>5. 文件权限为 600（仅 owner 可读写） |

---

#### TC-F-020: docker-compose.yaml 通过 env_file 引用 .env

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-020 |
| **标题** | docker-compose 通过 env_file 引用环境变量 |
| **需求来源** | Story 4 (AC4: docker-compose.yaml 通过 env_file: .env 引用) |
| **优先级** | 中 |
| **前置条件** | 1. `campus-docker-compose.yaml` 已部署<br>2. `.env` 文件已配置 |
| **测试步骤** | 1. 读取 `campus-docker-compose.yaml` 内容<br>2. 检查每个服务是否通过 `env_file` 引用 `.env`<br>3. 或检查是否通过 `environment` 中的 `${VAR}` 语法引用<br>4. 启动服务后检查容器内环境变量是否正确注入 |
| **预期结果** | 1. docker-compose.yaml 中通过 `env_file: .env` 或 `${VAR}` 引用环境变量<br>2. 启动后容器内环境变量值与 `.env` 文件一致<br>3. 敏感信息不出现在 docker-compose.yaml 中 |

---

#### TC-F-021: 服务异常崩溃后自动重启

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-021 |
| **标题** | 服务崩溃后自动重启（restart: always） |
| **需求来源** | Story 5 (AC1: 所有服务 restart: always) / Feature 2 |
| **优先级** | 高 |
| **前置条件** | 1. 所有服务已启动并 healthy<br>2. docker-compose.yaml 中配置了 `restart: always` |
| **测试步骤** | 1. 执行 `docker compose -f campus-docker-compose.yaml ps` 记录初始状态<br>2. 执行 `docker kill <gateway容器名>` 模拟崩溃<br>3. 等待 10-30 秒<br>4. 执行 `docker compose -f campus-docker-compose.yaml ps` 检查状态<br>5. 执行 `curl http://localhost:8080/health` 验证服务恢复 |
| **预期结果** | 1. 被 kill 的容器自动重启<br>2. 重启后状态恢复为 running/healthy<br>3. health check 接口返回 200<br>4. 无数据丢失 |

---

#### TC-F-022: etcd 数据挂载 volume 重启不丢

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-022 |
| **标题** | etcd 使用 volume 挂载数据，重启后数据保留 |
| **需求来源** | Story 5 (AC3: etcd 数据挂载 volume，重启不丢) |
| **优先级** | 高 |
| **前置条件** | 1. etcd 已启动<br>2. docker-compose.yaml 中 etcd 有 volume 配置 |
| **测试步骤** | 1. 向 etcd 写入测试数据：`docker exec etcd etcdctl put /test/key "test_value"`<br>2. 读取确认：`docker exec etcd etcdctl get /test/key`<br>3. 执行 `docker compose -f campus-docker-compose.yaml restart etcd`<br>4. 等待 etcd 启动完成<br>5. 再次读取：`docker exec etcd etcdctl get /test/key` |
| **预期结果** | 1. 重启前读取到 `test_value`<br>2. 重启后读取到 `test_value`，数据未丢失<br>3. etcd 服务状态恢复正常 |

---

#### TC-F-023: RabbitMQ 队列持久化

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-023 |
| **标题** | RabbitMQ 队列启用持久化，重启后队列和消息保留 |
| **需求来源** | Story 5 (AC2: RabbitMQ 队列持久化) |
| **优先级** | 高 |
| **前置条件** | 1. RabbitMQ 已启动<br>2. RabbitMQ 管理界面或 CLI 可访问 |
| **测试步骤** | 1. 通过 RabbitMQ 管理界面或 CLI 创建持久化队列<br>2. 发送一条持久化消息到队列<br>3. 确认消息存在于队列中<br>4. 执行 `docker compose -f campus-docker-compose.yaml restart rabbitmq`<br>5. 等待 RabbitMQ 启动完成<br>6. 检查队列是否存在，消息是否保留 |
| **预期结果** | 1. 重启后持久化队列仍然存在<br>2. 持久化消息仍然存在于队列中<br>3. RabbitMQ 服务状态恢复正常 |

---

#### TC-F-024: MinIO 数据挂载 volume 重启不丢

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-024 |
| **标题** | MinIO 使用 volume 挂载数据，重启后文件保留 |
| **需求来源** | Story 5 (AC4: MinIO 数据挂载 volume，重启不丢) |
| **优先级** | 高 |
| **前置条件** | 1. MinIO 已启动<br>2. docker-compose.yaml 中 MinIO 有 volume 配置 |
| **测试步骤** | 1. 上传测试文件到 MinIO 某个 bucket<br>2. 确认文件可正常读取<br>3. 执行 `docker compose -f campus-docker-compose.yaml restart minio`<br>4. 等待 MinIO 启动完成<br>5. 再次读取之前上传的文件 |
| **预期结果** | 1. 重启后 bucket 和文件仍然存在<br>2. 文件内容完整可读<br>3. MinIO 服务状态恢复正常 |

---

#### TC-F-025: ES 数据挂载 volume 重启不丢

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-025 |
| **标题** | Elasticsearch 使用 volume 挂载数据，重启后索引和数据保留 |
| **需求来源** | Story 5 (AC5: ES 数据挂载 volume，重启不丢) |
| **优先级** | 高 |
| **前置条件** | 1. ES 已启动<br>2. docker-compose.yaml 中 ES 有 volume 配置 |
| **测试步骤** | 1. 创建测试索引并写入文档：`curl -X PUT "localhost:9200/test_index/_doc/1" -H 'Content-Type: application/json' -d '{"test":"data"}'`<br>2. 查询确认：`curl "localhost:9200/test_index/_doc/1"`<br>3. 执行 `docker compose -f campus-docker-compose.yaml restart elasticsearch`<br>4. 等待 ES 启动完成（可能需 30-60 秒）<br>5. 再次查询：`curl "localhost:9200/test_index/_doc/1"` |
| **预期结果** | 1. 重启后测试索引和文档仍然存在<br>2. 查询返回原始数据<br>3. ES 服务状态恢复正常 |

---

#### TC-F-026: RDS MySQL 5 个独立数据库

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-026 |
| **标题** | RDS MySQL 创建 5 个独立数据库（每服务一个） |
| **需求来源** | Feature 4 / Integration (阿里云 RDS MySQL 8.0，5 个独立数据库) / 架构约定 |
| **优先级** | 高 |
| **前置条件** | 1. 阿里云 RDS for MySQL 已创建<br>2. 有数据库管理权限 |
| **测试步骤** | 1. 登录 RDS 数据库管理控制台或使用 MySQL 客户端连接<br>2. 执行 `SHOW DATABASES;`<br>3. 确认存在 5 个独立业务数据库（如 user_db、content_db、task_db、message_db、file_db）<br>4. 每个数据库分别连接测试 |
| **预期结果** | 1. 存在 5 个独立的业务数据库<br>2. 每个数据库可独立连接<br>3. 各数据库表结构完整<br>4. 无跨库直接访问 |

---

#### TC-F-027: Tair/Redis 连通性

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-027 |
| **标题** | Tair/Redis 实例连通性验证 |
| **需求来源** | Feature 4 / Integration (阿里云 Tair / Redis 7.0) |
| **优先级** | 高 |
| **前置条件** | 1. 阿里云 Tair/Redis 实例已创建<br>2. 安全组允许 ECS 内网 IP 访问 |
| **测试步骤** | 1. 从 ECS 上使用 redis-cli 连接 Tair<br>2. 执行 `PING` 命令<br>3. 执行 `SET test_key "test_value"` 和 `GET test_key`<br>4. 检查连接延迟 |
| **预期结果** | 1. PING 返回 PONG<br>2. SET/GET 操作成功<br>3. 连接延迟 < 10ms（内网）<br>4. 无连接超时或拒绝 |

---

#### TC-F-028: GitHub Actions 流水线耗时

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-028 |
| **标题** | CI 流水线总耗时小于 5 分钟 |
| **需求来源** | Story 1 (AC5: CI 总耗时 < 5 分钟) |
| **优先级** | 中 |
| **前置条件** | 1. GitHub Actions 已触发完整流水线<br>2. 6 个服务代码正常 |
| **测试步骤** | 1. 触发一次完整的 CI 流水线<br>2. 在 Actions 页面查看 workflow run 的总耗时<br>3. 记录各阶段耗时（checkout、build matrix、push） |
| **预期结果** | 1. workflow run 总耗时 < 5 分钟<br>2. 各构建 job 并行执行<br>3. 如使用 Go build cache，构建时间进一步缩短 |

---

#### TC-F-029: 阿里云安全组配置

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-029 |
| **标题** | ECS 安全组仅开放必要端口 |
| **需求来源** | Security (网络隔离：外部仅暴露 22 + 8080 + 9200 + 9000) / Feature 2 (端口冲突) |
| **优先级** | 高 |
| **前置条件** | 1. 阿里云 ECS 已创建<br>2. 安全组已配置 |
| **测试步骤** | 1. 登录阿里云控制台查看 ECS 安全组规则<br>2. 确认入方向规则仅包含：22（SSH，限 IP）、8080（gateway）、9200（ES）、9000（MinIO）<br>3. 尝试从外部访问未开放端口（如 3306） |
| **预期结果** | 1. 安全组仅开放 22、8080、9200、9000 端口<br>2. 22 端口限制特定 IP 访问<br>3. 3306（MySQL）、6379（Redis）等端口未对外开放<br>4. 未开放端口从外部无法连接 |

---

#### TC-F-030: RDS 数据库白名单配置

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-030 |
| **标题** | RDS / Tair 安全组仅允许 ECS 内网 IP 访问 |
| **需求来源** | Security (数据库白名单：RDS / Tair 安全组仅允许 ECS 内网 IP) |
| **优先级** | 高 |
| **前置条件** | 1. RDS 和 Tair 已创建<br>2. ECS 内网 IP 已知 |
| **测试步骤** | 1. 查看 RDS 白名单配置，确认仅包含 ECS 内网 IP<br>2. 查看 Tair 白名单配置，确认仅包含 ECS 内网 IP<br>3. 从外部 IP 尝试连接 RDS（应被拒绝）<br>4. 从外部 IP 尝试连接 Tair（应被拒绝） |
| **预期结果** | 1. RDS 白名单仅包含 ECS 内网 IP<br>2. Tair 白名单仅包含 ECS 内网 IP<br>3. 外部 IP 无法连接 RDS 和 Tair<br>4. ECS 内网可以正常连接 |

---

#### TC-F-031: ACR 凭证存储在 GitHub Secrets

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-031 |
| **标题** | ACR 凭证使用 GitHub Secrets 存储 |
| **需求来源** | Security (ACR 凭证：GitHub Actions Secrets 存储阿里云 RAM AccessKey) |
| **优先级** | 高 |
| **前置条件** | 1. GitHub 仓库 Settings → Secrets 已配置<br>2. 阿里云 RAM 子账号已创建 |
| **测试步骤** | 1. 检查 GitHub Secrets 中包含 ACR_REGISTRY、ACR_NAMESPACE、ACR_USERNAME、ACR_PASSWORD<br>2. 确认 CI workflow 中通过 `${{ secrets.XXX }}` 引用<br>3. 确认代码中无硬编码的 ACR 凭证<br>4. 检查 RAM 子账号权限是否最小化 |
| **预期结果** | 1. Secrets 列表包含所有 ACR 相关凭证<br>2. CI 日志中凭证被遮蔽（显示 ***）<br>3. 代码仓库中无明文凭证<br>4. RAM 子账号仅有 ACR push 权限 |

---

#### TC-F-032: 镜像多阶段构建（builder + distroless）

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-032 |
| **标题** | Dockerfile 使用多阶段构建，最终镜像基于 distroless |
| **需求来源** | Feature 1 (多阶段 Dockerfile：builder + distroless 最终镜像) |
| **优先级** | 中 |
| **前置条件** | 1. 6 个服务的 Dockerfile 已编写 |
| **测试步骤** | 1. 检查每个 Dockerfile 是否包含多阶段构建（FROM golang AS builder + FROM distroless 或类似基础镜像）<br>2. 构建镜像后检查最终镜像层<br>3. 检查最终镜像中是否包含 Go 编译工具链（应不包含） |
| **预期结果** | 1. 每个 Dockerfile 包含至少 2 个 FROM 阶段<br>2. 最终镜像不包含 Go 编译器、源码等<br>3. 最终镜像仅包含编译后的二进制文件和必要运行时依赖 |

---

#### TC-F-033: 部署验证脚本失败时返回非 0 退出码

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-033 |
| **标题** | verify.sh 脚本在验证失败时返回非 0 退出码 |
| **需求来源** | Feature 5 (Error handling: 脚本返回非 0 退出码，方便 CI 集成) |
| **优先级** | 中 |
| **前置条件** | 1. verify.sh 已部署<br>2. 故意制造一个服务不健康的情况（如停止某服务） |
| **测试步骤** | 1. 停止 gateway 服务：`docker stop <gateway容器>`<br>2. 执行 `./scripts/verify.sh`<br>3. 检查退出码：`echo $?`<br>4. 重新启动 gateway<br>5. 再次执行 `./scripts/verify.sh`<br>6. 检查退出码 |
| **预期结果** | 1. 服务不健康时脚本返回非 0 退出码<br>2. 脚本输出中标注失败的服务为 FAIL<br>3. 全部健康时脚本返回 0<br>4. CI 可根据退出码判断构建结果 |

---

#### TC-F-034: 回滚点保留上一个版本镜像

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-034 |
| **标题** | 保留上一个版本镜像用于回滚 |
| **需求来源** | Story 2 (AC5: 镜像保留上一个版本作为回滚点) |
| **优先级** | 中 |
| **前置条件** | 1. ACR 中已有至少两个版本的镜像<br>2. 已知上一次构建的 tag |
| **测试步骤** | 1. 登录 ACR 控制台，查看某服务的镜像 tag 列表<br>2. 确认存在至少两个不同 tag 的镜像<br>3. 记录最新 tag 和上一个 tag |
| **预期结果** | 1. ACR 中同一服务存在至少两个版本的镜像 tag<br>2. 可通过 `docker pull <镜像>:<旧tag>` 拉取上一版本<br>3. 旧版本镜像可正常运行 |

---

#### TC-F-035: runbook 部署文档可复现

| 字段 | 内容 |
|------|------|
| **编号** | TC-F-035 |
| **标题** | runbook 部署文档完整且可按步骤复现部署 |
| **需求来源** | Problem Statement (Proposed Solution #5: 编写部署验证脚本) / Phase 1 (runbook 部署文档) / User Persona 3 |
| **优先级** | 中 |
| **前置条件** | 1. runbook 文档已编写<br>2. 一台全新的 ECS 实例 |
| **测试步骤** | 1. 由未参与部署的同学阅读 runbook<br>2. 按照 runbook 步骤从零开始部署<br>3. 记录每步是否可成功执行<br>4. 最终执行 verify.sh 验证 |
| **预期结果** | 1. runbook 包含所有必要步骤<br>2. 按步骤可成功部署全套服务<br>3. 无遗漏步骤或歧义描述<br>4. verify.sh 全部 PASS |

---

### 边界测试 (TC-E)

#### TC-E-001: 镜像大小不超过 50MB

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-001 |
| **标题** | 微服务最终镜像大小不超过 50MB |
| **需求来源** | Feature 1 (Edge cases: 镜像超过 200MB → 警告，多阶段构建保证 < 50MB) |
| **优先级** | 高 |
| **前置条件** | 1. 6 个服务镜像已构建完成 |
| **测试步骤** | 1. 对每个服务执行 `docker images <服务名> --format "{{.Repository}}:{{.Tag}} {{.Size}}"`<br>2. 记录每个镜像的实际大小<br>3. 与 50MB 阈值对比 |
| **预期结果** | 1. 每个微服务镜像大小 < 50MB<br>2. 如存在超过 50MB 的镜像，CI 输出警告<br>3. 所有镜像总大小 < 300MB |

---

#### TC-E-002: ECS 资源在 4 核 8G 内运行全部服务

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-002 |
| **标题** | 10 个容器在 4 核 8G ECS 上的资源使用不超限 |
| **需求来源** | Technical Constraints - Performance (4 核 8G 资源分配方案) |
| **优先级** | 高 |
| **前置条件** | 1. 所有 10 个容器已启动<br>2. ECS 上安装了 docker stats 或 htop |
| **测试步骤** | 1. 执行 `docker stats --no-stream` 查看各容器资源使用<br>2. 执行 `free -h` 查看系统内存使用<br>3. 执行 `top` 或 `htop` 查看 CPU 使用<br>4. 观察 5 分钟内的资源波动 |
| **预期结果** | 1. 总内存使用 < 7G（系统预留 ~1G）<br>2. 总 CPU 使用 < 3.5 核（4 核的合理水位）<br>3. 无 OOM killer 触发<br>4. 各容器未超出分配的 CPU/内存限制（如有设置） |

---

#### TC-E-003: 并发 50 用户访问

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-003 |
| **标题** | 支持 10-50 并发用户请求 |
| **需求来源** | Technical Constraints - Performance (并发量：支持 10-50 并发用户) |
| **优先级** | 中 |
| **前置条件** | 1. 所有服务已启动且健康<br>2. 安装了压测工具（如 wrk、ab 或 hey） |
| **测试步骤** | 1. 使用 hey 或 ab 发送 50 个并发请求到 `/health`<br>2. 使用 hey 或 ab 发送 50 个并发请求到业务接口（如读帖）<br>3. 记录响应时间和成功率 |
| **预期结果** | 1. 50 并发下 health 接口成功率 > 99%<br>2. 单接口 P95 响应时间 < 1s<br>3. 无 5xx 错误<br>4. 服务无崩溃或重启 |

---

#### TC-E-004: 单接口 P95 响应时间小于 1 秒

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-004 |
| **标题** | 单接口 P95 响应时间 < 1s |
| **需求来源** | Technical Constraints - Performance (响应时间：单接口 P95 < 1s) |
| **优先级** | 中 |
| **前置条件** | 1. 所有服务已启动<br>2. 已安装性能测试工具 |
| **测试步骤** | 1. 对 `/health` 接口发送 100 个请求<br>2. 对 `/api/v1/content/post/<id>` 发送 100 个请求<br>3. 对 `/api/v1/task/<id>` 发送 100 个请求<br>4. 统计每个接口的 P95 响应时间 |
| **预期结果** | 1. health 接口 P95 < 200ms<br>2. 读帖接口 P95 < 1s<br>3. 读任务接口 P95 < 1s<br>4. 所有接口 P95 < 1s |

---

#### TC-E-005: ECS 60G 系统盘存储空间

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-005 |
| **标题** | 60G 系统盘满足容器 + ES + MinIO 存储需求 |
| **需求来源** | Technical Constraints - Performance (存储：ECS 60G 系统盘) |
| **优先级** | 中 |
| **前置条件** | 1. 所有容器已启动<br>2. ES 和 MinIO 有一定数据 |
| **测试步骤** | 1. 执行 `df -h /` 查看系统盘使用情况<br>2. 记录各部分占用空间：Docker 镜像、容器运行时数据、ES 索引、MinIO 对象数据、系统<br>3. 计算剩余可用空间 |
| **预期结果** | 1. 总使用量 < 50G（预留 10G 余量）<br>2. Docker 镜像总占用 < 1G<br>3. ES 索引 + MinIO 数据在合理范围内<br>4. 磁盘使用率 < 80% |

---

#### TC-E-006: 月成本不超过 200 元

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-006 |
| **标题** | 阿里云月度账单不超过 200 元 |
| **需求来源** | Success Metrics (月成本 < 200 元) / Story 1 |
| **优先级** | 中 |
| **前置条件** | 1. 所有云资源已使用一个月或已估算<br>2. 有阿里云控制台账单访问权限 |
| **测试步骤** | 1. 登录阿里云费用中心<br>2. 查看当前月份账单明细<br>3. 分别统计 ECS、RDS、Tair、ACR、带宽费用<br>4. 计算总费用 |
| **预期结果** | 1. 月总费用 < 200 元<br>2. 各项费用在预算范围内：<br>　- ECS e 系列 4 核 8G ~100 元<br>　- RDS MySQL 1 核 1G ~50 元<br>　- Tair Redis 1G ~30 元<br>　- ACR 免费<br>　- 带宽/流量 < 20 元 |

---

#### TC-E-007: 镜像拉起时间小于 3 分钟

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-007 |
| **标题** | docker compose up 到全部 healthy < 3 分钟 |
| **需求来源** | Success Metrics (镜像拉起时间：docker compose up 到全部 healthy < 3 分钟) |
| **优先级** | 中 |
| **前置条件** | 1. 所有镜像已缓存在 ECS 上（避免 pull 时间）<br>2. 所有服务已停止：`docker compose down` |
| **测试步骤** | 1. 记录开始时间<br>2. 执行 `docker compose -f campus-docker-compose.yaml up -d`<br>3. 轮询 `docker compose ps` 检查所有服务是否 healthy<br>4. 记录所有服务变为 healthy 的时间<br>5. 计算耗时 |
| **预期结果** | 1. 从 up 到全部 healthy 的总时间 < 3 分钟<br>2. 各服务健康检查通过<br>3. 无超时或启动失败 |

---

#### TC-E-008: 回滚操作在 5 分钟内完成

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-008 |
| **标题** | 出问题时切回上一版本 < 5 分钟 |
| **需求来源** | Success Metrics (回滚可用性：切回上一版本 < 5 分钟) |
| **优先级** | 中 |
| **前置条件** | 1. ACR 中保留有上一版本镜像<br>2. 当前运行的是新版本 |
| **测试步骤** | 1. 记录开始时间<br>2. 修改 docker-compose.yaml 中镜像 tag 为上一版本<br>3. 执行 `docker compose -f campus-docker-compose.yaml pull && docker compose -f campus-docker-compose.yaml up -d`<br>4. 等待所有服务 healthy<br>5. 执行 verify.sh 验证<br>6. 记录总耗时 |
| **预期结果** | 1. 整个回滚过程 < 5 分钟<br>2. 回滚后所有服务正常运行<br>3. verify.sh 全部 PASS |

---

#### TC-E-009: 6 个 Go 微服务内存分配在 256-512M 范围

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-009 |
| **标题** | 各微服务容器内存使用在 256-512M 分配范围内 |
| **需求来源** | Technical Constraints - Performance (6 个 Go 微服务：CPU 0.5 核 / 内存 256-512M) |
| **优先级** | 中 |
| **前置条件** | 1. 所有微服务已启动并有稳定流量 |
| **测试步骤** | 1. 执行 `docker stats --no-stream` 查看各微服务容器内存使用<br>2. 记录 gateway、user、content、task、message、file 的内存占用<br>3. 对比资源分配方案 |
| **预期结果** | 1. 各微服务内存使用在 256-512M 范围内<br>2. 无服务超出 512M 上限<br>3. 内存使用稳定，无持续增长（内存泄漏） |

---

#### TC-E-010: ES 容器内存不超过 1G 堆内存限制

| 字段 | 内容 |
|------|------|
| **编号** | TC-E-010 |
| **标题** | Elasticsearch 容器内存使用在 1G 限制内 |
| **需求来源** | Technical Constraints - Performance (Elasticsearch 8.12：CPU 0.5 核 / 内存 1G，堆内存 512M) |
| **优先级** | 中 |
| **前置条件** | 1. ES 已启动<br>2. ES 有基本索引数据 |
| **测试步骤** | 1. 执行 `docker stats --no-stream elasticsearch`<br>2. 查看 ES JVM 堆内存使用：`curl -s "localhost:9200/_cat/nodes?v&h=name,heap.percent"`<br>3. 观察 5 分钟内的内存波动 |
| **预期结果** | 1. 容器总内存 < 1G<br>2. JVM 堆内存使用率 < 75%（512M 堆的 75%）<br>3. 无 GC 长时间停顿<br>4. 无 OOM 导致 ES 重启 |

---

### 异常测试 (TC-ERR)

#### TC-ERR-001: Go 编译失败时 CI 终止且不推送镜像

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-001 |
| **标题** | 服务 Go 编译失败时 CI 标记失败且不推送镜像 |
| **需求来源** | Feature 1 (Error handling: CI 阶段失败立即终止，不推镜像) / Feature 3 (Edge cases: 单服务 build 失败) |
| **优先级** | 高 |
| **前置条件** | 1. 准备一个包含编译错误的代码分支 |
| **测试步骤** | 1. 在某服务中引入编译错误（如删除 import 包）<br>2. Push 到 main 分支触发 CI<br>3. 观察 Actions 日志<br>4. 检查 ACR 中是否有新镜像推送 |
| **预期结果** | 1. CI 日志显示编译错误<br>2. 该服务构建 job 标记为 failed<br>3. ACR 中无新镜像（旧版本镜像仍存在）<br>4. 失败不影响其他服务的构建（矩阵模式下） |

---

#### TC-ERR-002: ACR 推送失败时自动重试

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-002 |
| **标题** | ACR 镜像推送失败时 CI 自动重试 2 次 |
| **需求来源** | Feature 3 (Edge cases: ACR 推送失败 → CI 重试 2 次) |
| **优先级** | 中 |
| **前置条件** | 1. CI 流水线已配置重试机制<br>2. 模拟 ACR 推送失败（如临时网络中断或凭证过期） |
| **测试步骤** | 1. 模拟 ACR 推送失败场景<br>2. 观察 Actions 日志中的推送阶段<br>3. 确认是否进行了重试<br>4. 统计重试次数 |
| **预期结果** | 1. 首次推送失败后自动重试<br>2. 最多重试 2 次（共 3 次尝试）<br>3. 重试成功则 job 标记为通过<br>4. 3 次均失败则 job 标记为失败 |

---

#### TC-ERR-003: CI 失败不影响 ECS 运行上一版本

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-003 |
| **标题** | CI 构建失败时 ECS 继续运行上一个稳定版本 |
| **需求来源** | Feature 3 (Error handling: CI 失败不通知 ECS，ECS 继续跑上一个版本) |
| **优先级** | 高 |
| **前置条件** | 1. ECS 正在运行上一版本的所有服务<br>2. 新的 push 触发 CI 但构建失败 |
| **测试步骤** | 1. 确认 ECS 上当前运行的服务版本<br>2. 触发一次失败的 CI 构建<br>3. 在 CI 失败期间从公网访问 gateway API<br>4. 确认服务是否正常响应 |
| **预期结果** | 1. CI 失败不触发 ECS 上的任何变更<br>2. ECS 继续运行原版本服务<br>3. 公网 API 调用正常响应<br>4. 无服务中断 |

---

#### TC-ERR-004: RDS 连接失败时 verify.sh 报错

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-004 |
| **标题** | RDS 数据库连接失败时 verify.sh 标红并提示检查 .env |
| **需求来源** | Feature 5 (Edge cases: 数据库连接失败 → 标红，提示检查 .env) |
| **优先级** | 高 |
| **前置条件** | 1. verify.sh 已部署<br>2. 故意将 .env 中的 DB_HOST 改为错误地址 |
| **测试步骤** | 1. 修改 `.env` 中 DB_HOST 为无效地址<br>2. 重启受影响的服务<br>3. 执行 `./scripts/verify.sh`<br>4. 查看输出 |
| **预期结果** | 1. 数据库连通性检查显示 FAIL（标红）<br>2. 输出提示检查 `.env` 配置<br>3. 脚本返回非 0 退出码<br>4. 不影响其他非数据库依赖的检查项 |

---

#### TC-ERR-005: 单个服务健康检查失败时 verify.sh 标红

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-005 |
| **标题** | 单个服务不健康时 verify.sh 标红报警 |
| **需求来源** | Feature 5 (Edge cases: 单个服务健康检查失败 → 标红，CI 报警) |
| **优先级** | 高 |
| **前置条件** | 1. verify.sh 已部署<br>2. 故意停止某一个微服务（如 user 服务） |
| **测试步骤** | 1. 执行 `docker stop <user服务容器>` 停止 user 服务<br>2. 等待 10 秒<br>3. 执行 `./scripts/verify.sh`<br>4. 查看输出 |
| **预期结果** | 1. user 服务健康检查显示 FAIL（标红）<br>2. 其他服务健康检查正常显示 PASS<br>3. 脚本返回非 0 退出码<br>4. 输出清晰标注哪个服务失败 |

---

#### TC-ERR-006: ECS 资源不足时的降级处理

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-006 |
| **标题** | ECS 资源不足时可降级为 4 个核心服务 |
| **需求来源** | Feature 4 (Edge cases: ECS 资源不足 → 升级到 2 核 4G) / Risk Assessment (ECS 资源跑满) |
| **优先级** | 低 |
| **前置条件** | 1. 模拟 ECS 资源紧张（如同时运行其他进程占满内存）<br>2. 有可选的精简版 docker-compose |
| **测试步骤** | 1. 使 ECS 内存使用率 > 85%<br>2. 执行 docker compose up<br>3. 观察哪些服务能正常启动<br>4. 确认核心服务（gateway + 4 个核心服务）是否可用 |
| **预期结果** | 1. 核心服务可启动并提供基本功能<br>2. 非核心服务可能 OOM 但不拖垮整个系统<br>3. 有文档指导如何降级部署 |

---

#### TC-ERR-007: .env 缺失时 docker compose up 的错误提示

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-007 |
| **标题** | .env 文件缺失时 docker compose 有明确错误提示 |
| **需求来源** | Story 4 / Dependencies & Blockers (本地无 .env 文件) |
| **优先级** | 中 |
| **前置条件** | 1. 备份当前 .env 文件<br>2. 临时移除或重命名 .env |
| **测试步骤** | 1. 将 .env 重命名为 .env.bak<br>2. 执行 `docker compose -f campus-docker-compose.yaml up -d`<br>3. 记录错误信息<br>4. 恢复 .env 文件 |
| **预期结果** | 1. docker compose 输出明确的错误信息<br>2. 提示缺少 .env 文件或相关环境变量<br>3. 不影响其他服务的 compose 配置<br>4. 恢复 .env 后可正常启动 |

---

#### TC-ERR-008: GitHub Actions 凭证泄露防护

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-008 |
| **标题** | GitHub Actions 中凭证不在日志中明文显示 |
| **需求来源** | Security / Risk Assessment (GitHub Actions 凭证泄露) |
| **优先级** | 高 |
| **前置条件** | 1. CI 流水线已配置使用 Secrets<br>2. 有权限查看 Actions 日志 |
| **测试步骤** | 1. 触发一次完整的 CI 流水线<br>2. 查看 Actions 日志中所有涉及凭证的步骤<br>3. 搜索日志中是否包含 ACR 密码、RAM AccessKey 等敏感信息<br>4. 检查 GitHub Secrets 配置 |
| **预期结果** | 1. 日志中所有 secrets 引用显示为 `***`<br>2. 无明文密码或密钥出现在日志中<br>3. Actions workflow 文件中使用 `${{ secrets.XXX }}` 而非硬编码<br>4. RAM 子账号权限最小化（仅 ACR push） |

---

#### TC-ERR-009: 镜像仓库被误删时的恢复

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-009 |
| **标题** | ACR 镜像仓库误删后的恢复方案 |
| **需求来源** | Risk Assessment (镜像仓库被误删 → ACR 开启版本不可删；本地保留最近 5 个镜像) |
| **优先级** | 低 |
| **前置条件** | 1. ACR 已开启版本不可删除保护<br>2. 本地 Docker 保留有镜像缓存 |
| **测试步骤** | 1. 确认 ACR 是否开启了删除保护<br>2. 尝试在 ACR 控制台删除镜像 tag<br>3. 如删除失败（受保护），确认恢复方式<br>4. 检查本地 Docker 是否有镜像缓存可推送 |
| **预期结果** | 1. ACR 开启了版本删除保护，无法直接删除<br>2. 即使删除，可通过 CI 重新构建推送<br>3. 本地 Docker 保留最近 5 个镜像作为备份<br>4. 有文档说明恢复步骤 |

---

#### TC-ERR-010: 阿里云操作审计日志开启

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-010 |
| **标题** | 阿里云操作审计日志已开启 |
| **需求来源** | Security (审计日志：阿里云操作审计开启) |
| **优先级** | 低 |
| **前置条件** | 1. 阿里云账号有管理权限 |
| **测试步骤** | 1. 登录阿里云控制台<br>2. 进入操作审计（ActionTrail）页面<br>3. 确认审计日志已开启<br>4. 执行一个 ECS 操作（如重启）<br>5. 检查审计日志中是否有记录 |
| **预期结果** | 1. 操作审计状态为开启<br>2. 执行的操作被记录到审计日志<br>3. 日志包含操作时间、操作者、操作内容<br>4. 使用基础版（免费） |

---

#### TC-ERR-011: ES 冷启动 OOM 场景验证

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-011 |
| **标题** | ECS 资源充足时 ES 不出现 OOM |
| **需求来源** | PRD v1.1 变更 (1 核 2G 会出现 ES 冷启 OOM) / Risk Assessment (服务 OOM) |
| **优先级** | 高 |
| **前置条件** | 1. ECS 为 4 核 8G 配置<br>2. 所有服务已停止 |
| **测试步骤** | 1. 执行 `docker compose down` 停止所有服务<br>2. 执行 `free -h` 确认内存充足<br>3. 执行 `docker compose up -d elasticsearch` 仅启动 ES<br>4. 持续监控 ES 内存使用 2 分钟<br>5. 确认 ES 是否启动成功 |
| **预期结果** | 1. ES 在 4 核 8G 环境下正常启动<br>2. 内存使用稳定，不触发 OOM<br>3. ES health check 通过<br>4. 相比 1 核 2G 环境，无 OOM 问题 |

---

#### TC-ERR-012: 网络隔离 - 外部无法访问内网服务

| 字段 | 内容 |
|------|------|
| **编号** | TC-ERR-012 |
| **标题** | 外部无法直接访问 RDS、Tair、RabbitMQ 等内网服务 |
| **需求来源** | Security (网络隔离：阿里云 VPC 内网隔离) |
| **优先级** | 高 |
| **前置条件** | 1. 从一台非 ECS 的外部机器<br>2. 知道 RDS 和 Tair 的公网连接方式（如果有的话） |
| **测试步骤** | 1. 从外部机器尝试连接 RDS 的 3306 端口<br>2. 从外部机器尝试连接 Tair 的 6379 端口<br>3. 从外部机器尝试连接 RabbitMQ 的 5672 端口<br>4. 从外部机器尝试连接 ES 的 9200 端口（如未开放） |
| **预期结果** | 1. RDS 3306 端口从外部无法连接（超时或拒绝）<br>2. Tair 6379 端口从外部无法连接<br>3. RabbitMQ 5672 端口从外部无法连接<br>4. 仅 8080（gateway）和必要端口从公网可访问 |

---

### 状态转换测试 (TC-ST)

#### TC-ST-001: 服务从停止到启动到 healthy 状态转换

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-001 |
| **标题** | 单个服务从停止 → 启动 → healthy 的完整状态转换 |
| **需求来源** | Story 2 (AC2) / Feature 2 |
| **优先级** | 高 |
| **前置条件** | 1. 服务当前已停止<br>2. 依赖的中间件已正常运行 |
| **测试步骤** | 1. 记录服务初始状态为 `stopped`<br>2. 执行 `docker compose start <服务名>`<br>3. 每 5 秒执行 `docker inspect --format '{{.State.Status}}' <容器名>`<br>4. 记录状态变化过程<br>5. 直到状态变为 `running` 且 health check 为 `healthy` |
| **预期结果** | 1. 状态转换路径：stopped → running (starting) → running (healthy)<br>2. 从启动到 healthy 的时间 < 60s<br>3. 中间无 unexpected 重启<br>4. healthy 后 health check 持续通过 |

---

#### TC-ST-002: 服务从 healthy 到崩溃到自动恢复

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-002 |
| **标题** | 服务从 healthy → 崩溃 → 自动重启 → 恢复 healthy |
| **需求来源** | Story 5 (AC1: restart: always) |
| **优先级** | 高 |
| **前置条件** | 1. 服务当前为 healthy 状态<br>2. docker-compose 配置了 `restart: always` |
| **测试步骤** | 1. 确认服务状态为 healthy<br>2. 执行 `docker kill <容器名>` 模拟崩溃<br>3. 记录 kill 时间和容器状态变化<br>4. 每 5 秒检查容器状态<br>5. 确认服务恢复到 healthy |
| **预期结果** | 1. kill 后状态：running → exited<br>2. Docker 自动重启：exited → running (starting)<br>3. 健康检查通过：running (starting) → running (healthy)<br>4. 整个恢复过程 < 60s<br>5. 服务恢复后功能正常 |

---

#### TC-ST-003: 全栈从 down 到 up 的完整启动流程

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-003 |
| **标题** | docker compose down → up 到全部服务 healthy 的完整流程 |
| **需求来源** | Story 2 / Feature 2 |
| **优先级** | 高 |
| **前置条件** | 1. 所有容器已停止并移除 |
| **测试步骤** | 1. 执行 `docker compose -f campus-docker-compose.yaml down`<br>2. 确认所有容器已移除：`docker compose ps` 应为空<br>3. 记录开始时间<br>4. 执行 `docker compose -f campus-docker-compose.yaml up -d`<br>5. 使用 `docker compose logs -f` 观察启动过程<br>6. 每 10 秒检查 `docker compose ps`<br>7. 记录所有服务变为 healthy 的时间 |
| **预期结果** | 1. down 后所有容器和网络被清理<br>2. up 后启动顺序：基础设施 → 微服务 → gateway<br>3. 所有服务最终状态为 healthy<br>4. 总耗时 < 3 分钟<br>5. verify.sh 全部 PASS |

---

#### TC-ST-004: 单服务重启不影响其他服务

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-004 |
| **标题** | 重启单个微服务时其他服务不受影响 |
| **需求来源** | Feature 2 (Error handling: depends_on 失败不退出整个栈) |
| **优先级** | 高 |
| **前置条件** | 1. 所有服务已启动并 healthy |
| **测试步骤** | 1. 确认所有服务状态为 healthy<br>2. 执行 `curl http://localhost:8080/health` 确认 gateway 正常<br>3. 执行 `docker compose restart <user服务名>` 重启 user 服务<br>4. 在 user 重启过程中访问 gateway 的非 user 相关接口<br>5. 等待 user 服务恢复<br>6. 再次验证所有服务状态 |
| **预期结果** | 1. user 服务重启过程中，其他服务不受影响<br>2. gateway 对非 user 相关接口仍正常响应<br>3. user 服务恢复后，所有功能恢复正常<br>4. 其他服务的状态保持 healthy |

---

#### TC-ST-005: 版本升级 - 镜像 tag 切换

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-005 |
| **标题** | 通过切换镜像 tag 实现版本升级 |
| **需求来源** | Story 2 (AC5: 镜像保留上一个版本作为回滚点) / Feature 3 |
| **优先级** | 中 |
| **前置条件** | 1. 当前运行 v1 版本<br>2. ACR 中有 v2 版本镜像 |
| **测试步骤** | 1. 确认当前运行版本：检查容器镜像 tag<br>2. 修改 docker-compose.yaml 中某服务的镜像 tag 为 v2<br>3. 执行 `docker compose pull <服务名>` 拉取新镜像<br>4. 执行 `docker compose up -d <服务名>` 滚动更新<br>5. 确认新版本运行<br>6. 执行 verify.sh 验证 |
| **预期结果** | 1. 版本切换过程中其他服务不受影响<br>2. 新版本服务启动成功并 healthy<br>3. verify.sh 全部 PASS<br>4. API 响应正常 |

---

#### TC-ST-006: 版本回滚 - 切回旧版本

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-006 |
| **标题** | 从新版本回滚到旧版本 |
| **需求来源** | Success Metrics (回滚可用性: 切回上一版本 < 5 分钟) / Story 2 (AC5) |
| **优先级** | 高 |
| **前置条件** | 1. 当前运行 v2 版本<br>2. ACR 中保留有 v1 版本镜像 |
| **测试步骤** | 1. 确认当前运行 v2 版本<br>2. 修改 docker-compose.yaml 中镜像 tag 回 v1<br>3. 执行 `docker compose pull && docker compose up -d`<br>4. 等待所有服务 healthy<br>5. 执行 verify.sh 验证<br>6. 确认版本回退成功 |
| **预期结果** | 1. 回滚到 v1 版本成功<br>2. 所有服务 healthy<br>3. verify.sh 全部 PASS<br>4. 整个回滚过程 < 5 分钟 |

---

#### TC-ST-007: ECS 重启后服务自动恢复

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-007 |
| **标题** | ECS 重启后 docker-compose 服务自动拉起 |
| **需求来源** | Story 5 (AC1: restart: always) / Feature 2 |
| **优先级** | 高 |
| **前置条件** | 1. 所有服务已启动<br>2. Docker daemon 设置为开机自启<br>3. docker-compose 配置了 `restart: always` |
| **测试步骤** | 1. 确认所有服务运行正常<br>2. 执行 `sudo reboot` 重启 ECS<br>3. 等待 ECS 启动完成（1-2 分钟）<br>4. SSH 重新登录 ECS<br>5. 执行 `docker compose -f campus-docker-compose.yaml ps`<br>6. 执行 `./scripts/verify.sh` |
| **预期结果** | 1. Docker daemon 开机自启<br>2. 所有容器自动启动<br>3. 所有服务最终变为 healthy<br>4. verify.sh 全部 PASS<br>5. 数据未丢失（etcd、MinIO、ES 数据保留） |

---

#### TC-ST-008: RDS 自动备份与恢复

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-008 |
| **标题** | RDS 自动备份功能验证 |
| **需求来源** | Feature 4 (Error handling: 所有云资源都开备份) |
| **优先级** | 中 |
| **前置条件** | 1. RDS 已开启自动备份<br>2. RDS 有业务数据 |
| **测试步骤** | 1. 登录 RDS 控制台<br>2. 确认自动备份策略已配置<br>3. 检查备份文件列表<br>4. 确认备份文件时间与策略一致 |
| **预期结果** | 1. 自动备份已开启<br>2. 备份策略为每天自动备份<br>3. 备份文件列表中有近期备份<br>4. 可通过备份恢复数据库 |

---

#### TC-ST-009: 安全组从严格到临时开放再到收回

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-009 |
| **标题** | 验收时临时开放公网访问，验收后收回 |
| **需求来源** | Risk Assessment (评审老师 IP 不在白名单 → 安全组先临时开 0.0.0.0/0 验收，验收完收回) |
| **优先级** | 中 |
| **前置条件** | 1. ECS 安全组当前限制特定 IP |
| **测试步骤** | 1. 记录当前安全组规则<br>2. 临时将 8080 端口入方向规则改为 `0.0.0.0/0`<br>3. 从公网验证 API 可访问<br>4. 验收完成后将 8080 端口规则改回限制特定 IP<br>5. 再次从公网验证是否被拒绝 |
| **预期结果** | 1. 临时开放后，任何 IP 可访问 8080 端口<br>2. 收回后，非白名单 IP 无法访问 8080 端口<br>3. 22 端口始终限制特定 IP<br>4. 操作过程有记录 |

---

#### TC-ST-010: ECS 快照创建与恢复

| 字段 | 内容 |
|------|------|
| **编号** | TC-ST-010 |
| **标题** | ECS 磁盘快照每周备份与恢复验证 |
| **需求来源** | Feature 4 (Error handling: 所有云资源都开备份 → ECS 快照每周一次) |
| **优先级** | 低 |
| **前置条件** | 1. ECS 已配置自动快照策略<br>2. 有手动创建快照的权限 |
| **测试步骤** | 1. 登录 ECS 控制台<br>2. 确认自动快照策略已配置<br>3. 手动创建一个测试快照<br>4. 确认快照创建成功<br>5. （可选）从快照恢复验证 |
| **预期结果** | 1. 自动快照策略已配置<br>2. 手动快照创建成功<br>3. 快照包含完整的磁盘数据<br>4. 快照可用来恢复 ECS |

---

## 需求-测试用例覆盖矩阵

| 需求编号 / 来源 | 需求描述 | 覆盖的测试用例 |
|----------------|----------|---------------|
| Story 1 (AC1) | push main 触发 GitHub Actions | TC-F-001 |
| Story 1 (AC2) | 6 个服务镜像构建成功 | TC-F-002, TC-E-001 |
| Story 1 (AC3) | 镜像推送至 ACR | TC-F-003 |
| Story 1 (AC4) | 镜像 tag 含 commit SHA | TC-F-004 |
| Story 1 (AC5) | CI 总耗时 < 5 分钟 | TC-F-028 |
| Story 2 (AC1) | docker compose 拉起全部服务 | TC-F-005, TC-ST-003 |
| Story 2 (AC2) | 所有服务 health check 通过 | TC-F-006, TC-F-007, TC-ST-001 |
| Story 2 (AC3) | 异常崩溃后自动重启 | TC-F-021, TC-ST-002 |
| Story 2 (AC4) | verify.sh 一键验证 | TC-F-008, TC-F-009 |
| Story 2 (AC5) | 镜像保留上一版本 | TC-F-034, TC-ST-005, TC-ST-006 |
| Story 3 (AC1) | 公网 health 返回 200 | TC-F-010 |
| Story 3 (AC2) | 公网注册接口 2xx | TC-F-011 |
| Story 3 (AC3) | 公网发帖接口 2xx | TC-F-012 |
| Story 3 (AC4) | 公网发消息接口 2xx | TC-F-013 |
| Story 3 (AC5) | 公网上传文件接口 2xx | TC-F-014 |
| Story 3 (AC6) | 公网创建任务接口 2xx | TC-F-015 |
| Story 3 (AC7) | 公网查询接口 2xx | TC-F-016 |
| Story 4 (AC1) | .env 在 .gitignore 中 | TC-F-017 |
| Story 4 (AC2) | .env.example 提交作为模板 | TC-F-018 |
| Story 4 (AC3) | ECS 上 .env 保存真实值 | TC-F-019 |
| Story 4 (AC4) | docker-compose 通过 env_file 引用 | TC-F-020 |
| Story 5 (AC1) | 所有服务 restart: always | TC-F-021, TC-ST-007 |
| Story 5 (AC2) | RabbitMQ 队列持久化 | TC-F-023 |
| Story 5 (AC3) | etcd 数据 volume 挂载 | TC-F-022 |
| Story 5 (AC4) | MinIO 数据 volume 挂载 | TC-F-024 |
| Story 5 (AC5) | ES 数据 volume 挂载 | TC-F-025 |
| Feature 1 | 6 个微服务 Dockerfile 化 | TC-F-032, TC-E-001, TC-ERR-001 |
| Feature 2 | 统一 docker-compose.yaml | TC-F-005, TC-F-006, TC-F-007, TC-ST-003, TC-ST-004 |
| Feature 3 | GitHub Actions 流水线 | TC-F-001, TC-F-002, TC-F-003, TC-F-004, TC-F-028, TC-ERR-002, TC-ERR-003 |
| Feature 4 | 阿里云资源编排 | TC-F-026, TC-F-027, TC-F-029, TC-F-030, TC-E-002, TC-ST-008, TC-ST-010, TC-ERR-006 |
| Feature 5 | verify.sh 验证脚本 | TC-F-008, TC-F-009, TC-F-033, TC-ERR-004, TC-ERR-005 |
| Success Metric: 端到端主链路 | 注册→登录→发帖→读帖→发消息→上传头像 | TC-F-009, TC-F-011 ~ TC-F-016 |
| Success Metric: 月成本 | < 200 元 | TC-E-006 |
| Success Metric: 拉起时间 | < 3 分钟 | TC-E-007 |
| Success Metric: 回滚可用性 | < 5 分钟 | TC-E-008, TC-ST-006 |
| Security: SSH | 仅限特定 IP，密钥登录 | TC-F-029 |
| Security: 数据库白名单 | RDS/Tair 仅允许 ECS 内网 IP | TC-F-030, TC-ERR-012 |
| Security: 密钥管理 | .env 不入 git | TC-F-017, TC-F-018, TC-F-019, TC-F-020, TC-ERR-008 |
| Security: 网络隔离 | VPC 内网隔离 | TC-ERR-012, TC-ST-009 |
| Security: ACR 凭证 | GitHub Secrets 存储 | TC-F-031, TC-ERR-008 |
| Security: 审计日志 | 阿里云操作审计 | TC-ERR-010 |
| Performance: 资源分配 | 4 核 8G 内运行 | TC-E-002, TC-E-009, TC-E-010 |
| Performance: 响应时间 | P95 < 1s | TC-E-004 |
| Performance: 并发 | 10-50 并发用户 | TC-E-003 |
| Performance: 存储 | 60G 系统盘 | TC-E-005 |
| Risk: ECS 资源跑满 | 降级为 4 个核心服务 | TC-ERR-006 |
| Risk: 账单超 200 元 | 预算告警 | TC-E-006 |
| Risk: CI 构建慢 | Go build cache + 并发矩阵 | TC-F-028 |
| Risk: RDS/Tair 不通 | 安全组双确认 | TC-F-030, TC-ERR-004 |
| Risk: 服务启动顺序错乱 | depends_on + healthcheck | TC-F-007, TC-ST-003 |
| Risk: 镜像仓库误删 | ACR 版本不可删 | TC-ERR-009 |
| Risk: 凭证泄露 | Secrets + RAM 最小权限 | TC-ERR-008 |
| Risk: 服务 OOM | 限制内存 + 监控 | TC-ERR-011, TC-E-010 |
| Risk: ES 冷启 OOM | 4 核 8G 避免 OOM | TC-ERR-011 |
| Phase 1: runbook | 部署文档可复现 | TC-F-035 |
| 架构约定 | 每服务独立 MySQL | TC-F-026 |
| 架构约定 | TraceID 全链路透传 | （需专项测试，超出本部署 PRD 范围） |

---

## 统计汇总

| 测试类别 | 用例数量 | 优先级分布 |
|----------|---------|-----------|
| 功能测试 (TC-F) | 35 | 高: 23, 中: 12 |
| 边界测试 (TC-E) | 10 | 高: 2, 中: 8 |
| 异常测试 (TC-ERR) | 12 | 高: 6, 中: 3, 低: 3 |
| 状态转换测试 (TC-ST) | 10 | 高: 6, 中: 3, 低: 1 |
| **合计** | **67** | **高: 37, 中: 26, 低: 4** |

| 优先级 | 数量 | 占比 |
|--------|------|------|
| 高 | 37 | 55.2% |
| 中 | 26 | 38.8% |
| 低 | 4 | 6.0% |
