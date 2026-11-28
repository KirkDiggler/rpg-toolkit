# Run — 2026-11-28 — Courtyard S7 host material-channel projection

Slice [rpg-project#1192](https://github.com/KirkDiggler/rpg-project/issues/1192) of [Courtyard#867](https://github.com/KirkDiggler/rpg-project/issues/867).

## Shipped

- Session-owned channel DTOs now follow static wall/prop/frame projection and observed NPC roster identity; launch carries native intent to active/reserved members. API/Web delivery remains unfinished — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).
- Full session tests, race tests and lint passed on the real pushed Encounter4b9aee80 dependency; the release-tag repin remains a later merge requirement — [Encounter#2618](https://github.com/KirkDiggler/rpg-toolkit/pull/2618).

## Decided

- Delegate canonical JSON presence/field validation to its owner, then copy into host-owned types; no encounter type crosses the public seam and no asset/resource or lighting rule is introduced — [approved design](https://github.com/KirkDiggler/rpg-project/pull/1189).
- Roster channel intent comes only from the caller's observed identity. A test retains Alice's strength1 while Bob sees2 after reload; legacy absence and never-seen identity are not enriched — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).

## Found

- The full race run took590.142s with GOMAXPROCS4 on the shared host, close to the default package timeout; it passed without raising timeouts or weakening tests, but the measured margin is small — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).

## Mistaken

- The first full run exposed exact public-field whitelist expectations for walls/frames; adding only MaterialChannels preserves the guards against private state/parent identity rather than loosening them — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).
