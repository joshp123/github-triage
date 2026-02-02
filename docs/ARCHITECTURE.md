# Architecture

## Goal

Turn a high‑noise PR firehose into a **clear inventory snapshot** for openclaw
maintainers. Output is a single inventory file with counts + grouped lists.

## First principles (problem fit)

- **Input**: open PR firehose; noise dominates.
- **Constraint**: maintainers have limited time; slop ratio ≫ signal.
- **Desired output**: a single inventory snapshot (counts + grouped list).
- **Correctness**: classifications are evidence‑based; "good" should be rare without strong repo‑level signal.

## Constraints

- **ZFC**: no local heuristics or ranking; all judgment by LLM.
- **CLI only**: no daemon, no service.
- **Persistent data dir**: XDG data dir (`$XDG_DATA_HOME/github-triage`,
  required; fail fast if unset). Data is scoped per repo:
  `$XDG_DATA_HOME/github-triage/<org>/<repo>/`. In clawdinators set
  `XDG_DATA_HOME=/var/lib/clawd/memory`.
- **Safe by default**: no auto‑close, no remote mutations.

## System overview

```
┌────────────┐    ┌──────────────┐    ┌──────────────┐
│ GitHub API │──▶ │  Ingest +    │──▶ │  Map workers │
└────────────┘    │  Repo cache  │    │  (LLM)       │
                  └──────┬───────┘    └──────┬───────┘
                         │                   │
                         │                   ▼
                         │            ┌──────────────┐
                         │            │ Reduce pass  │
                         │            │ (LLM)        │
                         │            └──────┬───────┘
                         ▼                   ▼
                 ┌────────────────────────────────────┐
                 │ Data dir (rubric, map, inventory)  │
                 └────────────────────────────────────┘
```

## Data flow (map → reduce)

1. **Sync repo**: clone once, fetch each run.
2. **Ingest (mechanical)**:
   - prewarm maintainers → `maintainers.txt` (source: https://github.com/orgs/openclaw/people)
   - list PRs via `gh api graphql` (paged, state filter: open|closed|all)
   - write PR snapshot → `raw/pr-<num>.json`
   - write PR file paths → `raw/pr-<num>.files.json` (may be truncated)
   - compute `raw/pr-<num>.meta.json` (reopened flag)
   - optional enrich: full files + comments/reviews into `triage/comments/`
   - skip unchanged via `updated_at` cache
3. **Map**: `triage map` runs LLM classification → `triage/map/pr-N.md`.
4. **Reduce**: `triage reduce` runs LLM inventory snapshot → `triage/reduce/current.md`.

Optional: **Slop sweep** (fast pre-pass)
- `triage sweep` labels obvious slop vs needs-human.
- Writes cards to `triage/sweep/`.
- `triage close-queue` builds a close-ready list from sweep cards.
- Re-run full map on the needs-human subset if desired.

**File‑based map‑reduce**: prompts are static; the only input is PR number (or
DISCOVER/REDUCE). LLM reads fixed‑path files and calls **CLI write commands**
(no direct file writes). No stdout/JSON parsing.

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
   - LLM generates rubric from corpus sample.
3. **Triage**
   - Map + reduce; inventory snapshot.
4. **Feedback loop**
   - Maintainers edit rubric; optional “refresh rubric” mode.
5. **Later (explicitly out of scope now)**
   - Auto‑close and issue triage.
