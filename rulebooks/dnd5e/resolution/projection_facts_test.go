// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// The static half of the projection's answer: the numbers a caller needs about
// a character it is only allowed to hold a RECORD of.
//
// These hang off ProjectionTestSuite rather than a suite of their own, so they
// share its fixtures without re-running its tests under a second name.

// The facts come off the reconstituted sheet, and SPEED is the one that proves
// it happened.
//
// Every other field could have been echoed straight out of the record the
// caller already holds. Speed could not: it is not stored anywhere, it is
// derived from race when asked, and a Human's 30 is a number only a loaded
// sheet can produce. If this entry ever starts copying the record instead of
// reading the sheet, this is the assertion that notices.
func (s *ProjectionTestSuite) TestTheFactsComeOffTheLoadedSheet() {
	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: s.barbarian()})
	s.Require().NoError(err)

	s.Equal(projectedHeroID, out.Sheet.ID)
	s.Equal("Standre", out.Sheet.Name)
	s.Equal(1, out.Sheet.Level)
	s.Equal(14, out.Sheet.HitPoints)
	s.Equal(14, out.Sheet.MaxHitPoints)
	s.Equal(2, out.Sheet.ProficiencyBonus)

	s.Equal(30, out.Sheet.SpeedFeet,
		"a Human walks 30 feet, and that number is on no record — only a loaded sheet knows it")
}

// AN EMPTY HAND IS AN UNARMED STRIKE, not an absent attack.
//
// This fixture equips nothing at all, and the main hand still compiles: the
// rules say a creature with empty hands can punch, so character.AssembleAttack
// falls back to the unarmed strike rather than refusing. The projection reports
// what the rules say rather than what the inventory looks like.
func (s *ProjectionTestSuite) TestAnEmptyHandProjectsAnUnarmedStrike() {
	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: s.barbarian()})
	s.Require().NoError(err)

	s.Require().Equal([]string{refs.Weapons.UnarmedStrike().String()}, definitionRefs(out.Actions),
		"empty hands still punch, and a hand with no weapon adds no second attack")
	s.Require().NotNil(out.Actions[0].Attack)
	s.True(out.Actions[0].Attack.Delivery.IsMelee(), "a punch is made in reach")
	s.Equal(5, out.Actions[0].Attack.Delivery.MaxRangeFeet(), "and reach is 5 feet")
}

// A DUAL-WIELDER PROJECTS BOTH HANDS, the way a monster projects every
// definition it carries. The off hand is not an afterthought of the main hand:
// a sheet that dropped it would answer a host that the player has one attack.
func (s *ProjectionTestSuite) TestADualWielderProjectsBothHands() {
	record := s.barbarian()
	record.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial}
	record.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Dagger), Quantity: 1},
	}
	record.EquipmentSlots = character.EquipmentSlots{
		character.SlotMainHand: string(weapons.Shortsword),
		character.SlotOffHand:  string(weapons.Dagger),
	}

	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: record})
	s.Require().NoError(err)

	got := definitionRefs(out.Actions)
	s.Contains(got, refs.Weapons.ByID(string(weapons.Shortsword)).String(), "the main hand")
	s.Contains(got, refs.Weapons.ByID(string(weapons.Dagger)).String(), "and the off hand")
	s.NotContains(got, refs.Weapons.UnarmedStrike().String(), "a held weapon is not also a punch")
}

// A SHIELD IS NOT AN ATTACK. An off hand that holds no weapon adds no
// definition, so the character answers exactly the main hand.
func (s *ProjectionTestSuite) TestAShieldInTheOffHandAddsNoAttack() {
	record := s.barbarian()
	record.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial}
	record.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
		{Type: shared.EquipmentTypeArmor, ID: string(armor.Shield), Quantity: 1},
	}
	record.EquipmentSlots = character.EquipmentSlots{
		character.SlotMainHand: string(weapons.Shortsword),
		character.SlotOffHand:  string(armor.Shield),
	}

	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: record})
	s.Require().NoError(err)

	s.Equal([]string{refs.Weapons.ByID(string(weapons.Shortsword)).String()}, definitionRefs(out.Actions))
}

