#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
START_GATEWAY="${START_GATEWAY:-1}"
GATEWAY_PID=""

cleanup() {
  if [[ -n "${GATEWAY_PID}" ]]; then
    kill "${GATEWAY_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd python3
require_cmd curl

if ! python3 - <<'PY' >/dev/null 2>&1
import requests
PY
then
  echo "Missing Python dependency: requests" >&2
  echo "Install it with: python3 -m pip install requests" >&2
  exit 1
fi

echo "=========================================="
echo "  Sui-Nexus Demo: Multi-Agent Trading"
echo "=========================================="
echo ""

if [[ "${START_GATEWAY}" == "1" ]]; then
  require_cmd go
  echo "Step 1: Starting Gateway (in background)..."
  go run cmd/gateway/main.go &
  GATEWAY_PID=$!
  sleep 3
else
  echo "Step 1: Using existing Gateway at ${GATEWAY_URL}"
fi

echo "Checking Gateway readiness..."
HEALTH_BODY="$(curl -sS "${GATEWAY_URL}/health" || true)"
if [[ -z "${HEALTH_BODY}" ]]; then
  echo "Gateway health check failed. Is the service running at ${GATEWAY_URL}?" >&2
  exit 1
fi
echo "${HEALTH_BODY}"
if ! echo "${HEALTH_BODY}" | python3 -c 'import json,sys; sys.exit(0 if json.load(sys.stdin).get("ready") else 1)'; then
  echo "Gateway is not ready. Kafka must be configured before /api/v1/intent will accept demo tasks." >&2
  exit 1
fi

echo ""
echo "Step 2: Running Analyst Agent..."
python3 scripts/demo/analyst_agent.py << 'EOF'
Protocol X suffered a flash loan attack, token price dropped 40%
EOF

echo ""
echo "Step 3: Running Trader Agent..."
python3 scripts/demo/trader_agent.py << 'EOF'
{"sentiment": "bearish", "confidence": 0.85, "action": "sell", "target_tokens": ["SUI", "USDT"]}
EOF

echo ""
echo "=========================================="
echo "Demo completed. Query /api/v1/task/<task_id> for status and Sui digest."
echo "=========================================="
