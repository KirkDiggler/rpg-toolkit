# Run — 2026-11-28 — Courtyard S7 canonical material-channel custody

Slice [rpg-project#1192](https://github.com/KirkDiggler/rpg-project/issues/1192) of [Courtyard#867](https://github.com/KirkDiggler/rpg-project/issues/867). Base: encounter on main8517aac9; no other module is changed by this work.

## Shipped

- Canonical channel intent, strict persisted presence, native capture, static wall/prop/frame projection and member/mutable-prop observed custody are implemented; host/API/Web integration is not claimed complete — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).
- Full module tests, race tests and lint passed; native and reload checks preserve explicit Off/Scale1/Scale0.5, detached values, independent frame disclosure and unchanged mechanics — [plan T4](https://github.com/KirkDiggler/rpg-project/blob/design/material-channels/ideas/material-channels/plan.md#task-4-canonical-capture-persistence-and-observed-custody-t4).

## Decided

- Encounter carries and validates opaque target/mode/multiplier facts, without resource lookup or illumination meaning; memories capture values rather than rejoining current live appearance — [approved design](https://github.com/KirkDiggler/rpg-project/pull/1189).
- Material-channel natural-language read-back is explicitly acknowledged as not yet said under the existing guard; this slice does not claim prose-generation behavior — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).

## Found

- Removing only49 new nil channel fields from the compiled castle encoding reproduces the base content hash exactly; six regenerated pictures contain only new null channel fields. The encoder is unchanged and the legacy persisted SiteHash test remains7c7fa22b… — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).
- The existing unscoped Atlas wall read omits MaterialOverrides while AtlasFor carries them; tracked separately, not silently repaired as channel work — [Toolkit#2617](https://github.com/KirkDiggler/rpg-toolkit/issues/2617).

## Mistaken

- A conversion loop keyed by source pointer collapsed two channels when a caller shared one data pointer; an ordered pair loop and a failing regression now preserve both detached values — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).
- Standard JSON decoding alone erased invalid null presence and accepted unknown/duplicate/case-alias fields; the typed leaf now refuses those inputs before they become default or another mode — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).
- The first native test used whole typed-spec YAML re-encoding as an oracle; the unmodified castle fails that path on an unrelated temperament word. The test now round-trips the new typed channel value inside the original native document and compares all unaffected compiled facts — [slice](https://github.com/KirkDiggler/rpg-project/issues/1192).
