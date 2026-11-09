# Generic effects extraction — implementation evidence

**Status (2026-11-07):** provider and both item consumers merged. The provider
is released as **mechanics/effects v0.3.0**; both consumers pin that tag and
passed repeated local gates and final-head CI. Merge receipts are below and on #705.
This is the observed first-cut result, not proof of production condition/inventory adoption. [Plan](plan.md), [slice #705](https://github.com/KirkDiggler/rpg-project/issues/705).

## Observed checkpoints

| Component | PR / source checkpoint | Observed dependency |
|---|---|---|
| Provider | [#2291](https://github.com/KirkDiggler/rpg-toolkit/pull/2291), `e15c6e54ef812d5ae04326e51b6260123cd2020c` | `mechanics/effects`, including its `machine` package |
| Second Wind item S1 | [#2273](https://github.com/KirkDiggler/rpg-toolkit/pull/2273), `7d7ef342` | `mechanics/effects v0.2.2-0.20261106234706-e15c6e54ef81` |
| Rage / stored loot S2 | [#2287](https://github.com/KirkDiggler/rpg-toolkit/pull/2287), `7dadbb6455cc6e81363560712c4ba0f88584677b` | Same pushed-provider pseudo-version; S1 merged into S2 without rebase |

The provider baseline is main `aca78cdf`. The old `examples/effects` directory
is removed in the provider branch; there is one surviving module, no forwarding
package, copied runtime, `replace`, or `go.work`.

## Published provider and released consumer adoption

- Provider #2291 merged as `c7cf24617e7517ad24182afdac86df6650e43cf4` after the
  [independent MERGE-READY review](https://github.com/KirkDiggler/rpg-toolkit/pull/2291#pullrequestreview-5808914357)
  and green CI at `0477a36d`.
- [CI tagging run 42319670436](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42319670436)
  published `mechanics/effects/v0.3.0`; the tag dereferences to that merge commit.
- Both consumers now pin `v0.3.0`, with fresh full-module race tests, examples,
  vet and lint passing. S1 adoption commit is `161e24f2`; S2 merged that branch.
  No runtime Go code changed during released-tag adoption. Final pin audit also
  replaced inherited main-commit pseudo-versions with code-equivalent existing
  tags: resources v0.3.2 and D&D v0.213.5 (only historical Markdown differs for
  D&D, excluding its independently versioned child modules). Both full local
  gates pass again; no direct pseudo-version remains.
- Normal `go list -m -versions` now lists only v0.3.0. Explicit `-retracted`
  queries for v0.1.0, v0.2.0 and v0.2.1 return the published rationale, preserving
  access to old pins. The first probe immediately after tagging saw stale proxy
  metadata; direct lookup and subsequent normal proxy queries confirm propagation.
- S2 was retargeted to main before parent merge/deletion. S1 #2273 merged as
  `b76a1f6dd65954055bd9cabb80110552d4811889`; S2 #2287 merged as
  `3b82d2e4acf876bad21fd808ac11db662f60ddcc`. Examples are never tagged.
- One proportionate consumer review inspected S1 and then S2's additions, without
  duplicating the shared code review. Three Minor documentation findings were
  addressed and all threads resolved; no runtime blockers remained. Focused final
  verdicts: [S1 at 5c25762c](https://github.com/KirkDiggler/rpg-toolkit/pull/2273#pullrequestreview-5809002802)
  and [S2 at 98667400](https://github.com/KirkDiggler/rpg-toolkit/pull/2287#pullrequestreview-5809012790).
  Final-head CI passed: [S1](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42320776906)
  and [S2](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42321053762).
- S1's main catch-up conflicted only in the module map; S2's post-squash catch-up
  conflicted only on its README introduction. Both retained the intended content;
  no runtime Go or dependency changes were introduced by those resolutions.

## Contracts retained and spelling changed

The home is `github.com/KirkDiggler/rpg-toolkit/mechanics/effects`.
`effects.Compose(effects.ComposeInput{...})` assembles a fresh runtime from the
recipe, supplied vocabularies and instance inputs. It returns the same
`*Composed, error`; it does not attach to the bus or spend an item charge.
`machine.New(machine.Config{...})` remains unchanged.

All 25 moved Go files were compared as parsed ASTs against the baseline. After
normalizing only the import prefix and the agreed composer function/input-type
rename, the trees are identical (comments/source positions excluded). At the
initial source checkpoint, `go.mod` changed only its module path and `go.sum`
was byte-identical; the subsequent approved retraction adds release metadata. This is behavior
preservation evidence, not a claim that every pre-existing behavior is ideal.

Both consumers resolve exactly one effects module, at the new path. Their actual
`go list -m all` and package dependency inventories contain no `examples/effects`.
Second Wind's healing/resource behavior, Rage's real chains and lifetime rules,
and the stored-loot identity/charge assertions remain exercised by existing tests.

## Verification observed

- Provider baseline: fresh race tests, machine example and lint passed before moving.
- Extracted provider: `go fmt ./...`, `go mod tidy`, `go test -count=1 -race ./...`,
  machine `Example`, `go vet ./...`, and `golangci-lint run ./...` passed.
- Provider benchmark smoke (`go test -run '^$' -bench . -benchtime=1x ./...`)
  passed; this is execution coverage, **not** a performance conclusion.
- S1 and S2: baseline race tests and post-migration full-module race tests passed.
  S1's example and both S2 examples passed; both consumers passed vet and lint.
- Provider [CI run](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42314139680)
  passed at `e15c6e54`, including the actual `mechanics/effects` module job.
- S1 [CI run](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42314557510)
  passed at `7d7ef342`.
- S2 still targets S1, so automatic main-target PR CI did **not** run. Local
  consumer results are not substituted for that missing head CI.
- Module map: all 28 modules appear exactly once in their nearest map. ADR digest:
  all 51 records summarized. The source checkpoint checked 77 local doc links and
  staged whitespace. The root genre check retains 44 advisory findings across 88
  pre-existing law files; it is not globally clean and its policy was not weakened.

No new lifecycle/identity mutant was introduced: normalized runtime ASTs are
unchanged and the migration touches neither seam's behavior. Existing meaningful
consumer tests were retained, rather than manufacturing a failing test for a rename.

## Corrections and limits

The first plan recommended a top-level module. The operator selected the mechanics
layer and Compose/ComposeInput spelling before source moved. The selected module
path has history through `mechanics/effects/v0.2.1`, confirmed as an ancestor of
main; it is **not** a new module starting at v0.1.0. Existing version pins retain
the retired implementation. After the initial source checkpoint, KirkDiggler
approved `retract [v0.1.0, v0.2.1]` with an explanatory comment in go.mod. The
range withdraws the legacy API from automatic selection without deleting tags
or preventing explicit pinned builds. Consumers discover the retraction through
the next published release; it is not a compatibility shim or an automatic
migration. The new API needs CI's actual next pre-v1 release.

Historical knowledge decisions were not hand-edited. The new run record carries
the current ruling for ingest; live source maps and guides reflect the moved code.
The Git staging command initially named the now-absent old directory after `git mv`;
the staged deletion/new destination were checked explicitly before committing.

## Still required before final adoption

- Provider review, merge, actual tag, consumer re-pins/re-tests and S2 retargeting
  and final consumer reviews/merges are observed above. No delivery step remains
  for this bounded first cut; the production boundaries below remain outside it.
- Production action definitions/economy, shared rulebook effect vocabulary and
  condition factories, class-feature reuse, provenance beyond a content SourceRef,
  stacking/eligibility, designer parameter schemas, native encounter inventory,
  atomic persistence and production condition migration are not proved by this cut.

The examples' preparation-versus-delivery failure boundary remains as documented:
runtime charges are reserved during preparation; the host example commits saved
item state only after successful effect delivery. There is no distributed rollback.
