// Package identity implements the Nexus Agentic Protocol identity layer.
//
// It provides:
//   - CAManager       — creates/loads the Nexus root Certificate Authority
//   - Issuer          — issues and verifies X.509 agent and server certificates
//   - TokenIssuer     — issues and verifies RS256 JWT Task Tokens
//   - OIDCProvider    — OIDC discovery and JWKS HTTP endpoints
//   - RequireMTLS     — Gin middleware enforcing mutual TLS authentication
//   - RequireToken    — Gin middleware enforcing Bearer Task Token authentication
package identity
