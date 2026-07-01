# ============================================================
# Server Monitor - 多阶段构建
# 阶段1: builder  —— 用 Go 工具链编译静态二进制
# 阶段2: runtime —— 极简 alpine 运行镜像
# ============================================================

# ---------- 阶段 1：构建 ----------
FROM golang:1.25-alpine AS builder

# alpine 缺少 ca-certificates，HTTP 健康检查访问 HTTPS 目标时需要
RUN apk add --no-cache ca-certificates

WORKDIR /src

# 先单独拷贝依赖清单，利用 Docker 层缓存（改代码不重新下载依赖）
COPY go.mod go.sum ./
RUN go mod download

# 再拷贝源码与静态资源
COPY . .

# 静态编译：
#   CGO_ENABLED=0  —— modernc.org/sqlite 是纯 Go，无需 CGO
#   -ldflags="-s -w" —— 去除调试信息，缩小体积
#   GOOS=linux      —— 目标平台
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -ldflags="-s -w" -o /out/server_monitor .

# ---------- 阶段 2：运行 ----------
FROM alpine:latest

# 运行时也需要 ca-certificates（HTTPS 健康检查）+ tzdata（时区）
# 安装 iputils 是为了容器内 ICMP ping 备用（程序内部已实现，但保留备用）
RUN apk add --no-cache ca-certificates tzdata

# 容器内以非 root 用户运行（更安全）
RUN adduser -D -u 1001 app
USER app
WORKDIR /app

# 从构建阶段拷贝二进制
COPY --from=builder /out/server_monitor /app/server_monitor
# 静态前端资源
COPY --from=builder /src/static /app/static

# 数据库持久化目录（通过 volume 挂载）
RUN mkdir -p /app/data
VOLUME /app/data

# 环境变量默认值（可被 compose/运行参数覆盖）
ENV ADDR=:8080 \
    DB_PATH=/app/data/monitor.db \
    STATIC_DIR=/app/static

EXPOSE 8080

# 健康检查：容器自检（访问自身首页）
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/ >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/server_monitor"]
