# Ideas

Toolkit-scoped ideas progress from approved intent to observed implementation in
one self-contained directory.

## Structure

```text
docs/ideas/<idea-name>/
├── brainstorm.md       # Optional exploration and rejected alternatives
├── design.md           # Normative WHAT; reviewed and approved first
├── plan.md             # Executable HOW; added after design approval
└── implementation.md   # Observed result; added only after code lands
```

Supporting documents are allowed when the idea needs them, but tooling names
and agent runtime details do not define durable documentation directories.

## Lifecycle

1. Open an idea PR with `design.md`.
2. After design approval, add `plan.md` to the same PR.
3. Keep the idea PR open while implementation proceeds in its owning branch or
   repository. Keep design and plan current; stop for approval if implementation
   exposes an architectural change.
4. After implementation merges, add `implementation.md` with final commit/tag
   evidence, deviations, and nuances learned from the code.
5. Merge the idea PR after the implementation record and checks are reviewed.

The three files have different jobs:

- `design.md` is the approved contract and must not drift silently.
- `plan.md` is live execution state; completed, superseded, and blocked steps
  must be represented honestly.
- `implementation.md` is retrospective evidence, never a prediction or an
  advance placeholder.

## Active ideas

- [Actions are data](actions-are-data/) — implemented; includes design, completed plan, and implementation record
- [Encounter](encounter/)
- [Encounter anchoring](encounter-anchoring/)
- [Encounter transitions](encounter-transitions/)
- [Monster behavior](monster-behavior/)
- [NPC](npc/)
- [Play](play/)
- [Session SDK](session-sdk/)
- [Type-safe refs](type-safe-refs/)
- [World NPCs](world-npcs/)
