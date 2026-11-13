# Run — 2026-11-09 — the record appended a segment at a time

processed: 2026-11-12T165555Z

Base: `rpg-toolkit` `main` at ba6d40f2 (record v0.3.0). Story: [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879), slice [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897) leg (c), PR 1 of 4. PR: [#2352](https://github.com/KirkDiggler/rpg-toolkit/pull/2352). Consumer: [#2353](https://github.com/KirkDiggler/rpg-toolkit/pull/2353), encounter `ToCheckpoint`.

## Shipped

- `(*Log).DataFrom(&DataFromInput{Since})` writes the retained entries at or after Since as a legal `LogData` with its own audience table, first-seen over its own entries. NextSeq is the Log's. LastID is written as ToData writes it, relative to the result's own entries. Older entries are never copied. At Since = NextSeq it is the head: NextSeq and LastID, no entries.
- `ToData()` is DataFrom at Since 0, one body behind both.
- `JoinData(&JoinDataInput{Head, Floor, Segments})` drops every entry below Floor, re-interns the kept rows into one first-seen table, and returns exactly what ToData wrote at the save. It refuses by name, wrapping `ErrInvalidData`: a head with entries or a table, a Floor past the head's NextSeq, a malformed segment, a seq missing at or above Floor, an overlap at or above Floor, a head NextSeq or LastID that disagrees with the last segment.
- `EntryData` and the stored shape are unchanged. A segment loads on its own.
- The package doc states the segment, the head, and that a store trims below the floor while the join tolerates what it dropped.

## Decided

- **Each segment carries its own table**, because the table is rebuilt on every ToData and any TrimBefore renumbers it. A distinct audience then repeats once per segment, not per entry.
- **The join takes a Floor**, because the encounter trims to its retention window on every save. A verb that appends more than the window starts its segment above the store's last seq + 1, and a straddling segment holds entries the Log has trimmed. Below Floor the join drops and forgives; at or above it every seq must be present. Review of #2352 found both by probe.
- **The store keeps the Floor**, passed in, rather than a new LogData field, because the stored shape stays unchanged and the encounter reports the floor from ToCheckpoint.
- **The join equals ToData exactly**, a first-seen table over the joined entries, because then the property tests pin deep equality, stronger than load equality.
- **Empty segments are skipped**, because a beatless verb saves one.
- **Joined entries share Tags and Payload with the segments**, documented, because the api hands the join straight to LoadLog, which deep-copies.
- **Input structs**, `DataFromInput` and `JoinDataInput`, because every verb in the package takes one; nil is `ErrNilInput`.

## Found

- The first shape, a join with no floor, passed its own property test because that test never trimmed. The review's probe, a window smaller than one save's appends, refused the next save as a gap and brought trimmed entries back.

## Verification

| test | pins |
|---|---|
| `TestSavesJoinBackToToData` | 200 seeds, saves with and without beats and minted ids, no trim: join equals ToData, load equal |
| `TestSavesThatTrimJoinBackToToData` | 200 seeds, window 1 to 6, up to 11 appends per save, store drops segments wholly below the floor: join at the floor equals ToData, load equal |
| `TestASegmentStartingAboveTheStoreJoinsAtTheFloor` | the review's probe: a gap with no floor, ToData at the floor |
| `TestFloorRefusesByName` | floor past NextSeq, gap above the floor, everything stored below a floor short of NextSeq, overlap above the floor; overlaps wholly below it are dropped |
| `TestJoinRefusesByName` | ten malformed heads and segments |
| `TestSplitAtEverySeq`, `TestTrimmedHead`, `TestFreshAndEmptySegments`, `TestToDataIsDataFromZero`, `TestNilInputs` | edges |

Mutation: with the floor forced to 1, `TestSavesThatTrimJoinBackToToData` fails.

## Commands

```
cd play/record
go test -race -count=1 ./...     ok  1.593s
golangci-lint run ./...          0 issues.
```
