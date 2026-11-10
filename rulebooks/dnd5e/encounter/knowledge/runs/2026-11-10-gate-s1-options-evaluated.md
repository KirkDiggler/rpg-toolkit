# Run — 2026-11-10 — Gate S1: options are an evaluated menu

Slice [rpg-project#921](https://github.com/KirkDiggler/rpg-project/issues/921) of story [rpg-project#769](https://github.com/KirkDiggler/rpg-project/issues/769). Base: `rpg-toolkit` `main` at [`7dd61222`](https://github.com/KirkDiggler/rpg-toolkit/commit/7dd61222). Head: the [rpg-toolkit#2355](https://github.com/KirkDiggler/rpg-toolkit/pull/2355) branch `feat/769-gate-s1-options-evaluated`.

## Shipped

- The encounter pins `mind/behavior` v0.23.0, and its suite passes on it — [rpg-toolkit#2355](https://github.com/KirkDiggler/rpg-toolkit/pull/2355).
- Speak's offer and Speak's door read one `selection.Evaluation`; the door is `selection.Choose`, the first real consumer of it — [rpg-toolkit#2355](https://github.com/KirkDiggler/rpg-toolkit/pull/2355).

## Decided

- An option's candidate weight is 0: a line is chosen by its speaker, never drawn, and a zero-share eligible row is still choosable — [rpg-project#917 R5](https://github.com/KirkDiggler/rpg-project/issues/917).
- A line the target lacks and a line gated for this speaker stay one refusal, `ErrNoOption`, so the door cannot confirm a gated line exists (the probe law) — [rpg-project#921 plan](https://github.com/KirkDiggler/rpg-project/issues/921).
- A row's reason names only the requirement's kind (`fact`, `holds`, `not`, `none`), never the fact or prop it waits on; ineligible rows stay inside the encounter in this slice — [rpg-project#921 plan](https://github.com/KirkDiggler/rpg-project/issues/921).
- A blank or duplicate option id reaching the pool is returned wrapping `selection.ErrInvalidCandidate`, never swallowed — [rpg-toolkit#2355](https://github.com/KirkDiggler/rpg-toolkit/pull/2355).

## Found

- Only one encounter test die ignored its size (`facedDie`), and its faces (1 and 101 of 200) were already in range, so v0.23.0's refusal changed no expectation — [rpg-toolkit#2355](https://github.com/KirkDiggler/rpg-toolkit/pull/2355).
- The encounter has no drop verb, so a prop line gated on holding is proved refused before the hold, not after a drop as the plan wrote — [rpg-toolkit#2355](https://github.com/KirkDiggler/rpg-toolkit/pull/2355).
- The four public tests pass on base: the change is behavior-preserving, so the RED came from an internal test of the pool itself — [rpg-toolkit#2355](https://github.com/KirkDiggler/rpg-toolkit/pull/2355).

## Mistaken

- The first pre-commit run was piped through `tail`, which hid its failing exit while `option.go` was unstaged; the hook was rerun with everything staged and passed before the second commit — [rpg-toolkit#2355](https://github.com/KirkDiggler/rpg-toolkit/pull/2355).
