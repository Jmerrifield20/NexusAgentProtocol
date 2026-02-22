import type { NAPConfig, NAPState } from './types.js';
/**
 * Core registration logic.
 *
 * Called from the onboarding wizard on first run. Handles both registration
 * paths:
 *   - nap_hosted: requires email + password for the free-tier account
 *   - domain:     registers anonymously; user must complete DNS-01 challenge
 *
 * Returns the persisted NAPState so the caller can display the URI/next steps.
 */
export declare function registerAgent(config: NAPConfig, opts?: {
    /** Required for nap_hosted path. */
    email?: string;
    /** Required for nap_hosted path. */
    password?: string;
    /** Callback to log progress messages. */
    log?: (msg: string) => void;
}): Promise<NAPState>;
/**
 * Activate the registered agent. For nap_hosted, the user must have verified
 * their email first. For domain-verified, the DNS-01 challenge must be complete.
 *
 * Stores the resulting agent card and tokens in state.
 */
export declare function activateAgent(config: NAPConfig, opts?: {
    log?: (msg: string) => void;
}): Promise<NAPState>;
//# sourceMappingURL=register.d.ts.map