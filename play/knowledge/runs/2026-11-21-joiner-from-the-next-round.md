# Run — 2026-11-21 — Joining a fight S1 seated from the next round

Slice [rpg-project#1071](https://github.com/KirkDiggler/rpg-project/issues/1071) of story [rpg-project#1068](https://github.com/KirkDiggler/rpg-project/issues/1068). Base: `rpg-toolkit` `main` at [`4ee0ddd6`](https://github.com/KirkDiggler/rpg-toolkit/commit/4ee0ddd63111682b116955603235f62dc1b49152).

## Shipped

- A member inserted into a turn order for the next round is passed over until the round wraps, survives a save, and rides Insert, JoinMember and Transfer — [rpg-toolkit#2507](https://github.com/KirkDiggler/rpg-toolkit/pull/2507).

## Decided

- The clock passes over a member seated for the next round, and the encounter does not fake and cancel a turn, because a faked turn fires turn-start and turn-end for a creature that had no turn; the lead's call from the build plan — [rpg-project#1071](https://github.com/KirkDiggler/rpg-project/issues/1071).

## Found

- When the active member leaves and the order wraps without the round advancing, a member seated for the next round acts in the new pass, in the same round number it joined in; pinned by `TestRemovingTheActiveWrapsPastMembersForTheNextRound`; the lead ruled it stays — [fix list 1](https://github.com/KirkDiggler/rpg-project/issues/1071#issuecomment-7058591867).

## Mistaken

- none observed — [rpg-project#1071](https://github.com/KirkDiggler/rpg-project/issues/1071).
