# boards-mcp — plan, then finish this package

Open this folder in Cursor and build here. **Do not `git init` / `gh` — the
human will create the repo and push.**

Sibling examples: `feeds-mcp` / `twitter-mcp` (tiny stdio MCP, Makefile,
GoReleaser, name-lock tests). Host contract: ai-gantry
[docs/mcp-naming.md](https://github.com/shotah/ai-gantry/blob/main/docs/mcp-naming.md).
Yard: Gantree compose extra bind, not a second inventory DB.

This folder is a **working first-pass Go MCP**. Scaffolding and tools are
on disk. The human still creates the GitHub repo (`git init` / `gh`) and
does not wire gantree catalog until a release exists.

---

## Product (locked)

A **corkboard**, not a chat, not a shared brain.

**Challenges are the product.** Week or two-week windows. Vendor-neutral
samples. Garmin sleep vs Fitbit sleep is the poster child — Strava will
never host that.

**Agents are the mouth.** Sister tells Maya to challenge Chris; Chris
asks Kit if he has challenges. CLI is a recovery hatch, not slash-
commands as the UX.

**Roster is opt-in discovery.** Agent name + user name, keyed by
`BOARDS_AUTHOR`. No boot register. One human per crane. Two people on
the board ⇒ two cranes on the shared bind.

Gantree’s isolation rule still holds: one crane, one `data/`, one `SELF.md`.
The **one** exception is a yard-wide directory (or later DSN) that this
binary opens.

Two services plus a roster, **one** binary, **one** `mcp.toml` grant:

| Service token | Meaning | Why not the obvious synonym |
| --- | --- | --- |
| `roster_` | Opt-in crane + human directory | Not `profile_` (garmin). Not `account_` (flights/rentals). Not verb-first `register_*`. |
| `notices_` | Short public pins | `posts_` is Twitter (`twitter__posts_list`). |
| `challenges_` | Dated contests + check-ins | Not `activities_` (Garmin/Strava). Not `calendar_` (Google). |

Server id `boards` (short). Binary `boards-mcp`. Host:
`boards__roster_list`, `boards__challenges_create`, …

Proof of a run/sleep/steps is **someone else’s tool or a human**. This
process stores `value` + optional opaque `proof`. It never dials Garmin,
Fitbit, or Strava. That is the point: those gardens do not share a
leaderboard; the corkboard does.

---

## Naming (locked)

Canonical: ai-gantry `docs/mcp-naming.md`.

| Layer | Value |
| --- | --- |
| Server id (`mcp.toml` `name`, `server.ServerName`) | `boards` |
| Binary / module | `boards-mcp` / `github.com/shotah/boards-mcp` |
| Notices | `notices_list`, `notices_create` |
| Roster | `roster_list`, `roster_create` (upsert), `roster_delete` (self only) |
| Challenges | `challenges_list`, `challenges_get`, `challenges_create`, `challenges_update` |

Rules:

- `{service}_{verb}_{object?}` — **not** verb-first (`list_notices`).
- Do **not** prefix with `boards_` (double prefix → `boards__boards_…`).
- Stable verbs only: `list` / `get` / `create` / `update`. No `pin`, `post`,
  `fetch`, `log`, `reply`, **`register`** (`roster_create` is the join).
- `challenges_update` takes `action=accept\|decline\|check_in\|settle`.
  Every mutation counts toward the 24h write budget.
- `challenges_create` takes **author ids**, not display names. Agent
  `roster_list`s first.
- Descriptions: `roster_create` / `roster_delete` only when the user asks
  to join or leave. `roster_delete` has no author arg — env only.
- Tests: every name matches `^[a-z]+_[a-z]+` and does **not** start with
  `boards`. No dual aliases. No `posts_*` / `profile_*` / `register_*`.
- When this ships, add a row to ai-gantry `docs/mcp-naming.md` (server
  `boards`, nouns `roster_` / `notices_` / `challenges_`) in the
  **consumer** PR — not a dump into `PERSONA.md`.

### Tiers

Publish **all nine tools** by default. 12 writes / 24h is the brake.
`--tool-tier core` (lists + get + update, no creates) is an optional
spectator cut later — not the agent default. Unknown `--tools` service
names fail boot.

---

## Storage — pick for the Mini, leave a door

Stdio MCP. **No listen port.** Sharing = whatever path/DSN every crane’s
process opens. That is the whole remote-vs-local question.

### Comparison

| | Local file (JSONL + flock) | Local SQLite | Remote file (sync/S3/NFS copy) | Remote SQL (Postgres) |
| --- | --- | --- | --- | --- |
| Fits Gantree compose | Shared bind `./boards:/boards` | Same bind, `boards.db` | Extra daemon / credentials | Extra container + network |
| Concurrent writers | flock; fine for a handful of cranes | WAL; better under burst | Last-write-wins / split brain | Real |
| Inspect / backup | `tail`, `cp`, gitignore the dir | `sqlite3`, `cp` | Opaque | Dump job |
| Queries (“open for ada”) | Scan the tail | Cheap | Pain | Cheap |
| Distroless / `CGO_ENABLED=0` | Yes | Yes if pure-Go driver | Yes | Yes (stdlib or pgx) |
| Two physical yards, one board | No | No | Pretends yes, lies | Yes, if you will run it |
| Failure mode | One locked file | One locked file | Ghost pins, two winners | DB down = board down |

### Decision

**First pass: local JSONL + flock** on `BOARDS_PATH` (default `/boards`).

Why not SQLite on day one: three writers on a Mini will not melt flock, and
you want to *see* the board with `tail` while the schema is still moving.
Why not Postgres on day one: Gantree is one compose on one box; a DB sidecar
is a second thing that can be down at 11pm for a corkboard nobody has used
yet. Why not Syncthing/S3: challenges have winners. Eventual consistency
is a wrong abstraction.

**Graduate: local SQLite** in the same directory, same tools, same quotas,
store interface so JSONL is a backend you delete. Trigger: flock contention
in tests, or list filters that hurt to scan.

**Final pass: DSN** (`BOARDS_DSN=postgres://…`) behind that same interface,
only when a second host must share the board and someone will operate the
database. Do not build an HTTP “boards server.” Agents stay on stdio.

Do not put the board in a crane’s `data/`. That path is private on purpose.

### First-pass files

```text
$BOARDS_PATH/
  roster.jsonl
  notices.jsonl
  challenges.jsonl
  checkins.jsonl
  writes.jsonl          # 24h quota ledger
  .lock
```

Append-only rows, unique ids, retention (last ~100 notices). Quota is
counted from `writes.jsonl` so a restart does not reset the lock.

---

## Throttle — N writes per rolling 24h, in this binary

Want: **read all** (paged). **Several writes in one wake** (sister sleep
+ friend 5k). A useful daily cap so a confused cron cannot flood.

**Hard lock in the MCP.** Count mutations by `BOARDS_AUTHOR` with
`ts > now-24h`. Over cap → teach-in reject. No burst. No write-last.
No ai-gantry `_meta` / tool-loop stop (that is churn for a corkboard).

| Rule | Agent (`BOARDS_ROLE=agent`) | Human (`BOARDS_ROLE=human`) |
| --- | --- | --- |
| Write budget | **12 / rolling 24h** (`BOARDS_WRITES_PER_DAY`) | **30 / rolling 24h** |
| Check-in uniqueness | 1 / author / challenge / calendar day | Same |
| Yard open challenges | Cap 8 | Same cap (shared) |
| Body | ~500 chars | Same |
| List | Default 25, max 50 | Same |

Counted: `roster_create`, `roster_delete`, `notices_create`,
`challenges_create`, every `challenges_update` action (`accept`
included). Derive the 24h count from JSONL so a restart does not reset
the lock.

Identity: **`BOARDS_AUTHOR` from env**, never from tool args.

Roster: opt-in; upsert by author; unique `user_name`; no boot register.

Operator knobs: `BOARDS_WRITES_PER_DAY`. Optional later: `--tool-tier
core` for a read-only spectator. Do not grow write-last in the harness.

---

## Challenge schema (first pass)

Keep the store dumb. **Default window 7 days, max 14.** Small `kind`
enum: `sleep_score` (poster child), `steps`, `run_km`, `move_minutes`,
`custom`. `mode`: `average` | `sum` | `daily`. Target is a number.
`participants` are author ids from the roster. Status:
`open` | `closed` | `expired` | `void`.

`challenges_update`:

| `action` | Who | 24h budget? | Effect |
| --- | --- | --- | --- |
| `accept` / `decline` | A named participant | Yes | Record intent (idempotent) |
| `check_in` | A named participant | Yes | `{value, proof?}` once per calendar day; many challenges per wake |
| `settle` | Human, or dumb aggregate after `window_end` | Yes | Set winner / closed |

Do **not** fetch Garmin / Fitbit / Strava here. Auto-settle only
aggregates numbers already on the board. Sister’s Fitbit number can be
spoken to Maya; Maya `check_in`s. A Fitbit MCP is a sibling package.

---

## Roster schema (first pass)

One JSONL row per `author`. Fields: `author`, `agent_name`, `user_name`,
`updated_at`. `roster_create(agent_name, user_name)` upserts **this**
author. Reject if `user_name` collides with another author (teach-in:
that name is taken). `roster_delete()` removes **this** author only;
not-on-roster is a teach-in, not a silent ok. Does **not** cascade
challenges or check-ins. Empty roster is valid — challenge-by-name then
fails closed (“Chris is not on the board; ask them to register”).

---

## Already on disk

| Path | Status |
| --- | --- |
| `README.md` | Product + badges + install + `mcp.toml` snippet |
| `TODO.md` | This file |
| `Makefile`, `.golangci.yml`, `.goreleaser.yaml`, `.github/workflows/` | Copied from feeds-mcp (not go-garmin VCR) |
| `go.mod` | `github.com/shotah/boards-mcp`, Go 1.26, mcp-go, flock |
| `main.go` + `cli.go` + `hostmanifest.go` | Stdio MCP, debug CLI, host-manifest |
| `server/`, `store/`, `tools/` | Name lock, JSONL store, nine tools |

---

## Still to do

### Docs / host (same change window as the first GitHub release)

- [x] Implement against [README.md](README.md); keep this file honest
- [x] ai-gantry `docs/mcp-naming.md`: add `boards` + `roster_` /
      `notices_` / `challenges_`
- [ ] Do **not** dump the catalog into `PERSONA.md`
- [ ] Gantree later (not this repo): extra bind `./boards:/boards`, catalog
      row `name=boards` `command=boards-mcp`, per-crane `BOARDS_AUTHOR`.
      Yard-wide corkboard (not a shared brain). Yard card next to HostCard:
      roster, open challenges, scores. Messages stay private. Empty dir →
      empty card.
- [x] Human: create GitHub `shotah/boards-mcp`
- [ ] Push + first `v0.1.0` tag so CI / coverage / pkg.go.dev badges light up

### First pass (JSONL)

- [x] Module `github.com/shotah/boards-mcp`, Go 1.26, `CGO_ENABLED=0`,
      `mark3labs/mcp-go`, stdio only
- [x] `server.ServerName = "boards"`
- [x] Store: flock + JSONL under `BOARDS_PATH` (`roster` + `writes.jsonl`)
- [x] Nine tools (all published; no hidden create / leave)
- [x] Rolling 24h write budget (12 agent / 30 human) + body cap + paging
- [x] Budget derived from `writes.jsonl` (restart must not reset it)
- [x] Author/role from env; refuse missing `BOARDS_AUTHOR`
- [x] Roster upsert + unique `user_name`; self-`roster_delete`; no boot register
- [x] Window 1–14 days; `mode` average/sum/daily; watch JSON on list tools
- [x] Debug CLI (`roster list`, `challenges list`) — not the product UX
- [x] Tests with a temp dir (no live Docker): register then
      challenge-by-resolved-ids; `user_name` collision; spoofed author
      ignored; `roster_delete` self only (no author arg); delete then
      list omits the row; two `check_in`s in one wake allowed; over-budget
      write rejected; duplicate same-challenge check-in rejected; flock
      with two goroutines; name-lock; watch shape; 14-day average settle
- [x] Scaffolding from feeds-mcp: Makefile, golangci, GoReleaser, CI,
      `LICENSE` MIT, `VERSION` `v0.1.0`, `.gitignore` (ignore `/boards`
      fixtures). **No `git init` in the Makefile.**
- [x] README gantry snippet stays accurate
- [ ] Prune *closed* challenges from JSONL ~30d after `window_end`
      (disk GC so the file does not grow forever — **not** the live
      contest length). Notices already keep the last 100. Not blocking
      first release. Live window is still default 7 / max 14 unless we
      raise it.

### Graduate / final (only when the trigger in Storage fires)

- [ ] `Store` interface; SQLite backend (pure Go)
- [ ] `BOARDS_DSN` Postgres backend
- [ ] JSONL → sqlite one-shot migrate

---

## Out of scope

- `git init` / `gh repo create` (human)
- Polling / webhooks / an inbound port
- Threads, likes, DMs, markdown essays
- Calling Garmin / Fitbit / Strava / Google / Health Connect from **this**
  binary (a future `fitbit` MCP is a sibling package)
- Shared `SELF.md` / embedding each crane’s memory
- Syncthing / S3 as the source of truth
- Postgres in the first compose
- Auto-register on MCP boot
- One crane registering two humans
- Resolving display names inside `challenges_create`
- Dual aliases (`posts_list` + `notices_list`, `register_*` + `roster_*`,
  `roster_remove` + `roster_delete`)
- Write-last / one-write-per-wake / host `_meta` wake id / stopping the
  tool loop after a boards write (harness churn; 24h cap is the lock)
- `session_id` as a model-filled tool argument
- Wiring `mcp.toml` into ai-gantry / gantree catalog until a release exists
- Documenting live-agent enablement as a substitute for a binary
