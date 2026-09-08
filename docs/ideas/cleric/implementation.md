# Cleric creation implementation

The first contribution to [rpg-project#406](https://github.com/KirkDiggler/rpg-project/issues/406)
repairs level-one Cleric character creation on toolkit main `e894540e`.

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

- The D&D module suite was run. Its existing appearance timestamp assertion
  fails on Windows at `character/appearance_test.go:175`
  (`TestDraftSetAppearanceValidatesAtomicallyAndPreservesClassCarryover`), as it
  did before the Cleric changes.
- After adopting main's completeness handling, the character, choices and class
  package suites pass with only that known appearance test excluded.
- `go fmt ./...` and `go mod tidy` ran in the D&D module; no dependency changes.
- Local `golangci-lint` is unavailable. Race testing requires cgo, disabled in
  this environment. CI remains the source of lint and race validation.

The broader playable-Cleric journey remains open; these checks do not establish
API, web or live-game support.
