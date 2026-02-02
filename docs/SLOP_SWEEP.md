# Slop Sweep Options

Goal: identify close‑ready slop safely, without auto‑close.

## Option A — Conservative close‑queue (current)
- Run `triage sweep` over open PRs (writes to `triage/sweep/`).
- Only mark close‑ready if the sweep note says `close-ready: yes` for obvious spam/garbled/non‑English/empty PRs.
- Generate `triage/close/queue.md` from sweep cards.

## Option B — Consensus sweep
- Run sweep twice; keep only PRs labeled slop in both passes.
- Audit sample before closing.

## Option C — Cluster‑prioritized sweep
- Use doppelgangers clusters to pick large blobs first.
- Sweep those clusters and repeat.
