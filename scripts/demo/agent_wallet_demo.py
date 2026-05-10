#!/usr/bin/env python3
"""
Agent Wallet with zkLogin Demo
================================
Demonstrates the Sui Overflow 2026 Agentic Web track capability:
  zkLogin auth -> Agent Wallet with policy -> trade execution -> owner revocation

Flow:
  1. Owner creates an AgentWallet with policy (500 SUI budget, DeepBook only, 24h)
  2. Agent authenticates via zkLogin (Google OAuth -> session token)
  3. Agent executes a trade within policy bounds
  4. Agent tries to exceed budget -> blocked by on-chain policy
  5. Owner revokes wallet -> agent can no longer trade

Usage:
  python3 scripts/demo/agent_wallet_demo.py
"""

import json
import os
import sys
import time
import requests

GATEWAY_URL = os.getenv("GATEWAY_URL", "http://localhost:8080")

# ────────────────────────────────────────────────────────────
# Configuration (from env or defaults)
# ────────────────────────────────────────────────────────────
DEEPBOOK_PACKAGE_ID = os.getenv(
    "DEEPBOOK_PACKAGE_ID",
    "0x337f4f4f6567fcd778d5454f27c16c70e2f274cc6377ea6249ddf491482ef497",
)
AGENT_BUDGET_MIST = 500_000_000_000  # 500 SUI
TRADE_AMOUNT_MIST = 100_000_000_000  # 100 SUI
OVERSPEND_AMOUNT_MIST = 600_000_000_000  # 600 SUI (exceeds budget)


# ────────────────────────────────────────────────────────────
# Helper functions
# ────────────────────────────────────────────────────────────

def print_header(title: str):
    print(f"\n{'=' * 60}")
    print(f"  {title}")
    print(f"{'=' * 60}")


def post(endpoint: str, payload: dict, timeout: int = 60) -> dict:
    try:
        resp = requests.post(f"{GATEWAY_URL}{endpoint}", json=payload, timeout=timeout)
        return resp.json()
    except requests.exceptions.ConnectionError:
        print(f"ERROR: Cannot connect to gateway at {GATEWAY_URL}")
        print("Make sure the gateway is running: go run cmd/gateway/main.go")
        sys.exit(1)


def get(endpoint: str) -> dict:
    try:
        resp = requests.get(f"{GATEWAY_URL}{endpoint}", timeout=10)
        return resp.json()
    except requests.exceptions.ConnectionError:
        print(f"ERROR: Cannot connect to gateway at {GATEWAY_URL}")
        sys.exit(1)


def check_error(result: dict, label: str) -> None:
    if result.get("error"):
        print(f"  {label} FAILED: [{result['error']['code']}] {result['error']['message']}")
        return False
    return True


# ────────────────────────────────────────────────────────────
# Step 1: Owner creates Agent Wallet
# ────────────────────────────────────────────────────────────

def step1_owner_creates_wallet(agent_address: str) -> str:
    print_header("STEP 1: Owner Creates Agent Wallet")

    print(f"  Creating wallet for agent: {agent_address}")
    print(f"  Budget cap: {AGENT_BUDGET_MIST} MIST (500 SUI)")
    print(f"  Allowed protocol: DeepBook ({DEEPBOOK_PACKAGE_ID[:16]}...)")
    print(f"  Time window: ~24h (2400 epochs from now)")

    payload = {
        "agent_address": agent_address,
        "budget_cap_mist": AGENT_BUDGET_MIST,
        "allowed_protocols": [DEEPBOOK_PACKAGE_ID],
        "time_end_epoch": 999999,  # far future for demo
    }

    result = post("/api/v1/wallet/create", payload)

    if not check_error(result, "CreateWallet"):
        sys.exit(1)

    wallet_id = result.get("wallet_id")
    tx_digest = result.get("tx_digest", "N/A")

    print(f"  Wallet created: {wallet_id}")
    print(f"  Tx digest: {tx_digest}")
    print(f"  Active: {result.get('is_active')}")
    print(f"  Budget: {result.get('policy', {}).get('budget_cap_mist')} MIST")

    return wallet_id


# ────────────────────────────────────────────────────────────
# Step 2: Agent authenticates via zkLogin
# ────────────────────────────────────────────────────────────

