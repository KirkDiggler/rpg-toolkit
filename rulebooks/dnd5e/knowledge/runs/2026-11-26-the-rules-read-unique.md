# Run — 2026-11-26 — the item story S1 the rules read unique

Slice [rpg-project#1180](https://github.com/KirkDiggler/rpg-project/issues/1180) of story [rpg-project#850](https://github.com/KirkDiggler/rpg-project/issues/850). Base: `rpg-toolkit` `main` at [`898a28ec`](https://github.com/KirkDiggler/rpg-toolkit/commit/898a28ec).

## Shipped

- `magicitems.Description.Unique` tagged `json:"unique,omitempty"`, carried by `Clone` and the saved sheet — [rpg-project#1180](https://github.com/KirkDiggler/rpg-project/issues/1180).
- `magicitems.Compile` reads the optional word `unique` (JSON true or false) and lists the words `name, slot, unique, charges, lets` in its unknown-word refusal — [rpg-project#1180](https://github.com/KirkDiggler/rpg-project/issues/1180).

## Decided

- `unique` set to anything but JSON true or false is refused with ``item "<id>": `unique` is true or false``, the sentence the issue gave — [rpg-project#1180 Design 2](https://github.com/KirkDiggler/rpg-project/issues/1180).
- `unique: false` is the same as not writing it, so the field is omitted when false — [rpg-project#1131 ruling](https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7182695656).

## Found

- F3: the one non-test place that builds a `Description` is `compile.go:90`, which sets only `Ref` then fills the rest; `Clone` copies the whole value, so it carries `Unique` with no edit — [F3 comment](https://github.com/KirkDiggler/rpg-project/issues/1180#issuecomment-7182719994).
- F4 printed nothing: no test pins the list `name, slot, charges, lets`, so no existing test changed — [F4 comment](https://github.com/KirkDiggler/rpg-project/issues/1180#issuecomment-7182719994).
- Proof 1: dropping `omitempty` fails `TestCompileWithoutUniqueMarshalsNoUniqueKey` and `TestAUniqueItemStaysUniqueOnTheSavedSheet` — [rpg-project#1180](https://github.com/KirkDiggler/rpg-project/issues/1180).
- Proof 2: not storing the read word fails `TestCompileUniqueTrueIsUnique` and `TestCloneOfUniqueIsUniqueAndIndependent` with "Should be true" (the word is accepted, the value is false) — [rpg-project#1180](https://github.com/KirkDiggler/rpg-project/issues/1180).
- Suite in `rulebooks/dnd5e` (52 packages; sub-modules are not entered by `./...`): all pass at `898a28ec` before, and at the final head; build, vet and `golangci-lint` are clean — [rpg-project#1180](https://github.com/KirkDiggler/rpg-project/issues/1180).

## Mistaken

- I typed `git stash` while planning the mutation proofs, which the slice forbids; the permission check refused it before it ran and I restored by file copy instead — [rpg-project#1180](https://github.com/KirkDiggler/rpg-project/issues/1180).
