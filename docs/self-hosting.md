# Self-Hosting the NAP Registry

This guide covers deploying your own Nexus Agent Protocol registry. Agents registered on your instance get addresses rooted at your domain (`agent://yourdomain.com/...`). Any NAP client resolves them by querying your registry directly.

---

## Prerequisites

- **Go 1.22+** (for building from source)
- **PostgreSQL 16+**
- **A domain name** with DNS control (for TLS and agent resolution)
- **SMTP credentials** (optional, required for free-tier email verification)

---

## 1. Quick Start (Docker Compose)

The fastest way to get running locally:

```bash
git clone https://github.com/jmerrifield20/NexusAgentProtocol.git
cd NexusAgentProtocol
make dev
```

This starts three services:

| Service | Port | Description |
|---------|------|-------------|
| `postgres` | 5432 | PostgreSQL 16 with automatic migrations |
| `registry` | 8080 | NAP registry (HTTP) |
| `resolver` | 9090/9091 | gRPC resolver + REST gateway |

Verify:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Stop everything:

```bash
make dev-down
```

---

## 2. Building from Source

```bash
make build
# Produces:
#   bin/registry   — the registry server
#   bin/resolver   — gRPC resolver
#   bin/nap        — CLI tool
```

---

## 3. Database Setup

Create a PostgreSQL database and run migrations:

```bash
# Create database
createdb -U postgres nexus

# Set the connection string
export DATABASE_URL="postgres://nexus:nexus@localhost:5432/nexus?sslmode=disable"

# Run migrations
make migrate
```

The registry applies migrations from the `migrations/` directory in order:

| Migration | What it does |
|-----------|-------------|
| `001_init.sql` | Core schema: agents, certificates, trust_ledger, dns_challenges |
| `002_trustledger_genesis.sql` | Trust Ledger genesis entry |
| `003_users.sql` | User accounts, OAuth, email verification |
| `004-006` | Capabilities, A2A/MCP metadata, threat scoring |
| `007_federation.sql` | Federation support (registered_registries) |
| `008_user_profile.sql` | User profiles |
| `009_revocation_improvements.sql` | Suspended status, revocation reasons |
| `010_deprecation.sql` | Deprecated status, sunset dates |
| `011_abuse_reports.sql` | Abuse reporting table |
| `012_webhooks.sql` | Webhook subscriptions and deliveries |

---

## 4. Configuration

Copy and customize the config template:

```bash
cp configs/registry.yaml configs/registry.local.yaml
```

### Minimal production config

```yaml
registry:
  port: 8080
  frontend_url: "https://registry.yourdomain.com"
  role: standalone
  admin_secret: "${REGISTRY_ADMIN_SECRET}"  # set via env var

database:
  url: "${DATABASE_URL}"
  max_connections: 25

identity:
  ca_cert_path: "/etc/nap/ca.crt"
  ca_key_path: "/etc/nap/ca.key"
  cert_validity_days: 365

health:
  check_interval: "5m"
  probe_timeout: "10s"
  fail_threshold: 3
```

### Environment variables

Any config key can be set via environment variable. Use uppercase with underscores:

| Config key | Env var | Example |
|-----------|---------|---------|
| `database.url` | `DATABASE_URL` | `postgres://...` |
| `registry.port` | `REGISTRY_PORT` | `8080` |
| `registry.admin_secret` | `REGISTRY_ADMIN_SECRET` | `my-secret` |
| `registry.frontend_url` | `REGISTRY_FRONTEND_URL` | `https://...` |

---

## 5. TLS Configuration

### HTTP only (development)

The registry listens on HTTP by default. For local development this is fine.

### TLS termination (production)

In production, terminate TLS at a reverse proxy (nginx, Caddy, or a cloud load balancer) and forward to the registry's HTTP port.

**Caddy example:**

```
registry.yourdomain.com {
    reverse_proxy localhost:8080
}
```

**nginx example:**

