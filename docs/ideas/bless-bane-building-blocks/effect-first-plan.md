# Effect-first definitions implementation plan

> **For agentic workers:** use executing-plans for direct sequential execution. No implementation delegation is authorized. Keep one writer per worktree and one joined independent review after the actual consumer look, not a review campaign per task. This plan replaces the retained first-cut plan; do not redo its completed selector/leaf work.

**Goal:** complete typed rulebook definitions and configuration references share effect-first compositions, proved by native Rage, a concentration ring and an activity-sustained resistance ward.

**Architecture:** pure typed projection/data resolution → one canonical definition compiler → explicit placement → existing local/linked assemblies and custody. The named definitions package owns discoverable content; the vocabulary owns meanings. Native Rage uses the whole definition, not an unrelated private recipe.

**Tech stack:** Go1.27, existing effects/machine/assembly/lifetime, testify; current API handler/session seams; TypeScript/Vitest for the current opaque reader and provider-owned progress.

**Spec:** [design](design.md), [composition contract](composition-contracts.md), [reference/variant contract](reference-contracts.md), [binding/state mapping](binding-contracts.md), [acceptance](acceptance.md).

**Authority:** R6–R8 direction, [R9 acceptance](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6873569590), [R10 acceptance](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6883013648). API spellings and the behavior-preserving binding mapping below are lead implementation choices. Return only an actual new rules/ownership decision; no new approval gate for a faithful plan. Merge authority remains separate.

## Finish line, scope and inspected baseline

- S19#955 / neutral#2426 at185aae3c is already implemented. Retain it and its tests; root currently consumes real `v0.6.1-0.20261113213522-185aae3c494e`.
- S20#956 / root#2430 continues in `.worktrees/bless-bane-building-blocks`, branch `feat/bless-bane-building-blocks`. Production baseline c99f3d85; characterization checkpoint13fdf1c9 adds two native terminal-order tests with no production change. Full root race/vet/lint pass there.
- S21#957 / API tree of the same name starts at dev2fe647ea, branch `chore/bless-bane-building-blocks`; preserve newer parallel pins on catch-up. No adoption is yet done.
- S22#962 / web inspection tree of the same name is ataa481c03. Branch from/catch up to dev before writing. Its composed-Raging reader consumes `runtime.bag.turns_active`; the API already maps DescribeCondition detail to notes/private detail.
- Design and evidence remain in `.worktrees/effect-foundation-review`, PR2425. Runtime code stays in its owning trees.
- Deliver the implemented typed/configured three-consumer proof, source/IDE and native reader look, owning/consumer gates, one joined review and exact ready heads. Release only on operator go, inside-out on actual tags. No native ring acquisition/studio authoring/production promotion or operator-data reset is claimed.

## Global constraints

1. One nearest-go.mod module per runtime PR; root E1–E6 continue one root PR. Provider edits only for a measured missing capability, never local source overrides.
2. Develop on actual pushed pseudo-versions. Re-pin consumers to actual CI tags only after authorized provider merges. Never predict tags, rebase, force-push, skip hooks, add replace/go.work or reset operator data.
3. Keep coexistence/applicability/selection/arithmetic independent; preserve fixed recipient sets, pending finalization, source-owned payment, exact generation cleanup and End-before-detach.
4. No second interpreter, global mutable catalogue, deep field merge, implicit lifecycle or latest-catalogue reload.
5. Current native representation changes explicitly; no historical reader/dual writer/normalization. Other unmigrated payloads remain intact.
6. Source, fixture, API and browser evidence are different claims. A test-selected save outcome is not a natural roll. Source examples do not prove native inventory.
7. Required owning gates remain intact. Disclose the known unchanged Core pre-commit extraction/reporting failure separately; do not claim that target green.

## Review focus

