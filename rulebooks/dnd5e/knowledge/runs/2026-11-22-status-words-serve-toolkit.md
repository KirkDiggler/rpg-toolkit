# Run — 2026-11-22 — status effect powers: the words serve, toolkit slice

processed: 2026-11-25T190427Z

Slice [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095) of story [rpg-project#1036](https://github.com/KirkDiggler/rpg-project/issues/1036). Base: `rpg-toolkit` `main` at [`73a6f7cb`](https://github.com/KirkDiggler/rpg-toolkit/commit/73a6f7cb).

One line per entry. Every line ends with a link.

## Shipped

- `status.Words` carries `WrittenIn`, `Holds`, `Required`, `Narrows`, `When`, `Bounds` (and `Value.When`) from the one private table — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- `status.Refusal` (`Path`, `Says`) replaces every `fmt.Errorf("%w: ...", ErrBadStatus, ...)` in `compile.go`; `Error()` is unchanged byte for byte and `errors.Is(err, ErrBadStatus)` holds — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- `status.Say` says a whole status from the table's labels, with a `Parts` row per change, act, rule and `again` — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).

## Decided

- `Narrows` is computed as the `when` word of the change place (no extra table field), because the issue names `when` as the only narrowing word — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- `Refusal.Unwrap()` returns `[]error` so the engine's own refusal stays findable by `errors.Is` where the status compiled to a definition the engine refused; the issue says `Unwrap()` returns `ErrBadStatus`, which it still does — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- A refusal about the whole status (no input, bad id, `until.rounds`, definition refused, not an object or a repeated key) has an empty `Path`; an unknown top-level word has the word as its `Path` — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- `Say` orders the `when` labels by the table's order of `when` words, which is also alphabetical today — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- `TestSayUsesOnlyTheTablesLabels` accepts a part's text with its first letter upper-cased inside the sentence, because the sentence upper-cases the first letter of a sentence; and one extra test, `TestSayJoinsWhensAndLists`, pins the joins the castle's three do not use — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).

## Found

- Facts F1 to F5 all held at `origin/main` `73a6f7cb`; F1 printed `91:func Words(_ *WordsInput) *WordsOutput {`, F2 `numberBounds`/`changeWhen`/`when   []string`, F3 no matches, F4 the eight suite tests, F5 `rulebooks/dnd5e/v0.225.0` — [facts comment](https://github.com/KirkDiggler/rpg-project/issues/1095#issuecomment-7069596146).
- `go test ./status/ -count=1` before any change: ok in 0.040s; after: ok in 0.047s — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- New and changed tests each run under 0.02s: `TestWordsTellAFormWhatEachTakes` 0.00s, `TestSayTheCastlesStatuses` 0.00s, `TestSayUsesOnlyTheTablesLabels` 0.01s, `TestSayRefusesWhatCompileRefuses` 0.00s — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- The session module's `Status|Content` tests pass against this branch through a local `replace` (not committed), 15.8s — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
- The session module pins `rulebooks/dnd5e v0.224.0` while the latest tag is `v0.225.0` — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).

## Mistaken

- The first `TestSayUsesOnlyTheTablesLabels` required each part's text verbatim in the sentence and failed on the upper-cased first letter; the test now allows it — [rpg-project#1095](https://github.com/KirkDiggler/rpg-project/issues/1095).
