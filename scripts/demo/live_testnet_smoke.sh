#!/usr/bin/env bash
set -euo pipefail

# Minimal live verification for the post-publish agent-wallet flow.
# Requires:
#   - Gateway already running in real mode
#   - AGENT_WALLET_PACKAGE_ID, SUI_FUNDING_OBJECT_ID configured
#   - jq, curl available
#
# Note:
#   - The current JSON-RPC moveCall path has poor support for vector allowlists,
#     so live smoke keeps on-chain allowlist empty and validates policy via
#     budget/time/owner checks. The execute step still uses a full protocol ID.

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
OWNER_ADDRESS="${DEMO_OWNER_ADDRESS:-${DEMO_AGENT_ADDRESS:-}}"
OWNER_TOKEN="${DEMO_OWNER_TOKEN:-${DEMO_ZKLOGIN_TOKEN:-testnet-session-token}}"
AGENT_ADDRESS="${DEMO_AGENT_ADDRESS:-$OWNER_ADDRESS}"
AGENT_TOKEN="${DEMO_ZKLOGIN_TOKEN:-testnet-session-token}"
PROTOCOL_ID="${DEEPBOOK_PACKAGE_ID:-0xdee9}"
WALLET_BUDGET_MIST="${WALLET_BUDGET_MIST:-200000000}"
SAFE_AMOUNT_MIST="${SAFE_AMOUNT_MIST:-50000000}"
OVERSPEND_AMOUNT_MIST="${OVERSPEND_AMOUNT_MIST:-300000000}"
EXPECTED_PRICE="${EXPECTED_PRICE:-1000}"
OBSERVED_PRICE="${OBSERVED_PRICE:-1000}"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing dependency: $1"
    exit 1
  }
}

need_cmd curl
need_cmd jq

if [[ -z "${OWNER_ADDRESS}" ]]; then
  echo "Set DEMO_OWNER_ADDRESS or DEMO_AGENT_ADDRESS first."
  exit 1
fi

if [[ -z "${AGENT_ADDRESS}" ]]; then
  echo "Set DEMO_AGENT_ADDRESS first."
  exit 1
fi

if [[ -z "${AGENT_WALLET_PACKAGE_ID:-}" ]]; then
  echo "Set AGENT_WALLET_PACKAGE_ID first."
  exit 1
fi

if [[ -z "${SUI_FUNDING_OBJECT_ID:-}" ]]; then
  echo "Set SUI_FUNDING_OBJECT_ID first."
  exit 1
fi

echo "== Health =="
HEALTH_RESP="$(curl -sS "${GATEWAY_URL}/health" || true)"
if [[ -z "${HEALTH_RESP}" ]]; then
  echo "Gateway is not reachable at ${GATEWAY_URL}"
  exit 1
fi
echo "${HEALTH_RESP}" | jq .
echo "Note: queue may be unavailable in wallet-only live verification; continuing."

echo
echo "== Create Wallet =="
CREATE_RESP="$(
  curl -fsS -X POST "${GATEWAY_URL}/api/v1/wallet/create" \
    -H 'Content-Type: application/json' \
    -d "{
      \"agent_address\": \"${AGENT_ADDRESS}\",
      \"budget_cap_mist\": ${WALLET_BUDGET_MIST},
      \"allowed_protocols\": [],
      \"time_end_epoch\": 999999,
      \"user_address\": \"${OWNER_ADDRESS}\",
      \"session_token\": \"${OWNER_TOKEN}\"
    }"
)"
echo "${CREATE_RESP}" | jq .
WALLET_ID="$(echo "${CREATE_RESP}" | jq -r '.wallet_id')"
CREATE_DIGEST="$(echo "${CREATE_RESP}" | jq -r '.tx_digest')"
if [[ -z "${WALLET_ID}" || "${WALLET_ID}" == "null" ]]; then
  echo "Wallet creation failed."
  exit 1
fi

echo
echo "== Safe Trade =="
SAFE_RESP="$(
  curl -fsS -X POST "${GATEWAY_URL}/api/v1/wallet/execute" \
    -H 'Content-Type: application/json' \
    -d "{
      \"wallet_id\": \"${WALLET_ID}\",
      \"amount_mist\": ${SAFE_AMOUNT_MIST},
      \"protocol\": \"${PROTOCOL_ID}\",
      \"expected_price\": ${EXPECTED_PRICE},
      \"observed_price\": ${OBSERVED_PRICE},
      \"description\": \"live smoke trade\",
      \"user_address\": \"${AGENT_ADDRESS}\",
      \"session_token\": \"${AGENT_TOKEN}\"
    }"
)"
echo "${SAFE_RESP}" | jq .

echo
echo "== Overspend Block =="
set +e
OVERSPEND_RESP="$(
  curl -sS -X POST "${GATEWAY_URL}/api/v1/wallet/execute" \
    -H 'Content-Type: application/json' \
    -d "{
      \"wallet_id\": \"${WALLET_ID}\",
      \"amount_mist\": ${OVERSPEND_AMOUNT_MIST},
      \"protocol\": \"${PROTOCOL_ID}\",
      \"expected_price\": ${EXPECTED_PRICE},
      \"observed_price\": ${OBSERVED_PRICE},
      \"description\": \"overspend smoke check\",
      \"user_address\": \"${AGENT_ADDRESS}\",
      \"session_token\": \"${AGENT_TOKEN}\"
    }"
)"
set -e
echo "${OVERSPEND_RESP}" | jq .

echo
echo "== Revoke Wallet =="
REVOKE_RESP="$(
  curl -fsS -X POST "${GATEWAY_URL}/api/v1/wallet/${WALLET_ID}/revoke" \
    -H 'Content-Type: application/json' \
    -d "{
      \"wallet_id\": \"${WALLET_ID}\",
      \"user_address\": \"${OWNER_ADDRESS}\",
      \"session_token\": \"${OWNER_TOKEN}\"
    }"
)"
echo "${REVOKE_RESP}" | jq .

echo
echo "== Final Wallet State =="
curl -fsS "${GATEWAY_URL}/api/v1/wallet/${WALLET_ID}" | jq .

echo
echo "Create digest:  ${CREATE_DIGEST}"
echo "Wallet ID:      ${WALLET_ID}"
echo "Explorer link:  https://suiscan.xyz/txblock/${CREATE_DIGEST}?network=testnet"
