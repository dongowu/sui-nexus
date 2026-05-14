# Sui Overflow 2026 — 提交清单

## 赛道信息

- **主赛道**: Agentic Web (Intent Engine 子赛道)
- **副赛道**: Walrus
- **项目名**: Sui-Nexus
- **仓库**: (你的 GitHub 仓库 URL)

## 已部署合约 (Sui Testnet)

| 合约 | 地址 | Explorer |
|------|------|----------|
| Package | `0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d` | [View](https://suiexplorer.com/object/0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d?network=testnet) |
| Upgrade Cap | `0x7bd41eb7253f93e03f84fe2c963347b62a5cae57a29c8200c92e9a4c6bbfb06b` | [View](https://suiexplorer.com/object/0x7bd41eb7253f93e03f84fe2c963347b62a5cae57a29c8200c92e9a4c6bbfb06b?network=testnet) |

## 提交材料清单

- [ ] **GitHub 仓库** 公开可见
- [ ] **README.md** 含架构图、赛道说明、Quick Start
- [ ] **Demo 视频** (建议 3 分钟以内)
  - Agent Wallet Demo: `scripts/demo/agent_wallet_demo.py`
  - Walrus Memory Demo: `scripts/demo/walrus_memory_demo.py`
- [ ] **DoraHacks 项目页面** 填写完整
  - 项目描述 (英文)
  - 赛道选择
  - 仓库链接
  - Demo 视频链接
  - Sui 合约地址

## 演示前的环境准备

```bash
# 1. 启动依赖服务
docker run -d --name kafka -p 9092:9092 apache/kafka
docker run -d --name redis -p 6379:6379 redis:alpine

# 2. 设置环境变量
export HMAC_SECRET_KEY="dev-secret-key-change-in-prod"
export KAFKA_BROKERS="localhost:9092"
export REDIS_ADDR="localhost:6379"
export SUI_RPC_URL="https://fullnode.testnet.sui.io"
export SUI_SIGNER_PRIVATE_KEY="suiprivkey..."
export SUI_GAS_OBJECT_ID="0x..."
export AGENT_WALLET_PACKAGE_ID="0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d"

# 3. 启动网关
go run cmd/gateway/main.go

# 4. 运行 Demo
python3 scripts/demo/agent_wallet_demo.py
python3 scripts/demo/walrus_memory_demo.py
```

## 核心创新点 (评委视角)

1. **HMAC 无密钥认证** — AI agent 不需要持有私钥，消除最大安全风险
2. **zkLogin 身份** — Google OAuth + ZK proof → Sui 地址，零门槛 agent 上链
3. **PTB 原子结算** — 多 agent、多步骤在一个交易中完成，全部成功或全部回滚
4. **Move 策略执行** — 预算上限、协议范围、时间窗口全部在链上执行，不可绕过
5. **Guardian 风险层** — 滑点检查 + 预算检查 + 协议健康检查，三道防线
6. **Walrus 跨 agent 记忆** — 分析师写 Walrus → 交易员读 → 协调操作，真正多 agent 协作
