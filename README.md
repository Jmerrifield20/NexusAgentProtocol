# Nexus Agent Protocol (NAP)

An open-source, self-hosted authority for registering and resolving agents on the internet via the `agent://` URI scheme.

## Overview

The Nexus Agent Protocol (NAP) provides a decentralized identity mesh for autonomous agents. By decoupling agent identity from network topology via the `agent://` URI, agents maintain a persistent, verifiable persona regardless of their physical hosting environment.

### Core Services

| Service | Description |
|---------|-------------|
| **Registry** | REST + gRPC API for agent registration, certificate issuance |
| **Resolver** | High-speed `agent://` URI → endpoint translation |
| **Trust Ledger** | Merkle-tree audit log providing cryptographic non-repudiation |
| **CLI (`nap`)** | Command-line tool for agent management |

### `agent://` URI Format

```
agent://[trust-root]/[capability-node]/[agent-id]
```

- **trust-root** — Registry hostname (e.g., `nexusagentprotocol.com`)
- **capability-node** — Hierarchical classification path (e.g., `finance/taxes`)
- **agent-id** — Unique sortable Base32 identifier

## Quickstart

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- `make`

### 1. Start the local dev stack

```bash
make dev
```

This starts PostgreSQL, the registry server, and the resolver via Docker Compose.

### 2. Verify health

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

### 3. Register an agent

```bash
nap register \
  --trust-root nexusagentprotocol.com \
  --capability finance/taxes \
  --endpoint https://my-agent.example.com
```

### 4. Resolve an agent

```bash
nap resolve "agent://nexusagentprotocol.com/finance/taxes/agent_7x2v9q"
```

## Development

### Build

```bash
make build
# Produces: bin/registry, bin/resolver, bin/nap
```

### Test

```bash
make test
```

### Database migrations

```bash
# Apply all pending migrations
make migrate

# Roll back one migration
make migrate-down
```

### Regenerate protobuf

```bash
make proto
```

## Project Structure

```
nexus/
├── cmd/               # Binary entrypoints (registry, resolver, nap CLI)
├── internal/          # Private application packages
│   ├── registry/      # Handler, service, repository, model
│   ├── resolver/      # URI→endpoint translation
│   ├── identity/      # X.509 cert issuance, mTLS, OIDC Task Tokens
│   ├── trustledger/   # Merkle-tree audit log
│   └── dns/           # DNS-01 challenge verification
├── pkg/               # Public Go SDK packages
│   ├── uri/           # agent:// URI parsing & validation
│   ├── agentcard/     # agent-card.json schema & parser
│   └── client/        # Resolver client, mTLS, token exchange
├── api/
│   ├── openapi/       # OpenAPI 3.1 spec
│   └── proto/         # Protobuf definitions
├── migrations/        # SQL migration files
├── configs/           # YAML config templates
├── docker/            # Dockerfiles & docker-compose
├── web/               # Next.js developer portal
└── scripts/           # Build & helper scripts
```

## Configuration

Copy and edit the config templates:

```bash
cp configs/registry.yaml configs/registry.local.yaml
```

Key environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://nexus:nexus@localhost:5432/nexus` | PostgreSQL connection string |
| `REGISTRY_PORT` | `8080` | Registry HTTP port |
| `RESOLVER_PORT` | `9090` | Resolver gRPC port |
| `LOG_LEVEL` | `info` | Logging level |

## Developer Portal

The web portal is a Next.js app in `web/`:

```bash
cd web
pnpm install
pnpm dev
# Open http://localhost:3000
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Run tests: `make test`
4. Submit a pull request

Please read our [Contributing Guide](CONTRIBUTING.md) and follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Security

To report a security vulnerability, please email security@nexusagentprotocol.com rather than opening a public issue.

## License

Apache 2.0 — see [LICENSE](LICENSE).

---

*Nexus Project Team, 2026*
