# Run — 2026-11-21 — Authored interrupt S1 play proof

Slice [rpg-project#1070](https://github.com/KirkDiggler/rpg-project/issues/1070) of story [rpg-project#1061](https://github.com/KirkDiggler/rpg-project/issues/1061). Base: `rpg-toolkit` `main` at [`f07abf7d`](https://github.com/KirkDiggler/rpg-toolkit/commit/f07abf7d).

One line per entry. Every line ends with a link.

## Shipped

- A test scene (`interrupt_scene_test.go`) proving an author-written machine stops a walker on a heard failed claim, holds him, and lets him walk on — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).

## Decided

- none (a test-only slice; the lead chose the scene) — [rpg-project#1070](https://github.com/KirkDiggler/rpg-project/issues/1070).

## Found

- He stops: a failed athletics claim moves him to `watching` and he does not move while it lasts — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).
- He is stopped for two further world rounds after the line, because the line itself pays one round and the anchor is stamped with the round it started in; he is `ready` after the second, and walks on the third — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).
- A passed claim leaves him `ready` and walking — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).
- A plain line leaves him `ready` and walking — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).
- The skill does not matter to him: the machine reads only `heard.claim`; the line's check skill is not read — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).
- He walks six cells a round at 30 feet of speed — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).
- A creature hostile to the party is in a fight and has no world-clock walk: with no faction set he never moved; a neutral faction made him walk — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).

## Mistaken

- The first scene put him in a fight (no faction), so he never walked; the neutral garrison faction fixed the scene, not the engine — [rpg-toolkit#2506](https://github.com/KirkDiggler/rpg-toolkit/pull/2506).
