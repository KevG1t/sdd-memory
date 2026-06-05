# MCP Tools Reference

sdd-memory exposes a persistent memory API via the MCP stdio transport.

**Connect with:** `sdd-memory mcp`

---

## Quick Reference

| Tool | Purpose | Key Inputs | Returns |
|------|---------|------------|---------|
| `mem_save` | Save or update an observation | `content` (req), `project`, `scope`, `topic_key`, `title`, `type`, `tool_name`, `session_id` | `{status, id, sync_id, judgment_required}` |
| `mem_search` | Full-text search | `query` (req), `project` | `[]Observation` |
| `mem_get_observation` | Fetch one observation by id | `id` (req) | `Observation` |
| `mem_delete` | Soft-delete an observation | `id` (req), `hard` (bool, optional) | confirmation string |
| `mem_save_prompt` | Record a user prompt | `content` (req), `project`, `session_id` | `{status, id}` |
| `mem_capture_passive` | Passively capture context | `content` (req) | same as `mem_save` |
| `mem_context` | Recent sessions + observations | — | `{recent_sessions, recent_observations}` |
| `mem_current_project` | Detect project from cwd | — | `{project, path}` |
| `mem_doctor` | Run diagnostics | — | `{status, mode}` |
| `mem_judge` | Evaluate a decision (Lite: no-op) | — | `{status}` |
| `mem_compare` | Compare observations (alias for mem_judge) | — | `{status}` |
| `mem_session_start` | Begin a new session | `id`, `project` | confirmation string |
| `mem_session_end` | End a session with summary | `id`, `summary` | confirmation string |
| `mem_session_summary` | Update session summary | `session_id`, `content` | confirmation string |
| `mem_stats` | Memory statistics | `project` (optional) | `Stats` object |
| `mem_suggest_topic_key` | Suggest a topic_key from title | `title` | `{topic_key}` |
| `mem_update` | Patch an observation's fields | `id`, `title`, `content`, `type`, `scope` | confirmation string |

---

## Three-Branch Dedup — How mem_save Works

`mem_save` applies three branches in order inside a single transaction:

### Branch A — topic_key revision (update in place)

**Triggers when:** `topic_key` is non-empty AND a non-deleted row exists with the same `topic_key` + `project` + `scope`.

**What happens:** The existing row is updated with the new `content`, `title`, `type`, `tool_name`, and `normalized_hash`. `revision_count` is incremented. FTS index is refreshed. The **same `id`** is returned.

**Example:**
```json
// Call 1 → creates obs-abc, revision_count=1
{ "topic_key": "arch/auth", "project": "myapp", "content": "Initial design" }

// Call 2 → updates obs-abc, revision_count=2
{ "topic_key": "arch/auth", "project": "myapp", "content": "Revised design" }
// Response: { "id": "obs-abc", "sync_id": "obs-abc" }
```

### Branch B — hash dedup within 15-minute window

**Triggers when:** Branch A did not match AND `normalized_hash(content)` + `project` + `scope` + `type` + `title` matches a non-deleted row created **within the last 15 minutes**.

**What happens:** `duplicate_count` is incremented. No new row is inserted. The **same `id`** is returned.

**Normalized hash:** SHA-256 of `lowercase(collapse_whitespace(content))`. This means minor whitespace differences are treated as identical.

**Example:**
```json
// Call 1 → inserts obs-xyz, duplicate_count=1
{ "content": "hello world" }

// Call 2 (within 15 min, identical content) → collapses, duplicate_count=2
{ "content": "hello world" }
// Response: { "id": "obs-xyz", "sync_id": "obs-xyz" }
```

### Branch C — new insert

**Triggers when:** Neither Branch A nor Branch B matched.

**What happens:** A new row is inserted with a freshly generated `id = "obs-<16 hex chars>"`, `sync_id = id`, `revision_count = 1`, `duplicate_count = 1`.

---

## Soft-Delete Behavior

`mem_delete` soft-deletes by default (sets `deleted_at` timestamp). Hard delete is permanent.

| Mode | What changes | Sync mutation |
|------|-------------|---------------|
| Soft (`hard=false`) | `deleted_at = now()`, FTS row removed | Enqueued (upsert with deleted_at set) |
| Hard (`hard=true`) | Row removed from DB, FTS row removed | None (local-only) |

**Which tools exclude soft-deleted observations:**
- `mem_get_observation` — returns not-found
- `mem_search` — excluded from results
- `mem_context` — excluded from recent_observations
- `mem_stats` — not counted in `total_observations`
- Branch A of `mem_save` — does not revise soft-deleted rows

**Example:**
```json
// Soft-delete
{ "id": "obs-abc" }
// → "Observation obs-abc soft-deleted"

// Hard delete
{ "id": "obs-abc", "hard": true }
// → "Observation obs-abc hard-deleted"
```

---

## Tool Details

### mem_save

Saves or updates a memory observation using three-branch dedup.

**Inputs:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | yes | Observation text |
| `project` | string | no | Project name (empty = global) |
| `scope` | string | no | `"project"` (default) or `"personal"` |
| `topic_key` | string | no | Stable key for upsert (triggers Branch A) |
| `title` | string | no | Short label |
| `type` | string | no | `"note"` (default), `"decision"`, `"bugfix"`, etc. |
| `tool_name` | string | no | Name of the calling tool |
| `session_id` | string | no | Session identifier |

