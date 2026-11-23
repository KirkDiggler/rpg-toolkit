# Run — 2026-11-23 — a wake says whom it is about S2b the seen filter

Slice [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126) of story [rpg-project#1123](https://github.com/KirkDiggler/rpg-project/issues/1123). Base: `rpg-toolkit` `main` at [`6b302849`](https://github.com/KirkDiggler/rpg-toolkit/commit/6b302849629e68a03fa88e8ecfe1fd118b1d52d1).

## Shipped

- `seenfilter.go`: `requiredOfSeen` reads the leaves a `seen` guard requires (own leaf or under a top-level `all`; `noted:` eq true, `noted.as:` eq non-empty, `stance` eq), the member's requirement is rebuilt in `relistBehavior`, `landSightings` lands a beat only when one step's set holds, and `seenFilterOn` is the switch — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- Rests-on F1 to F9 all held at `6b302849`; F2's caller at a load is `rebuildWake` (`encounter.go:1205`, `data.go:3841`) — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1126#issuecomment-7094408975).
- The whole module suite before any change: every package `ok`; at the final head: every package `ok` — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- `go test -race -run 'SeenFilter|SeenAnyone|TopicBeat' -count=1 -v .` passes (32 top-level tests, 47.6 s total); `golangci-lint run ./...` prints `0 issues.` — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- Benchmark, `go test -run '^$' -bench 'SeenFilter' -benchtime 200x -count 3 .`, whole output below — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).

```
goos: linux
goarch: amd64
pkg: github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter
cpu: AMD Ryzen 9 7945HX with Radeon Graphics        
BenchmarkSeenFilter/requiring-a-note/filter-on-32         	     200	  21027527 ns/op	        82.00 log-entries/op	         0 seen-beats/op	17862305 B/op	   69970 allocs/op
BenchmarkSeenFilter/requiring-a-note/filter-on-32         	     200	  21121127 ns/op	        82.00 log-entries/op	         0 seen-beats/op	17862707 B/op	   69971 allocs/op
BenchmarkSeenFilter/requiring-a-note/filter-on-32         	     200	  21543297 ns/op	        82.00 log-entries/op	         0 seen-beats/op	17862672 B/op	   69971 allocs/op
BenchmarkSeenFilter/requiring-a-note/filter-off-32        	     200	  74825226 ns/op	      1837 log-entries/op	      1755 seen-beats/op	41758144 B/op	  345065 allocs/op
BenchmarkSeenFilter/requiring-a-note/filter-off-32        	     200	  74759777 ns/op	      1837 log-entries/op	      1755 seen-beats/op	41758096 B/op	  345067 allocs/op
BenchmarkSeenFilter/requiring-a-note/filter-off-32        	     200	  75256244 ns/op	      1837 log-entries/op	      1755 seen-beats/op	41758327 B/op	  345068 allocs/op
BenchmarkSeenFilter/requiring-nothing/filter-on-32        	     200	 106675412 ns/op	      3592 log-entries/op	      1755 seen-beats/op	56412785 B/op	  411755 allocs/op
BenchmarkSeenFilter/requiring-nothing/filter-on-32        	     200	 106452827 ns/op	      3592 log-entries/op	      1755 seen-beats/op	56412890 B/op	  411756 allocs/op
BenchmarkSeenFilter/requiring-nothing/filter-on-32        	     200	 107421239 ns/op	      3592 log-entries/op	      1755 seen-beats/op	56413180 B/op	  411759 allocs/op
BenchmarkSeenFilter/requiring-nothing/filter-off-32       	     200	 106609071 ns/op	      3592 log-entries/op	      1755 seen-beats/op	56412900 B/op	  411757 allocs/op
BenchmarkSeenFilter/requiring-nothing/filter-off-32       	     200	 106605996 ns/op	      3592 log-entries/op	      1755 seen-beats/op	56413145 B/op	  411758 allocs/op
BenchmarkSeenFilter/requiring-nothing/filter-off-32       	     200	 107077190 ns/op	      3592 log-entries/op	      1755 seen-beats/op	56412842 B/op	  411756 allocs/op
PASS
ok  	github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter	188.318s
```

## Decided

- Every new test failed before the change at compile (`undefined: seenFilterOn`, `seenLeaf`, `requiredOfSeen` and the kinds), since the issue names no existing symbol for them; the behaviour failures were not observed separately — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- Proof one, a leaf under `any` counted as required: `TestSeenFilterWhatAGuardRequires` (the `any` and "beside an any" rows) and `TestSeenFilterTheSamePlayBothWays` fail; the scene has a fifth guard (`g-either`, `any` of hostile or the note) for this — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- Proof two, the rebuild skipped in `relistBehavior`: `TestSeenFilterTheRequirementFollowsTheState/open_then_strict` fails, with 18 others — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- Proof three, the value of `noted.as:` ignored: `TestSeenFilterTheValueIsAsked`, `TheRequirementFollowsTheState/open_then_strict` and the edge test fail — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- The saved log may lose the declined `seen` observe entries; the same-play test compares logs by decoded payload without them and asserts the unfiltered run's extras are only those — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1126#issuecomment-7094538940), from [the gap](https://github.com/KirkDiggler/rpg-project/issues/1126#issuecomment-7094522809).
- Existing tests changed, all in `seen_anyone_internal_test.go`, as Amendment 1 specifies: `TestSeenAnyoneANonEnemyIsSeen` (its guard's step requires nothing), `TestSeenAnyoneNotHiddenBehindAFriend` (the one beat names zed; a new test beside it covers an open step), `TestSeenAnyoneAStrangerDoesNotMoveHim` and `TestSeenAnyoneEnemiesOnlyByTheWord/a_neutral_is_no_alarm` (gained in the refresh's own answer, no beat landed); the helper `saSees` now wraps a new `saSeesGained` — [the gap](https://github.com/KirkDiggler/rpg-project/issues/1126#issuecomment-7094523258).
- The `behavior_config` entries are compared as a multiset and left out of the in-order comparison — [the gap](https://github.com/KirkDiggler/rpg-project/issues/1126#issuecomment-7094709947).

## Found

- Every matched `seen` beat writes a `behavior_decision` observe entry also when no step fires: 51 of the 222 entries of one nine-move run, and 1837 log entries per benchmark iteration without the filter against 82 with it — [the gap](https://github.com/KirkDiggler/rpg-project/issues/1126#issuecomment-7094522809).
- A creature's `behavior_config` entry is written when it is first asked, so its place in the log moves with the filter — [the gap](https://github.com/KirkDiggler/rpg-project/issues/1126#issuecomment-7094709947).
- Filtered, a hall of 40 guards with a note-requiring step lands 0 `seen` beats per iteration against 1755 and costs about 21 ms against 75 ms; with steps that require nothing the two are the same, about 106.6 ms — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- The benchmark's sight toggles every guard's range, so the guards also come into each other's sight (the 1755 beats), not only the four party members — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
- The castle's compiled picture, hash pin and the shape of `EncounterData` did not move; every castle test passes unchanged — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).

## Mistaken

- The same-play test's first comparison, whole `EncounterData` marshalled, assumed equal records; the engine's declined-observe entries made it fail, and the amendment answered it — [the gap](https://github.com/KirkDiggler/rpg-project/issues/1126#issuecomment-7094522809).
- The first scene had no guard with an `any` step, so proof one did not fail the same-play test; a fifth guard was added and the proof rerun on the final tests — [rpg-project#1126](https://github.com/KirkDiggler/rpg-project/issues/1126).
