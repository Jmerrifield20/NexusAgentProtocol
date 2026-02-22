const DEFAULT_REGISTRY = 'https://registry.nexusagentprotocol.com';
export class NAPClient {
    base;
    constructor(registryURL) {
        this.base = (registryURL ?? DEFAULT_REGISTRY).replace(/\/$/, '');
    }
    // ── Auth ──────────────────────────────────────────────────────────────────
    async signup(email, password, displayName) {
        return this.post('/api/v1/auth/signup', {
            email,
            password,
            display_name: displayName,
        });
    }
    async login(email, password) {
        return this.post('/api/v1/auth/login', { email, password });
    }
    // ── Agents ────────────────────────────────────────────────────────────────
    async registerHosted(userToken, displayName, endpoint, description) {
        return this.post('/api/v1/agents', {
            registration_type: 'nap_hosted',
            display_name: displayName,
            description,
            endpoint,
        }, userToken);
    }
    async registerDomain(ownerDomain, capability, displayName, endpoint, description) {
        return this.post('/api/v1/agents', {
            trust_root: ownerDomain,
            owner_domain: ownerDomain,
            capability_node: capability,
            display_name: displayName,
            description,
            endpoint,
        });
    }
    async activate(agentUUID, userToken) {
        return this.post(`/api/v1/agents/${agentUUID}/activate`, null, userToken);
    }
    async updateEndpoint(agentUUID, endpoint, token) {
        await this.patch(`/api/v1/agents/${agentUUID}`, { endpoint }, token);
    }
    // ── HTTP helpers ──────────────────────────────────────────────────────────
    async post(path, body, token) {
        const res = await fetch(`${this.base}${path}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...(token ? { Authorization: `Bearer ${token}` } : {}),
            },
            ...(body != null ? { body: JSON.stringify(body) } : {}),
        });
        const data = (await res.json());
        if (!res.ok) {
            const msg = data['error'] ?? `HTTP ${res.status}`;
            throw new NAPError(msg, res.status);
        }
        return data;
    }
    async patch(path, body, token) {
        const res = await fetch(`${this.base}${path}`, {
            method: 'PATCH',
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${token}`,
            },
            body: JSON.stringify(body),
        });
        if (!res.ok) {
            const data = (await res.json().catch(() => ({})));
            const msg = data['error'] ?? `HTTP ${res.status}`;
            throw new NAPError(msg, res.status);
        }
    }
}
export class NAPError extends Error {
    status;
    constructor(message, status) {
        super(message);
        this.status = status;
        this.name = 'NAPError';
    }
}
//# sourceMappingURL=client.js.map