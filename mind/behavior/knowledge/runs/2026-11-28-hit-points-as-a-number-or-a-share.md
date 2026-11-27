# Run — 2026-11-28 — NPC behaviour 4a-2 hit points as a number or a share

Slice [rpg-project#1247](https://github.com/KirkDiggler/rpg-project/issues/1247) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`f87b7e0d`](https://github.com/KirkDiggler/rpg-toolkit/commit/f87b7e0d).

One line per entry. Every line ends with a link.

## Shipped

- `HitPointsWhen.AtMost` is hit points and `Percent` marks a share of the maximum; `holds` and `Validate` judge each in its own unit; README rows and one decisions line say both — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/36bbe26a).
- Six new tests in `hit_points_test.go` (raw number, share, same number two meanings, not told, each unit judged, the record), and the originals edited — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/36bbe26a).

## Decided

- The maximum guard sits before both forms, so a host that told nothing never makes a raw number read as nearly dead — [slice design 2](https://github.com/KirkDiggler/rpg-project/issues/1247).
- Edited existing tests (allowed): helper `atMost` and the `AtMost: 30` literals in TestOneConditionPerEntry gain `Percent: true`; the expected JSON in TestTheRecordReadsItAndAWhenWithoutItIsAsItWas gains `"Percent":true`; the refusal in TestTheNumberIsJudged gains the `%` — [allowed edits](https://github.com/KirkDiggler/rpg-project/issues/1247).

## Found

- F1 holds: `mind/behavior/v0.27.0` is the latest tag — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1247#issuecomment-7229051607).
- F2 printed two lines (type at table.go:553, comparison at :1476); the refusal sentence is split across two source lines — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1247#issuecomment-7229051607).
- F3 holds: helper and eight tests. F4 holds: build, vet and test passed before any change — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1247#issuecomment-7229051607).
- F5 printed: encounter data.go:2028, dungeonspec/compile.go:1330, dungeonspec/hit_points_test.go:35,42,51, dungeonspec/readback/hit_points_internal_test.go:18, hit_points_row_internal_test.go:19, hit_points_row_test.go:22; not touched, the next slice's — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1247#issuecomment-7229051607).
- Tests seen failing before the change (Percent field added bare): TestARawNumberIsHitPoints, TestEachNumberIsJudgedInItsOwnUnit, TestTheNumberIsJudged, TestTheSameNumberMeansDifferentThings, each on a real assertion. TestNotToldHoldsNothingInEitherForm and TestTheRecordReadsEachForm passed before the change (the old guard and omitempty already held) and are proven by (b) and (c) — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/36bbe26a).
- Proof (a) ignore Percent in holds: fails TestAShareIsOfTheMaximum, TestTheSameNumberMeansDifferentThings and four older tests — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/36bbe26a).
- Proof (b) guard dropped for raw: fails TestNotToldHoldsNothingInEitherForm — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/36bbe26a).
- Proof (c) omitempty removed: fails TestTheRecordReadsEachForm — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/36bbe26a).
- Suite before: pass. At final head: build, vet, test, golangci-lint (0 issues) pass — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/36bbe26a).

## Mistaken

- none observed — [commit](https://github.com/KirkDiggler/rpg-toolkit/commit/36bbe26a).
