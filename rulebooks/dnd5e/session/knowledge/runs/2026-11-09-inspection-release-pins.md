# Run — 2026-11-09 — inspection session on released providers

## Shipped

- Prepared the session consumer on published encounterv0.186.0, behaviorv0.22.0, machinev0.2.0 and recordv0.3.0; source catch-up preserves the existing inspection classification/tests and the independently shipped fenced-seat correction — [session PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2328).

## Decided

- Continue the agreed inside-out release after the operator merged encounter#2326; verify the real tag before changing the next consumer, with no hand tags, source overrides or new independent review round for the mechanical re-pin — [release brief](https://github.com/KirkDiggler/rpg-project/issues/898#issuecomment-6732268474).
- Keep the published record-format break explicit for subsequent host adoption; do not treat old inline-audience preview saves as compatible with recordv0.3.0 — [provider adoption](https://github.com/KirkDiggler/rpg-toolkit/pull/2331).

## Found

- Encounterv0.186.0 resolves to mergedb1e3f202; tagger42699517290 succeeds, its module tree equals reviewed115200c9, and Go's downloaded module origin names the same tag/hash — [encounter release](https://github.com/KirkDiggler/rpg-toolkit/pull/2326).
- Full session race suite on the released graph passes381.231s, workbench1.149s, lint0; the pin delta after main catch-up touches only session go.mod/go.sum, with no inspection production/test edits — [consumer evidence](https://github.com/KirkDiggler/rpg-toolkit/pull/2328).

## Mistaken

- The first full attempt could not open the castle fixture because `go list -m` returned no module Dir despite the version being selected. An explicit download of the same official module supplied its Dir/fixture, the exact failed case passed8.079s, then the full suite passed. No test weakening or repository-local fixture substitute was used; the cause of the absent cache directory is not guessed — [consumer evidence](https://github.com/KirkDiggler/rpg-toolkit/pull/2328).
