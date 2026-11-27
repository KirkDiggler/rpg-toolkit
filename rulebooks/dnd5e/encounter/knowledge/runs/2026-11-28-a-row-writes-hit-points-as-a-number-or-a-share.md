# Run — 2026-11-28 — NPC behaviour 4b-2 a row writes hit points as a number, or as a share with %

Slice [rpg-project#1248](https://github.com/KirkDiggler/rpg-project/issues/1248) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`676e70da`](https://github.com/KirkDiggler/rpg-toolkit/commit/676e70da).

## Shipped

- `hit_points: { at_most: 8 }` is a number of hit points and `{ at_most: 30% }` a share of the maximum: `HitPointsSpec.Percent`, compiled to `behavior.HitPointsWhen{AtMost, Percent}`, saved as `HitPointsData{at_most, percent}` (left out when false), said beside the row and by the site check — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).
- The pin `mind/behavior` `v0.27.0` to `v0.28.0` — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).

## Decided

- `at_most` is read from the scalar's text: ASCII digits only is hit points, 1 or more; digits then `%` is a whole percent 1 to 100; `+5`, `-5`, `30 %`, `%`, `30%%`, `12.5`, `0x10`, `1e3`, a list, a mapping and a number no int holds are refused with the "write a whole number" sentence, and quoting does not change the reading — [comment](https://github.com/KirkDiggler/rpg-project/issues/1248#issuecomment-7229270563).
- `HitPointsData.Percent` is tagged `percent,omitempty,omitzero`, because behaviour blobs are marshalled with `encoding/json/v2`, where `omitempty` does not leave a false bool out — [comment](https://github.com/KirkDiggler/rpg-project/issues/1248#issuecomment-7229270563).
- A list or mapping value is quoted back in flow style on one line (`[30]`, `{n: 30}`), the yaml encoder's spacing — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).
- `README.md`, `AGENTS.md` and `knowledge/decisions.md` do not describe the word, so none is edited — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).

## Found

- F1 to F4 hold at `676e70da`; the encounter is unchanged since `f87b7e0d` — [comment](https://github.com/KirkDiggler/rpg-project/issues/1248#issuecomment-7229270563).
- F5: at the pin alone the suite failed in five tests, all this word's own, which wrote a share as a bare number (four of the kind the issue named); `TestTheCastlesContentHashIsPinned` did not fail, so the constant stays — [comment](https://github.com/KirkDiggler/rpg-project/issues/1248#issuecomment-7229270563).
- Every `HitPoints` hit is accounted for in the first comment: the two numbers a host tells do not change — [comment](https://github.com/KirkDiggler/rpg-project/issues/1248#issuecomment-7229270563).
- `mind/behavior` `v0.28.0` tags `Percent bool` `json:",omitempty"`, which the encounter's v2 marshalling writes as `"Percent":false` for a number of hit points in a saved table; the round trip is right, and the module is not this PR's — [comment](https://github.com/KirkDiggler/rpg-project/issues/1248#issuecomment-7229270563).
- A fifth test of this word's own, `dungeonspec/readback/hit_points_internal_test.go`, is outside the issue's four files — [gap](https://github.com/KirkDiggler/rpg-project/issues/1248#issuecomment-7229343890).
- Refusals, each after `line N: `: `hit_points: { at_most: 0 }`: a number of hit points is 1 or more; `hit_points: { at_most: 101% }`: a share of the creature's own maximum is a whole percent from 1% to 100%; `hit_points: { at_most: 12.5 }`: write a whole number of hit points (`8`) or a whole percent of the creature's own maximum (`30%`) — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).
- Sentences: "if Goblin (gate-patrol)'s hit points are at most 8, " and "hit points at most 8" for a number; "if Goblin (gate-patrol)'s hit points are at most 30% of its maximum, " and "hit points at most 30%" for a share; the record says `hit points at most 8` and `hit points at most 30%` — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).
- Proofs: (a) reading `30%` as a number fails the compile, same-digits and picture tests and the site-check share case, and the turn tests cannot fail (they build their tables from Go values); (b) not carrying `Percent` in the compile fails the same set; (c) not saving `Percent` fails the WhenData round trip; (d) saying "of its maximum" for a number fails the site-check number case; (e) accepting `30 %` fails the refusal case `a space before the percent` — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).

## Mistaken

- The first refusal test expected `{ n: 30 }` for a mapping and the yaml encoder writes `{n: 30}`; caught by the first run — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).
- The first draft of the refusal sentence built a format string with a `%` in it; `go vet` caught it before any test ran — [slice](https://github.com/KirkDiggler/rpg-project/issues/1248).
