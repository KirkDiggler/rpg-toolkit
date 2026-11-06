# Effects extraction — first implementation cut

**Status:** structurally checked against the inspected provider/consumers; **not executable until H1 (module home) is settled**. Paths below use the recommended `effects/` home. This is the one architectural choice requested, not another approval of the planning process.

**Goal:** extract the existing generic effects composer and its machine as one independently versioned module, and prove the Second Wind/Rage/loot examples consume it without behavior changes.

**Authority:** [ADR-0048](../../adr/0048-equipment-granted-abilities.md), the [walkthrough](README.md), [effects story #703](https://github.com/KirkDiggler/rpg-project/issues/703) and [slice #705](https://github.com/KirkDiggler/rpg-project/issues/705). The operator selected items-first extraction, with condition migration later, and approved the walkthrough.

**Finish line:** a checked provider PR plus migrated example-consumer PRs, evidence against their actual pins, and an accurate implementation record. Provider merge/tag and consumer final re-pins follow explicit merge authority; this plan does not authorize automatic merges. Examples remain examples; no production item feature is claimed.

## Scope and unresolved choice

**H1 — recommended home:** `effects/`, module
`github.com/KirkDiggler/rpg-toolkit/effects`, with `effects/machine` a package inside
that module. Keep the present `New(Config)` surfaces for this cut. The components
and release unit already exist; this is promotion, not another machine design.

Alternative: `mechanics/effects` expresses the same generic layer but restores the
path of the retired implementation. Both homes can respect dependency direction;
top-level `effects` names this composer's responsibility directly. `core/effect`
is not the recommendation: core supplies types/contracts, while this runtime
owns event-driven composition. A third shared machine module is outside scope.

The existing record deliberately leaves the home until extraction. H1 must be
recorded on #705 before source files or consumer imports are moved.

**Excluded:** rulebook effect-kind relocation, shared production Raging factory,
legacy condition/class migration, designer schema/editor, inventory/storage APIs,
new item/activation protocols, `mind/machine` changes and unrelated legacy cleanup.
Those remain later contracts/slices, not prerequisites smuggled into this move.

## Inspected baseline and constraints

- Provider: toolkit main `aca78cdf`, `examples/effects` at Go 1.27.0; runtime
  requires core v0.12.0 and events v0.6.3. Dependencies remain unchanged initially.
- S1 consumer #2273: `probe/862-items` at `c6a84277`.
- S2 consumer #2287: `probe/items-rage-loot` at `6aeafe68`, stacked on #2273.
- No existing top-level `effects/` or `mechanics/effects/` directory is present in
  the inspected main tree. The retired rulebook representation is not adopted.
- CI discovers surviving modules from changed Go paths; a removed old module
  contributes no build directory. The provider PR moves one logical release unit
  and leaves only `effects/go.mod`, not a second compatibility module.
- The tagger excludes `examples/*`, but automatically initializes a new ordinary
  module at v0.1.0. Observe the actual tag after an authorized merge; never mint it.
- Current imported types, field/default semantics, validation, source/holder
  identity, lifecycle ordering and serialized blobs are preserved.
- Work in the named isolated worktrees. Consumers pin pushed provider commits
  during development; no `replace`, `go.work`, copied runtime or version invention.

## Task 1 — extract the provider without redesign

**Owner:** toolkit, the single surviving `effects` Go module; repo-level docs may accompany it.

**Files:** move `examples/effects/` to `effects/`, preserving its tests and prior
run record. Update `effects/go.mod`, actual Go imports, composer/machine import
seam allow-lists and negative import cases. Update live documentation links in
`knowledge/map.md`, `docs/architecture/components/mechanics.md`,
`docs/ideas/item-effect-composition/README.md`,
`docs/ideas/machine-foundation/README.md`, `docs/adr/0048-equipment-granted-abilities.md`,
`docs/status.md` and `docs/quality.md`. Historical pinned run records are not
rewritten; `mind/machine/doc.go`'s historical origin statement is not a dependency.
Record the items-first promotion amendment in the owning knowledge record without
erasing the earlier decision. Add this slice's run record.

**Contract:** consumers import the new module path and its `machine` package.
`effects.Config`, `Composed`, `OutputVocabulary`, `KindSpec`, machine recipe,
program, guard and blob contracts are unchanged. There is no forwarding package
at the old import path; previously pinned old commits remain valid history.

**Proof:** all current provider tests, especially `TestComposeRefuses`,
`TestMachineLifecycle`, `TestOutputsAttachedOnlyWhileInState`,
`TestEndPublishesThenDetaches`, `TestRageWalk`, `TestRecipeRoundTrip`,
`TestBlobIsFlat` and both `TestImportsStayInsideTheSeam` tests. The latter must
allow the new child path and still refuse rulebook imports / machine importing
its composer. Preserve byte-equal restore and publish-before-detach assertions.

- [ ] Baseline existing module race tests and example.
- [ ] Move the module; migrate actual imports and live links, not historical prose wholesale.
- [ ] Compare Go source before/after with import-path changes normalized; any behavior delta requires explanation, not silent inclusion.
- [ ] In `effects`: `go fmt ./...`, `go mod tidy`, `go test -count=1 -race ./...`, `go test -run Example -v ./machine`, `go vet ./...`, `golangci-lint run ./...`, benchmark smoke with `-benchtime=1x`.
- [ ] Verify no old example-module imports remain in current provider Go files/go.mod, and no second go.mod or compatibility alias remains at the old location.
- [ ] Stage the moved module, then run `scripts/check-knowledge-map.sh`, `scripts/check-decisions.sh`, doc-link/genre checks and `git diff --cached --check`.
- [ ] Push the provider and open a draft; report its real head for consumer adoption.

## Task 2 — migrate both item consumers against the pushed provider

**Owner:** toolkit `examples/items`, kept separate from the provider PR. Update
#2273 first, then merge its branch into the S2 worktree and update #2287. No provider
merge is needed to begin this integration.

**Files:** `examples/items/item.go`, `seam_test.go`, `go.mod`, `go.sum`, README and
run record in S1; S2 additionally `rage_recipe_test.go`, `rage_vocabulary_test.go`
and its relevant example docs/run record. Update imports to `effects` and
`effects/machine`, including the import-boundary test. Keep rulebook dependencies
in the consumer tests, not the generic item package.

**Contract:** the item composer and Rage preparation resolve exactly one effects
module path. No old/new mixed runtime remains in `go list -m all`. Configuration,
item state and prepared-result behavior do not change as part of re-pinning.

**Proof:** S1 `TestItemCompositionSeams` and `Example_itemComposition`; S2
`TestRageGrantingItem`, `TestBuiltItemIsRetrievedBeforeAnEncounterAndLootPreservesIt`,
`TestMissingOrNullStoredInstanceDoesNotCreateFreshLoot`,
`TestHostCommitsItemStateOnlyAfterEffectDeliverySucceeds`, `Example_rageLoot` and
the generic import-boundary test. Preserve 7/9 healing calculations, Rage's four
benefits/lifetimes, item provenance, spent loot and explicit commit-on-success limits.

- [ ] Use `go get github.com/KirkDiggler/rpg-toolkit/effects@<pushed-provider-sha>` and `go mod tidy`; remove the unused old module requirement through Go tooling.
- [ ] Run full-module race tests, both examples, vet and lint in the relevant worktree. Read actual output; an import-only compile is not consumer evidence.
- [ ] Confirm module/import inventory contains no `examples/effects` dependency.
- [ ] Re-run one lifecycle/identity mutant behind the existing consumers if the migration touches that seam; do not invent behavior changes merely for a rename.
- [ ] Push each consumer checkpoint and keep it draft while publication/re-pin requirements remain outstanding.

## Task 3 — release/adoption evidence, only with the required authority

**Order:** provider review/gates → authorized provider merge → observe CI's real
`effects/v…` tag → re-pin item consumers → focused verification / applicable review.
Retarget S2 to main before its base branch is merged/deleted; its current stacked
base does not trigger automatic PR CI. Do not claim no checks means green.

- [ ] Record the head-specific provider verdict and CI; obtain merge authority rather than inferring it from approval of the walkthrough.
- [ ] After an authorized merge, observe the actual provider tag and re-pin consumers with `go get`; repeat consumer checks against the tag.
- [ ] Write `implementation.md` beside this plan from observed module paths, versions, contracts, results and deviations. Explicitly name still-unrun production boundaries.
- [ ] Update #705 / linked consumer issues and source maps. Do not close broader condition/item stories on the first provider merge.

## Requirement coverage

| Requirement | Task | Concrete proof |
|---|---|---|
| One generic effects release unit; machine stays inside | 1 | Single surviving go.mod; module map; provider import seams |
| No rulebook/gameplay or serialization change | 1 | Normalized source comparison; full provider suite; byte-equal reload / removal ordering |
| Items-first consumer evidence | 2 | Actual pinned S1/S2 suites and runnable examples |
| Encounter-independent item state is retained | 2 | Existing outside-encounter retrieval, missing-data and spent-loot assertions |
| No dual old/new effects dependency | 1–2 | File/import inventory and `go list -m all` |
| Decision → plan → implementation trace | 1–3 | #705, linked ADR/walkthrough, PR/run evidence, observed implementation record |
| Publication is observed, not invented | 3 | Actual merge SHA/tag and consumer re-pin receipts |

## Provider / consumer seam check

| Provider | Consumer | Produced / consumed contract | Availability | Proof |
|---|---|---|---|---|
| Existing composer/machine | New module home | Same API/data/lifecycle, new import prefix | H1 pending; code exists | Provider suite and normalized diff |
| Pushed effects extraction head | Item S1 | Config, Composed and effect vocabulary | Task 1 before Task 2 | Real Go pseudo-version + item suite |
| Updated S1 + effects head | Rage/loot S2 | Same typed prepared effects and machine recipe | Merge S1 branch into S2; no rebase | Rage/loot suite + import inventory |
| CI-published effects tag | Final consumers | Same tested provider content on released pin | Authorized merge/tag before final adoption | Tag inspection and re-pin verification |

Planning check: requirements and interfaces have owners and named evidence.
Runtime API/wire, storage and rulebook migration are deliberately excluded, not
silently deferred required work. **H1 remains the explicit execution blocker.**
After it is resolved, the faithful plan needs no separate procedural approval.
