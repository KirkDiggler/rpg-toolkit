# Run — 2026-11-16 — studio today S13 the engine says the ways

Slice [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981) of story [rpg-project#966](https://github.com/KirkDiggler/rpg-project/issues/966). Base: `rpg-toolkit` `main` at [`11384cd3`](https://github.com/KirkDiggler/rpg-toolkit/commit/11384cd3).

## Shipped

- The read-back says every way through every join, right after its join: kinds `place.way.free`, `.lock`, `.line`, `.fight`, `.search`, `.told`, `.passage`, `.none`, about `{way, way:<join id>:<carrier>}`, each connected `through` its join, `by` its carrier, `needs`, `on-miss` and `{method, …}` — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
- Both folds say a join once with its ways beneath it — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).

## Decided

- The chain stops at join → bar → lift, as the lead scoped it: a line way is a line whose win (or plain saying) grants a topic a holding post's pass asks, through a row's recognition or its conversation's outcome, or teaches a fact every holder's routine leaves its posted states on; who the words reach and who witnesses are said as play's, not simulated — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
- `stand-aside` is a way through the gate: its 3-in-4 win teaches `gate-cowed`, and all three holders' routines (gate-1, gate-2, captain) leave every posted state on it; the sentence says a holder its words do not reach keeps holding. A line that draws off only some holders says it is not a way through on its own, and every way carries `{creature, <id>, draws-off}` per holder it lifts — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
- A requirement that something NOT hold is a bar said in the sentence, never a `needs` connection — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
- Bars, by the lead's ruling on the review: a join is barred by `lock`, `hold` and `hidden`, each way lifts one (`{bar, …, lifts}`), a passage's carrier lifts none; a line lifts the hold only when its grant reaches every holder and meets each holder's whole pass, knowing or carrying a pass's facts and props is `place.way.known`, and what lifts some holders only is `place.way.partial` with `still-holds`; the arrest's passage is carried by the lines that step the conversation into its jail (`come-quietly`), with no way when there is no jail; a line's id in a way is `<carrier>/<option>` — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2443#pullrequestreview-5913211546).
- A way is built without hub registration: its needs are its carrier's requirements, already in the hubs — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
- The DC is structured, by the lead's decision on the PR: `Connection` gains `Value int` (`value,omitempty`), set only on a way's `method` connections from a check — each alternative its own DC, a lock approach's ability and tool both the approach's; no other statement carries a value, so no golden moved — [rpg-toolkit#2443](https://github.com/KirkDiggler/rpg-toolkit/pull/2443).

## Found

- `place.way.none` is unreachable for a site that compiles: a concealment must list a check, a lock an approach, a held door always has its fight, and a door or floor with no bar is free; the kind is kept as the guard for "nothing skipped silently" — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
- The castle says 27 ways over 12 joins; `readback.Of` on the castle was 184 ms at main and 174 ms now (noise); the statements' JSON grew from 388,919 to 414,748 bytes — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
- The captain's `turn` (Persuasion 15, needs `king-burned-village`) is also a way through the gate: `guards-turned` draws all three holders off — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).

## Mistaken

- The closure read found `knowWays` letting a holder with no pass through (the engine's `passLets` says no) and both judges ignoring a pass's `not`; one `passJudge` now serves both, an empty pass is not met, a `not` is said and connected `unless`, and probe m6 and the `not` case are site tests whose mutants fail — [closure](https://github.com/KirkDiggler/rpg-toolkit/pull/2443#pullrequestreview-5913268078).

- Review of rpg-toolkit#2443 found the hold judged against a union of topics, not the engine's per-holder pass: a line on a creature holding no post, a pass on a password fact, and a pass needing a fact beside `admitted` were all said wrongly; a partial draw-off was counted as a way; a locked and held door's lock was said as a way through on its own; and the arrest's way named the arrest, not the `come-quietly` lines whose jail moves the taken. The castle's own list was right, which is why nothing caught it: every probe is now a small-site test and mutants A (any checked line is a way), B (any fact draws off) and C (partial as a way) each fail a way test — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2443#pullrequestreview-5913211546).

- The first cut put fold texts through `self()`, which capitalises, so a way read "it is Offered while"; mid-sentence substitution is now its own helper — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
- The ways were written before their tests; the named tests were then seen failing with the ways switched off (12 of 14) before being kept — [rpg-project#981](https://github.com/KirkDiggler/rpg-project/issues/981).
