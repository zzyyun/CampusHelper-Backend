# Product Requirements Document: CampusHelper 微信小程序上线（v2.0）

**Version**: 2.0
**Date**: 2026-06-28
**Author**: Sarah (Product Owner)
**Quality Score**: 85/100
**项目**: CampusHelper-Backend（校园互助平台后端）
**类型**: 上线工程 / 部署加固 PRD
**前置 PRD**: v1.1 `docs/cloud-deployment-prd.md`（基础设施已就绪）

---

## Executive Summary

将 CampusHelper 微信小程序从"本地开发联调"升级到"生产环境上线"。当前 v1 后端已在阿里云 ECS（121.41.74.238）通过 docker-compose 拉起 10 个容器并健康运行，但服务直接以 **HTTP 50000 端口** 暴露，不满足微信小程序对生产环境的强制要求（**必须 HTTPS、必须 ICP 备案域名、必须配置 request 合法域名**）。

本 PRD 的核心工作是在 ECS 上引入 **Nginx 反向代理 + 阿里云免费个人测试证书**，将 443 端口的 HTTPS 流量反代到 gateway 服务的 50000 端口，强制 HTTP 跳转 HTTPS；同时在微信公众平台后台配置 `rithupc.cn` 为合法请求域名，让小程序用户能通过真实 `wx.login()` 走通完整主链路。

**目标**：7 天内完成 HTTPS 化 + 体验版上线（5-20 同学可访问），15 天内通过微信审核发布正式版。

**Why now**：课设答辩与期末临近，需要可被外部用户访问的演示环境；同时 ICP 备案 + 域名已就绪，缺少的最后一块拼图是 HTTPS 与微信平台配置。

---

## Problem Statement

**Current Situation（现状）**：
- 后端服务运行于 `http://121.41.74.238:50000`（HTTP，无 TLS）
- 小程序 `utils/constants.js` 中 `BASE_URL = 'http://rithupc.cn:50000/api/v1'`
- 域名 `rithupc.cn` 已 ICP 备案，DNS A 记录已指向 `121.41.74.238` ✅
- ECS 443 端口未监听（仅 50000 通过 docker-proxy 暴露）
- 微信小程序要求：生产环境 **必须 HTTPS + 已备案域名 + request 合法域名白名单**
- 当前 wx.login() 测试只能返回 `40029 invalid code`（appid 正确，code 需真机），但**真机调试时所有 HTTPS 请求会被微信拦截**

**Proposed Solution（方案）**：

1. **HTTPS 化（核心）**
   - ECS 安装 Nginx（docker compose 增加 `campus-nginx` 容器，alpine:3.19 基础镜像）
   - 用 **阿里云免费个人测试证书**（控制台申请 → DNS 验证 → 下载 → 上传 ECS）
   - Nginx 反代 `443 → campus-gateway:50000`，强制 HTTP 301 跳转 HTTPS
   - WebSocket 升级头保留（为后续 message service WebSocket 推送预留）

2. **小程序配置更新**
   - `utils/constants.js` 中 `BASE_URL = 'https://rithupc.cn/api/v1'`（去端口，强制 HTTPS）
   - 微信公众平台 → 开发管理 → 服务器域名 → request/uploadFile/downloadFile 合法域名 添加 `https://rithupc.cn`
   - `project.config.json` 删除 `urlCheck: false`（关闭"不校验合法域名"）

3. **微信平台对接验证**
   - 真实 `wx.login()` 拿到 js_code → 调 `POST /api/v1/user/login` → 拿到 `access_token`
   - 用 access_token 调业务接口（发帖/读帖/点赞/任务）
   - 验证 `https://rithupc.cn/health` 公网可访问 + 200

4. **上线路径**
   - **Phase A（Day 1-3）**：HTTPS 化 + 自测（开发者工具 + 真机调试）
   - **Phase B（Day 4-7）**：体验版（上传代码 + 生成体验二维码，5-20 同学体验 14 天）
   - **Phase C（Day 8-15）**：提交微信审核 → 正式发布

