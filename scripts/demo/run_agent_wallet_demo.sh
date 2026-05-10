#!/usr/bin/env bash
set -euo pipefail
# Agent Wallet with zkLogin — Agentic Web Track Demo Launcher

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"

cleanup() { [ -n "${GATEWAY_PID:-}" ] && kill "${GATEWAY_PID}" 2>/dev/null || true; }
trap cleanup EXIT

command -v python3 >/dev/null 2>&1 || { echo "Missing: python3"; exit 1; }
python3 -c 'import requests' 2>/dev/null || { echo "pip install requests"; exit 1; }

if [ "${START_GATEWAY:-1}" = "1" ]; then
  command -v go >/dev/null 2>&1 || { echo "Missing: go"; exit 1; }
  echo "Starting Gateway..."
  go run cmd/gateway/main.go &
  GATEWAY_PID=$!
  sleep 3
fi

echo "Checking Gateway..."
curl -sS "${GATEWAY_URL}/health" || { echo "Gateway not reachable"; exit 1; }

echo ""
python3 scripts/demo/agent_wallet_demo.py
