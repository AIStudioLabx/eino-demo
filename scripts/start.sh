#!/usr/bin/env bash
# 在项目根目录启动全部服务：MCP Server、Agent Server、Frontend Server
# 用法：./scripts/start.sh  或从项目根执行 bash scripts/start.sh
# 使用 ComfyUI 相关 MCP 工具前，请设置环境变量：export RUNNINGHUB_API_KEY=你的APIKey

set -e
cd "$(dirname "$0")/.."
ROOT="$PWD"

PIDS_FILE="${ROOT}/.start_pids"
LOG_DIR="${ROOT}/logs"
mkdir -p "$LOG_DIR"

cleanup() {
  echo ""
  echo "正在停止所有服务..."
  if [[ -f "$PIDS_FILE" ]]; then
    while read -r pid; do
      [[ -z "$pid" ]] && continue
      kill "$pid" 2>/dev/null || true
    done < "$PIDS_FILE"
    rm -f "$PIDS_FILE"
  fi
  echo "已停止。"
  exit 0
}

trap cleanup SIGINT SIGTERM EXIT

echo "工作目录: $ROOT"
echo "日志目录: $LOG_DIR"
echo ""

: > "$PIDS_FILE"

# MCP Server :3333
echo "启动 MCP Server ( :3333 )..."
go run ./backend/mcp_server > "$LOG_DIR/mcp_server.log" 2>&1 &
echo $! >> "$PIDS_FILE"
sleep 1

# Agent Server :8082（依赖 MCP）
echo "启动 Agent Server ( :8082 )..."
go run ./backend/agent_server > "$LOG_DIR/agent_server.log" 2>&1 &
echo $! >> "$PIDS_FILE"
sleep 0.5

# Frontend Server :8081（需在项目根运行以正确提供 frontend/）
echo "启动 Frontend Server ( :8081 )..."
( cd "$ROOT" && go run ./backend/frontend_server > "$LOG_DIR/frontend_server.log" 2>&1 ) &
echo $! >> "$PIDS_FILE"

echo ""
echo "全部服务已启动。"
echo "  - MCP Server:     http://localhost:3333"
echo "  - Agent Server:  http://localhost:8082"
echo "  - Frontend:      http://localhost:8081"
echo ""
echo "日志: $LOG_DIR/*.log"
echo "按 Ctrl+C 停止所有服务。"
echo ""

wait
