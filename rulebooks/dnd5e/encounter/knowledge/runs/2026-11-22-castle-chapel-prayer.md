# Run — 2026-11-22 — status effects on a player: castle chapel prayer

processed: 2026-11-25T190427Z

Slice [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099) of story [rpg-project#1036](https://github.com/KirkDiggler/rpg-project/issues/1036). Base: `rpg-toolkit` `main` at [`7e19fae4`](https://github.com/KirkDiggler/rpg-toolkit/commit/7e19fae4).

One line per entry. Every line ends with a link.

## Shipped

- The castle declares the status `prayed` (advantage on ability checks, until the run ends, `again: ignored`) and the chapel altar gains a plain `pray` option that gives it to the speaker — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).
- Castle tests `TestMadKingThePrayerLeavesTheSpeakerPrayed` (1.76s under -race) and `TestMadKingThePrayerIsGivenOnceToEachWhoPrays` (1.76s under -race), and a read-back test `TestTheAltarsPrayerStatusIsSaid` — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).

## Decided

- The benefit hangs on the altar because the castle has no priest; one plain line stands in for the design's undecided prayer puzzle — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).
- Each player prays for themselves, because a thing's row gives only to the speaker — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).
- Existing tests edited: `castleContentHash` re-pinned `23df4760...` to `4d35d5b8...` (the castle changed); `TestDungeonspec_AThingToDoCompiles` now expects the altar's two options (the altar has a second option) — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).

## Found

- The castle's compiled golden `world-builder-v4-castle.compiled.json` does not move: regenerating it with `CONTENT_GOLDEN=write` changes no byte, because it records the world's cells and bodies, not a prop's options or the statuses — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).
- The issue's race-run pattern `KitchenRewards` matches no test; the kitchen tests are `TestMadKingHelpingTheCookLeavesTheSpeakerWarmedUp` (2.40s) and `TestMadKingTastingTheFeastLeavesTheSpeakerFeasted` (1.93s) — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).
- A member's position is an offset cell while a placed prop's compiled cell is dungeon-absolute: the altar's absolute (26, -12) is offset (20, -12) — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).

## Mistaken

- The first draft of the test read the altar's absolute cell as an offset cell and failed with "target out of range"; the offset was found by inverting `castleCell` — [rpg-project#1099](https://github.com/KirkDiggler/rpg-project/issues/1099).
