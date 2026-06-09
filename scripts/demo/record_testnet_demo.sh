#!/usr/bin/env bash
# scripts/demo/record_testnet_demo.sh
# Record real Sui testnet transactions for demo video
# Usage: HACKATHON_DEMO_MODE=false ./scripts/demo/record_testnet_demo.sh
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"

# ── Validate required environment ──────────────────────────────────────────
REQUIRED_VARS=(
  "SUI_SIGNER_MNEMONIC"
  "SUI_GAS_OBJECT_ID"
)

MISSING=""
for var in "${REQUIRED_VARS[@]}"; do
  if [ -z "${!var:-}" ]; then
    MISSING="${MISSING}  - ${var}\n"
  fi
done

if [ -n "${MISSING}" ]; then
  echo "╔════════════════════════════════════════════════════════════════════╗"
  echo "║  Missing required environment variables for testnet recording:   ║"
  echo -e "║  ${MISSING}║"
  echo "║                                                                    ║"
  echo "║  Setup steps:                                                      ║"
  echo "║  1. Create or fund a Sui testnet wallet:                           ║"
  echo "║       sui client active-addresses                                  ║"
  echo "║       sui client gas --assign <address>                           ║"
  echo "║                                                                    ║"
  echo "║  2. Export credentials:                                            ║"
  echo "║       export SUI_SIGNER_MNEMONIC='your 24-word mnemonic'           ║"
  echo "║       export SUI_GAS_OBJECT_ID='0x...'  # from 'sui client gas'    ║"
  echo "║                                                                    ║"
  echo "║  3. Optional DeepBook (for real DEX orders):                       ║"
  echo "║       export DEEPBOOK_POOL_ID='0x...'                             ║"
  echo "║       export DEEPBOOK_PACKAGE_ID='0xdee9'                          ║"
  echo "║                                                                    ║"
  echo "║  4. Run this script:                                               ║"
  echo "║       HACKATHON_DEMO_MODE=false ./scripts/demo/record_testnet_demo.sh║"
  echo "╚════════════════════════════════════════════════════════════════════╝"
  exit 1
fi

# ── Show current config ────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════════"
echo "  Sui-Nexus Testnet Recording Mode"
echo "══════════════════════════════════════════════════════"
echo "  RPC URL:          ${SUI_RPC_URL:-https://fullnode.testnet.sui.io}"
echo "  Package ID:       ${AGENT_WALLET_PACKAGE_ID:-0xa051bbf9517d8ee94f2339e69877e4eacec38d3f4893b0aedf84774d18c54433}"
echo "  Gateway:         ${GATEWAY_URL}"
echo "  DeepBook Pool:   ${DEEPBOOK_POOL_ID:-not configured}"
echo "══════════════════════════════════════════════════════"
echo ""

# ── Check gateway is reachable ──────────────────────────────────────────────
if ! curl -sS "${GATEWAY_URL}/health" > /dev/null 2>&1; then
  echo "ERROR: Gateway not reachable at ${GATEWAY_URL}"
  echo "Start it first: HACKATHON_DEMO_MODE=false go run cmd/gateway/main.go"
  exit 1
fi

# ── Run agent wallet demo ──────────────────────────────────────────────────
echo ""
echo "Running Agent Wallet demo (real testnet transactions)..."
echo ""

python3 scripts/demo/agent_wallet_demo.py

echo ""
echo "══════════════════════════════════════════════════════"
echo "  Testnet recording complete!"
echo "  Check tx digests on: https://suiexplorer.com/?network=testnet"
echo "══════════════════════════════════════════════════════"
echo ""
