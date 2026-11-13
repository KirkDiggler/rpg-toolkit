# Run — 2026-11-07 — canonical behavior session integration

processed: 2026-11-12T165555Z

Part of [live adoption #866](https://github.com/KirkDiggler/rpg-project/issues/866), under [story #859](https://github.com/KirkDiggler/rpg-project/issues/859).

## Shipped

- Prepared session launch/join propagation and ordinary-driver delegation for the canonical encounter configuration; this checkpoint is unmerged and does not claim API/Studio or a running stack — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).

## Decided

- Removed implicit monster-kind table folding instead of adding a compatibility arm; absent policy remains absent, and legacy placement table/machine input refuses under R9 — [R9](https://github.com/KirkDiggler/rpg-project/pull/877#issuecomment-6698028412).
- Kept compulsion lookup and resolution ahead of encounter delegation; no session-owned Composer, picker or second die capability — [consumer plan](https://github.com/KirkDiggler/rpg-project/pull/877).
- Pinned pushed encounter commit `c1cbf6236a4f410ced92295cccc147d29f2b3e83`; this consumer remains draft until provider release, with the provider itself explicitly held for the operator's local-stack look — [hold](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#issuecomment-6710899038).

## Found

- Complete authored patrol source survives launch and ordinary verb save/reload; reads consume no selection dice, and two world opportunities consume exactly four d100 choices — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).
- The real Command/EndTurn path performs zero ordinary choices while compelled and one after the command ends; an overlay that bypasses compulsion fails both actual position and die-count assertions without modifying repository bytes — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).
- Full session-module race tests passed (root 426.096s, workbench 1.203s), and lint reported zero issues — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).
- Rebuilt example transcripts differ only in instance-qualified ordinary answer keys and removal of unauthored hold rolls; movement, social verdicts, attack rolls and outcomes were compared unchanged — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).

## Mistaken

- Initial broad inventory retained legacy fixture assumptions about implicit policy, saved Machine state and unqualified answer keys; explicit fixture configuration and canonical saved-state assertions replace those assumptions without restoring retired production defaults — [R9](https://github.com/KirkDiggler/rpg-project/pull/877#issuecomment-6698028412).
