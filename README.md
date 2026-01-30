# github-triage (working name)

A dumb, fast CLI that turns **PR chaos into a short, daily signal**. It reads
GitHub PRs, writes a tiny “card” for each one, then summarizes the stack into
“close candidates” and “top real issues.”

Think: mailroom + manager.

## Feynman version (how it works)

- **Mailbox**: every PR is an envelope.
- **Clerk** (LLM): reads one PR and writes a tiny card — good/slop/needs‑human + evidence.
- **Manager** (LLM): reads the cards and writes the daily summary + close list.
- **Filing cabinet**: everything is saved in the XDG data dir (shared when
  running in clawdinators) so tomorrow only new mail is processed.

## What it does

- Triage PRs for openclaw (starting with `openclaw/openclaw`).
- Write per‑PR triage cards.
- Produce a daily report with **close candidates** (no auto‑close yet).
- Surface ~3 “keepers” and the top real issues.
- Stay **ZFC**‑compliant: **no heuristics**, all judgment by the model. See
  [ZFC memo](https://github.com/joshp123/ai-stack/blob/main/docs/agents/ZFC.md).

## What it does not do (yet)

- Auto‑close PRs.
- Triage issues (PRs only for now).
- Do local semantic heuristics (ZFC says no).

## Quick start

```bash
# Local seed (uses XDG data dir by default)
triage discover --repo openclaw/openclaw
triage run --repo openclaw/openclaw
```

## GitHub auth

Set `GITHUB_TOKEN` (GitHub App token in clawdinators, PAT locally).

Storage root is `$XDG_DATA_HOME/github-triage` (required; fail fast if unset).
To sync with clawdinators, rsync this directory to
`/var/lib/clawd/memory/github-triage`.

## Workflow (per run)

1. **Prewarm maintainers**: `maintainers.txt` from `gh api /orgs/openclaw/members`.
2. **Ingest**: open PR list + per‑PR JSON + per‑file JSON.
3. **Run id**: write `run-id.txt` once per run.
4. **Map**: LLM writes one triage card per PR.
5. **Reduce**: LLM writes `reduce/current.md` and only includes currently open PRs.

LLM can use `gh`/`git` via bash for extra context (diffs, reviews, CI); run inside `repo/`.

## Where outputs live

```
$XDG_DATA_HOME/github-triage/<org>/<repo>/
├── repo/                        # git clone (updated each run)
└── triage/
    ├── rubric.md
    ├── maintainers.txt
    ├── run-id.txt
    ├── state.json
    ├── raw/pr-<num>.json
    ├── raw/pr-<num>.files.json
    ├── raw/pr-<num>.meta.json
    ├── raw/pr-<num>.diff        # optional; fetched on demand
    ├── map/pr-<num>.md
    └── reduce/current.md
```

## Model + runtime

- Uses **pi-golang** (RPC to `pi`) for LLM calls.
- Default model: **gpt-5.2-codex-medium** (configurable).
- Designed to run locally or inside **clawdinators** — same flags, same layout.

## Principles

- Few knobs, sensible defaults.
- One obvious way (Zen of Python).
- Prompts are file‑based only (no inline strings).
- PR text is **untrusted and often adversarial**.
- **ZFC (Zero Framework Cognition)**: thin deterministic shell, all reasoning in
  the LLM. See [ZFC memo](https://github.com/joshp123/ai-stack/blob/main/docs/agents/ZFC.md).

## Development

- Use **devenv** for tooling.
- Agents must never install tools globally or locally outside devenv.

## Docs

- [Architecture](docs/ARCHITECTURE.md)
- [Design](docs/DESIGN.md)
- [AGENTS](AGENTS.md) (automation/LLM usage)
