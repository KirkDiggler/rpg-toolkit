# Run — 2026-11-09 — an unchanged member writes the same bytes

processed: 2026-11-12T165555Z

Leg (b) of [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897), PR 1 of 4, for story [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879). Base: `rpg-toolkit` `main` at [`1e8f93a8`](https://github.com/KirkDiggler/rpg-toolkit/commit/1e8f93a8).

## Shipped

- `BehaviorData.MarshalJSON` encodes through `encoding/json/v2` with `json.Deterministic(true)`. A recipe's `states` and `table` maps come out in sorted key order, so an unchanged member writes the same bytes on every save. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- `InteractionData.MarshalJSON` takes the same option. Its blob's bag is a map and had the same fault. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- `TestSnapshot_ByteIdenticalWithBehaviors` paces a walker on the small keep's garrison until the world clock raises, then encodes `ToData`, `ToCheckpoint`'s rest and every member four times each through `encoding/json` v1, as a host does. Every encode must be the first one's bytes. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- `TestInteractionData_ByteIdentical` encodes a record with a twelve-key bag four times. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

## Decided

- The option goes on the two `MarshalJSON` methods only. They are the only v2 encodes whose bytes reach storage. `behaviorRuntime.toData` and `newBehaviorRuntime`'s first round trip decode straight back into Go values, so their key order is never seen, and the definition digest already encodes deterministically. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- Four encodes, not two. Two encodes of a small map can agree by chance. Four never let the unfixed code pass in three runs. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- The test uses the small keep's garrison, not the 10k one. It has 57 members with behaviors, which is enough to see the fault every time, and it stays under the 5 s budget under `-race`. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

## Found

```
go test -race -count=1 -run 'TestSnapshot_ByteIdenticalWithBehaviors|TestInteractionData_ByteIdentical' -v .
```

| run, small keep garrison, 61 members, 57 with behaviors | members whose bytes differ across encodes |
|---|---|
| before the fix, eight encodes | 51, then 45 |
| before the fix, four encodes, three runs | 40, 38, 40 |
| after the fix | 0 |

- The snapshot's own promise, "an unchanged encounter must produce byte-identical snapshots", did not hold on any keep with behaviors. Sorting members by id was not enough. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- The test takes 4.6 s under `-race` when run alone. 3.75 s of that is compiling and opening the small keep, which a full run pays once per binary. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

## Mistaken

- The first draft took a fresh `ToData` for each member's encode. It cost 14 s under `-race`. One snapshot per encode round, read member by member, asks the same question in 1 s. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

## Verification

- Mutation: with the two `MarshalJSON` changes reverted, `TestSnapshot_ByteIdenticalWithBehaviors` fails in three of three runs, and `TestInteractionData_ByteIdentical` fails in three of three. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- `go vet` and `gofmt` are clean, and `go test ./...` passes for the module. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
