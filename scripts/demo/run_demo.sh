#!/bin/bash
set -e

echo "=========================================="
echo "  Sui-Nexus Demo: Multi-Agent Trading"
echo "=========================================="
echo ""

echo "Step 1: Starting Gateway (in background)..."
go run cmd/gateway/main.go &
GATEWAY_PID=$!
sleep 3

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
echo "Demo completed. Check Sui Explorer for TX."
echo "=========================================="

kill $GATEWAY_PID 2>/dev/null || true