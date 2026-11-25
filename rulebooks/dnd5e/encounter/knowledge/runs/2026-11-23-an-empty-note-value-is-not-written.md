# Run — 2026-11-23 — a note has a value, an empty value is not written

processed: 2026-11-25T190427Z

Slice [rpg-project#1118](https://github.com/KirkDiggler/rpg-project/issues/1118) of story [rpg-project#1007](https://github.com/KirkDiggler/rpg-project/issues/1007). Base: `rpg-toolkit` `main` at [`d11889e6`](https://github.com/KirkDiggler/rpg-toolkit/commit/d11889e6).

## Shipped

- `TriggerRecognized.As` carries `json:"As,omitempty"`; the castle's committed picture is regenerated and equals the one at `v0.245.0` — [rpg-project#1118](https://github.com/KirkDiggler/rpg-project/issues/1118).

## Decided

- The key keeps the Go name `As`, as `Topic` and `Outcomes` do; the content hash ignores JSON tags so the pin does not move — [rpg-project#1118](https://github.com/KirkDiggler/rpg-project/issues/1118).

## Found

- Rests on, all hold: F1 `As string` untagged at recognition.go:120; F2 `json:"Outcomes,omitempty"` at compile.go:334; F3 26 `"As":""` at origin/main and 0 `"As"` at v0.245.0; F4 pin `2b6632fd…` at contenthash_test.go:19; F5 `CONTENT_GOLDEN=write` at contentgolden_test.go:253 — [comment](https://github.com/KirkDiggler/rpg-project/issues/1118#issuecomment-7090218199).
- Step 4: `git diff --stat rulebooks/dnd5e/encounter/v0.245.0 -- <castle picture>` prints nothing — [rpg-project#1118](https://github.com/KirkDiggler/rpg-project/issues/1118).
- Whole module suite before the change: all ok; at the final head: all ok, hash pin untouched, golangci-lint 0 issues — [rpg-project#1118](https://github.com/KirkDiggler/rpg-project/issues/1118).

## Mistaken

- The tag was missed in slice 1 and the picture was regenerated instead of fixed; the new test `TestDungeonspec_NoteValue_AnEmptyValueIsNotWritten` now pins it, written first and seen failing — [rpg-project#1118](https://github.com/KirkDiggler/rpg-project/issues/1118).
