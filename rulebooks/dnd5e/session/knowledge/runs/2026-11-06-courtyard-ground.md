# Run — 2026-11-06 — Courtyard S2 SDK ground

Slice [#874](https://github.com/KirkDiggler/rpg-project/issues/874), story [#867](https://github.com/KirkDiggler/rpg-project/issues/867). Base: `main` at `9922e218`.

## Shipped

- SDK checkpoint carries optional region ground through snapshots and both discovery act families, without interpreting refs or adding visibility logic; consumer integration remains pending — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

## Decided

- Adopt the real pushed encounter commit as `v0.179.1-0.20261106192505-45055ff65bac`; keep this PR draft until the provider ships and the pin moves to its release tag — [encounter provider](https://github.com/KirkDiggler/rpg-toolkit/pull/2289).

## Found

- The existing act-family decoders carry the new typed nested field without another visibility adapter; copy-out tests and both discovery cases pass, as do the complete module tests and zero-issue lint — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).

## Mistaken

- The complete module run exposed an existing exact AtlasRegion field-list assertion; its expected shape now includes Ground while still forbidding room-local frames — [S2](https://github.com/KirkDiggler/rpg-project/issues/874).
