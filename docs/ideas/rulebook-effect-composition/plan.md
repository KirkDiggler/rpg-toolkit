# Shared Raging composition — implementation plan

> Execute directly under the workspace build-slice contract and test-driven development. The operator approved the complete design; this checked plan introduces no further planning-approval gate. Checkboxes are execution state, not claims of results.

**Goal:** one rulebook-owned Raging composition used by native Barbarian activation and the item example, with current-format persistence and honest native information/readers.

**Architecture:** conditions owns the registry, validated immutable bindings/origin and codec; D&D effects owns reusable mechanical meanings; mechanics/effects owns lifecycle execution and neutral observations. Hosts persist values and render owned facts, never interpret Rage rules.

**Tech stack:** Go 1.27, existing event/chain packages and testify suites; React/TypeScript/Vitest for the existing badge reader; normal API image and isolated walk environment.

**Spec:** [approved design](design.md), [walkthrough](README.md), [design PR #2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295), ADR-0048 plus ADR-0050 for the current persisted contract.

## Authority, baseline and finish line

KirkDiggler approved the complete design and proceeding on 2026-11-07. R1–R7 are settled. No legacy Rage reader/writer/normalization or automatic character reset is permitted.

Inspected baselines: toolkit main `3972f948`, API dev `590a201a`, web dev `2150256f`. The design worktree is `rpg-toolkit/.worktrees/rulebook-effect-composition`. Initial D&D conditions/features tests pass. Source evidence and initial drift are in the design PR/run record; web reader drift is web#1396.

Finish line: bounded slices below built/tested on real pushed pins, an isolated stack booted for the operator's early look, native journey verified, applicable independent review/dispositions published, and authorized inside-out release/tag adoption recorded. No source-stage pass substitutes for native persistence or reader proof. Design approval authorizes implementation, not unrequested merges, deleting character data or touching the operator's stack.

## Global constraints

- Preserve native Rage gameplay: class-derived +2/+3/+4 at Barbarian 1/9/16, Strength melee weapon predicate, physical resistance, Strength save/check advantage, grace/clock/sustain/end ordering and by-ref identity.
- Fixed item binding is positive and persists independently of item attachment; do not create wearer class levels or change item/class payment ownership.
- One registry, one recipe builder, fresh instances, one mutable bag; immutable bindings/origin are separate. No per-source Rage implementations or JSON-to-JSON runtime bridge.
- Other condition constructors and formats remain unchanged; no feature-to-actions migration, studio, inventory, new eligibility, concentration or rollback policy.
- One nearest Go module per toolkit runtime PR. Separate worktrees/branches; no replace/go.work, force-push, rebase or manual tags.
- Native information keeps its current attack/save Frame boundary. Four described benefits do not imply all four apply to an attack.

## Review focus and concrete proof

1. Snapshot/output aliasing could corrupt runtime without a gameplay action → S9 copy/retained-observation tests.
2. First-turn state can change without the host saving it → S9 nonterminal Changed ordering and S10/S12 real dirty/save/reload tests.
3. Missing current saved fields can silently re-grant grace or charges → S10 strict restore matrix and S11 spent-loot/restored-effect tests.
4. Origin can accidentally become a stacking qualifier or disappear from rows → S10 same-ref address and source row tests; S12 native declaration projection.
5. A Strength attack can accidentally receive save/check advantage, or a badge can compute class rules → S10 typed query-domain matrix and S12 canonical-reader tests/walk.

## Slice sequence and worktrees

| Slice | Deliverable | Release units / named worktrees |
|---|---|---|
| [Composable conditions S9 #881](https://github.com/KirkDiggler/rpg-project/issues/881) | Neutral effect validation/query/change-notification support | `mechanics/effects`; `.worktrees/raging-effects-support` |
| [Composable conditions S10 #882](https://github.com/KirkDiggler/rpg-project/issues/882) | One shared rulebook Raging definition and native creator | `rulebooks/dnd5e`; `.worktrees/raging-rulebook` |
| [Composable conditions S11 #883](https://github.com/KirkDiggler/rpg-project/issues/883) | Item/loot consumer of the shared creator | `examples/items`; `.worktrees/raging-items` |
| [Composable conditions S12 #884](https://github.com/KirkDiggler/rpg-project/issues/884) | Native load/act/save/information and reader adoption | separate `resolution` and `session` module PRs in `.worktrees/raging-resolution`, `.worktrees/raging-session`; protos `.worktrees/raging-provenance`; API `.worktrees/raging-composition`; web `.worktrees/raging-composition` |

Current source heads, measured gates and incomplete publication/native proof are recorded in [implementation.md](implementation.md). Source implementation is not release completion.

S9 → S10 → S11 and S12. Within S12: resolution → session → API; web reader can be built independently against the agreed current data fixture but joins the real stack before readiness. Runtime branches start from the owning base (`main` toolkit, `dev` API/web); consumers get exact pushed provider SHAs through Go tooling. A fresh builder obtains the design/plan from #2295, not from assumed copies in a runtime branch.

### Task 0 — accepted record and tracked slices

**Owner/files:** design branch; `design.md`, `README.md`, this plan; `docs/adr/0050-composed-condition-instances.md`, ADR indexes; root run record. Four project sub-issues under #703, with checklist/PR links.

- [x] Record full design approval/R1–R7 and the accepted ADR before runtime changes.
- [x] Create S9–S12 sub-issues (#881–#884) and connect them to #703; keep their evidence/heads current.
- [x] Check requirement/seam tables below and local links, ADR digest, whitespace and genre findings. Publish the plan on #2295.

**Done:** visible accepted contract and a bounded execution record; no invented implementation result.

### Task 1 / S9 — neutral provider support

**Files:** modify `mechanics/effects/{composed.go,vocabulary.go,doc.go,README.md}` and existing test vocabulary; create `snapshot.go`, `observations_test.go` and module run record. Machine implementation remains unchanged.

**Produces:**
- `type Snapshot struct { State string; Bag machine.Bag }` with JSON names `state`, `bag`; `(*Composed).Snapshot() Snapshot` returns an owned scalar-bag copy.
- `(*Composed).CurrentOutputs() []*Bound` returns a defensive slice in current recipe order. Bound is observational; `Params()` returns owned bytes and `Host()` returns detached ref values. Retaining/changing a returned slice/metadata/snapshot cannot alter the runtime.
- `KindSpec.Check func(Output) error` replaces the params-only callback. All affected in-module fixtures change in the same PR; no parallel compatibility hook.
- Successful nonterminal transitions attach new outputs, then invoke Changed. Tracker/staying mutations keep existing behavior. Terminal uses End-before-detach and does not publish a subsequent Changed.

**Tests / cycle:**
- [ ] Add `TestSnapshotAndOutputObservationsAreOwned`: mutate saved bag, output slice, params, holder/source refs; later observation and real damage remain unchanged; retained snapshot does not advance with the live turn.
- [ ] Add `TestFullOutputValidationRefusesBeforeAttachment`: a kind requires an amount; a missing amount fails Compose before any subscription. Check sees kind/guard/amount/params rather than only raw params.
- [ ] Add `TestNonterminalTransitionPublishesChangedAfterAttachment`: first turn changes state/bag; callback observes new state, turn count and new outputs on the real test bus. Failed new-output attachment does not claim a successful change.
- [ ] Run tests RED for absent capabilities/notification, implement the above, run GREEN; retain terminal publish-before-detach and import-seam assertions.
- [ ] Mutate away nonterminal Changed and defensive-copy behavior; targeted assertions must fail, then restore.
- [ ] Run module `go fmt ./...`, `go mod tidy`, `go test -count=1 -race ./...`, `go vet ./...`, `golangci-lint run ./...`; record evidence and push a draft provider PR for real consumer pins.

### Task 2 / S10 — reusable D&D vocabulary

**Files:** replace unused `rulebooks/dnd5e/effects/types.go`; add focused `doc.go`, `query.go`, `vocabulary.go`, `kinds.go`, `amounts.go` and suite tests in that package. D&D root go.mod/go.sum pins the pushed S9 provider.

**Produces:**
- `effects.Query` with optional `Action *contributions.Frame`, `Incoming *events.IncomingDamageEvent`, `Save *events.SavingThrowChainEvent`, `Check *events.AbilityCheckChainEvent`; more than one is invalid, none requests descriptions without applied benefits.
- `effects.BindInput{Origin events.ConditionOrigin, DisplayName string}` → `Bind(*BindInput) (*BindOutput,error)` with `Machine machine.Vocabulary`, `Outputs genericEffects.OutputVocabulary`; fresh tables from one private kind registry, no items/conditions import.
- `effects.AssessInput{Query Query, Output *genericEffects.Bound, Origin events.ConditionOrigin, DisplayName string}` → `Assess(*AssessInput) (*AssessOutput,error)` with one typed `contributions.Effect`. Generic KindSpec.Assess projects that same assessment into its narrow Row, not a second predicate/amount implementation.
- Closed kind constants `damage_bonus`, `resistance`, `advantage`. Resistance params name nonempty valid damage types; advantage params select `saving_throw` or `ability_check`. Fixed and by-class-level are the only amount functions installed in this cut. Kinds validate full output requirements and applicable frame identity.

**Tests / cycle:**
- [ ] `TestDamageBonusUsesRealFrame`: base 10 → 12/13/14 for class levels 1/9/16, and →13 for fixed +3 without class levels. Dexterity/ranged/nonweapon cases do not acquire the bonus; missing required facts fail execution honestly.
- [ ] `TestResistanceAndAdvantagesUseTheirOwnDomains`: physical vs fire damage; Strength vs Dexterity saves; Athletics vs non-Strength checks. Known wrong query domain is DoesNotApply, absent relevant facts Depends, no context only descriptions.
- [ ] `TestKindValidationAndOrigin`: unknown/malformed/extra params, missing amount, invalid types/domain/refs fail before attachment; rows and execution retain supplied entity provenance, not a hard-coded item or Raging name.
- [ ] Run RED/GREEN with real event chains; check imports refuse examples/items and conditions. Runtime mutation probes drop holder filtering/domain selection and must fail the targeted tests.

### Task 3 / S10 — shared creator, codec and native Rage

**Files:** extend `events/events.go`, `conditions/{descriptor.go,registry.go,factory.go,loader.go,action_effects.go}`, `contributions/effect.go`; add `conditions/{composed.go,composed_data.go,raging_definition.go}` and focused suites; update `features/rage.go` plus Rage/character/loader tests. Remove `conditions/raging.go` and `raging_assessment.go` once their assertions are carried through the real composition. No old-type alias or decoder.

**Produces:**
- `events.ConditionOrigin{Ref core.Ref, EntityID string}`; optional `ConditionOriginProvider` returns that value. It is distinct from ConditionAddressProvider.
- `CreateFromRefInput.Origin *events.ConditionOrigin`; Raging requires Origin, strict Config bindings, recipient and canonical ref; it refuses the legacy SourceRef field. Legacy descriptors refuse Origin rather than discard it, keeping their existing SourceRef contract.
- `RagingBindings{DamageBonus RagingDamageBonus}` where `RagingDamageBonus{Fixed *int, BarbarianLevel bool}` selects exactly one mode. Required fixed value is positive; class selector does not contain a cached level/amount. Unknown fields/trailing JSON are refused.
- `ValidateBindings(*ValidateBindingsInput) error`, input `{Ref string, Bindings json.RawMessage}`, routes through the same descriptor binding validator without fake holders or runtime creation. Non-composed refs refuse this operation.
- `ComposedData{Ref core.Ref, MemberID string, Origin *events.ConditionOrigin, Bindings json.RawMessage, Runtime *genericEffects.Snapshot}` with the JSON names in the walkthrough. Pointers distinguish missing/null saved sections. Runtime state must be live; bag is complete, typed, exact-keyed and lifecycle-consistent. Old format is refused.
- `ComposedCondition` delegates execution to the generic runtime, holds immutable binding/origin values once and serializes a typed Snapshot (no JSON parse bridge). Its origin is observable independently of its empty by-ref stacking qualifier.
- Descriptor-owned binding builder returns validated canonical binding bytes plus one plain recipe. Creation/restoration use it; no second condition registry.
- Optional `contributions.ActionEffectsAssessor.AssessActionEffects(*AssessActionInput) (*AssessEffectsOutput,error)` returns `Effects []Effect`; existing aggregator handles those rows as well as legacy ActionAssessor. Unique output IDs, source provenance and domain-honest decisions survive mapping; legacy grouping/row validation stays intact.
- Native Rage prepares via CreateFromRef before its existing resource payment/delivery. Its source is `(refs.Features.Rage(), owner.GetID())`, class binding explicit. Existing eligibility/payment/publication rules remain.

**Tests / cycle:**
- [ ] `TestSharedRagingCreation`: class and item inputs use the same registered definition and fresh state; no class-level fabrication, no shared bags, by-ref address unchanged; missing Origin/Config/mixed legacy SourceRef refused.
- [ ] `TestComposedRagingStrictRestore`: fresh/live/near-expiry snapshots round-trip through LoadJSON with origin and bindings; old JSON, missing/null runtime, partial/extra/wrong-kind bag, unknown refs/state, terminal/inconsistent state and invalid binding fail rather than reset.
- [ ] `TestRagingLifecycleParity`: retain actual grace, attack-attempt including miss, hit sustain, inactivity, late anchor, expiry, backwards clock, rest, combat-end and zero-HP assertions on public topics. Fixture representation may change; behavior assertions may not disappear.
- [x] QA correction #2312: the shared activity beat observes positive post-apply DamageTaken, never the legacy instruction-like DamageReceived topic. Root test consumes grace before testing hit sustain; resolution `TestNativeDamageSustainsRagingAcrossReload` exercises an actual resisted NPC strike, one application, dirty save/reload, one sustain, then inactivity. Both failed before the fix; full owning gates pass. Affected live recheck and focused review closure remain pending.
- [ ] `TestRagingInformationHasFourDomainHonestRows`: damage/resistance/save/check descriptions are distinguishable with unique IDs/origin; Strength attack does not receive save/check advantage. Typed context queries agree with execution and do not mutate/publish/spend.
- [ ] `TestNativeRageUsesSharedCreatorAndPersistsFirstTurn`: real class feature publishes new ConditionBehavior, character-owned payment unchanged, first turn makes the sheet dirty; ToData/load restores state and progress. Preparation refusal never spends; no new delivery rollback promise.
- [ ] Port concrete-type/private-field tests to factory/ref/current-data observations, preserving meaningful assertions; run full D&D root race/vet/lint, not only new suites. Add creation/load/source/lifecycle mutants and record which assertions reject them.
- [ ] Update docs/source map and module run record, push root PR on real S9 pin. Do not merge merely to unblock consumers.

### Task 4 / S11 — shared item consumer

**Files:** `examples/items/{item.go,rage_recipe_test.go,rage_vocabulary_test.go,rage_harness_test.go,rage_item_test.go,loot_example_test.go,README.md,RAGE.md,go.mod,go.sum}` and module run record.

**Consumes:** Tasks 1–3 exactly. Item attachment's own generic Check callback accepts full Output. Experimental Rage grant parameters carry RagingBindings; ValidateGrant delegates to the registry validator, and Resolve calls CreateFromRef with item origin. Remove the private Rage recipe/vocabulary implementation; no production item API promotion.

- [ ] Adapt tests first: a +3 retrieved item produces 13 on base 10 through the shared creator; no native class levels are supplied. Restore the resulting active condition separately after item detach/transfer, retaining source and bound amount.
- [ ] Preserve item identity/spent pools, missing/null stored instance refusal and commit-only-after-successful-delivery assertions; ensure class and item budgets remain separate in sequential same-holder use.
- [ ] Remove duplicate consumer Rage mechanics, pin pushed provider/root commits, and run full module race, both examples, vet and lint. Source grep/import inventory must show no local recipe/vocabulary engine left.
- [ ] Publish draft and its actual pin/test receipts; examples remain untagged.

### Task 5 / S12 — native resolution/session/API adoption

**Separate toolkit PRs:** resolution then session; neither may add rules or own a new bus. Pins use pushed root/resolution commits during development.

**Files:** resolution `go.mod/go.sum`, Rage fixtures in `resolve_test.go`, `check_test.go`, `contest_test.go`, `dirty_test.go`, `long_rest_test.go`, `contest_fold_test.go`, `cost_test.go`, `damage_custody_test.go`; add `raging_composition_test.go`. Session `go.mod/go.sum`, Rage fixtures in `conditions_test.go`, `death_test.go`, `information_test.go`, `clockboundary_test.go`; add `raging_composition_test.go`. Inventory concrete Raging type references before changing; update every affected fixture in these modules, not unrelated callers pinned to old versions.

- [x] Resolution `TestRagingCompositionSurvivesInteractionReload` (#2304): native activation, actual turn end, dirty output and next interaction reload preserve current state/bindings; expiry/removal exact; no extra clock grace. Exercise real resolution bus/collector, not an adapter mock.
- [x] Session native load/act/save/information proof (#2306): extend existing `TestRagingAgainIsRefusedAndThePanelSurvives`, `TestAttackDeclarationCarriesRageRow` and read/selector tests rather than duplicate their harness under the provisional plan name. Assert payment, persisted first-turn progress/origin, four distinct rows and stable selectors. Native no-bus/no-inner-type boundaries stay green.
- [x] Preserve existing clock-boundary and condition-removal fixtures; run each module's full race/vet/lint gates, commit/push separately.
- [x] Preserve `contributions.Source.SourceID` in SDK `EffectRow.SourceID`; `TestEffectProjectionRetainsSourceEntityWithoutChangingRowIdentity` catches the missing value without repurposing opaque row identity.
- [x] Operator merged additive proto `EffectRow.source_id = 9` (#422); verified publication v0.1.259 at generated 7dcb1b2 and adopted its Go/TS SDKs. API mapping and client target-overlay/held-row/DOM preservation tests failed before copying, then passed. This corrects the initial wire assumption, not the approved provenance requirement.

**API files:** `go.mod/go.sum`, `internal/handlers/dnd5e/v1alpha1/character/converters_test.go`, and existing engine/integration fixtures that use Raging; add a focused integration test beside the existing harness. `converters.go` is read-only unless opaque pass-through/metadata adaptation actually requires a change.

- [x] `TestConvertCharacterDataToProto_WithComposedRaging` (API #1193): canonical raw JSON retained, ref/name/description supplied by registry; no derived numerical rule added to API. Unknown/bad condition behavior remains fail-closed.
- [ ] Pin actual pushed session/root/resolution graph; verify fresh character creation, native activation, read and reload through existing API paths. No new RPC or permission classification is needed absent an actual wire change.
- [x] Run API `make test` and `make ci-check`; publish #1193 as a draft; published source-ID SDK adoption is complete, while the native walk/review and toolkit release pins remain pending. `ComposedRagingAcceptanceSuite/TestNativeActivationFirstBoundaryAndInformationReload` proves real handler/Redis activation, payment, first-boundary persistence and information reload; it does not replace normal-character creation/browser proof.

### Task 6 / S12 — reader and isolated native walk

**Web files:** `src/types/conditionData.ts`, its tests, `src/components/features/ConditionBadge.tsx`, and a component test alongside it. Other condition readers remain intact; no new rendering system.

- [x] `isRagingData` identifies the canonical ref/current shape, not removed damage_bonus/level fields. New tests accept current shape without those fields and refuse missing/invalid runtime data.
- [x] Badge tests use current data to display server description and runtime turn progress without a fabricated +N bonus or rules calculated from bindings; action-dependent values remain typed server answers. Preserve other condition-type tests.
- [x] RED/GREEN focused Vitest; complete `npm run ci-check` passed before draft web #1398. The initial command timed out and was rerun with a larger bound, not a reduced suite.
- [x] Adopt published source-ID SDK v0.1.259 from proto #422; verify the web lockfile and Go pin both resolve generated 7dcb1b2. No local generated binding substitute.
- [x] Build normal isolated `local/raging-composition` on :3307/:8307 with API 2bf39e23 and web 156a39bc; verify served heads, healthy API and matching local/container walk-site hash. Landing renders without browser errors and the real client lists the site. No operator stack/characters touched.
- [x] Operator early look received: “raging is acting like normal.” The one independent round is published, with no original code findings. Native QA then found #2312; its narrow fix requires focused closure, not another whole round. The overall QA/release task below remains incomplete. Try-it: create a fresh Barbarian → activate Rage → use a Strength melee action and inspect rows → end first turn → read/reload → sustain/expire → rest; observe correct budget, progress, domain-specific answers and removal. Item-specific provenance/storage remains proved by the separate real-chain example.
- [ ] After the operator's look/go, perform standing/relevant QA and one independent review round, publish dispositions, then obtain explicit merge authority. Re-pin providers to actual CI tags inside-out and repeat consumer gates; update implementation record and close only the delivered slices.

## Requirement coverage

| Requirement | Tasks | Proof |
|---|---|---|
| R1 actual class/item convergence | 2–5 | Shared factory source call sites plus native and item real-chain tests |
| R2 no legacy format/reset | 3,6 | Old/missing-state refusal; recreated current-format fixtures and new-character walk |
| R3 origin/bindings/runtime custody | 1–4 | Snapshot aliasing, strict round-trip, transfer/restore and separate-budget tests |
| R4 complete output validation and notifications | 1,3,5 | Missing-amount pre-attach refusal, Changed ordering and real dirty-save/reload |
| R5 one registry and strict restore | 3–5 | Registry validator/create/load integration, no parallel ref table, complete-state refusals |
| R6 honest information | 2,3,5,6 | Query-domain matrix, four unique rows, native declaration projection, no client math |
| R7 native reader/delivery boundary | 3–6 | Actual feature/class resources, API pass-through, badge tests and isolated walk |
| Observed release/adoption | 6 | Published review/head CI, actual tags and final pin receipts, not pseudo-version inference |

## Provider / consumer seam check

| Producer | Consumer | Exact consumed contract | Availability / proof |
|---|---|---|---|
| S9 mechanics/effects | D&D root and item attachment | Snapshot/CurrentOutputs, full Output Check, Changed on nonterminal state | Must push S9 first; no local replacements; compile and real dirty-state test |
| D&D kind table | ComposedCondition | Bind tables and Assess typed Effect from one rule | Same root module; query-domain and execution parity suites |
| Descriptor binding builder | CreateFromRef / LoadJSON / ValidateBindings | Same canonical RagingBindings/recipe; strict Origin and complete runtime | Same registry entry, no fake validator holder; negative input matrix |
| D&D root | Items / resolution / session | ConditionBehavior plus current ComposedData and plural information | Real pushed pin; remove concrete legacy fixtures, keep behavioral assertions |
| Resolution | Session | Dirty sheets, condition removal and information rows through existing ports | Real interaction/reload and no-inner-type/bus boundary suites |
| Session/root | API | Existing verb/data boundary, canonical opaque condition data | Existing handler/harness with normal exact module graph |
| API | Web | Existing condition ref/description/data plus information rows, with additive source_id | Proto #422 publication and exact generated SDK adoption before source-aware native wire/browser proof |

Checked translation: every approved requirement has an owner and concrete evidence; one action Frame is not falsely widened to all domains; definitions/bindings have a producer before consumers; old-data fallback is explicitly absent. No operator-owned design choice remains open. Any unanticipated ownership/wire/gameplay change stops only the affected task and is recorded on #2295, not hidden behind weaker acceptance.
