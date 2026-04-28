# Sui-Nexus

Infrastructure-grade multi-agent asynchronous settlement gateway for Sui blockchain.

## Overview

Sui-Nexus enables AI agents to execute complex, atomic transactions on Sui with:
- **Multi-agent intent execution** via standardized HTTP API
- **PTB (Programmable Transaction Blocks)** for atomic multi-party settlements
- **Walrus decentralized storage** for AI context/logs
- **HMAC authentication** replacing private key custody

## Quick Start

### Prerequisites
- Go 1.21+
- Kafka (required for `/api/v1/intent` processing; the gateway returns `503` when the queue is unavailable)
- Redis (optional - task lookup degrades without it)
- A Sui signer mnemonic or `suiprivkey...` private key plus a funded SUI coin object for live transfers

### Run Gateway

```bash
export SUI_SIGNER_PRIVATE_KEY="suiprivkey..."
export SUI_GAS_OBJECT_ID="0x..."
go run cmd/gateway/main.go
```

### Run Demo

```bash
./scripts/demo/run_demo.sh
```

See `docs/demo.md` for the judge-facing setup, readiness checks, and live Sui execution requirements.

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

Health check endpoint. Returns `503` when the required queue component is unavailable, so demos fail early instead of accepting intents that cannot be processed.

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
                              │   Storage   │          └─────────────┘
                              └─────────────┘
```

## License

MIT
