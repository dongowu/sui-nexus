# Sui Overflow 2026 Demo Script

## 🎯 3-Minute Pitch Structure

### Part 1: Problem (30 seconds)

**Script:**
> "AI agents are becoming autonomous economic actors, but they face a critical challenge: **how to safely execute on-chain transactions without exposing private keys**.
>
> Current solutions either require agents to hold keys directly (security risk) or use centralized custodians (trust issue). We need infrastructure-grade settlement for multi-agent collaboration."

**Visual:** Show diagram of traditional approach vs. Sui-Nexus

---

### Part 2: Solution (60 seconds)

**Script:**
> "Sui-Nexus is a **multi-agent asynchronous settlement gateway** built on Sui blockchain. Here's how it works:
>
> 1. **HMAC Authentication** - Agents sign intents with API keys, no private key exposure
> 2. **Kafka Queue** - Asynchronous processing for high throughput
> 3. **PTB (Programmable Transaction Blocks)** - Atomic multi-party settlements
> 4. **Walrus Storage** - Decentralized AI context/logs
>
> The key innovation: agents submit **intents**, not transactions. The gateway builds atomic PTBs that execute swaps, distribute rewards, and store AI context—all in one transaction."

**Visual:** Architecture diagram with data flow

---

### Part 3: Live Demo (90 seconds)

**Script:**
> "Let me show you a real scenario: **Breaking news triggers coordinated trading**."

#### Step 1: Check Gateway Health (10s)
```bash
curl http://localhost:8080/health
```

**Expected Output:**
```json
{
  "status": "healthy",
  "ready": true,
  "components": {
    "queue": {"ready": true, "required": true},
    "storage": {"ready": true, "required": false}
  }
}
```

#### Step 2: Analyst Agent Analyzes News (30s)
```bash
python3 scripts/demo/analyst_agent.py
```

**Input:** `Protocol X suffered a flash loan attack, token price dropped 40%`

**Output:**
```json
{
  "sentiment": "bearish",
  "confidence": 0.85,
  "action": "sell",
  "reason": "Negative news detected: Protocol X suffered..."
}
```

**Script:** "The analyst agent detects bearish sentiment and submits a sell intent to the gateway."

#### Step 3: Trader Agent Executes (20s)
```bash
python3 scripts/demo/trader_agent.py
```

**Script:** "The trader agent receives the analysis and executes the trade through our gateway."

#### Step 4: Show Sui Transaction (30s)
```bash
# Query task status
curl http://localhost:8080/api/v1/task/{task_id}
```

**Expected Output:**
```json
{
  "task_id": "abc-123",
  "status": "completed",
  "tx_digest": "8xK9mN...",
  "blob_id": "walrus://..."
}
```

**Script:**
> "Here's the magic: **One atomic transaction** on Sui that:
> - Executed the swap
> - Distributed rewards to both agents (10% analyst, 20% trader)
> - Stored the AI analysis context on Walrus
>
> Let's verify on Sui Explorer..."

**Open Browser:** `https://suiexplorer.com/txblock/{tx_digest}?network=testnet`

---

## 🎬 Demo Preparation Checklist

### Before Demo:
- [ ] Start Kafka: `docker run -d -p 9092:9092 apache/kafka`
- [ ] Start Redis: `redis-server`
- [ ] Fund Sui testnet account
- [ ] Set environment variables:
  ```bash
  export SUI_SIGNER_PRIVATE_KEY="suiprivkey..."
  export SUI_GAS_OBJECT_ID="0x..."
  export KAFKA_BROKERS="localhost:9092"
  export REDIS_ADDR="localhost:6379"
  ```
- [ ] Start gateway: `go run cmd/gateway/main.go`
- [ ] Test health endpoint
- [ ] Prepare browser with Sui Explorer open

### Backup Plan (if live demo fails):
- Pre-recorded video showing successful execution
- Screenshots of Sui Explorer transaction
- Prepared transaction digest to show

---

## 💡 Key Talking Points

### Technical Innovation:
1. **HMAC-based auth** - No private key custody needed
2. **PTB atomic execution** - Multi-step operations in one transaction
3. **Walrus integration** - Decentralized AI context storage
4. **Production-ready** - Kafka queue, Redis cache, graceful degradation

### Business Value:
1. **Security** - Agents never hold private keys
2. **Scalability** - Async queue handles high throughput
3. **Transparency** - All operations verifiable on-chain
4. **Composability** - Standard HTTP API for any AI framework

### Sui-Specific Features:
1. **PTB power** - Atomic multi-party settlements
2. **Walrus storage** - Perfect for AI context/logs
3. **Fast finality** - Sub-second transaction confirmation
4. **Low cost** - Efficient gas usage

---

## 🎤 Q&A Preparation

**Q: How do you prevent replay attacks?**
A: HMAC signatures include timestamps with a 5-minute window. Old signatures are rejected.

**Q: What if Kafka goes down?**
A: Health endpoint returns 503, and intent submissions are rejected immediately. No silent failures.

**Q: Can this scale to thousands of agents?**
A: Yes - Kafka handles high throughput, and we can horizontally scale gateway instances.

**Q: How do you handle failed transactions?**
A: Tasks are stored in Redis with retry logic. Failed tasks can be reprocessed.

**Q: Why not use account abstraction?**
A: HMAC auth is simpler and doesn't require on-chain state. Agents can start immediately with just an API key.

---

## 📊 Success Metrics to Highlight

- ✅ **Sub-second latency** from intent to Sui transaction
- ✅ **Zero private key exposure** for AI agents
- ✅ **Atomic execution** - all-or-nothing guarantees
- ✅ **Production-ready** - comprehensive error handling and monitoring
- ✅ **Sui-native** - leverages PTB and Walrus

---

## 🏆 Closing Statement (10 seconds)

**Script:**
> "Sui-Nexus makes AI agents first-class citizens on Sui. We're not just building a demo—we're building the infrastructure for the autonomous economy. Thank you!"
