# Run — 2026-11-18 — draw a machine S1 the engine says a machine's shape

Slice [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994) of story [rpg-project#993](https://github.com/KirkDiggler/rpg-project/issues/993). Base: `rpg-toolkit` `main` at [`bb5806f7`](https://github.com/KirkDiggler/rpg-toolkit/commit/bb5806f7).

## Shipped

- The read-back says a behaviour machine's shape as connections on the statements it already makes: `routine` `{state, <initial>, starts}`; `routine.state` `{state, <S>, is}` and `{state, <S>, final}` when terminal; `routine.wake` `{state, <S>, from}`, `{state, <Y>, to}` when it moves, `{beat, <on>, on}`, `{table, <key>, picks}` per pick; `routine.row` `{table, <key>, row-of}`; `creature.post` and `creature.blocking` `{state, <S>, while}` per state — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).

## Decided

- The new kinds and roles are exported constants beside `Connection` (`ConnState`, `ConnBeat`, `ConnTable`, `RoleStarts` … `RoleWhile`), so a consumer keys on a name; the existing roles stay literal strings, out of this slice's scope — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).
- New connections are appended after the existing ones on each statement, so no existing connection moves — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).
- A wake repeated under a fact or topic hub keeps all its connections, the new ones included, because a hub repeat keeps its statement's connections by contract; its path names the machine that scopes the state ids — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).
- The beat is the transition's `on` exactly as authored, never a normalised word — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).

## Found

- The engine refuses two picks on one step (`requests multiple picks`) and a pick on a `fact` step (selection only on time, turn, barred or approached), so a wake with two `picks` cannot come from a compiled site; the two-picks case is held by an internal test on `stepShape` — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).
- The castle's `gate-1-policy` says `starts posted`, 12 steps with a `to`, and `final` on no state — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).
- Only `hail-site.readback.json` moved among the goldens: the text and folded renderings print no connections — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).

## Mistaken

- The first machine site authored a step with two picks and a pick on a `fact` step; both were refused at compile, and the site now authors only shapes the engine runs — [rpg-project#994](https://github.com/KirkDiggler/rpg-project/issues/994).
