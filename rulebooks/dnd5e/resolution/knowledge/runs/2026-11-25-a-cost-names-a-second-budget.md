# Run — 2026-11-25 — one way to pay for an ability: a cost can name a second budget

processed: 2026-11-25T190427Z

Slice [rpg-project#1159](https://github.com/KirkDiggler/rpg-project/issues/1159) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`64c707aa`](https://github.com/KirkDiggler/rpg-toolkit/commit/64c707aa).

One line per entry. Every line ends with a link.

## Shipped

- `Input.Budget combat.PoolSource`: the opening payment of an interaction pays the pools the budget owns from it and the rest from the payer, in one `combat.Pay` through `combat.WithPools` — [rpg-project#1159](https://github.com/KirkDiggler/rpg-project/issues/1159).
- The pin of `rulebooks/dnd5e` from v0.229.0 to v0.231.0 (`go.mod` only), the suite green after the pin alone — [rpg-project#1159](https://github.com/KirkDiggler/rpg-project/issues/1159).
- `cost_budget_test.go`: the seven cases of the issue, a budget with no cost refused by `Input.Validate`, and the budget on the input in place of the cost — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1159#issuecomment-7146182536).

## Decided

- The budget is on `Input`, not `Cost`, because a Cost is data that is frozen inside a stored pause and the answers-are-data guard refuses an interface there; the guard tests stay unedited — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1159#issuecomment-7146182536).
- `payAtTheDoor` keeps its signature and calls `payAtTheDoorFrom` with a nil budget, so the reaction, movement, equip and contest call sites do not change — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1159#issuecomment-7146182536).
- A spell with a budget is refused with ErrBadCost before anything is paid; a monster payer keeps its existing sentence — [rpg-project#1159](https://github.com/KirkDiggler/rpg-project/issues/1159).

## Found

- Pin diff: `rulebooks/dnd5e v0.229.0 => v0.231.0`; nothing failed after the pin alone — [rpg-project#1159](https://github.com/KirkDiggler/rpg-project/issues/1159).
- Fixture: no non-fighter fixture exists; the new test adapts the activation fighter (wizard class, no features, no pools, no levels) — [rpg-project#1159](https://github.com/KirkDiggler/rpg-project/issues/1159).
- Proof 1: paying through the character alone fails "a wizard uses the ring" and the budget-at-0 retry. Proof 2: removing the spell refusal fails the spell test — [rpg-project#1159](https://github.com/KirkDiggler/rpg-project/issues/1159).
- Test 7c (a reaction's cost is not paid from the budget) is pinned by the diff: no existing reaction path can be driven from a new file without editing an existing test, and the four other `payAtTheDoor` call sites are unchanged — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1159#issuecomment-7146182536).
- Suite before: ok in 1.0s; at the final head: ok; race run of Budget|Effect|Cost ok in 15s; golangci-lint 0 issues — [rpg-project#1159](https://github.com/KirkDiggler/rpg-project/issues/1159).

## Mistaken

- The first design put an interface field on Cost; `TestEntriesReturnAnswersNotSheets` and `TestEveryOutcomeIsAnswerShaped` refused it and the brief was amended — [gap](https://github.com/KirkDiggler/rpg-project/issues/1159#issuecomment-7146165809).
