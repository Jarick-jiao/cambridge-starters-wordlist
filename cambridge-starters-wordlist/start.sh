#!/bin/bash
# 一键启动脚本
# 用法: ./start.sh [端口号]

PORT="${1:-8080}"
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

# 检查 Go
if command -v go &>/dev/null; then
    GO_CMD="go"
elif [ -x "/usr/local/go/bin/go" ]; then
    GO_CMD="/usr/local/go/bin/go"
elif [ -x "$HOME/sdk/go1.25.1/bin/go" ]; then
    GO_CMD="$HOME/sdk/go1.25.1/bin/go"
else
    echo "错误: 未找到 Go，请先安装 Go 1.25+"
    echo "  下载: https://go.dev/dl/"
    exit 1
fi

echo "使用 Go: $($GO_CMD version)"
echo ""

# 确保 mcp-server-sqlite 可用
if ! command -v npx &>/dev/null; then
    echo "错误: 未找到 npx，请先安装 Node.js (https://nodejs.org/)"
    exit 1
fi

# 首次运行需要下载依赖
if [ ! -f "go.sum" ] || [ ! -d "vendor" ]; then
    echo "拉取 Go 依赖..."
    $GO_CMD mod tidy
fi

# 编译
echo "编译中..."
$GO_CMD build -o starters-server . || { echo "编译失败"; exit 1; }

# 启动
echo "启动服务 (端口: $PORT)..."
echo "访问: http://localhost:$PORT"
echo ""
PORT="$PORT" ./starters-server