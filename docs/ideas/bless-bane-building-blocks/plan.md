# Bless/Bane building blocks implementation plan

> **For agentic workers:** use the executing-plans skill for direct, task-by-task execution. Implementation delegation is not authorized. One independent joined review follows the completed wave under the project review policy, not one campaign per task.

**Goal:** make typed effect composition and contribution selection useful tools, proved by native Bless/Bane and a short concentration-maintained raging-ring definition.

**Architecture:** the neutral effects module selects ordered applicable values without touching lifetimes. The rulebook owns typed definitions, mechanical meaning, composition compilation and named benefit bundles; its existing assemblies, application binder and roll consumer remain authoritative. Native Raging shares benefit declarations with the ring without changing its lifecycle.

**Tech stack:** Go 1.27, existing effects/machine/assembly/lifetime, testify suites, current rulebook keeper/bus/roll contracts.

**Spec:** [design](design.md), [walkthrough](README.md), [acceptance](acceptance.md). Authority: [Kirk's approval and ring success measure](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6865141821). R4's exact API spelling may evolve through a measured consumer; ownership/behavior changes return to the operator, not every local naming decision.

## Scope and finish line

- [S19 #955](https://github.com/KirkDiggler/rpg-project/issues/955): neutral provider, Task 1.
- [S20 #956](https://github.com/KirkDiggler/rpg-project/issues/956): root rulebook, Tasks 2–5, one runtime PR for that module.
- [S21 #957](https://github.com/KirkDiggler/rpg-project/issues/957): this wave's API adoption, Task 6.
- Design/receipts remain toolkit#2425, docs-only. Build the implementation, make the public Go ring example available for the operator's look, complete owning/consumer gates and the single joined review, and report exact ready heads. Merging/releasing waits for operator authority; it is not an implementation dependency.
- No new native inventory ring, studio feature, production promotion, operator-stack restart/reset, Raging lifecycle migration, unrelated condition conversion, general overrides engine or repo-wide panic sweep. The ring is an executable rulebook consumer, not proof of native item acquisition.

## Inspected baseline and working directories

Toolkit source `c55c9a060bfda6b64800bbc78f6346ae46dfcae9`; design first head `48c63097`. Fresh baseline tests passed in root effects/conditions/contributions/character/monster/monstertraits/rolls and both neutral effects packages.

API worktree `/home/kirk/game-dev/rpg-api/.worktrees/bless-bane-building-blocks`, branch `chore/bless-bane-building-blocks`, base `2fe647ea`: root v0.216.0, effects v0.6.0, encounter v0.207.1, resolution v0.75.1, session v0.194.0. API instructions and native acceptance helpers were inspected; full API baseline/gates belong to Task 6.

Web inspection tree `/home/kirk/game-dev/rpg-dnd5e-web/.worktrees/bless-bane-building-blocks`, detached `aa481c03`. `src/types/conditionData.ts` treats linked bindings/part bags as opaque and does not discriminate on roll_modifier. No web/proto change is expected; a measured contrary finding belongs at its owner.

Create separate toolkit release-unit trees from main for runtime: `.worktrees/contribution-selection` / `feat/contribution-selection`, and `.worktrees/bless-bane-building-blocks` / `feat/bless-bane-building-blocks`. Keep `.worktrees/effect-foundation-review` for design. Use absolute git paths and merge-on-top catch-up; preserve other work.

## Global constraints

1. One nearest-go.mod module per runtime PR. Publish meaningful checkpoints and open drafts on first working pushes.
2. T1 pushes source; root consumes a real Go-generated pseudo-version. T5 pushes root; API consumes its real pseudo-version. No local replace, go.work, copied toolkit source, manual tag, force-push or skipped hooks.
3. Release only after integration/review and operator go: effects merge → actual tag → root re-pin/verify/merge → actual root tag → API re-pin/verify/dev merge. Discover tags by merge SHA; do not predict versions.
4. Preserve source/payment/admission/generation, fixed intended recipients, independent pending finalization, authoritative-versus-observed facts and End-before-detach. Select contributors, not whole conditions.
5. No current-format fallback or automatic data reset. Explain changed declaration format and preserve old reachable refs while pinned consumers need them.
6. Test-selected RNG/concentration outcomes in module fixtures are not native browser observations. Do not manufacture live rolls or claim a native ring.
7. Exact-head remote CI and COMMENT verdicts precede merges. Existing unrelated gate failures must be measured and disclosed, never hidden by weakening tests.

## Review focus

- A later malformed candidate must not be hidden by an earlier winner → T1 whole-input refusal, T2 provider validation.
- Restored/subscribed order must not accidentally refresh age → T5 character and monster round-trip/handoff tests.
- One excluded output must not suspend other outputs or its owner's timers → T5 mixed-effect and loser-expiry tests.
- A nil/typed-nil definition or shared mutable amount/guard must not panic or alias another composition → T3/T4 constructor and ownership tests.
- Reusing Rage must not import native lifetime restrictions into the concentration ring or silently alter native scaling → T4 shared-output equivalence; T5 idle/failure and existing Raging regressions.

## Task 1 — Neutral contribution selection

**Delivers:** R1–R4; S19 #955. **Owner:** mechanics/effects. **Prerequisites:** clean isolated provider tree and owning baseline.

**Files:** create `mechanics/effects/selection.go`, `selection_test.go`; update `doc.go`, `README.md`; add a module-local run record.

**Interfaces:** `SelectionMode` with `AllApplicable = "all_applicable"`, `OldestApplicable = "oldest_applicable"`; `SelectionRule{Mode SelectionMode, Group string}`; `SelectionCandidate{ID string, Rule SelectionRule}`; `SelectionDecision{ID string, Selected bool, SelectedInstead string}`; `SelectContributions(*SelectContributionsInput{Candidates []SelectionCandidate}) (*SelectContributionsOutput{Decisions []SelectionDecision}, error)`. `ValidateSelectionRule(*ValidateSelectionRuleInput{Rule SelectionRule}) error` lets the root validate an unbound authored rule without fake candidate IDs; selection calls that same validator.

**Behavior:** input is already applicable and oldest-first. AllApplicable requires no group; OldestApplicable requires a nonblank group. Validate nil input, all modes/groups, nonblank IDs and duplicate IDs before returning decisions. SelectedInstead is empty for a winner and names the winning ID for an excluded candidate. Empty candidates return a valid empty output. Preserve input order, retain no state, and never sort or mutate inputs.

**Tests:** `TestContributionSelectionSuite`: `TestOldestPerGroup` inputs A-old/B-new in bless and C in bane → `[A:selected, B:excluded-by-A, C:selected]`; `TestAllApplicable` → both independent values selected without a group; `TestRemovalRecomputes` next input B/C → both selected; `TestWholeInputRefusal` malformed last entry after valid winner → nil output/error; `TestEmptyAndUnknownPolicy`; `TestUnboundRuleValidation` covers nil/malformed policies and both valid modes before candidate IDs exist; `TestInputAndOutputOwnership` mutation/repeated calls cannot share state.

- [ ] Add tests; run `go test -count=1 -run TestContributionSelectionSuite ./...` from mechanics/effects and observe the missing API failure.
- [ ] Implement the value-only selector and documented refusal contract.
- [ ] Run focused tests, then `go test -race -count=1 ./...`, `go vet ./...`, `golangci-lint run ./...` in that module.
- [ ] Update owning docs/run record; commit, push and open the provider draft linked to #955.
- [ ] Record the exact source SHA consumed by T2; do not merge to enable development.

**Completion evidence:** expected decisions and negative assertions, fresh owning gates, pushed provider SHA/PR. No D&D dependency or runtime state added.

## Task 2 — AddToRoll meaning and shared selection adapters

**Delivers:** R1–R4; first root checkpoint for S20 #956. **Owner:** rulebooks/dnd5e. **Prerequisites:** T1 pushed; root `go get .../mechanics/effects@<actual-T1-sha>` and tidy select a real pseudo-version.

**Files:** replace `effects/roll_modifier.go` with `effects/add_to_roll.go`; update `effects/kinds.go`, `effects/part_validation.go`, `effects/roll_modifier_test.go`, `effects/part_templates_test.go`; update `contributions/effect.go`, `events/roll_trace.go`, `conditions/roll_selection.go`, `conditions/linked_info.go`, `conditions/action_effects.go`, `conditions/named_linked.go`. Migrate current-format fixtures in `conditions/application_binding_test.go`, `application_test.go`, `linked_data_test.go`, `action_effects_test.go`, `baned_test.go`, `applied_twice_test.go`. Add `effects/add_to_roll_test.go`, `conditions/contribution_selection_test.go`. Only the root module's go.mod/go.sum change.

**Interfaces:** `KindAddToRoll = "add_to_roll"`; typed `Polarity` constants `Positive`, `Negative`. Canonical amount is `PartDeclaration.Amount.Dice`; params have `polarity`, `rolls`, `selection` (T1 SelectionRule), no second dice/group/subtract encoding. `DescribeRollOutput` returns Selection and unresolved DiceContribution values. `RollContributionMetadataOutput` carries `Applicable`, stable output `ID`, and `Selection`. Replace internal info-row RollGroup metadata with an optional owned SelectionRule. Keep the existing outward contribution values and roll-component/proto contracts.

**Behavior:** unsigned homogeneous dice only; polarity/domain/policy required and valid. Use T1 ValidateSelectionRule for unbound policy validation rather than duplicate the neutral rules or invent a candidate ID. Current attack/save domains only; unsupported guard, scalar amount, extra fields or terminal scope refuse. Negative maps to the existing subtractive component operator without negative faces. Both actual-roll and information adapters gather ordered applicable output IDs, call T1, and use its decisions; excluded rows name the winning source and retain no applied benefit. Preserve kind applicability separately before projection. Do not infer a winner from missing roll facts. Remove the old private claimant, not leave another policy loop. During this checkpoint the named helper may construct the new canonical data; T3 removes that helper and its panic.

**Tests:** `TestAddToRollSuite/TestPositiveNegativeDomains` (+1d4 attack/save, negative sign, known nonmatching request); `TestCanonicalAmountOnly` (reject params.dice, duplicate amount strategies and legacy kind); `TestRejectsUnsupportedOrMalformedDefinition`; `TestSelectionDoesNotRoll`. `TestContributionSelectionIntegrationSuite/TestBlessBaneAndAllApplicable` asserts oldest-per-group plus all-selected independent gifts; `TestInformationMatchesExecution` asserts identical selected output/source IDs and explicit exclusion reason; `TestOnlySelectedDiceReachResolver` asserts no resolver request for the losing die. Existing save-versus-outgoing-caster distinction tests remain.

- [ ] Add focused contract tests; observe failure on old kind/metadata/selection behavior.
- [ ] Implement the new meaning, migrate same-format fixtures and both adapters atomically; remove the old claimant and kind rather than add a dual reader.
- [ ] Run `go test -count=1 ./effects ./conditions ./contributions ./character ./monster ./monstertraits ./rolls` and then root `go test ./...`.
- [ ] Inspect the selected module graph and changed-file module ownership; update the run record.
- [ ] Commit/push the first working root checkpoint and open its draft for #956 immediately.

**Completion evidence:** source graph on a real provider pin, expected new-kind/refusal/selection assertions and green root baseline. This is not yet the typed authoring or ring proof.

## Task 3 — Typed construction and native Bless/Bane definitions

**Delivers:** R4 and the author-facing surface. **Owner:** root effects/conditions. **Prerequisites:** T2.

**Files:** create `effects/definitions.go`, `effects/compile_parts.go`, `effects/compile_outputs.go`, `effects/typed_definitions_test.go`; modify `conditions/blessed.go`, `baned.go`, `named_linked.go`, `descriptor.go`, `application.go`, `target_held.go`; extend `conditions/named_linked_test.go`, `blessed_test.go`, `baned_test.go`. Update effects README.

**Interfaces:** closed value-oriented `effects.Definition`; `SignedDice{Notation string, Polarity Polarity}`; `AddToRoll{Amount SignedDice, Rolls []contributions.RollKind, Selection generic.SelectionRule}`; `EndOn{Beat machine.Beat, Reason string}`. For the concrete ring consumer also provide typed `DamageBonus{Amount generic.Amount, When *machine.Guard}`, `Resistance{Types []damage.Type}`, `Advantage{On AdvantageDomain, Side AttackSide, When *machine.Guard}`, and unparameterized `Concentration`, `UntilEmpty`. Advantage domain/side constants map to the existing kind meanings; invalid side/domain pairs refuse.

`PartSpec{Key string, Effect Definition, OnTerminal generic.EndScope}`; `CompileParts(*CompilePartsInput{Parts []PartSpec}) (*CompilePartsOutput{Parts []PartDeclaration}, error)` lowers to the existing canonical templates and calls their validators. `CompileOutputs(*CompileOutputsInput{Parts []PartDeclaration}) (*CompileOutputsOutput{Outputs []generic.Output}, error)` projects validated payload declarations through the kind table's existing payload builders without binding fake actors; it rejects a maintenance part instead of silently skipping it. This lets native Raging reuse benefit declarations in its current recipe in T4.

**Behavior:** no compile/apply/pay/roll in literal value construction; no custom runtime callbacks. Nil/typed-nil definitions, duplicate or blank keys, unsupported terminal scope and malformed params return errors. Preserve order and clone nested amount/guard/slice values. Typed/native and JSON inputs share kind validation. Named descriptor parts become an error-returning factory, evaluated in existing error-returning prepare/create/load paths. Remove namedRollParts; Blessed/Baned explicitly declare AddToRoll plus recipient EndOn(rest), preserving group/sign and EndAttachment versus EndApplication distinction. Do not turn constructor failure into absent content or a new panic helper.

**Tests:** `TestTypedDefinitionsSuite/TestGoAndJSONCanonicalEquality`; `TestNilAndTypedNilNeverPanic`; `TestMalformedAndDuplicatePartsRefuse`; `TestReturnedValuesAreOwned`; `TestCompileOutputsRejectsMaintenance`; `TestRageLeafOutputsUseExistingMeanings`. `TestNamedLinkedConstructionSuite/TestNamedFactoryErrorPropagates` injects a failing named factory in package tests and observes preparation/create/load errors; `TestBlessBaneHaveExplicitRest` checks independent declaration and unchanged ordinary rest behavior. A bare AddToRoll without EndOn survives rest.

- [ ] Add tests and observe missing typed API/factory failures.
- [ ] Implement adapters and pure compilation, then native definitions; no second evaluator or schema.
- [ ] Run `go test -count=1 ./effects ./conditions ./character ./monster ./monstertraits ./spells` plus full root tests.
- [ ] Review the native definition call sites: no handwritten JSON or construction panic remains there; unrelated registry/trait panics are not claimed removed.
- [ ] Commit/push the root checkpoint and update its visible acceptance progress.

**Completion evidence:** compiled Go consumer code, canonical equality and error/ownership proofs, native definitions on the real typed path.

## Task 4 — Reusable Rage benefits and code-authored compositions

**Delivers:** R5; the producer seam the ring consumes. **Owner:** root conditions/effects. **Prerequisites:** T3.

**Files:** create `conditions/raging_benefits.go`, `raging_benefits_test.go`, `composition.go`, `composition_test.go`; modify `conditions/raging_definition.go`, `definition_compile.go`, `definition_test.go`. Preserve the native Raging codec and lifecycle body. Link examples from effects README.

**Interfaces:** `RagingBenefits(*RagingBenefitsInput{DamageBonus RagingDamageBonus}) (*RagingBenefitsOutput{Parts []effects.PartSpec}, error)`. It supplies four fresh EndPart values in stable damage/resistance/save/check order; damage accepts exactly the existing positive fixed or live-Barbarian-level strategy. A shared benefit-declaration helper feeds native ragingRecipe through CompileParts/CompileOutputs and this public bundle; no second handwritten four-output definition remains.

`CompositionSpec{Name string, Coordinator DefinitionRole, Benefits []effects.PartSpec, Maintenance []effects.PartSpec}`; `CompileComposition(*CompileCompositionInput{ID string, Spec CompositionSpec}) (*CompileDefinitionsOutput, error)` returns the existing immutable catalogue containing that one ID. Existing BindDefinition remains the only role-binding path. Factor expanded-definition validation so both this adapter and CompileDefinitions use it; retain JSON-library key/set checks before their expansion. Both paths require nonempty benefits/maintenance, valid coordinator/class/scope, unique keys and current capability constraints. No world lookup or fake participant IDs.

**Behavior:** native Raging retains its same fixed/live scaling, output order, activation grace/activity/duration/endings and saved bindings. The ring's bundle contains only benefits—no opaque Raging identity, cast restrictions or implicit native maintenance. CompileComposition may choose concentration without changing the benefit input. No overrides interpreter is needed; an optional convenience must earn a separate measured reason, not become a prerequisite.

**Tests:** `TestRagingBenefitsSuite/TestSharedNativeOutputs` checks equivalent mechanical kind/guard/amount/params and stable order; `TestFixedAndLiveScaling` asserts fixed2 and levels1/9/16 → 2/3/4, absent required live level remains unknown/refused; `TestNoMaintenanceInBenefits`; `TestInputMutationCannotChangeAnotherBundle`. `TestCompositionCompilerSuite/TestCodeAndLibraryBindEquivalentSpec`; `TestRejectsMisplacedOrDuplicateParts`; `TestCompilationIsPureAndOwned`; `TestMaintenanceChoiceDoesNotMutateBenefits`. Run existing Raging rule/runtime suites unchanged in behavioral assertions.

- [ ] Add producer and compiler tests; observe failures on missing public APIs.
- [ ] Extract the shared benefit source without moving native Raging lifecycle policy.
- [ ] Add the code-authoring adapter and common expanded validator; keep one binder/runtime.
- [ ] Run `go test -count=1 ./conditions ./effects ./character ./spells` and full root tests.
- [ ] Commit/push; update the public example to use the actual final spellings if the consumer improved R4.

**Completion evidence:** shared benefit source, existing native Raging behavior green, code/data composition equivalence and ownership tests.

## Task 5 — Prove usability, coexistence and durable selection

**Delivers:** all root acceptance, including the ring measure. **Owner:** root test consumers/docs. **Prerequisites:** T1–T4.

**Files:** create `conditions/typed_composition_example_test.go`, `typed_composition_test.go`, `character/contribution_order_test.go`, `monstertraits/contribution_order_test.go`; extend `conditions/definition_world_test.go` only for source-neutral fixture operations, `conditions/contribution_selection_test.go`, `character/blessed_test.go`; update `effects/README.md` and the idea's implementation receipt. Reuse `definition_source_test.go` and existing authored lifecycle/cookbook tests; do not build a second test engine.

**Interfaces:** consume T3/T4 public APIs and existing source budget, BindDefinition, PrepareApplication, binding/delivery, keeper, chain and roll interfaces. No new production API is introduced by the fixture.

**Tests:** `Example_typedRagingRing` contains the short Go definition with shared benefits and concentration/empty/rest maintenance, no raw JSON or item-name handler. `TestTypedCompositionSuite/TestRingKeepsIdleRecipient` → ten recipient turn ends leave the ring grant active; damage10 becomes12 on qualifying Strength melee weapon damage, incoming physical9 becomes4, and the two Strength advantages exist. `TestConcentrationFailureEndsOnlyRing` uses the actual returned consequence with a clearly test-selected failure, removes only its application and preserves independent Raging. `TestSingleBenefitNeedsNoRageGrant` builds resistance alone. `TestDifferentMaintenanceCannotRewriteActiveGrant` verifies owned definitions and restored state.

`TestContributionOrderSuite` on characters and monster attachment tests: admit A/B/C in order (two bless group instances plus bane), serialize/load/attach, assert A and C sources selected, remove A and assert B/C without age or lifetime renewal. Add `TestNonselectedExpiryDoesNotRevive` and `TestMixedConditionLosesOnlyOneContribution`. `TestUnnamedAttackOnlyGift` uses +1d6, AllApplicable and an explicit different EndOn: no new registry entry/handler; nonmatching saves get no die. `TestSelectedDiceOnly` counts the resolver's requested dice and proves no RNG on observation or the excluded candidate.

- [ ] Write the new public-consumer and ordering tests; observe failures for the old authoring surface and any uncovered semantics.
- [ ] Implement only measured seam fixes in their existing owners; no test-only mechanical shortcuts.
- [ ] Run targeted suites/examples, then root `go test -race -count=1 ./...`, `go vet ./...`, `golangci-lint run ./...`, and repository `make pre-commit` from the runtime tree. Disclose any unchanged workspace gate failure separately from owning-module results.
- [ ] Show the operator the actual short Go ring definition and executable output before treating the ergonomics measure as acknowledged; fixture evidence is not native inventory/UI evidence.
- [ ] Push the root integration checkpoint for API development and the bounded joined review; keep actual dependency graph and complete named-test receipt.

**Completion evidence:** passing new scenarios, actual example source/output, operator look recorded honestly, root gates and pushed SHA. Earlier JSON cookbook pass alone does not satisfy R5.

## Task 6 — Adopt the graph in API and complete release readiness

**Delivers:** S21 #957 and integration boundary. **Owner:** rpg-api root module. **Prerequisites:** T5 pushed root with pushed T1 provider selected; API tree inspected above. Read nearest API test guidance before running its integration commands; only one Docker-backed runner at a time.

**Files:** `go.mod`, `go.sum`, `knowledge/runs/2026-11-13-selected-effects-adoption.md`. Existing proof lives in `internal/integration/session/linked_maintenance_acceptance_test.go`, `effects_release_adoption_test.go`, `composed_raging_acceptance_test.go` and Bane acceptance. Add or adjust a test only for an actual measured integration gap; do not add production rule code or rewrite expected gameplay to fit the new provider.

**Interfaces:** unchanged session/handler/proto calls; API selects the root/neutral provider graph through real pseudo-versions during development. Preserve encounter/session/SDK/content pins at the current dev merge-on-top level; no downgrade to the inspected baseline.

**Behavior/tests:** fresh public cleric creation → Bless cast/payment → host recreation → current private source/maintenance rows and exact cleanup; existing paused Bane/delivery and Raging acceptance; True Strike last-member consumption → members_empty on both live streams and cold GetStory. Inspect the selected module graph and opaque web reader; no new UI or native ring claim. These are existing named behavioral regressions, not a fabricated RED for an otherwise mechanical re-pin.

- [ ] Run the API baseline, capture go.mod/graph, then go get actual pushed provider SHAs and tidy.
- [ ] Run `go test -count=1 ./internal/integration/session -run 'TestLinkedMaintenanceAcceptanceSuite|TestEffectsReleaseAdoptionSuite|TestComposedRagingAcceptanceSuite'`, plus the owning Bane suites discovered in this checkout.
- [ ] Run full `make test` and `make ci-check`; retain cmd/server and castle-copy parity. Open the consumer draft on its first working push.
- [ ] Complete one joined independent review of neutral/root/consumer work, resolve findings in-thread, and publish exact-head verdicts after current CI. Focus any later re-pin verdict on the actual changed graph.
- [ ] After operator merge go, release inside-out on actual tags and fresh owning/consumer gates. Record actual source heads, merge SHAs and selected graph; archive and remove only clean owned unserved trees. Preserve operator stacks, data, journal and needed provider refs.

**Completion evidence:** API local/remote gates, private/live/replay regression receipt, graph preservation and review verdict. Release/merge evidence is conditional on explicit authority, never assumed from design approval.

## Requirement coverage check

| Requirement / scenario | Task | Concrete proof |
|---|---|---|
| Coexistence is not contribution | T1/T2/T5 | Three live instances, A/C selected, B retained and individually aging |
| Independent groups and additive meaning | T2/T5 | Bless positive plus Bane negative sourced components; AllApplicable gifts both included |
| Shared selection for info/execution | T1/T2 | Same selected output IDs; excluded row identifies winner |
| No selection-induced lifecycle/RNG | T1/T5 | Stateless repeated calls, loser expiry/mixed effect and requested-dice counts |
| Stable oldest through reload | T5 | Character and monster order/handoff assertions |
| Typed construction without JSON/panic at call sites | T3 | Value definitions, canonical equality, typed-nil/factory error tests |
| New combination not new handler | T3/T5 | Unnamed +1d6 attack-only gift with independent EndOn |
| Ring ease, reusable Rage benefits | T4/T5 | Shared native benefit source plus compiled public Go ring example |
| Ring concentration replaces native upkeep | T4/T5 | Idle recipient survives; real failed-maintenance consequence removes only ring |
| Ownership and frozen active definitions | T3/T4/T5 | Mutation isolation and reload tests; same binder/custody |
| Preserve native Rage/current application | T4/T5/T6 | Existing native suites and API full owning gates |
| Own consumer adoption, no unrelated takeover | T6 | Graph diff, cmd/server/parity, current source/live/replay tests |
| No unsupported quantities or migration claims | T2/T3/T6 | Malformed/unsupported refusals, one current schema, scope/receipt disclosures |

## Provider/consumer seam check

| Provider | Consumer | Produced vs consumed contract | Availability / dependency | Proof |
|---|---|---|---|---|
| T1 mechanics/effects | T2 root adapters | SelectionRule + ordered candidate/decision values, no game data | Pushed source then real pseudo-version; no merge prerequisite | Neutral tests and selected output/source agreement |
| T2 root kind | T3 Go compiler / existing JSON compiler | Canonical add_to_roll amount/params and shared validation | Same root module, ordered tasks | Go/JSON equality and rejection matrix |
| T3 typed Parts/Outputs | T4 native Rage and code composition | Owned PartDeclaration / generic.Output values, no fake runtime identity | Same root module | Native four-output equivalence, no maintenance leakage |
| T4 composition compiler | T5 existing binder/source/host | Existing CompileDefinitionsOutput and BindDefinition contracts | No new binder/runtime | Public Go ring through real resource/keeper/chain/custody |
| Holder save/load | T2 selector through T5 | Ordered current conditions and stable output IDs | Existing capability inspected in character and monster preparation | Before/after reload and removal ordering |
| Root T5 + neutral T1 | API T6, unchanged session/resolution source | Existing ABI/runtime behavior through actual selected module graph | Pushed source during development; actual tags only before merge | Full API gates, graph and native regression receipt |
| API opaque linked payload | Existing web dev reader | Current linked envelope, opaque part kind/params | Inspected aa481c03; no generated-binding change expected | Reader inspection plus no unsupported-field assumptions in native contract |

## Plan self-check and dispositions

- Every accepted scenario has an owner and named proof above. The ring is implemented in T4/T5, not deferred to examples/items or satisfied by rerunning old JSON examples.
- R4 spellings are concrete here so tasks can meet; a consumer-driven spelling adjustment updates both producer and consumer contracts, not their meaning. R5 is the required outcome.
- All root files have one writer and ordered tasks. Provider and API are separate module worktrees/PRs. The design branch owns no runtime edits.
- T1 implementation preflight found an omitted producer capability: root needs policy validation before a candidate ID exists. Added ValidateSelectionRule to T1/T2 and its named test, keeping validation with its neutral owner instead of introducing fake IDs or a second validator.
- Generic selection does not carry D&D or mutable state. AddToRoll does not own lifetime; native Raging does not surrender its lifecycle to the ring.
- CompileComposition returns the existing catalogue/binder contract; both code/data use expanded-definition validation. No second interpretation path is introduced.
- Fixed roll amounts, check offers, arbitrary override policies and native inventory are explicit exclusions, not unimplemented advertised capabilities. No required decision remains blocked by the exploratory override suggestion.
- API/proto/web changes are not assumed absent because tests are green elsewhere: the actual API graph is built/tested, the reader is inspected, and new measured integration findings return to their owners.
- Documentation, resource ownership, persistence, events, cleanup, pin adoption and operator look are accounted for. No independent implementer or per-task review fanout is authorized.