- Cyclic/aliased definition graphs: reject active cycles, allow repeated acyclic subtrees with independent paths → E1/E2.
- Root-leaf/empty-value semantics and shadowing keys: no fake registration or last-writer-wins → E1/E5.
- Late invalid replacement or overlapping mounts: no partial results or mutation-order semantics → E2/E5.
- Native sparse/roundless/reset clocks and simultaneous endings: preserve measured semantics rather than substitute a turn counter → E3/E4.
- Current-format adoption drops progress/source or revives state: test frozen declarations, strict restore, existing notes/detail and opaque web reader together → E4/E6–E8.

## Sequence and release units

E1 canonical compiler → E2 catalogue/derivation → E3 maintenance parity → E4 complete named definition/local host → E5 linked placement/data consumers → E6 joined rulebook proof → E7 API → E8 web/native observation. E2/E3 are conceptually independent but use one sequential root writer. Do not wait for a merge to begin a consumer.

Root E1–E6 ship in existing#2430, API E7 in S21 and web E8 in S22. Existing neutral#2426 remains a provider prerequisite. Release order, only after go: neutral actual tag → root final re-pin/gates/merge/tag → API final re-pin/gates/dev merge → matching web dev merge. Read actual module graphs and merge targets; preserve parallel work.

## Task 1 — E1: Canonical values, intrinsic compilation and explicit lowering

**Owner/delivers:** root effects; R9 compiler/placement boundary. **Prerequisite:** baseline/source graph above.

**Files:** create `effects/definition_data.go`, `definition_projection.go`, `definition_compile.go`, `placement.go` and matching `_test.go`; update `effects/definitions.go`, `compile_parts.go`, `compile_outputs.go`, `part_templates.go`, `parts.go`, `part_validation.go`, `kinds.go`, `payload_parts.go`, `add_to_roll.go`, `duration_part.go`, `activity_part.go`, `concentration_part.go`, `end_parts.go`. Update owning docs and run record.

**Interfaces:** canonical `DefinitionData{Effect *EffectSpec, Composition *CompositionData}`; `EffectSpec` is the existing unbound kind/params/When/Amount shape, replacing the misleading PartTemplate name rather than creating a second format; `CompositionData{Members []MemberData}`; `MemberData{Key,Name,Description string; Definition DefinitionData}`. Typed `Definition` exposes `DefinitionData() (DefinitionData,error)`; built-in leaves, typed groups, canonical data and generic Go compositions project through it. Generic Go `Composition{Members []Member}` uses `Member{Key,Name,Description string; Definition Definition}`. Projection is pure and no interface/callback is stored in compiled state.

`Compile(*CompileInput{Definition Definition}) (*CompileOutput{Definition *CompiledDefinition},error)` is the single fallible facade. CompiledDefinition owns canonical data; `Data()` and `Leaves()` return owned views. `CompiledLeaf{Path []string; Effect EffectSpec}` is an ordered derived view, not a second mutable authority. `ValidateEffect(*ValidateEffectInput{Effect EffectSpec}) (*ValidateEffectOutput{Capabilities PartCapabilities},error)` uses the kind table without choosing a scope. Extend PartCapabilities with `AllowedScopes []generic.EndScope` and `RoundDuration bool`; return owned scope slices and keep world/attachment checks in binding.

`Place(*PlaceInput{Definition *CompiledDefinition, Mounts []Mount}) (*PlaceOutput{Parts []PlacedPart},error)`; `Mount{Key string,Path []string,OnTerminal generic.EndScope}`; `PlacedPart{MountKey string,Declaration PartDeclaration}`. Exactly one mount covers each leaf. The mount key prefixes the selected path suffix; root leaf uses the mount key. Placement is pure and creates no actor/application identity.

