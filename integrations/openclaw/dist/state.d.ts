import type { NAPState } from './types.js';
export declare function loadState(): Promise<NAPState | null>;
export declare function saveState(state: NAPState): Promise<void>;
export declare function clearState(): Promise<void>;
/** Returns true if the stored user token appears to still be valid (rough check). */
export declare function isTokenFresh(state: NAPState): boolean;
//# sourceMappingURL=state.d.ts.map