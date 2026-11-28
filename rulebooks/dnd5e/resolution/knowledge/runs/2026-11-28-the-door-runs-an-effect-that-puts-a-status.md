# Run — 2026-11-28 — one gate Rage 7b the door runs an effect that puts a status

Slice [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`f887da56`](https://github.com/KirkDiggler/rpg-toolkit/commit/f887da56).

One line per entry. Every line ends with a link.

## Shipped

- The effect machine runs a description whose effect puts a named status on its user: built at construction, paid at the door, delivered by the cast arm as a feature's status, told as an ActivationOutcome — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).
- The pin to rulebooks/dnd5e v0.250.0 and the guard allow-list for the sealed DC the new field reaches, `Actions.Effect.Condition.Save.DC` and `DirtyMonsters.Actions.Effect.Condition.Save.DC` — [gap and amendment](https://github.com/KirkDiggler/rpg-project/issues/1278#issuecomment-7234340893).

## Decided

- Design 2: a SIBLING, `prepareConditionFrom` with an origin, and `prepareCondition` delegates to it with nil, so the six callers do not change a character and an optional argument is not threaded through them — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).
- The status is built with no source ref and an origin of the ability and its user, exactly as Rage's own code builds it — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).
- A status that cannot be built is wrapped in ErrBadAction with the catalog's sentence; an older condition whose entry takes no origin is refused with "condition does not support Origin: <ref>" (Test 5 uses brutal_critical) — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).
- `preparedCast.conditionSource` carries the kind; the zero value stays "spell" — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).

## Found

- F3: `conditions []preparedDelivery` at activation.go:112, `ConditionSourceSpell` at :960, `prepareCondition` at contest.go:367, `publishCondition` at :558; six callers: action.go:997, activation.go:164, contest.go:1117, contest_pose.go:115, obey.go:246, strike.go:394 — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).
- Suite: green at origin/main before the pin; the pin alone failed `TestEntriesReturnAnswersNotSheets`; green after the allow-list commit and at the final head — [gap](https://github.com/KirkDiggler/rpg-project/issues/1278#issuecomment-7234340893).
- Test 2: the outcome (ability, kind, recipient, ref, description, detail, address, order), the condition-applied fact on the bus with its source kind, the saved raging condition (JSON), charges and economy are equal between the door and the old Rage, with the same member id; nothing differs — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).
- Test 8: the door does not refuse a user who holds the status: it charges again, replaces the instance (condition-removed with reason "replaced", then condition-applied) and she holds one raging — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).
- Proof (a) publishing as a spell fails Test 2 on the bus fact (source "feature" expected, "spell" actual); no outcome field or sheet byte shows the source kind. Proof (b) no origin: the raging definition itself refuses ("composed condition origin is required"), so every status test and Test 4's sentence fail. Proof (c) building in Start: Test 4 and Test 5 fail because NewAction no longer refuses; Start is also before the door charges, so nothing would have been charged — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).

## Mistaken

- The brief said the pin alone passes; it did not read `answers_are_data_test.go`, which refuses an interface field it has not been told about — [gap](https://github.com/KirkDiggler/rpg-project/issues/1278#issuecomment-7234340893).
- Test 4, written first, passed before the change for the wrong reason (the old "no supported body" refusal); it now asserts the catalog's sentence — [rpg-project#1278](https://github.com/KirkDiggler/rpg-project/issues/1278).
