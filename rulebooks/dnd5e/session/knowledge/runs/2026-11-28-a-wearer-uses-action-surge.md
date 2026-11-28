# Run — 2026-11-28 — second ability S5d-1b a wearer uses Action Surge

Slice [rpg-project#1265](https://github.com/KirkDiggler/rpg-project/issues/1265) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`55c6ec3d`](https://github.com/KirkDiggler/rpg-toolkit/commit/55c6ec3d).

## Shipped

- The pin `rulebooks/dnd5e` v0.247.0 to v0.248.0; `go mod tidy` moved only go.mod's one line and go.sum's two lines for it — [pin](https://github.com/KirkDiggler/rpg-toolkit/commit/e675cb67).
- `action_surge_wearer_test.go`: six tests, a player wearing a cloak that lets her use Action Surge uses it through the manager — [tests](https://github.com/KirkDiggler/rpg-toolkit/commit/47edc98a).
- Production files changed in this module: none — [slice](https://github.com/KirkDiggler/rpg-project/issues/1265).

## Decided

- The site is the Lieutenant's cloak site with its one `lets` entry replaced by `use: action surge` (cost one charge), the text checked to occur once; no new site file, because replacing text was enough — [slice](https://github.com/KirkDiggler/rpg-project/issues/1265).
- She wears it in a fight by two runs of one saved character: run one loots and equips in free roam, run two launches the same text over the same store with the Lieutenant standing (a launch is a long rest, so the cloak is 1 of 1), because the site's own scene ends its fight when the Lieutenant is downed — [comment](https://github.com/KirkDiggler/rpg-project/issues/1265#issuecomment-7231682253).
- The offers are read through `Manager.Afford`'s Activate rows, and the count from the saved sheet, because an Activate declaration carries no count — [comment](https://github.com/KirkDiggler/rpg-project/issues/1265#issuecomment-7231682253).
- The world-clock behaviour is reported as a gap and not pinned in a test, because a test would fix a behaviour that may be wrong — [gap](https://github.com/KirkDiggler/rpg-project/issues/1265#issuecomment-7231699721).

## Found

- F1 holds, F2 holds, F3: build, vet and the whole suite pass at `origin/main` and again after the pin; no test fails — [comment](https://github.com/KirkDiggler/rpg-project/issues/1265#issuecomment-7231682253).
- (a) Affordability is the rulebook's: `activations.go:87` `Available: ability.CanUse`; `worn_items.go:167` `slot := coreCombat.ActionFree` is the default for a cost that charges no slot, `action_economy.go:553` answers true for it, and `:185` refuses it when the pool is short — [comment](https://github.com/KirkDiggler/rpg-project/issues/1265#issuecomment-7231682253).
- (b) `Activate` prices the grant's cost cloned with a `Turn` (`activate.go:290-291`); on the turn clock it passes and gives the action; on the world clock it passes, spends the charge, and the saved economy is nil — [comment](https://github.com/KirkDiggler/rpg-project/issues/1265#issuecomment-7231682253).
- (c) The character's own ledger pays the item's charge: `ledger.go:192` and `:212` answer `ItemPoolKey` from the worn item's live record — [comment](https://github.com/KirkDiggler/rpg-project/issues/1265#issuecomment-7231682253).
- Her row: verb activate, slot none, available, "Action Surge · Lieutenant's cloak", giver (item id, "action-surge"), no count (`Remaining` is nil); the count is on the sheet, `Left["charges"]` 1 of a maximum of 1 — [tests](https://github.com/KirkDiggler/rpg-toolkit/commit/47edc98a).
- `Activate` returns ability `dnd5e:features:action_surge`, `GrantedCapacity` "1 action", saved `character:alice` and `encounter:sess`; the charge goes 1 to 0 and her actions 1 to 2 with her bonus action untouched — [tests](https://github.com/KirkDiggler/rpg-toolkit/commit/47edc98a).
- With her second action she attacks twice in one turn: two `struck` or `missed` beats in her story, a third attack and, without the surge, a second attack are each refused `declaration is stale` — [tests](https://github.com/KirkDiggler/rpg-toolkit/commit/47edc98a).
- With her action and bonus action spent the row is still available; using it gives back one action (1, bonus 0) and she attacks again — [tests](https://github.com/KirkDiggler/rpg-toolkit/commit/47edc98a).
- Spent, the same row id reads unavailable with why no budget, currency charges, needed 1, left 0, "the Lieutenant's cloak has no charges left", and `Activate` returns `activate: declaration is stale` (`ErrStaleDeclaration`), changing nothing — [tests](https://github.com/KirkDiggler/rpg-toolkit/commit/47edc98a).
- Told: an `activated` beat (ability named "Action Surge · Lieutenant's cloak", no target) then an `activation-result` of kind `capacity-granted` described "1 action"; the live `ActivateAct` carries the same result, its ability's giver is nil and the item is in the name — [tests](https://github.com/KirkDiggler/rpg-toolkit/commit/47edc98a).
- At the old pin (v0.247.0, go.mod restored after) the six tests fail as the lead expected: the row reads unavailable ("Should be true" at the row's available check) and `Activate` returns `activate: declaration is stale`; the only passing part is the control that one action is one attack — [tests](https://github.com/KirkDiggler/rpg-toolkit/commit/47edc98a).
- Outside a fight the row is offered and usable, the charge is spent, and the action it gave is not kept — [gap](https://github.com/KirkDiggler/rpg-project/issues/1265#issuecomment-7231699721).

## Mistaken

- none observed — [slice](https://github.com/KirkDiggler/rpg-project/issues/1265).
