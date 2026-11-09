# Run — 2026-11-08 — Captured wall and prop materials

## Shipped

- Encounter checkpoint captures opaque material overrides at the existing wall/prop owners, including native lowering, host persistence, recipient layout/reveal and mutable prop observations. SDK/API/Web integration remains outstanding — [Courtyard S3](https://github.com/KirkDiggler/rpg-project/issues/895).

## Decided

- Use one runtime SurfaceID/MaterialRef pair and explicit persisted/native representations; validate nonblank/unique scope IDs, not resource membership or game semantics — [design](https://github.com/KirkDiggler/rpg-project/blob/design/867-courtyard-ground/ideas/courtyard-walls/texture-design.md).
- Clone each list at input, snapshot, data and observation seams; no new source-document lookup, event type or audience computation — [plan A4](https://github.com/KirkDiggler/rpg-project/blob/design/867-courtyard-ground/ideas/courtyard-walls/atlas-implementation.md).

## Found

- Native current-castle compile differs only in the explicitly authored appearance lists; geometry, regions/ground, roster and mechanics compare equal. Invalid/null/duplicate/non-string source values refuse with materialOverrides paths — [S3](https://github.com/KirkDiggler/rpg-project/issues/895).
- Input/public-atlas/data mutation cannot repaint the encounter. JSON host reload, concealed-owner filtering, stored reveal rows and the existing mutable-prop memory/empty-evidence route retain the choices — [S3](https://github.com/KirkDiggler/rpg-project/issues/895).
- All nine module packages pass go test ./...; golangci-lint reports zero issues. No dependency, proto import or gameplay rule added — [S3](https://github.com/KirkDiggler/rpg-project/issues/895).

## Mistaken

- The first full module run found six compile snapshots missing the new absent runtime field. Re-captured only those six and independently parsed the diffs: only MaterialOverrides:null was added to existing wall rows. No fixture gameplay or assertion was removed — [S3](https://github.com/KirkDiggler/rpg-project/issues/895).
- Staticcheck requested explicit struct conversions for equivalent runtime/data pairs; corrected them and reran the full module/lint gates — [S3](https://github.com/KirkDiggler/rpg-project/issues/895).
