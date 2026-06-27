# gateway 服务多阶段 Dockerfile
# 基础镜像: golang:1.25-alpine (构建) + distroless/static (运行时)

# ===== 构建阶段 =====
FROM golang:1.25-alpine AS builder

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
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/app /app

# 端口 50000
EXPOSE 50000

# 服务名注入
ENV SERVICE_NAME=gateway

USER nonroot:nonroot

ENTRYPOINT ["/app"]
