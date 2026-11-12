# Run — 2026-11-09 — the checkpoint appends the record from Since

processed: 2026-11-12T165555Z

Slice [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897) leg (c), PR 2 of 4, of story [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879). Base: `rpg-toolkit` `main` at `09147992`. Head: the [rpg-toolkit#2353](https://github.com/KirkDiggler/rpg-toolkit/pull/2353) branch `feat/encounter-checkpoint-since`. Provider: `play/record` v0.4.0 ([rpg-toolkit#2352](https://github.com/KirkDiggler/rpg-toolkit/pull/2352), main `7dd61222`). Keep: the small keep of [rpg-toolkit#2339](https://github.com/KirkDiggler/rpg-toolkit/pull/2339), its party, no garrison. Machine: 32 threads.

## Shipped

- `ToCheckpoint(&ToCheckpointInput{Since})` returns `ToCheckpointOutput{Rest, Appended, Floor}`. — [rpg-toolkit#2353](https://github.com/KirkDiggler/rpg-toolkit/pull/2353)
- Rest is ToData with the story log replaced by its head: NextSeq and LastID, no entries, no table. A rest alone decodes as a legal trimmed-empty log, so the doc tells the host to refuse a rest without its segments by name. That refusal is the api leg's job.
- Appended is the story's segment from Since, `record.Log.DataFrom`. `snapshot` takes its log, so entries older than Since are never copied.
- Floor is the story's floor after this save's trim: the oldest retained seq, NextSeq when it retains none. The store trims its segments to it and passes it to `record.JoinData` on load.
- Since past NextSeq is refused with the new `ErrBadSince`, checked before retention runs. Appended is taken after the trim.
- ToData and WorldView are unchanged in meaning. The encounter keeps no record of what a store holds.

## Decided

- **Since is an argument, never a field**, because what a store holds is a fact about the session's fenced checkpoint, not about the encounter.
- **Floor is the floor after the trim in every retention mode**, because it is what the store trims to and what the join tolerates. Under RetentionUnbounded it stays at the first seq and trims nothing.
- **Since is checked before the trim, Appended taken after it**, because a refused call must change nothing, and a segment must never carry an entry the trim dropped. Retention never moves NextSeq, so validity is the same on either side.
- **`ErrBadSince` is new**, because no sentinel named a store claiming beats the encounter never wrote.
- **One snapshot body taking its log**, because ToData, WorldView and ToCheckpoint must not drift.
- **The tests use the small keep's party without its garrison**, because with the garrison the main test took 5.9 s under race, over the 5 s budget, and the record needs beats, not monsters.

## Found

- The first record shape had no trim floor. Review of #2352 showed a bounded window makes a segment start above the store's last seq + 1, and a straddling segment brings trimmed entries back. The record join took a Floor and this PR re-pinned to it; Floor here changed from "zero when unbounded" to the story's floor after the trim.
- A load is not ToData-stable by itself, so the load check compares a load of the join against a load of ToData, not against ToData.

## Mistaken

- At `ca3183f5` go.sum held lines for the replaced record pseudo-versions, untidy, and the `rest` shard failed on it. The v0.4.0 re-pin at `655b907d` ran `go mod tidy`; a second tidy shows no diff.
- The first bounded test compared whole-record load equality only after the last save, and the unbounded test's final comparison ran after three unsaved steps. Both now compare after every save.
- The Since refusal test ran unbounded, so it could not see a trim: a mutant that checked Since after the trim survived. `TestCheckpointRefusalTrimsNothing` runs a window smaller than the beats appended.
- A stray `// ` sat mid-sentence in the ToCheckpoint doc.

## Verification

| test | pins | -race |
|---|---|---|
| `TestCheckpointJoinsBackToToData` | saves after every few verbs, through JSON: the join equals ToData after each save; a load of the join writes what a load of ToData writes | 1.57 s |
| `TestCheckpointBoundedReportsTheFloor` | window 4, smaller than one save's beats: Floor is the oldest retained seq, a segment jumps past the store's last seq + 1, the join at the floor equals ToData after every save | 0.57 s |
| `TestCheckpointRefusalTrimsNothing` | window 2: a refused Since leaves the live log untrimmed; the next save trims, and Appended starts at the floor | 0.07 s |
| `TestCheckpointSinceOneIsTheWholeRecord` | Since 1 is the whole record; the rest is ToData but for the log | 0.13 s |
| `TestCheckpointBeatlessVerbAppendsNothing` | an empty segment | 0.05 s |
| `TestCheckpointRefusesSincePastTheStory` | ErrBadSince, ErrNilInput, Since at NextSeq | 0.04 s |

## Mutation checks

| mutant | dies on |
|---|---|
| Floor forced to 0 | `TestCheckpointBoundedReportsTheFloor` |
| the Since check moved after the trim | `TestCheckpointRefusalTrimsNothing` |

## Commands

```
cd rulebooks/dnd5e/encounter
go mod tidy                      no diff
go test -race -count=1 ./...     9 packages ok
golangci-lint run ./...          0 issues.
```