**Business Impact（业务价值）**：
- **从 demo 到 production**：满足微信小程序生产环境所有硬性要求
- **可被外部访问**：二维码、微信搜索、扫码都能进入
- **答辩交付物升级**：可演示真实用户场景（同学扫码登录、发失物招领）
- **安全合规**：HTTPS 加密传输，用户登录 token 不被中间人窃取

---

## Success Metrics

**Primary KPIs**：

| 指标 | 目标值 | 测量方法 |
|------|--------|----------|
| HTTPS 可用性 | `https://rithupc.cn/health` 持续 200 | curl 定时巡检（每 5 分钟）|
| SSL 证书有效 | 部署时证书已签发且加载成功 | openssl x509 一次性检查（部署时）|
| 真机 wx.login 成功率 | ≥ 95% | 微信开发者工具 + 真机调试日志 |
| 业务接口响应时间 | P99 < 500ms | gateway 日志 + Jaeger trace |
| 体验版用户数 | ≥ 5 个同学体验 14 天 | 微信公众平台后台 |
| 微信审核通过 | Phase C 提交后 ≤ 3 天通过 | 微信通知 |

**Validation（验证）**：
- 阶段交付物：每阶段结束跑 `verify-https.sh`（本文档附录 B）
- 最终验收：`https://rithupc.cn/health` + 真机走通"微信登录→发失物招领→读帖子"全链路

---

## User Personas

### Primary: 大学生用户（失主/拾主/跑腿/拼车发起人）
- **Role**: 高校在读学生，微信小程序活跃用户
- **Goals**: 找回丢失物品、发起跑腿代拿、加入拼车队伍、买卖二手
- **Pain Points**:
  - 之前只能本地 dev tools 跑，无法真实分享给同学
  - 扫码必须能进，否则不信任产品
- **Technical Level**: 微信小程序深度用户，懂扫码但不懂技术

### Secondary: 平台管理员（教师/学生助理）
- **Role**: 内容审核、用户管理、数据查看
- **Goals**: 审核违规内容、封禁恶意用户、看运营数据
- **Pain Points**:
  - 管理员后台只在 v2.1 计划中，本期 v2.0 仅做基础发布
- **Technical Level**: 中等

### Tertiary: 部署/运维同学（开发者本人）
- **Role**: 维护后端服务，处理上线问题
- **Goals**: 服务能上线运行、证书正确加载、日志可查
- **Pain Points**:
  - 当前需要 SSH ECS 手动操作，希望有脚本化部署
- **Technical Level**: 高级

---

## User Stories & Acceptance Criteria

### Story 1: 大学生通过微信扫码进入小程序

**As a** 失主大学生
**I want to** 在校园里扫同学的"失物招领"二维码进入小程序
**So that** 能直接联系拾主拿回物品

**Acceptance Criteria**：
- [ ] 微信扫描二维码后正常打开小程序首页（无"非合法域名"提示）
- [ ] 小程序顶部 loading 后显示学校列表
- [ ] 微信登录按钮可点击，调起 `wx.login()` 成功获取 js_code
- [ ] 后端用 js_code + 微信 API 换 access_token 成功（返回 `code: 0`）
- [ ] 登录成功后自动跳转首页，显示用户昵称

### Story 2: 已登录用户发失物招领帖

**As a** 已登录大学生
**I want to** 在小程序发帖（"丢失黑色雨伞一把"）
**So that** 同学能看到并联系

**Acceptance Criteria**：
- [ ] HTTPS 请求 `POST https://rithupc.cn/api/v1/content/posts` 成功
- [ ] 返回 `code: 0` + 帖子 ID
- [ ] 帖子写入 MySQL + 同步到 ES（搜索能搜到）
- [ ] 图片上传走 `https://rithupc.cn/api/v1/files/upload`（multipart/form-data）

### Story 3: 部署同学一次性验证 HTTPS 状态

**As a** 部署/运维同学
**I want to** 部署后用脚本一次性确认 HTTPS 正常 + 证书已加载
**So that** 知道上线准备已完成

**Acceptance Criteria**：
- [ ] `bash scripts/verify-https.sh` 跑通（见附录 B）
- [ ] 全部 6 项检查通过：DNS、HTTPS /health、HTTP 跳转、证书有效、SSL 协议、业务接口
- [ ] 输出"✅ HTTPS 验证全部通过"

