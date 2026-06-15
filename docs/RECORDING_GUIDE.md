# Sui Overflow 2026 — Real Testnet Demo Recording Guide

> Records real Sui testnet transactions for the submission demo video.
> All transactions are verifiable on-chain at `https://suiscan.xyz/?network=testnet`

## Pre-Recording Setup

### 1. Environment Variables

```bash
# Required — Sui credentials (from `sui client gas`)
export SUI_SIGNER_MNEMONIC='your 24-word testnet mnemonic'
export SUI_GAS_OBJECT_ID='0x...'
export SUI_FUNDING_OBJECT_ID='0x...'

# Required — Package ID (already deployed)
export AGENT_WALLET_PACKAGE_ID='0x262b81797305980a5ddf2c509a6ac8fb9577dee6ac6c96ceba6580bd3dde5058'

# Optional — Real DeepBook orders
export DEEPBOOK_POOL_ID='0x...'
export DEEPBOOK_PACKAGE_ID='0xdee9'

# Optional — Walrus (real blob storage)
export WALRUS_API_URL='https://walrus.testnet.sui.io'
```

### 2. Start Gateway (real mode)

```bash
# MUST be false — real testnet transactions
HACKATHON_DEMO_MODE=false \
SUI_SIGNER_MNEMONIC="$SUI_SIGNER_MNEMONIC" \
SUI_GAS_OBJECT_ID="$SUI_GAS_OBJECT_ID" \
go run cmd/gateway/main.go
```

### 3. Verify Gateway

```bash
curl -s http://localhost:8080/health | python3 -m json.tool
# Should show "ready": true, "demo_mode": false
```

### 4. Start Screen Recording

Use macOS QuickTime or `ffmpeg`:

```bash
# ffmpeg screen capture (2560x1440@30fps, ~50MB/min)
ffmpeg -f avfoundation -i "1:0" \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 128k \
  ~/Desktop/sui-nexus-demo-$(date +%Y%m%d-%H%M%S).mp4
```

## Demo Video Script (Target: 3 minutes)

### Part 1: The Problem (20 seconds)

**Narration** (read at 1.5x speed):
> "AI agents can build trading strategies. But right now, every agent needs a private key, and every trade needs a human signature. That's a security nightmare. One compromised key drains everything."

*Show: Terminal with agent code trying to sign with private key → compromise diagram*

---

### Part 2: The Solution (20 seconds)

**Narration**:
> "Sui-Nexus is the settlement layer for the AI agent economy. Instead of handing agents private keys, owners create policy-controlled wallets. Budget caps, protocol scope, time windows — all enforced ON-CHAIN in Move. The gateway is a relay, not an authority."

*Show: Architecture diagram (web/dashboard.html or README architecture section)*

---

### Part 3: Agent Wallet Live Demo (90 seconds)

#### Step 1 — Owner Creates Wallet (20s)

**Narration**:
> "Owner creates and funds an Agent Wallet. 500 SUI budget. DeepBook only. 24-hour window."

```bash
# This makes a REAL on-chain transaction
curl -X POST http://localhost:8080/api/v1/wallet/create \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_address": "0xAGENT_ADDRESS",
    "budget_cap_mist": 500000000000,
    "allowed_protocols": [],
    "time_end_epoch": 999999,
    "user_address": "0xOWNER_ADDRESS",
    "session_token": "testnet-session-token"
  }'
```

*Show: Command output with real `wallet_id` and `tx_digest`, open Explorer to show pending tx*
*Live note: current JSON-RPC path keeps on-chain allowlist empty during smoke verification; budget, owner, and revoke enforcement are still live.*

**Explorer verification**: Show the pending transaction in `https://suiscan.xyz/?network=testnet`

---

#### Step 2 — Agent Authenticates via zkLogin (20s)

**Narration**:
> "The AI agent authenticates via zkLogin. Google OAuth generates an ephemeral key pair. The client proves possession WITHOUT transmitting the key — zero knowledge proof. Groth16 verifies on gateway, and a session token is issued."

*Show: Browser flow OR command-line session token exchange (zkLogin is real OAuth flow)*

```bash
# Start OAuth flow in browser
open http://localhost:8080/api/v1/auth/zklogin
# After OAuth callback, get session token
```