**Behavior:** exactly one canonical node form; explicit empty composition valid; zero definition invalid. Unique nonblank sibling keys excluding `/`; preserve depth-first authored order. Reject active cycles in canonical and supported generic Go composition graphs; permit shared acyclic subtrees under distinct paths. Clone guards/amounts/params/slices. Split intrinsic versus scope validation; never synthesize fake key/holder/OnTerminal to validate a leaf. Reject missing/overlapping/invalid mounts, unsupported scopes and duplicate mount keys without partial output. Existing CompileParts callers can bridge to the intrinsic compiler plus explicit placement while migration proceeds; no second validator remains.

**Tests:** `TestDefinitionCompilerSuite` in effects: `TestRootEffectWithoutCatalogueOrHolder`, `TestExplicitEmptyVersusAbsent`, `TestOrderedQualifiedLeaves`, `TestDuplicateSiblingKeysRefuse`, `TestRepeatedSubtreeIsIndependent`, `TestCanonicalAndGoCyclesRefuse`, `TestMalformedLaterLeafRefusesAll`, `TestCompilationOwnsNestedValues`. `TestPlacementSuite`: root leaf, two group mounts, uncovered leaf, overlapping paths, duplicate keys, unknown path/unsupported scope and no implicit semantics for a group called maintenance.

- [ ] Write the named tests and observe the missing/new-contract RED.
- [ ] Implement projection/data/intrinsic compiler, then placement; keep existing callers on the same validators.
- [ ] Run `go test -count=1 ./effects ./conditions ./contributions` and full root `go test ./...`; inspect nil/typed-nil and cycle failures explicitly.
- [ ] Update docs/run record, commit/push the working root checkpoint; do not claim later consumers complete.

**Evidence:** canonical/path/ownership/error assertions, no runtime IDs/catalogue requirement, green root baseline and source SHA.

## Task 2 — E2: Catalogue, reference resolution and whole-member derivation

**Owner/delivers:** root effects plus refs; accepted R10. **Prerequisite:** E1.

**Files:** create `effects/catalogue.go`, `catalogue_data.go`, `reference.go`, `reference_resolve.go`, `derive.go`, `catalogue_test.go`, `reference_test.go`, `derive_test.go`; add `refs/definitions.go`; keep catalogue codecs out of core.

**Interfaces:** `DefinitionReference{Definition core.Ref,Path []string}`; `AuthoredDefinition` has exactly one effect/composition/use form, with authored composition members recursively containing authored definitions. `DefinitionUse{Reference DefinitionReference,Replace []MemberReplacement}`; `MemberReplacement{Path []string,Definition AuthoredDefinition}`. `CatalogueEntry{Ref core.Ref,Name,Description string,Definition AuthoredDefinition}`; `BuildCatalogue(*BuildCatalogueInput{Entries []CatalogueEntry}) (*BuildCatalogueOutput{Catalogue *Catalogue},error)` returns an immutable catalogue after whole-set validation. `Resolve(*ResolveInput{Definition AuthoredDefinition,Catalogue *Catalogue}) (*ResolveOutput{Definition DefinitionData},error)` supports inline content without a catalogue. `Derive(*DeriveInput{Definition DefinitionData, Replace []ResolvedMemberReplacement}) (*DeriveOutput{Definition DefinitionData},error)` is the pure canonical helper, where `ResolvedMemberReplacement{Path []string,Definition DefinitionData}` contains no uses. `DescribeCatalogue(*DescribeCatalogueInput{Catalogue *Catalogue}) (*DescribeCatalogueOutput{Entries []DefinitionDescription},error)` returns detached descriptions with Ref, Path, node form, Name/Description, declared Order and leaf Kind.

**Behavior:** definition refs use the definitions namespace; wire refs are strings validated by core.ParseString, with path arrays separate. Do not inherit core's permissive historical object decoding. Forward references are allowed. Resolve selected roots/groups/leaves exactly; no implicit parameter/class substitution. Validate replacement paths against the selected source before applying any: nonempty, existing, unique, no ancestor/descendant overlap. Replace a whole member value, retaining key/order/metadata; no params/amount/array merge. Resolve reference-bearing replacements and refuse active expansion cycles while permitting acyclic same-entry member reuse. Validate unused catalogue entries too. No network/global registration or live catalogue in instances.

