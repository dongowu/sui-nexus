# Sui Overflow 2026 — DoraHacks Submission Guide

## Project Information

| Field | Value |
|-------|-------|
| **Project Name** | Sui-Nexus |
| **Tracks** | Agentic Web (Intent Engine) + Walrus |
| **Repository** | (Your GitHub URL) |
| **Demo Video** | (YouTube/Cloud storage URL) |
| **Contract Address** | `0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d` |

---

## Project Description (English)

**Sui-Nexus** provides settlement infrastructure for AI agents on Sui blockchain. It delivers policy-controlled wallets where budget caps, protocol scope, and time windows are enforced on-chain by Move smart contracts — not middleware. Agents authenticate via HMAC signatures and zkLogin (Google OAuth + ZK proof), eliminating private key custody risks. A dual-layer Guardian risk engine pre-checks slippage and concentration before any transaction reaches the Move VM. Walrus integration enables cross-agent persistent memory, making multi-agent collaboration verifiable and stateful.

**Innovation**: Not another AI agent that trades — the settlement infrastructure every AI agent on Sui needs. Policy enforcement lives in Move, not config files. The gateway is a relay, not an authority.

---

## Tracks Covered

### Track 1: Agentic Web — Intent Engine

Demonstrates 9/9 requirements:
- [x] zkLogin + Move policy objects
- [x] Budget cap enforcement (on-chain)
- [x] Protocol scope (DeepBook only)
- [x] Time window enforcement
- [x] Agent autonomous execution
- [x] Real DeepBook order placement
- [x] Self-enforcing budget caps (blocked overspend)
- [x] On-chain activity log
- [x] Owner revocation demo

### Track 2: Walrus — AI Agent Memory System

Demonstrates 7/7 requirements:
- [x] Persistent data via Walrus (write + read)
- [x] Move MemoryObject for on-chain blob references
- [x] Long-running workflow: Analyst → Walrus → Trader
- [x] Multi-agent collaboration: shared memory across agents
- [x] Artifact-driven workflow: reports stored, retrieved, reused

---

## On-Chain Verification

| Contract | Testnet Address | Explorer |
|----------|----------------|----------|
| Package | `0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d` | [View](https://suiexplorer.com/object/0x28c35c355590d81c80f86b43b42d21041fdbc0ab34546ff558b48270a4ff277d?network=testnet) |
| Upgrade Cap | `0x7bd41eb7253f93e03f84fe2c963347b62a5cae57a29c8200c92e9a4c6bbfb06b` | [View](https://suiexplorer.com/object/0x7bd41eb7253f93e03f84fe2c963347b62a5cae57a29c8200c92e9a4c6bbfb06b?network=testnet) |

---

## Submission Checklist

- [ ] GitHub repository is public
- [ ] README.md has architecture, tracks, quick start
- [ ] Demo video recorded (< 3 minutes)
- [ ] DoraHacks project page filled out
- [ ] All tests pass: `go test ./...`
