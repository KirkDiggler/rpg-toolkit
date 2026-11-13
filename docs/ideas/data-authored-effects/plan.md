# Data-authored effect compositions — implementation plan

> **For implementers:** read the linked design and owning instructions, then use the executing-plans/build-slice skills task-by-task. Direct execution is the default; this plan does not authorize delegation. Keep the task checkboxes and exact-head evidence current.

**Goal:** establish reusable, data-authored effect composition in the rulebook and teach its public API through rulebook-local fixtures and executable examples.

**Execution state:** T1–T9 implementation, proof and authorized runtime release are complete. #2402 merged `a3afad9a` → mechanics/effects v0.6.0; #2401 merged `7c0d3457` → rulebooks/dnd5e v0.216.0 after a real-tag re-pin, fresh full gates/CI and focused head verdict. The operator's [merge-out go](https://github.com/KirkDiggler/rpg-project/issues/946#issuecomment-6840404069) accepts the reviewed fixture evidence, not a claimed manual walk. This PR carries the final documentation closeout; the owning issues carry its actual merge receipt. Minor #2404 and deferred #947 remain open. [Release evidence](implementation.md).

**Architecture:** a validated immutable library binds user/recipient roles into existing linked applications. The rulebook kind table owns meanings and projection; configured target facts are values in the action frame. Generic effects, source payment and existing delivery/lifetime machinery keep their current responsibilities; a small test source demonstrates payment without implementing an item engine.

**Tech stack:** Go 1.27 modules, existing JSON codecs and testify suites. T5 demonstrated a missing complete-output observation; root developed against the separate provider's pushed pseudo-version and now pins released mechanics/effects v0.6.0.

**Spec:** [design](design.md), [walkthrough](README.md), [exact example data/policies](examples.md), [brief](brief.md). R1–R6 accepted; [planning authority](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6833714502) and [rulebook-first correction](https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6835450799).

