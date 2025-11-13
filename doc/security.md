# Lucendex — Database & Security Hardening

This document defines production security requirements for storage, network, identity, API, token-handling, and zero‑trust enforcement.

---

## 1) Threat Model (High Level)

**Trust assumptions**: only signed transactions from client wallets are trusted. Backend never sees private keys. Partner API access is authenticated and rate‑controlled. No internal actor is assumed trustworthy by default.

**Primary threats**

* DB compromise / data exfiltration
* Relay/API impersonation or replay
* Quote or route tampering
* Abuse of public endpoints (DoS, spam)
* Insider/Lateral movement
* Supply chain / dependency compromise

---

## 2) Zero‑Trust Architecture Enforcement

* All inter‑service traffic uses **mTLS** with short‑lived certs from internal CA
* No implicit network trust: private subnets + firewall allowlists per service
* Relay is **optional** and processes **signed blobs only**
* Deterministic quote binding: **QuoteHash + TTL** embedded in tx memo
* No shared secrets in code; all secrets injected via sealed store
* All services run as **non‑root**, read‑only rootfs, seccomp, no CAP_SYS_ADMIN

---

## 3) Database Layer Hardening (PostgreSQL)

**Host / FS**

* Dedicated VM, private subnet only
* Full disk encryption (LUKS)
* SELinux/AppArmor enforcing
* Encrypted offsite backups (Age/GPG)

**PG Config**

* TLS 1.3 enforced for all clients
* `password_encryption = 'scram-sha-256'`
* `fsync=on`, `full_page_writes=on`
* `pgaudit` enabled for DDL/DML audit

**Access Model**

* No superuser used by app
* Least‑privilege roles per component (indexer_rw, router_ro, api_ro)
* Separate schemas per domain (core, metering)
* RLS if tenant isolation needed

**Monitoring & Recovery**

* PITR: WAL archiving + nightly base backup
* Quarterly restore drills
* Prometheus for lag, bloat, slow queries, autovacuum

---

## 4) Token / Secret Storage & Validation

**Never store:** private keys, raw seed phrases, user wallet secrets.

**API tokens (partners)**

* Stored hashed (bcrypt/argon2id) OR stored as **Ed25519 public keys only** (preferred)
* No symmetric API keys; only request‑signed headers
* Rotation supported via versioned key table
* Revocation via `revoked` flag + short cache TTL

**Signing model**

* Partners sign canonical request (method + path + bodyhash + timestamp)
* Server verifies Ed25519 signature before processing
* Replay protection: timestamp freshness + request‑id uniqueness in KV TTL bucket

---

## 5) API Security — Public vs Partner

**Public endpoints (`/public/*`)**

* Read‑only data & quote demo
* Global rate limits (KV token bucket)
* No state‑changing ops

**Partner endpoints (`/partner/*`)**

* Require Ed25519‑signed requests
* Quotas per partner enforced in KV
* Usage metering written to `usage_events`
* `/partner/submit` accepts **signed XRPL blobs only**

**Error model**

* Deterministic JSON errors
* No internal traces or secrets in responses

---

## 6) Data Integrity — Deterministic Quotes

All quote responses must:

* Include routing fee
* Include TTL (LastLedgerSequence bound)
* Include canonical hash (blake2b‑256) over sorted JSON
* Must be reproducible from indexer snapshot state

Transactions submitted must:

* Embed QuoteHash in memo
* Fail if LedgerIndex ≥ TTL
* Fail if QuoteHash mismatch

---

## 7) Abuse & DoS Controls

* KV‑based rate limiting per IP + per partner
* Circuit breaker for abnormal request spikes
* Fallback to degraded (read‑only) mode under load
* Reject overly large payloads & malformed JSON
* WAF rule set for public ingress

---

## 8) Operational Security

* Immutable images (`FROM scratch`), signed (cosign), SBOM emitted
* Dependency pinning + automated CVE scans
* Separate CI/CD runner (no build on prod nodes)
* mTLS only for DB and internal RPC
* No SSH; controlled console access via bastion with MFA

---

## 9) Compliance / Privacy Posture

* No PII persistence; metrics keyed by partner_id only
* Sanctions/IP geofence hook at API edge
* Deterministic, rule‑based — no discretionary reimbursements or manual edits

---

**Status:** living document — append as design evolves.

---

## 10) Row‑Level Security (RLS) Examples

Example: isolate `usage_events` by partner

```sql
ALTER TABLE usage_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY usage_partner_isolation
ON usage_events USING (partner_id::text = current_setting('lucendex.partner_id'));
```

At session auth:

```sql
SET lucendex.partner_id = '<uuid-from-auth-layer>';
```

---

## 11) API Signing Specification

