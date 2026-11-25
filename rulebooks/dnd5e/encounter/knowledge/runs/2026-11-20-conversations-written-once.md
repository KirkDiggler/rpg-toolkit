# Run — 2026-11-20 — a conversation's lines are written once and applied to carriers

processed: 2026-11-25T190427Z

Engine slice [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025) of the conversation shape ruled on [rpg-project#987](https://github.com/KirkDiggler/rpg-project/issues/987) (R20–R24). Base: `rpg-toolkit` `main` at [`072af96c`](https://github.com/KirkDiggler/rpg-toolkit/commit/072af96c). PR [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).

## Shipped

- The root `conversations: { <id>: { lines: [...] } }` and an `options:` entry `- use: <id>`, which stands for the conversation's lines, in order, at that place. Creatures, props and factions can all carry one. It is expanded at load, before bundles and the grammar. The runtime, session and protos are unchanged — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).
- The castle with the gate's lines written once compiles to the same `Compiled` and content hash as the copied castle — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).
- The read-back: one `shared-lines` statement per conversation, connected `{creature|prop|faction, <id>, carries}`, and its lines said once at `conversations.<id>.lines[k]`. Each carrier's use is a `shared-lines.use` statement connected `{shared-lines, <id>, speaks-lines-of}` — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).

## Decided

- A conversation is expanded at load, not compiled as one: what the engine runs is the copied form, so the runtime and the wire never learn the word — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).
- The read-back names it `shared-lines`, as subject and statement kind. `conversation` already names an `interactions:` entry, and `lines` reads as the `line` family — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).
- A faction may carry a conversation, the same way a faction's `options:` may hold lines today — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).
- Validation and compile expand into a copy of the carriers' lists (`withOwnLines`). The caller's spec keeps its `use:` entries, and nothing is cached on it — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).
- Refusals are said at authored paths only. A fault in a conversation's line is said once at the line. If only some of its carriers meet the fault, the refusal names the `use:` entries it came through. An id written twice inside one conversation is refused once at that conversation. A clash between a carrier's own line and a used line is refused on that carrier, naming both sources — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).
- Conversations do not nest, and one carrier uses a conversation once — [rpg-project#1025](https://github.com/KirkDiggler/rpg-project/issues/1025).
- A conversation's line statement connects `said-to` to the conversation. The content is written once, and its carriers read off the head's `carries` edges — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).

## Found

- Once a list holds a `use:`, the grammar's duplicate-id message cites an expanded index. Lists that hold a `use:` now have their ids judged in the expansion, where every index is the author's — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).
- If a conversation has a single carrier, a carrier-dependent fault (a `do:` line said to a creature) cannot be told apart from a fault in the line itself, so its refusal names no `use:` — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).
- The joins and the ways found a line by its index under `.options[`. They now find it by id and by its authored path, so a passage through a conversation's line is said once for each carrier — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).

## Mistaken

- The first head cached the authored lists on the spec and restored them only in decode. A compile left the caller's spec expanded, and a second compile after an edit read the stale cache. Review found it — [rpg-toolkit#2479](https://github.com/KirkDiggler/rpg-toolkit/pull/2479).
