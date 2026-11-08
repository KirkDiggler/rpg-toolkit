# Run — 2026-11-08 — seat clears ride the fenced save

Fixes [rpg-toolkit#2332](https://github.com/KirkDiggler/rpg-toolkit/issues/2332), found by the rpg-api S12a integration ([rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)) against session v0.172.0 ([#2322](https://github.com/KirkDiggler/rpg-toolkit/pull/2322)).

## Shipped

- Under the fence, the seats a verb clears are written inside its commit before the one fenced save: a closed run's players, and Exit's leaver carried on the scope. The character guards the clears take are held across the save. The plain path keeps its order, clears after the run is durable.
- A sheet or seat written after the scope's fenced save landed is refused with a `SaveError` wrapping `ErrBadRepository`, instead of being held and dropped at the guard's release.

- Also pins `play/record` v0.3.0 ([rpg-project#896](https://github.com/KirkDiggler/rpg-project/issues/896)). No session code or test reads `EntryData.Audience`; the stored entries are touched only by slicing (`record_test.go`) and by `Seq` (`conceal_test.go`).

## Decided

- The loud failure sits at the write, not the guard's release. The release is a `func()` that runs after the verb has returned its error, so a failure there could only panic or log. Refusing the write returns through the verb, names the aggregate, and is the earliest point the mistake exists.
- Clears move before the save rather than the fenced save gaining a separate clear set: a cleared seat is already a `SeatData` with an empty Session, so it rides `Seats` like a put.

## Found

- `Exit` cleared the leaver's seat after `commit`, and `commit` cleared a closed run's seats after `persist`. Both ran after the fenced save had taken the pending set, so the guard's release dropped them and the seat stayed set; relaunching the character then failed with `ErrSeatedElsewhere`.

## Evidence

- `TestAnExitUnderTheFenceClearsTheLeaversSeat` and `TestARunClosingUnderTheFenceClearsEverySeat` fail against main's `write.go` and pass here; `TestAStaleExitKeepsTheSeat` and `TestAWriteAfterTheFencedSaveIsRefused` cover the stale and late-write cases.
- `go test -race -count=1 ./rulebooks/dnd5e/session/...` passes, on play/record v0.3.0 too; `golangci-lint run ./rulebooks/dnd5e/session/...` reports 0 issues.
