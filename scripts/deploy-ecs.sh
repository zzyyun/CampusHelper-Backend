#!/usr/bin/env bash
# CampusHelper-Backend ECS 部署脚本
# 用法：bash deploy-ecs.sh [服务名|all] [镜像tag后缀]
# 示例：bash deploy-ecs.sh gateway abc1234
#       bash deploy-ecs.sh all abc1234

set -euo pipefail

# ── 配置 ──────────────────────────────────────────────────────────────────────
# 从环境变量或 secrets 注入，以下为默认值
ACR_REGISTRY="${ACR_REGISTRY:-crpi-xxx.cn-hangzhou.personal.cr.aliyuncs.com/campus_sends/campus}"
IMAGE_TAG="${2:-latest}"
SERVICE_INPUT="${1:-all}"

# 服务定义：服务名 → 端口 → 额外 docker run 参数
declare -A SERVICE_CONFIG=(
  ["gateway"]="50000:50000 --network host"
  ["user"]="50001:50001 --network host"
  ["content"]="50002:50002 --network host"
  ["task"]="50003:50003 --network host"
  ["message"]="50004:50004 --network host"
  ["file"]="50005:50005 --network host"
)

ALL_SERVICES="gateway user content task message file"

# ── 函数 ──────────────────────────────────────────────────────────────────────

log()  { echo "[$(date '+%H:%M:%S')] $*"; }
fail() { echo "[$(date '+%H:%M:%S')] ❌ $*" >&2; exit 1; }

# 健康检查：最多等 30 秒
health_check() {
  local service=$1
  local port
  port=$(echo "${SERVICE_CONFIG[$service]}" | cut -d: -f1)

  log "  等待 $service 健康检查（端口 $port）..."
  for i in $(seq 1 15); do
    if curl -sf "http://localhost:$port/health" > /dev/null 2>&1; then
      log "  ✅ $service 健康检查通过"
      return 0
    fi
    sleep 2
  done
  log "  ⚠️  $service 健康检查超时（30s），继续部署"
  return 1
}

# 部署单个服务
deploy_service() {
  local service=$1
  local image="${ACR_REGISTRY}:v1.0-${service}-${IMAGE_TAG}"
  local container_name="campus-${service}"

  log "部署 $service ..."
  log "  镜像: $image"

  # 1. 拉取新镜像
  if ! docker pull "$image" 2>/dev/null; then
    fail "拉取镜像失败: $image"
  fi

  # 2. 停止并移除旧容器（保留数据卷）
  if docker ps -a --format '{{.Names}}' | grep -q "^${container_name}$"; then
    log "  停止旧容器 $container_name ..."
    docker stop "$container_name" --time 10 2>/dev/null || true
    docker rm "$container_name" 2>/dev/null || true
  fi

  # 3. 启动新容器
  local config="${SERVICE_CONFIG[$service]}"
  local ports
  ports=$(echo "$config" | cut -d' ' -f1)
  local extra_args
  extra_args=$(echo "$config" | cut -d' ' -f2-)

  # 读取环境变量文件（如果存在）
  local env_file=""
  if [ -f "/opt/campus-helper/config/${service}.env" ]; then
    env_file="--env-file /opt/campus-helper/config/${service}.env"
  fi

  # shellcheck disable=SC2086
  docker run -d \
    --name "$container_name" \
    --restart unless-stopped \
    -p "$ports" \
    $env_file \
    $extra_args \
    "$image"

  log "  容器 $container_name 已启动"

  # 4. 健康检查
  health_check "$service"
}

# ── 主流程 ────────────────────────────────────────────────────────────────────

log "═══ CampusHelper-Backend 部署 ═══"
log "镜像 tag: $IMAGE_TAG"
log "目标服务: $SERVICE_INPUT"
echo ""

if [ "$SERVICE_INPUT" = "all" ]; then
  for svc in $ALL_SERVICES; do
    deploy_service "$svc"
    echo ""
  done
else
  if [ -z "${SERVICE_CONFIG[$SERVICE_INPUT]+x}" ]; then
    fail "未知服务: $SERVICE_INPUT（可选：$ALL_SERVICES）"
  fi
  deploy_service "$SERVICE_INPUT"
fi

# 部署验证
echo ""
log "═══ 部署完成，验证服务状态 ═══"
docker ps --filter "name=campus-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
log "✅ 部署完成"
