# Run — 2026-11-22 — Status rounds lifetime S1 the rules

Slice [rpg-project#1081](https://github.com/KirkDiggler/rpg-project/issues/1081) of story [rpg-project#1080](https://github.com/KirkDiggler/rpg-project/issues/1080). Base: `rpg-toolkit` `main` at [`d0262574`](https://github.com/KirkDiggler/rpg-toolkit/commit/d0262574).

## Shipped

- `StatusLapsedEvent`, beat `status_lapsed`, fact `lapsed.application`, part kind `lapse` and `status.Until{Rounds}` in `rulebooks/dnd5e` — [rpg-toolkit#2511](https://github.com/KirkDiggler/rpg-toolkit/pull/2511).
- Seven new tests (3 in `effects/lapse_part_test.go`, 4 in `status/lapse_test.go`), each under 0.01 s; no existing test or fixture edited — [rpg-toolkit#2511](https://github.com/KirkDiggler/rpg-toolkit/pull/2511).

## Decided

- The ending is written on the giving, as `until: { rounds: 3 }`, not inside the status (the operator, 2026-11-22) — [rpg-project#989 comment](https://github.com/KirkDiggler/rpg-project/issues/989#issuecomment-7066217434).
- `lapse` ends only its own application, by the `until_empty` pattern without the owner check, so two timed statuses on one holder end separately — [end_parts.go](https://github.com/KirkDiggler/rpg-toolkit/pull/2511/files).
- The timer is not in this module; nothing here publishes `StatusLapsedEvent` — [rpg-project#1081](https://github.com/KirkDiggler/rpg-project/issues/1081).
- The lapse part is appended in `library()` after the "no maintenance" check (the issue said in `Compile`), so a status that states no ending of its own is still refused — [compile.go](https://github.com/KirkDiggler/rpg-toolkit/pull/2511/files).

## Found

- All seven rests-on facts F1 to F7 held at `d0262574` — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1081#issuecomment-7066783700).
- Today's refusal for a status with no `maintenance` is "the status has no maintenance" (raised in `status()`), not the sentence in `library()` — [compile.go](https://github.com/KirkDiggler/rpg-toolkit/pull/2511/files).
- `/home` was 100% full from a 111 GB Go build cache, which made `go test ./...` and `golangci-lint` fail with "no space left on device" and a misleading VCS-status error; `go clean -cache` cleared it — [rpg-toolkit#2511](https://github.com/KirkDiggler/rpg-toolkit/pull/2511).

## Mistaken

- The first fixtures lacked `again`, so tests failed on "the status has no again" rather than on the thing under test; fixed before the PR opened — [rpg-toolkit#2511](https://github.com/KirkDiggler/rpg-toolkit/pull/2511).
