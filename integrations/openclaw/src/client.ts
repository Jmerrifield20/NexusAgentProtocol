import type {
  NAPAuthResponse,
  NAPRegisterResponse,
  NAPActivateResponse,
} from './types.js';

const DEFAULT_REGISTRY = 'https://registry.nexusagentprotocol.com';

export class NAPClient {
  private readonly base: string;

  constructor(registryURL?: string) {
    this.base = (registryURL ?? DEFAULT_REGISTRY).replace(/\/$/, '');
  }

  // ── Auth ──────────────────────────────────────────────────────────────────

  async signup(
    email: string,
    password: string,
    displayName?: string,
  ): Promise<NAPAuthResponse> {
    return this.post<NAPAuthResponse>('/api/v1/auth/signup', {
      email,
      password,
      display_name: displayName,
    });
  }

  async login(email: string, password: string): Promise<NAPAuthResponse> {
    return this.post<NAPAuthResponse>('/api/v1/auth/login', { email, password });
  }

  // ── Agents ────────────────────────────────────────────────────────────────

  async registerHosted(
    userToken: string,
    displayName: string,
    endpoint: string,
    description?: string,
  ): Promise<NAPRegisterResponse> {
    return this.post<NAPRegisterResponse>(
      '/api/v1/agents',
      {
        registration_type: 'nap_hosted',
        display_name: displayName,
        description,
        endpoint,
      },
      userToken,
    );
  }

  async registerDomain(
    ownerDomain: string,
    capability: string,
    displayName: string,
    endpoint: string,
    description?: string,
  ): Promise<NAPRegisterResponse> {
    return this.post<NAPRegisterResponse>('/api/v1/agents', {
      trust_root: ownerDomain,
      owner_domain: ownerDomain,
      capability_node: capability,
      display_name: displayName,
      description,
      endpoint,
    });
  }

  async activate(agentUUID: string, userToken?: string): Promise<NAPActivateResponse> {
    return this.post<NAPActivateResponse>(
      `/api/v1/agents/${agentUUID}/activate`,
      null,
      userToken,
    );
  }

  async updateEndpoint(
    agentUUID: string,
    endpoint: string,
    token: string,
  ): Promise<void> {
    await this.patch(`/api/v1/agents/${agentUUID}`, { endpoint }, token);
  }

  // ── HTTP helpers ──────────────────────────────────────────────────────────

  private async post<T>(path: string, body: unknown, token?: string): Promise<T> {
    const res = await fetch(`${this.base}${path}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      ...(body != null ? { body: JSON.stringify(body) } : {}),
    });

    const data = (await res.json()) as Record<string, unknown>;
    if (!res.ok) {
      const msg = (data['error'] as string | undefined) ?? `HTTP ${res.status}`;
      throw new NAPError(msg, res.status);
    }
    return data as T;
  }

  private async patch(path: string, body: unknown, token: string): Promise<void> {
    const res = await fetch(`${this.base}${path}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const data = (await res.json().catch(() => ({}))) as Record<string, unknown>;
      const msg = (data['error'] as string | undefined) ?? `HTTP ${res.status}`;
      throw new NAPError(msg, res.status);
    }
  }
}

export class NAPError extends Error {
  constructor(
    message: string,
    public readonly status: number,
  ) {
    super(message);
    this.name = 'NAPError';
  }
}
