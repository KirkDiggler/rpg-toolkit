# 2026-11-02 · When things fire S1 · The session reads and folds by kind

processed: 2026-11-05T201556Z

Story [rpg-project#825](https://github.com/KirkDiggler/rpg-project/issues/825), slice [rpg-project#828](https://github.com/KirkDiggler/rpg-project/issues/828), design R1 B. Session PR [rpg-toolkit#2246](https://github.com/KirkDiggler/rpg-toolkit/pull/2246), on encounter [rpg-toolkit#2245](https://github.com/KirkDiggler/rpg-toolkit/pull/2245).

## Shipped

- `WalkAct.Kind` at [`act_bodies.go:39`](https://github.com/KirkDiggler/rpg-toolkit/blob/80a54f94/rulebooks/dnd5e/session/act_bodies.go#L39), with the session's own `MoveKind` and the same three words. `Forced` is kept, deprecated, and set to kind == displaced.
- `walkPiece` decodes `kind` and keys `absorb` on mover, cause and kind, so a segment breaks when the kind changes. A beat with no `kind` derives it from `forced` alone at [`act_family.go:292`](https://github.com/KirkDiggler/rpg-toolkit/blob/80a54f94/rulebooks/dnd5e/session/act_family.go#L292). A cause never decides it.
- `forcedBy` suppresses the opportunity attack only for a displaced step, at [`mover.go:155`](https://github.com/KirkDiggler/rpg-toolkit/blob/80a54f94/rulebooks/dnd5e/session/mover.go#L155).
- `compelledIntent` is the one Routed that names compelled. A host's own Routed stays own.
- `moveKindFor` at [`castmove.go:242`](https://github.com/KirkDiggler/rpg-toolkit/blob/80a54f94/rulebooks/dnd5e/session/castmove.go#L242) is the one crossing from content's `Provokes`: true is compelled, false is displaced.

## Decided

- An unknown `kind` word reads as own, the least permissive reading. It is not refused, because the fold reads a story and does not validate it.
- `WalkAct` JSON always carries `kind`, with no `omitempty`, like `forced`.

## Found

- `mover_internal_test.go` had no compelled control, so `TestACompelledStepStillProvokes` was added. A cause alone never suppresses.
- The branch cannot compile against a released encounter until #2245 is tagged. It pins the pushed encounter commit as a pseudo-version, and re-pins to the tag at merge-out.

## Mistaken

- The first head shipped without this record and with `go.mod` on the old encounter, so CI was red.

— cross-team agent, on behalf of KirkDiggler
