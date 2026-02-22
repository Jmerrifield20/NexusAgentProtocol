import type { NAPConfig } from './types.js';
/**
 * Interactive onboarding wizard.
 *
 * Called by: `openclaw nap register`
 *
 * Prompts the user for the minimum info needed, writes ~/.openclaw/nap.json,
 * and prints next-step instructions. Does NOT activate — the user must verify
 * email (hosted) or DNS (domain) first, then run `openclaw nap activate`.
 */
export declare function onboardWizard(config: Partial<NAPConfig>, opts?: {
    log?: (msg: string) => void;
}): Promise<void>;
/**
 * Interactive activation step.
 * Called by: `openclaw nap activate`
 */
export declare function activateWizard(config: NAPConfig, opts?: {
    log?: (msg: string) => void;
}): Promise<void>;
//# sourceMappingURL=onboard.d.ts.map