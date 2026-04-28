#!/usr/bin/env python3
"""Trader Agent - Executes trades based on analyst reports."""

import hashlib
import hmac
import json
import os
import time
import requests

API_KEY = "trader-agent-key"
SECRET_KEY = os.getenv("HMAC_SECRET_KEY", "dev-secret-key-change-in-prod").encode()
GATEWAY_URL = os.getenv("GATEWAY_URL", "http://localhost:8080")

def sign_message(task_id: str, timestamp: int, action: str, amount: str) -> str:
    message = f"{task_id}:{timestamp}:{action}:{amount}"
    return hmac.new(SECRET_KEY, message.encode(), hashlib.sha256).hexdigest()

def submit_trade(amount: str, analyst_report: dict) -> str:
    import uuid
    task_id = str(uuid.uuid4())
    timestamp = int(time.time())

    # Determine trade params based on analyst report
    token_in = "USDT"
    token_out = "SUI" if analyst_report.get("action") == "buy" else "USDT"

    payload = {
        "task_id": task_id,
        "action": "Swap",
        "params": {
            "amount": amount,
            "token_in": token_in,
            "token_out": token_out,
            "slippage": "0.5"
        },
        "agents": [
            {"address": "0xTraderWalletAddress", "share": 0.2}
        ],
        "context_payload": __import__("base64").b64encode(
            json.dumps(analyst_report).encode()
        ).decode()
    }

    resp = requests.post(
        f"{GATEWAY_URL}/api/v1/intent",
        json=payload,
        headers={
            "X-API-Key": API_KEY,
            "X-Signature": sign_message(task_id, timestamp, "Swap", amount),
            "X-Timestamp": str(timestamp),
        },
        timeout=10,
    )
    if not resp.ok:
        raise RuntimeError(f"gateway rejected intent: {resp.status_code} {resp.text}")
    resp.raise_for_status()
    return resp.json()["task_id"]

if __name__ == "__main__":
    print("🤖 Trader Agent Started")
    print("Waiting for analyst report...")

    # Simulate receiving analyst report
    report_json = input("\n📋 Paste analyst report (JSON): ")
    analyst_report = json.loads(report_json)

    print(f"📈 Executing trade based on report: {analyst_report.get('action')}")

    task_id = submit_trade("1000", analyst_report)
    print(f"✅ Trade submitted: {task_id}")

    # Check status
    time.sleep(2)
    resp = requests.get(f"{GATEWAY_URL}/api/v1/task/{task_id}", timeout=10)
    print(f"📦 Task status: {resp.json()}")
