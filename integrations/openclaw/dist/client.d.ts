import type { NAPAuthResponse, NAPRegisterResponse, NAPActivateResponse } from './types.js';
export declare class NAPClient {
    private readonly base;
    constructor(registryURL?: string);
    signup(email: string, password: string, displayName?: string): Promise<NAPAuthResponse>;
    login(email: string, password: string): Promise<NAPAuthResponse>;
    registerHosted(userToken: string, displayName: string, endpoint: string, description?: string): Promise<NAPRegisterResponse>;
    registerDomain(ownerDomain: string, capability: string, displayName: string, endpoint: string, description?: string): Promise<NAPRegisterResponse>;
    activate(agentUUID: string, userToken?: string): Promise<NAPActivateResponse>;
    updateEndpoint(agentUUID: string, endpoint: string, token: string): Promise<void>;
    private post;
    private patch;
}
export declare class NAPError extends Error {
    readonly status: number;
    constructor(message: string, status: number);
}
//# sourceMappingURL=client.d.ts.map