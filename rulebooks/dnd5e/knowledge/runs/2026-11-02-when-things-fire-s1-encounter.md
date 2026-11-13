# 2026-11-02 · When things fire S1 · The encounter names the kind

processed: 2026-11-12T165555Z

Story [rpg-project#825](https://github.com/KirkDiggler/rpg-project/issues/825), slice [rpg-project#828](https://github.com/KirkDiggler/rpg-project/issues/828), design R1 B. Encounter PR [rpg-toolkit#2245](https://github.com/KirkDiggler/rpg-toolkit/pull/2245).

## Shipped

- `MoveKind` (`own`, `compelled`, `displaced`) at [`movekind.go`](https://github.com/KirkDiggler/rpg-toolkit/blob/b1bc3db2/rulebooks/dnd5e/encounter/movekind.go). The zero value reads as own. `Displaced()` is true for one word only.
- `DirectInput.Kind` replaces `Provokes`, and `MoveStep.Kind` replaces `Forced`. `Direct` refuses anything but compelled or displaced. The Routed arm refuses displaced. Both refusals use `ErrBadMoveKind`.
- The moved beat writes `kind` for compelled and displaced, and nothing for own. `forced: true` is written exactly when the kind is displaced.
- Held directives and paused turns store `move_kind` and resume with it, through a second hold too.

## Decided

- The pause's field is `moveKind` at [`pause.go:135`](https://github.com/KirkDiggler/rpg-toolkit/blob/b1bc3db2/rulebooks/dnd5e/encounter/pause.go#L135). `pause.kind` already names the `PauseKind`.
- `walkPath` spells a zero kind as `own` at [`clocks.go:1598`](https://github.com/KirkDiggler/rpg-toolkit/blob/b1bc3db2/rulebooks/dnd5e/encounter/clocks.go#L1598). A reloaded pause reads its kind back as a word, so a Mover is told the same word before and after a reload.
- A stored `move_kind` that disagrees with `forced`, or an unknown word, is refused on load at [`movekind.go:78`](https://github.com/KirkDiggler/rpg-toolkit/blob/b1bc3db2/rulebooks/dnd5e/encounter/movekind.go#L78). The two are written together, so a disagreement is corruption.

## Found

- A legacy directive pause with `forced` false loads as own, as the plan rules. Every such pause was a Dissonant Whispers flee, which is truly compelled. Own still provokes, so behaviour is the same, but its resumed beats carry no `kind`.
- The re-pause sites at `held.go:103` and `pause.go:1049` were unguarded by any test until the second-hold cases in `movekind_test.go` (review of #2245).

## Mistaken

- The first head shipped without this record and without the second-hold tests. Both were added on review.

— cross-team agent, on behalf of KirkDiggler
