# verify.sh - 端到端验证脚本

CampusHelper 微服务部署后的健康检查与主链路业务验证脚本。

## 快速使用

```bash
# 在 ECS 上执行
cd /opt/campus/campus-helper
./scripts/verify.sh

# 自定义 gateway 地址
GATEWAY_URL=http://1.2.3.4:50000 ./scripts/verify.sh

# 仅做健康检查（跳过业务调用）
./scripts/verify.sh --skip-biz

# 跳过数据库检查（避免泄露密码）
./scripts/verify.sh --skip-db
```

## 检查阶段

| 阶段 | 内容 | 失败时建议 |
|------|------|-----------|
| 1 | 容器运行状态 (docker ps) | `docker compose -f deployments/docker/campus-docker-compose.yaml ps` |
| 2 | 端口连通性 (TCP) | `nc -zv localhost 50000` |
| 3 | HTTP 健康端点 | `curl http://localhost:50000/health` |
| 4 | 云数据库连通性 | `mysql -h<host> -u<user> -p` |
| 5 | 主链路业务 | 需 token，参考 API 文档 |

## 主链路流程

1. **微信登录** → `/api/v1/user/login` (POST code)
2. **获取 token** → 提取 access_token
3. **创建帖子** → `/api/v1/content/posts` (POST)
4. **读取帖子** → `/api/v1/content/posts/{id}` (GET)
5. **发送消息** → `/api/v1/message/send` (POST)
6. **上传头像** → `/api/v1/file/upload` (multipart)

## 退出码

- `0` - 全部通过
- `1` - 有失败项（查看输出最后几行的"故障排查"）
- `2` - 参数错误

## 依赖

- `bash` 4+
- `curl`
- `docker` (可选，阶段 1)
- `jq` (阶段 5)
- `mysql-client` (可选，阶段 4)
- `redis-cli` (可选，阶段 4)

## 颜色输出

- 绿色 ✓ PASS
- 红色 ✗ FAIL
- 黄色 ⚠ WARN（不计入失败）
- 青色 标题

## 集成到 CI

```bash
# GitHub Actions
- name: Verify deployment
  run: ./scripts/verify.sh --skip-biz
  env:
    GATEWAY_URL: ${{ secrets.ECS_GATEWAY_URL }}
```
