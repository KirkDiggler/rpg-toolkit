# Run — 2026-11-27 — NPC turn 1d-2 Dash is the second driven ability

Slice [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`0db836af`](https://github.com/KirkDiggler/rpg-toolkit/commit/0db836af).

## Shipped

- A driven Dash charges a monster's sheet one action at the door and lands nothing else, with the pin to `rulebooks/dnd5e v0.240.0` — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).
- `Input.Validate` refuses a driven Dash with no cost, a nil profile, or a payer other than the runner, with `ErrBadActivation` — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).
- The refusal for an undriven ability names both driven abilities, and the three docs say two abilities — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).

## Decided

- A price that charges nothing is no payment: the three spend maps are read in `chargesSomething` because the rules' `movesNothing` is unexported, and Grants and Requires do not count — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1207#issuecomment-7200408611).
- The Dash step lands nothing because an NPC's movement is the encounter's, ruled "I agree A" on 2026-11-27 — [ruling](https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7200191665).
- The payment refusals sit in `Input.Validate` as the issue's Design 3 says, because Start never sees the cost — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).

## Found

- F1 to F5 held: pin was `v0.238.0`, `CostOfDash` exists, the case and refusal sit at activation.go:760 and :767, the suite passed at both pins — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1207#issuecomment-7200334045).
- F6 printed one line, driven_activation_test.go:144 `not yet driven`, which still holds, so no existing test was edited — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1207#issuecomment-7200334045).
- A character's Dash charges the action and adds the speed to MovementRemaining on the character's sheet — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1207#issuecomment-7200334045).
- Proof (a), removing the Validate refusals, failed the payer test, and both no-payment cases (no cost, nil profile) with "Target error should be in err chain" — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).
- Proof (b), banking the walk speed on the sheet, failed the first test with expected 0, actual 30 on MovementRemaining — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).
- Proof (c), removing the Dash case, failed the charge, no-action, run-and-Disengage and no-target tests with "not yet driven" — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).
- The suite passed before the pin, after it, and at the final head — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).
- Proof for amendment 1, removing the charges-nothing refusal, failed the empty-profile and grant-only cases with "Target error should be in err chain" — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1207#issuecomment-7200408611).

## Mistaken

- The first head let an empty or grant-only profile through `validateDrivenDash`, so a Dash could charge nothing; the lead's review found it and amendment 1 closed it — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1207#issuecomment-7200408611).
- The first draft of proof (b) was a no-op edit and would have proved nothing; it was redone as a real banking change before the result was read — [rpg-project#1207](https://github.com/KirkDiggler/rpg-project/issues/1207).
