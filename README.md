# Sui-Nexus

**The settlement infrastructure for the AI agent economy on Sui.**

> 🏆 Sui Overflow 2026 Submission — Agentic Web + Walrus Tracks

---

## Design Philosophy

### The Problem

AI agents today face three fundamental blockers on blockchain:

| Blocker | Current State | Sui-Nexus Solution |
|---------|--------------|-------------------|
| **Key custody** | Agents must hold private keys → security risk, compliance nightmare | HMAC + zkLogin — agents authenticate without keys |
| **Statelessness** | Agents lose context across sessions → can't build on past decisions | Walrus persistent memory + Move MemoryObject on-chain |
| **No guardrails** | Agents have unlimited access → one bad trade drains everything | Move-enforced policies: budget caps, protocol scope, time windows |

### The Insight

> **"Not another AI agent — the settlement layer every AI agent on Sui needs."**

Most projects build agents that trade. Sui-Nexus builds the infrastructure that makes agent trading safe, auditable, and composable. It's the difference between building a car and building the road.

### Design Principles

1. **Sui-native**: Every feature maps to a Sui primitive — PTB, Move objects, zkLogin, Walrus. No bolt-on payments.
2. **Trust-minimized**: Policy enforcement lives on-chain (Move), not in middleware. The gateway is a relay, not an authority.
3. **Graceful degradation**: Works without Redis. Fails loudly without Kafka. Every dependency is optional where possible.
4. **Agent-agnostic**: Any AI agent (Python, TypeScript, Rust) can connect via standard HTTP + HMAC.

---

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           AI AGENT LAYER                                 │
│                                                                          │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐       │
│  │  Analyst Agent   │  │  Trader Agent    │  │  Custom Agents   │       │
│  │  (Python, LLM)   │  │  (Python, LLM)   │  │  (Any Language)  │       │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘       │
│           │ HMAC                │ HMAC                 │ HMAC            │
└───────────┼─────────────────────┼──────────────────────┼────────────────┘
            │                     │                      │
