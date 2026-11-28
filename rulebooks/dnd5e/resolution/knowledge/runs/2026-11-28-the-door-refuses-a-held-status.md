# Run — 2026-11-28 — one way to pay for an ability S8c the door refuses a held status

Slice [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`a8922931`](https://github.com/KirkDiggler/rpg-toolkit/commit/a8922931fe8fab4a2621856089c002503f2c7fe6).

## Shipped

- The effect machine's preflight asks `features.RefuseHeldStatus` for a character or a creature; a held status is refused as `ErrActivationRefused` wrapping "already raging", before anything is charged — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).
- `rulebooks/dnd5e` pinned from `v0.251.0` to `v0.253.0` — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).

## Decided

- The machine holds the description and the actor id, and asks from those; an actor not in the cast is left to the delivery's own `Start`, as the issue directed — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).
- `TestAUserWhoAlreadyHoldsTheStatusIsGivenItAgainAtTheDoor` became `TestAUserWhoAlreadyHoldsTheStatusIsRefusedAtTheDoor`, the one allowed edit — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).

## Found

- F3: `effectMachine` is named only in `effect.go`; no pause kind in `pause.go` (`machineBeforeRoll` .. `machineCheck`) names it, so it is never frozen and holds the description with no thaw to prove — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).
- F5: `a_monster_runs_its_ability_test.go` builds a monster participant (`captain`, Second Wind via `GrantedAbility`) and pays it at the door; the creature tests are built beside it — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).
- Test 6 measured: a longsword hit (die 5, STR 16) deals 8 calm and 10 raging, so the status adds its `damage_bonus` of 2; a slashing 6 on the creature deals 6 calm and 3 raging, halved — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).
- Suite at the pin (before any other change) and at the final head: `ok` both times; build, vet and `golangci-lint run ./...` (0 issues) clean — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).
- Proof (a), `Start` not asking: Tests 2 and 5 failed — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).
- Proof (b), refusal returned unwrapped: Tests 2 and 5 failed on "Target error should be in err chain" — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).

## Mistaken

- none observed — [rpg-project#1290](https://github.com/KirkDiggler/rpg-project/issues/1290).
