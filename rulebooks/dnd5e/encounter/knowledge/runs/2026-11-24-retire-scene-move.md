# Run — 2026-11-24 — machine story road step 1 sceneMove is retired

Slice [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133) of story [rpg-project#989](https://github.com/KirkDiggler/rpg-project/issues/989). Base: `rpg-toolkit` `main` at [`b9439d32`](https://github.com/KirkDiggler/rpg-toolkit/commit/b9439d32).

## Shipped

- `SceneMove`, its validate, `applySceneMove`, `Consequence.SceneMove` and every reader of it are gone from the engine; `SceneMoveSpec` and its compile are gone from `dungeonspec` — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- A site that writes `sceneMove:` on a conversation's outcome or a machine state's outcome is refused at load at its own line, naming `relocate` and `on: fact` — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).

## Decided

- The retired key is a `retiredSceneMove` type beside `retiredRecognize` in `retired_words.go`, so the refusal is the sentence and not "not a key this build reads" — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- The state-outcome refusal row leaves the table of FieldErrors and becomes its own subtest, since the refusal is now a decode error at a line, not a field path; the issue said to change that row to expect the sentence — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- The acknowledged `sceneMove` entry in `readback/notyet_internal_test.go` is removed: the guard passes without it, the retired field is tagged `json:"-"` — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).

## Found

- Rests on, all hold: F1 eight files exactly the listed; F2 and F3 print nothing; F4 `retiredAt` `:34`, `retiredRecognize` `:39`; F5 `contenthash_test.go:19`; F6 six lines in four files — [comment](https://github.com/KirkDiggler/rpg-project/issues/1133#issuecomment-7115047441).
- Whole suite before any change: `go test ./... -count=1 -timeout 15m` green, all twelve packages — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- Failed first: the conversation-outcome test got a "scene movement needs guards and a departure signal" refusal instead of the retired sentence; the machine-state test got "a scene move follows a conversation entering a state" instead of it; the load-neither test passed, as it should — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- Proof the refusal can fail: with `UnmarshalYAML` returning nil both new refusal tests and the state-outcomes subtest failed; restored — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- Pin `castleContentHash`: old `2b6632fdc28de12a739c5b6caace19bfeb72463ef11a9d81a2403ca726894731`, new `ef6511b879c51cd92128e754d608c5fecfd1ef54734342436a2d423a475ab3fa`; `git diff --stat origin/main -- dungeonspec/testdata/` prints nothing, the picture is byte for byte — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- Existing tests changed or removed: `TestSceneMoveSurvivesReload` removed (its subject is gone); the `state_outcomes_internal_test.go` "a scene move" case removed (same); `dungeonspec/state_outcomes_test.go` scene-move row now a subtest expecting the retired sentence; `readback/notyet_internal_test.go` `sceneMove` entry removed; the hash pin moved — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- Whole suite at the final head: green, twelve packages; `go build`, `go vet`, `golangci-lint run` clean — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
- The strict-decode message for an unknown outcome key still lists `sceneMove` among the keys (the retired key remains a decodable name) — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).

## Mistaken

none observed — [rpg-project#1133](https://github.com/KirkDiggler/rpg-project/issues/1133).
