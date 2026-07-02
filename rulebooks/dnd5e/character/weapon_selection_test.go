package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// WeaponSelectionTestSuite tests Character.WeaponForActionRef — the
// toolkit-owned strike-ActionRef-to-weapon mapping (rpg-toolkit#712).
type WeaponSelectionTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *WeaponSelectionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestWeaponSelectionTestSuite(t *testing.T) {
	suite.Run(t, new(WeaponSelectionTestSuite))
}

// createArmedMonkCharacter creates a Monk holding a quarterstaff in the main
// hand — a weapon-holding Monk is exactly the case #712 broke: the resolver
// picked the equipped quarterstaff for unarmed_strike/flurry_strike instead
// of resolving unarmed.
func createArmedMonkCharacter(t *testing.T, bus events.EventBus) *Character {
	t.Helper()

	char := createTestMonkCharacter(t, bus)
	quarterstaff := weapons.All[weapons.Quarterstaff]
	char.inventory = []InventoryItem{
		{Equipment: &quarterstaff, Quantity: 1},
	}
	char.equipmentSlots = EquipmentSlots{
		SlotMainHand: weapons.Quarterstaff,
	}
	return char
}

// --- WeaponForActionRef: nil ref ---

func (s *WeaponSelectionTestSuite) TestWeaponForActionRef_NilRef_ReturnsError() {
	char := createTestMonkCharacter(s.T(), s.bus)

	sel, err := char.WeaponForActionRef(nil)

	s.Require().Error(err)
	s.Nil(sel)
}

// --- unarmed_strike / flurry_strike ignore what's equipped ---

func (s *WeaponSelectionTestSuite) TestUnarmedStrike_WeaponHoldingMonk_ResolvesUnarmed() {
	char := createArmedMonkCharacter(s.T(), s.bus)

	sel, err := char.WeaponForActionRef(refs.Actions.UnarmedStrike())

	s.Require().NoError(err)
	s.Require().NotNil(sel)
	s.Require().NotNil(sel.Weapon)
	s.Equal(weapons.UnarmedStrike, sel.Weapon.ID, "unarmed_strike must resolve the canonical unarmed weapon, not the equipped quarterstaff")
	s.Equal(combat.AttackHandMain, sel.AttackHand)
	s.Nil(sel.TwoWeapon)
}

func (s *WeaponSelectionTestSuite) TestFlurryStrike_WeaponHoldingMonk_ResolvesUnarmed() {
	char := createArmedMonkCharacter(s.T(), s.bus)

	sel, err := char.WeaponForActionRef(refs.Actions.FlurryStrike())

	s.Require().NoError(err)
	s.Require().NotNil(sel)
	s.Require().NotNil(sel.Weapon)
	s.Equal(weapons.UnarmedStrike, sel.Weapon.ID, "flurry_strike must resolve the canonical unarmed weapon, not the equipped quarterstaff")
	s.Equal(combat.AttackHandMain, sel.AttackHand)
	s.Nil(sel.TwoWeapon)
}

func (s *WeaponSelectionTestSuite) TestUnarmedStrike_UnarmedCharacter_ResolvesUnarmed() {
	// A character with nothing equipped should also resolve unarmed —
	// unarmed_strike never depends on equipment state either way.
	char := createTestMonkCharacter(s.T(), s.bus)

	sel, err := char.WeaponForActionRef(refs.Actions.UnarmedStrike())

	s.Require().NoError(err)
	s.Equal(weapons.UnarmedStrike, sel.Weapon.ID)
	s.Equal(combat.AttackHandMain, sel.AttackHand)
}

// --- off_hand_strike selects the off-hand weapon and wires a working context ---

func (s *WeaponSelectionTestSuite) TestOffHandStrike_SelectsOffHandWeapon() {
	char := createTWFCharacter(s.T(), s.bus) // shortsword main, dagger off (both light)

	sel, err := char.WeaponForActionRef(refs.Actions.OffHandStrike())

	s.Require().NoError(err)
	s.Require().NotNil(sel)
	s.Require().NotNil(sel.Weapon)
	s.Equal(weapons.Dagger, sel.Weapon.ID, "off_hand_strike must resolve the off-hand weapon, not the main-hand shortsword")
	s.Equal(combat.AttackHandOff, sel.AttackHand)
	s.Require().NotNil(sel.TwoWeapon, "off-hand resolution must populate a TwoWeaponContext or the rulebook hard-fails")
}

