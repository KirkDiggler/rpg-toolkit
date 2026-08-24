# Verified transcripts — show a module working as designed

Before the game had a UI, we watched the toolkit through printed
stories. The pattern survives because it is proof, not decoration: a
Go testable Example prints the module's signature scene, and its
`// Output:` block is checked character-for-character by `go test`.
The narration structurally cannot drift from what the code does.

## The recipe (one file, stock Go)

1. In your module's black-box test package, write
   `Example_<yourScene>()` that PLAYS the one scene your module exists
   for and narrates it with `fmt.Printf` — small: one story, aim for
   under twenty lines of output. End the function with a placeholder:

   ```go
   // Output:
   // PLACEHOLDER
   ```

2. Run `go test -run Example_<yourScene>` once. It fails, printing a
   `got:` block — the real transcript. Read it critically (is this the
   story you meant to tell?), then paste it over the placeholder.

3. Done. `go test` now proves the story on every run, and godoc renders
   it as the module's documentation.

## Getting feedback

```bash
./scripts/verify.sh <module-dir>     # e.g. play/intel, events
```

runs the module's gate (build, tests, vet, gofmt, lint) with one check
mark per step, re-proves the Examples, and prints every pinned
transcript. A module with no Examples gets a nudge instead of a
transcript.

Exemplars: `rulebooks/dnd5e/encounter/example_tombwatch_test.go` (a
whole scene: sight, the ghost, save/reload, the ending) and
`events/example_magic_test.go` (the original — API-shaped micro-scenes).

## When a transcript isn't enough

For modules with a big interactive surface, escalate to a terminal
workbench under `cmd/` — a small `main.go` REPL, no server involved.
Exemplars: `rulebooks/dnd5e/encounter/cmd/freeroam-workbench`
(walk the crypt, watch beliefs diverge from world truth) and
`rulebooks/dnd5e/session/cmd/session-workbench`
(drive a whole session through the active host seam). Workbenches are
optional; the transcript is the pattern.