**Tests:** `TestDefinitionCatalogueSuite` (typed/data entry equality, enumeration ownership, duplicate ref, unused invalid entry, forward reference); `TestDefinitionReferenceSuite` (root/group/member, missing/leaf traversal, strict wire ref, self/cross cycle, acyclic same-entry reuse, repeated selection ownership); `TestDefinitionDerivationSuite` (fixed2 replacement leaves original live scaling unchanged, late invalid/overlap/root replacement refusal, complete replacement rather than merge). JSON sketches in reference-contracts become executable fixtures.

- [ ] Add named resolver/derivation tests and observe RED.
- [ ] Implement strict codecs and resolution over E1 data; keep one intrinsic compiler.
- [ ] Run `go test -count=1 ./effects ./refs` then full root tests.
- [ ] Publish exact contract/fixture evidence and source checkpoint.

**Evidence:** owned resolved trees with no uses left inside, expected errors before any binding, actual code/data equality.

## Task 3 — E3: Native maintenance parity as reusable parts

**Owner/delivers:** root effects; R8 behavior-preserving decomposition. **Prerequisites:** E1; existing native characterization13fdf1c9.

**Files:** create `effects/round_duration.go`, `round_duration_test.go`; update `effects/definitions.go`, `kinds.go`, `parts.go`, `maintenance.go`, `activity_part.go`, `duration_part.go`, validation/inspection tests. Preserve the existing machine implementation.

**Interfaces:** typed `CombatActivity{SkipFirst bool}` over the existing kind. `RoundDuration{Rounds int,ExpiredReason,ResetReason string}`; kind `round_duration`, positive rounds and explicit nonblank reasons. It lowers to existing machine AnchorRound/DeriveTurnsActive and round guards, not a new clock. `RoundDurationInfo{Anchored bool,RoundActivated,TurnsActive,TotalRounds int}` is owned inspection data; extend PartInfo/maintenance descriptions with round-progress fields rather than pretending they are turn-end counts. Activity/round-duration support EndAttachment and EndApplication, while concentration/until-empty retain required application custody.

**Behavior:** match native grace/activity/clock cases in native-maintenance-evidence.md. Order activity before round duration so first accepted termination gates the sibling. Zero/negative round does not anchor; missing round retains known progress and is not reset; positive lower round resets; elapsed cap uses rounds-anchor≥Rounds-1. No activity and elapsed/reset simultaneously report no_combat_activity. Positive DamageTaken only; never apply HP again. Each part owns independent state and valid restore bounds.

**Tests:** `TestRoundDurationSuite`: sparse rounds1,2,3,4,8,9,10 expire10; anchor4 expires13; progress1,2,3,6 at4,5,6,9; unanchored roundless then6; negative then2; missing versus reset; invalid/terminal snapshots. `TestRagingMaintenanceAssemblySuite`: first-end grace, misses/positive damage, idle versus elapsed/reset precedence, local/application scope routing and End-before-detach contribution availability. Existing native suites remain unchanged until E4 rewires the consumer.

- [ ] Add new-part/assembly tests, observe RED before adding the meaning.
- [ ] Implement the reusable policy using existing machine tools; no named-Rage runtime branch.
- [ ] Run effects/native Raging suites, then full root race tests and vet/lint.
- [ ] Commit/push and record the exact native parity matrix.

**Evidence:** literal clock outcomes/reasons and independent state/ending assertions, not only happy-path consecutive turns.

## Task 4 — E4: Complete typed Raging definition and local assembly host

**Owner/delivers:** root named definitions/conditions/projection. **Prerequisites:** E1–E3.

