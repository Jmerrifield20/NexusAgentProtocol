import type { IncomingMessage, ServerResponse } from 'node:http';
import { loadState, saveState, isTokenFresh } from './state.js';
import { NAPClient } from './client.js';
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
export async function napStartupHook(
  config: NAPConfig,
  currentEndpoint: string,
  opts: { log?: (msg: string) => void } = {},
): Promise<NAPState | null> {
  if (!config.enabled) return null;

  const log = opts.log ?? console.log;
  const state = await loadState();

  if (!state) return null;

  if (state.status !== 'active') {
    log(`[NAP] Agent registered but not yet active (status: ${state.status})`);
    log(`[NAP] Run: openclaw nap activate`);
    return state;
  }

  log(`[NAP] Agent active: ${state.agent_uri}`);

  // Sync endpoint if it changed (best-effort — don't fail startup on error)
  if (state.user_token && currentEndpoint) {
    try {
      await syncEndpoint(config, state, currentEndpoint, log);
    } catch (err) {
      log(`[NAP] Warning: could not sync endpoint — ${(err as Error).message}`);
    }
  }

  return state;
}

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
export function createAgentCardHandler(
  state: NAPState | null,
): (req: IncomingMessage, res: ServerResponse) => boolean {
  return (req, res) => {
    if (req.url !== '/.well-known/agent.json') return false;
    if (req.method !== 'GET' && req.method !== 'HEAD') return false;

    if (!state?.agent_card_json) {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'agent card not yet available' }));
      return true;
    }

    res.writeHead(200, {
      'Content-Type': 'application/json',
      'Cache-Control': 'public, max-age=3600',
    });

    if (req.method === 'HEAD') {
      res.end();
    } else {
      res.end(state.agent_card_json);
    }

    return true;
  };
}

/**
 * Periodically refreshes the agent card from the registry and syncs the
 * endpoint if it drifts (e.g. after a Tailscale address change).
 *
 * Returns a cleanup function to stop the interval.
 */
export function startNAPSync(
  config: NAPConfig,
  getEndpoint: () => string,
  onStateUpdate: (state: NAPState) => void,
  opts: { log?: (msg: string) => void; intervalMs?: number } = {},
): () => void {
  const log = opts.log ?? console.log;
  const intervalMs = opts.intervalMs ?? 60 * 60 * 1000; // 1 hour default

  const timer = setInterval(async () => {
    try {
      const state = await loadState();
      if (!state || state.status !== 'active') return;

      const endpoint = getEndpoint();
      if (endpoint && state.user_token) {
        await syncEndpoint(config, state, endpoint, log);
        const updated = await loadState();
        if (updated) onStateUpdate(updated);
      }
    } catch (err) {
      log(`[NAP] Sync error: ${(err as Error).message}`);
    }
  }, intervalMs);

  // Allow the process to exit even if the timer is still pending
  if (typeof timer === 'object' && timer !== null && 'unref' in timer) {
    (timer as ReturnType<typeof setInterval> & { unref(): void }).unref();
  }

  return () => clearInterval(timer);
}

// ── Internal helpers ──────────────────────────────────────────────────────────

async function syncEndpoint(
  config: NAPConfig,
  state: NAPState,
  endpoint: string,
  log: (msg: string) => void,
): Promise<void> {
  if (!state.user_token) return;
  if (!isTokenFresh(state)) {
    log('[NAP] User token expired — cannot sync endpoint. Run: openclaw nap login');
    return;
  }

  const client = new NAPClient(config.registry_url);
  await client.updateEndpoint(state.agent_id, endpoint, state.user_token);

  const updated: NAPState = {
    ...state,
    endpoint_synced_at: new Date().toISOString(),
  };
  await saveState(updated);
  log(`[NAP] Endpoint synced: ${endpoint}`);
}
