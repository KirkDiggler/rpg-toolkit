# Run — 2026-11-28 — NPC behaviour 3b-3 a driven turn activates an ability its sheet can pay for

Slice [rpg-project#1235](https://github.com/KirkDiggler/rpg-project/issues/1235) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`7f71c972`](https://github.com/KirkDiggler/rpg-toolkit/commit/7f71c972).

## Shipped

- `SheetFacts.Abilities` (`AbilityFacts`: word, name, payable), refused at the door when a word or name is empty or a word repeats — [issue Design 1](https://github.com/KirkDiggler/rpg-project/issues/1235).
- `MonsterView.Abilities` (`AbilityView`), `facts.CanActivate`, the fifth intent `Activate` (not terminal), the optional `Activator` seam wired at construction, load and rebind, and `executeTurnIntent`'s `Activate` arm with four told refusals (cause `encounter:intent:activate`) — [issue Design 2 to 7](https://github.com/KirkDiggler/rpg-project/issues/1235).
- The anti-spin bound grows by the abilities the sheet lists; the `answered` beat names `ability` for an `activate` row; the site check's stat-block sentence ends "A behaviour's row uses one with `activate`."; the decision record's example sentence is corrected — [issue Design 8 to 12](https://github.com/KirkDiggler/rpg-project/issues/1235).
- The shared test host (`internal/turntest`) learns abilities (`Give`, `Uses`, `Activate`, `WithActivator`, `FreeActivations`, `FailActivate`); `With` and `Wrap` do not wire it — [issue Design 10](https://github.com/KirkDiggler/rpg-project/issues/1235).

## Decided

- `intentBound` is unchanged and the caller adds `len(sheet.Abilities)`, the smaller change; the world-clock caller (`machine.go`) needs none because nothing is payable there — [issue Design 8](https://github.com/KirkDiggler/rpg-project/issues/1235).
- `turntest.WithActivator(caps, host)` is a function taking the capabilities, as `With` and `Wrap` return them, not a method with no arguments — [issue Design 10](https://github.com/KirkDiggler/rpg-project/issues/1235).
- The host's activation sends the smallest input `RecordActivation` takes: an actor and an ability ref (`test:abilities:<word>`) and name, no target, no result — [issue Design 10](https://github.com/KirkDiggler/rpg-project/issues/1235).
- A test that needs a turn to START without a bonus action uses `AllowPerTurn(member, 1, 0)`, because a hand-set `Leave` is wiped when the member's turn starts — [issue Test 3](https://github.com/KirkDiggler/rpg-project/issues/1235).
- Test 1's second order is scripted high, low, low (the swing, then the ability), since the last candidate after the swing is the walk row — [gap](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227482814).

## Found

- F1 to F8 hold at `7f71c972`; F9 found no test pinning a `MonsterView`'s or `SheetFacts`' field list, and found two that pin the capabilities (`rebind_internal_test.go`, `standins_internal_test.go`), each edited by one line and a no-op type — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227475897), [gap](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227482430).
- F10: 29 test methods in 23 test files implement `Sheets`; none edited — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227475897).
- Every switch over an intent: `intentVerb` (`act.go`), `intentTarget` (`answer.go`), `executeTurnIntent` (`clocks.go`); every `Dasher` hit has an `Activator` beside it (`capabilities.go`, `encounter.go` field and construction, `data.go` load, `rebind.go`) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227475897).
- At the bound today the loop ends silently and the turn ends through `endDrivenTurn`; with a host that never charges (`FreeActivations`), a creature that sits six cells of speed away asks 15 times (2 + 2×6 + 1 ability) and then the turn ends, with 15 `activated` beats and a `turn-ended` beat; nothing hangs (Test 9) — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227475897).
- After the swing is spent `enemy: reach` is off and `enemy: walk` holds, so the Lieutenant's table adds a third pick, `toward`, with a stayed beat `already in reach` ending the turn — [gap](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227482814).
- Test 1, first die (always the first candidate): `answered activate (ability second wind)`, `activated` (Lieutenant, Second Wind), `answered attack`, the swing, `answered toward`. Second die (high, low, low): `answered attack`, the swing, `answered activate`, `activated`, `answered toward`. Both leave the bonus action, the action and the use at zero — [Test 1](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/encounter/activate_turn_test.go).
- The proofs, each on the committed tree, then restored: (a) not clearing `Payable` on the world clock fails Test 3(e) ("a round of the world pays for no ability"); (b) not clearing it when no `Activator` is wired fails Test 3(d) on the candidate assertion, and no nil dereference occurs, the executor's backstop would have told it; (c) the `intentFor` case below the line-row case fails Test 4 (no activation); (d) `done` true after a successful ask fails Test 1 in both orders and Test 9; (e) a bound not grown is noticed by Test 9 alone (14 asks, not 15), Test 1 does not notice — [issue Prove the tests bite](https://github.com/KirkDiggler/rpg-project/issues/1235).
- The castle's content hash pin passes unedited — [issue F9b](https://github.com/KirkDiggler/rpg-project/issues/1235).

## Mistaken

- I wrote the type surface, the table arms and the executor before the tests, against the issue's "tests first"; the tests were then shown to bite by running them under `origin/main`'s `clocks.go` and by the five proofs — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227475897).
- I edited the two capability tests before posting the first comment that was to list them; nothing was committed before it was posted — [gap](https://github.com/KirkDiggler/rpg-project/issues/1235#issuecomment-7227482430).
