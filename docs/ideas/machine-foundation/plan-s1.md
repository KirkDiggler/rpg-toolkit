# NPC behavior S1 — retained composition implementation plan

Goal: real behavior instances retain their own progress while an authored higher-priority investigation owns the action-selection opportunity, then patrol resumes its pending work. Use the real machine and weighted picker with exact scripted rolls.

Authority: [accepted ADR-0049](https://github.com/KirkDiggler/rpg-toolkit/blob/81b5ed0f4a475fa95b6c6fce768ad4a5d9ed6d52/docs/adr/0049-behavior-is-a-composer.md), [operator go-ahead and bounded contract](https://github.com/KirkDiggler/rpg-project/issues/866#issuecomment-6650897115), [S1](https://github.com/KirkDiggler/rpg-project/issues/866).

Owner: rpg-toolkit `mind/behavior`, monster-ai Team. Direct execution in `.worktrees/machine-foundation`, branch `feat/866-behavior-composer`, based on main `3182088c`. Other Go modules are read-only. `mind/machine/v0.1.0` is already released; adding it raises the selected core dependency from v0.11.0 to its required v0.12.1. No provider merge, pseudo-version, replace or go.work is needed.

Finish line: implemented module APIs, real-component scenario and refusal/reload tests, full module race/lint gates, one independent review round, documentation/journal and a module-bounded PR. No merge authorization. This is a foundation/component checkpoint, not live encounter/studio adoption.

## Contract and boundaries

- **Source/Params:** materialized `InstanceSpec{ID, Recipe machine.Recipe, Table}` in authored priority order, plus the supplied vocabulary. A content producer binds its own parameters into these ordinary data values; no runtime template engine is introduced.
- **Runtime:** `Composer` owns one private `machine.Blob` and compiled program per instance. Definitions/tables are copied at ingress. No actor registry, bus, geometry, rulebook, persistence I/O or goroutines.
- **Data:** `ComposerData` contains ordered `InstanceData{ID, Definition string, Blob machine.Blob}`. Definition is a deterministic digest of the materialized specification; it identifies which configuration the continuation belongs to, not a second mutable configuration. Reload receives the original config, refuses different instance order/configuration, and restores only validated blobs. The caller retains the immutable started configuration.
- **Projection:** results carry committed machine steps, selected instance/pick, and deferred instance IDs. Returned selections and snapshots do not alias owned state/configuration.
- `Observe` delivers a data-only input to every instance by default; an explicit instance ID targets completion/feedback to its owning attachment. If any firing transition requests a Pick, refuse with `ErrSelectionRequired`; do not silently execute or drop it. No state changes on error.
- `Decide` previews every instance on detached blobs. Commit data-only transitions; among transitions requesting a Pick, select the first instance in authored order and preserve the losing instances' original blobs. Invoke the existing weighted picker only for that selected instance. A selected empty/ineligible table produces its existing hold, not a fallback into a lower-priority role; because no row was selected, its action-producing machine step is not committed. An explicitly selected Hold row is eligible work and does commit. Return values; never execute a selected word.
- A transition may request at most one table pick in this first composer. Refuse a definition that requests more, rather than silently losing work. No general resource-lane scheduler is claimed.
- A failed decision leaves composer-owned state unchanged. Consumed dice or side effects in caller callbacks are not rolled back. Readers should be pure; no general transaction/compensation system is introduced.
- Time remains caller-supplied. Retaining a blob does not promise a virtual paused clock for every timer. The patrol fixture advances its pending leg only on an arrival input; this slice does not change current encounter patrol/dwell behavior.
- Parameter metadata is a producer-ingress tool: bool/int/string kind, requiredness, integer min/max, string choices, description/unit. Values are typed Go scalars. Schema metadata round-trips as JSON; no untyped JSON-number coercion or new editor/dungeonspec format is claimed. Producers use validated values to materialize recipes/tables. No implicit defaults or separate quality-profile framework.

The earlier spatial resume choice is not a global kernel policy. The authored fixture continues a pending table entry/waypoint; actual physical rejoining in the live encounter remains outside this module checkpoint. No competing continuation owner is placed in test helpers.

## Sequence

1. Parameter metadata validation and published machine dependency.
2. Composer construction, observation, ordered decision, snapshot/load.
3. Real patrol/investigation fixture, docs, gates and review.

All tasks change the same owning module, in order. Repository-level plan/run documentation accompanies it. Existing weighted-picker APIs remain unchanged. No generated bindings, host wiring, deployed content or consumer pin bumps occur here; a future adoption slice must prove those boundaries.

## Review focus

- Deferred candidates must not mutate retained progress; data-only updates must not be lost.
- A failed roll or observation requesting action must not partially commit state.
- Snapshot and returned Answer pointers/maps must not alias internal data.
- Changed definition/order, unknown state/bag keys and wrong scalar kinds must be refused on load.
- No hidden roll, extra lower-priority pick, implicit parameter default or silent unknown parameter.

## Task 1: Mechanical parameter metadata

**Delivers:** parameters describe mechanical suitability without a quality-profile subsystem.
**Files:** create `mind/behavior/parameters.go`, `parameters_test.go`; update only this module's `go.mod`/`go.sum` using the published machine dependency.
**Interfaces:**

- `ParameterSpec{Name, Description, Unit string; Kind machine.ScalarKind; Required bool; Min, Max *int; Choices []string}`.
- `CheckParametersInput{Schema []ParameterSpec; Values map[string]any}` → `CheckParametersOutput{Values map[string]any}` through `CheckParameters(*CheckParametersInput) (*CheckParametersOutput, error)`.
- Reuse `ErrNilInput`; add `ErrBadParameter`. Named schema/value errors wrap it. Return a detached scalar map, preserve false/zero/empty supplied values, and leave absent optional values absent.

**Behavior:** refuse duplicate/empty names, unsupported kinds, bounds on non-ints, choices on non-strings, reversed bounds, duplicate choices, unknown supplied keys, absent required values, wrong types and violated bounds/choices. Do not sample, coerce or invent defaults.

**Tests:** `ParameterSuite` covers accepted inclusive bounds and false/zero values; missing/unknown/wrong-kind values; schema refusals; string choices; detached result; JSON metadata round trip. The fixture later consumes the checked integer `investigation_rounds` in a real recipe's rounds guard.

- [x] Add named tests and run the focused suite red before implementation (new API absent).
- [x] Implement the metadata and checks.
- [x] Run focused tests and complete module race/lint checks, then commit.

**Verification:** from `mind/behavior`: `go test -count=1 -run '^TestParameterSuite$' ./...`; then `go test -race -count=1 ./...` and `golangci-lint run ./...`. Expect the new tests to fail before the API exists and pass after it; existing behavior suites remain green. Dependency pin changes are only machine v0.1.0 and its required selected core v0.12.1.

## Task 2: Stateful composition over existing primitives

**Delivers:** an actual continuation owner and exclusive selection, not test-only orchestration.
**Files:** create `mind/behavior/composer.go`, `composer_data.go`, `composer_test.go`; no changes to machine or encounter source.
**Interfaces:**

- `InstanceSpec{ID string; Recipe machine.Recipe; Table Table}`.
- `ComposerConfig{Instances []InstanceSpec; Vocabulary machine.Vocabulary}`; `NewComposer(*ComposerConfig) (*Composer, error)`.
- `ObserveInput{Instance string; On machine.Beat; Event, Env any; Now uint64}`; `ObserveOutput{Steps []InstanceStep}`; `InstanceStep{ID string; Step machine.Stepped}`; `Composer.Observe(*ObserveInput) (*ObserveOutput, error)`.
- `DecideInput{On machine.Beat; Event, Env any; Now uint64; Facts Facts; Temper Temper; Die Die}`; `DecideOutput{Instance string; Pick *PickOutput; Steps []InstanceStep; Deferred []string}`; `Composer.Decide(context.Context, *DecideInput) (*DecideOutput, error)`.
- `ComposerData{Instances []InstanceData}`; `InstanceData{ID, Definition string; Blob machine.Blob}`; `ToData() ComposerData`; `LoadComposerInput{Config ComposerConfig; Data ComposerData}`; `LoadComposer(*LoadComposerInput) (*Composer, error)`.
- Errors: `ErrBadComposition`, `ErrBadComposerData`, `ErrSelectionRequired`, `ErrUnknownInstance`; reuse `ErrNilInput` and the existing picker/provider sentinels in wrapped errors.

**Construction:** copy materialized specs via deterministic JSON; validate IDs/order, recipe compilation, declared fact readers, referenced table keys, at most one pick per transition, positive row weights and existing row validation plus one outcome word. Empty named tables remain legal holds. Compute a stable per-spec digest. Empty composition is legal; unknown input beats still refuse.

**Observation/decision:** work on cloned blobs until successful. On Decide, re-key the selected table through existing `KeyTime` semantics so no eligible action is an explained hold; restore its authored key on the result. Copy selected rows before passing them to caller fact callbacks, and deep-copy the selected answer before returning. A callback must not receive a writer to owned configuration. A selection error commits no blobs. A no-candidate hold preserves the selected instance's prior blob while committing data-only steps; an explicit Hold row commits normally. Observations that produce a Pick refuse without changes. Emit only committed step results; name losing action candidates as deferred. No state observer/publisher callbacks.

**Data:** clone bags and optional stamps on export/import. Load must match count, ID/order and definition digest, require a known nonempty state, and validate declared bag keys/scalar kinds. It does not call Step to validate a blob or look up edited content. It does not persist compiled programs/indexes or duplicate the materialized specs in the progress data.

**Tests:** `ComposerSuite` covers priority independent of weight, deferred-state preservation, data-only updates, explicit observation refusal, roll-error atomicity, unknown/nil inputs, constructor/table refusals, independent instances/copy-out, JSON reload, changed-config/order refusal and malformed blobs. Expected values are hand-checked. Use a strict scripted die that checks requested size and exhaustion; it is the only selection double.

- [x] Add focused tests, run red, then implement the real composition/data owners.
- [x] Run focused and full module gates and commit a working checkpoint.
- [x] Open the implementing draft on its first working push, linked to #866/#859 (toolkit#2277).

**Verification:** from `mind/behavior`: `go test -count=1 -run '^TestComposerSuite$' ./...`; `go test -race -count=1 ./...`; `golangci-lint run ./...`. Expect literal winning picks and unchanged deferred snapshots; invalid inputs return named errors, not panics or nil/nil.

## Task 3: Joined patrol/investigation proof and owning documentation

**Delivers:** the selected first capability through real components and a truthful handoff.
**Files:** create `mind/behavior/patrol_composition_test.go`, `composer_bench_test.go`; update `mind/behavior/doc.go`, `README.md`, and the root `knowledge/map.md` dependency edge; keep the PR's run record at root `knowledge/runs/2026-11-05-behavior-composer-s1.md`, the level spanning code plus map/plan. Update the separate project journal with observed evidence, not a copied backlog.

**Fixture:** materialize an investigation recipe using checked `investigation_rounds` (first fixture 2, metadata range 1–4; fixture choices, not game defaults). Priority order is investigation then patrol. Patrol advances from its first leg to the next only on an arrival input. A matching clang activates investigation; a later round ends it. Two equally weighted investigation movement answers produce different real picks at faces 1 and 101 of d200. The next patrol choice still names its unfinished waypoint, not a reset/recomputed route. The composer never resolves geometry or executes movement.

**Tests:** `PatrolCompositionSuite` proves initial patrol → arrival/progress → clang → exclusive investigation → completion → same pending patrol leg. Exercise both hand-chosen investigation roll branches, a reload during investigation, irrelevant/unknown clang, no movement budget (hold/no roll), and metadata changing the authored investigation duration. Assert exact die sizes/calls and returned destinations, not a mocked picker. Snapshot/load uses the same original materialized config. No registry/template callback performs hidden coordination.

- [x] Add the scenario and failure/boundary cases; verify they distinguish the missing continuation/selection behavior.
- [x] Update godoc/README with the new responsibilities, Observe/Decide distinction, configuration/progress separation, and exclusions.
- [x] Run full module gates and a targeted mutation control proving deferred progress cannot advance.
- [ ] Publish the run/journal receipts, obtain one independent whole-PR review, address required findings, and report actual PR readiness without merging.

**Verification:** `go test -count=1 -run '^TestPatrolCompositionSuite$' -v ./...`, complete module race suite and lint; `git diff --check`; `PRE_COMMIT_CACHE=false .githooks/pre-commit` from the worktree root with this module staged. No root make target that writes unrelated modules or invokes go.work. Baseline effects/machine is read-only. No browser walk is claimed: this slice adds no live adapter or dungeonspec field; the studio round-trip requirement applies when that authoring field is introduced.

## Requirement coverage

| Requirement | Tasks | Concrete proof |
|---|---|---|
| Reusable mechanical parameter description | 1, 3 | bounds/choices/refusal suite; real duration binding |
| Investigation owns the selection opportunity | 2, 3 | first requesting instance wins; d200 only for investigation |
| Patrol retains pending progress | 2, 3 | deferred blob unchanged; resumed answer targets pending waypoint |
| Real controlled rolls | 2, 3 | strict size/face script and real `Pick` result |
| Unknown/irrelevant input does not activate | 2, 3 | no transition/no inappropriate selection |
| Budget/empty-action semantics remain honest | 2, 3 | real hold, zero unexpected rolls, no lower-role fallback |
| Reload does not reset or resample | 2, 3 | JSON snapshot restored with identity check and same pending work |
| No fake orchestration in tests | 2, 3 | production Composer owns all instance mutation and arbitration |
| Truthful delivery | 3 | module gates, review, journal and explicit live-integration exclusions |

## Provider/consumer seams

| Provider | Consumer | Produced → consumed contract | Availability | Proof |
|---|---|---|---|---|
| `mind/machine/v0.1.0` | Composer | Recipe/Vocabulary → Program; Blob + input → Stepped | Published tag; core v0.12.1 selected by Go | real Compile/Step, validated private blobs |
| Existing behavior picker | Composer.Decide | Table/Facts/Temper/Die → PickOutput | Existing same module, unchanged | real weighted candidates/rolls/hold output |
| Parameter checker | Fixture/content producer | schema + typed values → checked detached values | Task 1 before Task 3 | validated duration appears in the executed guard |
| Composer state/data | Reloading caller | minimal progress + definition identity, original config → restored Composer | Task 2 | strict load and resumed scenario |
| Composer output | Future live execution adapter | chosen row and explanations, not executed movement | Not changed in this checkpoint | explicitly not claimed as keep/browser delivery |

Plan check: all current in-scope requirements have named proof. Binding/template authoring, live encounter routing, spatial recipient indexing, generalized memory policies, multiple action lanes, virtual paused clocks, asynchronous recovery and effects adoption are not implemented by this checkpoint. No other module is silently used as a scratch integration layer.

Integration findings: completion must be addressable by the selected instance ID, otherwise two attachments of one definition can both advance. `ObserveInput.Instance` now supplies that existing-owner address; empty retains broadcast semantics. A regression also demonstrated `Facts.Threat` could mutate a stored selector through the picker input; copying selected rows before that boundary closes the alias. Both refinements preserve the agreed ownership rather than adding a new coordinator or memory authority.

— monster-ai agent, on behalf of KirkDiggler
