# Cetus DEX Integration Guide

## Cetus Protocol on Sui Testnet

Cetus is a leading DEX on Sui. This integration uses their testnet deployment.

### Testnet Addresses

```bash
# Cetus Router Package
CETUS_PACKAGE_ID="0x2c8d603bc51326b8c13cef9dd07031a408a48dddb541963357661df5d3204809"

# Common Pools (Testnet)
SUI_USDC_POOL="0x..."  # Replace with actual pool ID
```

### Integration in PTB Builder

The builder now supports real Cetus swaps when these params are provided:
- `move_package_object_id`: Cetus package ID
- `move_module`: "router"
- `move_function`: "swap_exact_in"
- `move_type_arguments`: Token types
- `move_arguments`: Pool, coin, amounts

### Example Request

```json
{
  "task_id": "swap-001",
  "action": "Swap",
  "params": {
    "amount": "1000",
    "token_in": "USDC",
    "token_out": "SUI",
    "slippage": "0.5",
    "move_package_object_id": "0x2c8d603bc51326b8c13cef9dd07031a408a48dddb541963357661df5d3204809",
    "move_module": "router",
    "move_function": "swap_exact_in",
    "move_type_arguments": ["0x2::sui::SUI", "0x...::usdc::USDC"],
    "move_arguments": ["0xPoolID", "0xCoinID", "1000000000", "995000000"]
  }
}
```

## Note

For hackathon demo, you can:
1. Use Transfer action (already works)
2. Or provide real Cetus params for Swap
3. The gateway will execute via Sui SDK
