# Run — 2026-11-21 — Status effect powers S3 initiative rolls

Slice [rpg-project#1047](https://github.com/KirkDiggler/rpg-project/issues/1047) of story [rpg-project#1036](https://github.com/KirkDiggler/rpg-project/issues/1036). Base: `rpg-toolkit` `main` at [`068393f7`](https://github.com/KirkDiggler/rpg-toolkit/commit/068393f793dff41e05446b8e1305e6cd0991ca7e).

One line per entry. Every line ends with a link: a PR, an issue comment, a commit, or a design section anchor. This file rides on the work PR, under the `knowledge/runs/` of the lowest directory that contains everything the PR changed. The ingest reads it once and adds a `processed:` line at the top; nobody else edits it after merge.

## Shipped

- The fight-forming record entry (`bubble-formed`) carries each member's initiative roll in `rolls` — [rpg-toolkit#2500](https://github.com/KirkDiggler/rpg-toolkit/pull/2500).
- `InitiativeRoller.RollInitiative` returns `[]InitiativeRoll` — [rpg-toolkit#2500](https://github.com/KirkDiggler/rpg-toolkit/pull/2500).
- `PlacedInOrder` is the answer for a roller that orders without rolling — [rpg-toolkit#2500](https://github.com/KirkDiggler/rpg-toolkit/pull/2500).

## Decided

- The one seam changed its return type and no second method was added beside it, because two ways to ask for initiative drift; the lead's call — [rpg-project#1047](https://github.com/KirkDiggler/rpg-project/issues/1047).
- A roll with no Calculation means placed, not rolled, and is not recorded, so a record written by a placing roller is byte for byte what it was — [rpg-project#1047](https://github.com/KirkDiggler/rpg-project/issues/1047).
- Everyone in a fight reads every member's roll: the rolls ride the fight-forming entry, which today goes to the whole roster like every other entry (`audienceFor` in `encounter.go`, the pre-v1 "everyone" policy) and narrows to the fight's members when rpg-toolkit#940 lands; the operator's ruling of 2026-11-21 — [rpg-project#1035 comment](https://github.com/KirkDiggler/rpg-project/issues/1035#issuecomment-7051043597).

## Found

- A member who joins a running fight is placed last and never rolls; `applyVerdict` transfers it with `Pos: e.bubbleSize()` — [trigger.go at base](https://github.com/KirkDiggler/rpg-toolkit/blob/068393f7/rulebooks/dnd5e/encounter/trigger.go#L609).
- The engine does not check that the roller answered exactly the members it was asked about; true before this change — [review of rpg-toolkit#2500](https://github.com/KirkDiggler/rpg-toolkit/pull/2500#pullrequestreview-5999802330).
- A recorded calculation needs a `Ref` and a `Name` on every source and a `SourceID` on a dice source — [rpg-project#1047 Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1047#issuecomment-7051212649).

## Mistaken

- The slice issue left the run record out of its file list; the review caught it — [review of rpg-toolkit#2500](https://github.com/KirkDiggler/rpg-toolkit/pull/2500#pullrequestreview-5999802330).
- The slice issue did not say what a test calculation must carry; the lead added it as Amendment 1 before the builder met it — [rpg-project#1047 Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1047#issuecomment-7051212649).
