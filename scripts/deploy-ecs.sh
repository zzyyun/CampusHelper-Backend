#!/usr/bin/env bash
# CampusHelper-Backend ECS 部署脚本（基于 docker-compose）
# 用法：bash deploy-ecs.sh [服务名|all]
# 示例：bash deploy-ecs.sh gateway
#       bash deploy-ecs.sh all

set -euo pipefail

SERVICE_INPUT="${1:-all}"
COMPOSE_FILE="/opt/campus/campus-docker-compose.yaml"
ALL_SERVICES="gateway user content task message file"

# docker-compose 中定义的所有容器名
ALL_CONTAINERS="campus-etcd campus-rabbitmq campus-minio campus-es campus-gateway campus-user campus-content campus-task campus-message campus-file campus-nginx campus-ai-moderation"

log()  { echo "[$(date '+%H:%M:%S')] $*"; }
fail() { echo "[$(date '+%H:%M:%S')] ERROR $*" >&2; exit 1; }

# 加载 .env 环境变量（含 ACR_REGISTRY），供 docker compose 展开 ${ACR_REGISTRY} 使用
if [ -f /opt/campus/.env ]; then
  set -a
  # shellcheck disable=SC1091
  source /opt/campus/.env
  set +a
fi
# 注入 CONFIG_DIR（docker-compose 卷绑定变量插值用）
grep -q '^CONFIG_DIR=' /opt/campus/.env 2>/dev/null || echo "CONFIG_DIR=./config" >> /opt/campus/.env

if [ -z "${ACR_REGISTRY:-}" ]; then
  fail "ACR_REGISTRY 环境变量未设置，请在 /opt/campus/.env 中配置"
fi

log "=== CampusHelper-Backend 部署 ==="
log "目标服务: $SERVICE_INPUT"
echo ""

if [ ! -f "$COMPOSE_FILE" ]; then
  fail "docker-compose 文件不存在: $COMPOSE_FILE"
fi

cd "$(dirname "$COMPOSE_FILE")"

if [ "$SERVICE_INPUT" = "all" ]; then
  log "拉取所有服务镜像..."
  docker compose -f "$COMPOSE_FILE" pull
  log "停止并清理旧容器..."
  # docker compose down 只清理当前 project 的容器，跨 project 的旧容器需手动删除
  docker compose -f "$COMPOSE_FILE" down --remove-orphans 2>/dev/null || true
  for cname in $ALL_CONTAINERS; do
    docker rm -f "$cname" 2>/dev/null || true
  done
  log "重启所有服务..."
  docker compose -f "$COMPOSE_FILE" up -d
else
  if [[ ! " $ALL_SERVICES " =~ " $SERVICE_INPUT " ]]; then
    fail "未知服务: $SERVICE_INPUT（可选：$ALL_SERVICES）"
  fi
  # 单个服务部署：先强制删除旧容器避免名称冲突，再拉取并重启
  log "拉取 $SERVICE_INPUT 镜像..."
  docker compose -f "$COMPOSE_FILE" pull "$SERVICE_INPUT"
  log "强制删除旧容器（如有）..."
  docker rm -f "campus-${SERVICE_INPUT}" 2>/dev/null || true
  log "重启 $SERVICE_INPUT..."
  docker compose -f "$COMPOSE_FILE" up -d "$SERVICE_INPUT"
fi

log "等待服务启动（15s）..."
sleep 15

echo ""
log "=== 部署完成，验证服务状态 ==="
docker compose -f "$COMPOSE_FILE" ps

# 诊断：输出 gateway 和 ES 容器日志（排查健康检查失败）
for cname in campus-gateway campus-es; do
  echo ""
  log "=== $cname 日志（最近 30 行） ==="
  docker logs "$cname" --tail 30 2>&1 || true
done
log "部署完成"