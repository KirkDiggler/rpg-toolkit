# Run — 2026-11-10 — S6 PR 1: a kept encounter is rebound, and a gate holds it to a reload

Story [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879) "Alive, not lagging", slice S6 card [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927), PR 1 of 3. Base: `rpg-toolkit` `main` at [`178d11f4`](https://github.com/KirkDiggler/rpg-toolkit/commit/178d11f4).

## Shipped

- `(*Encounter).Rebind(*RebindInput{Capabilities})` binds a kept encounter to the next verb's capabilities in place of a reload. It replaces exactly the fields `LoadEncounter` binds, by the same rules, and nothing else. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- `ErrVerbInFlight` refuses a rebind while an act is open or a driven turn or world round runs. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- `rebind_test.go` is the equivalence gate. A seeded sequence of walks, strikes, falls, turn ends and a clang beside a listening post runs two ways. One copy is kept and rebound before every verb; the other is loaded from the stored checkpoint before every verb. After every verb the answer, the appended segment and the rest must be the same bytes. It runs on the small keep (40 verbs), its garrison (10 verbs) and the castle (12 verbs). — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- `TestRebindBindsEveryCapability` holds Rebind to every member of `Capabilities` by reflection, so a capability added later and not rebound fails by name. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)

## Decided

- Rebind is the encounter's, because the fields are unexported and the validation lives here. Session seams that read a current scope through a slot would rewrite every seam and hide the per-verb binding. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- Rebind takes `*RebindInput`, the module's input shape, not a bare `Capabilities`. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The id source and the site provider are not rebound. The id source is bound into the record at load, and the site is shared and read-only. The session passes neither per verb. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- Rebind clears no instance state. The gate found nothing to clear. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The garrison gate is allowlisted in `.github/testbudget/rulebooks/dnd5e/encounter.allow`. Only the garrison has listening posts and a patrol, and each verb loads 57 monsters twice. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)

## Found

- The gate caught no production difference. Across the three sites, a kept copy answered and wrote exactly what a reload did after every verb. The map's candidates hold: `struckBy` is cleared by the participation pass every train ends with, `lastLook` equals the saved high-water at a verb boundary, and `due`, `dueAt` and `listeners` stay in step with the machines. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- Ids differ between a kept copy and a reload on a real host, and both are right. The record draws entropy at every mint, so a kept id source and a fresh one step differently. The gate resets its counting source before every verb. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- A listening post hears nothing in a test whose sheets carry no passive Perception. The gate's sheets carry 40, or the listeners mutant survived. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)

Commands, from `rulebooks/dnd5e/encounter`:

```
go test -race -count=1 -run 'TestResidentCopy|TestRebind|TestAConcealed|TestCapabilities' -v .
go test -count=1 ./...
```

| test, under `-race`, site compile hoisted | local |
|---|---|
| `TestResidentCopyWritesWhatAReloadWrites` | 2.82 s |
| `TestResidentCopyWritesWhatAReloadWritesOnTheGarrison` | 5.17 s |
| `TestResidentCopyWritesWhatAReloadWritesOnTheCastle` | 3.28 s |

Mutation checks, each a line added to `Rebind` and restored:

| mutant | caught by |
|---|---|
| `lastLook` zeroed | small keep, verb 10 |
| a stale pending cell | small keep, verb 6; garrison, verb 1 |
| `dueAt` emptied | garrison, verb 5; castle, verb 3 |
| `due` emptied | garrison, verb 5; castle, verb 3 |
| `listeners` emptied | garrison, verb 9 (the clang); castle, verb 7 |
| `struckBy` left naming a striker | small keep, verb 19 (a fall to no blow) |
| `e.mover = in.Mover` dropped | `TestRebindBindsEveryCapability` |
| control: `sightReads` cleared | passes, as a cache should |
| control: `visibleCache` cleared | passes, as a cache should |

## Mistaken

- The first gate drew random hex steps and most walks hit walls, so most verbs were refused and never compared. Walks now follow the encounter's own route toward a seeded member. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)
- The first garrison run took 33 s under `-race`. Most of it was JSON round trips and opening 57 monsters per verb. The store now keeps bytes and compares them directly. — [rpg-project#927](https://github.com/KirkDiggler/rpg-project/issues/927)

## Verification

- The three gates, the refusals and the reflection test pass under `-race`; the module's `go test ./...` passes; `golangci-lint run ./...` reports 0 issues.
