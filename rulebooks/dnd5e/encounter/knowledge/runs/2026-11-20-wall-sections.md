# Run — 2026-11-20 — Captured wall-owned visual sections

## Shipped

- Candidate native `appearance.sections` lowers wall-local IDs/opaque refs and centre/width once into canonical feet; immutable sections survive site persistence, both atlas reads and complete wall reveal rows — [Courtyard S6](https://github.com/KirkDiggler/rpg-project/issues/1022).

## Decided

- Sections follow the owning wall's existing disclosure, not a new prop/door identity. They do not cut mechanical contributors; validation refuses invalid, overlapping and out-of-wall spans, including overlap with a door surround rather than merely its clear aperture — [continuation plan](https://github.com/KirkDiggler/rpg-project/blob/feat/wall-kit-fit/ideas/wall-kit-fit/sections-plan.md).

## Found

- Focused native/structural tests pass, including detached copies, cold site reload, unchanged mechanical contributors and a real finder-only reveal while the peer's wall stays hidden. Opening-only replacements contain no section rewrite — [slice](https://github.com/KirkDiggler/rpg-project/issues/1022).

## Mistaken

- The first full module race gate exposed six atlas characterization changes and the compiler-content hash pin. Every JSON change was proved to be only `Sections:null`; the content encoder deliberately includes every compiled struct field and is engine-version-local. Updated those pictures and its pin without changing the encoder. Persisted legacy SiteHash remains separately protected by omitted empty data — [slice](https://github.com/KirkDiggler/rpg-project/issues/1022).
