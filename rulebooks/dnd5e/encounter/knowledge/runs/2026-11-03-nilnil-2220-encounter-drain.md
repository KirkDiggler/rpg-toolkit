# Run — 2026-11-03 — nilnil #2220 — the encounter drain

Part of [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220). Base: `rpg-toolkit` `main` at 4864537f. Ruling on the fix shape: [the issue's plan comment](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).

## Shipped

- Encounter's whole drain: 15 product sites converted, and the `^encounter/` exclusion deleted from both configs — the module lints clean under its own live config — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- Four new sentinels, each at its seam: `errNoEnding` (firedReachedPosition, 4 caller guards), `errNoPause` (pauseFrom, 1 caller), `errNoExperience` (preparedExperience, 1 caller), `errNoDelta` (memberPerceptionDelta, 0 callers — its existing guard was already loud); `errNoDrive` gained driveIfStillRunning's no-player site; `errNoSettle` gained settleWalk's moved-nobody site — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- Seven sites answer empty maps, not sentinels: the four `*Now` projections (conditions, equipment, sheets, sight), firstContactWithNewEnemies, refreshChangedStanding's closed and nothing-changed paths — the projection of nobody is empty, not absent — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- Six internal pins: the two seam pins from the first tranche's file, plus one per new sentinel — [clocks_internal_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/clocks_internal_test.go).

## Decided

- The `*Now` family and the two walkers carry no news — an empty roster's projection IS empty — so they return valid empty maps under "zero values tell the truth"; the pointer-shaped seams carry real news and answer sentinels. The split is the line between "nothing to report" and "this didn't happen" — [#2220 ruling](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).
- `memberPerceptionDelta`'s `(nil, nil)` was a latent nil dereference, not a quiet skip: its caller assigns the result into a map and dereferences it. The sentinel makes that defensive branch loud — [observation_subject.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/observation_subject.go).
- `firedReachedPosition`'s four callers keep their nil-outcome branches: the guard `!errors.Is(err, errNoEnding)` falls through with a nil outcome, which every caller already knew how to read — [step.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/step.go).

## Found

- The drain inventory had been capped a third time: a config with no `issues:` section defaults `max-same-issues` to 3, so the bypass scan read "encounter 3, session 3" when the truth was 17 and 22 (session's 22 = 10 product + 11 test + 1 cmd). The flags must be explicit on every measurement run — [#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- The #829 compelled-move work that landed between waves did not drain any of these sites, contrary to the first impression — [#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- The empty-map answers surfaced one nil-pinned assertion (`TestRecordActivationSecondWind`'s `s.Nil(out.IntelDeltas)`) — updated to `s.Empty`, which is the substance it always protected: nothing was produced — [activation_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/activation_test.go).

## Mistaken

- The third caps trap in one wave; the fix is procedural — `--max-same-issues 0 --max-issues-per-linter 0` on every nilnil measurement, no exceptions — [#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- One edit in encounter.go was a self-replace no-op caught on read-back; redone with the intended three changes — [encounter.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/encounter.go).