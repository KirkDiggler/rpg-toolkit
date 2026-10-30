# Decisions — mind/behavior

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-10-29 · a deed condition may carry `who`, legal only on the deed words in `WhoDeeds` (today `fell`), and `HeldDeed` carries the deed's `Target` that `who` is compared against; the shape is `When.Who` and `HeldDeed.Target` · options: a new payload for the fall · because: `Target` is the name `deed.Deed` already gives the field it is projected from, so the fall needs no new payload and every verb's target rides along · [D1 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722), [mind/behavior/table.go](https://github.com/KirkDiggler/rpg-toolkit/blob/32f5b6d6ff2f3a0b54cd8b314c92a76e9138070f/mind/behavior/table.go#L336), [rpg-toolkit#2149](https://github.com/KirkDiggler/rpg-toolkit/pull/2149) · upheld: none yet
- 2026-10-29 · a fall is filed by who fell as well as who caused it, and only a fall: `deed.FiledUnder` keys `fell` by actor, verb and target, while `attacked`, `intimidated`, `persuaded` and `fled` keep filing by actor and verb until a use case asks whom. Kirk, 2026-10-29: "go with the first option, file by who fell. maybe the target will be the right answer but I will wait for a use case to show that." · options: filing every verb by its target; one deed per actor and verb · because: a witness held one deed per actor and verb (`deed.Subject`), so when the same hero dropped the king and then a loyalist, every witness's fall by that hero named the loyalist and `fell: { who: king }` stopped holding inside its span · [rpg-toolkit#2149](https://github.com/KirkDiggler/rpg-toolkit/pull/2149), [rpg-project#716](https://github.com/KirkDiggler/rpg-project/issues/716), [deed.FiledUnder](https://github.com/KirkDiggler/rpg-toolkit/blob/32f5b6d6ff2f3a0b54cd8b314c92a76e9138070f/mind/behavior/deed/deed.go#L105) · upheld: none yet
- 2026-10-29 · `mind/behavior` reads a selector `ally`: the nearest allied member held in sight, else in memory, legal on `toward`, `dash` and `away` and refused on `attack`; `Facts.SeenAllies` and `Facts.RememberedAllies` carry who is held, and the caller decides who counts as allied · options: a selector `leader` naming one member; a personal `follows:` binding · because: lane ruling D2 chose `ally`, with allegiance through the faction's disposition so it lives in one place; a mind is never handed an ally to strike · [D2 on rpg-project#722](https://github.com/KirkDiggler/rpg-project/issues/722), [table.go SelectorAlly](https://github.com/KirkDiggler/rpg-toolkit/blob/f6e5e4119d27f982f5385c4f18c1d0baf75a30fe/mind/behavior/table.go#L680), [rpg-toolkit#2156](https://github.com/KirkDiggler/rpg-toolkit/pull/2156) · upheld: none yet

## Findings

- none yet

## Friction

- none yet

## Open

- none yet
