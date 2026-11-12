# Run — 2026-11-07 — alive, not lagging S9 encounter: door crossings are site data; sight testimony decoded once

processed: 2026-11-12T165555Z

Slice [rpg-project#893](https://github.com/KirkDiggler/rpg-project/issues/893) of story [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879). Base: `rpg-toolkit` `main` at [`38e27241`](https://github.com/KirkDiggler/rpg-toolkit/commit/38e27241), with main merged in at [`ec4aecbf`](https://github.com/KirkDiggler/rpg-toolkit/commit/ec4aecbf) before review closed.

## Shipped

- Door openings live on the compiled field. They are built eagerly when doors attach, never written after, and never written in place. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- A door's crossings come from the cells near its rectangle, read through the field's bucketed centres, instead of from the whole floor. The answer is the whole-floor scan's, in the same order. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- Each held sighting's decoded testimony lives on the encounter, one slot per observer, subject and channel. A slot answers while the stored bytes are unchanged. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- The sight codec's strict key walk records which keys are present, so the decode parses the payload twice instead of three times. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- Heading variants start from the testimony the pass encoded, not from decoding it back. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- `BenchmarkGarrisonLoadAtlas` measures what a verb pays to load the garrison keep and build its atlas. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)

## Decided

- Openings are built eagerly when doors attach, not lazily on a miss. A field shared across games by the site tier must only be read, and a lazy write there is a data race. The build costs about 0.5 ms for the garrison's 36 doors. — [review on rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318), [R3](https://github.com/KirkDiggler/rpg-project/blob/c527af31/ideas/alive-not-lagging/design.md#where-the-instance-lives)
- Openings are keyed by door id, and every attach builds a fresh map. A door re-authored under the same id is new content, and the shallow copy the partition pass makes must not write the live field's map. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- Decoded testimony is held on the instance, not on the field, because what an observer holds changes with the game. — [R4](https://github.com/KirkDiggler/rpg-project/blob/c527af31/ideas/alive-not-lagging/design.md#law)
- A testimony slot is checked against the stored bytes, not invalidated on write. `mind/perception` writes the payloads and does not say which slots a pass rewrote, and it is another module. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- The pre-existing race was filed rather than fixed in this PR, to keep the PR to its slice. — [rpg-toolkit#2317](https://github.com/KirkDiggler/rpg-toolkit/issues/2317)

## Found

- F17's door-crossing cost is paid once per load, not on every step. A counter showed 36 calls per encounter open, one per footprint door, and none inside the timed steps. Live pays it every Move because the session loads the encounter once per verb. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- Loading the garrison keep and building its atlas fell from 155–257 ms to 37–49 ms, and from 823k to 97k allocations. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- `BenchmarkGarrisonMove` walk-step fell from about 20 ms to 11–16 ms on both keeps, and allocations fell from 408k to 180k (default) and from 368k to 146k (posts-asleep). The machine was loaded, so times are noisy. This is an upper bound for live play: the bench keeps one encounter open, so its testimony slots stay warm, while live loads once per verb and each Move starts cold. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- Raise-step changed by no measurable amount: the two rounds disagree. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- `go test -race` on the encounter package fails about one run in four. `TestTheVisibleCacheIsBounded` swaps a package variable while the background 10k keep launch reads it, and this has been on main since #2305. — [rpg-toolkit#2317](https://github.com/KirkDiggler/rpg-toolkit/issues/2317)

## Mistaken

- The first cut built openings lazily on a miss, a write after compile on a field the site tier will share. Review caught it, and the build moved to attach. — [review on rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- The first scan probes used only rectangles shorter than the scan's padding, so a box that ignored the door's reach survived mutation. A long gate leaf was added to the probes, and the mutation now fails. — [rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
- The first PR body did not say that the bench's walk-step drop is an upper bound for live play. Review asked for it, and the body and this record now say it. — [review on rpg-toolkit#2318](https://github.com/KirkDiggler/rpg-toolkit/pull/2318)
