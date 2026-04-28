# Demo Guide

This demo shows the gateway path that is ready for judges:

1. agent signs an intent with HMAC headers
2. gateway validates the timestamp and signature
3. gateway queues the task in Kafka
4. worker writes context to Walrus when configured
5. worker builds a Sui SDK transaction plan
6. worker signs and submits Transfer or MoveCall-backed Swap
7. task status can be queried from Redis

## Requirements

- Go on `PATH`
- Python 3 with `requests`
- Kafka reachable at `KAFKA_BROKERS`
- Optional Redis at `REDIS_ADDR` for task lookup
- For live Sui execution:
  - `SUI_SIGNER_PRIVATE_KEY` or `SUI_SIGNER_MNEMONIC`
  - `SUI_GAS_OBJECT_ID`
  - funded Sui testnet account

## Environment

```bash
export HMAC_SECRET_KEY="dev-secret-key-change-in-prod"
export KAFKA_BROKERS="localhost:9092"
export REDIS_ADDR="localhost:6379"
export SUI_RPC_URL="https://fullnode.testnet.sui.io"
export SUI_SIGNER_PRIVATE_KEY="suiprivkey..."
export SUI_GAS_OBJECT_ID="0x..."
export SUI_GAS_BUDGET="10000000"
```

## Run

```bash
./scripts/demo/run_demo.sh
```

If the gateway is already running:

```bash
START_GATEWAY=0 GATEWAY_URL=http://localhost:8080 ./scripts/demo/run_demo.sh
```

## What To Show Judges

- `GET /health` returns `ready: true`
- analyst/trader agents submit signed intents with headers
- `POST /api/v1/intent` returns `202`
- `GET /api/v1/task/<task_id>` returns task status and, after execution, a Sui transaction digest

## Swap Boundary

`Swap` can execute only when the request includes a real Move call:

- `move_package_object_id`
- `move_module`
- `move_function`
- `move_type_arguments`
- `move_arguments`

Without those fields, the swap remains a draft plan and is not submitted as a fake transaction.
