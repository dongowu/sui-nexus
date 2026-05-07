# Sui-Nexus

Infrastructure-grade multi-agent asynchronous settlement gateway for Sui blockchain.

## 🎯 Overview

Sui-Nexus enables AI agents to execute complex, atomic transactions on Sui with:
- **Multi-agent intent execution** via standardized HTTP API
- **PTB (Programmable Transaction Blocks)** for atomic multi-party settlements
- **Walrus decentralized storage** for AI context/logs
- **HMAC authentication** replacing private key custody
- **WebSocket real-time updates** for live monitoring
- **Move contracts** for on-chain memory objects

## Quick Start

### Prerequisites
- Go 1.21+
- Kafka (required for `/api/v1/intent` processing)
- Redis (optional - task lookup degrades without it)
- Sui CLI (for Move contract deployment)
- A funded Sui testnet account

### Environment Variables
```bash
export HMAC_SECRET_KEY="dev-secret-key-change-in-prod"
export KAFKA_BROKERS="localhost:9092"
export REDIS_ADDR="localhost:6379"
export SUI_RPC_URL="https://fullnode.testnet.sui.io"
export SUI_SIGNER_PRIVATE_KEY="suiprivkey..."
export SUI_GAS_OBJECT_ID="0x..."
export SUI_GAS_BUDGET="10000000"
```

### Start Services
```bash
# Start Kafka
docker run -d --name kafka -p 9092:9092 apache/kafka

# Start Redis
docker run -d --name redis -p 6379:6379 redis:alpine
```

### Run Gateway
```bash
go run cmd/gateway/main.go
```

### Open Dashboard
```bash
open web/dashboard.html
```

### Run Demo
```bash
./scripts/demo/run_demo.sh
```

See `docs/DEMO_SCRIPT.md` for complete hackathon demo guide.

## API Reference

### POST /api/v1/intent

Submit a trading intent from an AI agent.

**Headers:**
- `X-API-Key`: Agent API key
- `X-Signature`: HMAC-SHA256 signature over `task_id:timestamp:action:amount`
- `X-Timestamp`: Unix timestamp

**Body:**
```json
{
  "task_id": "uuid",
  "action": "Swap",
  "params": {
    "amount": "1000",
    "token_in": "USDT",
    "token_out": "SUI",
    "slippage": "0.5",
    "move_package_object_id": "0x...",
    "move_module": "router",
    "move_function": "swap_exact_in",
    "move_type_arguments": ["0x2::sui::SUI"],
    "move_arguments": ["0xPool", "0xInputCoin", "1000000000", "995000000"]
  },
  "agents": [
    {"address": "0x...", "share": 0.1}
  ],
  "context_payload": "base64-encoded-data"
}
```

### GET /api/v1/task/:task_id

Query task status by ID.

### GET /health

Health check endpoint. Returns `503` when the required queue component is unavailable.

### GET /ws

WebSocket endpoint for real-time task updates. Connect from Dashboard for live monitoring.

## Sui Execution Boundary

`Transfer` intents use the Sui Go SDK path: the gateway builds an unsigned `unsafe_transferSui` transaction, signs it with the configured signer, and submits it with `sui_executeTransactionBlock`.

`Swap` intents can also execute through the SDK when the request supplies a real Move call:

- `move_package_object_id`
- `move_module`
- `move_function`
- `move_type_arguments`
- `move_arguments`

If those fields are omitted, `Swap` remains a draft plan because real DEX routing still depends on concrete package, pool, coin object, and route parameters.

## Architecture

```
┌─────────────┐     HTTP      ┌─────────────┐    Kafka     ┌─────────────┐
│ Python Agent├──────────────►│  Go Gateway ├─────────────►│ PTB Builder │
└─────────────┘               └─────────────┘              └─────────────┘
                                    │                           │
                                    │                           ▼
                                    ▼                    ┌─────────────┐
                              ┌─────────────┐           │ Sui Network │
                              │    Redis    │           └─────────────┘
                              └─────────────┘                  │
                                    │                         ▼
                                    ▼                  ┌─────────────┐
                              ┌─────────────┐          │   Walrus    │
                              │  WebSocket  │          └─────────────┘
                              └─────────────┘
```

## 📁 Project Structure

```
sui-nexus/
├── cmd/gateway/           # Main entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── gateway/          # HTTP handlers & WebSocket
│   ├── kafka/            # Message queue
│   ├── model/            # Data models
│   ├── ptb/              # PTB builder & executor
│   ├── storage/          # Redis storage
│   └── walrus/           # Walrus client
├── pkg/hmac/             # HMAC authentication
├── scripts/demo/         # Python demo agents
├── move/                 # Sui Move contracts
├── web/                  # Dashboard UI
└── docs/                 # Documentation
```

## 🚀 Move Contract Deployment

```bash
cd move
sui move build
sui client publish --gas-budget 100000000
```

Save the Package ID:
```bash
export SUI_MEMORY_PACKAGE_ID="0x..."
```

## 📊 Key Features

- **HMAC Authentication**: No private key exposure for AI agents
- **Atomic PTB**: Multi-step operations in one transaction
- **WebSocket Updates**: Real-time task monitoring
- **Graceful Degradation**: Works without Redis, fails fast without Kafka
- **Production Ready**: Comprehensive error handling and testing

## License

MIT
