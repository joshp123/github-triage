# Rubric (v0)

This rubric is intentionally strict. Most PRs should be labeled **slop** unless
there is strong, repo‑level evidence they are aligned and correct.

## Labels

### good (extremely rare)
- Small, targeted bugfix or regression fix.
- Minimal diff; no new feature, integration, or config surface.
- Clear repo‑level alignment and evidence of the underlying bug.

### needs-human (rare)
- Security/safety/tool‑policy/auth/provider changes.
- Core runtime behavior changes with unclear repo‑wide impact.

### slop (default)
- Docs‑only changes.
- New features/integrations or config surface expansion.
- Dependency upgrades.
- New skills (should go to https://www.clawhub.com/).
- Large or multi‑topic PRs.
- Vague or low‑signal PRs, even if they look polished.
- PR content is primarily non‑English or unreadable/garbled.

## Decision checklist
- Is this a small, targeted bugfix with clear evidence? If not, lean slop.
- Does it add a feature, integration, or config surface? → slop.
- Is it docs‑only? → slop.
- Is it security/auth/tool policy or core runtime behavior? → needs‑human.
