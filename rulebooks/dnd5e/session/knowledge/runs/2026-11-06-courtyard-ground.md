# Run — 2026-11-06 — Courtyard S2 SDK ground

processed: 2026-11-12T165555Z

Slice [#874](https://github.com/KirkDiggler/rpg-project/issues/874), story [#867](https://github.com/KirkDiggler/rpg-project/issues/867). Base: `main` at `9922e218`.

## Shipped

- SDK checkpoint carries optional region ground through snapshots and both discovery act families, without interpreting refs or adding visibility logic; consumer integration remains pending — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

## Decided

- Adopt the real pushed encounter commit as `v0.179.1-0.20261106192505-45055ff65bac`; keep this PR draft until the provider ships and the pin moves to its release tag — [encounter provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2289).

- Advance the development pin to `v0.179.1-0.20261107231809-241e42630804` so the SDK consumes the canonical castle ground fold-back rather than an older provider fixture; no session behavior changes and the released-tag gate remains — [encounter provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2289).

## Found

- The existing act-family decoders carry the new typed nested field without another visibility adapter; copy-out tests and both discovery cases pass, as do the complete module tests and zero-issue lint — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

- Both module packages and zero-issue lint pass after the castle-content provider re-pin — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

## Mistaken

- The complete module run exposed an existing exact AtlasRegion field-list assertion; its expected shape now includes Ground while still forbidding room-local frames — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).
