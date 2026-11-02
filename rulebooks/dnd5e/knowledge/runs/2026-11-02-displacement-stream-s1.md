# 2026-11-02 · Displacement stream S1 · Forced movement reaches the session act

Story [rpg-project#818](https://github.com/KirkDiggler/rpg-project/issues/818), slice [rpg-project#819](https://github.com/KirkDiggler/rpg-project/issues/819), contract [rpg-toolkit#2227](https://github.com/KirkDiggler/rpg-toolkit/issues/2227). Encounter beat PR [rpg-toolkit#2242](https://github.com/KirkDiggler/rpg-toolkit/pull/2242); session act PR [rpg-toolkit#2243](https://github.com/KirkDiggler/rpg-toolkit/pull/2243).

## Shipped

- The movement-beat payload seam writes `forced: true` only for external physical displacement at [`encounter.go:1701`](https://github.com/KirkDiggler/rpg-toolkit/blob/9877f3121bf033a50a610a0e5e49bcb2651e6656/rulebooks/dnd5e/encounter/encounter.go#L1701-L1707) ([rpg-toolkit#2242](https://github.com/KirkDiggler/rpg-toolkit/pull/2242)); absent remains a walk.
- The session decodes that payload key and its fold key is mover + cause + forced at [`session/act_family.go:248`](https://github.com/KirkDiggler/rpg-toolkit/blob/38588c8fdc8deadadb5d5a75acbfb75a9b93ac28/rulebooks/dnd5e/session/act_family.go#L248-L271) ([rpg-toolkit#2243](https://github.com/KirkDiggler/rpg-toolkit/pull/2243)), so a same-cause rout and push cannot merge.
- The host payload seam exposes the result as [`session.WalkAct.Forced`](https://github.com/KirkDiggler/rpg-toolkit/blob/38588c8fdc8deadadb5d5a75acbfb75a9b93ac28/rulebooks/dnd5e/session/act_bodies.go#L37-L38) ([rpg-toolkit#2243](https://github.com/KirkDiggler/rpg-toolkit/pull/2243)); absent/legacy payloads decode as walks.

## Decided

- `forced` stays one bool at every layer. Its false/absent value is the authoritative ordinary-walk reading; no enum or client inference joins this slice ([rpg-toolkit#2227 review](https://github.com/KirkDiggler/rpg-toolkit/issues/2227#issuecomment-6586825216)).

## Found

- The directive resume hand-step had to copy the new fact beside its already-copied cause at [`encounter/held.go:145`](https://github.com/KirkDiggler/rpg-toolkit/blob/9877f3121bf033a50a610a0e5e49bcb2651e6656/rulebooks/dnd5e/encounter/held.go#L145), or the first resumed pushed beat would have silently read as a walk; [rpg-toolkit#2242](https://github.com/KirkDiggler/rpg-toolkit/pull/2242) pins both halves.

## Mistaken

- Nothing.

— cross-team agent, on behalf of KirkDiggler
