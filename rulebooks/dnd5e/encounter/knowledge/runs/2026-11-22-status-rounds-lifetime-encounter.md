# Run — 2026-11-22 — Status rounds lifetime S2 the engine

processed: 2026-11-25T190427Z

Slice [rpg-project#1082](https://github.com/KirkDiggler/rpg-project/issues/1082) of story [rpg-project#1080](https://github.com/KirkDiggler/rpg-project/issues/1080). Base: `rpg-toolkit` `main` at [`d0262574`](https://github.com/KirkDiggler/rpg-toolkit/commit/d0262574545602f5d8a73a857c474b32ef2d8628).

## Shipped

- `StatusGrant.Until`, the per-application timer, the `StatusLapsed` boundary, save and load, and `Exit` dropping a leaver's timers — [commit dc869774](https://github.com/KirkDiggler/rpg-toolkit/commit/dc869774).
- `apply_status.until: { rounds: N }` parsed, refused where wrong, carried to the grant and said back (full and folded) — [commit 3964022b](https://github.com/KirkDiggler/rpg-toolkit/commit/3964022b).
- The existing refusal sentence now names `until` (`status_test.go` "another key") — [issue 1082 brief](https://github.com/KirkDiggler/rpg-project/issues/1082).

## Decided

- The ending is written on the giving, where it is given (the operator's) — [rpg-project#989 comment](https://github.com/KirkDiggler/rpg-project/issues/989#issuecomment-7066217434).
- The number lives on the grant, never inside the opaque status block — [issue 1082 section 2.1](https://github.com/KirkDiggler/rpg-project/issues/1082).
- The host arms and clears the timer, because only the rules know a giving was fresh — [issue 1082 section 2.1](https://github.com/KirkDiggler/rpg-project/issues/1082).
- A timer whose status already ended fires harmlessly — [issue 1082 section 2.4](https://github.com/KirkDiggler/rpg-project/issues/1082).
- Timers are saved because they are something that happened in play — [issue 1082 section 2.2](https://github.com/KirkDiggler/rpg-project/issues/1082).
- The dungeonspec type is `StatusUntilSpec` and the not-yet-read kind `statusUntil`, to avoid the disposition's names — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1082).

## Found

- The castle content hash encodes every field of the compiled form, so a new nil `StatusGrant.Until` moves the pin to `23df4760...d278`; re-pinned in this PR under Amendment 2, castle testdata unchanged against main — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1082#issuecomment-7066990985).
- The folded read-back text wraps lines, so folded-form assertions compare with whitespace collapsed — [status_until_test.go](https://github.com/KirkDiggler/rpg-toolkit/blob/3964022b/rulebooks/dnd5e/encounter/dungeonspec/readback/status_until_test.go).
- Castle HallGuard and Kitchen tests pass under -race (2.8 to 6.8 s each), so nothing slowed — [commit 3964022b](https://github.com/KirkDiggler/rpg-toolkit/commit/3964022b).
- Existing tests edited: `status_test.go` "another key" sentence only (one more key allowed) — [commit 3964022b](https://github.com/KirkDiggler/rpg-toolkit/commit/3964022b).
- New lapse tests each run under 0.1 s; the new dungeonspec and read-back tests under 2 s — [commit 3964022b](https://github.com/KirkDiggler/rpg-toolkit/commit/3964022b).

## Mistaken

- The first session named the dungeonspec type `UntilSpec` and the kind `"until"` without searching; both clashed with a disposition's. Renamed per the amendment — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1082#issuecomment-7066887589).
