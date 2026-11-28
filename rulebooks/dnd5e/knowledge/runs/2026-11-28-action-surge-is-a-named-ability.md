# Run — 2026-11-28 — second ability 5c-1 action surge is a named ability

Slice [rpg-project#1259](https://github.com/KirkDiggler/rpg-project/issues/1259) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`4ed327b5`](https://github.com/KirkDiggler/rpg-toolkit/commit/4ed327b5).

One line per entry. Every line ends with a link.

## Shipped

- Action Surge is one entry in the table of named abilities: word `action surge`, no values, one use, one more action this turn, no slot charged — [slice](https://github.com/KirkDiggler/rpg-project/issues/1259).
- A counted ability must be paid from a count: `counted` on the entry, `features.MustBeCounted`, a refusal in `DescribeFromRef`, and in the kit when written and when loaded — [slice, Design 3](https://github.com/KirkDiggler/rpg-project/issues/1259).
- The words table says it: `features.AbilityWord.Counted` and `kits.Value.NeedsCount` (a new exported field) — [amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1259#issuecomment-7231108554).

## Decided

- The smallest `lets` entry in the words tests uses the first ability that needs no count, and a test that writes an entry for a value of `use` writes the `uses` the table says it needs; no assertion changed — [amendments 2 and 3](https://github.com/KirkDiggler/rpg-project/issues/1259#issuecomment-7231153371).
- No README lines about adding a named ability exist; the `counted` field's comment on the entry type carries the lesson — [slice, files list](https://github.com/KirkDiggler/rpg-project/issues/1259).

## Found

- F1-F8 hold at `4ed327b5`; F8 found no place of the third kind: every hit of `refs.Features.SecondWind()` is the class feature's or the entry's own, plus one stale doc comment fixed — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1259#issuecomment-7231050228).
- Other modules: only two run records in `session/knowledge/runs` quote `it is one of second wind`; no test in `encounter`, `resolution` or `session` pins the list — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1259#issuecomment-7231050228).
- Refusals: `action surge must be paid from a count: its giver gives it uses or a cost`; `<holder>: lets[0]: \`use\` is "action surge", which gives its user something for its count alone; write \`uses\` or a \`cost\``; `<holder> grant "action-surge" is action surge, which must be paid from a pool`; `action surge takes no values` — [slice, Design 3](https://github.com/KirkDiggler/rpg-project/issues/1259).
- `monster`, `character` and `combat` needed no change: a stat block and a worn item describe and pay Action Surge with no code of their own — [slice, Design 4](https://github.com/KirkDiggler/rpg-project/issues/1259).
- Proofs: dropping `GrantSlots` fails the description literal and the 2-actions assertions (1 action, not 2); `counted` false fails the count refusals, the kit's two refusals and the words test; removing the compile refusal fails the written-uncounted case, and the load refusal still catches it (the contract test then reads the load sentence); charging an action fails with `Should be empty, but was map[action:1]` and 1 action left — [slice, proofs](https://github.com/KirkDiggler/rpg-project/issues/1259).
- The suite and lint pass before and at the final head — [slice, commands](https://github.com/KirkDiggler/rpg-project/issues/1259).
- THE COUNT: the ability itself is `features/action_surge_named_ability.go` and the table entry; the one new rule is `counted`, `MustBeCounted`, `DescribeFromRef`, `kits/compile.go`, `kits/kit.go`; the words table is `features/named_ability_words.go` and `kits/words.go`; the rest is tests and this record — [slice](https://github.com/KirkDiggler/rpg-project/issues/1259).

## Mistaken

- The brief left out a words-table field for "must be counted"; seven tests that build the smallest entry from the table showed why it cannot be left out — [gap](https://github.com/KirkDiggler/rpg-project/issues/1259#issuecomment-7231094947).
- Amendment 1 guessed the helper alone would do; two more loops overwrote `use` with no count — [gap](https://github.com/KirkDiggler/rpg-project/issues/1259#issuecomment-7231168790).