```nginx
server {
    listen 443 ssl;
    server_name registry.yourdomain.com;

    ssl_certificate     /etc/ssl/certs/registry.crt;
    ssl_certificate_key /etc/ssl/private/registry.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### mTLS (port 8443)

The registry also listens on port 8443 for mTLS connections. This is used by domain-verified agents that hold X.509 certificates issued by the registry CA. The CA certificate and key are auto-generated on first startup and written to the `certs/` directory.

To pre-generate the CA:

```bash
make gen-ca
# CA generated in certs/ — copy certs/ca.crt to trust it in your OS/browser.
```

---

## 6. SMTP Setup

Email is required for free-tier (NAP-hosted) agent registration. Without SMTP, users cannot verify their email and activate agents.

```yaml
email:
  smtp_host: "smtp.example.com"
  smtp_port: 587
  smtp_username: "noreply@yourdomain.com"
  smtp_password: "${SMTP_PASSWORD}"
  from_address: "noreply@yourdomain.com"
```

If you don't need free-tier registration (domain-verified agents only), you can skip SMTP configuration entirely. The registry uses a no-op email sender when SMTP is not configured.

---

## 7. Health Checker

The built-in health checker probes active agent endpoints on a schedule:

```yaml
health:
  check_interval: "5m"    # how often to check all endpoints
  probe_timeout: "10s"    # timeout per probe
  fail_threshold: 3       # consecutive failures before marking degraded
```

The health checker:
- Sends `HEAD` requests (falls back to `GET`) to each active agent's endpoint
- Only writes to the database on **status transitions** (healthy to degraded, or recovery)
- Logs transitions and records them in the Trust Ledger
- Dispatches `agent.health_degraded` webhook events on degradation

---

## 8. Observability

### Prometheus metrics

The registry exposes metrics at `GET /metrics` in Prometheus exposition format:

- `nap_agents_total` (gauge, by status) — agent counts
- `nap_requests_total` (counter, by method/path/status) — HTTP request rates
- `nap_request_duration_seconds` (histogram) — latency distribution
- `nap_health_checks_total` (counter, by result) — health probe outcomes
- `nap_ledger_entries_total` (counter) — Trust Ledger appends
- `nap_webhook_deliveries_total` (counter, by success) — webhook delivery attempts

### Prometheus scrape config

```yaml
scrape_configs:
  - job_name: nap-registry
    static_configs:
      - targets: ['registry.yourdomain.com:8080']
```

### Logging

```yaml
log:
  level: info    # debug | info | warn | error
  format: json   # json | console
```

---

## 9. Webhooks

Users can subscribe to lifecycle events via `POST /api/v1/webhooks`. Webhook deliveries include an `X-NAP-Signature` HMAC-SHA256 header for payload verification. Failed deliveries are retried with exponential backoff.

No additional configuration is needed — webhook support is built in.

---

## 10. Federation

To join the NAP federation (optional), see the [Federation Protocol](spec/nap-federation-protocol.md). The short version:

1. Set `registry.role: federated` in your config
2. Obtain an intermediate CA certificate from the root registry
3. Configure the intermediate CA paths and root registry URL

```yaml
registry:
  role: federated

federation:
  root_registry_url: "https://registry.nexusagentprotocol.com"
  intermediate_ca_cert: "/etc/nap/intermediate.crt"
  intermediate_ca_key: "/etc/nap/intermediate.key"
  dns_discovery_enabled: true
  remote_resolve_timeout: 5s
```

---

## 11. OAuth (Optional)

To enable GitHub/Google login on your registry:

```yaml
oauth:
  github:
    client_id: "${GITHUB_CLIENT_ID}"
    client_secret: "${GITHUB_CLIENT_SECRET}"
    redirect_url: "https://registry.yourdomain.com/api/v1/auth/oauth/github/callback"
  google:
    client_id: "${GOOGLE_CLIENT_ID}"
    client_secret: "${GOOGLE_CLIENT_SECRET}"
    redirect_url: "https://registry.yourdomain.com/api/v1/auth/oauth/google/callback"
```

---

## 12. Production Checklist

- [ ] PostgreSQL with connection pooling and backups configured
- [ ] TLS termination in front of the registry
- [ ] `REGISTRY_ADMIN_SECRET` set to a strong random value
- [ ] SMTP configured (if supporting free-tier registration)
- [ ] CA certificate and key stored securely (backed up)
- [ ] `frontend_url` set to your public registry URL
- [ ] Prometheus scraping `/metrics`
- [ ] Log aggregation configured
- [ ] DNS `_nap-registry.yourdomain.com` TXT record published (for federation discovery)
- [ ] Firewall: expose ports 8080 (HTTP) and 8443 (mTLS) only as needed

---

## 13. Running Tests

```bash
# Unit tests
make test

# Integration tests (starts Postgres via Docker)
make test-integration

# Lint
make lint
```
