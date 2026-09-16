#!/usr/bin/env bash
# ==============================================================================
# 我不是神 · 交易员工作台一键启动脚本 (Vivaldi / Arc / Chrome 自动加载插件与分屏看盘)
# ==============================================================================

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXT_PATH="${PROJECT_DIR}/app/extension"
LOCAL_URL="http://127.0.0.1:8765"
MULTICHART_URL="${LOCAL_URL}/multichart"
PORTFOLIO_URL="${LOCAL_URL}/portfolio"
TRADINGVIEW_URL="https://www.tradingview.com/chart/"

echo "🚀 [我不是神] 正在检查本地后端服务 (:8765)..."

# Check if backend is alive; otherwise build and start the managed current source.
if ! curl -s --connect-timeout 1 "${LOCAL_URL}/api/health" > /dev/null 2>&1; then
  echo "⚡ 本地服务未运行，正在自动启动 Go 后端..."
  "${PROJECT_DIR}/scripts/trading-cockpit-service.sh" start
fi

if ! curl -fsS --max-time 3 "${LOCAL_URL}/api/health" >/dev/null; then
  echo "🔴 本地服务未通过健康检查，不打开工作台。" >&2
  exit 1
fi
echo "🟢 本地服务存活: ${LOCAL_URL}（数据就绪状态请看设置页）"

# Choose browser (default to Vivaldi, fallback to Arc or Chrome)
if [ -d "/Applications/Vivaldi.app" ]; then
  echo "🌐 正在启动 Vivaldi 并自动加载【我不是神】AI 扩展插件..."
  open -n -a "/Applications/Vivaldi.app" --args \
    --load-extension="${EXT_PATH}" \
    "${MULTICHART_URL}" \
    "${TRADINGVIEW_URL}" \
    "${PORTFOLIO_URL}"
elif [ -d "/Applications/Arc.app" ]; then
  echo "⚡ 正在启动 Arc 浏览器..."
  open -a "/Applications/Arc.app" "${MULTICHART_URL}"
else
  echo "🌐 正在使用 Google Chrome 启动..."
  open -a "Google Chrome" "${MULTICHART_URL}"
fi

echo "✨ 交易工作台已全部启动完毕！"
