import { NAPClient, NAPError } from './client.js';
import { loadState, saveState, isTokenFresh } from './state.js';
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
export async function registerAgent(config, opts = {}) {
    const log = opts.log ?? console.log;
    const client = new NAPClient(config.registry_url);
    const endpoint = config.endpoint ?? '';
    const isDomainVerified = Boolean(config.owner_domain);
    if (isDomainVerified) {
        return registerDomainVerified(client, config, endpoint, log);
    }
    if (!opts.email || !opts.password) {
        throw new Error('email and password are required for the free hosted tier');
    }
    return registerHosted(client, config, endpoint, opts.email, opts.password, log);
}
// ── Free hosted path ──────────────────────────────────────────────────────────
async function registerHosted(client, config, endpoint, email, password, log) {
    log('Creating NAP account…');
    let token;
    let userEmail;
    try {
        const auth = await client.signup(email, password, config.display_name);
        token = auth.token;
        userEmail = auth.user.email;
        log(`Account created — check ${email} to verify your address before activating.`);
    }
    catch (err) {
        if (err instanceof NAPError && err.status === 409) {
            // Account already exists — log in instead
            log('Account already exists — logging in…');
            const auth = await client.login(email, password);
            token = auth.token;
            userEmail = auth.user.email;
        }
        else {
            throw err;
        }
    }
    log('Registering agent…');
    const reg = await client.registerHosted(token, config.display_name, endpoint, config.description);
    const state = {
        agent_id: reg.id,
        agent_uri: reg.agent_uri ?? reg.uri,
        user_token: token,
        email: userEmail,
        status: 'pending',
        registered_at: new Date().toISOString(),
    };
    await saveState(state);
    log(`Registered: ${state.agent_uri}`);
    log('Verify your email, then run: openclaw nap activate');
    return state;
}
// ── Domain-verified path ──────────────────────────────────────────────────────
async function registerDomainVerified(client, config, endpoint, log) {
    const domain = config.owner_domain;
    const capability = config.capability ?? 'assistant';
    log(`Registering agent under ${domain}…`);
    const reg = await client.registerDomain(domain, capability, config.display_name, endpoint, config.description);
    const state = {
        agent_id: reg.id,
        agent_uri: reg.agent_uri ?? reg.uri,
        status: 'pending',
        registered_at: new Date().toISOString(),
    };
    await saveState(state);
    log(`Registered: ${state.agent_uri}`);
    log(`Next: complete DNS-01 verification for ${domain}`);
    log(`  POST ${config.registry_url ?? 'https://registry.nexusagentprotocol.com'}/api/v1/dns/challenge`);
    log(`  body: {"domain": "${domain}"}`);
    return state;
}
// ── Activation ────────────────────────────────────────────────────────────────
/**
 * Activate the registered agent. For nap_hosted, the user must have verified
 * their email first. For domain-verified, the DNS-01 challenge must be complete.
 *
 * Stores the resulting agent card and tokens in state.
 */
export async function activateAgent(config, opts = {}) {
    const log = opts.log ?? console.log;
    const state = await loadState();
    if (!state) {
        throw new Error('No NAP registration found. Run: openclaw nap register');
    }
    const client = new NAPClient(config.registry_url);
    // Refresh token if needed (hosted only)
    let token = state.user_token;
    if (token && !isTokenFresh(state)) {
        log('Refreshing NAP session…');
        if (!state.email) {
            throw new Error('Cannot refresh session — email not stored. Re-run: openclaw nap register');
        }
        throw new Error('NAP user token has expired. Re-authenticate with: openclaw nap login');
    }
    log('Activating agent…');
    const result = await client.activate(state.agent_id, token);
    const updated = { ...state, status: 'active' };
    if (result.agent_card_json)
        updated.agent_card_json = result.agent_card_json;
    if (result.task_token)
        updated.task_token = result.task_token;
    await saveState(updated);
    if (result.private_key_pem) {
        log('');
        log('⚠️  Store this private key securely — it will NOT be shown again:');
        log(result.private_key_pem);
        if (result.certificate) {
            log(`Certificate serial: ${result.certificate.serial}`);
        }
    }
    if (result.agent_card_json) {
        log('');
        log('Agent card ready. Deploy it at /.well-known/agent.json on your domain.');
        log('The OpenClaw Gateway serves it automatically when NAP is enabled.');
    }
    log(`Agent active: ${state.agent_uri}`);
    return updated;
}
//# sourceMappingURL=register.js.map