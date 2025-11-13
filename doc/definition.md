# Lucendex — Project Documentation

This document aggregates four foundational planning artifacts: value proposition, MVP scope, go‑to‑market, and revenue model. It serves as a developer reference and strategic planning base.

---

## 1) Value Proposition — One Pager

**Statement**
Lucendex is a neutral, non‑custodial execution layer for XRPL trading — a deterministic routing, quoting, and settlement infrastructure that wallets, fintechs, funds, and exchanges can integrate to access deep XRPL liquidity without building DEX infrastructure themselves.

**Problems We Solve**

* Existing XRPL DEX UIs are not infrastructure‑grade (no deterministic quote binding, no circuit breakers, no routing across liquidity).
* CEXs dominate price discovery and liquidation cascades, increasing systemic fragility.
* Institutional integrators need a safe, auditable execution layer rather than another retail‑facing DEX.

**Core Design Pillars**

* Non‑custodial by design — never hold user funds.
* Deterministic execution — quote hash + TTL binding.
* Safety rails — circuit breakers, sanity checks, fallback broadcast.
* Neutral infrastructure — not competing as a DEX brand or token casino.

**Primary Customers**

* Wallets & frontends (embed Lucendex routing)
* Funds & market‑makers (best execution + API)
* Token issuers (XRPL liquidity access)
* Custodians & fintechs (non‑custodial trading rails)

---

## 2) MVP Scope — Thin‑Trade Bootstrap UI + Core Engine

**In‑Scope (must ship)**

* XRPL connect (read & submit)
* Real‑time indexer for AMM + orderbooks
* Router (AMM + book best‑path)
* Quote → hash + TTL binding
* Thin‑trade UI for single‑hop swaps via wallet signature
* Optional relay (signed‑blob forward only)
* Circuit break checks (price delta guard)
* Logging & metrics (API + trades + rejects)

**Out‑of‑Scope (explicitly NOT in V1)**

* No leverage / no derivatives
* No custody
* No own token
* No incentives / liquidity mining
* No cross‑chain in V1
* No listing fees / launchpad features

**V1 Acceptance Criteria**

* A trade can be quoted, signed, and executed end‑to‑end via UI
* Deterministic quote reproducibility verified
* Safety killswitch rejects extreme mispriced trades
* API consumers can integrate without UI

---

## 3) Go‑To‑Market — First Customers & Approach

**Target: Infrastructure Buyers — not retail traders**

1. **Wallets** (GemWallet, Xumm competitors, custodial wallets)
   Sell: embedded routing instead of building their own.
2. **Funds / bots**
   Sell: deterministic execution API + fallback relay + route quality.
3. **Token Issuers on XRPL**
   Sell: liquidity routing + credibility for their trading pairs.
4. **Fintech integrations**
   Sell: non‑custodial execution pipe for compliant products.

**Acquisition Strategy**

* Build credibility with technical whitepaper + reference UI
* Ship public API and demo client — "copy/paste to integrate"
* Private outreach to top XRPL wallets & LPs
* Publish stress‑resilience case study vs CEX crash behavior
* No retail marketing, no token airdrop, no hype campaigns

---

## 4) Revenue Model — Neutral Infra Aligned

**Primary revenue streams**

1. **Routing Fee (bps)** on executed swaps routed via Lucendex engine
2. **Premium API** (higher rate limits / SLAs / priority routing)
3. **Enterprise integration** (wallet/fintech SDK integration fees)
4. **Future — cross‑chain routing fee** once sidechains/bridges added

**Intentionally excluded in early phases**

* No token issuance
* No pay‑to‑list model
* No retail fee schemes
* No leverage liquidations revenue

**Why this is credible for compliance & partnerships**

* Revenue is for infrastructure service — not speculation
* Neutral alignment (no proprietary trading against users)
* Transparent economics — easy for partners to justify

---

*End of initial doc — next step is to attach technical architecture and execution milestones under this same document in subsequent updates.*
