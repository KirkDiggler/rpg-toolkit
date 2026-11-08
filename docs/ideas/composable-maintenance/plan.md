# Composable maintenance implementation plan

> Execute directly with the executing-plans and build-slice skills. No delegation or extra plan-approval gate is implied. Read this plan with the approved design; preserve its native/toolkit evidence boundaries.

**Goal:** compose concentration, duration and payload behavior through neutral tools, migrate native Bless/Bane and shared concentration upkeep, and prove concentration-held resistance without a second implementation.

**Architecture:** one coordinator owns a generic lifetime; entity-local assemblies run independent parts using the existing effects machine. Rulebook adapters own identity, transport and meaning. Native roll selection, save execution and host load-act-save stay at their current seams.

**Tech stack:** Go 1.27 multi-module toolkit, Go API, TypeScript/React web, existing generated protobuf SDKs. **Spec:** [design](design.md), [walkthrough](README.md), [acceptance](acceptance.md). **Authority:** [Kirk's detailed approval and fresh-character clarification](https://github.com/KirkDiggler/rpg-toolkit/pull/2323#issuecomment-6726787390).

## Finish line and constraints

Finish the wave through runnable integration, operator early look, bounded native QA, one independent review round and gate-ready PRs. Explicit merge authorization is still required; then publish/tag/repin inside-out. A partial provider is not completion of S3.

- Fresh characters; no historical-save reader, dual writer, normalization, automatic reset or parallel old concentration engine. Unmigrated payload types are current behavior, not compatibility branches.
- Raging remains a current independent consumer with its existing saved representation; removing it or migrating the whole catalog is outside this cut.
- One nearest-go.mod module per toolkit PR, isolated worktrees, absolute `git -C`, no rebase/force push/`--no-verify`/manual tags/replace/go.work.
- Native scope is Bless/Bane plus regression of all shared concentration upkeep. Non-spell resistance is toolkit proof, not native item inventory or a new spell.
- No new RPC or protobuf field is required by the inspected native spell path. If a real consumer needs another fact, stop that seam and solve it at its owner; do not fabricate it in API/web.
- Existing local stacks are retired. Only boot a new isolated maintenance stack at the integration task, not during provider construction. No dev/bench promotion or reset.

## Inspected baseline and sequence

| Repo/module | Baseline used |
|---|---|
| Toolkit | `49e9918b`, then `origin/main d4033752`; the latter only changes play/record, not the inspected effects/root/resolution paths |
| Design worktree | `design/bless-bane-composition`, merged main on top at `855e8f86` |
| API | `origin/dev 3abf710`, worktree `.worktrees/composable-maintenance` |
| Web | `origin/dev e2247b1e`, worktree `.worktrees/composable-maintenance` |
| Protos | Existing published v0.1.260 consumer baseline; no local generation planned |

Development dependency: T1–T4 generic → T5–T8 root → T9 resolution → T10 session → T11 API/web. T12 item-example adoption follows the root provider. T13 joins the real stack. Development consumes pushed provider commits; no provider merge is a development prerequisite. Release order is generic → root → resolution/item example → session → API/web, then record closeout.

Release-unit deliverables under story #703: [S13 #903](https://github.com/KirkDiggler/rpg-project/issues/903) neutral provider (T1–T4); [S14 #904](https://github.com/KirkDiggler/rpg-project/issues/904) shared rulebook parts/application (T5–T8); [S15 #905](https://github.com/KirkDiggler/rpg-project/issues/905) existing item-example adoption (T12); S3 #706 owns native resolution/session/API/web and the integrated proof (T9–T11/T13). Their issues link back here; each Go module still has its own PR.

## Review focus

1. Duplicate/re-entrant end requests and a late notice targeting a replacement: T1/T7 must retain the first reason and refuse foreign application ownership.
2. Teardown/reload mistaken for a gameplay end or fresh initialization: T4/T6/T9 must preserve exact state without new checks, clock steps or effects.
3. Same-beat independent parts and arithmetic overflow: T3/T4 must not suppress a sibling or partially commit a failed update.
4. Self-Bless during concentration damage: T9 must roll with current contributions before failure cleanup; no die is rolled by information.
5. Concrete-class dependencies outside Bless/Bane: T8–T10 and the caller inventory must cover rest, departure, independent children, area-only and zero-delivery holds.

## Shared contracts (new names are implementation contracts)

### Generic provider contracts

- `AmountResult{Scalar *int, Dice string}` is exactly one alternative; a scalar zero is present, not absent. `Validate() error` refuses both/neither. `ScalarValue() (int,error)` refuses a dice description. `AmountFn` returns `(AmountResult,error)`; `Bound.Amount` follows it. `Row.Amount` is `*AmountResult`, nil when there is no answered amount. Dice grammar remains vocabulary-owned.
- `KindSpec.Execution` is `KindSubscribed` (zero/default, requires Attach) or `KindQuery` (requires assessment, no Attach). Unknown modes and a query kind carrying Attach are refused. Existing subscribed consumers adopt the typed amount API; no legacy evaluator remains.
- `Bound.CanReact()` reports current attachment/state/assembly permission to request new work. It is false during End even while contributions remain selectable. The concentration subscriber consumes it rather than caching another ending flag.
- `ComposeInput.InstanceKey` namespaces `Bound.ID`; a standalone empty key preserves the existing single-condition identity, while assemblies require a nonempty instance key.
- `LifetimeInput{ID string, Members []string, Enders []string}` owns copied, unique, nonempty keys in declared order. `NewLifetime(LifetimeInput)(*Lifetime,error)` is fresh creation. `RestoreLifetime(RestoreLifetimeInput{Definition LifetimeInput, Snapshot *LifetimeSnapshot})(*Lifetime,error)` requires a live complete snapshot and a unique subset of declared member keys.
- `LifetimeSnapshot{Phase LifetimePhase, LiveMembers []string}` is defensive. `RequestEnd(EndRequest{Requestor,Reason string})(EndPlan,error)` validates authority/reason and returns `EndPlan{Accepted bool, Reason string, Members []string}`. First acceptance enters Ending; repeat valid requests return Accepted=false and preserve the first reason. `CompleteEnd() error` transitions Ending→Ended; repeated completion is idempotent, completion from Live is refused.
- `MemberRemoved(MemberRemovedInput{Key string})(MemberRemovedOutput,error)` returns `{Removed bool, Before,After int}`. Known already-absent keys are idempotent; unknown keys are errors. Membership cannot change after ending starts.
- `AssemblyInput{Key string, Parts []PartInput, Lifecycle AssemblyLifecycle}`; `PartInput{Key string, Compose ComposeInput, OnTerminal EndScope}`. `EndScope` is local part or application. `Assemble(AssemblyInput)(*Assembly,error)` constructs without attaching; parts may not supply their own End/Changed hooks because assembly owns those hooks.
- `AssemblyLifecycle{End func(context.Context,events.EventBus,AssemblyEnd) error, Changed func(context.Context,events.EventBus,AssemblyChange) error}`. End carries assembly key, part key, scope and reason; Changed carries assembly/part keys. Application-ending parts require End. `Assembly` supplies Apply/Remove/IsApplied, `Snapshot() AssemblySnapshot`, and `CurrentOutputs() []PartOutput`, where `PartOutput{PartKey string, Output *Bound}` follows declared part/output order. `AssemblySnapshot{Parts map[string]Snapshot}` stores every declared part. `RestoreAssembly(RestoreAssemblyInput{Definition AssemblyInput, Snapshot *AssemblySnapshot})(*Assembly,error)` requires exact part coverage and refuses terminal application-ending parts; terminal local parts stay completed and are not attached.
- Machine additions: `Guard.Bag *BagLeaf{Key string, Op Op, Value any}` reads a declared scalar with the same eq/gte type rules as facts; `Act.Add *AddAct{Key string, Delta int}` modifies a declared integer. Overflow refuses the transition without committing any act in that transition or publishing Changed. Neither addition embeds D&D timing.

### Rulebook/application contracts

Use the existing `dnd5e/effects` package for part declarations/binders: inspected imports do not depend on combat/actions, so actions may consume its declaration types without a cycle. Confirm the seam with `go list` after edits; separate data-only declarations if later imports require it, without introducing another catalog.

- `effects.PartDeclaration{Key string, Kind generic.KindName, Params json.RawMessage, OnTerminal generic.EndScope}` is authored data. `effects.BindPart(BindPartInput)(*BindPartOutput,error)` consumes `{Declaration PartDeclaration, Ref core.Ref, HolderID,ApplicationID,DisplayName string, Origin events.ConditionOrigin, Owner events.ConditionAddress}` and produces `{Part generic.PartInput, Capabilities PartCapabilities}`. `PartCapabilities{Concentration,Duration bool}` declares static capabilities; `InspectPart(InspectPartInput{Declaration PartDeclaration, Snapshot generic.Snapshot})(*PartInfo,error)` interprets owned runtime state and returns optional concentration/duration information without callers reading bag keys. The same kind table owns validation and meaning; named descriptors select it.
- `conditions.ApplicationSpec{CoordinatorParts []effects.PartDeclaration, Recipients []RecipientSpec}`; recipient specs name a canonical attachment ref and validated config, with explicit shared or independent lifetime. `PrepareApplicationInput{ID,CoordinatorID string, Origin ConditionOrigin, Spec ApplicationSpec, Existing []ParticipantConditions}` produces immutable `PreparedApplication`, including an optional exact `Replacement` concentration-owner address. It attaches/spends/rolls/removes nothing.
- `ParticipantConditions{MemberID string, Conditions []events.ConditionBehavior}` names the supplied universe. `BindApplicationsInput{Participants []ParticipantConditions, Bus events.EventBus}` produces an interaction-local `ApplicationBinding`; it validates existing linked graphs and binds runtime transport. It is neither a condition catalog nor a persistent/global index.
- `CreateRecipientInput{Prepared *PreparedApplication, SpecIndex int, MemberID string}` produces `CreateRecipientOutput{Condition ConditionBehavior, Receipt DeliveryReceipt}` through the existing registry. `DeliveryReceipt{Key string, Address ConditionAddress, LinkMode LinkMode}` with explicit `LinkComposed` or `LinkAdopted` records only actual admitted delivery; the mode distinguishes backlink-bearing composed nodes and explicitly adopted current payloads.
- `SealApplicationInput{Prepared *PreparedApplication, Delivered []DeliveryReceipt, Binding *ApplicationBinding}` returns the coordinator `ConditionBehavior`. Sealing validates the current delivered nodes, installs immutable address bindings and the initial lifetime. Failed/independent deliveries are not shared members. A zero-delivery hold remains possible according to existing native semantics.
- `ApplicationBinding` registers new prepared/recipient/coordinator attachments, accepts scoped end requests, applies member-removal notices and tears down without gameplay endings. Root keepers/transport verify both application identity and exact addresses before acting; a stale member may not remove a new occupant. The binding captures the interaction bus for in-flight dispatch cleanup.
- `LinkedConditionData`: ref/member/address qualifier/application ID/role, immutable origin and bindings, plus `{Parts AssemblySnapshot, Lifetime *LifetimeSnapshot}` runtime. Lifetime exists only on coordinator; live member keys refer to immutable address bindings. Register `refs.Conditions.LinkedEffects()` for the neutral coordinator. Blessed/Baned keep their canonical recipient refs and caster-qualified replacement addresses; origin is never parsed from a qualifier.
- `LinkedApplicationInfo` exposes identity, role, owner/member links as detached values. `MaintenanceInfo` exposes optional concentration `{Owner ConditionAddress, SourceRef core.Ref, SourceName string, ChildCount int}` and duration `{Remaining int, SkipNext bool}`. More than one concentration claim per maintainer is invalid. `DescribeCondition` supplies display/origin/known duration to native views without API parsing rules.
- Parts: concentration describes one positive-damage CON follow-up (DC max(10,damage/2)), with only the exact coordinator as failure owner; zero HP ends without rolling. Duration owns remaining caster-end count/grace. End-on-combat/rest and until-empty are separate parts. The latter reacts to actual nonempty→empty loss, not empty construction. Roll modifier describes signed unresolved `1d4` for attack/save domains; resistance reuses the existing meaning.
- `effects.DescribeRoll(DescribeRollInput{Output *generic.Bound, Request *events.DescribeRollContributionsInput, Origin events.ConditionOrigin, DisplayName string})(*DescribeRollOutput,error)` returns `{Applicable bool, Group string, Contributions []events.DiceContribution}` through the same kind assessment as information. `Query.Roll *RollQuery{MemberID string, Kind contributions.RollKind}` names the actual recipient, unlike an outgoing save-shaped action. `PartInfo{Concentration bool, Duration *DurationInfo}` interprets part snapshots; payload inspection validates state without inventing maintenance.
- `CastProfile.Maintenance` carries declarations instead of `CastConcentration`; source content supplies data and the root compiler reports the concentration claim. Resolution consumes the prepared contract, not kind/spell names. Toolkit maintenance end information uses generic content identity; native spell projection remains spell-only for this cut, with a named refusal for an unsupported non-spell native presentation.

## Task contracts

Execution checkpoint: generic T1–T4 are source-complete in [toolkit #2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327), through `cc0c8dee`, with missing-API RED, targeted behavioral/mutation RED and full module race/vet/lint GREEN. Their consumer integration, independent wave review and release remain pending. The requested independent provider/adoption review closed its two Important and one Minor findings at provider `5d95849c` / root `08b2d161` ([closure](https://github.com/KirkDiggler/rpg-toolkit/pull/2327#pullrequestreview-5819875758)). T5 then completed its source vocabulary at root `0b2f830f` in [#2329](https://github.com/KirkDiggler/rpg-toolkit/pull/2329), consuming provider `eb5ffb63`'s new CanReact query. Full owning gates pass. That later vocabulary/API addition is not claimed covered by the earlier review. T6–T13, linked transport and native migration remain incomplete.

Each behavioral task follows these steps: add named assertions → observe the expected RED → implement only its contract → run focused checks → run its owning release-unit gates before pushing → update run record and commit. Tests below are required assertions, not completed receipts.

### T1 — Neutral lifetime value

**Delivers:** R3/R4/R7's scoped member ownership and exactly-once end plans. **Owner:** mechanics/effects. **Prerequisites:** inspected generic module only.
**Files:** create `mechanics/effects/lifetime.go`, `lifetime_test.go`; update module `doc.go` and add its dated run record.
**Interfaces:** generic Lifetime contracts above; no bus, rulebook imports or hidden IDs.
**Behavior/tests:** `TestLifetimeSuite/TestEndIsAuthorizedOrderedAndIdempotent`: members a,b; remove a; end by authorized duration with reason expired yields only b; repeat damage end preserves expired and yields no second plan. `TestSnapshotDoesNotAliasCallerOrRuntime`: mutate constructor slices, snapshot slices and plan slices without changing internal state. `TestRestoreRejectsMissingDuplicateForeignOrNonLiveState`: absent snapshot/phase, repeated a, unknown x, Ending/Ended all refused. `TestMembershipAndCompletionRefusals`: unknown removal, removal after end, unauthorized/empty reason and completion before request fail; known duplicate removal is a no-op.
- [x] Add tests and observe missing Lifetime API failure.
- [x] Implement defensive values and state transitions.
- [x] Run `go test ./... -run TestLifetimeSuite -count=1` in mechanics/effects; mutation that accepts a second end fails the first test.
- [x] Run module gates, record and commit checkpoint `646c3117`.
**Evidence:** exact test output and commit, not an application/runtime delivery claim.

### T2 — Descriptive amounts and query-only kinds

**Delivers:** R5 and distinct output instance identity. **Owner:** mechanics/effects. **Prerequisites:** none beyond T1 sharing one writer/PR.
**Files:** create `amount_result.go`, `amount_result_test.go`; modify `output.go`, `vocabulary.go`, `composed.go`, `composed_test.go`, `testvocab_test.go`, `observations_test.go`, `doc.go`.
**Interfaces:** AmountResult, AmountFn, KindSpec.Execution, InstanceKey contracts above.
**Tests:** `TestAmountResultSuite/TestDiceIsNeverEvaluatedByInspection` uses a dice description and a roller spy with zero calls; `TestScalarZeroIsPresentAndDiceRefusesScalarRead`; `TestInvalidUnionIsRefused`; `TestQueryKindHasNoSubscriptionAndStillRequiresAssessment`; `TestSubscribedKindStillRequiresAttach`; `TestInstanceKeysSeparateEqualRecipeOutputs`. Update existing numeric fixtures to explicit scalar results; no legacy integer adapter.
- [x] Write failing assertions and run the named suites.
- [x] Change the single amount contract and kind validation; update all module consumers.
- [x] Verify `go test ./... -count=1`; instance-prefix mutant fails the identity assertion.
- [x] Update docs/run record and commit `5cdd2cb3` after module gates.

### T3 — Neutral countdown operations

**Delivers:** R3 independent duration state without a game-specific generic clock. **Owner:** mechanics/effects/machine within the same module. **Prerequisites:** existing compiler/guard/act contracts.
**Files:** modify `machine/{recipe,guard,acts,validate,machine}.go`; create `machine/bag_arithmetic_test.go`; update `machine/doc.go` and validation/JSON tests.
**Interfaces:** BagLeaf/AddAct above; internal `run` returns `(changed bool, err error)` and applies a transition's acts only after all writes validate.
**Tests:** `TestBagArithmeticSuite/TestCounterGuardAndDeltaRoundTrip` starts remaining=2, fires two declared beats, gets 1 then terminal with unchanged input recipe; `TestRefusesUnknownWrongTypedAndMultipleActForms`; `TestOverflowDoesNotPartiallyWriteOrNotify` puts MaxInt at n and an earlier flag write in the same transition, then adds 1 and expects neither write nor Changed; `TestBagUnknownIsNotFalse` preserves three-valued semantics.
- [x] RED with new guard/act shapes.
- [x] Implement validation, owned staged bag changes and overflow refusal.
- [x] Run `go test ./machine -count=1`; retain all existing Rage machine semantics and seam tests.
- [x] Record overflow-guard mutation and commit `b0b6ce6a` after module gates.

### T4 — Local assembly of existing runtimes

**Delivers:** R3/R4 independent parts, snapshots and teardown. **Owner:** mechanics/effects. **Prerequisites:** T1–T3.
**Files:** create `assembly.go`, `assembly_test.go`; modify `composed.go` only for InstanceKey integration; update `doc.go`, `README.md`, seam/run records.
**Interfaces:** Assembly contracts above. No part may supply End/Changed hooks; assembly owns the lifecycle routing.
**Tests:** `TestAssemblySuite/TestTwoPartsReactToOneBeat` updates two differently keyed bags once; `TestPartCompletionDoesNotEndSibling`; `TestApplicationEndRoutesOnceBeforeDetach`; `TestRestoredCompletedPartDoesNotRevive`; `TestMissingOrTerminalMaintenanceSnapshotIsRefused`; `TestLaterAttachFailureUnwindsEarlierParts`; `TestRemoveOnlyDetachesAndSnapshotsAreOwned`; `TestDuplicateKeysOrNestedLifecycleHooksAreRefused`.
- [x] RED suites.
- [x] Compose each part, namespace output IDs and route callbacks without flattening transitions.
- [x] Run `go test -race ./...`, `go vet ./...`, `golangci-lint run ./...` in mechanics/effects; seam tests retain existing allowed imports.
- [x] Push provider draft #2327 at `cc0c8dee`, with T1–T4 evidence and no merge/tag claim.

### T5 — Rulebook part meanings and typed contribution adoption

**Delivers:** R1/R3/R5 vocabulary, reusing resistance and recipient selection. **Owner:** rulebooks/dnd5e root module. **Prerequisites:** pushed T4 provider pin, not a local replacement.
**Files:** modify `effects/{types,kinds,amounts,attach,query,vocabulary}.go`; create `effects/{parts,concentration_part,duration_part,roll_modifier,parts_test}.go`; update `conditions/composed.go` numeric consumers and current Raging tests.
**Interfaces:** PartDeclaration/BindPart and capability outputs above. The root kind table supplies recipe, output and typed assessment meaning once. No second concentration class or dice subscriber.
**Tests:** `TestPartsSuite/TestConcentrationAndDurationAreIndependent`; `TestConcentrationDescribesOneSaveAndZeroHPDoesNotRoll`; `TestDurationGraceAndRemainingSurviveRestore`; `TestUntilEmptyRequiresActualLastMemberLoss`; `TestRollModifierDescribesDomainsWithoutRolling`; `TestResistanceMeaningIsSharedWithRaging`; existing `TestSharedComposedRaging` for unchanged scalar behavior/current saved shape (reuse, not a duplicate fixture).
- [x] Pin provider with real `go get`/tidy; current provider `eb5ffb63`.
- [x] Observe missing-API/behavioral RED; implement meanings and scalar API adoption.
- [x] Run focused and full root race/vet/lint, including existing Raging creation/information fixtures; reaction/owner-scope/subtraction mutants fail before restoration.
- [x] Commit source checkpoint `0b2f830f`; root stays draft because T6–T8 and consumers are incomplete.

### T6 — Linked codec and current recipient definitions

**Delivers:** R4/R8 one current linked format and source-qualified Bless/Bane. **Owner:** rulebook root. **Prerequisites:** T5.
**Files:** create `conditions/{linked_data,linked_condition,linked_info,linked_data_test}.go`; modify `conditions/{descriptor,registry,factory,loader,blessed,baned}.go`, `refs/conditions.go`; preserve `composed_data.go` for current Raging.
**Interfaces:** LinkedConditionData, LinkedApplicationInfo, MaintenanceInfo/DescribeCondition above; existing CreateFromRef/LoadJSON remain routing boundaries.
**Tests:** `TestLinkedDataSuite/TestOwnerAndRecipientRoundTripWithoutDuplicatedState`; `TestMissingForeignDuplicateAndWrongRoleStateIsRefused`; `TestObsoleteBlessBaneAndConcentrationAreRefusedNotNormalized`; `TestOriginAddressAndApplicationStayDistinct`; `TestCoexistingBlessSourcesUseExistingSelection`; `TestRecipientOnlyInformationDoesNotInventDuration`.
- [ ] Tests start with current creator-produced bytes, not a hand-authored invented snapshot.
- [ ] Implement current codec and registry entries; remove superseded Bless/Bane runtime implementations rather than retain alternate paths.
- [ ] Run `go test ./conditions ./character ./monstertraits -count=1`; fix fixture callers listed in caller inventory without deleting gameplay assertions.
- [ ] Record progress and exact bytes/negative assertions.

### T7 — Preparation, sealing and interaction-local transport

**Delivers:** R2/R4/R6/R7 scoped linked lifecycle. **Owner:** rulebook root. **Prerequisites:** T5/T6.
**Files:** create `conditions/{application,application_binding,application_test,application_binding_test}.go`; update root event/address and keeper helpers only where the inspected protocol requires them.
**Interfaces:** Prepare/CreateRecipient/Seal/BindApplications contracts above; explicit current participant list, typed receipts and end transport. Identity comes from the caller; no hidden UUID/clock.
**Tests:** `TestApplicationSuite/TestPreparationDoesNotAttachSpendRollOrDisplace`; `TestOnlyDeliveredSharedRecipientsJoin`; `TestZeroDeliveryDoesNotInventLastChildLoss`; `TestSourceQualifiedReplacementForgetsOldMemberBeforeNewJoin`; `TestApplicationBindingSuite/TestMissingForeignAndCompetingLinksFailBeforeMutation`; `TestDuplicateRemovalIsIdempotentAndStaleRemovalCannotHitReplacement`; `TestChildRemovalAndSelfRecipientDoNotStripSiblings`; `TestEndOrderingAndExternalOwnerFactAreExactlyOnce`; `TestBindingTeardownDoesNotEndSavedApplications`.
- [ ] RED against missing protocol and current ref-only composed removal.
- [ ] Implement graph validation, owned binding state and actual removal transport; no observer that merely logs an unfulfilled end request.
- [ ] Run root focused suites; inject a missing recipient and verify no coordinator deletion is returned as success.
- [ ] Record the joined source-to-removal behavior, not just value-unit tests.

### T8 — Shared upkeep, views and current spell declarations

**Delivers:** R6/R8/R9 one current concentration path. **Owner:** rulebook root. **Prerequisites:** T7.
**Files:** modify `combat/actions/cast.go`, `spells/cast.go`, `character/{character,status_view}.go`, monster condition/view counterparts in caller inventory, `conditions/{registry,factory,concentrating}.go`, current root tests in inventory; remove superseded concentration implementation/data constructors.
**Interfaces:** CastProfile.Maintenance and protocol queries above. Keep all current spell maximum durations/grace/independent-child declarations; no one-off Bless/Bane branch.
**Tests:** `TestMaintenanceDeclarationsSuite/TestEveryCurrentConcentrationProfileUsesParts`; `TestOneConcentrationClaimIsReportedAndMultipleClaimsAreRefused`; `TestGenericSourceDoesNotPretendToBeASpell`; `TestConcentratedResistanceWithAndWithoutMaintenance`; `TestIndependentRagingSurvivesWardEnd`; existing root concentration/rest/keeper tests rewritten through current constructors with the same behavior assertions.
- [ ] Pin data/behavior boundaries with import/seam check and RED tests.
- [ ] Migrate declarations/views and delete old runtime/codec paths together.
- [ ] Run full root `go test -race ./...`, `go vet ./...`, `golangci-lint run ./...` in root module only.
- [ ] Push root draft with closed caller-inventory disposition and current-format evidence.

### T9 — Real native delivery, damage, rest and departure

**Delivers:** R6/R7/R9 native concentration behavior. **Owner:** resolution module. **Prerequisites:** pushed root T8.
**Files:** modify `resolution/{action,resolve,concentration,damagetaken,depart,long_rest,short_rest,projection,check}.go` as applicable to the truth-bearing entry inventory; create `resolution/linked_application_test.go`; update inspected concentration/Bless/cast/area/departure fixtures in caller inventory.
**Interfaces:** root prepared application/binding APIs; existing cast outcome/FollowUp/removal fact protocols. Pass a stable application identity into cast creation and freeze/resume it; native callers supply current act identity plus an application ordinal, never silently regenerate on resume.
**Tests:** `TestLinkedApplicationSuite/TestCastSealReloadAndExactRemoval`; `TestSelfBlessConcentrationUsesDieBeforeCleanup`; `TestTwoRecipientsProduceOneDamageFollowUp`; `TestARefusedCastDoesNotDisplaceOrPay`; `TestPaidZeroDeliveryAndRecastKeepCurrentSemantics`; `TestAreaOnlyAndIndependentPayloadUseSameUpkeep`; `TestDepartureRequiresAllAffectedHoldersAndLeavesNoOrphans`; `TestUnsupportedNonSpellNativePresentationIsRefusedByName`; existing `TestConcentrationSuite` and `TestCastActionTestSuite/TestBless` remain live regression proof.
- [ ] Pin pushed root with go get/tidy; RED real resolver fixtures, not manual damage-received publishers.
- [ ] Bind/validate at each applicable attach entry, migrate concrete assertions and preserve typed save/record paths.
- [ ] Full module race/vet/lint; native damage trace applied once and exact linked dirty saves asserted.
- [ ] Push draft; no unrun browser claim.

### T10 — Session continuation, information and persistence

**Delivers:** R4/R7/R9 current canonical data across host verbs. **Owner:** session module. **Prerequisites:** pushed T9 and root.
**Files:** modify `session/{cast,write,turn,rest,depart,information,effects}.go` only where actual protocol/pin changes require; create `session/linked_application_test.go`; update `concentration_break_test.go`, `turn_concentration_test.go`, `bless_test.go`, `bless_internal_test.go` and inventoried callers.
**Interfaces:** stable native application identity from `scope.enc.CurrentAct()` with deterministic local ordinal; frozen continuation carries it; host receives data and information, no runtime objects.
**Tests:** `TestLinkedSessionSuite/TestFreshCastRoundTripsAcrossInteractions`; `TestResumeKeepsApplicationIdentity`; `TestSourceRowsAndMaintenanceSummarySurviveReload`; `TestRecastRestLeaveAndAreaClosurePersistEveryDirtyHolder`; `TestRecipientInformationWithNoOwnerContextHasNoFakeCountdown`; existing effect-row identities and source IDs remain independently asserted.
- [ ] RED the persistence/reader joins and adopt real provider pins.
- [ ] Implement only host wiring and data custody; no concentration or dice rules in session.
- [ ] Full session race/vet/lint and structural no-inner-type seam tests.
- [ ] Push draft for API adoption.

### T11 — API/web current-data adoption

**Delivers:** R8–R10 real reader boundary. **Owners:** API and web, separate PRs. **Prerequisites:** pushed T10; existing generated SDK.
**Files (API):** modify `internal/handlers/dnd5e/v1alpha1/character/converters.go`, add `linked_conditions_test.go`; extend `internal/integration/session/bless_acceptance_test.go`, add `linked_maintenance_acceptance_test.go`; pin go.mod/go.sum. Existing session/act converters only change if actual provider types require it.
**Files (web):** `src/types/conditionData.ts`, `conditionData.test.ts`, `src/components/features/ConditionBadge.tsx` and tests; existing `combat-experience/EffectRows.test.tsx`, `actionTooltip.test.ts`, `ActionDockEffects.test.tsx` for native mounted projection proof.
**Interfaces:** root-owned metadata and unchanged native spell SDK facts. API preserves raw bytes and maps metadata; web validates current shape, keeps bindings opaque and renders server facts. No new method permission or proto generation because no RPC/wire addition is planned.
**Tests:** API `TestConvertLinkedConditionPreservesBytesSourceAndDescription`, `TestLinkedMaintenanceAcceptanceSuite/TestNativeCreationCastReloadAndConcentrationEnd` using actual handlers/miniredis; web `rejects obsolete linked condition data`, `renders server maintenance without calculating remaining turns`, `preserves selected dice and separate source identities`, plus mounted HUD regression. Explicitly distinguish a component-only badge from the native HUD.
- [ ] RED current-reader/projection tests with provider-produced data.
- [ ] Implement source mapping/readers, not rules or normalization.
- [ ] API `make test` and `make ci-check`; web focused tests then one complete `npm run ci-check` at PR boundary.
- [ ] Push drafts with exact dependency/SDK provenance and no native gameplay claim yet.

### T12 — Existing item consumers still consume the provider

**Delivers:** current Second Wind/Rage item examples remain valid after generic/root API changes. **Owner:** examples/items module. **Prerequisites:** root T8/provider T4.
**Files:** `examples/items/go.mod`, `go.sum`, affected existing test consumers only, module run record. No native inventory implementation.
**Tests:** retain fixed non-Barbarian Rage bonus, independent item/class pools, source identity, spent loot, transfer, reload and grace assertions. No compatibility shims for the old amount contract.
- [ ] Adopt pushed providers and run examples/race/vet/lint in this module.
- [ ] Correct direct API usage only if compilation identifies it; preserve all behavior assertions.
- [ ] Push its own module draft with evidence and planned real-tag re-pin.

### T13 — Native acceptance, review and authorized release

**Delivers:** S3's integrated finish line. **Owner:** cross-team verification; no new gameplay owner. **Prerequisites:** T1–T12 green on pushed real pins.
**Files/artifacts:** untracked API `content/composable-maintenance-walk.yaml`, local manifest `envs/local/composable-maintenance.env`, checked mounted content, `docs/ideas/composable-maintenance/{implementation,qa}.md`, issue checklist and PR receipts. Design worktree owns docs; source worktrees own code. Ports chosen only after checking availability.
**Native scenario:** normally create eligible casters and recipient; verify acquisition/private sheet; cast multi-recipient Bless/Bane; inspect source-aware rows; reload; exercise replacement/removal/rest/departure. Repeat relevant castle fight path. Positive damage/concentration outcomes may be named real-path test-backed where dice miss in the bounded walk; never force live evidence. Resistance example remains toolkit proof.
- [ ] Boot isolated stack, verify source/served content and pages, then give operator early look before independent review/walk.
- [ ] One independent runtime review round, disposition findings, complete bounded native QA with measured evidence.
- [ ] Report exact heads and remaining merge authority; do not auto-merge.
- [ ] On explicit merge-out authorization, merge/tag/repin inside-out, rerun consumer gates/head CI, close only delivered slices and clean only this wave's stack/worktrees.
- [ ] Keep journal #863 open; preserve investigation, failures and proof limits.

## Requirement coverage

| Requirement / acceptance | Tasks | Concrete proof |
|---|---|---|
| R1 reusable concentration/duration/resistance | T5/T8 | same resistance meaning, with/without maintenance, independent Raging remains |
| R2 local custody/shared lifetime | T1/T6/T7/T10 | owner-only lifetime, recipient links, joined reload and dirty persistence |
| R3 independent part reactions | T3/T4 | TwoPartsReactToOneBeat, overflow no partial update, no interpreter import expansion |
| R4 identities/state once | T1/T2/T6/T7/T10 | no alias, instance ID separation, foreign/stale replacement refusal, resume keeps ID |
| R5 dice/selection | T2/T5/T9/T11 | zero-roll information, scalar/dice refusal, two Bless + Bane, self-Bless save |
| R6 pure prepare and paid delivery | T7/T8/T9 | no side effects before door, only actual receipts, zero-delivery/recast |
| R7 end/refusal/teardown | T1/T4/T7/T9/T10 | exactly once order, external fact not doubled, no orphan leave, teardown not expiry |
| R8 one fresh-character path | T6/T8/T11 | old linked formats fail, old concentration symbols retired, no automatic reset |
| R9 shared upkeep/current consumers | T8/T9/T10/T12 | inventory closure, area/independent/rest/departure regressions and item adoption |
| R10 native/toolkit evidence boundaries | T11/T13 | real acquisition/API persistence/HUD walk; explicit resistance and dice proof limits |

## Provider/consumer seams

| Provider | Consumer | Produced vs consumed contract | Availability | Joined proof |
|---|---|---|---|---|
| T1/T4 generic | T5–T7 root | Lifetime/Assembly inputs, owned snapshots, EndPlan and callbacks as declared above | pushed provider pin | root concentration-held resistance and end transport |
| T2 generic | T5 root/T12 items | AmountResult and query-kind validation; scalar consumers use ScalarValue | pushed provider pin | no-roll description + old numeric behavior |
| T3 machine | T5 duration part | BagLeaf/AddAct, checked all-or-nothing transition | same provider module | grace/countdown reload |
| T5–T8 root | T9 resolution | prepared/bound/sealed application, current profile/metadata, exact receipt addresses | pushed root pin | native cast/damage/departure with real sheets |
| T9 resolution | T10 session | canonical data, stable application ID, outcomes/dirty holders | pushed resolution + root pins | fresh-interaction persistence and resumed identity |
| T10 session/root | T11 API | current bytes and provider metadata, no inferred rules | pushed session pin | actual handler/miniredis cast/reload/end |
| Existing SDK/API facts | T11 web | native spell identities, effect rows/source IDs and server descriptions | existing generated release; no bindings generated locally | native HUD + component tests |
| T11 API/web | T13 walk | verified images/source refs and served YAML | built only at integration | two-client native journey and relevant castle repeat |

## Visible check findings and dispositions

- **First-match trap:** resolved by T4's independent runtimes; no task concatenates part transitions.
- **Scalar-only evaluator:** resolved by the single typed API in T2 and all producer/consumer updates; no adapter rolls a die to satisfy an int.
- **Hidden concrete concentration callers:** 51 toolkit Go files are inventoried in [callers.md](callers.md); T6/T8–T10 own their fixture/protocol migration. Preserve assertions rather than deleting evidence.
- **Fresh-data ruling:** resolved by removal/refusal tests. Current Raging and unmigrated payloads are not obsolete compatibility paths.
- **Authored data dependency:** inspected imports permit action maintenance declarations to consume root effect declarations; verify no cycle at T8, keeping pure declaration data separate if needed.
- **Part binding metadata:** root Bind already requires authored display/origin, and current outputs carry their attachment ref. BindPartInput therefore explicitly receives Ref and DisplayName; neither is inferred from a part kind or opaque instance ID. This fills the agreed binding contract rather than adding a new metadata authority.
- **Scoped generation safety:** application ID is distinct from recipient replacement address; T7 validates current ownership, and T9/T10 retain ID across a frozen continuation.
- **Missing native consumer fact:** none required for the scoped spell path today. A newly exposed fact is an architectural gap at that boundary, not an excuse for a client calculation.
- **Path check:** the initial task sketch said session/leave.go; inspection identifies session/depart.go. The task now names the real path; no worker is left to guess it.
- **Shared-file order:** T1–T4 are one module writer/PR; T5–T8 one root writer/PR. Nested modules use separate worktrees/pins. API and web worktrees are isolated and currently inspection-only.
- **Verification state:** commands/tests in this plan are expected checks. Existing baseline receipts are on #706; new-task checkboxes remain unchecked until their evidence exists.

No required architectural decision remains intentionally hidden in a task. If fresh consumer inspection contradicts a contract, record the exact mismatch and apply the approved architectural-gap rule before changing its owner or scope.
