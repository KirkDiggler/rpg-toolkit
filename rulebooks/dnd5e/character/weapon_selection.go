package character

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// WeaponSelection is the toolkit-owned resolution of "which weapon does this
// strike ActionRef swing" for a specific character (rpg-toolkit#712).
//
// Before this, the combat resolver picked the attack weapon from the
// equipped slot / AttackHand alone and never consulted the submitted
// ActionRef: a weapon-holding Monk's unarmed_strike swung the equipped
// weapon instead of resolving unarmed, and off_hand_strike swung the
// main-hand weapon. Callers MUST build combat.AttackInput /
// ResolveAttackHitInput from a WeaponSelection instead of re-deriving the
// weapon choice themselves — "which weapon does this ref swing" is rules
// knowledge the toolkit owns, not something a resolver switches on.
type WeaponSelection struct {
	// Weapon is the resolved weapon to pass on AttackInput.Weapon /
	// ResolveAttackHitInput.Weapon.
	Weapon *weapons.Weapon

	// AttackHand is the hand to pass on AttackInput.AttackHand /
	// ResolveAttackHitInput.AttackHand.
	AttackHand combat.AttackHand

	// TwoWeapon is non-nil iff AttackHand is AttackHandOff. The resolver
	// MUST install it on the resolve context via
	// combat.WithTwoWeaponContext(ctx, sel.TwoWeapon) before calling
	// ResolveAttackHit / ResolveAttack, or the rulebook's off-hand
	// validation hard-fails with "two-weapon context not available".
	//
	// Its GetActionEconomy returns a disposable stand-in ActionEconomy, NOT
	// the character's live one: a granted off_hand_strike already paid its
	// bonus-action cost through the character's own economy (ExecuteAction,
	// upstream of weapon selection — see encounter.spendStrikeEconomy).
	// Wiring the live economy here would consume the bonus action a second
	// time. The stand-in still reports the character's REAL main/off hand
	// weapon IDs, so the rulebook's light-weapon check validates against
	// reality.
	TwoWeapon combat.TwoWeaponContext
}

// WeaponForActionRef resolves the weapon (hand + optional two-weapon
// context) a strike ActionRef swings with. This is the toolkit-owned
// ref-to-weapon mapping:
//
//   - unarmed_strike / flurry_strike: always the canonical unarmed-strike
//     weapon, regardless of what is equipped. A Monk's Martial Arts /
//     Flurry of Blows strikes are unarmed even when holding a weapon.
//   - off_hand_strike: the equipped off-hand weapon (falling back to
//     unarmed if the slot is empty), AttackHand set to "off", with a
//     TwoWeapon context populated so the resolver's off-hand validation
//     does not hard-fail.
//   - anything else (the standard attack/strike refs): the equipped
//     main-hand weapon, falling back to unarmed if the slot is empty —
//     unchanged behavior for the normal attack path.
//
// Returns an error if ref is nil.
func (c *Character) WeaponForActionRef(ref *core.Ref) (*WeaponSelection, error) {
	if ref == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "WeaponForActionRef: ref is nil")
	}

	switch ref.ID {
	case refs.Actions.UnarmedStrike().ID, refs.Actions.FlurryStrike().ID:
		return &WeaponSelection{
			Weapon:     unarmedStrikeWeapon(),
			AttackHand: combat.AttackHandMain,
		}, nil
	case refs.Actions.OffHandStrike().ID:
		return c.offHandWeaponSelection(), nil
	default:
		return c.mainHandWeaponSelection(), nil
	}
}

// mainHandWeaponSelection resolves the equipped main-hand weapon, falling
// back to the canonical unarmed-strike weapon when the slot is empty.
func (c *Character) mainHandWeaponSelection() *WeaponSelection {
	weapon := unarmedStrikeWeapon()
	if equipped := c.GetEquippedSlot(SlotMainHand); equipped != nil {
		if w := equipped.AsWeapon(); w != nil {
			weapon = w
		}
	}
	return &WeaponSelection{Weapon: weapon, AttackHand: combat.AttackHandMain}
}

// offHandWeaponSelection resolves the equipped off-hand weapon, falling
// back to the canonical unarmed-strike weapon when the slot is empty, and
// builds the disposable TwoWeaponContext the rulebook's off-hand validation
// requires.
func (c *Character) offHandWeaponSelection() *WeaponSelection {
	weapon := unarmedStrikeWeapon()
	var offInfo *combat.EquippedWeaponInfo
	if equipped := c.GetEquippedSlot(SlotOffHand); equipped != nil {
		if w := equipped.AsWeapon(); w != nil {
			weapon = w
			offInfo = &combat.EquippedWeaponInfo{WeaponID: w.ID}
		}
	}

	var mainInfo *combat.EquippedWeaponInfo
	if equipped := c.GetEquippedSlot(SlotMainHand); equipped != nil {
		if w := equipped.AsWeapon(); w != nil {
			mainInfo = &combat.EquippedWeaponInfo{WeaponID: w.ID}
		}
	}

	return &WeaponSelection{
		Weapon:     weapon,
		AttackHand: combat.AttackHandOff,
		TwoWeapon: &grantedOffHandContext{
			characterID: c.id,
			mainHand:    mainInfo,
			offHand:     offInfo,
		},
	}
}

// unarmedStrikeWeapon returns the canonical unarmed-strike weapon
// definition from the toolkit catalog. Using the catalog entry (not an ad
// hoc stub) matters: the Martial Arts damage-dice upgrade
// (rulebooks/dnd5e/conditions/martial_arts.go) classifies an attack as
// unarmed by matching the weapon ref against refs.Weapons.UnarmedStrike(),
// so a look-alike stub with a different ID would silently skip the Monk's
// dice upgrade.
func unarmedStrikeWeapon() *weapons.Weapon {
	w, _ := weapons.GetByID(weapons.UnarmedStrike) // always present in weapons.SpecialWeapons
	return &w
}

// grantedOffHandContext is a disposable combat.TwoWeaponContext scoped to
// one character, built fresh at WeaponForActionRef time. It satisfies the
// rulebook's off-hand validation (light-weapon check + bonus-action gate)
// without reading or mutating the character's live ActionEconomy — the
// granted off_hand_strike's bonus-action cost is already paid via the
// character's own ExecuteAction before weapon selection runs.
type grantedOffHandContext struct {
	characterID string
	mainHand    *combat.EquippedWeaponInfo
	offHand     *combat.EquippedWeaponInfo
}

// GetMainHandWeapon implements combat.TwoWeaponContext.
func (g *grantedOffHandContext) GetMainHandWeapon(characterID string) *combat.EquippedWeaponInfo {
	if characterID != g.characterID {
		return nil
	}
	return g.mainHand
}

// GetOffHandWeapon implements combat.TwoWeaponContext.
func (g *grantedOffHandContext) GetOffHandWeapon(characterID string) *combat.EquippedWeaponInfo {
	if characterID != g.characterID {
		return nil
	}
	return g.offHand
}

// GetActionEconomy implements combat.TwoWeaponContext. It returns a fresh
// stand-in ActionEconomy (bonus action available) rather than the
// character's live economy — see the TwoWeapon field doc on WeaponSelection
// for why.
func (g *grantedOffHandContext) GetActionEconomy(characterID string) *combat.ActionEconomy {
	if characterID != g.characterID {
		return nil
	}
	return combat.NewActionEconomy()
}