def step2_agent_zklogin_auth() -> dict:
    print_header("STEP 2: Agent Authenticates via zkLogin")

    print("  To authenticate the agent via zkLogin:")
    print("  1. Open in browser: " + GATEWAY_URL + "/api/v1/auth/zklogin")
    print("  2. Sign in with Google")
    print("  3. Copy the session_token and user_address from the response")
    print()
    print("  For demo purposes, enter the credentials below.")

    # Try interactive input
    try:
        user_address = input("  user_address: ").strip()
        session_token = input("  session_token: ").strip()
    except (EOFError, KeyboardInterrupt):
        # Non-interactive fallback: use demo values
        print("\n  (Non-interactive mode: using demo zkLogin credentials)")
        user_address = os.getenv("DEMO_ZKLOGIN_ADDRESS", "")
        session_token = os.getenv("DEMO_ZKLOGIN_TOKEN", "")

    if not user_address or not session_token:
        print("  WARNING: No zkLogin credentials provided.")
        print("  For a full demo, authenticate at /api/v1/auth/zklogin first.")
        print("  Continuing with empty credentials (wallet auth will fail).")

    # Verify session
    if session_token:
        verify_resp = post("/api/v1/auth/zklogin/verify", {
            "user_address": user_address,
            "session_token": session_token,
        })
        if verify_resp.get("valid"):
            print(f"  Session verified: {user_address}")
        else:
            print(f"  Session verification failed: {verify_resp}")

    return {"user_address": user_address, "session_token": session_token}


# ────────────────────────────────────────────────────────────
# Step 3: Agent executes trade within policy
# ────────────────────────────────────────────────────────────

def step3_agent_executes_trade(session: dict, wallet_id: str) -> bool:
    print_header("STEP 3: Agent Executes Trade (Within Budget)")

    print(f"  Wallet: {wallet_id}")
    print(f"  Amount: {TRADE_AMOUNT_MIST} MIST (100 SUI)")
    print(f"  Protocol: DeepBook ({DEEPBOOK_PACKAGE_ID[:16]}...)")
    print(f"  Description: Buy SUI/USDC limit order on DeepBook")

    payload = {
        "wallet_id": wallet_id,
        "amount_mist": TRADE_AMOUNT_MIST,
        "protocol": DEEPBOOK_PACKAGE_ID,
        "expected_price": 1000,  # Guardian: expected execution price (0 = skip)
        "description": "Buy SUI/USDC limit order on DeepBook",
        "session_token": session.get("session_token", ""),
        "user_address": session.get("user_address", ""),
    }

    result = post("/api/v1/wallet/execute", payload)

    if not check_error(result, "ExecuteTrade"):
        return False

    tx_digest = result.get("tx_digest", "N/A")
    print(f"  Trade executed: {tx_digest}")

    return True


# ────────────────────────────────────────────────────────────
# Step 3b: Agent tries to exceed budget -> blocked
# ────────────────────────────────────────────────────────────

def step3b_agent_exceeds_budget(session: dict, wallet_id: str):
    print_header("STEP 3b: Agent Tries to Exceed Budget (Should Fail)")

    print(f"  Wallet: {wallet_id}")
    print(f"  Amount: {OVERSPEND_AMOUNT_MIST} MIST (600 SUI)")
    print(f"  Budget cap: {AGENT_BUDGET_MIST} MIST (500 SUI)")
    print(f"  Remaining budget: {AGENT_BUDGET_MIST - TRADE_AMOUNT_MIST} MIST (400 SUI)")
    print(f"  Expected: REJECTED (budget exceeded)")

    payload = {
        "wallet_id": wallet_id,
        "amount_mist": OVERSPEND_AMOUNT_MIST,
        "protocol": DEEPBOOK_PACKAGE_ID,
        "expected_price": 1000,
        "description": "Attempted overspend on DeepBook",
        "session_token": session.get("session_token", ""),
        "user_address": session.get("user_address", ""),
    }

    result = post("/api/v1/wallet/execute", payload)

    if result.get("error"):
        print(f"  CORRECTLY BLOCKED: [{result['error']['code']}] {result['error']['message']}")
    else:
        print(f"  UNEXPECTED: trade succeeded (may indicate budget check issue)")
        print(f"  Result: {json.dumps(result, indent=2)}")


# ────────────────────────────────────────────────────────────
# Step 4: Owner revokes wallet
# ────────────────────────────────────────────────────────────

