# Run — 2026-11-10 — Gate S2: the entrance remembers

Slice [rpg-project#922](https://github.com/KirkDiggler/rpg-project/issues/922) of story [rpg-project#769](https://github.com/KirkDiggler/rpg-project/issues/769). Base: `rpg-toolkit` `main` at [`c921f229`](https://github.com/KirkDiggler/rpg-toolkit/commit/c921f229). Head: the [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358) branch `feat/769-gate-s2-entrance-remembers`.

## Shipped

- An option's `requires` is a list, all of which must hold; each item is one named requirement (`requires:<n>:<kind>`) in the Gate S1 evaluation — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- `TriggerInteraction` and the `{ interaction: { id, in: [...] } }` predicate, legal only on an option's `requires` and judged per run — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- The interaction instance: `FieldInput.Interactions`, compiled on `mind/machine` with a vocabulary of its own `on:` names, persisted as `interactions` (definition, digest, blob) beside `doors`, and a root `interactions:` key in dungeonspec — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- `Option.Advance`, stepped synchronously in Speak after the answer, writing an `interaction` beat to the witnesses when a transition fires — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).

## Decided

- A persisted single-object `requires` is refused with `ErrInvalidData` by `OptionData.UnmarshalJSON`, never read as a list of one; pre-release, runs saved before the list are started again — [plan on #922](https://github.com/KirkDiggler/rpg-project/issues/922).
- An interaction recipe refuses a `wake_in` act as well as a `pick`, because no registry wakes an interaction and the act would be authored and never happen — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- An interaction recipe requires a valid `ref`, as a behavior definition's does, because a zero ref cannot round-trip through the persisted canonical recipe — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- The dungeonspec recipe is carried as `InteractionSpec{Recipe *BehaviorValue}` rather than a bare `machine.Recipe`, so it is read exactly as `behaviorDefinitions`' `recipe:` is, unknown members refused — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- `encounter.CheckInteraction` is exported so dungeonspec refuses a bad recipe at the author's path, mirroring `CheckBehaviors` — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- An advance whose beat the current state does not listen for is a silent no-op with no beat; revisit if play shows it hides authoring holes — [plan open item (b) on #922](https://github.com/KirkDiggler/rpg-project/issues/922).
- Interactions are not site data: they ride `FieldInput` for the host's convenience but are never in the site hash, because the definition and progress are the run's — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).

## Found

- A prop or placed option with `requires` changes the site hash, because the site JSON now carries `requires` as a list; checkpoints of such content from before this change no longer resolve — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
- The session's `speak_test.go` builds an option with a single-trigger `Requires` and will need the list form at its repin — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).

## Mistaken

- The first test constant for the gate collided with an existing `frontGate` in the package's tests and was renamed before the first push — [rpg-toolkit#2358](https://github.com/KirkDiggler/rpg-toolkit/pull/2358).
