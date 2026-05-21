# Sui Overflow 2026 — Demo Video Script & Recording Guide

> This guide helps you create a compelling 3-minute demo video for the Sui Overflow 2026 submission.

## Pre-Recording Checklist

- [ ] Gateway running: `HACKATHON_DEMO_MODE=true go run ./cmd/gateway/main.go`
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
- Click "Start Demo" on dashboard

**Step 2: Two Wallets Created** (20 seconds)
- Agent Alpha (500 MIST) and Agent Bravo (100 MIST)

**Step 3: Safe Trade Passes Guardian** (30 seconds)
- Agent Alpha submits 100 MIST → Guardian passes → Trade executed

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

**Show**: Explorer with contract address `0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d`

---

## Recording Tips

### What to INCLUDE
- ✅ "Overspend blocked" moment (key differentiator)
- ✅ Explorer chain verification
- ✅ Both tracks demonstrated

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
