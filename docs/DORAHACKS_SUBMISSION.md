# Sui Overflow 2026 — DoraHacks Submission Guide

## Project Information

| Field | Value |
|-------|-------|
| **Project Name** | Sui-Nexus |
| **Tracks** | Agentic Web (Intent Engine) + Walrus |
| **Repository** | (Your GitHub URL) |
| **Demo Video** | (YouTube/Cloud storage URL) |
| **Contract Address** | `0xa051bbf9517d8ee94f2339e69877e4eacec38d3f4893b0aedf84774d18c54433` |

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
| Package | `0xa051bbf9517d8ee94f2339e69877e4eacec38d3f4893b0aedf84774d18c54433` | [View](https://suiexplorer.com/object/0xa051bbf9517d8ee94f2339e69877e4eacec38d3f4893b0aedf84774d18c54433?network=testnet) |
| Upgrade Cap | `0x225f7b278c1fc2d3b5cf3d38a5f5e344463aaaf67f52a97b4a51008499a2145f` | [View](https://suiexplorer.com/object/0x225f7b278c1fc2d3b5cf3d38a5f5e344463aaaf67f52a97b4a51008499a2145f?network=testnet) |

---

## Submission Checklist

- [ ] GitHub repository is public
- [ ] README.md has architecture, tracks, quick start
- [ ] Demo video recorded (< 3 minutes)
- [ ] DoraHacks project page filled out
- [ ] All tests pass: `go test ./...`