> 注：本 PRD 范围不考虑长期证书监控/自动续期（如需要可后续做 v2.1）

### Story 4: 体验版用户 14 天有效期

**As a** 体验版用户
**I want to** 体验 14 天后系统提示体验结束
**So that** 知道这是预览版不是正式版

**Acceptance Criteria**：
- [ ] 微信公众平台配置体验版有效期 14 天
- [ ] 体验期结束后用户访问提示"体验已结束"
- [ ] 提交正式审核后切换为正式版，体验限制解除

---

## Functional Requirements

### Core Features

**Feature 1: Nginx 反向代理（HTTPS 终止）**
- Description: 在 ECS 上以容器方式运行 Nginx，监听 443 端口，加载阿里云免费个人测试证书，反代到 gateway:50000
- User flow: 客户端 → `https://rithupc.cn/api/v1/...` → Nginx:443 → `campus-gateway:5000/api/v1/...` → gateway 服务
- Edge cases:
  - HTTP 访问 → 301 永久跳转到 HTTPS
  - 证书文件缺失或权限错 → Nginx 启动失败，docker logs 看详细错误
  - 高并发 → Nginx worker 数 ≥ 4，keepalive 64
- Error handling: 上游不可达时返 502 + 友好 JSON `{"code":502,"message":"service unavailable"}`

**Feature 2: SSL 证书签发（阿里云免费个人测试证书）**
- Description: 在阿里云 SSL 证书控制台申请个人测试证书（免费 1 年），DNS 验证域名所有权，下载证书（含 .pem + .key）上传到 ECS，Nginx 引用
- User flow: 阿里云控制台 → SSL 证书 → 申请免费证书 → 选"个人测试" → 填域名 rithupc.cn → DNS 验证（添加一条 TXT 记录）→ 等待签发（5-30 分钟）→ 下载 Nginx 格式 → 上传到 `/opt/campus/deployments/nginx/certs/` → Nginx reload
- Edge cases:
  - DNS 验证失败 → 检查 `dig _dnsauth.rithupc.cn TXT` 是否返回预期值
  - 下载的 PEM 格式不对 → 用 `cat fullchain.pem privkey.pem > cert.pem` 合并
- Error handling: 证书加载失败时 Nginx 启动失败，docker logs 看详细错误
- **⚠️ 重要限制**：阿里云免费个人测试证书**仅可用于个人测试**，不可用于商业生产；本 PRD 范围为课设验证演示，**不考虑长期服务**（证书 1 年过期后另行处理）

**Feature 3: 小程序 baseURL HTTPS 化**
- Description: 修改 `scripts/miniapp/utils/constants.js`，将 HTTP 改为 HTTPS 并去端口
- User flow: 用户改 `BASE_URL` → 重新编译小程序 → 真机预览
- Edge cases:
  - 开发期仍想用 HTTP 调试 → 用 `project.config.json` 的 `urlCheck: true` + 微信开发者工具"不校验合法域名"开关
- Error handling: 编译报错时检查 yaml/JSON 语法

**Feature 4: 微信公众平台配置**
- Description: 在微信公众平台后台添加 `rithupc.cn` 为合法请求域名
- User flow: 登录 mp.weixin.qq.com → 开发管理 → 服务器域名 → 添加 → 微信校验文件下载放到 `/var/www/html/.well-known/` → 提交
- Edge cases:
  - 校验文件无法下载 → 检查 Nginx 是否能 serve `.well-known/`
  - 添加失败 → 检查域名是否已 ICP 备案
- Error handling: 添加失败时返回明确错误信息

**Feature 5: 真机调试验证**
- Description: 用真机微信扫码开发者工具生成的预览码，跑通完整主链路
- User flow: 微信开发者工具 → 预览 → 生成二维码 → 真机扫码 → 走"登录→发帖→读帖"全链路
- Edge cases:
  - 真机无法连接后端 → 检查 443 端口在安全组放行
  - wx.login 返 `40029` → 检查 appid/secret 与 ECS 配置一致
