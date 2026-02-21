# Nexus Agentic Protocol (NAP): The 2026 Manifesto

## 1. Vision Statement
In the 2026 agentic economy, identity is the new perimeter. The **Nexus Agentic Protocol (NAP)** aims to transition the internet from a collection of "websites for humans" to a decentralized "mesh for agents." By decoupling identity from network topology via the `agent://` URI, we enable agents to maintain a persistent, verifiable persona regardless of their physical hosting environment.

---

## 2. Technical Specification: The `agent://` URI
Following the IETF 2026 drafts (e.g., `draft-narvaneni-agent-uri-02`), the NAP URI standardizes how autonomous systems address one another.

### Standard Format:
`agent://[trust-root]/[capability-node]/[agent-id]`

* **Trust Root:** The hostname of the Central Authority (e.g., `nexus.io`).
* **Capability Node:** A hierarchical classification path (e.g., `/finance/taxes`).
* **Agent ID:** A unique, sortable Base32 identifier (e.g., `agent_7x2v9q...`).

---

## 3. The Central Authority (The Nexus Registry)
The Nexus is the hosted application that serves as the "Global Registrar." 

### Core Services:
1.  **Identity Minting:** Verifies ownership via DNS-01 challenges and issues X.509 Agent Identity Certificates.
2.  **The Resolver:** A high-speed API that translates `agent://` addresses into real-time transport endpoints (HTTPS/gRPC/WebSocket).
3.  **Trust Ledger:** A Merkle-tree based audit log of all agent registrations, providing non-repudiation and cryptographic proof of identity.

---

## 4. Cross-Domain Security Architecture
To maintain a Zero-Trust environment across disparate domains, NAP implements the following:

### A. Mutual TLS (mTLS)
Every connection requires both the 'Caller' and 'Receiver' to present certificates signed by the Nexus Registry. Connections lacking valid attestation are terminated at the handshake layer.

### B. OIDC for Agents
NAP utilizes OpenID Connect (OIDC) extensions for machine-to-machine authorization. Agents exchange "Task Tokens" that are cryptographically bound to a specific duration and set of permissions (scopes).

### C. The .well-known Discovery
Every NAP-compliant domain must host an `agent-card.json` at:
`https://[domain]/.well-known/agent-card.json`

---

## 5. The SDK: Nexus-Core
To spearhead adoption, we provide the **Nexus-Core SDK**, a library that handles:
* URI Resolution and Caching.
* Automated mTLS certificate rotation.
* JSON-RPC 2.0 message framing.

---

## 6. Project Roadmap
* **Q1 2026:** Launch Alpha Registry at `nexus.io`.
* **Q2 2026:** Open-source the Nexus-Core Python/TypeScript libraries.
* **Q3 2026:** Submit the finalized `agent://` URI scheme to IANA.
* **Q4 2026:** Enable federated registries for private enterprise clusters.

---
*Created by the Nexus Project Team, 2026.*
