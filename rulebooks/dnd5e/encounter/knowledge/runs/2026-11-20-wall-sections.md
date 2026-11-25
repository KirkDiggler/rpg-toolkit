# Run — 2026-11-20 — Captured wall-owned visual sections

## Shipped

- Candidate native `appearance.sections` lowers wall-local IDs/opaque refs and centre/width once into canonical feet; immutable sections survive site persistence, both atlas reads and complete wall reveal rows — [Courtyard S6](https://github.com/KirkDiggler/rpg-project/issues/1022).

- Review F1 now accepts native/canonical touching-section arithmetic within the existing eight-ULP line allowance, without rewriting captured spans or permitting real overlaps. Native decode/lower/construction regressions fail before the fix and pass after; the full module suite and lint pass — [finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2503#discussion_r4689158294).

## Decided

- Apply the existing endpoint roundoff policy to section contact at both native and canonical boundaries because independently scaled centres/widths can overlap by a few ULPs even when the authored intervals touch exactly — [finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2503#discussion_r4689158294).

- Sections follow the owning wall's existing disclosure, not a new prop/door identity. They do not cut mechanical contributors; validation refuses invalid, overlapping and out-of-wall spans, including overlap with a door surround rather than merely its clear aperture — [continuation plan](https://github.com/KirkDiggler/rpg-project/blob/feat/wall-kit-fit/ideas/wall-kit-fit/sections-plan.md).

## Found

- Focused native/structural tests pass, including detached copies, cold site reload, unchanged mechanical contributors and a real finder-only reveal while the peer's wall stays hidden. Opening-only replacements contain no section rewrite — [slice](https://github.com/KirkDiggler/rpg-project/issues/1022).

## Mistaken

- The first full module race gate exposed six atlas characterization changes and the compiler-content hash pin. Every JSON change was proved to be only `Sections:null`; the content encoder deliberately includes every compiled struct field and is engine-version-local. Updated those pictures and its pin without changing the encoder. Persisted legacy SiteHash remains separately protected by omitted empty data — [slice](https://github.com/KirkDiggler/rpg-project/issues/1022).
