#!/bin/bash
# 一键构建 6 个微服务镜像
# 用法:
#   ./build/docker/build.sh           # 构建所有服务
#   ./build/docker/build.sh user      # 构建指定服务
#   ./build/docker/build.sh user v1.0 # 构建并打 tag

set -e

REGISTRY="${ACR_REGISTRY:-campus}"
TAG="${2:-dev}"
SERVICE="${1:-all}"

SERVICES=(gateway user content task message file)

build_one() {
  local svc=$1
  local tag=$2
  echo "==> Building $svc:$tag ..."
  docker buildx build \
    --platform linux/amd64 \
    -f "build/docker/${svc}.Dockerfile" \
    -t "${REGISTRY}/${svc}:${tag}" \
    --load \
    .
  echo "✓ ${REGISTRY}/${svc}:${tag}"
}

if [ "$SERVICE" = "all" ]; then
  for svc in "${SERVICES[@]}"; do
    build_one "$svc" "$TAG"
  done
else
  if [[ " ${SERVICES[*]} " =~ " $SERVICE " ]]; then
    build_one "$SERVICE" "$TAG"
  else
    echo "ERROR: Unknown service '$SERVICE'. Valid: ${SERVICES[*]}"
    exit 1
  fi
fi

echo ""
echo "=== 镜像列表 ==="
docker images "${REGISTRY}/*" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