**Files:** create `definitions/doc.go`, `definitions/raging.go`, `definitions/catalogue.go`, `definitions/raging_test.go`; update `conditions/raging_definition.go`, `raging_benefits.go`, `composed.go`, `composed_data.go`, `descriptor.go`, `linked_info.go`, `character/status_view.go` and current Raging/factory/restore tests. Remove the asymmetric helper/private recipe after parity is established; no duplicate implementation remains.

**Interfaces:** `definitions.Raging(*RagingInput{DamageBonus RagingDamageBonus}) (*RagingDefinition,error)`; `definitions.RagingDamageBonus{Fixed *int,BarbarianLevel bool}` retains existing JSON/choice semantics and moves to its named-content owner. Update native creation callers; do not add another persisted format. RagingDefinition exposes Benefits and Maintenance typed fields; benefits expose DamageBonus/PhysicalResistance/StrengthSavingThrows/StrengthChecks. Maintenance exposes CombatActivity, Duration and an Endings group with ZeroHP/Rest/CombatEnd. Every group/whole value implements DefinitionData projection from its actual fields, with fixed member keys/order. The catalogue entry uses refs.Definitions.Raging, not the active condition ref.

Native local host compiles the whole definition and places benefits with EndPart and maintenance with EndAttachment. ComposedCondition becomes assembly-backed, retains ref/origin/by-ref admission and existing ConditionBehavior/assessment protocols. `LocalBindings{Parts []PartDeclaration}` is the immutable resolved local binding payload. ComposedData retains Ref/MemberID/Origin and stores `Bindings *LocalBindings` plus `Runtime *generic.AssemblySnapshot` (`runtime.parts`); old single-machine active data is refused, never normalized. CreateFromRef's RagingBindings input remains the source's construction contract, not the active save format. Load uses saved resolved parts, not latest catalogue reconstruction.

Add a shared maintenance-inspection capability implemented by local and linked conditions. DescribeCondition provides round progress in Display.Detail, preserving “Active for N turns” through existing API fields. No duplicate mutable top-level progress bag. Native activation/price/cast restrictions remain at their existing owners.

**Tests:** `TestCompleteRagingDefinitionSuite` (fields/full/group/member projections share actual values, owned mutation and native catalogue equivalence); existing Raging suites including13fdf1c9 priority tests through the new native instance; `TestComposedDefinitionRestoreSuite` (frozen parts, exact snapshot coverage, no terminal required maintenance revival, old current-format refusal, origin/ref unchanged); status-view tests assert provider progress1/2/3/6 and source. Verify no separate private ragingRecipe remains as an execution authority.

- [ ] Write new definition/host/state tests, watch old shape fail where behavior/API changes.
- [ ] Build typed definition/projection, then local adapter; preserve native semantic assertions while migrating layout-specific fixtures.
- [ ] Run root race/vet/lint and inspect the complete production call chain from features.Rage through CreateFromRef.
- [ ] Publish current-envelope/projection examples for E7/E8; commit/push.

**Evidence:** actual native condition consumes complete definition; inspected state/identity contract and fresh regression gates, not just matching two duplicate recipes.

## Task 5 — E5: Linked application placement and data-authoring migration

**Owner/delivers:** root conditions/adapters; no second binder. **Prerequisites:** E1/E2/E4.

**Files:** replace condition-first authoring portions of `conditions/definition.go`, `definition_compile.go`, `definition_bind.go`, `composition.go`; update `application.go`, associated tests, `conditions/testdata/effects/library.json`, code/data fixture helpers. Preserve `application_binding.go` ownership protocol unless a measured seam requires an explicit fix.

