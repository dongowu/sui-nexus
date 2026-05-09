#!/usr/bin/env python3
"""Analyst Agent - AI agent that analyzes market data and generates trading reports.

Uses real LLM (OpenAI/Groq) for market analysis.
Set LLM_PROVIDER and API key via environment variables.
"""

import hashlib
import hmac
import json
import os
import time
import requests

API_KEY = "analyst-agent-key"
SECRET_KEY = os.getenv("HMAC_SECRET_KEY", "dev-secret-key-change-in-prod").encode()
GATEWAY_URL = os.getenv("GATEWAY_URL", "http://localhost:8080")

# Lazy-load LLM client to avoid import errors if not installed
_llm_client = None

def get_llm_client():
    """Get or create the LLM client instance."""
    global _llm_client
    if _llm_client is None:
        from llm_client import LLMClient
        _llm_client = LLMClient()
    return _llm_client

def sign_message(task_id: str, timestamp: int, action: str, amount: str) -> str:
    message = f"{task_id}:{timestamp}:{action}:{amount}"
    return hmac.new(SECRET_KEY, message.encode(), hashlib.sha256).hexdigest()

def generate_report(news: str) -> dict:
    """Analyze market news using real LLM and return trading recommendation.

    Args:
        news: Market news text to analyze

    Returns:
        dict with keys: sentiment, confidence, action, target_tokens, reason
    """
    try:
        client = get_llm_client()
        return client.analyze_market_news(news)
    except ImportError:
        # Fallback to simulated analysis if LLM not available
        print("⚠️ LLM client not available, using simulated analysis")
        if "hack" in news.lower() or "attack" in news.lower():
            return {
                "sentiment": "bearish",
                "confidence": 0.85,
                "action": "sell",
                "target_tokens": ["SUI", "USDT"],
                "reason": f"Negative news detected: {news}"
            }
        return {
            "sentiment": "bullish",
            "confidence": 0.65,
            "action": "buy",
            "target_tokens": ["SUI"],
            "reason": f"Positive news: {news}"
        }

def submit_intent(action: str, amount: str, context: dict) -> str:
    import uuid
    task_id = str(uuid.uuid4())
    timestamp = int(time.time())

    payload = {
        "task_id": task_id,
        "action": action,
        "params": {
            "amount": amount,
            "token_in": "USDT",
            "token_out": "SUI",
            "slippage": "0.5"
        },
        "agents": [
            {"address": "0xAnalystWalletAddress", "share": 0.1}
        ],
        "context_payload": __import__("base64").b64encode(
            json.dumps(context).encode()
        ).decode()
    }

    resp = requests.post(
        f"{GATEWAY_URL}/api/v1/intent",
        json=payload,
        headers={
            "X-API-Key": API_KEY,
            "X-Signature": sign_message(task_id, timestamp, action, amount),
            "X-Timestamp": str(timestamp),
        },
        timeout=10,
    )
    if not resp.ok:
        raise RuntimeError(f"gateway rejected intent: {resp.status_code} {resp.text}")
    resp.raise_for_status()
    return resp.json()["task_id"]

if __name__ == "__main__":
    print("🤖 Analyst Agent Started")
    print("Waiting for market news input...")

    # Simulate receiving news
    news = input("\n📰 Enter market news (e.g., 'Protocol X suffered a flash loan attack'): ")

    print(f"📊 Analyzing: {news}")
    report = generate_report(news)
    print(f"📋 Analysis Result: {json.dumps(report, indent=2)}")

    if report["action"] == "sell":
        print("📤 Submitting sell intent to gateway...")
        task_id = submit_intent("Swap", "1000", report)
        print(f"✅ Task submitted: {task_id}")
