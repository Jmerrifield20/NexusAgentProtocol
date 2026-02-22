import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { homedir } from 'node:os';
const CONFIG_DIR = join(homedir(), '.openclaw');
const STATE_PATH = join(CONFIG_DIR, 'nap.json');
export async function loadState() {
    try {
        const raw = await readFile(STATE_PATH, 'utf8');
        return JSON.parse(raw);
    }
    catch {
        return null;
    }
}
export async function saveState(state) {
    await mkdir(CONFIG_DIR, { recursive: true });
    await writeFile(STATE_PATH, JSON.stringify(state, null, 2), { mode: 0o600 });
}
export async function clearState() {
    try {
        const { unlink } = await import('node:fs/promises');
        await unlink(STATE_PATH);
    }
    catch {
        // already gone
    }
}
/** Returns true if the stored user token appears to still be valid (rough check). */
export function isTokenFresh(state) {
    if (!state.user_token)
        return false;
    try {
        // JWT is three base64url segments; decode the payload (middle segment)
        const payload = state.user_token.split('.')[1];
        if (!payload)
            return false;
        const decoded = JSON.parse(Buffer.from(payload.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString());
        if (!decoded.exp)
            return true;
        // Consider expired 5 minutes early to avoid edge cases
        return decoded.exp - 300 > Math.floor(Date.now() / 1000);
    }
    catch {
        return false;
    }
}
//# sourceMappingURL=state.js.map