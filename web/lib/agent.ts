export interface Agent {
  id: string;
  trust_root: string;
  capability_node: string;
  agent_id: string;
  display_name: string;
  description: string;
  endpoint: string;
  status: string;
  trust_tier: string;
  registration_type: string;
  owner_domain: string;
  cert_serial: string;
  created_at: string;
  updated_at: string;
  expires_at: string | null;
  version: string;
  tags: string[];
  support_url: string;
  pricing_info: string;
  last_seen_at: string | null;
  health_status: string;
}

export function agentURI(a: Agent): string {
  return `agent://${a.trust_root}/${a.capability_node.split(">")[0]}/${a.agent_id}`;
}

export const TIER_STYLES: Record<string, string> = {
  trusted:    "bg-emerald-100 text-emerald-700",
  verified:   "bg-indigo-100 text-indigo-700",
  basic:      "bg-blue-100 text-blue-700",
  unverified: "bg-gray-100 text-gray-500",
};

export const STATUS_STYLES: Record<string, string> = {
  active:  "bg-green-100 text-green-700",
  pending: "bg-yellow-100 text-yellow-700",
  revoked: "bg-red-100 text-red-700",
  expired: "bg-gray-100 text-gray-500",
};

export const HEALTH_STYLES: Record<string, string> = {
  healthy:  "bg-green-50 text-green-600",
  degraded: "bg-amber-50 text-amber-600",
  unknown:  "bg-gray-50 text-gray-400",
};

export function healthLabel(s: string): string {
  return s === "unknown" ? "health unknown" : s;
}
