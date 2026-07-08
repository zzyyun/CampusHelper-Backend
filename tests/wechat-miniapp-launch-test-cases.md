# CampusHelper 微信小程序上线方案 - 测试用例文档

**项目**: CampusHelper-Backend（校园互助平台后端）
**版本**: v2.0
**前置 PRD**: wechat-miniapp-launch-prd.md
**生成日期**: 2026-07-08

---

## 目录

1. [测试范围概述](#1-测试范围概述)
2. [功能测试（TC-F）](#2-功能测试tc-f)
3. [边界测试（TC-E）](#3-边界测试tc-e)
4. [异常测试（TC-ERR）](#4-异常测试tc-err)
5. [状态转换测试（TC-ST）](#5-状态转换测试tc-st)
6. [需求-测试用例覆盖矩阵](#6-需求-测试用例覆盖矩阵)

---

## 1. 测试范围概述

本测试用例文档覆盖 CampusHelper 微信小程序上线方案的四大核心领域：

- **HTTPS 基础设施**：Nginx 反向代理、SSL 证书、HTTP/HTTPS 跳转
- **小程序配置**：BASE_URL 修改、合法域名配置、开发者工具校验
- **微信平台对接**：wx.login 登录、业务接口调用、体验版与正式版
- **上线路径**：Phase A/B/C 各阶段验证、verify-https.sh 脚本

### 测试环境前提

| 项目 | 值 |
|------|-----|
| ECS IP | 121.41.74.238 |
| 域名 | rithupc.cn |
| 微信 AppID | wxa782f10bddd49b38 |
| 证书来源 | 阿里云免费个人测试证书 |
| Nginx 版本 | 1.25-alpine |
| Gateway 端口 | 50000 |
| HTTPS 端口 | 443 |
| HTTP 端口 | 80 |

---

## 2. 功能测试（TC-F）

### 2.1 Nginx 反向代理与 HTTPS 终止

#### TC-F-001：HTTPS 健康检查接口返回 200

- **需求来源**: Feature 1 (Nginx 反向代理) / Story 3 (部署验证)
- **优先级**: 高
- **前置条件**:
  - Nginx 容器 `campus-nginx` 已启动并运行
  - SSL 证书已正确加载至 `/etc/nginx/certs/`
  - gateway 服务健康运行
  - ECS 安全组已放行 443 端口
- **测试步骤**:
  1. 执行 `curl -sk https://rithupc.cn/health`
  2. 检查 HTTP 响应状态码
  3. 检查响应体内容是否为 JSON 且 `code: 0`
- **预期结果**: HTTP 响应状态码为 200，响应体包含有效的健康状态 JSON

---

#### TC-F-002：HTTP 强制跳转 HTTPS（301）

- **需求来源**: Feature 1 (Nginx 反向代理) - Edge case: HTTP 访问 301 跳转
- **优先级**: 高
- **前置条件**:
  - Nginx 容器已运行
  - 80 端口已放行
- **测试步骤**:
  1. 执行 `curl -s -o /dev/null -w "%{http_code}" http://rithupc.cn/health`
  2. 检查响应状态码
  3. 执行 `curl -sI http://rithupc.cn/health | grep -i location` 确认跳转目标
- **预期结果**: 返回 301 状态码，Location 头指向 `https://rithupc.cn/health`

---

#### TC-F-003：Nginx 反代至 gateway 业务接口

- **需求来源**: Feature 1 (Nginx 反向代理) - User flow
- **优先级**: 高
- **前置条件**:
  - Nginx 已配置 `proxy_pass http://campus-gateway:50000`
  - gateway 服务已启动
- **测试步骤**:
  1. 执行 `curl -sk "https://rithupc.cn/api/v1/content/posts?school_id=1"`
  2. 检查 HTTP 状态码
  3. 检查响应体是否为合法 JSON（帖子列表）
- **预期结果**: 返回 200，响应体为帖子列表 JSON 数据

---

#### TC-F-004：Nginx 请求头透传

- **需求来源**: Feature 1 (Nginx 反向代理) - proxy_set_header 配置
- **优先级**: 中
- **前置条件**:
  - Nginx 已正确配置 `X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto` 头
- **测试步骤**:
  1. 执行 `curl -sk https://rithupc.cn/api/v1/content/posts?school_id=1 -H "X-Test-Debug: true"`
  2. 在 gateway 日志中查看是否收到 `X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto: https` 头
- **预期结果**: gateway 收到 `X-Forwarded-Proto: https`、`X-Real-IP` 为客户端 IP

---

#### TC-F-005：Nginx WebSocket 升级头保留

- **需求来源**: Feature 1 (Nginx 反向代理) - WebSocket 为后续 message service 预留
- **优先级**: 中
- **前置条件**:
  - Nginx 运行，客户端发送 WebSocket 升级请求
- **测试步骤**:
  1. 使用 `wscat -c wss://rithupc.cn/ws`（或等效工具）发起 WebSocket 连接
  2. 检查连接是否成功建立
  3. 检查 Nginx 日志确认 `Connection: upgrade` 头被透传
- **预期结果**: WebSocket 连接成功建立，Nginx 正确透传升级头

---

#### TC-F-006：verify-https.sh 脚本全部 6 项检查通过

- **需求来源**: Story 3 (部署同学一次性验证) / Appendix B
- **优先级**: 高
- **前置条件**:
  - HTTPS 基础设施已部署完成
  - 脚本 `scripts/verify-https.sh` 已放置
- **测试步骤**:
  1. 执行 `bash scripts/verify-https.sh`
  2. 检查输出中 6 项检查（DNS、HTTPS /health、HTTP 跳转、证书有效、SSL 协议、业务接口）是否全部通过
  3. 检查最终输出是否包含 "HTTPS 验证全部通过"
- **预期结果**: 所有 6 项检查通过，输出 "✅ HTTPS 验证全部通过"

---

#### TC-F-007：DNS 解析验证

- **需求来源**: Story 3 (部署验证) - verify-https.sh 第 1 项
- **优先级**: 高
- **前置条件**:
  - 域名 rithupc.cn DNS A 记录已配置
- **测试步骤**:
  1. 执行 `dig +short rithupc.cn`
  2. 取第一条结果
  3. 与预期 IP `121.41.74.238` 对比
- **预期结果**: dig 返回的 A 记录 IP 为 `121.41.74.238`

---

### 2.2 SSL 证书

#### TC-F-008：SSL 证书已签发且域名匹配

- **需求来源**: Feature 2 (SSL 证书签发) / Success Metrics (SSL 证书有效)
- **优先级**: 高
- **前置条件**:
  - 阿里云证书已签发并上传到 ECS
- **测试步骤**:
  1. 执行 `openssl x509 -in /opt/campus/deployments/nginx/certs/rithupc.cn.pem -noout -subject -dates`
  2. 检查 Subject 中包含 `rithupc.cn`
  3. 检查到期日期是否在未来（1 年内）
- **预期结果**: 证书 Subject 包含 `rithupc.cn`，到期日距今约 11-12 个月

---

#### TC-F-009：SSL 证书链完整性

- **需求来源**: Feature 2 (SSL 证书签发) - fullchain.pem 包含完整证书链
- **优先级**: 高
- **前置条件**:
  - 证书已上传
- **测试步骤**:
  1. 执行 `openssl s_client -connect rithupc.cn:443 -servername rithupc.cn < /dev/null 2>/dev/null | grep "Verify return code"`
  2. 检查返回码是否为 `0 (ok)`
- **预期结果**: 证书链验证通过，返回码为 `0 (ok)`

---

#### TC-F-010：SSL Session Cache 启用

- **需求来源**: Technical Constraints - Performance: SSL session cache
- **优先级**: 低
- **前置条件**:
  - Nginx 已配置 `ssl_session_cache shared:SSL:10m`
- **测试步骤**:
  1. 两次连续 `openssl s_client` 连接到 `rithupc.cn:443`
  2. 第二次连接检查是否复用了 SSL session（`Reused, TLSv1.3` 或 `Reused, TLSv1.2`）
- **预期结果**: 第二次连接显示 `Reused`，表明 session cache 生效

---

### 2.3 小程序配置

#### TC-F-011：小程序 BASE_URL 为 HTTPS

- **需求来源**: Feature 3 (小程序 baseURL HTTPS 化) / Phase B
- **优先级**: 高
- **前置条件**:
  - 小程序代码已拉取
- **测试步骤**:
  1. 打开 `scripts/miniapp/utils/constants.js`
  2. 检查 `BASE_URL` 值
- **预期结果**: `BASE_URL` 值为 `'https://rithupc.cn/api/v1'`，无端口号，无 HTTP

---

#### TC-F-012：project.config.json 已移除 urlCheck: false

- **需求来源**: Feature 3 (小程序 baseURL HTTPS 化) / Phase B
- **优先级**: 高
- **前置条件**:
  - 小程序代码已拉取
- **测试步骤**:
  1. 打开 `scripts/miniapp/project.config.json`
  2. 搜索 `urlCheck` 字段
  3. 确认不存在 `urlCheck: false` 配置
- **预期结果**: `project.config.json` 中不存在 `urlCheck: false`，微信开发者工具将执行合法域名校验

---

#### TC-F-013：微信开发者工具无"非合法域名"提示

- **需求来源**: Story 1 (扫码进入小程序) - AC: 无"非合法域名"提示
- **优先级**: 高
- **前置条件**:
  - 小程序 BASE_URL 已修改为 HTTPS
  - 微信公众平台已添加合法域名
  - 微信开发者工具已安装并登录
- **测试步骤**:
  1. 在微信开发者工具中编译小程序
  2. 打开调试面板查看 Network 标签
  3. 观察是否有任何请求被拦截（非合法域名提示）
- **预期结果**: 所有请求正常发出，无"非合法域名"拦截提示

---

### 2.4 微信公众平台配置

#### TC-F-014：request 合法域名已添加

- **需求来源**: Feature 4 (微信公众平台配置) / Phase B
- **优先级**: 高
- **前置条件**:
  - 已登录微信公众平台 mp.weixin.qq.com
  - 域名已 ICP 备案
- **测试步骤**:
  1. 登录微信公众平台 → 开发管理 → 服务器域名
  2. 查看 request 合法域名列表
  3. 确认 `https://rithupc.cn` 已添加
- **预期结果**: `https://rithupc.cn` 在 request 合法域名列表中

---

#### TC-F-015：uploadFile 合法域名已添加

- **需求来源**: Feature 4 (微信公众平台配置) / Phase B
- **优先级**: 高
- **前置条件**: 同 TC-F-014
- **测试步骤**:
  1. 在微信公众平台查看 uploadFile 合法域名列表
  2. 确认 `https://rithupc.cn` 已添加
- **预期结果**: `https://rithupc.cn` 在 uploadFile 合法域名列表中

---

#### TC-F-016：downloadFile 合法域名已添加

- **需求来源**: Feature 4 (微信公众平台配置) / Phase B
- **优先级**: 高
- **前置条件**: 同 TC-F-014
- **测试步骤**:
  1. 在微信公众平台查看 downloadFile 合法域名列表
  2. 确认 `https://rithupc.cn` 已添加
- **预期结果**: `https://rithupc.cn` 在 downloadFile 合法域名列表中

---

#### TC-F-017：微信校验文件可通过 HTTPS 访问

- **需求来源**: Feature 4 (微信公众平台配置) - Edge case: 校验文件无法下载
- **优先级**: 高
- **前置条件**:
  - 微信校验文件已放置到 `/opt/campus/deployments/nginx/html/.well-known/`
  - Nginx 已配置 `location /.well-known/` 路由
- **测试步骤**:
  1. 执行 `curl -sk https://rithupc.cn/.well-known/<校验文件名>`（使用实际校验文件名）
  2. 检查 HTTP 状态码
  3. 检查响应内容是否与微信后台下载的文件一致
- **预期结果**: 返回 200，文件内容与微信后台校验文件一致

---

### 2.5 微信登录与业务接口

#### TC-F-018：wx.login 获取 js_code 成功

- **需求来源**: Story 1 (扫码进入小程序) - AC: 调起 wx.login() 成功获取 js_code
- **优先级**: 高
- **前置条件**:
  - 小程序已正确配置 AppID
  - 真机或开发者工具中打开小程序
- **测试步骤**:
  1. 在真机或开发者工具中触发登录流程
  2. 调用 `wx.login()` 获取 js_code
  3. 在 vConsole 或日志中查看 js_code 返回值
- **预期结果**: `wx.login()` 返回 `success` 状态，包含有效的 js_code（非空字符串）

---

#### TC-F-019：后端用 js_code 换 access_token 成功

- **需求来源**: Story 1 (扫码进入小程序) - AC: 后端用 js_code + 微信 API 换 access_token 成功
- **优先级**: 高
- **前置条件**:
  - 后端已配置正确的微信 AppID 和 Secret
  - 小程序已获取 js_code
- **测试步骤**:
  1. 使用小程序发起登录请求：`POST https://rithupc.cn/api/v1/user/login`，携带 js_code
  2. 检查响应体
  3. 确认返回 `code: 0` 且包含 `access_token` 字段
- **预期结果**: 返回 `code: 0`，响应体包含有效的 `access_token`（JWT 格式）

---

#### TC-F-020：登录成功后首页显示用户昵称

- **需求来源**: Story 1 (扫码进入小程序) - AC: 登录成功后自动跳转首页，显示用户昵称
- **优先级**: 中
- **前置条件**:
  - 用户已完成登录
- **测试步骤**:
  1. 在真机或模拟器中完成登录流程
  2. 观察是否自动跳转至首页
  3. 检查首页顶部/个人中心是否显示用户昵称
- **预期结果**: 自动跳转首页，页面上正确显示当前用户的微信昵称

---

#### TC-F-021：已登录用户发布失物招领帖

- **需求来源**: Story 2 (已登录用户发失物招领帖) - AC: POST 成功返回 code:0 + 帖子 ID
- **优先级**: 高
- **前置条件**:
  - 用户已登录，持有有效 access_token
- **测试步骤**:
  1. 发送 `POST https://rithupc.cn/api/v1/content/posts`
  2. 请求头携带 `Authorization: Bearer <access_token>`
  3. 请求体包含帖子标题、内容、school_id、类型（失物招领）
  4. 检查响应体
- **预期结果**: 返回 `code: 0`，响应体包含新创建的帖子 ID

---

#### TC-F-022：帖子写入 MySQL 并同步到 ES

- **需求来源**: Story 2 (已登录用户发失物招领帖) - AC: 帖子写入 MySQL + 同步到 ES
- **优先级**: 高
- **前置条件**:
  - 帖子已成功发布（TC-F-021 通过）
  - ES 服务正常运行
- **测试步骤**:
  1. 发布一个测试帖子（标题：`测试失物招领_E2E_001`）
  2. 通过读取帖子接口查询刚发布的帖子 ID，确认 MySQL 数据存在
  3. 通过搜索接口搜索标题 `测试失物招领_E2E_001`，确认 ES 索引中存在
- **预期结果**: MySQL 和 ES 中均存在该帖子，数据一致

---

#### TC-F-023：图片上传接口 HTTPS 可用

- **需求来源**: Story 2 (已登录用户发失物招领帖) - AC: 图片上传走 multipart/form-data
- **优先级**: 高
- **前置条件**:
  - 用户已登录
  - 上传域名已配置为合法域名
- **测试步骤**:
  1. 发送 `POST https://rithupc.cn/api/v1/files/upload`
  2. 请求头携带 `Authorization` 和有效的 multipart/form-data
  3. 上传一张测试图片
  4. 检查响应体是否返回文件 URL
- **预期结果**: 返回 `code: 0`，响应体包含图片访问 URL

---

#### TC-F-024：业务接口 P99 响应时间 < 500ms

- **需求来源**: Success Metrics - 业务接口响应时间 P99 < 500ms
- **优先级**: 中
- **前置条件**:
  - HTTPS 部署完成，gateway 正常运行
- **测试步骤**:
  1. 对 `GET https://rithupc.cn/api/v1/content/posts?school_id=1` 连续发送 100 个请求
  2. 记录每次请求的响应时间
  3. 排序后取第 99 百分位
- **预期结果**: P99 响应时间 < 500ms

---

### 2.6 体验版与正式版

#### TC-F-025：体验版上传代码成功

- **需求来源**: Phase C (体验版 + 正式版) - 上传代码为体验版
- **优先级**: 高
- **前置条件**:
  - Phase B 自测通过
  - 微信公众平台账号可用
- **测试步骤**:
  1. 登录微信公众平台 → 版本管理
  2. 上传小程序代码
  3. 选择"体验版"，设置体验版有效期
  4. 确认上传状态
- **预期结果**: 代码上传成功，体验版状态显示"已启用"

---

#### TC-F-026：体验版二维码可生成并扫码进入

- **需求来源**: Story 4 (体验版用户 14 天有效期) / Phase C
- **优先级**: 高
- **前置条件**:
  - 体验版已上传成功（TC-F-025）
- **测试步骤**:
  1. 在微信公众平台版本管理页面，点击体验版二维码
  2. 下载二维码图片
  3. 用微信真机扫描二维码
  4. 观察是否成功打开小程序
- **预期结果**: 真机微信扫码后正常打开小程序体验版，无报错

---

#### TC-F-027：体验版有效期 14 天配置

- **需求来源**: Story 4 (体验版用户 14 天有效期) - AC: 配置体验版有效期 14 天
- **优先级**: 中
- **前置条件**:
  - 体验版已上传
- **测试步骤**:
  1. 在微信公众平台版本管理页面
  2. 查看体验版有效期设置
  3. 确认是否可设置为 14 天
- **预期结果**: 体验版有效期可配置为 14 天，页面显示到期日期

---

#### TC-F-028：提交微信审核

- **需求来源**: Phase C - 提交微信审核
- **优先级**: 高
- **前置条件**:
  - 体验版收集反馈完成，关键 bug 已修复
  - 类目资质、隐私协议等材料已准备
- **测试步骤**:
  1. 登录微信公众平台 → 版本管理 → 提交审核
  2. 填写审核信息（版本描述、类目、隐私协议 URL 等）
  3. 提交审核
  4. 查看审核状态
- **预期结果**: 审核提交成功，状态显示"审核中"

---

#### TC-F-029：正式版发布

- **需求来源**: Phase C - 审核通过 → 正式发布
- **优先级**: 高
- **前置条件**:
  - 微信审核已通过（收到通过通知）
- **测试步骤**:
  1. 登录微信公众平台 → 版本管理
  2. 查看审核通过状态
  3. 点击"发布"
  4. 确认发布成功
  5. 真机微信搜索或扫码进入正式版小程序
- **预期结果**: 正式版发布成功，用户可通过搜索或扫码访问

---

#### TC-F-030：体验版切换正式版后体验限制解除

- **需求来源**: Story 4 (体验版用户 14 天有效期) - AC: 提交正式审核后切换为正式版，体验限制解除
- **优先级**: 中
- **前置条件**:
  - 正式版已发布
- **测试步骤**:
  1. 使用之前体验版的用户，重新进入小程序
  2. 确认是否可以正常访问所有功能
  3. 确认是否不再提示"体验已结束"
- **预期结果**: 正式版用户可正常访问所有功能，无体验版限制提示

---

### 2.7 验证脚本

#### TC-F-031：verify-https.sh 业务接口检查路径正确

- **需求来源**: Story 3 (部署验证) - verify-https.sh 第 6 项
- **优先级**: 中
- **前置条件**:
  - HTTPS 部署完成
- **测试步骤**:
  1. 执行 `curl -sk -o /dev/null -w "%{http_code}" "https://rithupc.cn/api/v1/content/posts?school_id=1"`
  2. 检查 HTTP 状态码
  3. 执行 `curl -sk -o /dev/null -w "%{http_code}" "https://rithupc.cn/health"`
  4. 检查 HTTP 状态码
- **预期结果**: 两个接口均返回 200

---

## 3. 边界测试（TC-E）

### 3.1 Nginx 配置边界

#### TC-E-001：Nginx worker 连接数达到上限

- **需求来源**: Feature 1 (Nginx 反向代理) - High concurrency: worker_connections 4096
- **优先级**: 中
- **前置条件**:
  - Nginx 已配置 `worker_connections 4096`
  - 可使用压测工具（如 wrk/ab）
- **测试步骤**:
  1. 使用 `ab -n 10000 -c 100 https://rithupc.cn/health` 发送 10000 请求，100 并发
  2. 检查响应状态码分布
  3. 检查是否有 502/503 错误
  4. 查看 Nginx error.log 是否有 `worker_connections are not enough` 警告
- **预期结果**: 所有请求返回 200，无 502/503 错误，Nginx 无 worker 连接不足警告

---

#### TC-E-002：client_max_body_size 上传文件刚好 10MB

- **需求来源**: Feature 1 (Nginx 反向代理) - client_max_body_size 10M
- **优先级**: 中
- **前置条件**:
  - 用户已登录
- **测试步骤**:
  1. 准备一个恰好 10MB 的文件
  2. 通过 `POST https://rithupc.cn/api/v1/files/upload` 上传
  3. 检查响应状态码
- **预期结果**: 上传成功，返回 200 和文件 URL

---

#### TC-E-003：client_max_body_size 超过 10MB

- **需求来源**: Feature 1 (Nginx 反向代理) - client_max_body_size 10M 边界
- **优先级**: 中
- **前置条件**:
  - 用户已登录
- **测试步骤**:
  1. 准备一个 11MB 的文件
  2. 通过 `POST https://rithupc.cn/api/v1/files/upload` 上传
  3. 检查响应状态码
- **预期结果**: 返回 413 (Request Entity Too Large) 或网关层拒绝，不返回 500

---

#### TC-E-004：TLS 握手时间 < 100ms

- **需求来源**: Technical Constraints - Performance: TLS 握手时间 < 100ms
- **优先级**: 低
- **前置条件**:
  - HTTPS 部署完成
- **测试步骤**:
  1. 执行 `curl -sk -w "TLS handshake: %{time_appconnect}s\n" -o /dev/null https://rithupc.cn/health`
  2. 重复 10 次，记录每次 TLS 握手时间
  3. 计算平均值
- **预期结果**: 平均 TLS 握手时间 < 100ms

---

#### TC-E-005：SSL Session Timeout 1 天生效

- **需求来源**: Technical Constraints - Performance: ssl_session_timeout 1d
- **优先级**: 低
- **前置条件**:
  - Nginx SSL session cache 已启用
- **测试步骤**:
  1. 连接 `rithupc.cn:443`，获取 SSL session ID
  2. 等待一段时间后再次连接，确认 session 仍被复用
  3. 确认 Nginx 配置中 `ssl_session_timeout 1d` 正确
- **预期结果**: session timeout 在 1 天内有效，超时后不再复用

---

### 3.2 安全头边界

#### TC-E-006：HSTS 头存在且 max-age 正确

- **需求来源**: Security - HSTS: max-age=31536000
- **优先级**: 高
- **前置条件**:
  - HTTPS 部署完成
- **测试步骤**:
  1. 执行 `curl -skI https://rithupc.cn/health | grep -i "strict-transport-security"`
  2. 检查返回值是否包含 `max-age=31536000`
- **预期结果**: 响应头包含 `Strict-Transport-Security: max-age=31536000`

---

#### TC-E-007：X-Frame-Options 头阻止 iframe 嵌入

- **需求来源**: Security - X-Frame-Options: SAMEORIGIN
- **优先级**: 中
- **前置条件**:
  - HTTPS 部署完成
- **测试步骤**:
  1. 执行 `curl -skI https://rithupc.cn/health | grep -i "x-frame-options"`
  2. 检查返回值
- **预期结果**: 响应头包含 `X-Frame-Options: SAMEORIGIN`

---

#### TC-E-008：X-Content-Type-Options 头阻止 MIME 嗅探

- **需求来源**: Security - X-Content-Type-Options: nosniff
- **优先级**: 中
- **前置条件**:
  - HTTPS 部署完成
- **测试步骤**:
  1. 执行 `curl -skI https://rithupc.cn/health | grep -i "x-content-type-options"`
  2. 检查返回值
- **预期结果**: 响应头包含 `X-Content-Type-Options: nosniff`

---

#### TC-E-009：Referrer-Policy 头正确

- **需求来源**: Security - Referrer-Policy: strict-origin-when-cross-origin
- **优先级**: 低
- **前置条件**:
  - HTTPS 部署完成
- **测试步骤**:
  1. 执行 `curl -skI https://rithupc.cn/health | grep -i "referrer-policy"`
  2. 检查返回值
- **预期结果**: 响应头包含 `Referrer-Policy: strict-origin-when-cross-origin`

---

### 3.3 TLS 协议边界

#### TC-E-010：TLS 1.3 协议支持

- **需求来源**: Technical Constraints - TLS 1.2+ 强制
- **优先级**: 高
- **前置条件**:
  - Nginx 已配置 `ssl_protocols TLSv1.2 TLSv1.3`
- **测试步骤**:
  1. 执行 `echo | openssl s_client -connect rithupc.cn:443 -tls1_3 -servername rithupc.cn 2>/dev/null | grep "Protocol"`
  2. 检查返回协议版本
- **预期结果**: 返回 `TLSv1.3`，表明 TLS 1.3 握手成功

---

#### TC-E-011：TLS 1.2 协议支持

- **需求来源**: Technical Constraints - TLS 1.2+ 强制
- **优先级**: 高
- **前置条件**:
  - Nginx 已配置 `ssl_protocols TLSv1.2 TLSv1.3`
- **测试步骤**:
  1. 执行 `echo | openssl s_client -connect rithupc.cn:443 -tls1_2 -servername rithupc.cn 2>/dev/null | grep "Protocol"`
  2. 检查返回协议版本
- **预期结果**: 返回 `TLSv1.2`，表明 TLS 1.2 握手成功

---

#### TC-E-012：TLS 1.1 连接被拒绝

- **需求来源**: Security - TLS 1.2+ 强制（禁用 TLS 1.0/1.1）
- **优先级**: 高
- **前置条件**:
  - Nginx SSL 配置已禁用 TLS 1.1
- **测试步骤**:
  1. 执行 `echo | openssl s_client -connect rithupc.cn:443 -tls1_1 -servername rithupc.cn 2>&1`
  2. 检查是否出现握手失败错误
- **预期结果**: 握手失败，返回 `alert protocol version` 或类似错误，无法建立连接

---

#### TC-E-013：TLS 1.0 连接被拒绝

- **需求来源**: Security - TLS 1.2+ 强制（禁用 TLS 1.0/1.1）
- **优先级**: 高
- **前置条件**:
  - Nginx SSL 配置已禁用 TLS 1.0
- **测试步骤**:
  1. 执行 `echo | openssl s_client -connect rithupc.cn:443 -tls1 -servername rithupc.cn 2>&1`
  2. 检查是否出现握手失败错误
- **预期结果**: 握手失败，返回 `alert protocol version` 或类似错误，无法建立连接

---

#### TC-E-014：强密码套件验证

- **需求来源**: Security - 强密码套件: ECDHE-ECDSA-AES128-GCM-SHA256 等
- **优先级**: 中
- **前置条件**:
  - Nginx 已配置 ssl_ciphers
- **测试步骤**:
  1. 执行 `echo | openssl s_client -connect rithupc.cn:443 -servername rithupc.cn 2>/dev/null | grep "Cipher is"`
  2. 检查返回的密码套件是否在强密码列表中（如 `ECDHE-RSA-AES128-GCM-SHA256` 等）
- **预期结果**: 使用的密码套件为配置的强密码套件之一（GCM 系列）

---

### 3.4 Nginx 高并发边界

#### TC-E-015：Nginx worker_processes 验证

- **需求来源**: Feature 1 (Nginx 反向代理) - Edge case: 高并发, worker >= 4
- **优先级**: 低
- **前置条件**:
  - Nginx 已配置 `worker_processes auto`
- **测试步骤**:
  1. 执行 `docker exec campus-nginx ps aux | grep nginx | wc -l`
  2. 检查 worker 进程数
- **预期结果**: worker 进程数 >= 4（取决于 ECS CPU 核数）

---

#### TC-E-016：keepalive_timeout 65s 验证

- **需求来源**: Technical Constraints - keepalive_timeout 65s
- **优先级**: 低
- **前置条件**:
  - Nginx 已配置 `keepalive_timeout 65`
- **测试步骤**:
  1. 使用 `curl -sk --keepalive-time 100 https://rithupc.cn/health` 保持长连接
  2. 在 65 秒内发送第二个请求，确认复用连接
  3. 超过 65 秒后发送请求，确认重新建立连接
- **预期结果**: 65 秒内连接复用，超过 65 秒后重新建立新连接

---

### 3.5 域名与 DNS 边界

#### TC-E-017：带 www 的域名访问

- **需求来源**: Feature 1 (Nginx 反向代理) - server_name 配置
- **优先级**: 低
- **前置条件**:
  - Nginx server_name 配置为 `rithupc.cn`
- **测试步骤**:
  1. 执行 `curl -sk -o /dev/null -w "%{http_code}" https://www.rithupc.cn/health`
  2. 检查响应状态码
- **预期结果**: 返回 200（如 Nginx 配置了 www 域名）或 404/其他错误（如未配置）

---

#### TC-E-018：子域名访问不被反代

- **需求来源**: Feature 1 (Nginx 反向代理) - server_name 隔离
- **优先级**: 低
- **前置条件**:
  - Nginx server_name 仅配置 `rithupc.cn`
- **测试步骤**:
  1. 执行 `curl -sk -o /dev/null -w "%{http_code}" https://random.rithupc.cn/health`
  2. 检查响应状态码
- **预期结果**: 返回 Nginx 默认页面或 404，不返回 gateway 内容

---

### 3.6 体验版边界

#### TC-E-019：体验版到期前 1 天仍可访问

- **需求来源**: Story 4 (体验版用户 14 天有效期) - Edge case
- **优先级**: 中
- **前置条件**:
  - 体验版已配置 14 天有效期
  - 距离到期还有 1 天
- **测试步骤**:
  1. 用微信扫描体验版二维码
  2. 确认小程序可正常打开和使用
- **预期结果**: 到期前小程序可正常访问，无限制提示

---

#### TC-E-020：体验版到期当天提示体验已结束

- **需求来源**: Story 4 (体验版用户 14 天有效期) - AC: 体验期结束后提示"体验已结束"
- **优先级**: 中
- **前置条件**:
  - 体验版 14 天有效期已到期
- **测试步骤**:
  1. 用微信扫描体验版二维码
  2. 观察页面提示
- **预期结果**: 页面显示"体验已结束"提示，无法正常使用功能

---

#### TC-E-021：体验版可容纳 20 用户

- **需求来源**: Story 4 (体验版用户 14 天有效期) - 5-20 同学体验
- **优先级**: 低
- **前置条件**:
  - 体验版已上传
- **测试步骤**:
  1. 邀请 20 名同学同时扫描体验版二维码
  2. 每名同学完成登录和基础操作
  3. 检查是否有同学被限制访问
- **预期结果**: 20 名同学均可正常访问和使用小程序

---

## 4. 异常测试（TC-ERR）

### 4.1 Nginx 异常处理

#### TC-ERR-001：上游 gateway 不可达时返回 502

- **需求来源**: Feature 1 (Nginx 反向代理) - Error handling: 上游不可达返 502
- **优先级**: 高
- **前置条件**:
  - Nginx 正常运行
  - gateway 服务停止或不可达
- **测试步骤**:
  1. 停止 gateway 服务：`docker stop campus-gateway`
  2. 等待 5 秒
  3. 执行 `curl -sk https://rithupc.cn/health`
  4. 检查响应状态码和响应体
  5. 恢复 gateway：`docker start campus-gateway`
- **预期结果**: 返回 502 状态码，响应体为 `{"code":502,"message":"service unavailable"}` 友好 JSON

---

#### TC-ERR-002：SSL 证书文件缺失时 Nginx 启动失败

- **需求来源**: Feature 1 (Nginx 反向代理) / Feature 2 (SSL 证书) - Edge case: 证书文件缺失
- **优先级**: 高
- **前置条件**:
  - Nginx 容器已停止
- **测试步骤**:
  1. 备份当前证书文件
  2. 删除证书文件：`rm /opt/campus/deployments/nginx/certs/rithupc.cn.pem`
  3. 尝试重启 Nginx：`docker restart campus-nginx`
  4. 检查容器状态：`docker ps | grep campus-nginx`
  5. 查看日志：`docker logs campus-nginx`
  6. 恢复证书文件并重启
- **预期结果**: Nginx 容器退出（非 running），日志显示证书文件不存在的错误

---

#### TC-ERR-003：SSL 证书私钥不匹配时 Nginx 启动失败

- **需求来源**: Feature 2 (SSL 证书签发) - PEM 格式不正确
- **优先级**: 高
- **前置条件**:
  - 正常证书已备份
- **测试步骤**:
  1. 用其他域名的 key 文件替换当前 key 文件
  2. 尝试重启 Nginx
  3. 检查容器状态和日志
  4. 恢复原 key 文件并重启
- **预期结果**: Nginx 容器退出，日志显示证书和私钥不匹配的错误

---

#### TC-ERR-004：Nginx 配置语法错误时启动失败

- **需求来源**: Risk Assessment - Nginx 配置错误导致服务不可用
- **优先级**: 高
- **前置条件**:
  - Nginx 容器已停止
- **测试步骤**:
  1. 故意修改 nginx.conf（如删除分号）
  2. 尝试重启 Nginx
  3. 检查容器状态和日志
  4. 恢复原配置并重启
- **预期结果**: Nginx 启动失败，日志显示具体语法错误行号

---

#### TC-ERR-005：证书私钥文件权限错误

- **需求来源**: Security - 证书私钥文件权限 600
- **优先级**: 中
- **前置条件**:
  - 正常证书权限已备份
- **测试步骤**:
  1. 修改 key 文件权限：`chmod 644 /opt/campus/deployments/nginx/certs/rithupc.cn.key`
  2. 重启 Nginx
  3. 检查 Nginx 是否能正常启动和加载证书
  4. 恢复权限：`chmod 600` 并重启
- **预期结果**: Nginx 可启动（非 root 用户读取），但应有安全告警；建议生产使用 600

---

### 4.2 微信平台异常处理

#### TC-ERR-006：wx.login 返回无效 js_code

- **需求来源**: Feature 5 (真机调试验证) - Edge case: wx.login 返 40029
- **优先级**: 高
- **前置条件**:
  - 后端登录接口已部署
- **测试步骤**:
  1. 使用过期的 js_code（5 分钟前获取的）调用 `POST /api/v1/user/login`
  2. 检查响应状态码和错误信息
  3. 使用伪造的 js_code 调用同一接口
  4. 检查响应
- **预期结果**: 后端返回明确错误信息（如 `code: 40029`），不崩溃，不返回 500

---

#### TC-ERR-007：微信 AppID/Secret 配置不一致

- **需求来源**: Feature 5 (真机调试验证) - Edge case: wx.login 返 40029
- **优先级**: 高
- **前置条件**:
  - 后端微信配置可修改
- **测试步骤**:
  1. 修改后端配置中的微信 Secret 为错误值
  2. 使用有效的 js_code 调用登录接口
  3. 检查响应
  4. 恢复正确 Secret
- **预期结果**: 返回微信 API 错误（如 `invalid appsecret`），不暴露 Secret 值

---

#### TC-ERR-008：微信公众平台域名校验文件无法下载

- **需求来源**: Feature 4 (微信公众平台配置) - Edge case: 校验文件无法下载
- **优先级**: 高
- **前置条件**:
  - 微信校验文件未放置到 `.well-known/` 目录
- **测试步骤**:
  1. 暂时移除 `.well-known/` 目录中的校验文件
  2. 在微信公众平台尝试保存域名配置
  3. 检查微信后台返回的错误信息
  4. 恢复校验文件
- **预期结果**: 微信公众平台提示域名校验失败（无法下载校验文件），保存不成功

---

#### TC-ERR-009：域名未完成 ICP 备案时添加失败

- **需求来源**: Feature 4 (微信公众平台配置) - Edge case: 添加失败，检查域名是否已 ICP 备案
- **优先级**: 中
- **前置条件**:
  - 使用一个未备案的域名进行测试
- **测试步骤**:
  1. 在微信公众平台尝试添加一个未备案的域名
  2. 观察保存时的错误提示
- **预期结果**: 微信公众平台提示域名未完成备案，不允许添加

---

### 4.3 网络与基础设施异常

#### TC-ERR-010：ECS 安全组未放行 443 端口

- **需求来源**: Phase A - ECS 安全组放行 80/443 端口
- **优先级**: 高
- **前置条件**:
  - 安全组配置可修改
- **测试步骤**:
  1. 在阿里云控制台临时移除安全组 443 端口规则
  2. 从外部执行 `curl -sk https://rithupc.cn/health`
  3. 检查连接是否超时
  4. 恢复安全组规则
- **预期结果**: 连接超时或被拒绝，无法访问 HTTPS 服务

---

#### TC-ERR-011：ECS 安全组未放行 80 端口

- **需求来源**: Phase A - ECS 安全组放行 80/443 端口
- **优先级**: 高
- **前置条件**:
  - 安全组配置可修改
- **测试步骤**:
  1. 在阿里云控制台临时移除安全组 80 端口规则
  2. 从外部执行 `curl -s -o /dev/null -w "%{http_code}" http://rithupc.cn/health`
  3. 检查连接是否超时
  4. 恢复安全组规则
- **预期结果**: 连接超时或被拒绝，HTTP 跳转无法完成

---

#### TC-ERR-012：Nginx 容器意外重启后自动恢复

- **需求来源**: Feature 1 (Nginx 反向代理) - docker-compose restart: always
- **优先级**: 中
- **前置条件**:
  - Nginx 容器已配置 `restart: always`
- **测试步骤**:
  1. 执行 `docker kill campus-nginx`（模拟意外崩溃）
  2. 等待 10 秒
  3. 执行 `docker ps | grep campus-nginx`
  4. 执行 `curl -sk https://rithupc.cn/health` 验证服务恢复
- **预期结果**: 容器在 10 秒内自动重启，HTTPS 服务恢复正常

---

#### TC-ERR-013：gateway 服务崩溃后 Nginx 返回 502 并恢复后自动恢复

- **需求来源**: Feature 1 (Nginx 反向代理) - Error handling: 上游不可达
- **优先级**: 中
- **前置条件**:
  - Nginx 和 gateway 均正常运行
- **测试步骤**:
  1. 执行 `docker kill campus-gateway`
  2. 立即执行 `curl -sk https://rithupc.cn/health`，确认返回 502
  3. 执行 `docker start campus-gateway`
  4. 等待 gateway 健康检查通过
  5. 执行 `curl -sk https://rithupc.cn/health`，确认恢复 200
- **预期结果**: gateway 停止时返回 502，gateway 恢复后自动恢复 200

---

#### TC-ERR-014：DNS 记录变更后 Nginx 行为

- **需求来源**: Risk Assessment - ECS 公网 IP 变更
- **优先级**: 低
- **前置条件**:
  - DNS 记录可修改（测试环境）
- **测试步骤**:
  1. 记录当前 DNS 解析 IP
  2. 临时将 DNS A 记录改为其他 IP
  3. 执行 `dig +short rithupc.cn` 确认变更
  4. 恢复原 DNS 记录
- **预期结果**: DNS 变更后域名解析到新 IP，恢复后解析回原 IP，Nginx 服务本身不受影响

---

### 4.4 证书过期异常

#### TC-ERR-015：证书即将过期告警

- **需求来源**: Risk Assessment - 免费证书 1 年过期
- **优先级**: 中
- **前置条件**:
  - 可模拟证书即将过期（通过调整系统时间或使用测试证书）
- **测试步骤**:
  1. 检查证书到期日期
  2. 确认到期前 30 天是否有监控/告警机制
- **预期结果**: 文档中记录证书到期日期，提醒提前续期；当前版本无自动告警（v2.1 计划）

---

#### TC-ERR-016：证书过期后 HTTPS 访问

- **需求来源**: Risk Assessment - 免费证书 1 年过期
- **优先级**: 中
- **前置条件**:
  - 可使用已过期的测试证书
- **测试步骤**:
  1. 替换为过期证书
  2. 重启 Nginx
  3. 执行 `curl -sk https://rithupc.cn/health`
  4. 检查浏览器/客户端是否报证书过期错误
  5. 恢复正常证书
- **预期结果**: 浏览器/微信客户端提示证书过期，拒绝访问（curl -k 可绕过但仍返回警告）

---

## 5. 状态转换测试（TC-ST）

### 5.1 服务生命周期状态转换

#### TC-ST-001：Nginx 从停止到启动的完整流程

- **需求来源**: Feature 1 (Nginx 反向代理) / Phase A
- **优先级**: 高
- **前置条件**:
  - Nginx 容器已停止
- **测试步骤**:
  1. 确认 Nginx 容器状态为 stopped：`docker ps -a | grep campus-nginx`
  2. 启动 Nginx：`docker start campus-nginx`
  3. 等待 5 秒
  4. 确认容器状态为 running：`docker ps | grep campus-nginx`
  5. 执行 `curl -sk https://rithupc.cn/health` 验证可用
- **预期结果**: 容器从 stopped 转为 running，HTTPS 服务可用

---

#### TC-ST-002：gateway 服务重启后 Nginx 反代恢复

- **需求来源**: Feature 1 (Nginx 反向代理) - depends_on gateway
- **优先级**: 高
- **前置条件**:
  - Nginx 和 gateway 均正常运行
- **测试步骤**:
  1. 记录当前状态：Nginx running, gateway running
  2. 重启 gateway：`docker restart campus-gateway`
  3. 等待 gateway 健康检查通过
  4. 检查 Nginx 是否自动重新连接 gateway
  5. 执行 `curl -sk https://rithupc.cn/health`
- **预期结果**: gateway 重启后，Nginx 自动恢复反代，业务接口返回 200

---

#### TC-ST-003：docker-compose up 全量启动顺序

- **需求来源**: Feature 1 (Nginx 反向代理) - depends_on: gateway condition: service_healthy
- **优先级**: 高
- **前置条件**:
  - 所有容器已停止
- **测试步骤**:
  1. 停止所有容器：`docker-compose down`
  2. 执行 `docker-compose up -d`
  3. 监控启动顺序：`docker-compose logs -f`
  4. 确认 gateway 先启动并通过健康检查
  5. 确认 Nginx 在 gateway 健康后启动
  6. 执行 `curl -sk https://rithupc.cn/health`
- **预期结果**: 启动顺序为 gateway → Nginx，最终 HTTPS 服务可用

---

### 5.2 上线阶段状态转换

#### TC-ST-004：Phase A 完成状态 → Phase B 入口

- **需求来源**: Phase A → Phase B 转换
- **优先级**: 高
- **前置条件**:
  - Phase A 全部 checklist 完成
- **测试步骤**:
  1. 逐项确认 Phase A checklist（docker compose 加 nginx、证书申请、上传、配置、安全组、health 验证）
  2. 执行 `verify-https.sh` 确认全部通过
  3. 确认可以进入 Phase B（修改小程序配置）
- **预期结果**: Phase A 交付物全部完成，可进入 Phase B

---

#### TC-ST-005：Phase B 完成状态 → Phase C 入口

- **需求来源**: Phase B → Phase C 转换
- **优先级**: 高
- **前置条件**:
  - Phase B 全部 checklist 完成
- **测试步骤**:
  1. 逐项确认 Phase B checklist（BASE_URL 修改、urlCheck 删除、合法域名添加、真机调试全链路通过）
  2. 确认真机扫码无"非合法域名"提示
  3. 确认"登录 → 发帖 → 读帖 → 点赞"全链路通过
  4. 确认可以进入 Phase C（上传体验版）
- **预期结果**: Phase B 交付物全部完成，可进入 Phase C

---

#### TC-ST-006：体验版 → 正式版状态转换

- **需求来源**: Phase C - 审核通过 → 正式发布
- **优先级**: 高
- **前置条件**:
  - 体验版已上线
  - 提交审核并通过
- **测试步骤**:
  1. 记录当前状态：体验版可用，有 14 天有效期限制
  2. 完成审核流程并发布正式版
  3. 验证正式版状态：无 14 天有效期限制
  4. 确认所有用户（非仅体验版用户）可访问
- **预期结果**: 小程序从体验版转为正式版，限制解除，全量用户可访问

---

### 5.3 登录状态转换

#### TC-ST-007：未登录 → 登录中 → 已登录 → 登出状态转换

- **需求来源**: Story 1 (扫码进入小程序) + 通用登录流程
- **优先级**: 高
- **前置条件**:
  - 小程序已正确配置，HTTPS 可用
- **测试步骤**:
  1. 打开小程序，确认处于未登录状态（显示登录按钮）
  2. 点击登录，触发 wx.login，确认进入"登录中"状态（loading）
  3. 登录成功，确认跳转首页，显示用户昵称（已登录状态）
  4. 进入个人中心，点击退出登录
  5. 确认回到未登录状态
- **预期结果**: 状态转换正确：未登录 → 登录中 → 已登录 → 未登录

---

#### TC-ST-008：登录 Token 过期后的状态转换

- **需求来源**: Story 1 (登录) + 安全要求
- **优先级**: 中
- **前置条件**:
  - 用户已登录，持有 access_token
- **测试步骤**:
  1. 使用已登录状态访问业务接口（如发帖）
  2. 等待 token 过期（或手动使 token 失效）
  3. 再次访问业务接口
  4. 检查是否触发重新登录
- **预期结果**: token 过期后接口返回未授权错误，客户端跳转到登录页面

---

### 5.4 微信平台配置状态转换

#### TC-ST-009：微信公众平台域名配置从"未添加"到"已验证"

- **需求来源**: Feature 4 (微信公众平台配置)
- **优先级**: 高
- **前置条件**:
  - 域名未添加到微信公众平台
- **测试步骤**:
  1. 确认当前微信公众平台服务器域名列表中无 `rithupc.cn`
  2. 下载微信校验文件
  3. 将校验文件放到 Nginx 的 `.well-known/` 目录
  4. 在微信公众平台添加 `https://rithupc.cn` 为合法域名
  5. 点击保存，触发校验
  6. 确认校验通过
  7. 确认域名出现在合法域名列表中
- **预期结果**: 域名从"未添加"成功转为"已验证"状态

---

#### TC-ST-010：SSL 证书从申请到生效的完整流程

- **需求来源**: Feature 2 (SSL 证书签发)
- **优先级**: 高
- **前置条件**:
  - 阿里云 SSL 控制台可访问
- **测试步骤**:
  1. 在阿里云 SSL 控制台申请免费个人测试证书
  2. 添加 DNS TXT 记录（`_dnsauth.rithupc.cn`）
  3. 等待 DNS 生效（`dig _dnsauth.rithupc.cn TXT`）
  4. 等待证书签发
  5. 下载证书（Nginx 格式）
  6. 上传到 ECS `/opt/campus/deployments/nginx/certs/`
  7. 配置 Nginx 引用证书
  8. 重启/重载 Nginx
  9. 验证 `https://rithupc.cn/health` 返回 200
- **预期结果**: 证书从"申请中" → "签发中" → "已签发" → "已部署" → "HTTPS 可用"，全流程成功

---

### 5.5 代码/配置变更状态转换

#### TC-ST-011：小程序从 HTTP 模式切换到 HTTPS 模式

- **需求来源**: Feature 3 (小程序 baseURL HTTPS 化)
- **优先级**: 高
- **前置条件**:
  - 小程序当前使用 HTTP BASE_URL
- **测试步骤**:
  1. 确认当前 `constants.js` 中 `BASE_URL` 为 HTTP
  2. 修改 `BASE_URL` 为 `https://rithupc.cn/api/v1`
  3. 在微信开发者工具中重新编译
  4. 验证请求通过 HTTPS 发送
  5. 验证业务接口正常返回
- **预期结果**: 小程序从 HTTP 模式切换到 HTTPS 模式，所有请求走 HTTPS

---

#### TC-ST-012：urlCheck 从 false 到 true 的状态转换

- **需求来源**: Feature 3 (小程序 baseURL HTTPS 化) - 删除 urlCheck: false
- **优先级**: 高
- **前置条件**:
  - project.config.json 当前包含 `urlCheck: false`
- **测试步骤**:
  1. 确认当前 `urlCheck: false` 存在
  2. 删除该字段
  3. 在微信开发者工具中重新编译
  4. 尝试访问一个不在合法域名列表中的 URL
  5. 确认请求被拦截
- **预期结果**: 删除后微信开发者工具启用域名校验，非法域名请求被拦截

---

## 6. 需求-测试用例覆盖矩阵

| 需求编号 | 需求描述 | 测试用例编号 |
|---------|---------|-------------|
| Feature 1 | Nginx 反向代理（HTTPS 终止） | TC-F-001, TC-F-002, TC-F-003, TC-F-004, TC-F-005, TC-E-001, TC-E-002, TC-E-003, TC-E-015, TC-E-016, TC-ERR-001, TC-ERR-002, TC-ERR-003, TC-ERR-004, TC-ERR-005, TC-ERR-012, TC-ERR-013, TC-ST-001, TC-ST-002, TC-ST-003 |
| Feature 2 | SSL 证书签发（阿里云免费个人测试证书） | TC-F-008, TC-F-009, TC-F-010, TC-E-004, TC-E-005, TC-ERR-002, TC-ERR-003, TC-ERR-015, TC-ERR-016, TC-ST-010 |
| Feature 3 | 小程序 baseURL HTTPS 化 | TC-F-011, TC-F-012, TC-F-013, TC-ST-011, TC-ST-012 |
| Feature 4 | 微信公众平台配置 | TC-F-014, TC-F-015, TC-F-016, TC-F-017, TC-ERR-008, TC-ERR-009, TC-ST-009 |
| Feature 5 | 真机调试验证 | TC-F-018, TC-F-019, TC-F-020, TC-ERR-006, TC-ERR-007 |
| Story 1 | 大学生通过微信扫码进入小程序 | TC-F-018, TC-F-019, TC-F-020, TC-ST-007, TC-ST-008 |
| Story 2 | 已登录用户发失物招领帖 | TC-F-021, TC-F-022, TC-F-023 |
| Story 3 | 部署同学一次性验证 HTTPS 状态 | TC-F-006, TC-F-007, TC-F-031 |
| Story 4 | 体验版用户 14 天有效期 | TC-F-025, TC-F-026, TC-F-027, TC-F-030, TC-E-019, TC-E-020, TC-E-021, TC-ST-006 |
| Success Metrics | HTTPS 可用性 | TC-F-001, TC-F-006, TC-F-007 |
| Success Metrics | SSL 证书有效 | TC-F-008, TC-F-009 |
| Success Metrics | wx.login 成功率 ≥ 95% | TC-F-018, TC-F-019, TC-ERR-006 |
| Success Metrics | 业务接口 P99 < 500ms | TC-F-024, TC-E-004 |
| Performance | TLS 握手 < 100ms | TC-E-004 |
| Performance | Nginx worker ≥ 4 | TC-E-001, TC-E-015 |
| Performance | keepalive_timeout 65s | TC-E-016 |
| Performance | SSL session cache | TC-F-010, TC-E-005 |
| Security | TLS 1.2+ 强制 | TC-E-010, TC-E-011, TC-E-012, TC-E-013 |
| Security | 强密码套件 | TC-E-014 |
| Security | HSTS 头 | TC-E-006 |
| Security | 安全头（X-Frame-Options 等） | TC-E-007, TC-E-008, TC-E-009 |
| Security | 证书私钥权限 600 | TC-ERR-005 |
| Security | 安全组放行 80/443 | TC-ERR-010, TC-ERR-011 |
| Phase A | HTTPS 基础设施部署 | TC-ST-001, TC-ST-003, TC-ST-004, TC-ST-010 |
| Phase B | 小程序配置 + 自测 | TC-F-011, TC-F-012, TC-F-013, TC-F-014, TC-F-015, TC-F-016, TC-ST-005, TC-ST-011, TC-ST-012 |
| Phase C | 体验版 + 正式版 | TC-F-025, TC-F-026, TC-F-027, TC-F-028, TC-F-029, TC-F-030, TC-ST-006 |
| Risk: Nginx 配置错误 | Nginx 配置错误导致服务不可用 | TC-ERR-004 |
| Risk: 证书申请失败 | 阿里云免费证书申请失败 | TC-ERR-015, TC-ST-010 |
| Risk: IP 变更 | ECS 公网 IP 变更 | TC-ERR-014 |
| Risk: 免费证书限制 | 证书 1 年过期 | TC-ERR-015, TC-ERR-016 |

---

## 附录：测试用例统计

| 类别 | 编号前缀 | 数量 |
|------|---------|------|
| 功能测试 | TC-F | 31 |
| 边界测试 | TC-E | 21 |
| 异常测试 | TC-ERR | 16 |
| 状态转换测试 | TC-ST | 12 |
| **合计** | | **80** |

| 优先级 | 数量 |
|--------|------|
| 高 | 42 |
| 中 | 26 |
| 低 | 12 |

---

*本文档基于 wechat-miniapp-launch-prd.md (v2.0) 生成，覆盖 HTTPS 基础设施、小程序配置、微信平台对接、上线路径四大领域，共 80 个测试用例。*
