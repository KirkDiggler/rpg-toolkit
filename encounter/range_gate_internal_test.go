package encounter

// range_gate_internal_test.go — white-box unit coverage for the pure
// distance/reach helpers in range_gate.go (rpg-toolkit#864). Lives in
// package encounter (not encounter_test) so it can exercise the unexported
// helpers (checkReach, meleeReachForWeapon, meleeReachForCombatant) directly
// with hand-built fixtures, rather than driving a full hydrated
// character/monster through TakeStrikePhased/NPCAct just to prove the
// weapon-reach arithmetic — that integration coverage lives in
// combat_phased_test.go / combat_test.go (encounter_test package).

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// bareCombatant is a minimal combat.Combatant test double. The encounter
// range gate deliberately uses the canonical default reach and leaves weapon
// selection to the typed Strike boundary.
type bareCombatant struct{}

func (bareCombatant) GetID() string        { return "bare" }
func (bareCombatant) GetHitPoints() int    { return 1 }
func (bareCombatant) GetMaxHitPoints() int { return 1 }
func (bareCombatant) AC() int              { return 10 }
func (bareCombatant) ApplyDamage(_ context.Context, _ *combat.ApplyDamageInput) *combat.ApplyDamageResult {
	return &combat.ApplyDamageResult{}
}
func (bareCombatant) IsDirty() bool                       { return false }
func (bareCombatant) MarkClean()                          {}
func (bareCombatant) AbilityScores() shared.AbilityScores { return shared.AbilityScores{} }
func (bareCombatant) ProficiencyBonus() int               { return 0 }
func (bareCombatant) PassivePerception() int              { return 10 }

// meleeWeaponCombatant embeds bareCombatant and carries a weapon fixture for
// the direct meleeReachForWeapon tests. Weapon-specific reach is not consulted
// by meleeReachForCombatant.
type meleeWeaponCombatant struct {
	bareCombatant
	weapon *weapons.Weapon
}

func (m meleeWeaponCombatant) MeleeWeapon() *weapons.Weapon { return m.weapon }

var (
	_ combat.Combatant = bareCombatant{}
	_ combat.Combatant = meleeWeaponCombatant{}
)

func TestCheckReach_WithinMax_NoError(t *testing.T) {
	err := checkReach(core.Hex{Q: 0, R: 0, S: 0}, core.Hex{Q: 1, R: 0, S: -1}, 1, "reach")
	require.NoError(t, err)
}

func TestCheckReach_BeyondMax_WrapsErrOutOfRange(t *testing.T) {
	err := checkReach(core.Hex{Q: 0, R: 0, S: 0}, core.Hex{Q: 3, R: 0, S: -3}, 1, "reach")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrOutOfRange))
	require.Contains(t, err.Error(), "3 hexes")
	require.Contains(t, err.Error(), "reach 1")
}

func TestCheckInteractReach_Adjacent_NoError(t *testing.T) {
	err := checkInteractReach(core.Hex{Q: 0, R: 0, S: 0}, core.Hex{Q: 1, R: 0, S: -1})
	require.NoError(t, err)
}

func TestCheckInteractReach_Distant_ErrOutOfRange(t *testing.T) {
	err := checkInteractReach(core.Hex{Q: 0, R: 0, S: 0}, core.Hex{Q: 10, R: 0, S: -10})
	require.True(t, errors.Is(err, ErrOutOfRange))
}

func TestMeleeReachForWeapon_Nil_DefaultsToOne(t *testing.T) {
	require.Equal(t, 1, meleeReachForWeapon(nil))
}

func TestMeleeReachForWeapon_NonReachWeapon_IsOne(t *testing.T) {
	w, err := weapons.GetByID(weapons.Scimitar)
	require.NoError(t, err)
	require.Equal(t, 1, meleeReachForWeapon(&w))
}

func TestMeleeReachForWeapon_ReachProperty_IsTwo(t *testing.T) {
	w, err := weapons.GetByID(weapons.Glaive)
	require.NoError(t, err)
	require.Equal(t, 2, meleeReachForWeapon(&w))
}

func TestMeleeReachForCombatant_ReachWeapon_UsesCanonicalDefault(t *testing.T) {
	glaive, err := weapons.GetByID(weapons.Glaive)
	require.NoError(t, err)
	reach := meleeReachForCombatant(meleeWeaponCombatant{weapon: &glaive})
	require.Equal(t, 1, reach)
}

func TestMeleeReachForCombatant_NoWeapon_DefaultsToOne(t *testing.T) {
	reach := meleeReachForCombatant(meleeWeaponCombatant{weapon: nil})
	require.Equal(t, 1, reach)
}

func TestMeleeReachForCombatant_NilCombatant_DefaultsToOne(t *testing.T) {
	require.Equal(t, 1, meleeReachForCombatant(nil))
}

func TestMeleeReachForCombatant_NonProvider_DefaultsToOne(t *testing.T) {
	require.Equal(t, 1, meleeReachForCombatant(bareCombatant{}))
}
