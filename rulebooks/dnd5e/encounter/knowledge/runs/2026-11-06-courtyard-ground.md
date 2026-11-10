# Run — 2026-11-06 — Courtyard S2 encounter ground

Slice [#874](https://github.com/KirkDiggler/rpg-project/issues/874), story [#867](https://github.com/KirkDiggler/rpg-project/issues/867). Base: `main` at `9922e218`.

## Shipped

- Encounter-module checkpoint adds optional region base/paint source, captured persistence and recipient-filtered atlas/discovery appearance; no SDK/API/Web adoption is claimed — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

- Folded the appearance into the canonical QA castle fixture through the Studio's native ground-edit/export helpers; its new route test walks the courtyard with host reloads and retains the captured ground — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

## Decided

- Convert authored paint once beside region cells, retain the detached authored copy for saving, and share one filter for full/partial delivered paint; refs remain opaque — [design/plan](https://github.com/KirkDiggler/rpg-project/pull/875).
- Refresh 25 content goldens only for the new optional runtime field's `Ground:null`; a structural comparison verified every prior value remained identical rather than accepting unexamined golden churn — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

- Keep the gatehouse and all other regions untouched; paint dirt between the existing gate/hall thresholds and open barracks/armoury edges within the courtyard's existing cells, because the agreed change is appearance rather than circulation — [S2 plan](https://github.com/KirkDiggler/rpg-project/pull/875).

## Found

- Snapshot and normal/concealment discovery tests establish per-cell paint filtering, including two secrets in one region; source tests preserve same-as-base paint and refuse null/empty/padded/orphan/conflicting assignments — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).
- Full encounter-module tests, including castle and replay packages, passed after the additive golden refresh; lint reported zero issues — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

- The actual served castle source matched the API and toolkit baseline byte-for-byte before editing. Studio export changed only courtyard ground; the compiled golden comparison likewise changed only that region's Ground value, with all existing gameplay/geometry facts identical — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).
- The new castle test first failed for absent ground, then passed its entrance appearances, unchanged partitioning and twelve one-cell moves with JSON host reloads after the content fold-back — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

## Mistaken

- The initial implementation updated atlas/reveal cuts but missed the reveal payload serializer; the new discovery test failed on missing ground, and the serializer now carries it — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).
- Empty base validation initially followed a return for absent paint; the base-only negative test exposed it, and validation now occurs before that return — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).
