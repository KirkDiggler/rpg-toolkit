# Run — 2026-11-28 — rage 7e the session rages through the door

Slice [rpg-project#1281](https://github.com/KirkDiggler/rpg-project/issues/1281) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`8161083e`](https://github.com/KirkDiggler/rpg-toolkit/commit/8161083e).

## Shipped

- The session pins rules `v0.251.0` and the resolver `v0.87.1`; a barbarian rages through `Manager.Activate`, paid at the door — [pins commit](https://github.com/KirkDiggler/rpg-toolkit/commit/92d847aa).
- A described ability the resolver cannot build is `ErrBadActivation` with the resolver's sentence as text, in the described branch of `Manager.Activate`; its doc comment says Rage is paid at the door — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1281).
- Tests: `TestABarbarianRagesThroughTheDoor`, `TestRageSpentOrAlreadyRagingIsRefusedAndNothingIsSpent`, `TestADescribedAbilityNamedWithATargetIsAMalformedActivation` — [issue](https://github.com/KirkDiggler/rpg-project/issues/1281).

## Decided

- `actions.EffectProfile.Condition` is added to `selectorMechanicalFields` (the one guard entry): a named status is what the ability does, as `Healing` is; no production change — [amendment 1, gap 2](https://github.com/KirkDiggler/rpg-project/issues/1281).
- The malformed-activation fix sits in `activate.go` and `translateResolution` is untouched, because other verbs use it — [amendment 1, gap 1](https://github.com/KirkDiggler/rpg-project/issues/1281).

## Found

- Suite at the old pins: build, vet and the whole suite pass — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1281#issuecomment-7234706466).
- Suite after the pins alone: 3 failures (`TestAMalformedActivationIsNotAnAbilityRefusing`, `TestUnsuccessfulActivationAttemptsAppendNoStory/malformed_selected_activation`, `TestSelectorProjectionClassifiesEveryDefinitionField`) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1281#issuecomment-7234706466).
- Suite at the final head: build, vet, `golangci-lint` (0 issues) and the whole suite pass — [pins commit](https://github.com/KirkDiggler/rpg-toolkit/commit/92d847aa).
- F5 (`-run '(?i)rag'`, count of `--- PASS`): 52 at the old pins, 56 at the final head; the 4 added are the new Rage tests and their two subtests — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1281).
- Evidence the door paid: at rules `v0.251.0` the feature path refuses Rage, and the saved status names Rage and her id as its origin (`origin.ref` `dnd5e:features:rage`, `origin.entity_id` `alice`) — [issue](https://github.com/KirkDiggler/rpg-project/issues/1281).
- Before the fix, Second Wind and Action Surge named with a target answered `activate: resolution: invalid action definition: ... names nobody else; got targets [bob]`, the resolver's own sentinel, outside the verb's documented errors — [gap 1](https://github.com/KirkDiggler/rpg-project/issues/1281#issuecomment-7234706640).
- Test 2's two errors, both `activate: declaration is stale` (`ErrStaleDeclaration`): Rage with no charges, and Rage while raging — [issue](https://github.com/KirkDiggler/rpg-project/issues/1281).
- Tests 1 and 2 pass at the old pins too, as the issue allows — [issue](https://github.com/KirkDiggler/rpg-project/issues/1281).
- Proofs: (a) Test 1 expecting 2 charges fails with `expected: 2, actual: 1` ("one charge is spent"); (b) Test 2 expecting 0 charges after the second Rage fails with `expected: 0, actual: 1` ("nothing more is charged"); (c) removing the mapping fails the two existing tests and Test 3's two subtests — [issue](https://github.com/KirkDiggler/rpg-project/issues/1281).

## Mistaken

- The first F5 pattern `Rag|Rage` was case-sensitive and counted 2; the corrected command counts 52 — [gap 1](https://github.com/KirkDiggler/rpg-project/issues/1281#issuecomment-7234706640).
- The issue named `boundary_test.go` and `convert_test.go` for the type-walking guard; the one that failed was `selector_projection_test.go` — [gap 2](https://github.com/KirkDiggler/rpg-project/issues/1281#issuecomment-7234706813).
