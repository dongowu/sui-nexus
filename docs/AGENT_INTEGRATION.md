# Agent 接入指南

## 🤖 如何让你的 AI Agent 接入 Sui-Nexus

### 核心概念

AI Agent 通过 **HTTP API** 提交"意图"，无需持有私钥。

### 接入步骤

## 1️⃣ 获取 API Key 和 Secret

```python
API_KEY = "your-agent-name"
SECRET_KEY = b"your-secret-key"  # 与网关共享
```

## 2️⃣ 生成 HMAC 签名

```python
import hashlib
import hmac
import time

def sign_intent(task_id, timestamp, action, amount, secret_key):
    """生成 HMAC-SHA256 签名"""
    message = f"{task_id}:{timestamp}:{action}:{amount}"
    return hmac.new(secret_key, message.encode(), hashlib.sha256).hexdigest()

# 示例
task_id = "task-001"
timestamp = int(time.time())
action = "Transfer"
amount = "1000"

signature = sign_intent(task_id, timestamp, action, amount, SECRET_KEY)
```

## 3️⃣ 提交意图到网关

```python
import requests
import json
import base64

GATEWAY_URL = "http://localhost:8080"

def submit_intent(task_id, action, amount, recipient, context_data=None):
    """提交意图到 Sui-Nexus 网关"""
    timestamp = int(time.time())
    signature = sign_intent(task_id, timestamp, action, amount, SECRET_KEY)
    
    # 构建请求
    payload = {
        "task_id": task_id,
        "action": action,  # "Transfer" 或 "Swap"
        "params": {
            "amount": amount,
            "dest_addr": recipient  # Transfer 需要
        },
        "agents": [
            {"address": "0xYourAgentAddress", "share": 0.1}
        ]
    }
    
    # 可选：添加 AI 上下文
    if context_data:
        payload["context_payload"] = base64.b64encode(
            json.dumps(context_data).encode()
        ).decode()
    
    headers = {
        "Content-Type": "application/json",
        "X-API-Key": API_KEY,
        "X-Signature": signature,
        "X-Timestamp": str(timestamp)
    }
    
    response = requests.post(
        f"{GATEWAY_URL}/api/v1/intent",
        json=payload,
        headers=headers
    )
    
    return response.json()

# 使用示例
result = submit_intent(
    task_id="task-001",
    action="Transfer",
    amount="1000",
    recipient="0xRecipientAddress",
    context_data={"reason": "Payment for service"}
)

print(f"Task ID: {result['task_id']}")
print(f"Status: {result['status']}")
```

## 4️⃣ 查询任务状态

```python
def get_task_status(task_id):
    """查询任务执行状态"""
    response = requests.get(
        f"{GATEWAY_URL}/api/v1/task/{task_id}",
        headers={
            "X-API-Key": API_KEY,
            "X-Signature": "dummy",  # GET 请求也需要认证
            "X-Timestamp": str(int(time.time()))
        }
    )
    return response.json()

# 查询状态
status = get_task_status("task-001")
print(f"Status: {status['status']}")
if status.get('tx_digest'):
    print(f"Sui TX: https://suiscan.xyz/txblock/{status['tx_digest']}?network=testnet")
```

## 5️⃣ 实时监听（WebSocket）

```python
import websocket
import json

def on_message(ws, message):
    task = json.loads(message)
    print(f"Task {task['task_id']} updated: {task['status']}")
    if task.get('tx_digest'):
        print(f"TX Digest: {task['tx_digest']}")

ws = websocket.WebSocketApp(
    "ws://localhost:8080/ws",
    on_message=on_message
)
ws.run_forever()
```

## 📋 完整示例：交易 Agent

```python
import hmac
import hashlib
import time
import requests
import uuid

class TradingAgent:
    def __init__(self, api_key, secret_key, gateway_url="http://localhost:8080"):
        self.api_key = api_key
        self.secret_key = secret_key.encode()
        self.gateway_url = gateway_url
        self.agent_address = "0xYourAgentAddress"
    
    def sign(self, task_id, timestamp, action, amount):
        message = f"{task_id}:{timestamp}:{action}:{amount}"
        return hmac.new(self.secret_key, message.encode(), hashlib.sha256).hexdigest()
    
    def transfer(self, recipient, amount, reason=None):
        task_id = str(uuid.uuid4())
        timestamp = int(time.time())
        signature = self.sign(task_id, timestamp, "Transfer", amount)
        
        payload = {
            "task_id": task_id,
            "action": "Transfer",
            "params": {"amount": amount, "dest_addr": recipient},
            "agents": [{"address": self.agent_address, "share": 0.1}]
        }
        
        headers = {
            "X-API-Key": self.api_key,
            "X-Signature": signature,
            "X-Timestamp": str(timestamp)
        }
        
        response = requests.post(
            f"{self.gateway_url}/api/v1/intent",
            json=payload,
            headers=headers
        )
        return response.json()

# 使用
agent = TradingAgent("trader-001", "secret-key")
result = agent.transfer("0xRecipient", "1000")
print(f"Submitted: {result['task_id']}")
```

## 🔑 支持的操作类型

### Transfer（转账）
```python
{
    "action": "Transfer",
    "params": {
        "amount": "1000",
        "dest_addr": "0x..."
    }
}
```

### Swap（交易）
```python
{
    "action": "Swap",
    "params": {
        "amount": "1000",
        "token_in": "USDT",
        "token_out": "SUI",
        "slippage": "0.5"
    }
}
```

## ⚠️ 注意事项

1. **时间戳窗口**：签名有效期 5 分钟
2. **签名格式**：`task_id:timestamp:action:amount`
3. **金额单位**：以最小单位计（MIST）
4. **API Key**：与网关管理员协商获取

## 🚀 快速开始

参考 `scripts/demo/analyst_agent.py` 和 `scripts/demo/trader_agent.py` 获取完整示例。
