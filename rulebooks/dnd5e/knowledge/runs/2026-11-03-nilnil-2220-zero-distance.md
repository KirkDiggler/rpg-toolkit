# Run — 2026-11-03 — nilnil #2220 — ForOption zero-distance sentinel

processed: 2026-11-12T165555Z

Part of [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220). Base: `rpg-toolkit` `main` at 8622a295. Ruling on the fix shape: [the issue's plan comment](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).

## Shipped

- `CastMove.ForOption` answers `ErrZeroDistance` for an authored option that chose zero cells, instead of `(nil, nil)`; the sentinel is the package's first, documented as a signal, not a failure — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).
- Named test `TestForOptionsZeroDistanceIsASignal` pins the sentinel and that the bound option keeps its distance and leaves without the rest of the menu; Thorn Whip's in-module pin updated to the sentinel — [rpg-toolkit#2220](https://github.com/KirkDiggler/rpg-toolkit/issues/2220).

## Decided

- The sentinel is bare, not wrapped with the option: the caller (resolution's action) wraps with its own context, so a wrap here would only repeat what the branch site already knows — the #2220 ruling asks the error to carry the signal, not the log line — [ForOption](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/combat/actions/move.go).
- "Nil means an explicit zero-distance choice" is gone from the doc: under the ruling there is no nil return without an error, so the doc names the sentinel instead — [ForOption](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/combat/actions/move.go).

## Found

- The only consumer of the zero-distance branch is resolution's `action.go:195`; it adapts in the resolution PR of this wave — [the wave table](https://github.com/KirkDiggler/rpg-toolkit/issues/2220#issuecomment-6612643483).
- The in-module pin lived in the spells package (`TestThornWhipDeclaresOptionalOnHitPull`), not the actions package — the named test now lives with the type — [cast_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/spells/cast_test.go).

## Mistaken

- `rg -rn` mangled a search — ripgrep's `-r` is `--replace` and swallowed the `n`; grep habits. Re-run with `-n` — [move.go](https://github.com/KirkDiggler/rpg-toolkit/blob/main/rulebooks/dnd5e/combat/actions/move.go).