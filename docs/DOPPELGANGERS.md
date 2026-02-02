# Doppelgangers Deep‑Dive (Architecture + Integration)

Source: https://github.com/badlogic/doppelgangers

## What it is
A Node CLI that fetches issues/PRs, builds embeddings, projects them to 2D/3D
(UMAP), and emits an interactive HTML viewer for manual triage.

## Pipeline (Doppelgangers)

1. **Fetch** (`src/triage.ts`)
   - Uses `gh api graphql --paginate` to fetch PRs (and optionally issues).
   - GraphQL query includes `number`, `title`, `body`, `url`, `state`, and
     `files(first: 20)` (paths only).
   - Writes a single JSON file (default `prs.json`).

2. **Embed** (`src/embed.ts`)
   - Builds text from title + body + files list.
   - Creates embeddings via OpenAI (or local `node-llama-cpp`).
   - Writes JSONL `embeddings.jsonl` with per‑item embeddings.

3. **Project + Build Viewer** (`src/build.ts`)
   - PCA → UMAP (2D + 3D), caches projections.
   - Emits `triage.html` with a scatter plot + selection tools.

## What we can reuse

### 1) Fast ingest (direct win)
- GraphQL can fetch **100 PRs per page with file paths**.
- This replaces per‑PR `gh api /pulls/{n}` + `/files` calls.
- **Win**: far fewer API round‑trips; ingest time drops.

### 2) Clustering to reduce LLM volume (indirect win)
- Embeddings + UMAP clusters similar PRs together.
- Human can inspect a cluster and decide whether to:
  - ignore it (slop), or
  - sample one or two PRs for deeper inspection.
- **Win**: fewer PRs sent to the LLM, faster runs, less cost.
- **ZFC‑safe**: clustering is *exploratory*, not automatic labeling.

### 3) Viewer (optional)
- The HTML scatter is a fast manual tool, but not required for integration.

## Proposed integration (ZFC‑safe)

**Stage A: Ingest**
- Port GraphQL fetch into `triage ingest` (done in this repo).
- Store PR snapshot + truncated file list under `triage/raw/`.

**Stage B: Cluster prep (optional, manual)**
- Enrich raw cache (full files + comments/reviews), optional:
  - `triage enrich --repo openclaw/openclaw --state open`
- Export `items.json` from the raw cache:
  - `triage cluster-export --repo openclaw/openclaw --state open`
- `items.json` includes title/body/files and (if present) comments/reviews.

**Stage C: Cluster (manual)**
- Run embeddings + UMAP to produce a cluster map.
- Manual step: pick clusters to investigate; pass selected PR numbers into
  `triage map --pr ...`.

**Key principle:** clustering never assigns labels; it only helps humans
choose which PRs deserve LLM time.

## Notes + Constraints
- File list from GraphQL is truncated; if a PR looks important, map prompt
  can fetch full diff via `gh pr diff`.
- We should keep the clustering tool separate from the core map/reduce path
  until the workflow is validated.
