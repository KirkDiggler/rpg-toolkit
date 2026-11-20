# Run — 2026-11-20 — a conversation's line keeps the note edges of its copies

Follow-up to [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479) under [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025). Found while rewriting the castle onto `conversations:`. Base: `rpg-toolkit` `main` at [`dcccad8a`](https://github.com/KirkDiggler/rpg-toolkit/commit/dcccad8a).

## Shipped

- A conversation's line, said once at `conversations.<id>.lines[k]`, now states every note edge its copies state. A `give: { to: spoken_to }` takes `{who, spoken_to, takes}`, and every creature carrier of the conversation `may-note` the topic by that way. A read of a noted topic (`noted:`, `not: noted:`) names every creature carrier as `whose` note it may be. Before, the line had no `spoken_to` taker and no `whose`, so its carriers appeared only as `may-note` by `heard` — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).
- `TestAConversationWrittenOnceStatesEveryEdgeOfItsCopies` builds `testdata/conversation-edges-site.yaml` twice: once as written, and once with each `use:` copied out in place. For every carrier and every line, each connection the copy states, of the same kind at the same place under the line, must also be stated by the line written once. Across the whole read-back, every connection the copies state must appear somewhere in the shared form — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).

## Decided

- Every creature carrier of a conversation may be the one spoken to. So the line written once gives the union of what each copy gives: `spoken_to` resolves to every carrier, and so does `carriers_who_witnessed`, because each carrier may hear the line said to another. Faction and prop carriers add nothing, as their copies name no creature spoken to — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).
- `said-to` and `done-to` still point at the conversation, as #2479 decided. The test accepts the carrier as reachable through the head's `carries` edge. It is the only difference between the copied and shared forms that is not a loss — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).

## Found

- `fact`, `tells`, `advance`, `apply_status`, `relocate`, `activity` and loudness lost nothing in the shared form. Their edges do not depend on who the line is said to — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).
