export interface UserClaims {
  user_id: string;
  email: string;
  username: string;
  tier: string;
  exp: number;
}

const TOKEN_KEY = "nexus_token";

export const getToken = (): string | null => {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEY);
};

export const setToken = (t: string): void => {
  localStorage.setItem(TOKEN_KEY, t);
};

export const clearToken = (): void => {
  localStorage.removeItem(TOKEN_KEY);
};

export function getUser(): UserClaims | null {
  const token = getToken();
  if (!token) return null;
  const parts = token.split(".");
  if (parts.length !== 3) return null;
  try {
    // Base64url decode the payload segment
    const payload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = payload + "=".repeat((4 - (payload.length % 4)) % 4);
    const decoded = atob(padded);
    return JSON.parse(decoded) as UserClaims;
  } catch {
    return null;
  }
}

export function isLoggedIn(): boolean {
  const user = getUser();
  if (!user) return false;
  return user.exp * 1000 > Date.now();
}
