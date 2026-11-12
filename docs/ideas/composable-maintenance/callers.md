# Composable maintenance — concrete caller inventory

Inspection at toolkit `d4033752` (effect-related source unchanged from `49e9918b`). Query: `rg -l 'ConcentratingCondition|CastConcentration|NewBlessedCondition|NewBanedCondition' rulebooks/dnd5e --glob '*.go'`. Paths below are relative to `rulebooks/dnd5e`.

This inventory is a migration worklist, not permission to delete tests. Preserve behavior assertions while replacing constructors, concrete assertions and combined upkeep declarations. Comments naming the old form are corrected when the implementation changes. Re-run the query before closing T8–T10; classify each remaining occurrence explicitly, never add an alias to hide it.

## Root module — T6/T8

Disposition: runtime/declarations migrated at `94aac8d9` / `8d37e5fd`, retired godoc links corrected at `4a73a227`. The inventory query now finds no old constructors/types in root source packages. Named definitions remain as current content, not compatibility runtimes. Cast facts derive from parts; condition/character views query maintenance. Root gameplay assertions were ported to the current creator and real linked keepers; full root race/vet/lint pass. Additional affected files discovered during compilation: `character/casting_test.go`, `conditions/target_held_test.go`, `events/damage_taken.go` and `refs/conditions.go`. Native nested modules below remain open, including the R11 paused-initial-delivery boundary.

- `character/blessed_test.go`
- `character/character.go`
- `character/cleric_finalize_test.go`
- `character/concentration_test.go`
- `character/death_save_test.go`
- `character/one_condition_per_identity_test.go`
- `character/status_view_test.go`
- `combat/actions/area.go`
- `combat/actions/cast.go`
- `combat/actions/cast_test.go`
- `combat/actions/information.go`
- `combat/actions/move.go`
- `conditions/action_bearing_test.go`
- `conditions/action_effects_test.go`
- `conditions/applied_twice_test.go`
- `conditions/baned.go`
- `conditions/baned_test.go`
- `conditions/blade_ward.go`
- `conditions/blessed.go`
- `conditions/blessed_test.go`
- `conditions/concentrating.go`
- `conditions/concentrating_test.go`
- `conditions/factory.go`
- `conditions/instance_detail_test.go`
- `conditions/ref_contract_test.go`
- `conditions/true_strike.go`
- `monster/condition_removed_test.go`
- `refs/conditions.go`
- `spells/cast.go`

## Resolution — T9

- `resolution/action.go`
- `resolution/areas_test.go`
- `resolution/bless_test.go`
- `resolution/concentration_test.go`
- `resolution/death_save_test.go`
- `resolution/depart.go`
- `resolution/depart_test.go`
- `resolution/pause_scenes_test.go`
- `resolution/sanctuary_cooldown_test.go`
- `resolution/short_rest.go`
- `resolution/short_rest_test.go`
- `resolution/strike_pose_test.go`
- `resolution/strike_test.go`

## Session — T10

- `session/concentration_break_test.go`
- `session/death_save_test.go`
- `session/depart_test.go`
- `session/effects_test.go`
- `session/multiattack_turn_test.go`
- `session/rest_test.go`
- `session/selector_projection.go`
- `session/selector_projection_test.go`
- `session/turn_concentration_test.go`

The plan also names transport/information entry points that do not currently refer to the concrete type. They still require integration, not merely a search-and-replace of this inventory.