- Error handling: 微信开发者工具 vConsole 看 console 错误

### Out of Scope（明确不在本期范围）
- ❌ 域名备案（已就绪）
- ❌ 小程序代码优化 / 性能调优
- ❌ 管理后台 UI（v2.1 计划）
- ❌ 推送通知（WebSocket、订阅消息等）
- ❌ HTTPS 双向认证（mTLS）
- ❌ WAF / 防 DDoS（v3 计划）
- ❌ 多域名支持（v3 计划）
- ❌ 内容审核 AI 升级（已独立的 v3.0 PRD）

---

## Technical Constraints

### Performance
- HTTPS TLS 握手时间 < 100ms（Nginx + 现代 cipher suite）
- 反代后业务接口 P99 < 500ms（不引入明显延迟）
- Nginx worker 数 ≥ 4，keepalive_timeout 65s
- SSL session cache 开启（shared:SSL:10m）

### Security
- TLS 1.2+ 强制（禁用 TLS 1.0/1.1）
- 强密码套件（ECDHE-ECDSA-AES128-GCM-SHA256 等）
- HSTS 头：`Strict-Transport-Security: max-age=31536000`
- 安全头：`X-Frame-Options: SAMEORIGIN` / `X-Content-Type-Options: nosniff` / `Referrer-Policy: strict-origin-when-cross-origin`
- 证书私钥文件（`*.pem` / `*.key`）权限 600，Nginx 容器以非 root 读取
- 微信 API 凭证从 Secrets 读，不入仓
- 不开放 ECS 22 端口公网（用阿里云控制台堡垒机或临时白名单）

### Integration
- **微信开放平台**：`wx.login()` → js_code → 微信 API 换 unionid + openid → 后端 JWT
- **阿里云 SSL 控制台**：申请免费个人测试证书，DNS 验证域名所有权
- **阿里云 DNS**：A 记录已配置，验证 `dig rithupc.cn +short` 返 `121.41.74.238`
- **微信公众平台**：服务器域名白名单（request/uploadFile/downloadFile）
- **Jaeger**：HTTPS 链路可追踪（gin middleware 自动加 trace headers）

### Technology Stack
- **Web 服务器**：Nginx 1.25+ (alpine 镜像)
- **TLS 协议**：TLS 1.2 / TLS 1.3
- **证书来源**：阿里云免费个人测试证书（1 年有效）
- **Docker 镜像**：`nginx:1.25-alpine`
- **配置文件**：`/opt/campus/deployments/nginx/`
- **架构**：Nginx 容器加入 campus-net，depends_on gateway

---

## MVP Scope & Phasing

### Phase A: HTTPS 化基础设施（Day 1-3）⭐ MVP
- [ ] ECS 安装 Nginx（docker compose 加 `campus-nginx` 服务）
- [ ] 阿里云 SSL 控制台申请免费个人测试证书（DNS 验证）
- [ ] 下载证书（fullchain.pem + privkey.pem）上传到 `/opt/campus/deployments/nginx/certs/`
- [ ] 配置 Nginx 反代 + HTTPS + HTTP 跳转
- [ ] ECS 安全组放行 80/443 端口
- [ ] 验证 `https://rithupc.cn/health` 公网返回 200
- [ ] 写 `verify-https.sh`（见附录 B）

**MVP 定义**：用户能通过 `https://rithupc.cn/api/v1/...` 调通全部业务接口（无"非合法域名"提示）

### Phase B: 小程序配置 + 自测（Day 4-7）
- [ ] 修改 `utils/constants.js` → `BASE_URL = 'https://rithupc.cn/api/v1'`
- [ ] 删除 `project.config.json` 的 `urlCheck: false`
- [ ] 微信公众平台 → 添加 `https://rithupc.cn` 为 request/uploadFile/downloadFile 合法域名
- [ ] 微信开发者工具预览 → 真机扫码
- [ ] 真机跑通"wx.login → 发帖 → 读帖 → 点赞"全链路
- [ ] 修复发现的 bug

