# Run — 2026-11-22 — hook linter only

Slice [rpg-toolkit#2517](https://github.com/KirkDiggler/rpg-toolkit/issues/2517), no story. Base: `rpg-toolkit` `main` at [`ca0c08eb`](https://github.com/KirkDiggler/rpg-toolkit/commit/ca0c08ebfdfcd6e52fae9829d82dc6ab0dbe833a).

## Shipped

- The three hook scripts no longer run tests; the hook README and the run-tests how-to say so — [rpg-toolkit#2517](https://github.com/KirkDiggler/rpg-toolkit/issues/2517).

## Decided

- Formatting, `go mod tidy` and the linter stay in the hook; only the test step goes, because they take seconds — [the issue](https://github.com/KirkDiggler/rpg-toolkit/issues/2517).
- `pre-commit.backup` loses its `make test-all` step, the same as the other two — [amendment 1](https://github.com/KirkDiggler/rpg-toolkit/issues/2517#issuecomment-7068832370).

## Found

- `pre-commit.backup` ran its tests through `make test-all`, not `go test`, so the issue's first grep missed it — [gap](https://github.com/KirkDiggler/rpg-toolkit/issues/2517#issuecomment-7068829595).
- The hook run on a staged change in `rulebooks/dnd5e/session` takes 6.9s real with the test step gone — [issue](https://github.com/KirkDiggler/rpg-toolkit/issues/2517).
- The hook fails when any unstaged modification exists, so a proof run needs every other change staged first — [issue](https://github.com/KirkDiggler/rpg-toolkit/issues/2517).

## Mistaken

- The issue's F1 named a `go test` line in `pre-commit.backup` that the command did not print; found at Step 0 and answered by amendment 1 — [gap](https://github.com/KirkDiggler/rpg-toolkit/issues/2517#issuecomment-7068829595).
- My first proof run used an unformatted `//` line and unstaged edits, so the hook failed on modifications; rerun with the edits staged passed — [issue](https://github.com/KirkDiggler/rpg-toolkit/issues/2517).

— cross-team agent, on behalf of KirkDiggler
