# Run — 2026-11-04 — Effects construction API cleanup

Prerequisite [rpg-toolkit#2275](https://github.com/KirkDiggler/rpg-toolkit/issues/2275) for [Keep equipment S1, rpg-project#862](https://github.com/KirkDiggler/rpg-project/issues/862). Base: toolkit main [3182088c](https://github.com/KirkDiggler/rpg-toolkit/commit/3182088c4cf0898596aa0c0f4eafe50bc5cc6d21).

## Shipped

- No merge/release; the provider checkpoint replaces positional construction with `effects.New(effects.Config)` and `machine.New(machine.Config)`, including callers, examples and documentation — [construction guide](../../README.md#constructing-an-instance).

## Decided

- KirkDiggler requested a named Compose input after reading the item probe, because seven positional arguments obscure their meanings; the same issue in five-argument machine.New is included, while simple accessors/callbacks and return types remain unchanged — [scope and checked plan](https://github.com/KirkDiggler/rpg-toolkit/issues/2275).
- Preserve validation timing, default state/bag behavior and bag/source ownership rather than mix ergonomic cleanup with lifecycle redesign; value input structs introduce no new nil-input case — [input contracts](../../composed.go).

- KirkDiggler preferred idiomatic package-qualified `New(Config)` over the first `ComposeInput`/`NewInput` cut or stuttering `NewMachine(MachineConfig)`; configuration is named by its package, while input/output naming remains for operations — [cleanup record](https://github.com/KirkDiggler/rpg-toolkit/issues/2275).

## Found

- The pre-change provider race suite passes; migrating callers first fails compilation on the absent input structs/signatures, and the implemented migration restores both package suites — [constructor callers](../../composed_test.go).
- Dropping State when forwarding to machine.New makes the existing byte-equal reload check fail (raging becomes fresh); restoring forwarding passes — [round-trip assertion](../../composed_test.go).
- The machine.New supplied bag stays uncopied and its nil-bag default is a fresh clone; unknown/terminal state refusal remains on Apply — [machine.Config](../../machine/machine.go).

## Mistaken

- No accidental behavior change observed; the failed compile and dropped-State failure are deliberate verification steps, not provider defects — [verification scope](https://github.com/KirkDiggler/rpg-toolkit/issues/2275).
