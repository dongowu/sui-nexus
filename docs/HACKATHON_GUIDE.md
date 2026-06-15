# Sui Overflow 2026 — 提交清单

## 赛道信息

- **主赛道**: Agentic Web (Intent Engine 子赛道)
- **副赛道**: Walrus
- **项目名**: Sui-Nexus
- **仓库**: https://github.com/your-username/sui-nexus

## 已部署合约 (Sui Testnet)

| 合约 | 地址 | Explorer |
|------|------|----------|
| Package | `0x262b81797305980a5ddf2c509a6ac8fb9577dee6ac6c96ceba6580bd3dde5058` (v2) | [View](https://suiscan.xyz/object/0x262b81797305980a5ddf2c509a6ac8fb9577dee6ac6c96ceba6580bd3dde5058?network=testnet) |
| Upgrade Cap | `0x225f7b278c1fc2d3b5cf3d38a5f5e344463aaaf67f52a97b4a51008499a2145f` | [View](https://suiscan.xyz/object/0x225f7b278c1fc2d3b5cf3d38a5f5e344463aaaf67f52a97b4a51008499a2145f?network=testnet) |

## 提交材料清单

- [ ] **GitHub 仓库** 公开可见
- [ ] **README.md** 含架构图、赛道说明、Quick Start
- [ ] **Demo 视频** (建议 3 分钟以内) — 参考 [DEMO_VIDEO_GUIDE.md](DEMO_VIDEO_GUIDE.md)
- [ ] **DoraHacks 项目页面** 填写完整 — 参考 [DORAHACKS_SUBMISSION.md](DORAHACKS_SUBMISSION.md)
- [ ] **测试通过**: `go test ./...`

## 演示前的环境准备

### 评委一键演示（推荐）

```bash
HACKATHON_DEMO_MODE=true ./scripts/demo/run_agent_wallet_demo.sh
```

然后打开 `web/dashboard.html`，依次点击：

1. `Create Agent Wallet`
2. `Execute Safe Trade`
3. `Attempt Overspend`

这个模式不需要 Kafka、Redis、zkLogin、Sui 私钥或 gas coin。它会明确返回 `demo-*` digest，用于展示产品闭环；真实 testnet package 地址和 live 执行路径仍保留在 README 和配置中。

```bash
# 1. 启动依赖服务
docker run -d --name kafka -p 9092:9092 apache/kafka:3.7.0
docker run -d --name redis -p 6379:6379 redis:alpine

# 2. 设置环境变量
export HMAC_SECRET_KEY="dev-secret-key-change-in-prod"
export KAFKA_BROKERS="localhost:9092"
export REDIS_ADDR="localhost:6379"
export SUI_RPC_URL="https://fullnode.testnet.sui.io"
export SUI_SIGNER_PRIVATE_KEY="suiprivkey..."
export SUI_GAS_OBJECT_ID="0x..."
export SUI_FUNDING_OBJECT_ID="0x..."
export AGENT_WALLET_PACKAGE_ID="0x262b81797305980a5ddf2c509a6ac8fb9577dee6ac6c96ceba6580bd3dde5058"

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
