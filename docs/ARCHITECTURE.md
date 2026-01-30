# Architecture

## Goal

Turn a high‑noise PR firehose into a **small daily signal** for openclaw
maintainers. Output is a daily report: keepers, close candidates, top issues.

## First principles (problem fit)

- **Input**: open PR firehose; noise dominates.
- **Constraint**: maintainers have ~10 minutes/day; slop ratio ≫ signal.
- **Desired output**: a tiny daily report (≤3 keepers + close candidates + top issues).
- **Correctness**: report is actionable with evidence; nothing auto‑closes.

## Constraints

- **ZFC**: no local heuristics or ranking; all judgment by LLM.
- **CLI only**: no daemon, no service.
- **Persistent data dir**: XDG data dir (`$XDG_DATA_HOME/github-triage`,
  required; fail fast if unset). Data is scoped per repo:
  `$XDG_DATA_HOME/github-triage/<org>/<repo>/`. In clawdinators set
  `XDG_DATA_HOME=/var/lib/clawd/memory`.
- **Safe by default**: no auto‑close.

## System overview

```
┌────────────┐    ┌──────────────┐    ┌──────────────┐
│ GitHub API │──▶ │  Ingest +    │──▶ │  Map workers │
└────────────┘    │  Repo cache  │    │  (LLM)       │
                  └──────┬───────┘    └──────┬───────┘
                         │                   │
                         │                   ▼
                         │            ┌──────────────┐
                         │            │ Judge pass   │
                         │            │ (LLM)        │
                         │            └──────┬───────┘
                         │                   ▼
                         │            ┌──────────────┐
                         │            │ Reduce pass  │
                         │            │ (LLM)        │
                         │            └──────┬───────┘
                         ▼                   ▼
                 ┌────────────────────────────────────┐
                 │   Data dir (rubric, map, report)   │
                 └────────────────────────────────────┘
```

## Data flow (map → judge → reduce)

1. **Sync repo**: clone once, fetch each run.
2. **Ingest (mechanical)**:
   - prewarm maintainers → `maintainers.txt`
   - list open PRs via `gh` (paged)
   - fetch PR JSON → `raw/pr-<num>.json`
   - fetch PR files → `raw/pr-<num>.files.json`
   - compute `raw/pr-<num>.meta.json` (reopened flag)
   - write `run-id.txt` once per run
   - skip unchanged via `updated_at` cache
3. **Map**: per‑PR LLM triage → `triage/map/pr-N.md` (includes triage-id + reopened flag).
4. **Judge** (optional): LLM rewrites the triage file to match rubric.
5. **Reduce**: LLM produces daily report → `triage/reduce/current.md`.
6. **Report**: CLI can copy to a dated file if desired.

**File‑based map‑reduce**: prompts are static; the only input is PR number (or
DISCOVER/REDUCE). LLM reads fixed‑path files and **writes output files via
toolcalls**. Reduce includes only currently open PRs (via `gh pr list`). No
stdout/JSON parsing.

## Components

- **CLI runner**: orchestrates stages, flags, config.
- **GitHub adapter**: fetch PRs, diffs, metadata.
- **Repo cache**: local git checkout at `<data-root>/repo/`.
- **LLM client**: pi‑golang (RPC to `pi`).
- **Prompt runner**: static prompts; PR number as the only input.
- **Storage layer**: per‑repo XDG data dir with file‑per‑PR outputs.

## Multi-repo

- Runner loops repos in the org via `gh repo list`.
- For each repo, it sets `<data-root>` to `$XDG_DATA_HOME/github-triage/<org>/<repo>`
  and runs ingest → map → reduce.

## Plan (phases)

1. **Bootstrap (local)**
   - Repo sync + PR ingest + raw cache.
   - Manual rubric stub.
2. **Discovery**
   - LLM generates taxonomy + rubric from corpus sample.
3. **Triage**
   - Map + judge + reduce; daily report.
4. **Feedback loop**
   - Maintainers edit rubric; optional “refresh rubric” mode.
5. **Later (explicitly out of scope now)**
   - Auto‑close and issue triage.
