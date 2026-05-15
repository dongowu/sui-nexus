#!/usr/bin/env bash
# Sui-Nexus Dual-Agent Conflict Demo
# 双 agent 冲突演示：Agent A 正常交易 → Agent B 越权被 Guardian 拦截
#
# Prerequisites:
#   go run ./cmd/gateway/ &
#   GATEWAY_URL defaults to http://localhost:8080

set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
WS_URL="${WS_URL:-ws://localhost:8080/ws}"
DEMO_TOKEN="demo-session-token"
BOLD="\033[1m"
GREEN="\033[32m"
RED="\033[31m"
YELLOW="\033[33m"
CYAN="\033[36m"
RESET="\033[0m"

header() { echo -e "\n${BOLD}${CYAN}═══ $1 ═══${RESET}\n"; }
pass_msg() { echo -e "${GREEN}✓ $1${RESET}"; }
fail_msg() { echo -e "${RED}✗ $1${RESET}"; }
info_msg() { echo -e "${YELLOW}→ $1${RESET}"; }

# ────────────────────────────────────────────
# Step 0: Health check
# ────────────────────────────────────────────
header "Step 0: Health Check"
if ! curl -sf "$GATEWAY_URL/api/v1/health" > /dev/null 2>&1; then
    fail_msg "Gateway is not running at $GATEWAY_URL"
    echo "  Start it with: go run ./cmd/gateway/"
    exit 1
fi
pass_msg "Gateway is healthy at $GATEWAY_URL"

# ────────────────────────────────────────────
# Step 1: Create wallets for two agents
# ────────────────────────────────────────────
header "Step 1: Create Agent Wallets"

info_msg "Creating Wallet A — Agent 0xAlice, budget 500 MIST, protocol DeepBook"
RESP_A=$(curl -sf -X POST "$GATEWAY_URL/api/v1/wallet/create" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_address": "0xAlice",
    "budget_cap_mist": 500,
    "allowed_protocols": ["DeepBook"],
    "time_end_epoch": 999999
  }')
WALLET_A=$(echo "$RESP_A" | jq -r '.wallet_id')
pass_msg "Wallet A created: $WALLET_A (budget 500 MIST)"

info_msg "Creating Wallet B — Agent 0xBob, budget 100 MIST, protocol DeepBook"
RESP_B=$(curl -sf -X POST "$GATEWAY_URL/api/v1/wallet/create" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_address": "0xBob",
    "budget_cap_mist": 100,
    "allowed_protocols": ["DeepBook"],
    "time_end_epoch": 999999
  }')
WALLET_B=$(echo "$RESP_B" | jq -r '.wallet_id')
pass_msg "Wallet B created: $WALLET_B (budget 100 MIST)"

# ────────────────────────────────────────────
# Step 2: Agent A executes a valid trade
# ────────────────────────────────────────────
header "Step 2: Agent A Executes Valid Trade"

info_msg "Agent A (0xAlice) requests 100 MIST trade on DeepBook..."
RESP_A1=$(curl -sf -X POST "$GATEWAY_URL/api/v1/wallet/execute" \
  -H "Content-Type: application/json" \
  -d "{
    \"wallet_id\": \"$WALLET_A\",
    \"amount_mist\": 100,
    \"protocol\": \"DeepBook\",
    \"expected_price\": 1000,
    \"description\": \"Alice swap SUI to USDC\",
    \"session_token\": \"$DEMO_TOKEN\",
    \"user_address\": \"0xAlice\"
  }")

GUARDIAN_A1=$(echo "$RESP_A1" | jq -r '.guardian.passed')
if [ "$GUARDIAN_A1" = "true" ]; then
    pass_msg "Guardian passed — trade executed"
    echo "  Tx: $(echo "$RESP_A1" | jq -r '.tx_digest')"
    echo "  Remaining balance: $(echo "$RESP_A1" | jq -r '.balance_mist') MIST"
else
    fail_msg "UNEXPECTED: Guardian rejected Agent A's valid trade"
    echo "  $(echo "$RESP_A1" | jq -r '.guardian.message')"
    exit 1
fi

# ────────────────────────────────────────────
# Step 3: Agent B tries to exceed budget
# ────────────────────────────────────────────
header "Step 3: Agent B Attempts Budget Exceed — Guardian Blocks"

