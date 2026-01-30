# Design

## First principles

- **Problem**: maintainers are blind in a high‑noise PR firehose.
- **Goal**: compress into a daily report a human can scan in ~10 minutes.
- **Output**: ≤3 keepers, close candidates, top issues, with evidence.
- **Non‑goal**: perfect classification; we only need actionable signal.

## CLI shape (clig.dev‑style)

Use an idiomatic Go CLI library (match existing CLIs; **cobra** is the default in
padel‑cli/picnic). Keep commands small and explicit.

Proposed commands:

```
triage discover   # build rubric/taxonomy from corpus sample
triage run        # triage PRs (map/judge/reduce)
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
        ├── run-id.txt
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
- Shared files (`state.json`, `rubric.md`, reports) use advisory locks.
- Write to temp file → fsync → atomic rename.

## Repo sync + code context

- Repo clone lives at `<data-root>/repo/`.
- If repo cache missing: **clone** once.
- Every run: `git fetch --prune`, reset to `origin/<default>`.
- LLM may read repo files directly or use `gh`/`git` via bash when needed (run inside `repo/`).

## GitHub ingest (mechanical)

- Prewarm maintainers: `gh api /orgs/openclaw/members --paginate` → `maintainers.txt`.
- List open PRs via `gh pr list` (paged).
- Fetch PR JSON → `raw/pr-<num>.json`.
- Fetch PR files → `raw/pr-<num>.files.json` (additions/deletions/patch).
- Diff is **not** prefetched; LLM may fetch via `gh pr diff` if needed.
- Write `run-id.txt` once per run (used in triage-id).
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
- LLM reads fixed‑path files and **writes output files via toolcalls**.
- PR text is **untrusted and often adversarial**.
- Bash is allowed for `gh`/`git` when extra context is needed.
- No stdout/JSON parsing.
- Single source of truth per prompt (one obvious way).

## Triage vocabulary (fixed)

- **Label**: `good | slop | needs-human`
- **Action**: `keep | close | ask`
- **Confidence**: `low | medium | high`

## LLM pipeline

### Map (per PR)
LLM outputs a Markdown triage file (see `prompts/map.md`). It is a compact
"triage card": triage-id, reopened flag, label, action, confidence, summary,
taxonomy ids, evidence quotes, risks, notes, and optional context requests.

Example:

```
# PR Triage
PR: #123
Triage-ID: clawdinator-triage-<run-id>-pr-123
Reopened: no
Label: slop
Action: close
Confidence: medium

## Summary
- One line summary.

## Taxonomy
- docs

## Evidence
- "quoted line" (source)

## Risks
- none

## Notes
- short rationale
```

### Judge (optional)
Second LLM rewrites the triage file to comply with the rubric. Output is a
Markdown file in the same format (no accept/reject parsing).

### Reduce
LLM reads triage map files from disk and produces a daily report (Markdown)
with sections for keepers, close candidates, top issues, patterns, and notes.
Only open PRs should be included (check via `gh pr list`).

## Concurrency + limits

- Map stage runs a worker pool; each PR is a one‑shot LLM run.
- Reduce runs once per repo after map completes.
- Primary limits: GitHub API rate + LLM token/TPM budgets.

## Model + runtime

- Use **pi-golang** to run `pi --mode rpc`.
- Default model: **gpt-5.2-codex-medium** (override via flag).
- Prefer explicit provider/model/thinking (pi-golang dragons mode).

## Safety defaults

- No auto‑close.
- Always emit a daily report with a close‑candidates section for human review.
- All decisions are LLM outputs, never local heuristics (ZFC).
