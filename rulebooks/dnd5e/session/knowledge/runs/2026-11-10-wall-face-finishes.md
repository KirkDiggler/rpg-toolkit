# Run — 2026-11-10 — Courtyard S5 wall-face projection

Work [Project #919](https://github.com/KirkDiggler/rpg-project/issues/919), design [Project #915](https://github.com/KirkDiggler/rpg-project/pull/915), shared controls [Web #1402](https://github.com/KirkDiggler/rpg-dnd5e-web/issues/1402).

## Shipped

- Session-owned WallFaceFinish DTO, detached snapshot projection and strict atomic reveal decoding; API/generated wire/UI integration remains downstream — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).

## Decided

- Project exactly the composition's captured side/material values, with no resource lookup, regional inference or fresh source enrichment. Missing legacy lists remain absent and opening-only replacements keep their existing scope — [design](https://github.com/KirkDiggler/rpg-project/blob/design/1402-studio-appearance/ideas/studio-appearance/design.md).
- Develop on the provider's actual pushed bb120b71 checkpoint, resolved by Go to encounter v0.188.1-0.20261110060037-bb120b71b4c0; release-tag adoption remains required before an authorized consumer merge — [provider PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2356).

## Found

- Snapshot/reveal equality, both-direction alias isolation, missing legacy fields and malformed-list atomic refusal were RED before implementation and GREEN afterward; reflection transcription probes include FaceFinishes explicitly — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).
- Disabling only face-list validation makes invalid side/blank/duplicate records and their valid sibling rows leak through the decoder; the focused regression fails. The mutation is restored — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).
- Full race/atomic-coverage gate passes both packages (session253.700s; workbench1.164s) and golangci-lint reports0issues. No release or operator deployment is implied — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).

## Mistaken

- none observed — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).
