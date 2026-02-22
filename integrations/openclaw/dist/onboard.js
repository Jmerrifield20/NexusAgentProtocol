import { createInterface } from 'node:readline/promises';
import { stdin, stdout } from 'node:process';
import { loadState } from './state.js';
import { registerAgent, activateAgent } from './register.js';
/**
 * Interactive onboarding wizard.
 *
 * Called by: `openclaw nap register`
 *
 * Prompts the user for the minimum info needed, writes ~/.openclaw/nap.json,
 * and prints next-step instructions. Does NOT activate — the user must verify
 * email (hosted) or DNS (domain) first, then run `openclaw nap activate`.
 */
export async function onboardWizard(config, opts = {}) {
    const log = opts.log ?? console.log;
    const rl = createInterface({ input: stdin, output: stdout });
    try {
        log('');
        log('NAP Agent Registration');
        log('──────────────────────────────────────────────────────');
        log('The Nexus Agent Protocol gives your OpenClaw Gateway a');
        log('stable, globally-resolvable agent:// URI.');
        log('');
        // Check for existing registration
        const existing = await loadState();
        if (existing) {
            log(`Already registered: ${existing.agent_uri} (status: ${existing.status})`);
            const reset = await rl.question('Re-register? This will overwrite the existing state. [y/N] ');
            if (reset.toLowerCase() !== 'y') {
                log('Aborted.');
                return;
            }
        }
        // ── Determine registration path ───────────────────────────────────────────
        const hasExistingDomain = Boolean(config.owner_domain);
        let registrationType = 'hosted';
        if (!hasExistingDomain) {
            log('Registration type:');
            log('  1) Free hosted  — agent://nap/<capability>/<id>');
            log('  2) Domain-verified — agent://<your-domain>/<capability>/<id>');
            log('');
            const choice = await rl.question('Choose [1]: ');
            registrationType = choice.trim() === '2' ? 'domain' : 'hosted';
        }
        else {
            registrationType = 'domain';
            log(`Using domain-verified registration for: ${config.owner_domain}`);
        }
        log('');
        // ── Shared fields ─────────────────────────────────────────────────────────
        const displayName = config.display_name ??
            (await rl.question('Agent display name: '));
        const descriptionRaw = config.description ?? (await rl.question('Short description (optional): '));
        const description = descriptionRaw || undefined;
        const endpointRaw = config.endpoint ??
            (await rl.question('Public endpoint URL (e.g. https://myagent.example.com) — leave blank to set later: '));
        const endpoint = endpointRaw || undefined;
        // ── Path-specific fields ──────────────────────────────────────────────────
        const napConfig = {
            enabled: true,
            display_name: displayName,
        };
        if (config.registry_url)
            napConfig.registry_url = config.registry_url;
        if (description)
            napConfig.description = description;
        if (endpoint)
            napConfig.endpoint = endpoint;
        if (registrationType === 'domain') {
            const domain = config.owner_domain ?? (await rl.question('Owner domain (e.g. acme.com): '));
            const capabilityRaw = config.capability ?? (await rl.question('Capability node [assistant]: '));
            const capability = capabilityRaw || 'assistant';
            napConfig.owner_domain = domain;
            napConfig.capability = capability;
            log('');
            await registerAgent(napConfig, { log });
        }
        else {
            // Free hosted — needs account credentials
            log('');
            log('You need a free NAP account. Enter your email and a new password, or');
            log('your existing credentials if you already have an account.');
            log('');
            const email = await rl.question('Email: ');
            const password = await readPassword(rl, 'Password (min 8 chars): ');
            log('');
            await registerAgent(napConfig, { email, password, log });
        }
    }
    finally {
        rl.close();
    }
}
/**
 * Interactive activation step.
 * Called by: `openclaw nap activate`
 */
export async function activateWizard(config, opts = {}) {
    const log = opts.log ?? console.log;
    const state = await loadState();
    if (!state) {
        log('No registration found. Run: openclaw nap register');
        return;
    }
    log(`Activating agent: ${state.agent_uri}`);
    log('');
    await activateAgent(config, { log });
}
// ── Helpers ───────────────────────────────────────────────────────────────────
/** Read a password without echoing to the terminal (best-effort on TTY). */
async function readPassword(rl, prompt) {
    // readline/promises doesn't support silent input natively.
    // We write the prompt and tell the user input won't echo.
    stdout.write(prompt + '(input hidden) ');
    // Temporarily suppress echo if we're on a real TTY
    const tty = stdout.isTTY ? stdout : null;
    if (tty && process.stdin.isTTY) {
        process.stdin.setRawMode?.(true);
    }
    const pass = await rl.question('');
    if (tty && process.stdin.isTTY) {
        process.stdin.setRawMode?.(false);
    }
    stdout.write('\n');
    return pass;
}
//# sourceMappingURL=onboard.js.map