# Sui Move Contract Deployment Guide

## Prerequisites
- Sui CLI installed: `cargo install --locked --git https://github.com/MystenLabs/sui.git --branch testnet sui`
- Funded testnet account

## Build Contract
```bash
cd move
sui move build
```

## Deploy to Testnet
```bash
sui client publish --gas-budget 100000000
```

## Expected Output
```
Transaction Digest: 8xK9mN...
Package ID: 0xabcd1234...
```

## Update Gateway Config
After deployment, update your environment:
```bash
export SUI_MEMORY_PACKAGE_ID="0xabcd1234..."
```

## Test Memory Creation
```bash
sui client call \
  --package $SUI_MEMORY_PACKAGE_ID \
  --module agent_memory \
  --function create_memory \
  --args "task-123" "walrus://blob456" \
  --gas-budget 10000000
```
