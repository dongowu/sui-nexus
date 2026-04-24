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
- Kafka (optional - graceful degradation without it)
- Redis (optional - graceful degradation without it)

### Run Gateway

```bash
go run cmd/gateway/main.go
```

### Run Demo

```bash
./scripts/demo/run_demo.sh
```

## API Reference

### POST /api/v1/intent

Submit a trading intent from an AI agent.

**Headers:**
- `X-API-Key`: Agent API key
- `X-Signature`: HMAC-SHA256 signature
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
    "slippage": "0.5"
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

Health check endpoint.

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