info_msg "Agent B (0xBob) requests 600 MIST — budget cap is only 100 MIST..."
RESP_B1=$(curl -sf -X POST "$GATEWAY_URL/api/v1/wallet/execute" \
  -H "Content-Type: application/json" \
  -d "{
    \"wallet_id\": \"$WALLET_B\",
    \"amount_mist\": 600,
    \"protocol\": \"DeepBook\",
    \"expected_price\": 1000,
    \"description\": \"Bob tries to drain the wallet\",
    \"session_token\": \"$DEMO_TOKEN\",
    \"user_address\": \"0xBob\"
  }")

GUARDIAN_B1=$(echo "$RESP_B1" | jq -r '.guardian.passed')
if [ "$GUARDIAN_B1" = "false" ]; then
    echo ""
    echo -e "  ${RED}╔══════════════════════════════════════════════╗${RESET}"
    echo -e "  ${RED}║  🛡️  GUARDIAN INTERCEPTED                    ║${RESET}"
    echo -e "  ${RED}╠══════════════════════════════════════════════╣${RESET}"
    echo -e "  ${RED}║                                            ║${RESET}"
    echo -e "  ${RED}║  Risk: $(printf '%-36s' "$(echo "$RESP_B1" | jq -r '.guardian.risk_type')")║${RESET}"
    echo -e "  ${RED}║  Reason: $(printf '%-34s' "$(echo "$RESP_B1" | jq -r '.guardian.reason')")║${RESET}"
    echo -e "  ${RED}║  Requested: $(printf '%-32s' "$(echo "$RESP_B1" | jq -r '.guardian.requested') MIST")║${RESET}"
    echo -e "  ${RED}║  Allowed: $(printf '%-34s' "$(echo "$RESP_B1" | jq -r '.guardian.allowed') MIST")║${RESET}"
    echo -e "  ${RED}║                                            ║${RESET}"
    echo -e "  ${RED}║  Message: $(printf '%-34s' "$(echo "$RESP_B1" | jq -r '.guardian.message' | head -c 50)...")║${RESET}"
    echo -e "  ${RED}║                                            ║${RESET}"
    echo -e "  ${RED}╚══════════════════════════════════════════════╝${RESET}"
    echo ""
    pass_msg "Guardian correctly blocked the overspend attempt"
else
    fail_msg "UNEXPECTED: Guardian allowed the overspend! Test FAILED"
    exit 1
fi

# ────────────────────────────────────────────
# Step 4: Agent B adjusts to a valid amount
# ────────────────────────────────────────────
header "Step 4: Agent B Adjusts to Valid Amount"

info_msg "Agent B (0xBob) adjusts trade to 50 MIST — within budget cap..."
RESP_B2=$(curl -sf -X POST "$GATEWAY_URL/api/v1/wallet/execute" \
  -H "Content-Type: application/json" \
  -d "{
    \"wallet_id\": \"$WALLET_B\",
    \"amount_mist\": 50,
    \"protocol\": \"DeepBook\",
    \"expected_price\": 1000,
    \"description\": \"Bob adjusted to comply with policy\",
    \"session_token\": \"$DEMO_TOKEN\",
    \"user_address\": \"0xBob\"
  }")

GUARDIAN_B2=$(echo "$RESP_B2" | jq -r '.guardian.passed')
if [ "$GUARDIAN_B2" = "true" ]; then
    pass_msg "Guardian passed — adjusted trade executed"
    echo "  Tx: $(echo "$RESP_B2" | jq -r '.tx_digest')"
    echo "  Remaining balance: $(echo "$RESP_B2" | jq -r '.balance_mist') MIST"
else
    fail_msg "UNEXPECTED: Guardian rejected the adjusted trade"
    echo "  $(echo "$RESP_B2" | jq -r '.guardian.message')"
    exit 1
fi

# ────────────────────────────────────────────
# Summary
# ────────────────────────────────────────────
header "Demo Complete — Summary"

echo -e "  Wallet A (0xAlice): budget 500 MIST"
echo -e "    ├── Trade 100 MIST → ${GREEN}APPROVED${RESET}"
echo -e "    └── Wallet A remaining: $(curl -sf "$GATEWAY_URL/api/v1/wallet/$WALLET_A" | jq -r '.balance_mist') MIST"
echo ""
echo -e "  Wallet B (0xBob): budget 100 MIST"
echo -e "    ├── Trade 600 MIST → ${RED}BLOCKED${RESET} (Guardian: budget exceeded)"
echo -e "    └── Trade 50 MIST  → ${GREEN}APPROVED${RESET}"
echo ""
echo -e "  ${BOLD}Sui-Nexus${RESET}: Every AI agent on Sui needs this settlement layer."
echo ""
echo -e "  ${YELLOW}Verify on Sui Explorer:${RESET}"
echo -e "  https://suiexplorer.com/object/0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d?network=testnet"
