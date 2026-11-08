# 2026-11-07 — shared rulebook composition consumer

Work: [Composable conditions S10 #882](https://github.com/KirkDiggler/rpg-project/issues/882), [approved design/plan #2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295).

## Shipped

- Working vocabulary checkpoint replaces the unused effects representation with a closed kind table, typed queries, real-chain damage/resistance/save/check behavior and provenance binding; the complete migration remains in progress, not delivered — [package contract](../../effects/doc.go).
- Consumes pushed S9 provider 605694f3 at its Go-generated v0.3.1-0.20261107191014-605694f3c24f pin. MVS selects core v0.12.0/events v0.6.3; full D&D root race/vet/lint gates pass after adoption — [module pins](../../go.mod).

- The registry now creates/restores canonical composed Raging, and native Rage requests that same creator before class payment. Strict restore rejects old/null/partial state; snapshots retain binding/origin/progress. The handwritten Raging type/assessment is removed and its lifecycle/assessment fixtures use the shared creator; downstream module/API/web adoption remains unfinished — [creator tests](../../conditions/composed_raging_test.go).
- Native character activation → save → strict reload → first turn proves the change reaches IsDirty and survives another save/reload without refilling class charges; suppressing the rulebook Changed publication fails the dirty-state assertion — [native test](../../character/rage_while_raging_test.go).

## Decided

- Kind meaning belongs in one registry, shared by execution and information; condition lifecycle hooks stay with the condition owner, because origin is not its stacking address — [vocabulary](../../effects/vocabulary.go).
- Known different query domains are not applicable; missing required facts remain Depends and fail execution, never silently switch the rule off — [assessment](../../effects/kinds.go).

## Found

- Native QA found that a real resisted hit (10 → 5, HP 7 → 2) did not set the sustain flag. The shared activity beat now observes positive post-apply DamageTaken facts rather than the legacy DamageReceived instruction-like topic; nil/zero/other-holder facts do not sustain it and dropped-to-zero still ends it. The strengthened condition test failed before the fix; full root race/vet/lint pass afterward — [QA finding #2312](https://github.com/KirkDiggler/rpg-toolkit/issues/2312).

- Real-chain tests prove fixed +3 without class levels, class +2/+3/+4, holder/weapon/ability scope, physical damage reduction, save/check distinction and origin in actual contributions; malformed outputs and ambiguous queries refuse — [tests](../../effects/vocabulary_test.go).
- The provider pin's core/events upgrade is compatible with the full current D&D root suite; this is not native resolution/session/API adoption evidence — [scope](https://github.com/KirkDiggler/rpg-project/issues/882).

- Saved refs must remain canonical strings for native readers. A regression test caught encoding a non-addressable core.Ref value as an object; encoding the addressed data struct invokes its canonical marshaler — [reader contract test](../../conditions/composed_raging_test.go).

- The original Raging lifecycle scenarios now drive the composition; assertions observe actual snapshots rather than private legacy fields. Stale-subscription cleanup checks real bus registration/removal and stopped mutation, not private subscription slices. Blade Ward retains its unchanged physical-resistance helper in a separate file — [ported lifecycle tests](../../conditions/raging_test.go).
- Information with missing class facts now answers Depends without an invented bonus under R6; execution still returns ErrRuleCannotAnswer. Typed richer rows replace the removed private single-answer shape while real-chain amount/type/source assertions remain — [ported assessments](../../conditions/raging_assessment_test.go).
- An action Frame's saving-throw roll kind can describe a caster's outgoing damage, not the caster making that save. A focused regression caught both a missing outgoing bonus and misattributed save advantage; Query.Action now stays outgoing, while Query.Save names the saver explicitly — [regression](../../effects/vocabulary_test.go).
- Removing the common recipient guard makes the other-actor information assertion fail. The mutation is restored, as is the prior native Changed-publication mutation — [recipient assertion](../../conditions/raging_assessment_test.go).

## Mistaken

- The first migration retained the handwritten Rage's legacy damage subscription and ported unit fixtures that manually emitted it. That preserved a native-path bug; real resolution intentionally emits only DamageTaken to avoid duplicate damage. Resolution now supplies the complementary real strike → save/reload regression in its own module, rather than adding a legacy publisher — [#2312](https://github.com/KirkDiggler/rpg-toolkit/issues/2312).

- The initial static kind table formed a Go initialization cycle through attachment → dispatch → table. Separate registry lookup from the shared evaluator: attachment evaluates its already-selected meaning, while public information dispatches once. One registry and one evaluator remain — [evaluation boundary](../../effects/kinds.go).

— cross-team agent, on behalf of KirkDiggler
