# Run — 2026-11-13 — Composable conditions S19 contribution selection

Slice [rpg-project#955](https://github.com/KirkDiggler/rpg-project/issues/955), story [#703](https://github.com/KirkDiggler/rpg-project/issues/703). Base [fefd3254](https://github.com/KirkDiggler/rpg-toolkit/commit/fefd3254).

## Shipped

- Source checkpoint supplies pure explicit AllApplicable/OldestApplicable selection, ordered per-contribution decisions and unbound policy validation; release remains pending — [slice](https://github.com/KirkDiggler/rpg-project/issues/955).

## Decided

- Kirk separates condition coexistence and independent lifetimes from contribution selection; the neutral selector consumes already-applicable values and neither rolls nor mutates instances — [design agreement](https://github.com/KirkDiggler/rpg-toolkit/pull/2425#issuecomment-6865141821).
- The implementation adds ValidateSelectionRule to the plan's initial API because the rulebook compiler must validate a policy before output IDs exist; the alternative would duplicate validation or invent an identity — [task contract](https://github.com/KirkDiggler/rpg-toolkit/blob/design/bless-bane-building-blocks/docs/ideas/bless-bane-building-blocks/plan.md#task-1--neutral-contribution-selection).

## Found

- External tests distinguish input order from lexical IDs, independent groups from all-applicable values, next-query handoff from retained winners, whole-input refusal from partial output, and copied decisions from mutation aliases — [tests](../../selection_test.go).

## Mistaken

- The first owning lint run found the new test's local imports grouped with third-party testify imports; explicit grouping now follows the adjacent external-suite files rather than assuming gofmt enforces goimports — [tests](../../selection_test.go).
