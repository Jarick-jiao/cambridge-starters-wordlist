# 樱桃学习 · 剑桥少儿英语平台 (Gin + MCP SQLite)

樱桃学习 —— 剑桥 Starters & Movers 单词与语法学习平台，含英语词汇/语法卡片、数学运算练习、任务管理、学习仪表盘。采用樱桃暖阳风视觉设计。

## 前置依赖

| 依赖 | 最低版本 | 安装 |
|------|---------|------|
| Go | 1.25+ | https://go.dev/dl/ |
| Node.js | 18+（用于 mcp-server-sqlite） | https://nodejs.org/ |

## 快速启动

### 方式一：一键脚本

```bash
cd cambridge-starters-wordlist
chmod +x start.sh
./start.sh          # 默认端口 8080
./start.sh 3000     # 自定义端口
```

### 方式二：手动编译运行

```bash
cd cambridge-starters-wordlist

# 拉取依赖
go mod tidy

# 编译
go build -o starters-server .

# 运行
./starters-server              # 默认 8080
PORT=3000 ./starters-server    # 自定义端口
```

### 方式三：直接运行（不编译）

```bash
go run .
```

## 访问

| 方式 | 地址 |
|------|------|
| 本机浏览器 | http://localhost:8080 |
| 局域网设备（手机/平板） | http://`你的电脑IP`:8080 |

查看本机 IP：
```bash
# macOS
ifconfig | grep "inet "
# Linux
ip addr | grep "inet "
```

## 停止服务

终端中按 `Ctrl + C`。

## 功能模块

- **英语学习**：剑桥 Starters 词汇卡片 + 语法点，点击发音（英式 en-GB），点击标记已掌握
- **数学练习**：加减乘除、3 级难度（个位/两位数/进退位）、即时对错反馈、计时器、错题列表
- **任务管理**：家长创建每日任务，支持进度跟踪和自动完成
- **学习仪表盘**：今日任务、连续天数、已掌握单词、数学正确率、各分类进度条
- **自动打卡**：数学/英语学习完成后自动更新对应任务进度

## 文件说明

| 文件 | 说明 |
|------|------|
| `main.go` | 入口，启动 MCP 客户端和 Gin 服务 |
| `mcp_client.go` | MCP JSON-RPC 客户端（与 mcp-server-sqlite 通信） |
| `db_ops.go` | 数据库操作层（单词进度/任务/数学/日志） |
| `handlers.go` | HTTP API 处理器 |
| `server.go` | Gin 路由和静态文件服务 |
| `cambridge-starters-wordlist.html` | 前端单页应用 |
| `start.sh` | 一键启动脚本 |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 服务监听端口 |
| `STATIC_DIR` | `.` | 静态文件目录 |