# Task 13 report — dependency cutover verification checkpoint

## Changes

- `rulebooks/dnd5e/resolution/strike.go` projects the richer canonical
  `AttackModifierSource` values onto the post-roll event's established
  reference-only source fields.
- `encounter/range_gate_internal_test.go` removes the stale assertion against
  the deleted `MeleeWeaponProvider` API while retaining the reach fixtures.
- `rulebooks/dnd5e/session/attack_internal_test.go` adds the Task 11 boundary
  regression: typed `StrikeOutcome.DamageInstances` never become typed session
  history; only aggregate `StrikeOutcome.Damage` is recorded.

## Verification

- PASS: `git diff --check`.
- PASS: D&D focused suites: `go test ./damage ./weapons ./monster/... ./events ./combat ./conditions -count=1`.
- PASS: resolution full suite against the temporary local workspace after the
  post-roll source projection fix: `go test ./... -count=1`.
- BLOCKED: encounter focused suite. Existing encounter tests still construct
  removed `DamageChainInput.WeaponDamageDice`/`WeaponDamageType` fields and
  reference removed `conditions.SneakStrikeInput`.
- BLOCKED: session focused suite. Its published `rulebooks/dnd5e/encounter`
  dependency and the current local module are on incompatible, unrelated
  encounter API generations (`Atlas.Rooms`, `ErrNoConnection`, `Locate`, and
  related members). The aggregate-only test itself is present but cannot be
  compiled until that external dependency drift is released or separately
  migrated.

The temporary `go.work`/`go.work.sum` files used for local verification were
removed before this commit. No local `replace` directive or workspace artifact
is retained.

## Concerns

The bounded checkpoint did not create or push release tags. The remaining
encounter/session dependency-generation blockers must be resolved before the
full repository release gate can be claimed green.

## Review follow-up

- Updated the reach-gate fixture so `meleeReachForCombatant` asserts the
  canonical default reach of 1 even when handed a glaive; the direct
  `meleeReachForWeapon` test remains the proof that a Reach property resolves
  to 2.
- Focused range-gate test attempt was blocked before compilation because the
  sandbox could not download the encounter module's published `events@v0.6.2`
  and D&D `v0.71.1-0.20260808232907-1eb7569a9e1f` dependencies.