### Phase C: 体验版 + 正式版（Day 8-15）
- [ ] 上传代码为体验版（mp.weixin.qq.com）
- [ ] 生成体验二维码，邀请 5-20 同学体验
- [ ] 收集反馈 + 修复关键问题
- [ ] 提交微信审核（准备类目资质、隐私协议等）
- [ ] 审核通过 → 正式发布

### Phase D: 后续增强（v2.1+，本期不做）
- [ ] WAF / 防 DDoS
- [ ] HTTPS 双向认证（mTLS）
- [ ] 多域名支持
- [ ] CDN 加速
- [ ] IPv6 支持

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| 阿里云免费证书申请失败（DNS 验证不通过） | Medium | High | 检查 `dig _dnsauth.rithupc.cn TXT`；DNS 记录添加后等 5-10 分钟生效 |
| 微信公众平台域名校验失败 | Medium | Medium | 校验文件放 `.well-known/`，Nginx 单独 location serve |
| 微信审核不通过（资质不全） | Medium | High | 提前准备：类目资质（校园助手类）、隐私协议、运营者身份证 |
| 体验版同学 14 天后失效 | Low | Low | 提前 7 天提交正式审核，预留缓冲 |
| Nginx 性能瓶颈 | Low | Medium | 4 worker + 64 keepalive + upstream keepalive |
| ECS 公网 IP 变更（重启后） | Low | High | 阿里云弹性 IP / 域名 CNAME 改 IP |
| 微信 wx.login 频次限制 | Low | Low | 后端缓存 session_key，频次控制 |
| Nginx 配置错误导致服务不可用 | Low | High | 配置 diff 审核 + nginx -t 语法检查 + reload（不 restart）|
| **免费证书仅限个人测试** | High | High | ⚠️ 阿里云明确禁止商业生产；本 PRD 范围明确"不考虑长期服务"，仅做课设演示 |

---

## Dependencies & Blockers

**Dependencies（依赖）**：
- 域名 `rithupc.cn` 已 ICP 备案 ✅（用户已确认）
- DNS A 记录已指向 121.41.74.238 ✅（dig 验证通过）
- 微信小程序 AppID `wxa782f10bddd49b38` 已注册 ✅
- 阿里云账号已实名认证（个人）✅（可申请免费证书）
- 阿里云 ECS 安全组可放行 80/443（需手动配置）
- 微信公众平台账号（运营者）有"服务器域名"配置权限
- 阿里云账号（个人实名认证过）可申请免费证书

**Known Blockers（已知阻塞）**：
- ⚠️ **阿里云安全组**：当前只放行了 50000/9200/9000-9001/5672，需追加 80/443
- ⚠️ **DNS 验证**：首次申请免费证书时需添加一条 `_dnsauth.rithupc.cn` TXT 记录
- ⚠️ **免费证书限制**：仅限个人测试；本 PRD 不考虑长期服务（证书 1 年到期后另行处理）
- ⚠️ **微信审核类目**：校园助手类需提供"运营者身份证 + 学校授权证明"（视具体审核员要求）

---

## Appendix

### Appendix A: 实施 Checklist（详细步骤）

#### A.1 准备 Nginx 配置

```bash
# 在 ECS 上创建 nginx 目录
ssh root@121.41.74.238 "mkdir -p /opt/campus/deployments/nginx/{conf,conf.d,html,logs,certs,acme}"
```

#### A.2 在阿里云 SSL 控制台申请免费个人测试证书

1. 登录 https://yundun.console.aliyun.com （或 https://www.alibabacloud.com 阿里云控制台）
2. 搜索"SSL 证书" → 进入"数字证书管理服务"控制台
3. 左侧菜单：SSL 证书 → 免费证书 → 立即购买
4. 选"个人测试证书"（0 元）→ 立即购买 → 数量 20（够用）
5. 回到 SSL 证书 → 免费证书 → 证书申请
6. 填写：
   - 域名：`rithupc.cn`
   - 验证方式：**DNS 验证**（自动验证）
7. 系统生成一条 DNS 记录，类似：
   - 主机记录：`_dnsauth.rithupc.cn`
   - 记录类型：TXT
   - 记录值：`2024xxxxxx...`
