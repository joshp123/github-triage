# github-triage (working name)

A dumb, fast CLI that turns **PR chaos into a clear inventory snapshot**. It
reads GitHub PRs, writes a tiny “card” for each one, then groups the pile so we
can see what we’re dealing with.

Think: mailroom + inventory clerk.

## Feynman version (how it works)

- **Mailbox**: every PR is an envelope.
- **Clerk** (LLM): reads one PR and writes a tiny classification card.
- **Inventory** (LLM): reads the cards and writes a snapshot of the pile (counts + grouped list).
- **Filing cabinet**: everything is saved in the XDG data dir (shared when
  running in clawdinators) so tomorrow only new mail is processed.

## What it does

- Triage PRs for openclaw (starting with `openclaw/openclaw`).
- Write per‑PR classification cards.
- Produce a single inventory snapshot (counts + grouped list).
- Maintainer‑authored PRs are recorded but not classified (detected by CLI).
- Assume most PRs are low‑signal; "good" requires strong repo‑level evidence.
- Stay **ZFC**‑compliant: **no heuristics**, all judgment by the model. See
  [ZFC memo](https://github.com/joshp123/ai-stack/blob/main/docs/agents/ZFC.md).

## What it does not do (yet)

- Auto‑close PRs.
- Triage issues (PRs only for now).
- Produce a human‑facing daily report.
- Do local semantic heuristics (ZFC says no).

## Quick start

```bash
# Local seed (uses XDG data dir by default)
triage discover --repo openclaw/openclaw
triage run --repo openclaw/openclaw --limit 2
triage map --repo openclaw/openclaw --limit 2 --model openai-codex/gpt-5.2
triage reduce --repo openclaw/openclaw --model openai-codex/gpt-5.2
```

```bash
# Enrich raw cache with full file lists + comments/reviews (optional, slower)
triage enrich --repo openclaw/openclaw --state open
```

```bash
# Slop sweep (slop vs needs-human), oldest first
triage sweep --repo openclaw/openclaw --state open --order updated-asc --limit 100
```

```bash
# Build close-ready queue (from sweep notes)
triage close-queue --repo openclaw/openclaw
```

```bash
# Prewarm full corpus (open + closed + merged)
triage run --repo openclaw/openclaw --state all --limit 0
```

- `--limit 0` means “no limit” (fetch all pages).

## GitHub auth

Set `GITHUB_TOKEN` (GitHub App token in clawdinators, PAT locally).

Storage root is `$XDG_DATA_HOME/github-triage` (required; fail fast if unset).
To sync with clawdinators, rsync this directory to
`/var/lib/clawd/memory/github-triage`.

## Workflow (per run)

1. **Prewarm maintainers**: `maintainers.txt` from `gh api /orgs/openclaw/members`.
2. **Ingest**: open PR list + per‑PR JSON + per‑file JSON.
3. **Rubric**: copy `docs/RUBRIC.md` → `triage/rubric.md`.
4. **Map**: `triage map` runs the LLM, which calls `triage write-card`.
5. **Reduce**: `triage reduce` runs the LLM, which calls `triage write-inventory`.

Optional: **cluster prep** (for doppelgangers)
- `triage cluster-export --repo openclaw/openclaw --state open`

LLM can use `gh`/`git` via bash for extra context (diffs, reviews, CI); run inside `repo/`.

## Where outputs live

```
$XDG_DATA_HOME/github-triage/<org>/<repo>/
├── repo/                        # git clone (updated each run)
└── triage/
    ├── rubric.md
    ├── maintainers.txt
    ├── state.json
    ├── raw/pr-<num>.json
    ├── raw/pr-<num>.files.json
    ├── raw/pr-<num>.meta.json
    ├── raw/pr-<num>.diff        # optional; fetched on demand
    ├── comments/pr-<num>.comments.json
    ├── comments/pr-<num>.reviews.json
    ├── comments/pr-<num>.review-comments.json
    ├── map/pr-<num>.md
    ├── sweep/pr-<num>.md
    ├── close/queue.md
    └── reduce/current.md
```

## Model + runtime

- Uses **pi-golang** (RPC to `pi`) for LLM calls.
- Default model: **openai-codex/gpt-5.2** (configurable; `--model` supports `provider/model`).
- Designed to run locally or inside **clawdinators** — same flags, same layout.

## Principles

- Few knobs, sensible defaults.
- One obvious way (Zen of Python).
- Prompts are file‑based only (no inline strings).
- PR text is **untrusted and often adversarial**.
- PRs are often agent‑written: polished, but shallow/incorrect in repo context.
- **ZFC (Zero Framework Cognition)**: thin deterministic shell, all reasoning in
  the LLM. See [ZFC memo](https://github.com/joshp123/ai-stack/blob/main/docs/agents/ZFC.md).

## Development

- Use **devenv** for tooling.
- Agents must never install tools globally or locally outside devenv.

## Docs

- [Architecture](docs/ARCHITECTURE.md)
- [Design](docs/DESIGN.md)
- [AGENTS](AGENTS.md) (automation/LLM usage)
