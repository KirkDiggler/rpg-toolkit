# Cleric creation implementation

The first contribution to [rpg-project#406](https://github.com/KirkDiggler/rpg-project/issues/406)
repairs level-one Cleric character creation. [PR #1585](https://github.com/KirkDiggler/rpg-toolkit/pull/1585)
merged as `b2f3b88d` on 2026-09-08 and is released as D&D `v0.150.0`.
The refreshed [plan.md](plan.md) records subsequent integration work.

Clerics receive light/medium armor, shield and simple-weapon proficiencies,
plus one fixed shield. Life's existing subclass data supplies heavy armor.
The selected domain reaches validation, and recording, completeness and final
validation use subclass requirements. Duplicate skills and cantrips are rejected;
Life's starting warhammer requires proficiency from the compiled grants.
Main already supplies spell-choice completeness, so this contribution reuses it.

The Cleric suite verifies draft and character JSON round trips, equipment
quantities and alternatives, background grants, invalid choices, and class/domain
replacement. Reload preserves damaged HP and spent slot data. This is creation
support; prepared spells, runtime casting, domain spells, Disciple of Life and
concentration remain outside this PR. Other domains' automatic grants and
equipment prerequisites are not migrated here.

## Verification

- The full D&D module suite passes on native Windows with no exclusions.
- The existing appearance test assumed consecutive updates cross a clock tick.
  It now uses a reloaded draft's zero timestamp as the baseline, retaining the
  strict update assertion and invalid-input atomicity checks without sleeps or
  production changes. That test passes 100 consecutive native Windows runs.
- `go fmt ./...` and `go mod tidy` ran in the D&D module; no dependency changes.
- Local `golangci-lint` is unavailable. Race testing requires cgo, disabled in
  this environment. The initial creation commit passed CI; CI also validates
  the subsequent timestamp-test change.

The broader playable-Cleric journey remains open; these checks do not establish
API, web or live-game support.

## Next contribution: Sacred Flame content (2026-09-08)

Added the level-one Sacred Flame profile to the existing cast table: 60 feet,
one creature, Dexterity save against the supplied DC, 1d8 radiant on failure,
negated on success, with no condition or concentration. Profile tests use the
reviewed 2014 entry in `source-index.json`. A creation/reload regression proves
a Cleric with Wisdom 16 supplies DC 13 and retains all three known cantrips,
while only Sacred Flame among those choices has executable content.

The full D&D module suite passes natively on Windows with no skips. Formatting
and diff checks pass. No other module or repository pins changed. This content
slice is submitted for review; session execution, cover semantics, higher-level scaling,
and the shared damage-chain issue #1582 remain outside the validated claim.