**Returns:** `{ "status": "saved", "id": "obs-...", "sync_id": "obs-...", "judgment_required": false }`

**Example:**
```json
{
  "content": "Switched from sessions to JWT for stateless auth",
  "project": "myapp",
  "type": "decision",
  "topic_key": "auth/jwt-switch"
}
```

---

### mem_search

Full-text search across all non-deleted observations.

**Inputs:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `query` | string | yes | Search query (FTS5 phrase search) |
| `project` | string | no | Filter by project |

**Returns:** JSON array of `Observation` objects, ordered by relevance.

**Example:**
```json
{ "query": "JWT authentication", "project": "myapp" }
```

---

### mem_get_observation

Fetches a single observation by its `id`. Returns not-found error for soft-deleted observations.

**Inputs:** `id` (string, required)

**Returns:** `Observation` JSON object.

**Example:** `{ "id": "obs-abc123" }`

---

### mem_delete

Soft-deletes an observation (sets `deleted_at`). Removes from FTS index immediately. Pass `"hard": true` for permanent row removal.

**Inputs:** `id` (string, required), `hard` (bool, optional, default false)

**Example:** `{ "id": "obs-abc123" }` or `{ "id": "obs-abc123", "hard": true }`

---

### mem_save_prompt

Records a user prompt in the `user_prompts` table. This tool does NOT create an observation — prompts are stored separately and counted via `mem_stats`.

**Inputs:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | yes | Prompt text |
| `project` | string | no | Project context |
| `session_id` | string | no | Session identifier |

**Returns:** `{ "status": "saved", "id": "<integer id>" }`

**Example:** `{ "content": "What is the auth design?", "session_id": "sess-abc" }`

---

### mem_capture_passive

Passively captures context as an observation with a timestamped topic key. Internally calls `mem_save`.

**Inputs:** `content` (string, required), plus any `mem_save` fields.

---

### mem_context

Returns recent sessions and recent non-deleted observations for current context loading.

**Returns:** `{ "recent_sessions": [...], "recent_observations": [...] }`

**Example:** `{}` (no inputs required)

---

### mem_current_project

Detects the current project name from the working directory.

**Returns:** `{ "project": "sdd-memory", "path": "/home/user/dev/sdd-memory" }`

---

### mem_doctor

Runs diagnostics. In Lite mode always returns healthy.

**Returns:** `{ "status": "healthy", "mode": "lite" }`

---

### mem_session_start

Creates a new session record.

**Inputs:** `id` (string, optional — auto-generated if empty), `project` (string, optional)

**Example:** `{ "id": "sess-20240601", "project": "myapp" }`

---

### mem_session_end

Ends a session and optionally records a summary.

**Inputs:** `id` (string), `summary` (string)

---

### mem_session_summary

Updates the summary for an existing session.

**Inputs:** `session_id` (string), `content` (string, the summary text)

---

### mem_stats

Returns memory statistics. Observation count excludes soft-deleted rows. Prompt count uses `user_prompts` with tombstone exclusion.

**Inputs:** `project` (string, optional — currently returns global stats)

**Returns:**
```json
{
  "total_sessions": 5,
  "total_observations": 42,
  "total_prompts": 18,
  "projects": ["myapp", "sdd-memory"]
}
```

---

### mem_suggest_topic_key

Suggests a stable `topic_key` from a title string (lowercased, spaces → hyphens).

**Inputs:** `title` (string)

**Returns:** `{ "topic_key": "my-feature-title" }`

---

### mem_update

Patches an existing observation's mutable fields and refreshes the FTS index.

**Inputs:** `id` (string, required), `title`, `content`, `type`, `scope` (all optional strings)

**Example:** `{ "id": "obs-abc", "content": "Updated content" }`

---

## Observation Field Reference

All 17 fields of the `Observation` type:

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| `id` | string | no | Primary key, format `obs-<16hex>` |
| `sync_id` | string | yes | Cloud sync identifier; equals `id` for locally-originated rows |
| `session_id` | string | yes | Session that created this observation |
| `type` | string | no | Category: `note`, `decision`, `bugfix`, `pattern`, `config`, etc. |
| `title` | string | no | Short human-readable label |
| `project` | string | yes | Project scope (NULL = global) |
| `scope` | string | no | `project` or `personal` |
| `topic_key` | string | yes | Stable upsert key (NULL = no key, uses hash dedup only) |
| `content` | string | no | Full observation text |
| `tool_name` | string | yes | Name of the MCP tool that created this observation |
| `normalized_hash` | string | yes | SHA-256 of normalized content (for Branch B dedup) |
| `revision_count` | int | no | Number of times this row has been updated via Branch A |
| `duplicate_count` | int | no | Number of identical writes collapsed via Branch B |
| `last_seen_at` | string | yes | Timestamp of the most recent access/dedup hit (RFC3339) |
| `deleted_at` | string | yes | Set when soft-deleted; NULL means active |
| `created_at` | string | no | Insert timestamp (RFC3339) |
| `updated_at` | string | no | Last modification timestamp (RFC3339) |

---

## Notes

- All timestamps use RFC3339 format (`2006-01-02T15:04:05Z`).
- `project` field accepts empty string from callers; the store normalizes it to NULL internally.
- FTS search uses FTS5 standalone mode (not external-content). The index holds its own text copy, making safe to UPDATE the source row without FTS corruption.
- Soft-deleted observations remain in the database but are invisible to all read tools. They can be recovered only via direct SQL until a future recovery tool is added.
