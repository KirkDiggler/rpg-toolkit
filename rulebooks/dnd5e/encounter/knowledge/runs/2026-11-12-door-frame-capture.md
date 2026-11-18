# Run — 2026-11-12 — Courtyard S4 door-frame capture

Slice [Project #906](https://github.com/KirkDiggler/rpg-project/issues/906), story [Project #867](https://github.com/KirkDiggler/rpg-project/issues/867). Base `main` at [5dcae25](https://github.com/KirkDiggler/rpg-toolkit/commit/5dcae25ff21893c10cabbcd5699da15805f621f8).

## Shipped

- Provider implementation checkpoint: typed native frame facts, canonical capture, detached site persistence, independent projection and reveal values; publication and downstream adoption remain pending — [continuation](https://github.com/KirkDiggler/rpg-project/issues/906#issuecomment-6844016998).

## Decided

- Native `door.framePresentation` supplies resolved appearance because the composition must not inspect a vendor catalog or recover an edited source scene; lowering converts visual lengths once and preserves unitless phase — [contract](https://github.com/KirkDiggler/rpg-api-protos/pull/436).
- Keep visual spans separate from passage width because presentation cannot change mechanical footprints; native and constructor validation refuse overflowing or overlapping frame spans — [continuation](https://github.com/KirkDiggler/rpg-project/issues/906#issuecomment-6844016998).
- Persist dedicated data values and clone each span/list at input, data and projection boundaries because an independent reader must not mutate captured appearance — [contract](https://github.com/KirkDiggler/rpg-api-protos/pull/436).

## Found

- The existing recipient projection already permits a door with a withheld wall and withholds the whole cut for a concealed door; no new disclosure policy or parent association is needed — [measured boundary](https://github.com/KirkDiggler/rpg-project/issues/906#issuecomment-6844016998).
- Focused frame tests exercise input/output mutation, both readers, JSON reload, explicit zero phase, native length conversion, unchanged mechanical contributors, malformed source paths and semantic reveal equality; the entire encounter module passes race tests and lint — [slice](https://github.com/KirkDiggler/rpg-project/issues/906).

## Mistaken

- Review found that a persisted scalar phase normalized missing/null to zero even under the original valid site's hash; presence is now retained in the Data type and refused before canonical construction. The cold-site regression preserves explicit zero, absent legacy frames and valid bytes/hash, and its presence-guard mutant fails — [review finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2412#discussion_r4561915936).

- Existing structural equality relied on comparable records; adding detached frame pointers made equal copies appear changed, so a red parent-later reveal test required semantic frame/span comparison before this checkpoint — [contract constraint](https://github.com/KirkDiggler/rpg-api-protos/pull/436).
