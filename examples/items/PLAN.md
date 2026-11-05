# Item package experiment — next cut

Authority: the operator requested continuing the item example after merging the effects cleanup. This is an exploratory package under `examples/`, not extraction or production integration. [Slice #862](https://github.com/KirkDiggler/rpg-project/issues/862), [Proposed ADR](https://github.com/KirkDiggler/rpg-toolkit/pull/2272).

## Shape to exercise

- `New(Config) (*Item, error)` binds a pure authored `Definition` to an instance ID and optional saved `Data`. The definition declares grants and pools; Config also supplies consumer grant validation and the supported recovery-trigger vocabulary.
- `Item.Attach(ctx, AttachInput)` composes real effects outputs for grant availability and targeted recovery on the caller's bus. `Detach` removes that wiring without refilling charges. This first package supports these grant/recovery outputs, not arbitrary passive-item recipes.
- `Item.Offers(OffersInput)` returns inspectable, copied declarations with source identity and affordability; it does not execute a rule or expose a mutable pool.
- `Use[Result](ctx, UseInput[Result])` coordinates a selected grant's item-owned payment and a consumer-supplied pure resolver, returning its typed outcome. Item and Config are not generic. One generic operation avoids a D&D result import or untyped result/JSON conversion in the item layer.
- Resolver failure restores the locally reserved item charge before any world delivery. The resolver must return a value, not mutate/publish the game outcome. This is the existing probe's bounded calculation contract, not arbitrary transactional rollback.
- `Item.Data()` stores item identity, definition ref and remaining pool values. The generated attachment recipe has one state and no mutable bag; no effect runtime is persisted. Restore requires all declared pool states and rejects invalid counts/ref mismatches rather than refilling/clamping them.
- Second Wind's ref/parameters, fixed dice and real `healing.Resolve` stay in the consumer tests. A test-only long-rest-only variant continues to test configuration, not change the first ring rule.

## Files and sequence

1. Replace the test-local item implementation with a thin consumer adapter in `fixture_test.go` and update `items_test.go`/`example_test.go` to consume the public package. Initial missing-API compile errors are an interface checkpoint, not behavioral defect evidence.
2. Add `definition.go` (source/config/data types and ingress validation), `item.go` (instance ownership, effects composition and inspection), `use.go` (typed outcome boundary). Extend `doc.go`/README with responsibilities and limits. No provider modules change.
3. Retain all eight existing consumer scenarios, replacing private-pool assertions with public snapshots. Add view-copy, mismatched/incomplete saved state, nil resolver, and import-boundary checks. Keep literal expected healing 7/9 and charges 0/1.
4. Run whole-module race tests, example, vet and lint. Remove the holder recovery filter temporarily: the transfer test must fail; restore it and re-run. Verify grants and persisted state cannot mutate the runtime through returned copies.
5. Publish this example checkpoint in #2273 and record findings in the journal/ADR. Neither merge nor declare the candidate API final.

## Coverage / seams

| Requirement | Consumer proof |
|---|---|
| R1 item level | Two real healing outcomes, 7/9; no class level supplied |
| R2 item-owned pool | Two same-definition items; transfer/reload and source IDs |
| R3 item composer | Public Item drives real effects outputs, owns grant selection and charge accounting |
| R4/R5 configured recovery / one charge | Short and long triggers, alternate-policy fixture, wrong holder refused |
| Rules stay outside generic items | Only test consumer imports D&D; import-boundary assertion |
| Values decide, no duplicate delivery | Typed resolver result returned; no item-side HP mutation or action-economy claim |
| No mutable projection authority | Mutating offered parameters or saved pool map does not alter live state |

Open: production action-definition/availability integration, actual equipment/rest eligibility, world outcome delivery, passive-item composition and arbitrary multi-effect lifetimes. These are not silently solved by the fixture. No complete reusable grant schema or studio form is accepted here.

## Observed checkpoint

- The candidate is implemented in `definition.go`, `item.go` and `use.go`; D&D parsing/healing stays in `fixture_test.go`.
- The original consumer scenarios now call the public example package. Additional assertions cover copied definitions/projections, complete saved state, context cancellation, consumer validation, missing resolver, pool references and the non-test import boundary.
- Three behavioral mutants are caught: remove holder scoping on recovery, charge zero on successful use, or return an internal parameter buffer. All mutants are removed.
- Exact test/lint/CI receipts and the current head are recorded in PR #2273. This is a draft experiment for operator inspection, not an accepted API or permission to merge the item work.
