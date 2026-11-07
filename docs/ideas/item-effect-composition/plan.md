# Effects extraction — first implementation cut

**Status:** checked for execution. H1 is settled by KirkDiggler on 2026-11-06: `mechanics/effects/`, with machine in the same module. The naming decision is `effects.Compose(effects.ComposeInput{...})` and `machine.New(machine.Config{...})`. No merge/release is authorized by executing this plan.

**Goal:** extract the existing generic effects composer and its machine as one independently versioned module, and prove the Second Wind/Rage/loot examples consume it without behavior changes.

**Authority:** [ADR-0048](../../adr/0048-equipment-granted-abilities.md), the [walkthrough](README.md), [effects story #703](https://github.com/KirkDiggler/rpg-project/issues/703) and [slice #705](https://github.com/KirkDiggler/rpg-project/issues/705). The operator selected items-first extraction, with condition migration later, and approved the walkthrough.

**Finish line:** a checked provider PR plus migrated example-consumer PRs, evidence against their actual pins, and an accurate implementation record. Provider merge/tag and consumer final re-pins follow explicit merge authority; this plan does not authorize automatic merges. Examples remain examples; no production item feature is claimed.

## Scope and settled choices

**H1 — settled home:** `mechanics/effects/`, module
`github.com/KirkDiggler/rpg-toolkit/mechanics/effects`; `machine/` remains a package
inside that release unit. The mechanics layer groups generic composition with
other rulebook-neutral mechanisms such as resources. It does not contain D&D rules.

The first plan proposed top-level `effects/` for its standalone name. The operator
preferred the mechanics home after comparing the layer fit; its old module history
is retained, not its retired implementation. `core/effect` is not selected because
core supplies types/contracts, not this event-driven runtime. No third/shared
machine is extracted.

The public composition operation is `Compose(ComposeInput)`, with the same named
fields and `*Composed, error` result. The machine keeps `New(Config)`. These naming
changes are explicit; runtime/data/lifecycle semantics do not change.

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
  and leaves only `mechanics/effects/go.mod`, not a second compatibility module.
- The tagger excludes `examples/*`. The chosen path has tags through
  `mechanics/effects/v0.2.1`; that tag is an ancestor of inspected main. This is a
  deliberate replacement API on the next pre-v1 version, not a fresh v0.1.0.
  Observe CI's actual tag after an authorized merge; never mint or reuse one.
- Current imported types, field/default semantics, validation, source/holder
  identity, lifecycle ordering and serialized blobs are preserved.
- Work in the named isolated worktrees. Consumers pin pushed provider commits
  during development; no `replace`, `go.work`, copied runtime or version invention.

## Task 1 — extract the provider without redesign

**Owner:** toolkit, the single surviving `mechanics/effects` Go module; repo-level docs may accompany it.

**Files:** move `examples/effects/` to `mechanics/effects/`, preserving its tests and prior
run record. Update `mechanics/effects/go.mod`, actual Go imports, composer/machine import
seam allow-lists and negative import cases. Update live documentation links in
`knowledge/map.md`, `docs/architecture/components/mechanics.md`,
`docs/ideas/item-effect-composition/README.md`,
`docs/ideas/machine-foundation/README.md`, `docs/adr/0048-equipment-granted-abilities.md`,
`docs/status.md` and `docs/quality.md`. Historical pinned run records are not
rewritten; `mind/machine/doc.go`'s historical origin statement is not a dependency.
Record the items-first promotion amendment in this slice's run record for ingest,
without manually rewriting the historical knowledge decisions. Add that run record.

**Contract:** consumers import the new module path and its `machine` package.
`effects.ComposeInput` (the renamed construction Config), `Composed`, `OutputVocabulary`, `KindSpec`, machine recipe,
program, guard and blob contracts are unchanged. There is no forwarding package
at the old import path; previously pinned old commits remain valid history.

**Proof:** all current provider tests, especially `TestComposeRefuses`,
`TestMachineLifecycle`, `TestOutputsAttachedOnlyWhileInState`,
`TestEndPublishesThenDetaches`, `TestRageWalk`, `TestRecipeRoundTrip`,
`TestBlobIsFlat` and both `TestImportsStayInsideTheSeam` tests. The latter must
allow the new child path and still refuse rulebook imports / machine importing
its composer. Preserve byte-equal restore and publish-before-detach assertions.

- [x] Baseline existing module race tests and example.
- [x] Move the module; migrate actual imports and live links, not historical prose wholesale.
- [x] Compare Go source before/after with import paths and the explicit Compose/ComposeInput rename normalized: all 25 ASTs match.
- [x] In `mechanics/effects`: `go fmt ./...`, `go mod tidy`, `go test -count=1 -race ./...`, `go test -run Example -v ./machine`, `go vet ./...`, `golangci-lint run ./...`, benchmark smoke with `-benchtime=1x`.
- [x] Verify no old example-module imports remain in current provider Go files/go.mod, and no second go.mod or compatibility alias remains at the old location.
- [x] Stage the moved module, then run map/digest/link/genre/whitespace checks. Genre retains 44 pre-existing advisory findings; see [evidence](implementation.md).
- [x] Push the provider and open a draft; consumer source checkpoint is `e15c6e54` in #2291.

## Task 2 — migrate both item consumers against the pushed provider

**Owner:** toolkit `examples/items`, kept separate from the provider PR. Update
#2273 first, then merge its branch into the S2 worktree and update #2287. No provider
merge is needed to begin this integration.

**Files:** `examples/items/item.go`, `seam_test.go`, `go.mod`, `go.sum`, README and
run record in S1; S2 additionally `rage_recipe_test.go`, `rage_vocabulary_test.go`
and its relevant example docs/run record. Update imports to `mechanics/effects`
and `mechanics/effects/machine`, including the import-boundary test, and migrate
composition calls to `effects.Compose(effects.ComposeInput{...})`. Keep rulebook dependencies
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

- [x] Use `go get github.com/KirkDiggler/rpg-toolkit/mechanics/effects@<pushed-provider-sha>` and `go mod tidy`; remove the unused old module requirement through Go tooling.
- [x] Run full-module race tests, both examples, vet and lint in the relevant worktree. Read actual output; an import-only compile is not consumer evidence.
- [x] Confirm module/import inventory contains no `examples/effects` dependency.
- [x] Assess the conditional lifecycle/identity mutant: not applicable to unchanged runtime ASTs and untouched behavior; retain the existing meaningful tests.
- [x] Push each consumer checkpoint and keep it draft while publication/re-pin requirements remain outstanding. S1 `7d7ef342`, S2 `7dadbb64`; [evidence](implementation.md).

## Task 3 — release/adoption evidence, only with the required authority

**Order:** provider review/gates → authorized provider merge → observe CI's real
`mechanics/effects/v…` tag → re-pin item consumers → focused verification / applicable review.
Retarget S2 to main before its base branch is merged/deleted; its current stacked
base does not trigger automatic PR CI. Do not claim no checks means green.

- [x] Record the head-specific provider verdict and CI; operator cleared continuing after the retraction. Provider merged as `c7cf2461` after independent review at `0477a36d`.
- [x] Observe CI's actual `mechanics/effects/v0.3.0` tag and re-pin both consumers with `go get`; repeat race/example/vet/lint checks against the tag. [Release evidence](implementation.md#published-provider-and-released-consumer-adoption).
- [x] Write `implementation.md` from observed source/consumer checkpoints, contracts, results and deviations; release remains outstanding. Explicitly name still-unrun production boundaries.
- [x] Update #705 / linked consumer issues and source maps with source integration evidence, leaving release/adoption outstanding. Do not close broader condition/item stories on the first provider merge.

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
| Existing composer/machine | Mechanics module home | Same data/lifecycle, named Compose input and new import prefix | H1 settled; code exists | Provider suite and normalized diff |
| Pushed effects extraction head | Item S1 | ComposeInput, Composed and effect vocabulary | Provider `e15c6e54` consumed | Real Go pseudo-version + item suite |
| Updated S1 + effects head | Rage/loot S2 | Same typed prepared effects and machine recipe | Merge S1 branch into S2; no rebase | Rage/loot suite + import inventory |
| CI-published effects tag | Final consumers | Same tested provider content on released pin | Authorized merge/tag before final adoption | Tag inspection and re-pin verification |

Planning check: requirements and interfaces have owners and named evidence.
Server API/wire, storage and rulebook migration are deliberately excluded, not
silently deferred required work. **H1 is settled; the scoped plan can execute.**
New ownership/lifecycle changes still return to design rather than being hidden in the move.
