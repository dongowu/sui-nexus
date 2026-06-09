# Sui Overflow 2026 — Demo Video Script & Recording Guide

> This guide helps you create a compelling 3-minute demo video for the Sui Overflow 2026 submission.

## Pre-Recording Checklist

- [ ] Judge demo mode: `HACKATHON_DEMO_MODE=true ./scripts/demo/run_agent_wallet_demo.sh`
- [ ] Live smoke mode: `bash scripts/demo/live_testnet_smoke.sh`
- [ ] Dashboard open: `open web/dashboard.html`
- [ ] Sui Explorer tab open for on-chain verification

## Video Structure (Total: 3 minutes)

### Part 1: The Problem (30 seconds)

**Narration**:
> "AI agents can generate trading strategies. But who holds the keys? Who sets the limits? Who audits the trail?"

---

### Part 2: The Solution (20 seconds)

**Narration**:
> "Sui-Nexus is the settlement layer for the AI agent economy on Sui. Every agent gets a policy-controlled wallet. Budget caps. Protocol whitelists. Time windows. All enforced ON-CHAIN in Move."

---

### Part 3: Live Demo — Agent Wallet (90 seconds)

**Step 1: Start the Demo** (10 seconds)
- Run judge demo mode for local deterministic receipts, or run the live smoke script for real testnet receipts

**Step 2: Wallet Created And Funded** (20 seconds)
- Owner session creates and funds the agent wallet

**Step 3: Safe Trade Passes Guardian** (30 seconds)
- Agent submits 100 MIST with `expected_price` + `observed_price` → Guardian passes → trade executed

**Step 4: Overspend Blocked** (30 seconds) ⭐
- Agent Bravo attempts 600 MIST → Guardian intercepts
- **This is the key moment!**

---

### Part 4: Walrus Memory (30 seconds)

**Narration**:
> "AI agents are stateless today. Sui-Nexus fixes this with Walrus persistent memory."

Run: `python3 scripts/demo/walrus_memory_demo.py`

---

### Part 5: Chain Verification (30 seconds)

**Show**: Explorer with contract address `0xa051bbf9517d8ee94f2339e69877e4eacec38d3f4893b0aedf84774d18c54433`

---

## Recording Tips

### What to INCLUDE
- ✅ "Overspend blocked" moment (key differentiator)
- ✅ Explorer chain verification
- ✅ Both tracks demonstrated
- ✅ `wallet_id` returned from create response

### What to AVOID
- ❌ Errors in terminal
- ❌ Rushing through demo
- ❌ No chain verification

---

## Post-Recording Checklist

- [ ] Video is under 3 minutes
- [ ] Overspend blocked scene included
- [ ] Explorer verification shown
- [ ] Export as MP4 (H.264)