8. 到阿里云 DNS 解析控制台（或 DNSPod）添加该 TXT 记录
9. 等待 5-10 分钟，SSL 控制台自动签发
10. 签发后 → 下载 → 选 **Nginx** 格式 → 得到压缩包

#### A.3 上传证书到 ECS

```bash
# 解压后得到两个文件：
#   rithupc.cn.pem       (fullchain，包含证书链)
#   rithupc.cn.key       (private key)

# 上传到 ECS（假设本地有这些文件）
scp -i /tmp/campus.pem rithupc.cn.pem rithupc.cn.key \
  root@121.41.74.238:/opt/campus/deployments/nginx/certs/

# 验证证书内容
ssh root@121.41.74.238 \
  "openssl x509 -in /opt/campus/deployments/nginx/certs/rithupc.cn.pem -noout -subject -dates"
```

#### A.4 docker-compose.yml 添加 nginx 服务

```yaml
  nginx:
    image: nginx:1.25-alpine
    container_name: campus-nginx
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/conf/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/conf.d:/etc/nginx/conf.d:ro
      - ./nginx/certs:/etc/nginx/certs:ro
      - ./nginx/html:/usr/share/nginx/html:ro
      - ./nginx/acme:/var/www/acme:ro
    networks:
      - campus-net
    depends_on:
      gateway:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "https://localhost/health || exit 1"]
      # 注：healthcheck 用 HTTPS 自检，需要证书已 mount
    deploy:
      resources:
        limits:
          cpus: '0.2'
          memory: 128M
```

#### A.5 Nginx 主配置

```nginx
# /opt/campus/deployments/nginx/conf/nginx.conf
user nginx;
worker_processes auto;
worker_rlimit_nofile 65535;
events { worker_connections 4096; }
http {
  include /etc/nginx/mime.types;
  sendfile on;
  tcp_nopush on;
  keepalive_timeout 65;
  client_max_body_size 10M;  # 文件上传 5MB + 余量

  # SSL 配置
  ssl_protocols TLSv1.2 TLSv1.3;
  ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
  ssl_prefer_server_ciphers off;
  ssl_session_cache shared:SSL:10m;
  ssl_session_timeout 1d;
  ssl_session_tickets off;
  ssl_stapling on;
  ssl_stapling_verify on;

  # 安全头
  add_header Strict-Transport-Security "max-age=31536000" always;
  add_header X-Frame-Options "SAMEORIGIN" always;
  add_header X-Content-Type-Options "nosniff" always;
  add_header Referrer-Policy "strict-origin-when-cross-origin" always;

  # 微信校验文件
  location /.well-known/ {
    root /usr/share/nginx/html;
  }

  # HTTP → HTTPS 强制跳转
  server {
    listen 80;
    server_name rithupc.cn;
    return 301 https://$host$request_uri;
  }

  # HTTPS 主服务
  server {
    listen 443 ssl http2;
    server_name rithupc.cn;

    ssl_certificate /etc/nginx/certs/fullchain.crt;
    ssl_certificate_key /etc/nginx/certs/rithupc.cn.key;

    # 上游（gateway）
    location / {
      proxy_pass http://campus-gateway:50000;
      proxy_set_header Host $host;
      proxy_set_header X-Real-IP $remote_addr;
      proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
      proxy_set_header X-Forwarded-Proto $scheme;
      proxy_http_version 1.1;
      proxy_set_header Connection "";
      proxy_read_timeout 60s;
    }
  }
}
```

#### A.6 微信公众平台配置

1. 登录 https://mp.weixin.qq.com
2. 开发管理 → 服务器域名
3. 填入：
   - request 合法域名：`https://rithupc.cn`
   - uploadFile 合法域名：`https://rithupc.cn`
   - downloadFile 合法域名：`https://rithupc.cn`
4. 下载微信校验文件，放到 `/opt/campus/deployments/nginx/html/.well-known/`
5. Nginx reload：`docker exec campus-nginx nginx -s reload`
6. 微信后台点击"保存"触发校验

#### A.7 小程序 baseURL 修改

```javascript
// scripts/miniapp/utils/constants.js
const BASE_URL = 'https://rithupc.cn/api/v1'
```

