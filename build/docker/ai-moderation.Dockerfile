# ai-moderation 服务 Dockerfile（含 ONNX Runtime cgo 依赖）
# 与其他微服务的关键差异：
#   - CGO_ENABLED=1（onnxruntime-go 需要 cgo）
#   - 运行时需要 libonnxruntime.so
#   - 模型文件通过 volume 挂载（不在镜像内）

ARG BUILDER_IMAGE=golang:1.25-alpine
ARG BASE_IMAGE=alpine:3.19

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

# 安装 onnxruntime 运行时依赖
RUN apk add --no-cache libstdc++ libgcc

# 从 builder 复制 onnxruntime 动态库（如果存在）
COPY --from=builder /usr/lib/libonnxruntime* /usr/lib/ 2>/dev/null || true

# 从 builder 复制编译产物
COPY --from=builder /app/ai-moderation /usr/local/bin/ai-moderation

# 模型目录（运行时由 volume 挂载）
RUN mkdir -p /models

EXPOSE 50061 9091

ENTRYPOINT ["ai-moderation"]
