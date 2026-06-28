# gateway 服务多阶段 Dockerfile
# 基础镜像: golang:1.25-alpine (构建) + distroless/static (运行时)

# 全局 build-arg（必须在 FROM 之前声明才能跨 stage 使用）
ARG BUILDER_IMAGE=golang:1.25-alpine
ARG BASE_IMAGE=gcr.io/distroless/static-debian12:nonroot
ARG RUNTIME_USER=nonroot:nonroot

# ===== 构建阶段 =====
FROM ${BUILDER_IMAGE} AS builder

# 国内加速（CI 环境按需启用）
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /src

# 优先复制依赖文件以利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# 复制源码
COPY . .

# 静态链接 + 体积裁剪
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -buildid=" \
    -o /out/app \
    ./cmd/gateway

# ===== 运行时阶段 =====
FROM ${BASE_IMAGE}

# alpine 基础镜像需要 ca-certificates（distroless 内置 TLS 证书）
RUN if command -v apk >/dev/null 2>&1; then \
      apk add --no-cache ca-certificates netcat-openbsd; \
    fi

COPY --from=builder /out/app /app

# 端口 50000
EXPOSE 50000

# 服务名注入
ENV SERVICE_NAME=gateway

USER ${RUNTIME_USER}

ENTRYPOINT ["/app"]
