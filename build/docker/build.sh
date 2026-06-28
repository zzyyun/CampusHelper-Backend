#!/bin/bash
# 一键构建 6 个微服务镜像
# 用法:
#   ./build/docker/build.sh                          # 构建所有服务（默认 distroless）
#   ./build/docker/build.sh user                     # 构建指定服务
#   ./build/docker/build.sh user v1.0                # 构建并打 tag
#   ./build/docker/build.sh user v1.0 alpine         # 用 alpine 基础镜像（gcr.io 不可达时使用）
#   ALPINE=1 ./build/docker/build.sh all             # 通过环境变量启用 alpine
#
# alpine 模式适用于 gcr.io 不可达的网络环境（如中国大陆）：
#   - BASE_IMAGE=alpine:3.19 替代 gcr.io/distroless/static-debian12:nonroot
#   - RUNTIME_USER=nobody 替代 nonroot:nonroot
# 镜像体积会大 ~5-8MB（alpine 基础层），但国内可稳定拉取。
# docker build --build-arg BASE_IMAGE=alpine:3.19 --build-arg RUNTIME_USER=nobody -f build/docker/gateway.Dockerfile -t campus/gateway:tag .

set -e

REGISTRY="${ACR_REGISTRY:-campus}"
SERVICE="${1:-all}"
TAG="${2:-dev}"
MODE="${3:-${ALPINE_MODE:-default}}"

SERVICES=(gateway user content task message file)

# 根据模式设置 build-arg
BUILD_ARGS=()
if [ "$MODE" = "alpine" ] || [ "${ALPINE:-0}" = "1" ]; then
  echo "==> Mode: alpine (gcr.io 替代方案)"
  BUILD_ARGS=(
    --build-arg "BASE_IMAGE=alpine:3.19"
    --build-arg "RUNTIME_USER=nobody"
  )
else
  echo "==> Mode: default (distroless, 海外/有 gcr.io 访问的环境)"
fi

build_one() {
  local svc=$1
  local tag=$2
  echo "==> Building $svc:$tag ..."
  docker buildx build \
    --platform linux/amd64 \
    -f "build/docker/${svc}.Dockerfile" \
    -t "${REGISTRY}/${svc}:${tag}" \
    --load \
    "${BUILD_ARGS[@]}" \
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