**Interfaces:** `ApplicationDefinition{Name string,Coordinator DefinitionRole,Definition effects.Definition,Placements []ApplicationPlacement}`; `ApplicationPlacement{Key string,Path []string,Attachment ApplicationAttachment,OnTerminal generic.EndScope}`; attachment constants CoordinatorAttachment/RecipientAttachment select distinct attachments, not creature IDs. `CompileApplicationDefinition(*CompileApplicationDefinitionInput{Definition ApplicationDefinition}) (*CompileApplicationDefinitionOutput{Definition *CompiledApplicationDefinition},error)` validates E1 data and complete placement/capability coverage. `BindApplicationDefinition(*BindApplicationDefinitionInput{Definition *CompiledApplicationDefinition,UserID,RecipientID,ApplicationID string,Origin ConditionOrigin}) (*BindDefinitionOutput,error)` returns the existing ApplicationSpec shape. The catalogue is not a prerequisite. Data resolution happens first through E2. Keep the existing general multi-recipient ApplicationSpec API for native spells.

**Behavior:** roles select actual participants only at binding; no lookup/payment/attachment in compilation. CoordinatorAttachment follows the definition's Coordinator role; RecipientAttachment follows RecipientID. Even when both map to the same creature, they remain distinct attachments/addresses. EndApplication maintenance is coordinator-owned; recipient-local EndAttachment is explicit. Do not bucket by creature role alone and accidentally combine self-use state. Preserve the current linked host's nonempty coordinator/recipient requirements rather than invent implicit upkeep. Validate concentration/custody requirements across the whole mapping, not through magic group names. Retire BenefitSets-only/one-entry-catalogue construction as the public composition boundary; do not retain a dual JSON format. Frozen linked bindings still own all resolved declarations/snapshots.

**Tests:** `TestApplicationDefinitionSuite` (direct definition without catalogue, root/group mounts, roles/self-use, missing/overlap/scope/custody refusals, no mutation/spend/attachment); migrate old catalogue/binder tests to new current authoring while retaining behavioral assertions; complete pending/collision/independent-finalization suites unchanged in meaning.

- [ ] Add new application-definition tests; observe old authoring boundary fail.
- [ ] Adapt to the existing Prepare/Create/BindApplications protocol; remove replaced public paths rather than copy the interpreter.
- [ ] Run effects/conditions/character/monstertraits/spells tests and full root gates.
- [ ] Commit/push current code/data schemas and migration/refusal receipt.

**Evidence:** one canonical compiler/resolver plus existing custody, no catalogue requirement on inline content and no implicit host-policy inference.

## Task 6 — E6: Prove native Rage, ring and ward through both authoring surfaces

**Owner/delivers:** root executable consumers/docs. **Prerequisites:** E4/E5.

**Files:** update `conditions/typed_composition_example_test.go`, `typed_composition_test.go`, `authored_example_test.go`, `authored_cookbook_test.go`, `authored_lifecycle_test.go`, `definition_source_test.go`, `definition_world_test.go`, `contribution_order_test.go`, `monstertraits/contribution_order_test.go`; add `definitions/code_config_parity_test.go`, configuration fixtures and owning README/source maps. Reuse real hosts; no fake mechanics.

**Tests:** `TestThreeCompositionConsumersSuite`: native full Raging in Go/config has equal resolved meaning/placement and native outcomes; fixed2 ring uses explicit whole-member derivation, idle target survives10 ends/reload, damage10→12/physical9→4, actual maintainer consequence ends only its grant; resistance-only ward selects one member plus Rage upkeep, adds no damage/advantages, and has the same grace/activity/sparse-round/end order as native maintenance. Mutation of source/derived/catalogue after compile cannot alter other consumers or active saves. Existing oldest/all, loser expiry, sibling-effect and character/monster order proofs remain. No new condition-name handler or arbitrary runtime callback.

- [ ] Write new code/config equivalence and three-consumer tests before adapter fixes; label fixture-selected outcomes honestly.
- [ ] Run executable examples and named suites; resolve only measured gaps in the owning layer.
- [ ] Run full root race/vet/lint and repository make pre-commit, disclose unchanged Core failure separately if present; check docs links/genres.
- [ ] Publish the actual typed field/config examples and reader-compatible provider source for early view and E7/E8.
- [ ] Record exact source heads and evidence; no merge or native inventory claim.