func (s *WeaponSelectionTestSuite) TestOffHandStrike_TwoWeaponContext_ReflectsRealEquippedWeapons() {
	char := createTWFCharacter(s.T(), s.bus)

	sel, err := char.WeaponForActionRef(refs.Actions.OffHandStrike())
	s.Require().NoError(err)
	s.Require().NotNil(sel.TwoWeapon)

	mainHand := sel.TwoWeapon.GetMainHandWeapon(char.GetID())
	offHand := sel.TwoWeapon.GetOffHandWeapon(char.GetID())
	s.Require().NotNil(mainHand)
	s.Require().NotNil(offHand)
	s.Equal(weapons.Shortsword, mainHand.WeaponID)
	s.Equal(weapons.Dagger, offHand.WeaponID)

	// Unknown character id must not leak this character's equipment.
	s.Nil(sel.TwoWeapon.GetMainHandWeapon("someone-else"))
	s.Nil(sel.TwoWeapon.GetOffHandWeapon("someone-else"))
}

func (s *WeaponSelectionTestSuite) TestOffHandStrike_TwoWeaponContext_DoesNotChargeLiveEconomy() {
	// The granted off_hand_strike's bonus action was already spent through
	// the character's own economy upstream of weapon selection (see
	// encounter.spendStrikeEconomy). The TwoWeaponContext handed to the
	// rulebook's off-hand validation must be a disposable stand-in, not the
	// character's live economy, or the validation would double-charge (and
	// with a real economy already at 0 bonus actions, hard-fail).
	char := createTWFCharacter(s.T(), s.bus)
	char.actionEconomy = &ActionEconomyData{
		ActionsRemaining:      1,
		BonusActionsRemaining: 0, // already spent on the granted strike
		ReactionsRemaining:    1,
	}

	sel, err := char.WeaponForActionRef(refs.Actions.OffHandStrike())
	s.Require().NoError(err)
	s.Require().NotNil(sel.TwoWeapon)

	econ := sel.TwoWeapon.GetActionEconomy(char.GetID())
	s.Require().NotNil(econ)
	s.True(econ.CanUseBonusAction(), "TwoWeaponContext must hand the rulebook a usable stand-in, not the character's already-spent live economy")
	s.Require().NoError(econ.UseBonusAction(), "off-hand validation's bonus-action gate must not hard-fail")

	// The character's real economy must be untouched by that call.
	s.Equal(0, char.actionEconomy.BonusActionsRemaining)
}

// TestOffHandStrike_ResolvesThroughAttackChain_NoHardFail drives the
// selection through the real rulebook chain (combat.ResolveAttackHit) to
// prove end-to-end that off_hand_strike no longer hard-fails for lack of a
// TwoWeaponContext — the exact failure #712 called out.
func (s *WeaponSelectionTestSuite) TestOffHandStrike_ResolvesThroughAttackChain_NoHardFail() {
	attacker := createTWFCharacter(s.T(), s.bus)
	defender := createTestFighterCharacter(s.T(), s.bus)

	sel, err := attacker.WeaponForActionRef(refs.Actions.OffHandStrike())
	s.Require().NoError(err)
	s.Require().NotNil(sel.TwoWeapon)

	reg := gamectx.NewCombatantRegistry()
	reg.Add(attacker)
	reg.Add(defender)
	ctx := combat.WithCombatantLookup(s.ctx, reg)
	ctx = combat.WithTwoWeaponContext(ctx, sel.TwoWeapon)

	_, err = combat.ResolveAttackHit(ctx, &combat.ResolveAttackHitInput{
		AttackerID: attacker.GetID(),
		TargetID:   defender.GetID(),
		Weapon:     sel.Weapon,
		EventBus:   s.bus,
		AttackHand: sel.AttackHand,
	})

	s.Require().NoError(err, "off_hand_strike must not hard-fail once a WeaponSelection-built TwoWeaponContext is wired")
}

// --- normal attack/strike refs are unchanged: main-hand weapon ---

func (s *WeaponSelectionTestSuite) TestNormalAttackRef_ResolvesMainHandWeapon() {
	char := createTWFCharacter(s.T(), s.bus)

	sel, err := char.WeaponForActionRef(refs.Actions.Strike())

	s.Require().NoError(err)
	s.Require().NotNil(sel.Weapon)
	s.Equal(weapons.Shortsword, sel.Weapon.ID)
	s.Equal(combat.AttackHandMain, sel.AttackHand)
	s.Nil(sel.TwoWeapon)
}

func (s *WeaponSelectionTestSuite) TestNormalAttackRef_UnarmedCharacter_FallsBackToUnarmed() {
	char := createTestMonkCharacter(s.T(), s.bus) // nothing equipped

	sel, err := char.WeaponForActionRef(refs.Actions.Strike())

	s.Require().NoError(err)
	s.Equal(weapons.UnarmedStrike, sel.Weapon.ID)
	s.Equal(combat.AttackHandMain, sel.AttackHand)
}