Canonical request =

```
METHOD + "
" + PATH + "
" + CANONICAL_QUERY + "
" + SHA256(body) + "
" + TIMESTAMP
```

Client computes:

```
sig = Ed25519.Sign(privKey, canonicalRequest)
```

Headers:

```
X-Partner-Id: <uuid>
X-Request-Id: <uuid>
X-Timestamp: <rfc3339>
X-Signature: base64(sig)
```

Server verifies:

1. timestamp drift < T (e.g. 60s)
2. request-id not seen in KV (anti‑replay)
3. Ed25519.Verify(pubKey, canonicalRequest, sig)
4. only then route to handler

---

## 12) Partner Quota / KV Logic

Token bucket per partner per window:

```
key = fmt("rl:partner:%s:%d", partnerID, unixWindow)
count = KV.INCR(key)
if count == 1: KV.SET_TTL(key, window)
if count > LIMIT(plan): reject 429
```

Where plan = {free, pro, enterprise}; limits loaded from DB → KV cache.

---

## 13) Incident Response Playbook (High Level)

1. **Detection**: alert on anomaly (rippled lag, SLO breach, quota bypass, signature failure spike).
2. **Contain**: enable degraded mode → disable relay → restrict to read‑only.
3. **Preserve**: snapshot logs, DB WAL, KV dump, node state.
4. **Eradicate**: patch/rollback, rotate partner keys if needed.
5. **Recover**: re‑enable by tier (partners first, public last).
6. **Post‑mortem**: timeline, contributing factors, control changes.

No discretionary reimbursements — deterministic rules only.

---

## 14) Backup & Restore SOP

**Backup**

* Nightly Postgres base backup (pgBackRest or wal‑g)
* Continuous WAL archive to encrypted bucket
* KV snapshot hourly (Raft snapshot shipped encrypted)
* Offsite copy (Age/GPG encrypted)

**Restore Drill**

```bash
# create new clean instance
# restore base + replay WAL to target LSN
# verify schema + counts + integrity
```

Run drill at least quarterly; document RTO/RPO.

---

## 15) Structured Threat Model (STRIDE Mapping)

| Threat                  | Example                       | Mitigation                                                                        |
| ----------------------- | ----------------------------- | --------------------------------------------------------------------------------- |
| **S — Spoofing**        | forged partner calls          | Ed25519 req signing + mTLS + replay guard + **mandatory 2FA for partner console** |
| **T — Tampering**       | route/fee modified in transit | QuoteHash bound in memo + TLS + mTLS                                              |
| **R — Repudiation**     | partner denies call           | signed requests + audit log + request-id                                          |
| **I — Info Disclosure** | DB dump exfil                 | FDE + least privilege + no PII                                                    |
| **D — DoS**             | flood quote/submit            | per-partner KV rate limit + circuit breaker                                       |
| **E — Elevation**       | service lateral move          | mTLS per-svc, no root, seccomp, SELinux                                           |

---

## 16) Mandatory 2‑Factor Authentication Policy

Lucendex requires **strong 2FA** wherever any human has access to privileged actions or secrets.

**Enforced locations**

* Partner dashboard / key management UI
* Admin console (deploy, revoke keys, rotate secrets)
* Any interface that can create/modify API keys
* Bastion / SSH (if any), package registry, CI/CD

**Accepted 2FA methods**

* FIDO2 / WebAuthn hardware key (**preferred**)
* TOTP (RFC6238) with per-device binding
* Push-based MFA **not allowed** without FIDO fallback

**2FA Enforcement Rules**

* 2FA is **not optional** — no fallback to password-only
* Recovery codes must be single-use and encrypted at rest
* Admin actions require **step-up MFA** even with active session
* Failed 2FA attempts rate‑limited and audited

**No SMS-based 2FA** — disallowed (SIM‑swap risk)

---

| Threat                  | Example                       | Mitigation                                  |
| ----------------------- | ----------------------------- | ------------------------------------------- |
| **S — Spoofing**        | forged partner calls          | Ed25519 req signing + mTLS + replay guard   |
| **T — Tampering**       | route/fee modified in transit | QuoteHash bound in memo + TLS + mTLS        |
| **R — Repudiation**     | partner denies call           | signed requests + audit log + request‑id    |
| **I — Info Disclosure** | DB dump exfil                 | FDE + least privilege + no PII              |
| **D — DoS**             | flood quote/submit            | per‑partner KV rate limit + circuit breaker |
| **E — Elevation**       | service lateral move          | mTLS per‑svc, no root, seccomp, SELinux     |

---

**Status:** sections 10–15 appended. This doc now covers DB security, API, quotas, RLS, IR, backups, threat model.
