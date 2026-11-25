# Run — 2026-11-25 — one way to pay for an ability: second wind is a named ability

processed: 2026-11-25T190427Z

Slice [rpg-project#1156](https://github.com/KirkDiggler/rpg-project/issues/1156) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`6c766ce1`](https://github.com/KirkDiggler/rpg-toolkit/commit/6c766ce1).

## Shipped

- `features.DescribeFromRef` asks for Second Wind with a level, a giver and the giver's pools, and `SecondWind.Definition` is one asker of it — [#1156](https://github.com/KirkDiggler/rpg-project/issues/1156).
- `features.ValidateParameters` checks the ref and values with the same contract and sentences — [#1156](https://github.com/KirkDiggler/rpg-project/issues/1156).
- Exported names added: `DescribeFromRefInput`, `DescribeFromRef`, `ValidateParametersInput`, `ValidateParameters`, `SecondWindParameters`; none differs from Design 1 — [#1156](https://github.com/KirkDiggler/rpg-project/issues/1156).

## Decided

- The giver's Ref, Label and SourceID are required and Name is not, as the issue lists — [Design 3](https://github.com/KirkDiggler/rpg-project/issues/1156).
- Empty pools are stored as nil, so a giver without pools costs the bonus action alone — [Design 2](https://github.com/KirkDiggler/rpg-project/issues/1156).
- The fighter keeps its own description sentence, set after the ask — [Design 4](https://github.com/KirkDiggler/rpg-project/issues/1156).

## Found

- All six "Rests on" facts held at 6c766ce1 — [comment](https://github.com/KirkDiggler/rpg-project/issues/1156#issuecomment-7145442422).
- Test 1's expected bytes were captured by marshalling `SecondWind.Definition` of a level 1 and a level 3 fighter named "hero" with a throwaway test on origin/main before `second_wind.go` changed; the test was deleted and the bytes pasted into the new test — [Test 1](https://github.com/KirkDiggler/rpg-project/issues/1156).
- Proof 1: `1d10` to `1d8` fails TestAFightersDescriptionIsUnchangedByteForByte and TestARingAsksForLevelOne; proof 2: lenient decode fails TestEachRefusalNamesTheThing and TestValidateParametersAgreesWithDescribeFromRef; each restored with git checkout — [proofs](https://github.com/KirkDiggler/rpg-project/issues/1156).
- features, character and combat pass before and after; the whole module suite, go vet, golangci-lint (0 issues) and race on features (9.9s) pass at the final head — [gates](https://github.com/KirkDiggler/rpg-project/issues/1156).

## Mistaken

- none observed — [#1156](https://github.com/KirkDiggler/rpg-project/issues/1156)

— cross-team agent, on behalf of KirkDiggler
