# Run — 2026-11-12 — Courtyard S4 frame presentation at the host seam

Slice [Project #906](https://github.com/KirkDiggler/rpg-project/issues/906), story [Project #867](https://github.com/KirkDiggler/rpg-project/issues/867).

## Shipped

- Session implementation checkpoint: seam-owned frame DTOs, detached snapshot copies and atomic reveal decoding; consumers and release remain pending — [continuation](https://github.com/KirkDiggler/rpg-project/issues/906#issuecomment-6844016998).

## Decided

- Use a phase pointer because zero is a valid explicit fact while absence is malformed; preserve this distinction without an inner runtime type crossing the host seam — [wire contract](https://github.com/KirkDiggler/rpg-api-protos/pull/436).
- Reuse existing side/ref validation and refuse a malformed frame with all sibling reveal rows because partial installation would corrupt the recipient's layout cache — [continuation](https://github.com/KirkDiggler/rpg-project/issues/906#issuecomment-6844016998).
- Develop on the provider's pushed commit rather than merge or replace it because the wave must be integrated before publication — [provider PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2412).

## Found

- Snapshot and reveal copies preserve explicit zero, visual-only extents and legacy absence; mutating either side cannot reach the other — [provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2412).
- Adopted encounter `17ffaea59d76` through its actual pushed-commit pseudo-version so the host uses the cold-SiteData missing/null-phase refusal; this changes no seam DTO or valid captured bytes and still requires a released tag before merge — [review fix](https://github.com/KirkDiggler/rpg-toolkit/pull/2412#discussion_r4562032802).
- Final full module race tests pass, including the host-shape guards; lint reports zero issues. API/Web integration and real captured play are not established by these checks — [slice](https://github.com/KirkDiggler/rpg-project/issues/906).

## Mistaken

- The first full run exposed field-roster guards that still described the old contract; reading only the log tail initially missed the second guard and cost a repeat. Both now name the reviewed fields, with nested guards preserving the ban on parent identity, full wall-origin distance and mutable state — [contract](https://github.com/KirkDiggler/rpg-api-protos/pull/436).
