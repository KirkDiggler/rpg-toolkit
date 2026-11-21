# Run — 2026-11-22 — Kept status S2 a status hold has a name

Slice [rpg-project#1093](https://github.com/KirkDiggler/rpg-project/issues/1093) of story [rpg-project#1090](https://github.com/KirkDiggler/rpg-project/issues/1090). Base: `rpg-toolkit` `main` at [`b7fbc2d3`](https://github.com/KirkDiggler/rpg-toolkit/commit/b7fbc2d3c39d7e57bf568ea957981101a6cead35).

## Shipped

- `StatusSourceRef`, the status source admitted by `nativeMaintenanceSpell`, and a status receiver's removal named by the status — [rpg-toolkit#2521](https://github.com/KirkDiggler/rpg-toolkit/pull/2521).
- `status_hold_test.go`: eight tests, each 0.00s, no existing test edited — [rpg-toolkit#2521](https://github.com/KirkDiggler/rpg-toolkit/pull/2521).

## Decided

- A status an author wrote is accepted as something a member can concentrate on, beside spells — [rpg-project#1093](https://github.com/KirkDiggler/rpg-project/issues/1093).
- A broken hold names its receivers by the status's name — [rpg-project#1093](https://github.com/KirkDiggler/rpg-project/issues/1093).
- A cast stays spell-only: `nativeMaintenanceSpell` is as on main and a sibling `nativeMaintenanceHold` is what a member holds, per Amendment 1 — [amendment](https://github.com/KirkDiggler/rpg-project/issues/1093#issuecomment-7069264929).
- A monster is proved as a keeper here — [rpg-project#1093](https://github.com/KirkDiggler/rpg-project/issues/1093).

## Found

- Rests on: F1 the spell-only check at `maintenance_information.go` holds; F2 five non-test callers holds; F3 `application_binding.go:64` holds; F4 the session's `statusSourceRef` returns `site/statuses` holds; F5 linked catalog name "Linked effects" holds; F6 `StatusSourceRef` unused holds — [comment](https://github.com/KirkDiggler/rpg-project/issues/1093).
- `linked_cast_test.go:180` pins the refusal text "unsupported non-spell native maintenance source", which the cast path keeps unchanged — [linked_cast_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/feat/status-hold-identity/rulebooks/dnd5e/resolution/linked_cast_test.go#L180).
- The shared world fixture holds only the hero, the bard and the wolf, so the bard is the second receiver and the spell-hold test has the wolf strike the bard — [status_hold_test.go](https://github.com/KirkDiggler/rpg-toolkit/pull/2521/files).

## Mistaken

- I first reworded the refusal message and returned a pointer from `StatusSourceRef`; I found the pinned message and the issue's value signature before running anything and corrected both — [rpg-toolkit#2521](https://github.com/KirkDiggler/rpg-toolkit/pull/2521).
- The widened check reached the cast path: `cast_application.go:29` shares the function, so a cast naming a status was accepted and charged a slot; the review found it and `TestACastNamingAStatusIsRefused` now pins it — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2521#pullrequestreview).
