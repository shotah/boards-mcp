# Agent / contributor notes

## Tool naming (required)

Canonical: [ai-gantry `docs/mcp-naming.md`](https://github.com/shotah/ai-gantry/blob/main/docs/mcp-naming.md).
Package-specific checklist: [TODO.md](TODO.md).

```text
{service}_{verb}_{object}[_{qualifier}]
```

| Layer | Value |
| --- | --- |
| Server id | `boards` |
| Tools | `roster_list`, `roster_create`, `roster_delete`, `notices_list`, `notices_create`, `challenges_list`, `challenges_get`, `challenges_create`, `challenges_update` |
| Host-facing | `boards__roster_list`, … |

Rules:

1. **Service first** — `roster_create`, not `register` / `create_roster`.
2. **No server id on the tool** — never `boards_roster_…`.
3. **Stable verbs** — `list` / `get` / `create` / `update` / `delete`. Leave is `roster_delete`, not `roster_remove`.
4. **No dual aliases.**
5. Tests: `^[a-z]+_[a-z]+` and name does **not** start with `boards`.

Do not register at boot. Author is env `BOARDS_AUTHOR`. Do not `git init` here.
