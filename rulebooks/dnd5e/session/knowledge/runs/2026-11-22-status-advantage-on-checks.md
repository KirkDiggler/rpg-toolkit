# Run — 2026-11-22 — status effect powers S-prayer: advantage on a line's check

processed: 2026-11-25T190427Z

Slice [rpg-project#1098](https://github.com/KirkDiggler/rpg-project/issues/1098) of story [rpg-project#1036](https://github.com/KirkDiggler/rpg-project/issues/1036). Base: `rpg-toolkit` `main` at [`7e19fae4`](https://github.com/KirkDiggler/rpg-toolkit/commit/7e19fae4).

## Shipped

- `prayer_status_test.go`: three played tests (no status rolls one d20; a given `advantage: checks` status rolls the next line's check with two d20 and names Prayed; the same after a reload of every store) — [rpg-project#1098](https://github.com/KirkDiggler/rpg-project/issues/1098).

## Decided

- The attack-side test (no Prayed source on an attack) was left out: a fight needs initiative and turn dice scripted around the same d20s, which the issue calls not cheap — [rpg-project#1098](https://github.com/KirkDiggler/rpg-project/issues/1098).
- A local `d20Script` roller scripts only d20s, because the answer pick draws its own die and ate a face of `sequenceDice` — [rpg-project#1098](https://github.com/KirkDiggler/rpg-project/issues/1098).

## Found

- A line's check roll rides the player's `session.SpeechAct` as `Roll` (`*Roll`): advantage is in `Roll.Influences[]` (`Kind: InfluenceAdvantage`, `Name: "Prayed"`, `SourceRef`, `SourceID`) and the dice in `Roll.Terms[0]` (`DieSize: 20`, `Rolled: [5 18]`, `Faces: [18]`, `KeptIndices`); `Roll.Dice` carries only the kept face — [act_roll.go](https://github.com/KirkDiggler/rpg-toolkit/blob/7e19fae4/rulebooks/dnd5e/session/act_roll.go#L22-L144).
- The status works end to end: same scripted dice miss (total 4) without it and pass (total 17) with it, and it survives a Manager rebuilt on the same stores — [rpg-project#1098](https://github.com/KirkDiggler/rpg-project/issues/1098).

## Mistaken

- The first run used `sequenceDice`, and the pick's draw consumed a scripted d20; replaced by `d20Script` — [rpg-project#1098](https://github.com/KirkDiggler/rpg-project/issues/1098).
