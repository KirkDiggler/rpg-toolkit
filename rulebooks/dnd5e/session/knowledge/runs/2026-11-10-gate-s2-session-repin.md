# Run — 2026-11-10 — the entrance remembers (Gate S2) session repin

Slice [rpg-project#922](https://github.com/KirkDiggler/rpg-project/issues/922) of story [rpg-project#769](https://github.com/KirkDiggler/rpg-project/issues/769). Base: `rpg-toolkit` `main` at [`c921f229`](https://github.com/KirkDiggler/rpg-toolkit/commit/c921f229).

## Shipped

- Session pins encounter at the provider's pushed head `v0.190.1-0.20261110062610-2ad3ec2b8362` and `mind/behavior v0.23.0`; `play/record` follows to v0.4.0 transitively — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- `encounter.BeatInteraction` is record-only in `act_family.go`, and `TestFoldAnInteractionBeatIsNoAct` proves the beat folds into no act and no "has no family" piece — [rpg-project#922 Task 6](https://github.com/KirkDiggler/rpg-project/issues/922).
- `speak_test.go` builds `Requires` in the list form `[]encounter.Trigger{...}`, asserting the same thing — [rpg-project#922 Task 1](https://github.com/KirkDiggler/rpg-project/issues/922).

## Decided

- Test dice clamp to `min(face, size)` rather than tables or expectations changing: `faceDice`, `sequenceDice`, `scriptedDice`, `discoveryReviewDice` and `actorLoadCheckingDice`; the old picker landed the last row for a face above the die, and the die's top face lands that same row, so every expectation stays — [rpg-project#922 Task 6](https://github.com/KirkDiggler/rpg-project/issues/922).
- The interaction beat stays record-only in this slice; projecting it to a panel is the next slice's call — [rpg-project#922 Task 6](https://github.com/KirkDiggler/rpg-project/issues/922).

## Found

- Behavior v0.23.0 refuses a die face above the die size; with `faceDice` unclamped the castle kitchen launch suite (`TestLaunch_TemplateSuite`, `temper: { soldier: 1 }` on a fixed 10) fails, and passes clamped — [rpg-project#922 Task 6](https://github.com/KirkDiggler/rpg-project/issues/922).
- The only compile break the repin revealed in session is `speak_test.go:752` (`Requires` single trigger) — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).

## Mistaken

- none observed — [rpg-project#922](https://github.com/KirkDiggler/rpg-project/issues/922).
