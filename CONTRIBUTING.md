# Contributing to Lucendex

Thank you for your interest in contributing to Lucendex!

## Code of Conduct

- Be respectful and professional
- Focus on technical merit
- Welcome newcomers
- Provide constructive feedback

## How to Contribute

### Reporting Bugs

1. Check existing issues first
2. Include minimal reproducible example
3. Provide Go version, OS, and relevant logs
4. Expected vs actual behavior

### Suggesting Features

1. Open an issue first to discuss
2. Explain the use case
3. Consider backward compatibility
4. Be open to alternative solutions

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for new functionality
4. Ensure all tests pass (`make test`)
5. Follow Go conventions (run `gofmt`)
6. Update documentation if needed
7. Commit with clear messages
8. Push and create PR

## Development Setup

```bash
# Clone repo
git clone https://github.com/lucendex/lucendex.git
cd lucendex

# Install dependencies
cd backend && go mod download

# Run tests
make test

# Build
make build
```

## Testing Requirements

**All contributions must include tests.**

- New features: Add tests
- Bug fixes: Add regression test
- Minimum 80% coverage for critical paths
- Security-critical code: 90%+ coverage

### Running Tests

```bash
# All tests
make test

# Specific package
cd backend && go test ./internal/router/... -v

# With coverage
make test-coverage

# Security tests
make test-security
```

### Test Structure

Use table-driven tests:

```go
func TestQuoteHash(t *testing.T) {
    tests := []struct {
        name    string
        input   QuoteParams
        want    [32]byte
        wantErr bool
    }{
        {
            name: "valid quote",
            input: QuoteParams{...},
            want: expectedHash,
            wantErr: false,
        },
        // More cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ComputeQuoteHash(tt.input)
            // assertions...
        })
    }
}
```

## Code Style

### Go Conventions

- `gofmt` formatted
- Exported functions have godoc comments
- Error messages lowercase, no punctuation
- Use `errors.New()` for static errors
- Prefer stdlib over dependencies

### Security

- No panics in production code
- Validate all inputs
- Use parameterized SQL queries
- Never log secrets
- Constant-time comparisons for crypto

### Comments

- Explain WHY, not WHAT
- Document invariants
- Note security assumptions
- API contracts must be documented

## Project Structure

```
backend/
├── internal/
│   ├── router/      Core routing engine
│   ├── parser/      AMM/orderbook parsing
│   ├── xrpl/        XRPL client
│   ├── kv/          Key-value store
│   ├── api/         API handlers
│   └── store/       Database layer
├── cmd/
│   ├── api/         API server
│   ├── indexer/     XRPL indexer
│   └── router/      Standalone router
└── db/
    ├── schema.sql   Database initialization
    └── migrations/  Schema migrations
```

## Review Process

1. Automated tests must pass
2. Code review by maintainer
3. Security review for auth/crypto changes
4. Documentation review
5. Merge to main

## Areas We're Looking For Help

- **Performance**: Router pathfinding optimization
- **Testing**: Increase coverage, especially edge cases
- **Documentation**: API examples, integration guides
- **XRPL Features**: New transaction types, AMM improvements
- **Monitoring**: Metrics, alerting, observability

## Questions?

- Open an issue for discussion
- Email: dev@lucendex.com
- XRPL Discord: #lucendex

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
