# Run — 2026-11-16 — studio today S12 the fold reads well for one thing

processed: 2026-11-25T190427Z

Slice [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980) of story [rpg-project#966](https://github.com/KirkDiggler/rpg-project/issues/966). Base: `rpg-toolkit` `main` at [`cdc616bd`](https://github.com/KirkDiggler/rpg-toolkit/commit/cdc616bd).

## Shipped

- `ReadBack.FoldedEach()` and `FoldedEachText()`: the fold without sharing across creatures and routines, and `cmd/readback --each` to print it — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).
- In both folds an answer says "at `speak`" and no more; each creature (and each side whose lines speak) carries one entry "At `speak`, {self}'s words reach …" standing for every answer it explains, by path — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).

## Decided

- One implementation with a `share` switch (`fold(share bool)`), so the two folds cannot drift apart — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).
- The loudness entry sits after a creature's posts and before its lines, and is said in both folds per creature even where `Folded()` names the lines as shared, because each creature's rows carry their own explanation in the full statements — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).
- The loudness entry shows "[said for every answer below at `speak`]" in the text form instead of its paths, through a new unexported `note`; its `Paths` still carry every row and told record it explains — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).
- A row's fact learners fold to "every member {self}'s words reach at `speak`, seen or not, {self} included, learns the fact …"; the explanation of the reach is the loudness entry's — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).
- Routine rows are left as they fold today: their short form already names the loudness by its verb and never explained the reach, so there was nothing repeated to fold — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).

## Found

- On the castle: `Folded()` was 404 items and 115,290 JSON bytes at main; now 418 items and 108,182 bytes. `FoldedEach()` is 483 items and 120,509 bytes; the statements are 388,919 bytes. `FoldedEach()` takes 0.8 ms and `Folded()` 7.7 ms (no race, 32 cores): the sharing step's comparisons are most of the fold's cost — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).

## Mistaken

- The first loudness entry used the `terse` display, which printed "[at each line's requires, above]", a note meant for the conversation's line list; it now has its own note — [rpg-project#980](https://github.com/KirkDiggler/rpg-project/issues/980).
- Review found the loudness entry's paths pinned on one row only (a first-path-only mutant and a nil told-record mutant both survived), a side's entry placed below the lines its note says are below it, and a folded fact clause that said "at `speak`" twice; a test now holds every explaining row and told record to its entry and the entry above the lines, and the three mutants fail — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2442#pullrequestreview-5912972554).
