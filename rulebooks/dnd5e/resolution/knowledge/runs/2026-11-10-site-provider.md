# Run — 2026-11-10 — resolution's load takes the site provider

processed: 2026-11-12T165555Z

Leg 2a of S5, [rpg-project#914](https://github.com/KirkDiggler/rpg-project/issues/914), story [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879). Base: `rpg-toolkit` `main` at 008db1d6 (the encounter leg, [rpg-toolkit#2345](https://github.com/KirkDiggler/rpg-toolkit/pull/2345), merged). PR: [rpg-toolkit#2348](https://github.com/KirkDiggler/rpg-toolkit/pull/2348); reviewed at c6ef1daa.

## Shipped

- `Input.Sites` (the host's `encounter.SiteProvider`; nil is `encounter.DefaultSites`) and `Input.Site` (the stored site piece) reach the one `LoadEncounter` as supplied, so an injected provider is never bypassed by the process default; an unknown hash fails closed with `encounter.ErrUnknownSite` — [resolve.go](../../resolve.go).
- A site piece beside a lent world is refused with `ErrBadWorld`: it is the source of a world this package loads, and a lent world loads nothing.
- encounter pinned `v0.126.0` → `v0.188.0`, the first version with the site tier.

## Decided

- Both the provider and the piece, mirroring `LoadEncounterInput`, rather than the provider alone: the session resolves the piece from its repository with the verb's context and hands it on, so no repository call hides inside a provider that runs on a background context.

## Found: the pin jump moved three tests, none weakened

- `holdout_test.go`: a disposition's `Until` became a list of steps (`[]encounter.UntilStep`, rpg-project#717). The one step `{When: TriggerFact}` with no `Then` is the documented default (the other of hostile and neutral), so the camp turns exactly as the old single trigger did and every assertion stands.
- `live_test.go`: the record log now carries a minted id on every entry (wall clock and crypto/rand by default, play/record), so a world built twice differs in its ids. The test now builds ONE world and runs it both loaded and lent; that is the comparison the test states (the same data, two ways), and all its equalities still hold byte for byte.
- `answers_are_data_test.go`: a member's behavior machine now rides `EncounterData` with interface-typed values. The allow-list names each path with its reason: a recipe's bag, guard and set values are `mind/machine` scalars refused at recipe compile (`machine.ErrBadBag`, `machine.ErrBadGuard`); a saved progress blob's bag is checked on restore by `mind/behavior` (`checkCompositionBlob`, `ErrBadComposerData`). The walk still refuses every unlisted interface and every type with a `ToData`.
- Four cast tests that walled the field inline (`world.Field.Walls`) now compile a walled site through the process default (`withWall`, sitehelp_test.go). Same wall, same refusal asserted; the field is site content the checkpoint names by hash.

## Evidence

```
go test -race -count=1 ./rulebooks/dnd5e/resolution/...
golangci-lint run ./rulebooks/dnd5e/resolution/...
```

- Race suite passes; lint 0 issues.
- The AST pin on the one `LoadEncounterInput` literal names `Sites: in.Sites` and `Site: in.Site`; dropping either fails it.
