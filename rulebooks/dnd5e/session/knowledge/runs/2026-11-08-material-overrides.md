# Run — 2026-11-08 — Material override SDK transcription

## Shipped

- SDK checkpoint carries captured material choices through wall/prop snapshots and existing reveal decoders; API/Web integration remains outstanding — [Courtyard S3](https://github.com/KirkDiggler/rpg-project/issues/895).

## Decided

- Consume actual pushed encounter e3849c88 via v0.179.1-0.20261108215032-e3849c889aa5; no local replacements. Keep this separate module PR draft until the provider release tag is adopted — [encounter provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2330).
- Project detached SDK-owned SurfaceID/MaterialRef values and refuse malformed complete event bodies; do not enrich them from current source or infer resource compatibility/audience — [design](https://github.com/KirkDiggler/rpg-project/blob/design/867-courtyard-ground/ideas/courtyard-walls/texture-design.md).

## Found

- Snapshot and payload selections agree; returned lists do not alias the provider. Wrong kinds, blank identities, null items and duplicate targets refuse entire wall/prop payloads, including valid sibling rows — [S3](https://github.com/KirkDiggler/rpg-project/issues/895).
- Both module packages pass go test ./... and golangci-lint reports zero issues — [S3](https://github.com/KirkDiggler/rpg-project/issues/895).

## Mistaken

- Two explicit structural field-inventory tests detected the added fact; inventories now include MaterialOverrides while retaining their no-parent/no-private-state checks — [S3](https://github.com/KirkDiggler/rpg-project/issues/895).
