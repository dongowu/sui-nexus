# Sui Overflow 2026 - 完整部署指南

## 🎯 快速开始（5分钟）

### 1. 启动依赖服务
```bash
# Kafka
docker run -d --name kafka -p 9092:9092 apache/kafka

# Redis
docker run -d --name redis -p 6379:6379 redis:alpine
```

### 2. 配置环境变量
```bash
export HMAC_SECRET_KEY="dev-secret-key-change-in-prod"
export KAFKA_BROKERS="localhost:9092"
export REDIS_ADDR="localhost:6379"
export SUI_RPC_URL="https://fullnode.testnet.sui.io"
export SUI_SIGNER_PRIVATE_KEY="suiprivkey..."
export SUI_GAS_OBJECT_ID="0x..."
export SUI_GAS_BUDGET="10000000"
```

### 3. 启动网关
```bash
go run cmd/gateway/main.go
```

### 4. 打开 Dashboard
```bash
open web/dashboard.html
```

### 5. 运行 Demo
```bash
./scripts/demo/run_demo.sh
```

## 📋 评委演示清单

- [ ] 展示架构图（README.md）
- [ ] 启动 Dashboard（web/dashboard.html）
- [ ] 检查健康状态（curl /health）
- [ ] 运行 Analyst Agent
- [ ] 运行 Trader Agent
- [ ] 展示 Sui Explorer 交易
- [ ] 强调关键创新点

## 🏆 获奖关键点

### 技术创新
1. **HMAC 认证** - 无需私钥托管
2. **PTB 原子执行** - 多步骤一次完成
3. **Walrus 集成** - AI 上下文去中心化存储

### 生产就绪
1. Kafka 异步队列
2. Redis 状态缓存
3. 优雅降级机制
4. 完整的错误处理

### Sui 生态集成
1. PTB 多方结算
2. Walrus 存储
3. Move 合约（MemoryObject）

## 📞 支持

问题？查看 docs/DEMO_SCRIPT.md 获取详细演示脚本。
