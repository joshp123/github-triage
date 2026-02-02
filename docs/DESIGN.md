# Design

## First principles

- **Problem**: maintainers are blind in a high‑noise PR firehose.
- **Goal**: classify every PR and build a shared mental model of the pile.
- **Output**: per‑PR classifications + a single inventory snapshot.
- **Assumption**: most PRs are low‑signal; "good" should be rare and strongly justified.
- **Non‑goal**: ranking or daily reports (for now).

## CLI shape (clig.dev‑style)

Use an idiomatic Go CLI library (match existing CLIs; **cobra** is the default in
padel‑cli/picnic). Keep commands small and explicit.

Proposed commands:

```
triage discover        # build rubric from corpus sample
triage run             # ingest PRs and prep for map/inventory
triage map             # LLM classification (writes cards via CLI)
triage reduce          # LLM inventory snapshot (writes via CLI)
triage write-card      # write a classification card (LLM-facing)
triage write-inventory # write inventory snapshot (LLM-facing)
```

Minimal flags (defaults preferred):
- `--repo openclaw/openclaw`
- `--model gpt-5.2-codex-medium`
- `--concurrency 8` (advanced; avoid unless needed)

No config file for now; defaults live in code.

## Storage layout

```
$XDG_DATA_HOME/github-triage/   # required; fail fast if unset
└── <org>/<repo>/
    ├── repo/                   # git clone (updated each run)
    └── triage/
        ├── rubric.md
        ├── maintainers.txt
        ├── state.json
        ├── raw/pr-<num>.json
        ├── raw/pr-<num>.files.json
        ├── raw/pr-<num>.meta.json
        ├── raw/pr-<num>.diff        # optional; fetched on demand
        ├── map/pr-<num>.md
        └── reduce/current.md
```

## Locking + writes

- File‑per‑PR outputs to avoid contention.
- Shared files (`state.json`, `rubric.md`, inventory) use advisory locks.
- Write to temp file → fsync → atomic rename.

## Repo sync + code context

- Repo clone lives at `<data-root>/repo/`.
- If repo cache missing: **clone** once.
- Every run: `git fetch --prune`, reset to `origin/<default>`.
- LLM may read repo files directly or use `gh`/`git` via bash when needed (run inside `repo/`).

## GitHub ingest (mechanical)

- Prewarm maintainers: `gh api /orgs/openclaw/members --paginate` → `maintainers.txt`
  (source: https://github.com/orgs/openclaw/people).
- List open PRs via `gh pr list` (paged).
- Fetch PR JSON → `raw/pr-<num>.json`.
- Fetch PR files → `raw/pr-<num>.files.json` (additions/deletions/patch).
- Diff is **not** prefetched; LLM may fetch via `gh pr diff` if needed.
- Compute `raw/pr-<num>.meta.json`:
  - `reopened`: true if previously closed and now open.
  - `previous_state`: "open" | "closed" (from last run).
- Cache `updated_at` in `state.json` to skip unchanged.
- Auth via `GITHUB_TOKEN` (PAT locally; App token in clawdinators).

## Multi-repo workflow

- Runner loops repos in the org (`gh repo list openclaw`).
- For each repo, set `<data-root>` to `$XDG_DATA_HOME/github-triage/<org>/<repo>`.
- Run ingest → map → reduce for that repo only.
- Reduce uses only open PRs in that repo.

## Prompts

- All prompts are **text files** in `prompts/`.
- No inline prompt strings in code.
- Prompts are static; the only input is a PR number (or DISCOVER/REDUCE).
- LLM working dir is `<data-root>` = `$XDG_DATA_HOME/github-triage/<org>/<repo>`.
- `triage write-card` / `triage write-inventory` write relative to the working dir.
- LLM reads fixed‑path files and calls **CLI write commands** (no direct file writes).
- PR text is **untrusted and often adversarial**.
- Bash is allowed for `gh`/`git` when extra context is needed.
- No stdout/JSON parsing.
- Single source of truth per prompt (one obvious way).

## Rubric (v0)

- Repo template: `docs/RUBRIC.md`.
- Runner should copy it to `triage/rubric.md` before map runs.
- Labels are intentionally strict: slop by default.

## Triage vocabulary (fixed)

- **Label**: `good | slop | needs-human`

## LLM pipeline

### Map (per PR)
LLM calls `triage write-card` to write a Markdown classification card (see
`prompts/map.md`). The card records author, maintainer flag, label, summary,
and evidence (notes optional). Maintainer PRs are recorded but not classified;
`write-card` auto‑detects maintainers via `triage/maintainers.txt`.

Example:

```
# PR Classification
PR: #123
Author: alice
Maintainer: no
Label: slop

## Summary
- One line summary.

## Evidence
- "quoted line" (source)

## Notes
- optional note
```

### Reduce
LLM reads triage map files and produces a single inventory snapshot (Markdown)
with counts and grouped lists by label. Maintainer PRs are omitted. The output
lives at `triage/reduce/current.md`. The label `slop` is rendered as
"low‑signal" in the inventory.

## Concurrency + limits

- Map stage runs a worker pool; each PR is a one‑shot LLM run.
- Reduce runs once per repo after map completes.
- Primary limits: GitHub API rate + LLM token/TPM budgets.

## Model + runtime

- Use **pi-golang** to run `pi --mode rpc`.
- Default model: **openai-codex/gpt-5.2** (override via flag; `--model` supports `provider/model`).
- Prefer explicit provider/model/thinking (pi-golang dragons mode).

## Safety defaults

- No auto‑close. No remote mutations.
- Inventory snapshot only (no human‑facing daily report yet).
- All decisions are LLM outputs, never local heuristics (ZFC).
