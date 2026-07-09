#!/usr/bin/env bash
# CampusHelper-Backend ECS 部署脚本（基于 docker-compose）
# 用法：bash deploy-ecs.sh [服务名|all]
# 示例：bash deploy-ecs.sh gateway
#       bash deploy-ecs.sh all

set -euo pipefail

SERVICE_INPUT="${1:-all}"
COMPOSE_FILE="/opt/campus/docker-compose.yaml"
ALL_SERVICES="gateway user content task message file"

log()  { echo "[$(date '+%H:%M:%S')] $*"; }
fail() { echo "[$(date '+%H:%M:%S')] ❌ $*" >&2; exit 1; }

log "═══ CampusHelper-Backend 部署 ═══"
log "目标服务: $SERVICE_INPUT"
echo ""

if [ ! -f "$COMPOSE_FILE" ]; then
  fail "docker-compose 文件不存在: $COMPOSE_FILE"
fi

cd "$(dirname "$COMPOSE_FILE")"

if [ "$SERVICE_INPUT" = "all" ]; then
  log "拉取所有服务镜像..."
  docker compose -f "$COMPOSE_FILE" pull
  log "重启所有服务..."
  docker compose -f "$COMPOSE_FILE" up -d --force-recreate
else
  if [[ ! " $ALL_SERVICES " =~ " $SERVICE_INPUT " ]]; then
    fail "未知服务: $SERVICE_INPUT（可选：$ALL_SERVICES）"
  fi
  log "拉取 $SERVICE_INPUT 镜像..."
  docker compose -f "$COMPOSE_FILE" pull "$SERVICE_INPUT"
  log "重启 $SERVICE_INPUT..."
  docker compose -f "$COMPOSE_FILE" up -d --force-recreate "$SERVICE_INPUT"
fi

log "等待服务启动（15s）..."
sleep 15

echo ""
log "═══ 部署完成，验证服务状态 ═══"
docker compose -f "$COMPOSE_FILE" ps
log "✅ 部署完成"