// TWO OFFERS FOR ONE SWING are ErrConflictingOffers and never ErrBadAttack: a
// monk holding a Shillelagh club has Martial Arts and Shillelagh both offering
// to be the swing, the pick is the player's, and the host must be able to say
// so rather than call the loadout unreadable.
func (s *ProjectionTestSuite) TestCompetingOffersAreNotABadAttack() {
	record := s.barbarian()
	record.ClassID = classes.Monk
	record.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponSimple}
	record.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Club), Quantity: 1},
	}
	record.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: string(weapons.Club)}
	martialArts, err := conditions.NewMartialArtsCondition(
		conditions.MartialArtsInput{MemberID: projectedHeroID}).ToJSON()
	s.Require().NoError(err)
	shillelagh, err := conditions.NewShillelaghCondition(projectedHeroID, conditions.ShillelaghConfig{
		Weapons:    []conditions.HeldWeapon{{Slot: string(character.SlotMainHand), ItemID: string(weapons.Club)}},
		WeaponSlot: string(character.SlotMainHand),
		Ability:    abilities.WIS,
	})
	s.Require().NoError(err)
	shillelaghJSON, err := shillelagh.ToJSON()
	s.Require().NoError(err)
	record.Conditions = []json.RawMessage{martialArts, shillelaghJSON}

	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: record})

	s.Require().Error(err)
	s.Require().Nil(out)
	s.Assert().ErrorIs(err, ErrConflictingOffers)
	s.Assert().ErrorIs(err, character.ErrConflictingOffers, "the root's word still reaches the caller")
	s.Assert().NotErrorIs(err, ErrBadAttack, "the player owes a pick; the loadout is not broken")
}

// definitionRefs is the ref string of each definition, for membership
// assertions that do not pin how many there are.
func definitionRefs(definitions []combatActions.Definition) []string {
	out := make([]string, 0, len(definitions))
	for i := range definitions {
		out = append(out, definitions[i].Ref.String())
	}

	return out
}

// A MAIN HAND THAT WILL NOT COMPILE is ErrBadAttack, not the general
// "this participant is unusable".
//
// The two are different repairs, and the sentinel is what tells them apart: a
// weapon that will not compile is a broken loadout, while a participant that
// will not attach is a broken sheet. This used to answer ErrBadParticipant,
// which a seam could only translate as "the whole character is corrupt" — the
// narrowing #1296 recorded on the other side of the boundary.
//
// The fixture is the reachable case rather than an invented one: a stored sheet
// whose main-hand slot names an item that is not in its inventory. The equip
// path cannot produce that; a persisted record can.
func (s *ProjectionTestSuite) TestAnUncompilableMainHandIsABadAttack() {
	record := s.barbarian()
	record.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: armor.ChainMail}

	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: record})

	s.Require().Error(err)
	s.Require().Nil(out)
	s.Assert().ErrorIs(err, ErrBadAttack,
		"a weapon that will not compile is a broken loadout")
	s.Assert().NotErrorIs(err, ErrBadParticipant,
		"and NOT the general participant refusal — a seam translating that can only "+
			"report the whole character as corrupt")
}

// The facts go with the armour class: a record carrying a condition this
// build cannot parse does not project at all. Facts from a sheet missing an
// effect would be written onto the member as though the sheet were whole.
func (s *ProjectionTestSuite) TestTheFactsDoNotOutliveAnUnreadableCondition() {
	record := s.barbarian(json.RawMessage(`{"ref":"nonsense","x":`))

	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: record})
	s.Require().Error(err)
	s.Require().Nil(out, "no half a projection: no facts without the fold")
	s.Require().Contains(err.Error(), "nonsense")
}