┌───────────┼─────────────────────┼──────────────────────┼────────────────┐
│           ▼                     ▼                      ▼                 │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                     GO GATEWAY (Gin)                             │    │
│  │                                                                  │    │
│  │  ┌──────────┐  ┌──────────────┐  ┌──────────────┐              │    │
│  │  │ HMAC Auth │  │ Rate Limiter │  │ CORS / Recover│             │    │
│  │  └──────────┘  └──────────────┘  └──────────────┘              │    │
│  │                                                                  │    │
│  │  ┌──────────────────────────────────────────────────────────┐   │    │
│  │  │                    API Endpoints                          │   │    │
│  │  │                                                          │   │    │
│  │  │  /api/v1/intent          → Agent trade submission        │   │    │
│  │  │  /api/v1/task/:id        → Task status query             │   │    │
│  │  │  /api/v1/auth/zklogin    → zkLogin OAuth flow            │   │    │
│  │  │  /api/v1/wallet/create   → Agent wallet creation         │   │    │
│  │  │  /api/v1/wallet/execute  → Policy-enforced execution     │   │    │
│  │  │  /api/v1/wallet/:id/revoke → Owner revocation            │   │    │
│  │  │  /api/v1/parse           → NLP intent parsing            │   │    │
│  │  │  /ws                     → Real-time WebSocket           │   │    │
│  │  └──────────────────────────────────────────────────────────┘   │    │
│  │                                                                  │    │
│  │  ┌──────────────────────────────────────────────────────────┐   │    │
│  │  │               GUARDIAN RISK LAYER                          │   │    │
│  │  │  Slippage check | Budget check | Protocol health check    │   │    │
│  │  └──────────────────────────────────────────────────────────┘   │    │
│  └───────────────┬─────────────────────────────────────────────────┘    │
│                  │                                                      │
│  ┌───────────────┼───────────────────────────────────────────────┐    │
│  │               ▼                                                │    │
│  │  ┌─────────────────────┐  ┌──────────────┐  ┌──────────────┐ │    │
│  │  │    Kafka Queue      │  │  Redis Cache │  │  WebSocket   │ │    │
│  │  │  (async processing) │  │  (task/wallet│  │  (live push) │ │    │
│  │  └─────────┬───────────┘  └──────────────┘  └──────────────┘ │    │
│  │            │                                                   │    │
│  │            ▼                                                   │    │
│  │  ┌─────────────────────────────────────────────────────────┐  │    │
│  │  │                   PTB BUILDER                             │  │    │
│  │  │                                                          │  │    │
│  │  │  Swap (Cetus)  │  Transfer  │  AgentWallet  │  DeepBook  │  │    │
│  │  └─────────────────────────────┬───────────────────────────┘  │    │
│  │                                │                               │    │
│  │                                ▼                               │    │
│  │  ┌─────────────────────────────────────────────────────────┐  │    │
│  │  │                  PTB EXECUTOR                             │  │    │
│  │  │  Sui Go SDK → SignAndExecuteTransactionBlock              │  │    │
│  │  └─────────────────────────────────────────────────────────┘  │    │
│  └────────────────────────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                           SUI BLOCKCHAIN                                │
│                                                                         │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐      │
│  │  agent_wallet    │  │  agent_memory    │  │  DeepBook / Cetus│      │
│  │  (Move)          │  │  (Move)          │  │  (DEX)           │      │
│  │                  │  │                  │  │                   │      │
│  │  · budget caps   │  │  · Walrus blob ID│  │  · limit orders  │      │
│  │  · protocol scope│  │  · task_id       │  │  · swap routes   │      │
│  │  · time windows  │  │  · agent_address │  │                   │      │
│  │  · activity log  │  │  · timestamp     │  │                   │      │
│  │  · revocation    │  │                  │  │                   │      │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘      │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────┐      │
│  │                        WALRUS                                 │      │
│  │  Decentralized storage for AI agent context, logs, and memory │      │
│  └──────────────────────────────────────────────────────────────┘      │
└────────────────────────────────────────────────────────────────────────┘
```

### Agent Wallet Flow (zkLogin + Policy Enforcement)

```
Owner (Human)                    Gateway                      Sui Chain
    │                                │                              │
    │ POST /wallet/create            │                              │
    │ { agent: "0x..", budget: 500 } │                              │
    │ ──────────────────────────────►│                              │
    │                                │ agent_wallet::create_wallet ─►
    │                                │                          WalletCreated
    │                                │                              │
Agent (AI)                          │                              │
    │                                │                              │
    │ GET /auth/zklogin              │                              │
    │ ──────────────────────────────►│                              │
    │                            OAuth → Google → JWT              │
    │ ◄── salt + jwt + ephemeral ───│                              │
    │                                │                              │
    │ @mysten/zklogin generates:     │                              │
    │   Poseidon → address_seed     │                              │
    │   Blake2b → sui_address       │                              │
    │   Groth16 → zk_proof          │                              │
    │                                │                              │
    │ POST /auth/zklogin/submit-proof│                             │
    │ ──────────────────────────────►│                              │
    │ ◄── session verified ─────────│                              │
    │                                │                              │
    │ POST /wallet/execute           │                              │
    │ { amount: 100, protocol: DB }  │                              │
    │ ──────────────────────────────►│                              │
    │                                │ GUARDIAN checks:             │
    │                                │   ✓ slippage < 5%           │
    │                                │   ✓ budget remaining        │
    │                                │   ✓ protocol allowed        │
    │                                │                              │
    │                                │ agent_wallet::execute_trade ─►
    │                                │   ✓ is_active? ✓            │
    │                                │   ✓ time window? ✓          │
    │                                │   ✓ agent_addr match? ✓     │
    │                                │   ✓ budget cap? ✓           │
    │                                │                          TradeExecuted
    │                                │                         Activity logged
    │                                │                              │
    │                                │ (optional) DeepBook order ──►
    │ ◄── tx_digest ────────────────│                              │
```

### Walrus Memory Flow (Cross-Agent Context)

```
Analyst Agent              Gateway              Walrus            Sui Chain
    │                         │                    │                    │
    │ 1. LLM analysis         │                    │                    │
    │    → report JSON        │                    │                    │
    │                         │                    │                    │
    │ 2. POST /intent         │                    │                    │
    │    context_payload ────►│                    │                    │
    │                         │ 3. Write to Walrus┌┘                   │
    │                         │   ───────────────►│                    │
    │                         │   ◄── blob_id ────│                   │
    │                         │                    │                    │
    │                         │ 4. PTB → MoveCall  │                    │
    │                         │   agent_memory::   │                    │
    │                         │   create_memory    │                    │
    │                         │   ───────────────────────────────────►│
    │                         │                                    MemoryObject
    │                         │                                    { blob_id, task_id }
    │                         │                    │                    │
Trader Agent                 │                    │                    │
    │                         │                    │                    │
    │ 5. GET /task/:id        │                    │                    │
    │    ◄── blob_id ────────│                    │                    │
    │                         │                    │                    │
    │ 6. Read Walrus blob     │                    │                    │
    │    ───────────────────────────────────────►│                    │
    │    ◄── analysis JSON ──────────────────────│                    │
    │                         │                    │                    │
    │ 7. Execute with context │                    │                    │
    │    "analyst says BUY"   │                    │                    │
    │    POST /intent ───────►│                    │                    │
    │                         │                    │                    │
    │    Shared memory → coordinated action       │                    │
```

---

## Sui Primitives Used

| Primitive | Usage | Depth |
|-----------|-------|-------|
| **PTB (Programmable Transaction Blocks)** | Atomic multi-agent settlement: swap → distribute to N agents in one tx | Core |
| **Move Objects** | AgentWallet (policy state), MemoryObject (Walrus blob ref) | Core |
| **zkLogin** | Agent identity: Google OAuth → client-side ZK proof → Sui address | Core |
| **Walrus** | AI context storage: agent analysis, logs, cross-agent shared memory | Core |
| **DeepBook V3** | Limit order placement within agent wallet policy bounds | Integration |

---

## Track Submissions

### Track 1: Agentic Web — "Agent Wallet with zkLogin"

**Key Demo**: `scripts/demo/agent_wallet_demo.py`

Demonstrates the full Intent Engine + Agent Wallet flow:

1. **Owner creates wallet** with Move-enforced policy (500 SUI budget, DeepBook only, 24h window)
2. **Agent authenticates via zkLogin** (Google OAuth → client-side ZK proof → session)
3. **Agent executes trade** within policy bounds → Guardian passes → on-chain execution
4. **Agent attempts overspend** → rejected by Move contract (budget cap enforced on-chain)
5. **Owner revokes wallet** → agent permanently frozen → on-chain event emitted
6. **Activity log verified** on-chain via Sui Explorer

**Innovation**: Not an agent — the settlement layer every agent needs. zkLogin identity + Move policy enforcement + Guardian risk layer + DeepBook execution.

### Track 2: Walrus — "AI Agent Memory System"

**Key Demo**: `scripts/demo/walrus_memory_demo.py`

Demonstrates cross-agent persistent memory:

1. **Analyst Agent** analyzes market news via LLM → writes context to Walrus
2. **Gateway** stores blob → mints MemoryObject on-chain (blob_id + task_id)
3. **Trader Agent** queries shared memory → reads analyst's context from Walrus
4. **Trader executes** informed by analyst's research → coordinated multi-agent action

**Innovation**: Walrus as the memory layer for the AI agent economy. Not just storage — verifiable, composable, on-chain referenced memory that persists across sessions and agents.

---

## Quick Start

### Prerequisites

- Go 1.21+
- Kafka (required for `/api/v1/intent` processing)
- Redis (optional — task lookup degrades without it)
- Sui CLI (for Move contract deployment)
- A funded Sui testnet account

### Environment Variables

```bash
# Core
export HMAC_SECRET_KEY="dev-secret-key-change-in-prod"
export KAFKA_BROKERS="localhost:9092"
export REDIS_ADDR="localhost:6379"
export SUI_RPC_URL="https://fullnode.testnet.sui.io"
export SUI_SIGNER_PRIVATE_KEY="suiprivkey..."
export SUI_GAS_OBJECT_ID="0x..."
export SUI_GAS_BUDGET="10000000"

# zkLogin (for Agentic Web track)
export ZKLOGIN_ENABLED=true
export ZKLOGIN_CLIENT_ID="your-google-client-id"
export ZKLOGIN_CLIENT_SECRET="your-google-client-secret"

# Agent Wallet
export AGENT_WALLET_ENABLED=true
export AGENT_WALLET_PACKAGE_ID="0x..."  # after sui client publish

# DeepBook (for real DEX orders)
export DEEPBOOK_PACKAGE_ID="0x..."      # DeepBook V3 testnet package
export DEEPBOOK_POOL_ID="0x..."         # SUI/USDC pool on testnet
```

### Deploy Move Contracts

```bash
cd move
sui move build
sui client publish --gas-budget 100000000
# Save the package ID → AGENT_WALLET_PACKAGE_ID
```

### Start Services

```bash
docker run -d --name kafka -p 9092:9092 apache/kafka
docker run -d --name redis -p 6379:6379 redis:alpine
```

### Run Gateway

```bash
go run cmd/gateway/main.go
```

### Run Demos

```bash
# Agentic Web Track Demo
python3 scripts/demo/agent_wallet_demo.py

# Walrus Track Demo
python3 scripts/demo/walrus_memory_demo.py

# Original Agent Trading Demo
./scripts/demo/run_demo.sh
```

### Open Dashboard

```bash
open web/dashboard.html
```

---

## API Reference

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `GET` | `/health` | None | Health check (Kafka/Redis status) |
| `GET` | `/ws` | None | WebSocket real-time task updates |
| `GET` | `/api/v1/auth/zklogin` | OAuth | Initiate Google OAuth flow |
| `GET` | `/api/v1/auth/zklogin/callback` | OAuth | OAuth callback → zkLogin params |
| `POST` | `/api/v1/auth/zklogin/submit-proof` | Session | Submit ZK proof from client |
| `POST` | `/api/v1/auth/zklogin/verify` | Session | Verify zkLogin session |
| `POST` | `/api/v1/wallet/create` | None | Create Agent Wallet (Move) |
| `POST` | `/api/v1/wallet/execute` | zkLogin | Agent executes trade (Guardian + Move) |
| `POST` | `/api/v1/wallet/:id/revoke` | None | Owner revokes wallet |
| `GET` | `/api/v1/wallet/:id` | None | Query wallet state |
| `GET` | `/api/v1/wallet/:id/activity` | None | Query on-chain activity log |
| `POST` | `/api/v1/intent` | HMAC | Submit agent trading intent |
| `GET` | `/api/v1/task/:task_id` | HMAC | Query task status |
| `POST` | `/api/v1/parse` | None | NLP intent parsing |

---

## Project Structure

```
sui-nexus/
├── cmd/gateway/main.go           # Entry point
├── internal/
│   ├── config/config.go          # 30+ env-driven config fields
│   ├── gateway/
│   │   ├── handler.go            # HTTP handlers (intent, task, health)
│   │   ├── router.go             # Gin route registration
│   │   ├── middleware.go         # HMAC auth, rate limiter, CORS
│   │   ├── websocket.go          # WebSocket hub for live push
│   │   ├── agent_wallet.go       # Agent Wallet handlers + Guardian
│   │   ├── parser_client.go      # NLP service client
│   │   └── zklogin/              # zkLogin OAuth + session management
│   │       ├── provider.go       # Google/Twitch OAuth providers
│   │       ├── proof.go          # ZK proof submission types
│   │       └── ephemeral.go      # Ephemeral key manager
│   ├── kafka/                    # Kafka producer + consumer
│   ├── model/                    # Domain types
│   │   ├── intent.go             # Intent, Task, AgentShare
│   │   ├── task.go               # TaskEvent for streaming
│   │   └── agent_wallet.go       # Wallet policy, activity types
│   ├── ptb/
│   │   ├── builder.go            # PTB builder (Swap, Transfer, Wallet, DeepBook)
│   │   └── executor.go           # PTB executor (Sui SDK + raw RPC)
│   ├── storage/redis.go          # Redis task/wallet cache
│   └── walrus/client.go          # Walrus HTTP client
├── pkg/hmac/signer.go            # HMAC-SHA256 signing
├── move/
│   ├── sources/
│   │   ├── agent_wallet.move     # Agent Wallet Move contract
│   │   └── agent_memory.move     # MemoryObject for Walrus blob refs
│   └── Move.toml
├── scripts/
│   ├── demo/
│   │   ├── agent_wallet_demo.py  # Agentic Web track demo
│   │   ├── walrus_memory_demo.py # Walrus track demo
│   │   ├── analyst_agent.py      # LLM market analyst
│   │   ├── trader_agent.py       # Trade executor
│   │   └── llm_client.py         # OpenAI/Groq unified client
│   └── nlp/intent_parser.py      # NLP intent parsing service
├── web/dashboard.html            # Real-time WebSocket dashboard
└── docs/                         # Integration guides, demo scripts
```

---

## Key Features

- **HMAC Authentication**: AI agents authenticate via HMAC-SHA256 signatures — no private key custody
- **zkLogin Identity**: Agent identity via Google OAuth + client-side ZK proof (Poseidon + Blake2b + Groth16)
- **Atomic PTB Settlement**: Multi-agent, multi-step transactions in one Sui PTB
- **On-Chain Policy Enforcement**: Budget caps, protocol scope, time windows enforced by Move contract
- **Guardian Risk Layer**: Pre-flight slippage, concentration, and protocol health checks
- **Activity Log**: Every agent action recorded on-chain via Move events
- **Owner Revocation**: Instant, irreversible wallet freeze by owner
- **DeepBook Integration**: Limit order placement within policy bounds
- **Walrus Memory**: Decentralized context storage with on-chain blob references
- **Graceful Degradation**: Works without Redis; fails cleanly without Kafka
- **WebSocket Dashboard**: Real-time task and wallet status monitoring
- **NLP Intent Parsing**: Natural language → structured DeFi intents

## License

MIT
