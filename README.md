# Server Monitor

一个轻量级的服务监控面板，支持 TCP 端口、HTTP/HTTPS、ICMP Ping 三种检查方式，
可在浏览器直观查看各个服务的运行状态、可用率和响应时间趋势。

## 技术栈

- **后端**：Go（标准库 `net/http`）
- **存储**：SQLite（纯 Go 驱动 `modernc.org/sqlite`，无需 CGO）
- **前端**：原生 HTML/JS 单页 + Chart.js（CDN）

## 功能

- 添加 / 编辑 / 删除监控服务
- 三种检查类型：
  - **TCP 端口** — 检测能否建立 TCP 连接（覆盖 MySQL / Redis / SSH 等）
  - **HTTP/HTTPS** — 发起请求并按状态码（2xx/3xx）判断存活
  - **ICMP Ping** — 检测主机可达性与延迟
- 每个服务显示最近 30 次检查的状态点条 + 可用率（Uptime %）
- 点击「历史趋势」查看近 24 小时响应时间折线图
- 后端定时检查（按每个服务自定义间隔）+ 前端每 5 秒轮询

## 快速开始

### 方式一：Docker（推荐）

```bash
# 用 docker compose 一键启动（自动构建镜像）
docker compose up -d --build

# 查看日志
docker compose logs -f

# 停止 / 删除
docker compose down
```

打开 http://localhost:8080 即可。监控数据持久化在 docker volume `monitor-data` 中，
`docker compose down` 不会丢数据，`docker compose down -v` 才会清除。

自定义宿主机端口（容器内固定 8080）：

```bash
HOST_PORT=9090 docker compose up -d --build
# 然后访问 http://localhost:9090
```

单独用 docker（不使用 compose）：

```bash
docker build -t server-monitor .
docker run -d --name server-monitor \
  -p 8080:8080 \
  -v monitor-data:/app/data \
  --cap-add=NET_RAW \
  --restart unless-stopped \
  server-monitor
```

### 方式二：本地运行

```bash
# 构建
go build -o server_monitor .

# 运行（默认监听 :8080，数据库 data.db）
./server_monitor

# 自定义参数
./server_monitor --addr :9090 --db /var/lib/server_monitor.db
```

打开浏览器访问 http://localhost:8080 ，点击「+ 添加服务」即可。

## 配置

可通过命令行参数或环境变量配置（环境变量优先级低于命令行参数）：

| 参数 | 环境变量 | 默认值 | 说明 |
| ---- | -------- | ------ | ---- |
| `--addr` | `ADDR` | `:8080` | HTTP 监听地址 |
| `--db` | `DB_PATH` | `data.db` | SQLite 数据库路径 |
| `--static` | `STATIC_DIR` | `static` | 静态前端资源目录 |

## ICMP 权限说明

ICMP 需要 raw socket 权限（Linux/Mac 上通常需 root）。本程序会：

1. 启动时探测是否有 ICMP 权限；
2. **无权限时自动回退**为对目标主机常用端口（80/443/22/8080）的 TCP 握手，
   近似判断主机可达性，保证程序不会因权限问题报错。

如需启用真正的 ICMP，可赋予 capability（Linux）：

```bash
sudo setcap cap_net_raw=+ep ./server_monitor
```

Docker 部署时，`docker-compose.yml` 已通过 `cap_add: NET_RAW` 授予该权限；单独
`docker run` 时请加 `--cap-add=NET_RAW`。

## REST API

| 方法   | 路径                          | 说明                       |
| ------ | ----------------------------- | -------------------------- |
| GET    | `/api/services`               | 列出所有服务 + 最近状态    |
| POST   | `/api/services`               | 添加服务                   |
| PUT    | `/api/services/{id}`          | 编辑服务                   |
| DELETE | `/api/services/{id}`          | 删除服务                   |
| POST   | `/api/services/{id}/check`    | 立即触发一次检查           |
| GET    | `/api/services/{id}/history`  | 历史趋势（`?hours=24`）    |

## 项目结构

```
server_monitor/
├── main.go          # 启动、HTTP server、静态文件服务
├── store.go         # SQLite 初始化 + CRUD + 历史查询
├── checker.go       # TCP/HTTP/ICMP 检查器 + 定时调度器
├── api.go           # REST handler
├── static/
│   └── index.html   # 单页前端 UI
├── go.mod / go.sum
└── README.md
```
