# Run — 2026-11-23 — a note has a value S3b the read-back says it

processed: 2026-11-25T190427Z

Slice [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115) of story [rpg-project#1007](https://github.com/KirkDiggler/rpg-project/issues/1007). Base: `rpg-toolkit` `main` at [`e5e422c0`](https://github.com/KirkDiggler/rpg-toolkit/commit/e5e422c0).

## Shipped

- A valued note is said in the sentence and in the `topic` connection's `is`, for a give and an ask, on a line, a post, a conversation's state, a machine's state and a machine's step — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- `as` on a give is said: in `saidKeys["give"]`, out of `acknowledged` — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- The not-yet walk descends into `behaviorDefinitions.<id>.outcomes.<state>[i]` (each an `outcome`, its `give` a `give`) — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- Places now saying a value: a line's give (`lines.go`, sentence and folded), a conversation state's give (`conversations.go`), a machine state's give (`routines.go`, sentence and folded), a line's or a post's ask and not-ask (`words.go`), a step's `noted.as:` (`routinewords.go`), the ways (`ways.go`: the grant sentence and edges, the `needs` edge, the `unless` sentence and edge) — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- Proof: the new tests failed first for the right reason, and dropping `Is` from `noteAskEdges` fails the line-ask, post and step tests — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- Whole suite before: all 12 packages ok; at the final head: all 12 packages ok; no golden changed; `golangci-lint` 0 issues — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).

## Decided

- A topic with no value is said by one helper that returns the quoted topic unchanged, so every existing sentence stays byte for byte — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- `ways.go`'s grant sentence ("makes the creature it is said to note …") and its `needs`/`unless` edges say the value too: the issue's rule is "where a topic is named", and these name one — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- `noted.as:` is said only for `eq`; any other comparison keeps the generic enemy-fact sentence — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).

## Found

- F1 to F8 all hold at `e5e422c0`: F1 two lines, F2 six lines (ways four, words two), F3 two plus two, F4 one, F5 one, `:318`, F6 fields exist, F7 one, F8 two files — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- Every statement is repeated by the hubs under `topic.*` kinds at the same path, so a test finds a site's statement by kind or skips pointers — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
- The behaviour definition's `outcomes` entry is an `outcome` mapping; the walk change was four lines — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).

## Mistaken

- During the mutant proof the backup copy went to a different directory than the restore read; the file was restored by hand and the tests re-run green — [rpg-project#1115](https://github.com/KirkDiggler/rpg-project/issues/1115).
