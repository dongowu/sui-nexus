# Sui-Nexus — 真实测试网交易录制指南

> 为 Sui Overflow 2026 黑客松录制真实 on-chain 交易 demo 视频

## 环境状况总览

| 项目 | 状态 |
|------|------|
| Sui CLI | ✅ 已安装 `/opt/homebrew/bin/sui` |
| 测试网钱包 | ✅ `0x79ee84d793ed41f9868a63c7d0f2e62b2752ea0078944db44940b751d27a05a1` |
| Gas Coin | ✅ `0x111fbd6db848078d54afcd654406d572cccc1cc78e705333750c1e5c006e017d` (余额: 0.47 SUI) |
| 已部署 Package | ✅ `0x262b81797305980a5ddf2c509a6ac8fb9577dee6ac6c96ceba6580bd3dde5058` (v2) |
| 私钥 | ✅ 已导出（`suiprivkey1qq6ulqzpm...`） |

**注意**: 钱包只有 0.47 SUI，足够录制 demo 但建议后续补充测试网 funds。

---

## 第一步：生成环境配置文件

```bash
# 运行自动化设置脚本
bash scripts/demo/setup_testnet_env.sh

# 屏幕输出会显示生成的文件路径（可能是 /tmp/sui-nexus-env.sh）
```

---

## 第二步：启动 Gateway（真实交易模式）

```bash
# 加载环境变量
source /tmp/sui-nexus-env.sh   # 或 source .env.testnet

# 启动 Gateway（后台运行）
HACKATHON_DEMO_MODE=false go run cmd/gateway/main.go &
sleep 3

# 验证 Gateway 就绪
curl -s http://localhost:8080/health | python3 -m json.tool
# 确认 demo_mode: false, ready: true
```

---

## 第三步：录制 Agent Wallet Demo

```bash
# 方式 A: 使用 Python demo 脚本（自动处理会话）
source /tmp/sui-nexus-env.sh
python3 scripts/demo/agent_wallet_demo.py
# 脚本会自动使用 DEMO_ZKLOGIN_ADDRESS + DEMO_ZKLOGIN_TOKEN

# 方式 B: 手动 curl 命令（更直观展示给评委）
# 创建钱包
curl -X POST http://localhost:8080/api/v1/wallet/create \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_address": "0x79ee84d793ed41f9868a63c7d0f2e62b2752ea0078944db44940b751d27a05a1",
    "budget_cap_mist": 500000000000,
    "allowed_protocols": [],
    "time_end_epoch": 999999,
    "user_address": "0x79ee84d793ed41f9868a63c7d0f2e62b2752ea0078944db44940b751d27a05a1",
    "session_token": "testnet-session-token"
  }'

# 执行安全交易（100 SUI）
curl -X POST http://localhost:8080/api/v1/wallet/execute \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id": "<WALLET_ID>",
    "amount_mist": 100000000000,
    "protocol": "0xdee9",
    "expected_price": 1000,
    "observed_price": 1000,
    "description": "Limit order: Buy SUI on DeepBook",
    "user_address": "0x79ee84d793ed41f9868a63c7d0f2e62b2752ea0078944db44940b751d27a05a1",
    "session_token": "testnet-session-token"
  }'

# 尝试超额交易（被拦截）
curl -X POST http://localhost:8080/api/v1/wallet/execute \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id": "<WALLET_ID>",
    "amount_mist": 600000000000,
    "protocol": "0xdee9",
    "expected_price": 1000,
    "observed_price": 1000,
    "description": "Attempted overspend",
    "user_address": "0x79ee84d793ed41f9868a63c7d0f2e62b2752ea0078944db44940b751d27a05a1",
    "session_token": "testnet-session-token"
  }'

# 撤销钱包
curl -X POST "http://localhost:8080/api/v1/wallet/<WALLET_ID>/revoke" \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id": "<WALLET_ID>",
    "user_address": "0x79ee84d793ed41f9868a63c7d0f2e62b2752ea0078944db44940b751d27a05a1",
    "session_token": "testnet-session-token"
  }'
```

---

## 第四步：录制 Walrus Memory Demo

```bash
python3 scripts/demo/walrus_memory_demo.py
```

---

## 第五步：录制屏幕（QuickTime）

```bash
# 打开 QuickTime
open -a "QuickTime Player"

# 文件 → 新建屏幕录制
# 选择区域：2560×1440 或 1920×1080
# 点击录制

# 按 Option+5 调出截屏工具栏

# 或者用 ffmpeg（需先安装: brew install ffmpeg）
ffmpeg -f avfoundation -i "1:0" \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 128k \
  ~/Desktop/sui-nexus-recording-$(date +%Y%m%d-%H%M%S).mp4
```

---

## Explorer 验证

每个交易完成后，用真实 tx_digest 在 Explorer 验证：

```
https://suiexplorer.com/txblock/<TX_DIGEST>?network=testnet
```

---

## 问题排查

### Gateway 返回 "zkLogin is not configured"
当 `HACKATHON_DEMO_MODE=false` 且 `ZKLOGIN_ENABLED=false` 时，agent_wallet execute 需要绕过 zkLogin 验证。
检查 `DEMO_ZKLOGIN_TOKEN=testnet-session-token` 是否设置。

### InsufficientGas
0.47 SUI 足够录制，但 DeepBook 订单可能需要更多。跳过 DEEPBOOK_POOL_ID 让 policy transaction 先走通。

### 交易超时
```bash
# 确认 RPC 可达
curl -s -X POST https://fullnode.testnet.sui.io:443 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"sui_getTotalTransactionBlocks","params":[]}'
# 应返回 "result": <数字>
```

---

## .env.testnet 文件内容

```bash
# Sui-Nexus Testnet Environment — Real Transaction Mode
export SUI_RPC_URL="https://fullnode.testnet.sui.io:443"
export SUI_SIGNER_PRIVATE_KEY="suiprivkey1qq6ulqzpm7hkhxaa7z7gyvfa44mahz03xwr2uzal0vp9t3qt7z972ag8454"
export SUI_GAS_OBJECT_ID="0x111fbd6db848078d54afcd654406d572cccc1cc78e705333750c1e5c006e017d"
export SUI_FUNDING_OBJECT_ID="0x..."
export SUI_SIGNER_MNEMONIC=""
export AGENT_WALLET_PACKAGE_ID="0x262b81797305980a5ddf2c509a6ac8fb9577dee6ac6c96ceba6580bd3dde5058"
export DEEPBOOK_PACKAGE_ID="0xdee9"
export DEEPBOOK_POOL_ID=""
export WALRUS_API_URL="https://walrus.testnet.sui.io"
export SERVER_PORT="8080"
export HMAC_SECRET_KEY="dev-secret-key-change-in-prod"
export KAFKA_BROKERS="localhost:9092"
export REDIS_ADDR="localhost:6379"
export ZKLOGIN_ENABLED="false"
export DEMO_AGENT_ADDRESS="0x79ee84d793ed41f9868a63c7d0f2e62b2752ea0078944db44940b751d27a05a1"
export DEMO_ZKLOGIN_ADDRESS="0x79ee84d793ed41f9868a63c7d0f2e62b2752ea0078944db44940b751d27a05a1"
export DEMO_ZKLOGIN_TOKEN="testnet-session-token"
export HACKATHON_DEMO_MODE="false"
export AGENT_WALLET_ENABLED="true"
export SUI_GAS_BUDGET="10000000"
```

> ⚠️ 不要将这个文件提交到 git！它包含私钥。