```json
// scripts/miniapp/project.config.json - 删除 urlCheck 字段
{
  "appid": "wxa782f10bddd49b38",
  "setting": {
    "babelSetting": { ... }
  }
}
```

### Appendix B: verify-https.sh

```bash
#!/bin/bash
# verify-https.sh — HTTPS 上线验证
# 用法: bash scripts/verify-https.sh

set -e
DOMAIN="rithupc.cn"

echo "=== 1. DNS 解析 ==="
IP=$(dig +short $DOMAIN | head -1)
[ "$IP" = "121.41.74.238" ] && echo "✅ DNS: $DOMAIN → $IP" || { echo "❌ DNS 错误: $IP"; exit 1; }

echo "=== 2. HTTPS 健康检查 ==="
HTTP_CODE=$(curl -sk -o /dev/null -w "%{http_code}" https://$DOMAIN/health)
[ "$HTTP_CODE" = "200" ] && echo "✅ HTTPS /health: 200" || { echo "❌ /health: $HTTP_CODE"; exit 1; }

echo "=== 3. HTTP 跳转 ==="
HTTP_REDIRECT=$(curl -s -o /dev/null -w "%{http_code}" http://$DOMAIN/health)
[ "$HTTP_REDIRECT" = "301" ] && echo "✅ HTTP → HTTPS 301" || { echo "❌ HTTP 跳转: $HTTP_REDIRECT"; exit 1; }

echo "=== 4. 证书有效期 ==="
DAYS=$(echo | openssl s_client -servername $DOMAIN -connect $DOMAIN:443 2>/dev/null | openssl x509 -noout -enddate | cut -d= -f2)
echo "✅ 证书到期: $DAYS"

echo "=== 5. SSL 协议 ==="
PROTO=$(echo | openssl s_client -servername $DOMAIN -connect $DOMAIN:443 2>/dev/null | grep "Protocol" | awk '{print $NF}')
[ "$PROTO" = "TLSv1.3" ] || [ "$PROTO" = "TLSv1.2" ] && echo "✅ 协议: $PROTO" || { echo "❌ 协议: $PROTO"; exit 1; }

echo "=== 6. 业务接口 ==="
for path in "/api/v1/content/posts?school_id=1" "/health"; do
  CODE=$(curl -sk -o /dev/null -w "%{http_code}" "https://$DOMAIN$path")
  echo "  $path: $CODE"
done

echo ""
echo "✅ HTTPS 验证全部通过"
```

### Appendix C: Glossary

- **ICP 备案**：工信部要求的网站/域名备案，中国大陆境内网站必须
- **免费个人测试证书**：阿里云提供的 0 元证书，1 年有效，**仅限个人测试**（不可商业）
- **DNS 验证**：CA 颁发机构通过 DNS TXT 记录验证域名所有权
- **fullchain.pem**：包含完整证书链（域名证书 + 中间证书）的 PEM 文件
- **privkey.pem**：证书对应的私钥
- **Nginx 反向代理 (Reverse Proxy)**：Nginx 接收外部请求，转发到内部服务
- **TLS 终止 (TLS Termination)**：在反向代理（Nginx）上解密 HTTPS，后端用 HTTP
- **HSTS**：HTTP Strict Transport Security，强制浏览器使用 HTTPS
- **wx.login()**：微信小程序 API，获取用户登录凭证（js_code）
- **js_code**：微信登录凭证，5 分钟有效，需后端用 appid+secret 换 openid+session_key

### Appendix D: References
- 微信小程序生产环境要求：https://developers.weixin.qq.com/miniprogram/dev/framework/ability/network.html
- 阿里云 SSL 证书（免费个人测试）：https://yundun.console.aliyun.com
- Nginx 反向代理：https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/
- 阿里云 ICP 备案：https://beian.aliyun.com/
- 前置 PRD：`docs/cloud-deployment-prd.md`（v1.1 基础设施）
- 相关 PRD：`docs/ai-moderation-content-service-v3.0-prd.md`（v3.0 AI 审核）

---

*This PRD was created through interactive requirements gathering with quality scoring to ensure comprehensive coverage of HTTPS 化、微信平台对接、上线路径三方面的完整可执行性。*