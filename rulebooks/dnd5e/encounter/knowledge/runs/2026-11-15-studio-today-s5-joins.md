# Run — 2026-11-15 — studio today S5 the engine says which places join

processed: 2026-11-25T190427Z

Slice [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971) of story [rpg-project#966](https://github.com/KirkDiggler/rpg-project/issues/966). Base: `rpg-toolkit` `main` at [`4ff62fe5`](https://github.com/KirkDiggler/rpg-toolkit/commit/4ff62fe5).

## Shipped

- The read-back says every join between two regions under the place, in a group "Where the places join": kinds `place.join.door`, `place.join.open`, `place.join.passage`, and `place.region.unjoined` for a region in none — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).
- `encounter.FloorTopology` answers, without a live encounter, the open crossings between two regions' cells, each door's crossings and each placed prop's cells, in the input's authored frame — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).

## Decided

- An exported helper in `encounter` rather than constructing an `Encounter` in the read-back, because every castle door is a footprint door whose crossings are traced in the field's private plane, and constructing an `Encounter` needs a roster, sight and initiative the read-back has no business inventing — [plan Task 5](https://github.com/KirkDiggler/rpg-project/blob/studio-today-design/ideas/encounter-studio/plan-places-and-ways.md#task-5-the-engine-says-which-places-join).
- A new function, not a field on `PartitionFloorOutput`, because `PartitionFloor` runs on every dungeonspec compile and the door and prop measurements would cost every compile for one reader — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).
- Open floor is a crossing between two regions on the partition canvas (walls, every door closed, fixed props), not "two regions share a partition", because three regions in a row in one partition would otherwise join the two ends that never touch; every open join still shares a partition, held by a castle test — [plan Task 5 behaviour](https://github.com/KirkDiggler/rpg-project/blob/studio-today-design/ideas/encounter-studio/plan-places-and-ways.md#task-5-the-engine-says-which-places-join).
- A join carries `{concealment, <id>, hidden-by}` when its door or its carrying prop is hidden, because the issue asks the join to connect to the concealment on it and the concealment statement connects to nothing a consumer could follow from the door or prop — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).
- Feet decide a join, by the lead's ruling under design R30 on the review: a seam feet cross is `place.join.open` (its sentence says when sight is blocked there), a seam only sight crosses is `place.seam.sight` about `{seam, sight:<a>:<b>}` with role `sees`, never a join; `FloorTopology` reports each seam's `Moves` and `Sees` — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2441#discussion_r4577704565).
- A door that joins no two regions is said about the door: `place.door.within` (role `within`) or `place.door.edge` (role `edge`), and an unjoined region with an edge door names it — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2441#discussion_r4577704569).
- One passage per arrest, with the jail states that set the taken down named in the sentence, because the jail reads the arrest's cell and an arrest alone moves nobody; a region the passage both leaves and reaches is not a "from" — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).
- The arrest's "from" includes the regions of its loose area cells, so the castle's passage leaves the Gatehouse and `room-1-region` (the road cells of rows 13 and 14) — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).
- The joins group is one group under the place, about the site, rather than a group per join, so the place section stays one reading — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).

## Found

- `readback.Of` on the castle takes 206 ms; `FloorTopology` is 30 ms of it (benchmark, 32 cores, no race) — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).
- The castle compiles 24 structural walls with 7 openings, every one with a door, so it has no doorless gap; a doorless opening is `StructuralOpeningInput.Door == nil` in the compiled field — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).
- The castle's joins: seven doors, two open wings of the courtyard (Armoury, Barracks), the grate's two ways and the gate arrest; every region with cells is in one — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).

## Mistaken

- The first cut decided open floor by sight alone, so a window read as a join "for sight, hearing and feet alike" and a curtain as no join; the castle has neither, so nothing in the suite saw it until review probed both — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2441#discussion_r4577704565).
- The first cut dropped a door whose sides are one region with nothing said, and no test failed with the same-region check removed; the within and edge tests now kill that mutant — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2441#discussion_r4577704569).
- The first seizure test run failed because the arrest passage shares its path with the arrest outcome and the test found statements by path alone; the test now selects the outcome kind — [rpg-project#971](https://github.com/KirkDiggler/rpg-project/issues/971).
