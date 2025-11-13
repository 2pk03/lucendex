# Lucendex

[![Tests](https://github.com/lucendex/lucendex/actions/workflows/test.yml/badge.svg)](https://github.com/lucendex/lucendex/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/lucendex/lucendex/branch/main/graph/badge.svg)](https://codecov.io/gh/lucendex/lucendex)
[![Go Report Card](https://goreportcard.com/badge/github.com/lucendex/lucendex)](https://goreportcard.com/report/github.com/lucendex/lucendex)
[![Go](https://img.shields.io/badge/go-1.25+-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**Non-custodial, deterministic routing engine for XRPL decentralized exchange.**

Lucendex is neutral infrastructure that wallets, fintechs, and funds integrate to access deep XRPL liquidity without building DEX infrastructure themselves.

## Features

- **Deterministic Execution**: QuoteHash binds all parameters (tamper-evident via blake2b-256)
- **Non-Custodial**: Never holds user funds - users sign transactions client-side
- **Circuit Breakers**: Automatic price anomaly protection
- **Real-Time Indexing**: AMM pools + orderbook state synchronized with XRPL ledger
- **Ed25519 Authentication**: Secure API access with request signing
- **Zero-Trust Architecture**: mTLS, least-privilege DB roles, audit logging

## Architecture

```
┌─────────────┐
│   Partner   │
│ (Wallet/App)│
└──────┬──────┘
       │ HTTPS + Ed25519 signature
       ▼
┌─────────────────────────────────┐
│      API Server (Go)            │
│  • Ed25519 auth                 │
│  • Rate limiting                │
│  • Quote generation             │
└────────┬────────────────────────┘
         │
         ▼
   ┌────────────┐     ┌──────────┐
   │   Router   │────▶│ KV Store │
   │ Pathfinder │     └──────────┘
   └────────────┘
         │
         ▼
   ┌─────────────────────────────┐
   │      PostgreSQL             │
   │  • AMM pools                │
   │  • Orderbook state          │
   │  • Quote registry           │
   └─────────────────────────────┘
```

## Quick Start

### Prerequisites

- PostgreSQL 15+
- rippled full-history node
- Go 1.25+

### 1. Database Setup

```bash
# Create database
createdb lucendex

# Run migrations
cd backend/db
psql -d lucendex -f schema.sql
for f in migrations/*.sql; do
    psql -d lucendex -f "$f"
done
```

### 2. Build

```bash
make build
# Or manually:
cd backend
go build -o bin/indexer ./cmd/indexer
go build -o bin/api ./cmd/api
go build -o bin/router ./cmd/router
```

### 3. Configure

```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=lucendex
export RIPPLED_WS=wss://xrplcluster.com
```

### 4. Run

```bash
# Start indexer (syncs XRPL data)
./backend/bin/indexer &

# Start API server
./backend/bin/api &

# API now running on http://localhost:8080
```

### 5. Create Partner (Manual)

```sql
-- Insert partner
INSERT INTO partners (id, name, plan, router_bps, status)
VALUES (
    gen_random_uuid(),
    'My Wallet',
    'pro',
    20,
    'active'
);

-- Generate Ed25519 keypair externally, then:
INSERT INTO api_keys (id, partner_id, public_key, label)
VALUES (
    gen_random_uuid(),
    '<partner-id>',
    '<ed25519-public-key-hex>',
    'Production Key'
);
```

## Testing

```bash
# Run all tests
make test

# With coverage
make test-coverage

# Security tests only
make test-security

# Specific package
cd backend && go test ./internal/router/... -v
```

**Current Test Status:**
- API Authentication: 5/5 passing
- Router: 8/8 passing (pathfinder, validator, hash, breaker)
- KV Store: 4/4 passing
- Parser: 2/2 passing
- Total: 11/11 passing

## API Endpoints

### Partner Endpoints (Auth Required)

**POST /partner/v1/quote**
```json
{
  "in": "XRP",
  "out": "USD.rIssuer",
  "amount": "100"
}
```

**GET /partner/v1/pairs**
Lists available trading pairs

**GET /partner/v1/usage?month=2025-11**
Monthly usage metrics

**GET /partner/v1/health**
System health check

### Authentication

Every request requires Ed25519 signature:

```http
X-Partner-Id: <uuid>
X-Request-Id: <uuid>
X-Timestamp: <RFC3339>
X-Signature: base64(Ed25519.Sign(canonical_request))
```

Canonical request format:
```
METHOD + "\n" + PATH + "\n" + QUERY + "\n" + SHA256(BODY) + "\n" + TIMESTAMP
```

## Security

- **Zero custody**: Never holds user funds
- **Ed25519 signing**: Asymmetric key authentication
- **Replay protection**: Request-ID uniqueness tracking
- **Rate limiting**: Per-partner quotas (100/1000/10000 req/min)
- **Circuit breakers**: Price anomaly detection
- **Audit logging**: All operations logged to PostgreSQL

## Commercial Hosted Service

Don't want to self-host? Use our managed infrastructure:

- **99.9% SLA** with multi-region deployment
- **Enterprise support** with dedicated account team
- **Managed partner onboarding** (we handle key generation)
- **Compliance assistance** for regulated entities
- **Volume pricing** for high-throughput integrators

Contact: hello@lucendex.com

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

We welcome:
- Bug reports and fixes
- Performance improvements
- Additional test coverage
- Documentation improvements
- Integration examples

## License

[MIT License](LICENSE) - See LICENSE file for details.

**Note**: The core Lucendex engine is open source. Partner onboarding and account management for the commercial hosted service is managed separately.

## Links

- **Website**: https://lucendex.com
- **Documentation**: https://lucendex.com/docs
- **XRPL**: https://xrpl.org