**Evidence:** compiler/runtime parity across all three consumers, actual source ergonomics and no dropped first-cut guarantees.

## Task 7 — E7: API adopts the current native definition graph and projection

**Owner/delivers:** API S21#957. **Prerequisite:** pushed E6 root and neutral provider source; fresh merge-on-top dev catch-up.

**Files:** API go.mod/go.sum; `internal/integration/session/composed_raging_acceptance_test.go`, `linked_maintenance_acceptance_test.go`, `effects_release_adoption_test.go`; `internal/handlers/dnd5e/v1alpha1/character/composed_raging_test.go`, current v2 private-sheet tests as needed; `internal/orchestrators/lobby/start_encounter_session_stack_test.go`; module run record. Inspect all actual ComposedData test constructors before migration.

**Interfaces/behavior:** committed real provider pseudo-versions, then actual tags before merge. Existing handler/session/proto calls; API stores/marshals canonical provider data and passes DescribeCondition detail. No API progress arithmetic or source inference. Preserve all newer encounter/session/SDK/content pins and castle parity.

**Tests:** normal character creation/finalization and native activation → saved current assembly envelope → private notes/detail include provider-owned progress/source → cold-host continuation/cleanup. Existing Bless/Bane pending/save and members_empty live/GetStory proofs pass on the actual graph. Malformed/old active data refuses; do not reset data or hide it with a fallback.

- [ ] Baseline the current dev graph; adopt actual pushed SHAs with go get/tidy and inspect the graph diff.
- [ ] Update only current-format fixtures/assertions, preserving semantic acceptance and public creation paths.
- [ ] Run focused native suites, full make test (including cmd/server) and make ci-check before the working PR push.
- [ ] Open/update API draft, hand exact graph and sample provider envelopes/details to E8.

**Evidence:** actual API graph plus public/private/live/reload outcomes, not a provider fixture presented as transport proof.

## Task 8 — E8: Web current reader, native view and joined readiness

**Owner/delivers:** web S22#962; API/world observation coordination. **Prerequisites:** E7 working graph/projection; own web branch/worktree from dev, not operator checkout.

**Files:** `src/types/conditionData.ts`, `conditionData.test.ts`, `src/components/features/ConditionBadge.tsx`, `ConditionBadge.test.tsx`; relevant status/effect-row tests and owning run record. Keep other unmigrated payload readers unchanged. No generated SDK edit or new proto field expected.

**Behavior/tests:** `current assembly Raging envelope` accepted with opaque parts/bags; old single-machine or malformed current data refused; badge renders supplied “Active for6 turns” notes even when private bag layout has no turns_active field, proving it no longer computes progress; no duplicate progress line or invented benefit. Current source/linked/unmigrated readers remain correct. Native builder-owned stack: normally created barbarian activates Rage, reads provider progress/source, reloads and observes an ordinary end/cleanup; natural observations are separated from module-only sparse-clock and ring fixtures.

- [ ] Write the current-reader/progress rendering tests and observe old reader/formatter RED.
- [ ] Implement opaque current reading and rendering only; run targeted Vitest and one npm run ci-check at the PR boundary.
- [ ] Boot only a builder-owned stack on independent ports with actual source graph, verify served refs, publish early view and do the bounded native observation after the operator look per project workflow.
- [ ] Complete one joined independent review of the wave, resolve findings in-thread and publish exact-head verdicts after all required CI.
- [ ] On separate operator merge go, release inside-out using actual tags, focused re-pin gates/verdicts, archive evidence and remove only clean owned unserved trees. Preserve operator data/stacks and needed provider refs.

**Evidence:** current reader/UI, served graph, honest native receipt, complete gates/review. No automatic production promotion or generic ring inventory claim.

## Requirement coverage