**Slice:** [Composable conditions S16 #946](https://github.com/KirkDiggler/rpg-project/issues/946) owns T1–T9: rulebook API, primary fixture library, executable examples and evidence. [#947](https://github.com/KirkDiggler/rpg-project/issues/947) is a separate deferred examples/items adoption check, not a prerequisite or deliverable here. Design/plan PR: toolkit #2399, not an implementation PR.

## Scope, finish line and authority

The finish line is reusable production rulebook APIs, a canonical rulebook-local fixture library, public-API examples with a small test source, named positive/refusal/lifecycle proofs, a deterministic executable cookbook, documentation and one source-verified root-module implementation PR with required independent review. No item-module dependency, production inventory engine, native browser acceptance or shipping is inferred.

The earlier plan made examples/items the primary cookbook. Kirk's correction—“I was thinking of getting the rulebook package in order”—moves the teaching and integration proof into the rulebook under R6; downstream item adoption is separate.

No second plan-approval gate is required. Execution follows the agreed design when implementation is the current task; architectural gaps return only their affected decision. Separate explicit authority is required to merge/release. The preceding wave's authority does not apply here.

Native studio, inventory deletion, target offers/range validation, API/protos, observation disclosure policy, native frame-producer adoption and the rest of #707 are excluded. The operator's live stack/run and journal #863 remain untouched.

## Global constraints

- One root-module implementation PR for production code and its external-package tests; one writer per isolated worktree. All runtime tasks are sequential in that worktree.
- No per-item runtime branch, callbacks in content, second kind table, new state interpreter or historical save reader.
- Payment remains source-owned. Pure preflight precedes a test-source resources.Resource.Use(1); world delivery follows payment. Do not copy items.Use or build an item/refund engine in the test harness.
- Preserve fixed intended recipients, independent finalization, exact generation/address checks, immutable origin, and End-before-detach observations.
- A frame holds observer-captured facts for information and authoritative facts for execution; no live-target lookup repairs missing information.
- Ring/ward/damage gift: user-owned concentration, no duration cap, no implicit target-unconsciousness ending. Scroll: recipient activity, activity-only first-end grace, ten recipient turn ends including that first end. Sausage: Strength melee weapon non-opportunity advantage and all attacks against the eater advantaged, ending at next received eater turn start/rest/zero HP.
- Fixture damage is fixed +2; test-source pools have maximum 1, price 1, no recovery. Exhaustion is not inventory deletion. Native Barbarian Raging and Reckless acquisition/activation remain unchanged.
- Native consumers do not acquire configured target-side support merely by a provider test passing; their frame producers must adopt the new contract before native use.
- Production and external-test consumers share the same root module; no re-pin separates those tests from the rulebook. The neutral observation prerequisite uses a real pushed provider pin. No examples/items dependency, replace/go.work or source copying. Separately authorized downstream adoption uses real published pins; do not predict release tags.

## Inspected baseline and dependency order

Planning branch `design/data-authored-effects`, documentation head `a01c1a6f7a37633651ec067ecd003aa14494b7bd`; relevant runtime baseline `d7069af9`. Comparison with fetched main `8a9bdec774a56138ed431afd2b7a86da13429f3e` finds no runtime changes in effects/conditions/contributions/items; only item knowledge records differ. Reconcile again at implementation start, preserving parallel work by merge-on-top.

Inspected: effects `parts.go`, `part_validation.go`, `kinds.go`, `query.go`, `attach.go`, `vocabulary.go`, maintenance/roll files; conditions `application*.go`, `linked_*.go`, `action_effects.go`, `target_held.go`; contributions `frame.go`; items `definition.go`, `item.go`, `use.go` and current Rage fixtures. Baseline root effects/conditions and item tests pass; these are not implementation evidence.

| Scope | Working branch/worktree at implementation | PR ownership |
|---|---|---|
| Neutral observations | `feat/authored-effect-observations`, `rpg-toolkit/.worktrees/authored-effect-observations` | `mechanics/effects` only, #950 / PR2402 |
| Root | `feat/authored-effect-rulebook`, `rpg-toolkit/.worktrees/authored-effect-rulebook` from current main | `rulebooks/dnd5e` only, #946 / PR2401 |
| Docs | existing `design/data-authored-effects`, `.worktrees/effect-building-blocks` | #2399 owns idea documents; completed package README handed to root #2401 |

Development is driven by the approved public-use examples. Start T7's canonical fixture data, then T1 → T2 → T3 → T4 → T5 → T6 → finish T7 → T8 → T9 in one root worktree. T4 supplies compiler/binder RED tests as those interfaces are introduced; do not add prematurely uncompilable cookbook tests that block unrelated package checks, temporary test-exclusion tags, or a fake provider. T7/T8 external-package tests exercise the same module's public API and feed required fixes directly back into it.

Authorized release order, **not authority granted here**: neutral observation provider #2402 merges → discover/verify its actual tag → re-pin root #2401 with go get/tidy and repeat full root gates/current-head CI → merge root including its fixture tests and verify its actual tag → documentation closeout. Deferred #947 may adopt that API separately; it is not a gate for #946.

The originally conditional generic-provider gap was demonstrated in T5: Bound.Params lacked authored When/Amount for exact execution checks. [S17/#950](https://github.com/KirkDiggler/rpg-project/issues/950) holds the bounded prerequisite contract. Output.Clone and Bound.Definition return detached authored data without state/permission changes; three named observation tests were RED→GREEN, full provider race/vet/lint pass, and root now delegates its clone helper. This is a separate release unit, not a widened item scope.

## Review focus

1. Mutable nested guards/amount maps escape through compiled libraries, bindings, frames or reload: T1/T2/T4/T6 mutation-isolation tests must pin this.
2. Self-use or adversarial invocation strings collide with a coordinator/another application's attachment: T4/T6 address/refusal tests must fail before payment.
3. Missing or stale target detail silently removes the sausage drawback or queries live state: T5 unknown/contradictory/generation tests must distinguish information from execution.
4. Attack support accidentally changes save-shaped outgoing damage, save/check advantage or actor identity: T2/T5 domain regression tests must retain existing meanings.
5. Reload/replacement/delivery failure duplicates payment, refunds a world mutation, resurrects an ended effect or hides an orphan: T6/T7/T8 exact-boundary tests must preserve custody and spent availability.

## Interface ledger

These are implementation contracts, not claims that the names already exist. Existing public APIs not listed as changed remain intact. All new package functions use Input/Output structs; error-only validation follows the existing ValidatePart convention.

| Produced by | Contract consumed later |
|---|---|
| T1 | `contributions.ConfiguredOutput{ID string, Definition generic.Output}`; `HeldEffectDetail{MemberID string, Held HeldCondition, ApplicationID string, DisplayName string, OriginRef core.Ref, OriginID string, Outputs []ConfiguredOutput}`; `Frame.HeldEffects []HeldEffectDetail`; `ConfiguredOutput.Clone() ConfiguredOutput` |
| T2 | `effects.PartTemplate{Kind generic.KindName, Params json.RawMessage, When *machine.Guard, Amount *generic.Amount}`; `type PartClass string` with `PartClassBenefit = "benefit"` and `PartClassMaintenance = "maintenance"`; `ValidatePartTemplate(*ValidatePartTemplateInput{Template PartTemplate}) (*ValidatePartTemplateOutput{Class PartClass, Capabilities PartCapabilities}, error)`; PartTemplate/PartDeclaration `Clone` value methods; PartDeclaration adds optional When/Amount |
| T2 | Kind entry supplies one payload-output builder reused by part binding, validation and projection; no lookup back through kindTable from its own initializer callbacks |
| T3 | `KindCombatActivity = "combat_activity"`, params `{skip_first: bool}`; `BeatTurnStart = "turn_start"`; end_on accepts rest/combat_end/dropped_to_zero/turn_start; actual last-member-loss reason becomes `members_empty` |
| T4 | `conditions.DefinitionLibrary{Parts map[string]effects.PartTemplate, BenefitSets map[string][]string, Definitions map[string]EffectDefinition}`; `PartSelection{Key, Part, Set string}`; `DefinitionRole` values user/recipient; `EffectDefinition{Name string, Coordinator DefinitionRole, Maintenance, Benefits []PartSelection}` |
| T4 | `CompileDefinitions(*CompileDefinitionsInput{Data json.RawMessage}) (*CompileDefinitionsOutput{Definitions *CompiledDefinitions}, error)`; `ValidateDefinitionReference(*ValidateDefinitionReferenceInput{Definitions *CompiledDefinitions, DefinitionID string}) error` for source configuration without fake participant IDs |
| T4 | `BindDefinition(*BindDefinitionInput{Definitions *CompiledDefinitions; DefinitionID, UserID, RecipientID, ApplicationID string; Origin dndevents.ConditionOrigin}) (*BindDefinitionOutput{CoordinatorID string, Spec ApplicationSpec}, error)`; compiled internals unexported, no mutable catalogue accessor |
| T5 | `effects.AttachmentIdentity{Address dndevents.ConditionAddress, ApplicationID string}`; optional `Attachment *AttachmentIdentity` on BindInput/AssessInput; `BindPartInput.Address` is the **local** address, distinct from existing Owner |
| T5 | `effects.ProjectPartOutput(*ProjectPartOutputInput{Declaration PartDeclaration, Output *generic.Bound}) (*ProjectPartOutputOutput{Definition generic.Output, AgainstHolder bool}, error)`; `AssessConfiguredOutput(*AssessConfiguredOutputInput{Detail contributions.HeldEffectDetail, OutputID string, Frame contributions.Frame}) (*AssessOutput, error)` |
| T5 | `conditions.ProjectHeldEffects(*ProjectHeldEffectsInput{HolderID string, Conditions []dndevents.ConditionBehavior}) (*ProjectHeldEffectsOutput{Details []contributions.HeldEffectDetail}, error)` |
| T6 | Existing PrepareApplication/ResumeApplication/BindApplications/FinalizeRecipient/CheckComplete remain the lifecycle seam; strengthen preflight collision checks and deep-copy extended declarations |
| T7 | External `conditions_test` helpers only: `fixtureSource{ID, UserID, DefinitionID string; Ref core.Ref; Available bool; Budget *resources.Resource}` and `definitionUseInput{Source *fixtureSource; RecipientID, ApplicationID string; Definitions *conditions.CompiledDefinitions; Participants []conditions.ParticipantConditions}` → `useDefinition(ctx, definitionUseInput) (*preparedDefinitionGrant, error)`; result carries Prepared, copied Spec, Origin and Remaining. This is source/world scaffolding, not a production item API. |

Template validation returns the **existing** concentration capability as well as class so compilation can reject competing concentration declarations without branching on a kind name. This fills the agreed whole-library validation contract; it is not a second registry.

## Task contracts

Paths below are repository-relative. Commands run from the indicated owning module in its implementation worktree, never from the repository root as though it had a go.mod. The new suites are named explicitly; a missing suite is not a passing verification.

### T1 — Immutable configured-effect facts

**Delivers:** C1/R4, the inert frame contract consumed by target-side effects.
**Owner:** root module, contributions; no rule registry or event imports.
**Prerequisites:** existing Frame/HeldCondition/Clone and generic.Output.
**Files:** create `rulebooks/dnd5e/contributions/configured_effects.go`, `configured_effects_test.go`; modify `contributions/frame.go`, `frame_test.go`, `doc.go` under the same module.
**Interfaces:** T1 ledger. HeldCondition stays comparable; details are a separate collection. Clone deep-copies Params, guard trees/values and amount pointers/class-step maps.
**Behavior:** absent detail is unknown; present empty Outputs is known-none; present nil Outputs is malformed. Detail needs a valid matching member/held identity, generation, display/origin and unique output IDs. Frame.Validate checks structural data only; it cannot import the effects kind table. Semantic kind/guard validation belongs to T5. A malformed unused detail still fails validation.
**Tests:** `TestConfiguredEffectFactsSuite/TestUnknownVersusKnownEmpty`; `TestRejectsMissingHoldingDuplicateDetailAndOutputIDs`; `TestCloneOwnsNestedOutputData` (mutate cloned params, guard child and class-step map; original unchanged); `TestLegacyHeldIdentityRemainsComparable`; retain TestFrameSuite/TestHeldSuite.
- [x] Add the named suite and frame regression assertions.
- [x] Run focused tests; record missing-field/clone-isolation RED.
- [x] Implement value validation and copying without importing events/effects rulebook packages.
- [x] Run focused tests and existing frame tests.
- [x] Record evidence and commit this bounded root change.
**Verification:** root cwd: `go test -count=1 ./contributions -run 'Test(ConfiguredEffectFactsSuite|FrameSuite|HeldSuite)$'`.
**Completion evidence:** all named tests execute/pass; no changes to named condition meanings or native frame producers.

### T2 — Validated templates and reusable payload parts

**Delivers:** C2/R4, independent damage and save/check advantage alongside existing resistance.
**Owner:** root effects plus condition declaration-copy boundaries.
**Prerequisites:** T1 copied generic output values.
**Files:** create `rulebooks/dnd5e/effects/part_templates.go`, `payload_parts.go`, `part_templates_test.go`, `payload_parts_test.go`; modify `effects/{parts,part_validation,kinds,resistance_part,roll_modifier,doc}.go`; update the validation hooks in concentration/duration/end parts to reject unsupported extended fields; modify `conditions/application.go` (`cloneDeclarations`) and `conditions/linked_data.go` (`cloneLinkedBindings`).
**Interfaces:** T2 ledger. Template/PartDeclaration clones use the T1 value-copy capability. Assign kind-owned class/capabilities in the existing kind table. Payload hooks call direct builders/checkers rather than recursively redispatching through the table initializer.
**Behavior:** damage_bonus supports the existing fixed/class-scaled scalar meanings and frame-fact When; save/check advantage supports ability guards. Resistance retains types-only part configuration. Other kinds reject unsupported When/Amount rather than ignoring them; roll_modifier retains Params.dice as its sole amount source. Authoring guards allow Fact/All/Any/Not, not bags/flags/rounds/class-level guard nodes. Validate literal types and canonical ability values; amount-by-class remains available independently of guard forms. Maintenance has no benefit-set class.
**Tests:** `TestPartTemplateSuite/TestClassifiesThroughKindEntry`; `TestRejectsUnknownFieldsKindsFactsAndStateGuards`; `TestRejectsUnsupportedAmountOrWhen`; `TestPayloadPartsSuite/TestFixedDamageAndClassScalingUseExistingMeaning` (+2 on Strength melee weapon damage, no bonus for Dex/ranged/nonweapon; class thresholds remain 1→2,9→3,16→4); `TestStrengthSaveAndCheckParts`; `TestResistanceUsesOneExistingFold` (9 physical→4, two resistance sources still 4); `TestDeclarationClonesOwnGuardsAndAmounts` across both condition clone paths. Existing save-shaped damage and roll-modifier tests remain green.
- [x] Add template/part and copy-isolation tests.
- [x] Run focused tests; confirm missing part support/ignored-field or clone RED.
- [x] Add kind-owned pure validation/building and copy support.
- [x] Run new suites plus existing vocabulary, parts and roll suites.
- [x] Update effects godoc; record evidence and commit.
**Verification:** root cwd: `go test -count=1 ./effects ./conditions -run 'Test(PartTemplateSuite|PayloadPartsSuite|RulebookEffectVocabulary|PartsSuite|ApplicationBindingSuite)$'`; then `go test -count=1 ./effects` to include existing roll/maintenance tests.
**Completion evidence:** a part composes each existing payload meaning without a named Rage constructor or new generic-module API.

### T3 — Selectable activity and ending policies

**Delivers:** C3/R2/R5, scroll upkeep independent of benefits and concentration.
**Owner:** root effects; current reason vocabulary in conditions.
**Prerequisites:** T2 kind/template classification; existing typed beats and duration part.
**Files:** create `rulebooks/dnd5e/effects/activity_part.go`, `activity_part_test.go`; modify `effects/{kinds,vocabulary,end_parts,end_parts_test,parts_validation_test,doc}.go`, `conditions/end_reasons.go`, `conditions/concentrating_test.go`.
**Interfaces:** T3 ledger; register activity and the additional end-on beats in the same vocabulary. InspectPart/DescribeMaintenance must accept the new maintenance without exposing bag interpretation to callers.
**Behavior:** holder attack attempts, including misses, or positive DamageTaken mark activity; each checked holder end clears both flags. Explicit skip_first bypasses only that first activity check. Duration continues to count independently. Zero/nonholder/legacy instruction damage cannot sustain it. end_on fires once for the selected holder/beat. Use `members_empty` for actual last-member loss; preserve the existing exported Go symbol `conditions.ConcentrationEndedSpellEnded` with its single new value so separately pinned callers still compile when adopting the provider—no old wire value/reader or second runtime. Do not rename unrelated caster-down vocabulary.
**Tests:** `TestCombatActivitySuite/TestFirstEndGraceDoesNotSkipDuration` (combined ten-end fixture: first end leaves nine, activity check skipped); `TestMissAndPositiveAppliedDamageSustain`; `TestZeroForeignAndLegacyDamageDoNotSustain`; `TestFlagsClearAndSurviveReload`; `TestActivityDoesNotRequestConcentrationOrApplyDamage`; `TestEndOnHolderTurnStartAndZeroHP`; existing `TestPartsSuite/TestUntilEmptyRequiresActualLastMemberLoss`; assert reason members_empty once (reuse the existing proof rather than a duplicate wrapper). Retain native Rage/Reckless tests without rewriting their expected gameplay.
- [x] Add activity/end-policy tests and update only the intentional last-member reason expectations.
- [x] Run focused tests; observe missing activity/beat support RED.
- [x] Implement independent recipes using the existing machine, not a monolithic Rage clone.
- [x] Run focused suites and root condition regressions.
- [x] Update owning docs; record reason-contract change and commit.
**Verification:** root cwd: `go test -count=1 ./effects ./conditions -run 'Test(CombatActivitySuite|PartsSuite|ApplicationBindingSuite|RecklessAttackSuite)$'`; `go test -count=1 ./effects ./conditions`.
**Completion evidence:** ring maintenance can omit activity; scroll activity is a separately testable part. Nested resolution/encounter consumers are inspected, not edited in this PR; their adoption remains separate.

### T4 — Compile the library and bind participant roles

**Delivers:** C4/R1/R4, one pure authoring path into existing ApplicationSpec.
**Owner:** root conditions.
**Prerequisites:** T2/T3. Attack templates become accepted when T5 provides their meaning; tests here use available damage/resistance/save-check templates, not a fake attack provider.
**Files:** create `rulebooks/dnd5e/conditions/definition.go`, `definition_compile.go`, `definition_bind.go`, `definition_test.go` (the compiler/binding suite shares one test file).
**Interfaces:** T4 ledger, including identity-free ValidateDefinitionReference for item configuration. JSON names match examples.md exactly: parts/benefit_sets/definitions, key/part/set, name/coordinator/maintenance/benefits. CompileDefinitions owns decoded data and all maps/slices; no mutable catalogue map is returned.
**Behavior:** validate every entry, even unused; reject duplicate/unknown JSON members, missing names/roles/lists, unknown refs, nested sets, lifecycle leaves in benefits, benefits in maintenance, multiple concentration claims and repeated expanded leaves/keys. IDs/keys are nonblank and exclude `/`; set expansion is selection-key/part-ID and preserves order. Binding assigns EndApplication to coordinator maintenance and EndPart to recipient benefits, uses the existing linked-effects ref and LifetimeShared, and keeps item origin separate. Derive recipient AddressSourceID deterministically from the whole invocation (for example a tagged JSON tuple), never from origin; it must differ from the owner address for self-use. All returned values are owned copies. Bind validates source/ref/IDs but does not attach, inspect world state or spend.
**Tests:** `TestDefinitionCompilerSuite/TestExpandsOneSetIntoRingAndScroll`; `TestStrictWholeLibraryValidation` (all refusal cases above, including bad unused entry); `TestUnparameterizedMaintenanceCannotClaimAnAmount`; `TestValidateReferenceNeedsNoFakeActors`; `TestBindingRolesAndSelfUse` (A→B, A→A, user-owned versus recipient-owned coordinator); `TestBindingDoesNotSpendAttachOrAlias`; `TestReturnedSpecMutationCannotChangeLibrary`. Errors name the offending definition/part and field rather than only saying invalid.
- [x] Add minimal authored JSON and named compile/binding tests.
- [x] Run them; confirm missing compiler/binder RED.
- [x] Implement the pure interfaces using T2 validation and existing application types.
- [x] Run compiler/binding tests and existing linked-data tests.
- [x] Document interfaces; record evidence and commit.
**Verification:** root cwd: `go test -count=1 ./conditions -run 'Test(DefinitionCompilerSuite|LinkedDataSuite|ApplicationBindingSuite)$'`.
**Completion evidence:** several definitions share one set, but binding copies the actual selected configuration; invalid content is rejected without runtime construction/payment.

### T5 — Configured attack bearings and honest target information

**Delivers:** C5/R4, the sausage's two sides through the same rule meanings.
**Owner:** root effects/conditions, consuming T1 frame values.
**Prerequisites:** T1/T2; T4 for end-to-end configured definition tests. T3 supplies sausage end policies.
**Files:** create `rulebooks/dnd5e/effects/attack_advantage.go`, `attack_advantage_test.go`, `output_projection.go`; create `conditions/held_effect_details.go`, `held_effect_details_test.go`; modify `effects/{types,parts,kinds,query,attach,vocabulary}.go`, `conditions/{linked_condition,target_held,action_effects}.go`, `effects/parts_test.go` helper inputs and relevant tests. Do not edit resolution/frame.go or native observers.
**Interfaces:** T5 ledger. Add `FactOpportunity = "action.opportunity"` reading existing Frame.Action.Opportunity. Pass local Address plus ApplicationID from buildLinked into BindPart/Bind/Assess; Owner stays the coordinator. An against-holder part refuses missing/wrong attachment identity before attachment. Cover both BindPart and the complete output Check hook of a direct Compose/Bind caller so this cannot be deferred to a paid Apply failure. Other existing vocabulary callers do not acquire a fabricated address requirement.
**Behavior:** advantage params `on:attack` require by_holder/against_holder; save/check params reject side. By-holder matches actor; against-holder matches target without swapping frame actors/abilities. Do not globally reclassify Query.Action: it currently serves outgoing damage, including save-shaped damage. Route attack assessment through the kind's dedicated shared evaluator; both public assessment and execution must use it (executeAssessment currently calls assessMeaning directly).

Projection uses Assembly.CurrentOutputs plus the same validated payload builder, not terminal Snapshot state, private bags or copied effect logic. Emit a known-empty detail for a configured linked attachment with no target-bearing outputs. Frame validation remains structural; effects validates semantic kinds/guards. AssessTargetHeldEffects handles plural configured rows with stable target-prefixed output IDs and keeps fixed named-condition paths. Missing detail produces one truthful Depends row; empty detail produces no configured target contribution. Execution requires exact member/ref/source/application/configuration agreement with the live applying output and fails on absent or contradictory authoritative detail for the applying output. Use semantic/canonical data equality, not raw JSON whitespace or map iteration order. Retained observed values never follow a live mutation.
**Tests:** `TestAttackAdvantageSuite/TestByHolderAndAgainstHolderDoNotSwapActors`; `TestMissingSideAndUnsupportedSaveSideRefused` (attack side absent/empty/null; save/check side present even when empty/null); `TestOpportunityAndAbilityFacts`; `TestAttackSupportPreservesSaveShapedDamage`; `TestConfiguredHeldEffectsSuite/TestObservedUnknownEmptyAndKnown`; `TestSnapshotDoesNotReadLiveTarget`; `TestLiveGenerationOrConfigurationMismatchRefusesExecution`; `TestCanonicalRoundTripIsNotAForeignConfiguration`; `TestEndReportRetainsOutputsThenDropsThem`; `TestSelfAttackNamesTwoIndependentSources`; `TestFixedNamedConditionsKeepTheirExistingPath`. Agree on both info rows and actual AttackChain advantage-source lists, not just strings.
- [x] Add the named actor/target, unknown and round-trip assertions.
- [x] Run focused tests; confirm unsupported attack meaning/projection RED.
- [x] Implement one shared evaluator, provider projection and configured target-row routing.
- [x] Run new suites plus existing action/target/vocabulary/clone regressions.
- [x] Update doc.go contracts and record the deferred native-producer adoption boundary; commit.
**Verification:** root cwd: `go test -count=1 ./effects ./conditions ./contributions -run 'Test(AttackAdvantageSuite|ConfiguredHeldEffectsSuite|DefinitionCompilerSuite|RulebookEffectVocabulary|ConfiguredEffectFactsSuite)$'`; then `go test -count=1 ./effects ./conditions ./contributions`.
**Completion evidence:** authoritative sausage attacks and copied observer information agree through actual chain subscriptions; no callback in Frame and no item-name dispatch.

### T6 — Admission, frozen definitions and exact application custody

**Delivers:** C6/R4, the new declarations safely use the existing protocol.
**Owner:** root conditions/character fixtures.
**Prerequisites:** T4/T5 and all selected parts from T2/T3.
**Files:** modify `rulebooks/dnd5e/conditions/application.go` (including ResumeApplication), `linked_data.go` only where needed; create `conditions/application_admission.go`, `authored_application_test.go`; introduce the shared `definition_world_test.go` host here for custody proof; extend `application_test.go`, `application_pending_test.go`, `application_ownership_test.go`, `linked_data_test.go`. Keep the actual delivery/finalization implementation unless a named test demonstrates required wiring.
**Interfaces:** existing PrepareApplication/ResumeApplication, BindApplications, CreateApplication/CreateRecipient, FinalizeRecipient and CheckComplete. Complete participant data is supplied by the host. Binding-produced local address is never the coordinator address.
**Behavior:** before payment, reject candidate/owner address collisions with existing live attachments or another live owner's fixed pending bindings, except exact addresses owned by the explicitly described concentration replacement; an unrelated same-origin effect is not replacement authority. Copy When/Amount through preparation, serialization and resume. Restore uses saved declarations and part states, not CompileDefinitions or latest catalogue. Retain per-entry pending/live state and terminal refusal; no state repair, default refill or second membership list.
**Tests:** `TestAuthoredApplicationSuite/TestSelfUseHasTwoDistinctLocalAddresses`; `TestCrossApplicationCollisionRefusedBeforePayment` (crafted invocation/address values, unrelated same-origin condition, another owner's pending intended address); `TestReplacementAllowsOnlyTheDescribedOwner`; `TestRestoredDeclarationDoesNotReadChangedLibrary`; `TestPendingAuthoredDeliveryResumesWithoutNewOwner`; `TestEarlyAppliedUnfinalizedRecipientRefused`; `TestOldGenerationCannotRemoveNewBenefit`; `TestFrozenGuardsAmountsAndConfigAreOwned`; `TestEndingCannotResurrectAfterReload`. Retain the existing R11 and independent-Raging ward assertions.
- [x] Add these cases against real keepers and the existing application binding.
- [x] Run them; identify the actual admission/copy RED rather than rewriting satisfied paths.
- [x] Implement minimal collision/copy/codec wiring without replacing lifecycle machinery.
- [x] Run new and existing R11/maintenance regressions, then the full root gate.
- [x] Record root run evidence, commit and push the implementation checkpoint; continue T7 in the same module.
**Verification:** root cwd: `go test -count=1 ./conditions -run 'Test(AuthoredApplicationSuite|ApplicationBindingSuite|LinkedDataSuite)$'`; then `go test -race ./...`, `go vet ./...`, `golangci-lint run ./...`.
**Completion evidence:** clean root gates and a pushed SHA. These checks are module proof, not native session/API adoption.

### T7 — Rulebook-local fixtures and a small source demonstration

**Delivers:** C7/R3/R5/R6, public-API usage beside the reusable rulebook implementation.
**Owner:** root module, external `conditions_test` package; no examples/items import or dependency.
**Prerequisites:** canonical fixture data can be installed before T1; executable source/host tests follow T6. Production compiler/binder tests arrive in T4. The root module already pins mechanics/resources v0.3.1; its Resource.Use/Current/SetCurrent API is inspected, not a new provider to build.
**Files:** create `rulebooks/dnd5e/conditions/testdata/effects/library.json`, `conditions/definition_source_test.go`, `definition_world_test.go`, `authored_cookbook_test.go` under the root module. No item-module source, go.mod or go.sum edits.
**Interfaces:** T7 ledger. Test setup supplies Source.ID/Ref/UserID/DefinitionID and a one-use Resource. It calls the public ValidateDefinitionReference, BindDefinition and PrepareApplication APIs; only after valid preflight does Budget.Use(1) settle payment. The returned prepared result is not applied. The host performs paid replacement/delivery/finalization and CheckComplete. Source availability, origin and test-world participants are scaffolding, not authored effect rules.
**Behavior:** load the five primary effect definitions plus a fire-ward data variant from one canonical library. Other unit suites use minimal local contract examples, not a second cookbook. There are no item-name branches, offer registries, refund callbacks or substitute effect handlers. A single small source helper drives all definitions. The host uses real keepers, bus, staged chains and combat settlement; each interaction reloads, binds applications before Apply, then closes binding/local handles. Save source remaining availability explicitly; validate it is within 0..1 before Resource.SetCurrent so clamping cannot repair corrupt saved data. Restoring active conditions does not require their old source catalogue to remain available.
**Tests:** `TestAuthoredEffectsCookbookSuite/TestFiveDefinitionsUsePublicAPI`; `TestInvalidSourceOrPreflightCostsNothing` (bad definition, missing recipient, collision, exhausted/unavailable source); `TestPaymentIsSettledBeforeDeliveryAndOnlyOnce`; `TestPreparationDoesNotDeliver`; `TestFixedBonusNeedsNoBarbarianLevel`; `TestSourceStateRefusesInvalidRemainingBeforeRestore`; `TestNoExamplesDependency` checks the real production+test dependency graph.
- [x] Add the canonical effect fixture data, without constructing a second item format.
- [x] After T6, add external-package source/host assertions and run them to observe missing test plumbing or real API integration RED.
- [x] Implement only the small source/world scaffolding; repair effect defects in production rulebook code.
- [x] Run public-API and dependency-boundary checks.
- [x] Record evidence and commit in the root implementation PR, not a new item PR.
**Verification:** root cwd: `go test -count=1 ./conditions -run 'TestAuthoredEffectsCookbookSuite|TestNoExamplesDependency'`; the seam test inspects `go list -deps -test ./conditions ./effects ./contributions` and rejects examples/items or other examples-module dependencies.
**Completion evidence:** all definitions load and execute through the root module's production APIs. No real item detach, offer, inventory or consumption claim is made.

### T8 — Executable cookbook and joined lifecycle proof

**Delivers:** C8/R2–R5, observable examples rather than isolated green units.
**Owner:** root module, external `conditions_test` cookbook.
**Prerequisites:** T7 and production rulebook APIs in the same worktree; no re-pin.
**Files:** create `rulebooks/dnd5e/conditions/authored_lifecycle_test.go`, `authored_information_test.go`, `authored_example_test.go`; extend `conditions/definition_world_test.go` only for host operations, not effect handlers.
**Interfaces:** approved examples.md policies and existing FollowUp/Consequence protocol. The fixture host may choose a test outcome and deliver the returned exact owner-removal consequence; label that as host-driven, not a native saving-throw resolver run.
**Behavior:** load the canonical fixture bytes, perform paid delivery through the actual provider, rebuild a fresh host from saved source-availability and condition data, and drive the declared holder events. The host supplies observations and test outcomes; all benefit selection, guards, upkeep and ending behavior come from the loaded definitions and provider. Failed delivery and valid pending state remain distinct.
**Tests (all under TestAuthoredEffectsCookbookSuite):**
- `TestRingMaintainsAnotherIdleHolder`: A→B; B's eligible base 10 damage becomes 12, physical 9 settles to 4, Strength save/check advantage applies only to B. Beyond ten B ends benefits remain. B's applied damage produces no concentration follow-up for A; A's produces exactly one CON/DC10 request with item origin. A's zero HP/replacement/rest ends exact recipients; B's zero HP alone is not an implicit ring end.
- `TestScrollUsesRecipientActivityAndCountsTenEnds`: A pays, B owns both coordinator and payload; first B end leaves nine duration units and does not require activity; later missed attack or positive applied damage sustains; A's activity/turns/departure do not. Reload after grace; sustained activity cannot exceed ten B ends; rest/combat end/zero HP have the declared effects.
- `TestWardAndDamageGiftSelectOnlyTheirPieces`: ward has no bonus/advantages; damage gift has no resistance/advantages. Load an authored fire-only variant through the same loader (9 fire→4, physical remains9) without changing a handler.
- `TestSausageInformationMatchesBothAttackBearings`: eligible Strength melee weapon non-opportunity attack gains advantage; Dex/ranged/nonweapon/opportunity variants do not. All attacks against eater, including ranged, gain advantage. Unknown configured detail yields Depends; known-none yields no invented drawback; authoritative mismatch refuses. First subsequent eater turn start removes both sides; another member's turn does not.
- `TestFollowUpConsequenceEndsOnlyItsApplication`: use actual returned OnFailure owner/reason for a host-selected failure; no extra damage instruction or second follow-up. Retain another source's independent ward/Raging. Do not claim native dice/save resolution.
- `TestExhaustedUnavailableSourceAndReloadDoNotRefillOrEndGrant`: test-source availability remains at zero; making it unavailable does not remove its already-granted effect. No real detach/inventory lifecycle is claimed. Changed library data does not modify frozen active benefits; counters/flags/origin survive a new host.
- `TestDeliveryFailureIsNotARefundablePureFailure`: after settled use, fail actual delivery, return that delivery error and keep the price spent. A pending entry with no actual attachment remains valid/saveable under R11; an actual unfinalized attachment must fail CheckComplete. Do not require every pending entry to fail the boundary or claim failed delivery succeeded.
- `TestEndingObservationAndIndependentSources`: information/chain observations remain available at the provider's End-before-detach observation point, then disappear; nonmatching generations/sources remain intact. Do not assume every downstream removal subscriber runs before a keeper detaches its payload.

`Example_dataAuthoredEffects` prints a deterministic summary for all five examples using the same loaded fixture host: selected benefits, maintainer, remaining uses and final ending outcome. Assertions elsewhere pin detailed refusal cases; the Example must actually run the changed path, not print expected labels independently.
- [x] Add the named joined assertions and deterministic Example expectations.
- [x] Run them and capture RED for any incorrect joining/host protocol.
- [x] Implement only host/test plumbing; fix actual rules in their owning provider, not in the fixture driver.
- [x] Run every cookbook assertion, existing root/Raging/Reckless/maintenance regressions and full root gates.
- [x] Save the executable transcript and exact root commit; commit in the same implementation PR.
**Verification:** root cwd: `go test -count=1 -v ./conditions -run 'TestAuthoredEffectsCookbookSuite|Example_dataAuthoredEffects'`; then `go test -race ./...`, `go vet ./...`, `golangci-lint run ./...`.
**Completion evidence:** five executed fixtures and named lifecycle/refusal proofs; evidence states test-host versus native boundaries explicitly.

### T9 — Documentation, joined review and release handoff

**Delivers:** C9/R3, a reader can use the pieces and find honest completion evidence.
**Owner:** root implementation, package README and run record in #2401; idea docs in #2399's documentation worktree.
**Prerequisites:** T6/T8 integrated source gates. No automatic release authorization.
**Files:** update `rulebooks/dnd5e/effects/README.md` in the root implementation worktree to the implemented capability matrix and root cookbook command; update `docs/ideas/data-authored-effects/{README,brief,design,examples,plan}.md`; create `docs/ideas/data-authored-effects/implementation.md`; add `rulebooks/dnd5e/knowledge/runs/2026-11-12-authored-effect-definitions.md` to the root implementation PR. No examples/items documentation change is required.
**Interfaces:** documentation points at the real canonical library and exact verified commits. Replace or mechanically verify the illustrative JSON against the fixture rather than maintaining a second hand-edited definition source. Until runtime merges, use working commit links rather than pretending files exist on main.
**Behavior:** preserve the accepted contracts; record any corrected implementation detail in the PR/plan. One bounded independent review covers the root production API and external-test cookbook after author/operator fixture view; focus closure on findings, not a fresh campaign. Source-verification readiness and actual released adoption remain distinct.
**Checks:** all task/requirement proofs linked; owning full gates and exact-head CI green; record review findings/dispositions; package examples and relative links resolve; doc genre advisories reported without weakening. Pre-commit target's documented Core/Events scope does not replace root gates; never bypass hooks. No browser QA requirement is fabricated for the fixture-only cut.
- [x] Update current package documentation, cookbook instructions and owning run records.
- [x] Run documentation checks and publish the exact executable transcript for operator review.
- [x] Run the required bounded independent joined review and close important findings on the same record (reviews5883376897/5883377068; no blocking findings; Minor deferred to #2404).
- [x] Publish exact-head CI/local gates, limits and release receipts on #946; keep deferred #947 separate.
- [x] Under operator authority, merge/tag neutral observations, adopt the actual tag and rerun root gates, merge/tag root, and publish this documentation closeout. The owning issues record the final documentation merge/ship receipt; #947 is not delivered.
**Verification:** owning root commands above; run `make pre-commit` from the root implementation worktree's repository root and report its documented limited scope or exact coverage-extraction failure without committing incidental Core/Events edits; `git -C /home/kirk/game-dev/rpg-toolkit/.worktrees/authored-effect-rulebook diff --check`; repository-relative link check; run `scripts/check-doc-genres.sh` from the game-dev root checkout. At authorized release, discover the root tag from CI and verify its target against the merge SHA; downstream re-pinning belongs to separately selected #947.
**Completion evidence:** implementation/review/CI/graph receipts and explicit remaining release authority. No branch/worktree/runtime cleanup that destroys the operator's preserved walk or its older provider references.

## Visible plan checks

### Requirement coverage

| In-scope requirement | Implementing tasks | Concrete proof |
|---|---|---|
| C1 — inert configured facts, known/unknown distinction, owned copies (Law20–23) | T1,T5 | TestUnknownVersusKnownEmpty, TestCloneOwnsNestedOutputData, TestObservedUnknownEmptyAndKnown |
| C2 — raw meanings, configurable independent benefits, no item handler (Law1–7,15–16) | T2,T5,T7 | TestFixedDamageAndClassScalingUseExistingMeaning, TestResistanceUsesOneExistingFold, TestFiveDefinitionsUsePublicAPI |
| C3 — independent maintenance and holder clock, neutral end reason (Law13–19) | T3,T8 | TestFirstEndGraceDoesNotSkipDuration, TestScrollUsesRecipientActivityAndCountsTenEnds, TestRingMaintainsAnotherIdleHolder |
| C4 — strict whole-library compilation and role binding (Law2–12,14) | T4,T6,T7 | TestStrictWholeLibraryValidation, TestBindingRolesAndSelfUse, TestCrossApplicationCollisionRefusedBeforePayment |
| C5 — by/against-holder execution/information agreement, no live lookup (Law20–25) | T5,T8 | TestSnapshotDoesNotReadLiveTarget, TestLiveGenerationOrConfigurationMismatchRefusesExecution, TestSausageInformationMatchesBothAttackBearings |
| C6 — current codec, frozen declarations, exact custody/end (Law26–27) | T2,T6,T8 | TestRestoredDeclarationDoesNotReadChangedLibrary, TestPendingAuthoredDeliveryResumesWithoutNewOwner, TestEndingObservationAndIndependentSources |
| C7 — source payment/provenance and no native consumption fiction (Law8–12,28) | T7,T8 | TestInvalidSourceOrPreflightCostsNothing, TestPaymentIsSettledBeforeDeliveryAndOnlyOnce, TestExhaustedUnavailableSourceAndReloadDoNotRefillOrEndGrant |
| C8 — five exact fixture policies (R5; Law29) | T7,T8 | Five named scenario tests plus Example_dataAuthoredEffects |
| C9 — rulebook-local teaching/tests, owning gates, review and honest limits (R6) | T7–T9 | NoExamplesDependency, current capability table, root cookbook transcript, exact commit/gates/review dispositions |

Deferrals are exactly the Scope section: no hidden native inventory/studio/observer-producer work and no all-condition migration. Preserving existing native Raging/Reckless is regression work, not a new native feature claim.

### Provider/consumer seams

| Provider | Consumer | Produced vs consumed contract | Availability / order | Proof |
|---|---|---|---|---|
| Generic effects + #950/#2402 observation support | T1/T2/T3/T5/T6 | Existing PartInput/CurrentOutputs/Snapshot/Lifetime plus owned Output.Clone and Bound.Definition | Provider released v0.6.0 and root re-pinned/verified before its v0.216.0 release | Observation isolation/end-window tests plus configured-fact equality/cookbook |
| T1 contributions | T2/T5 | ConfiguredOutput.Clone and Frame.HeldEffects with exact absence semantics | T1 first; contributions never imports rulebook effects/events | Mutation-isolation and structural-refusal tests |
| T2/T3 kind table | T4 compiler | ValidatePartTemplate class/capabilities, field support and payload/maintenance declarations | Available before each compiler fixture is admitted | Strict whole-library/dual-concentration and field-refusal cases |
| T4 compiler/binder | T6 application | Copied ApplicationSpec + CoordinatorID, immutable Origin/ApplicationID supplied explicitly | Same root worktree, ordered tasks | Self-use/collision, mutation, pending/resume tests |
| T2 output builder + CurrentOutputs | T5 projection | Validated immutable generic.Output, stable live output ID, against-holder bearing | One builder shared; no new interpreter | Canonical round-trip and pre-terminal projection tests |
| T5 effects/conditions | T5 target assessment + T8 host | HeldEffectDetail values and exact local AttachmentIdentity, not Owner address | Producers and consumers built together in root, host adopts afterward | Unknown/empty, wrong generation/configuration and real chain parity |
| T1–T6 production rulebook | T7/T8 external conditions_test package | Public Compile/ValidateReference/Bind/Project APIs; no private helpers | Same module and worktree; no re-pin or examples dependency | Public-API cookbook and TestNoExamplesDependency |
| Existing mechanics/resources v0.3.1 | T7 small test source | Resource.Use, Current and validated restoration | Already pinned by the root module; source scaffolding only | Price/preflight and invalid-source-state tests |
| T7 canonical library | T8 tests/T9 docs | Same loaded bytes and single adapter | Library maintained once; docs link or derive | Five fixture executions + deterministic Go Example |
| Future native producers | New authored target effects in native play | Must populate observed/authoritative configured details | Explicitly **not provided** by this cut | No native acceptance claim; follow-on integration owns it |

Shared-file ordering: T2 establishes parts/kind hooks and clones; T3 extends their lifecycle entries; T5 extends advantage/query/attachment and projection; T6 strengthens application boundaries. T7/T8 consume those public interfaces inside the same module. One root writer owns production and test changes. The demonstrated neutral observation prerequisite is isolated in its own worktree/PR and consumed through a real pin; no examples-module integration is required.

### Findings and dispositions from the plan check

| Finding | Disposition |
|---|---|
| Query.Action is currently outgoing-damage context, including save-shaped actions; changing it globally for attacks would break existing damage meaning. | T5 uses kind-specific attack assessment and tests the old domain explicitly. |
| executeAssessment bypasses custom kind assessment; using a different path for attack execution would split meaning. | T5 makes both paths call one attack evaluator; test rows against actual chain sources. |
| New When/Amount pointers would alias through both existing condition clone helpers. | T2/T6 name both helpers and mutation/round-trip tests. |
| Full-library validation needs concentration capability as well as payload/lifecycle class. | T2 returns existing PartCapabilities from the same kind entry; T4 enforces the existing single-concentration rule, with no name switch. |
| Frame facts cannot import effects/events without a cycle; structural validation cannot know the rulebook registry. | T1 uses core/generic values only; T5 owns semantic validation and exact projection. |
| A terminal snapshot can coexist with still-observable pre-terminal outputs. | T5 projects CurrentOutputs, preserving the existing end-observation contract. |
| Generic linked owner and recipient use the same ref, so self-use needs distinct addresses; foreign address collisions can otherwise fail after payment. | T4 derives distinct payload identity; T6 preflight tests exact planned-replacement exceptions, unrelated live collisions and another live owner's pending bindings. Pending bindings are existing immutable intent, not a new reservation store. |
| until_empty emits spell_ended; a separately versioned resolution test imports the exported Go constant. | T3 changes the sole current reason value to members_empty while keeping the existing exported symbol; no nested-module edit or old-format reader. |
| Definition selection can be validated before participant IDs exist; fake IDs would misrepresent validation. | T4 defines ValidateDefinitionReference; T7 demonstrates it as a public rulebook capability. |
| Moving the demo out of the item package must not introduce another item engine or make payment an effect rule. | T7 uses the already-pinned Resource only in a small external-test source, after pure preflight; T8 keeps delivery failures spent/explicit. |
| A failed delivery may leave valid pending-without-attachment state; treating all pending as a boundary failure would reintroduce global sealing. | T8 distinguishes the returned delivery error from CheckComplete's orphan check and retains R11 resume semantics. |
| The rulebook test source is not proof of production item detachment, consumption or native save/window resolution. | T8 models exhausted/unavailable source state and a host-selected follow-up outcome; real item adoption stays in deferred #947. |
| A package README in both runtime and design branches would create avoidable parallel ownership. | T9 hands the completed README from #2399 to #2401 so its relative links match the implemented source; the design PR removes its copy. One writer remains. |
| The previous cut made examples/items the primary teaching/integration surface rather than the rulebook. | R6 moves T7/T8 and their canonical data into the root module, expands #946 to the complete deliverable, and leaves #947 as separate deferred adoption. |

Plan check result: all C1–C9 requirements have an owner and named proof; every consumed capability has a producing task or the explicit #950 prerequisite. T1–T8 now have committed source and observed verification; exact heads, test boundaries and deviations are in implementation.md. Operator go, independent review and focused real-tag release closure are recorded in implementation.md. Both runtime PRs are merged/tagged; this documentation PR carries closeout, whose actual merge is recorded on the owning issues. The joined review was not repeated.