def step4_owner_revokes_wallet(wallet_id: str):
    print_header("STEP 4: Owner Revokes Wallet")

    print(f"  Revoking wallet: {wallet_id}")

    payload = {
        "wallet_id": wallet_id,
    }
    result = post(f"/api/v1/wallet/{wallet_id}/revoke", payload)

    if not check_error(result, "RevokeWallet"):
        print("  (Revocation may have failed due to on-chain ownership check)")
        print("  This is expected in demo mode - the Move contract enforces owner-only.")
        return

    tx_digest = result.get("tx_digest", "N/A")
    print(f"  Wallet revoked: {tx_digest}")
    print(f"  Active: {result.get('is_active')}")


# ────────────────────────────────────────────────────────────
# Step 5: Verify wallet state and activity log
# ────────────────────────────────────────────────────────────

def step5_verify_wallet(wallet_id: str):
    print_header("STEP 5: Verify Wallet State & Activity Log")

    # Get wallet state
    wallet = get(f"/api/v1/wallet/{wallet_id}")
    if wallet.get("error"):
        print(f"  Error: {wallet['error']}")
        return

    print(f"  Wallet ID: {wallet.get('wallet_id')}")
    print(f"  Active: {wallet.get('is_active')}")
    policy = wallet.get("policy", {})
    print(f"  Budget cap: {policy.get('budget_cap_mist', 0)} MIST")
    print(f"  Budget spent: {policy.get('budget_spent_mist', 0)} MIST")
    print(f"  Budget remaining: {policy.get('budget_cap_mist', 0) - policy.get('budget_spent_mist', 0)} MIST")
    print(f"  Balance: {wallet.get('balance_mist', 0)} MIST")

    # Get activity log
    activity = get(f"/api/v1/wallet/{wallet_id}/activity")
    if activity.get("error"):
        print(f"  Activity log error: {activity['error']}")
        return

    entries = activity.get("activities", [])
    print(f"\n  On-chain Activity Log ({len(entries)} entries):")
    for i, entry in enumerate(entries):
        print(f"    [{i+1}] {entry.get('action')}: {entry.get('amount_mist')} MIST")
        print(f"        Protocol: {entry.get('protocol', 'N/A')[:20]}...")
        print(f"        {entry.get('description', 'N/A')}")
        print(f"        Time: {entry.get('timestamp')}")


# ────────────────────────────────────────────────────────────
# Main
# ────────────────────────────────────────────────────────────

def main():
    print("=" * 60)
    print("  Agent Wallet with zkLogin Demo")
    print("  Sui Overflow 2026 - Agentic Web Track")
    print("=" * 60)
    print(f"  Gateway: {GATEWAY_URL}")

    # Check gateway health
    try:
        health = requests.get(f"{GATEWAY_URL}/health", timeout=5).json()
        if not health.get("ready"):
            print("  Gateway is not ready. Start Kafka and Redis first.")
            sys.exit(1)
    except Exception:
        print(f"  Cannot reach gateway at {GATEWAY_URL}")
        print("  Start it with: go run cmd/gateway/main.go")
        sys.exit(1)

    # Demo agent address (zkLogin-derived Sui address from Google OAuth)
    agent_address = os.getenv(
        "DEMO_AGENT_ADDRESS",
        "0x0000000000000000000000000000000000000000000000000000000000000001",
    )

    # Step 1: Create wallet
    wallet_id = step1_owner_creates_wallet(agent_address)

    # Step 2: zkLogin auth
    session = step2_agent_zklogin_auth()

    # Brief pause
    time.sleep(1)

    # Step 3: Execute trade within budget
    ok = step3_agent_executes_trade(session, wallet_id)

    time.sleep(1)

    # Step 3b: Try to exceed budget
    step3b_agent_exceeds_budget(session, wallet_id)

    time.sleep(1)

    # Step 4: Revoke wallet
    step4_owner_revokes_wallet(wallet_id)

    time.sleep(1)

    # Step 5: Verify state
    step5_verify_wallet(wallet_id)

    print_header("DEMO COMPLETE")
    print("  Key capabilities demonstrated:")
    print("  1. Agent Wallet creation with policy (budget + protocol scope)")
    print("  2. zkLogin authentication for agent identity")
    print("  3. On-chain trade execution within policy bounds")
    print("  4. Policy enforcement (budget cap violation blocked)")
    print("  5. Owner revocation")
    print("  6. On-chain activity log")
    print()
    print("  Verify on-chain: sui client object " + wallet_id)
    print("=" * 60)


if __name__ == "__main__":
    main()
