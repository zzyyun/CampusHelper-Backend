# ai-moderation 服务 Dockerfile（含 ONNX Runtime cgo 依赖）
# 与其他微服务的关键差异：
#   - CGO_ENABLED=1（onnxruntime-go 需要 cgo）
#   - 运行时需要 libonnxruntime.so（dlopen 加载，文件名须为 onnxruntime.so）
#   - 使用 Debian slim 替代 Alpine，避免 musl 与 ONNX Runtime glibc 二进制不兼容
#   - 模型文件通过 volume 挂载（不在镜像内）

ARG BUILDER_IMAGE=golang:1.25-alpine
ARG BASE_IMAGE=debian:bookworm-slim
ARG ONNXRUNTIME_VERSION=1.17.3

# ===== 构建阶段 =====
FROM ${BUILDER_IMAGE} AS builder

ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=1

# 安装编译依赖（gcc + musl-dev for cgo）
RUN apk add --no-cache gcc musl-dev git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 构建 ai-moderation 服务（启用 onnx_enabled build tag）
RUN go build -tags onnx_enabled -ldflags="-s -w" -o /app/ai-moderation ./cmd/ai-moderation/

# ===== 运行阶段 =====
FROM ${BASE_IMAGE}

ARG ONNXRUNTIME_VERSION

# 安装运行时依赖 + 下载 ONNX Runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl libgomp1 \
    && curl -fsSL https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz \
       -o /tmp/ort.tgz \
    && tar -xzf /tmp/ort.tgz -C /tmp \
    && cp /tmp/onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /usr/lib/ \
    && ln -s /usr/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /usr/lib/onnxruntime.so \
    && rm -rf /tmp/ort.tgz /tmp/onnxruntime-linux-x64-${ONNXRUNTIME_VERSION} \
    && rm -rf /var/lib/apt/lists/*

# 从 builder 复制编译产物
COPY --from=builder /app/ai-moderation /usr/local/bin/ai-moderation

# 模型目录（运行时由 volume 挂载）
RUN mkdir -p /models

EXPOSE 50061 9091

ENTRYPOINT ["ai-moderation"]
