import type { IncomingMessage, ServerResponse } from 'node:http';
import type { NAPConfig, NAPState } from './types.js';
/**
 * Gateway startup hook.
 *
 * Call this once when the OpenClaw Gateway boots. It will:
 *   1. Load the persisted NAP state (if any).
 *   2. Skip silently if NAP is not enabled or no registration exists yet.
 *   3. If the agent is active, sync the current endpoint to the registry
 *      (in case the Gateway's public URL changed since last run).
 *
 * @returns The loaded NAPState, or null if NAP is not active.
 */
export declare function napStartupHook(config: NAPConfig, currentEndpoint: string, opts?: {
    log?: (msg: string) => void;
}): Promise<NAPState | null>;
/**
 * Returns an HTTP request handler that serves the A2A agent card at
 * `/.well-known/agent.json`.
 *
 * Wire this into the OpenClaw Gateway's HTTP server before the main router:
 *
 * ```ts
 * const napHandler = createAgentCardHandler(napState);
 * server.on('request', (req, res) => {
 *   if (napHandler(req, res)) return;  // handled
 *   mainRouter(req, res);
 * });
 * ```
 *
 * Returns `true` if the request was handled, `false` otherwise.
 */
export declare function createAgentCardHandler(state: NAPState | null): (req: IncomingMessage, res: ServerResponse) => boolean;
/**
 * Periodically refreshes the agent card from the registry and syncs the
 * endpoint if it drifts (e.g. after a Tailscale address change).
 *
 * Returns a cleanup function to stop the interval.
 */
export declare function startNAPSync(config: NAPConfig, getEndpoint: () => string, onStateUpdate: (state: NAPState) => void, opts?: {
    log?: (msg: string) => void;
    intervalMs?: number;
}): () => void;
//# sourceMappingURL=gateway.d.ts.map