| Requirement | Tasks | Proof |
|---|---|---|
| Root effect/composition without condition/catalogue | E1/E5 | Root/empty/mount contracts and direct application compilation |
| Ordered named structure, cycle/error/ownership | E1/E2 | Qualified leaves, repeated subtree, active cycles, late errors, mutation isolation |
| Typed fields and same generic metadata | E4/E6 | Native fields/full/group/member projections and catalogue equality |
| Exact ref/member selection and explicit variants | E2/E6 | Fixed2 derivation, no implicit live-level substitution, invalid/overlap refusals |
| Native Rage consumes real complete definition | E3/E4/E6 | Sparse/roundless/reset/grace/terminal-order parity; private unrelated recipe removed |
| Ring and reverse-direction ward | E5/E6 | Both code/config paths and real runtime outcomes |
| Placement is explicit, groups are not lifetimes | E1/E5 | Full coverage/scope checks, no magic maintenance label |
| Independent selection/lifetimes/custody | E1/E5/E6 | Retained all/oldest, expiry, sibling effects, pending/finalization/generation tests |
| Frozen current saves and no migration/reset | E4/E5/E7/E8 | Owned parts/state, strict reload/refusal and opaque current reader |
| Preserve source/progress end-to-end | E4/E7/E8 | Provider detail → API notes/v2 detail → badge, native observation |
| Own adoption/review/release discipline | E7/E8 | Actual graph, gates, joined review, operator merge authority and actual tags |

## Provider/consumer seams

| Provider | Consumer | Exact contract / availability | Proof |
|---|---|---|---|
| Existing neutral185aae3c | Root selection | Existing SelectionRule/SelectContributions on real pseudo-version; unchanged | Existing neutral/root tests |
| E1 projection/compiler | E2/E4/E5 | DefinitionData/CompiledDefinition, owned leaves and Place result | Inline/data equivalence, path/ownership/refusal tests |
| E2 catalogue/resolver | E4/E6 | Ref+path+whole-member replacement → canonical DefinitionData, no uses left | Code/config three-consumer and invalid-reference tests |
| E3 maintenance kinds | E4/E5 | Activity/RoundDuration/EndOn with explicit allowed local/application scopes and inspection | Native chronology, priority and runtime host tests |
| E4 named definitions | Native/linked adapters | Actual typed fields and complete projected tree; placement mapping explicit | Same source used by native Rage/ring/ward |
| E4 current state/progress | E7 API | ComposedData assembly snapshot; DescribeCondition detail, same condition ref/origin | Current persistence/private-source/progress tests |
| E7 actual API graph | E8 web | Current opaque envelope plus existing notes/detail | Reader/rendering and served native observation |

## Self-check and engineering choices

- Native terminal collisions are now characterized at13fdf1c9; no new rules choice is hidden in part order.
- PartTemplate's data is the unbound EffectSpec, not another language. Go projections can fail, but canonical compile has one validator and retains no callback.
- The complete named definition has its own package to avoid effects→conditions cycles. Conditions remains the runtime/identity adapter, not the reusable authoring unit.
- Local native Rage changes to an assembly snapshot explicitly. Progress moves through existing provider-owned detail/notes, so no proto expansion or client bag arithmetic is required by the inspected paths.
- A distinct web deliverable was added because the native reader actually depends on the old bag. Earlier “linked parts are opaque” evidence is not reused outside its scope.
- Plan self-check caught a role/attachment conflation: a recipient-maintained or self-use grant can map both attachments to one creature. E5 therefore names attachment placement separately from the coordinator's user/recipient role; no participant ID is used as a grouping shortcut.
- Root tasks share one writer in order. Actual provider publication, native source look, consumer adoption, review and final release are all represented. The old first-cut completion markers do not skip these tasks.
- No unresolved required product/rules decision is known. If implementation would change a measured native rule, lifecycle authority or persistence policy beyond this mapping, stop that affected task with evidence rather than weakening tests.
