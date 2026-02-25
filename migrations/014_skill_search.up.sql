-- Migration 014: Promote skill and tool identifiers to indexed columns for structured search.
-- Also enables the primary_skill URI segment (agent://<org>/<cat>/<primary_skill>/<agent_id>).

ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS primary_skill TEXT   NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS skill_ids     TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS tool_names    TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS agents_primary_skill_idx ON agents (primary_skill) WHERE primary_skill != '';
CREATE INDEX IF NOT EXISTS agents_skill_ids_idx     ON agents USING GIN (skill_ids);
CREATE INDEX IF NOT EXISTS agents_tool_names_idx    ON agents USING GIN (tool_names);

-- Backfill primary_skill for existing agents from capability_node.
-- 3-level path  (a>b>c) → "c"
-- 2-level path  (a>b)   → "b"
-- 1-level only          → '' (keep 3-segment URI)
UPDATE agents SET primary_skill =
    CASE
        WHEN capability_node LIKE '%>%>%' THEN split_part(capability_node, '>', 3)
        WHEN capability_node LIKE '%>%'   THEN split_part(capability_node, '>', 2)
        ELSE ''
    END
WHERE primary_skill = '';
