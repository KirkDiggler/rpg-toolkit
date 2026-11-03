# 2026-11-03 · The guard holds the gate, S1 · Session

Story [rpg-project#836](https://github.com/KirkDiggler/rpg-project/issues/836), slice [rpg-project#841](https://github.com/KirkDiggler/rpg-project/issues/841), Task 2 of `ideas/the-guard-holds-the-gate/plan.md` (B6 at the session, D4). On encounter [rpg-toolkit#2258](https://github.com/KirkDiggler/rpg-toolkit/pull/2258).

## Shipped

- `go.mod` pins encounter at `v0.174.1-0.20261103223351-1348abbe853a`, #2258's final head, with no replace. Earlier commits pinned f0240055 and d6b62bc3.
- That pseudo-version is temporary. A follow-up commit moves the pin to the encounter release tag CI mints when #2258 merges, before this PR merges.
- Launch hands the placement's blocking states to the join beside its recipe at [`launch.go:237`](https://github.com/KirkDiggler/rpg-toolkit/blob/c9842667/rulebooks/dnd5e/session/launch.go#L237). Without it the castle's gate-1 is seated as a body anyone walks through.
- [`TestLaunch_PlacementBlockingStatesReachTheSeatedMember`](https://github.com/KirkDiggler/rpg-toolkit/blob/c9842667/rulebooks/dnd5e/session/launch_machine_test.go#L90) asserts the compiled states reach the seated member unchanged.
- [`TestCastleTheGuardHoldsTheGate`](https://github.com/KirkDiggler/rpg-toolkit/blob/c9842667/rulebooks/dnd5e/session/castle_gate_post_test.go#L93) walks alice toward the cell past gate-1, across the shut gate door. His body refuses the step before the door would. The walk aims past him, never at him. The Move answers `stopped` with "Guard stands in the way", one cell short. After the captain stands aside with gate-1 watching, he is `cowed` and the next walk crosses.
- [`TestABumpOnTheTurnClockPaysNoMovement`](https://github.com/KirkDiggler/rpg-toolkit/blob/c9842667/rulebooks/dnd5e/session/move_turnclock_test.go#L342) asserts the walked cell is paid and the bump is not. Paying the bump fails it.
- The gatehouse and troupe goldens, and their transcript prose, follow gate-1 onto his post. The thunderwave test aims at gate-2 and fails three saves, because gate-1 beneath the shut gate has nowhere to be shoved.

## Decided

- `encounter.ErrBodyInTheWay` joins the composition sentinels the session must never leak. No session refusal carries it.
- No `read.go` mapping for `ErrBodyInTheWay`. `Move` is the session's one `Step` caller, and the encounter always wraps a body refusal in `StepObstructedError`.
- `gate-cowed` is taught by alice's own stand-aside line on a die the test sets. The encounter teaches a fact only through an answer, a record or a door, so there is no direct way to hand it to a witness.

## Found

- A posted guard standing his post told a `stayed` beat with an empty reason. #2258's final head fixed it, and the goldens now show "gate-1 stays: already there".
- In the troupe golden, a voice draws posted gate-1 six cells off the crossing for a round, and he walks back the next. The crossing is open that round. This is #2258's draw row working as written.

## Mistaken

- The first castle walk ended on gate-1's cell. It passed only because alice did not see him when the walk began. #2258's review dropped the walk-end relaxation, so the walk now aims at the cell past him.
- The first castle test asserted the posted state by name. The record writes the recipe's initial state as empty.

— cross-team agent, on behalf of KirkDiggler