For demo purposes (no browser required):
```bash
export DEMO_ZKLOGIN_ADDRESS='0xAGENT_ADDRESS'
export DEMO_ZKLOGIN_TOKEN='your-real-session-token'
```

---

#### Step 3 — Agent Executes Safe Trade (20s)

**Narration**:
> "Agent executes a 100 SUI trade. Guardian pre-flight checks: slippage OK, within budget, protocol allowed. All green. The Move wallet enforces the same policy on-chain as the final backstop."

```bash
curl -X POST http://localhost:8080/api/v1/wallet/execute \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id": "<WALLET_ID_FROM_STEP_1>",
    "amount_mist": 100000000000,
    "protocol": "0xdee9",
    "expected_price": 1000,
    "observed_price": 1000,
    "description": "Limit order: Buy SUI on DeepBook",
    "user_address": "0xAGENT_ADDRESS",
    "session_token": "<SESSION_TOKEN>"
  }'
```

*Show: Real tx_digest in output, confirm on Explorer*

---

#### Step 4 — Overspend Blocked (20s) ⭐ KEY MOMENT ⭐

**Narration**:
> "Now the agent tries to spend 600 SUI. The budget cap is 500. Guardian says no. Move contract is the on-chain backstop — even if the gateway is compromised, the blockchain enforces the cap."

```bash
curl -X POST http://localhost:8080/api/v1/wallet/execute \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id": "<WALLET_ID_FROM_STEP_1>",
    "amount_mist": 600000000000,
    "protocol": "0xdee9",
    "expected_price": 1000,
    "observed_price": 1000,
    "description": "Attempted overspend — should fail",
    "user_address": "0xAGENT_ADDRESS",
    "session_token": "<SESSION_TOKEN>"
  }'
```

*Show: `"error": {"code": "ERR_GUARDIAN_REJECTED", "message": "Budget cap exceeded"}` in red*

---

#### Step 5 — Owner Revokes Wallet (10s)

**Narration**:
> "Owner revokes the wallet. One transaction. Permanently frozen. Agent is done."

```bash
curl -X POST "http://localhost:8080/api/v1/wallet/<WALLET_ID_FROM_STEP_1>/revoke" \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id": "<WALLET_ID_FROM_STEP_1>",
    "user_address": "0xOWNER_ADDRESS",
    "session_token": "testnet-session-token"
  }'
```

*Show: `is_active: false` in response*

---

### Part 4: Walrus Memory (30 seconds)

**Narration**:
> "Now let's look at the second track — Walrus persistent memory. Two AI agents share context across sessions."

```bash
python3 scripts/demo/walrus_memory_demo.py
```

*Show: Analyst writes to Walrus → Trader reads from Walrus → coordinated follow-up intent*

**Explorer verification**: Show the MemoryObject on-chain referencing the Walrus blob

---

### Part 5: Chain Verification (20 seconds)

**Narration**:
> "Every transaction is on Sui testnet. Let's verify."

*Open Explorer tabs showing:*
1. `agent_wallet::create_wallet` transaction
2. `agent_wallet::execute_trade` transaction
3. `agent_wallet::revoke` transaction
4. `agent_memory::create_memory` transaction

---

## Recording Checklist

- [ ] Gateway running with `HACKATHON_DEMO_MODE=false`
- [ ] Real `tx_digest` values in output (not `demo-*`)
- [ ] Explorer tab open for each transaction
- [ ] Walrus demo shows real `blob_id` (not `demo-blob-*`)
- [ ] Overspend blocked moment captured
- [ ] Video under 3 minutes
- [ ] Audio narration synced to demo
- [ ] Export as MP4 (H.264)

## Troubleshooting

### "sui client gas" returns empty
```bash
sui client gas --assign $(sui client active-addresses)
```

### Gateway refuses transaction
Check that `SUI_SIGNER_MNEMONIC` and `SUI_GAS_OBJECT_ID` are correctly set and the account has sufficient gas.

### DeepBook order fails
DeepBook pool may need to be funded with USDC. Omit `DEEPBOOK_POOL_ID` to record the policy tx without the DEX order — the policy enforcement still demonstrates the full flow.
