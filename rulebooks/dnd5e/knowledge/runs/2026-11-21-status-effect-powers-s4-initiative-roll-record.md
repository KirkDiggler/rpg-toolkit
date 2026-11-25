# Run — 2026-11-21 — Status effect powers S4 initiative roll record

processed: 2026-11-25T190427Z

Slice [rpg-project#1049](https://github.com/KirkDiggler/rpg-project/issues/1049) of story [rpg-project#1036](https://github.com/KirkDiggler/rpg-project/issues/1036). Base: `rpg-toolkit` `main`.

One line per entry. Every line ends with a link: a PR, an issue comment, a commit, or a design section anchor. This file rides on the work PR, under the `knowledge/runs/` of the lowest directory that contains everything the PR changed. The ingest reads it once and adds a `processed:` line at the top; nobody else edits it after merge.

## Shipped

- `initiative.RollOne` rolls one member's initiative and returns its full record; every `conditions.InitiativeContribution` carries the `Ref` of what adds it; `refs.Rules.Initiative()` — [rpg-toolkit#2501](https://github.com/KirkDiggler/rpg-toolkit/pull/2501).

## Decided

- Initiative rolls through `rolls.RollD20`, the one d20 entry every other roll uses, so advantage on initiative later has a place to plug in; the lead's call — [rpg-project#1049](https://github.com/KirkDiggler/rpg-project/issues/1049).
- A rule's contribution names the rule's own ref (`dnd5e:rules:dexterity-initiative`), which the rules library already built, and no ref is made up at a call site — [rpg-project#1049](https://github.com/KirkDiggler/rpg-project/issues/1049).
- A zero modifier is kept and shown — [rpg-project#1049](https://github.com/KirkDiggler/rpg-project/issues/1049).
- Every refusal happens before the die, so a refused roll consumes no randomness — [rpg-project#1049 fix list 1](https://github.com/KirkDiggler/rpg-project/issues/1049#issuecomment-7051351118).

## Found

- The engine refuses a recorded roll whose sources carry no ref, and before this the initiative answer carried names only — [rpg-project#1047 comment](https://github.com/KirkDiggler/rpg-project/issues/1047#issuecomment-7051212649).

## Mistaken

- The first build rolled the d20 before refusing a bad modifier source, and its refusal test used dice that accept any number of rolls; the slice issue listed the refusals without saying they come before the die; the independent review caught both — [rpg-toolkit#2501 review](https://github.com/KirkDiggler/rpg-toolkit/pull/2501#pullrequestreview-5999801946).
- The slice issue left the run record out of its file list — [rpg-project#1049 addendum](https://github.com/KirkDiggler/rpg-project/issues/1049#issuecomment-7051394539).
- Fix list item 3 asked for a test of something that cannot be observed; the builder's gap report caught it — [rpg-project#1049 gap](https://github.com/KirkDiggler/rpg-project/issues/1049#issuecomment-7051379889).
