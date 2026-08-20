// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CharacterAttackTestSuite drives the character half of the attack seam: a
// sheet plus an equipped weapon compiles into the same profile a stat block
// does, and the machine runs it without knowing the difference.
type CharacterAttackTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestCharacterAttackSuite(t *testing.T) {
	suite.Run(t, new(CharacterAttackTestSuite))
}

func (s *CharacterAttackTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// heroSheet is a fighter's persisted sheet: STR 16 (+3), DEX 14 (+2),
// proficiency +2, and whatever weapons and grants the case needs.
//
// Ability scores are chosen so STR and DEX are DIFFERENT (+3 vs +2). A sheet
// where they tie cannot tell a finesse derivation from a Strength one.
func (s *CharacterAttackTestSuite) heroSheet(
	profs []proficiencies.Weapon, equipped map[character.InventorySlot]string, conds ...json.RawMessage,
) *character.Data {
	inventory := make([]character.InventoryItemData, 0, len(equipped))
	slots := character.EquipmentSlots{}
	for slot, id := range equipped {
		inventory = append(inventory, character.InventoryItemData{
			Type: shared.EquipmentTypeWeapon, ID: id, Quantity: 1,
		})
		slots[slot] = id
	}

	return &character.Data{
		ID:       heroID,
		PlayerID: "player-1",
		Name:     "Grog",
		Level:    1,
		ClassID:  classes.Fighter,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:           14,
		MaxHitPoints:        14,
		ArmorClass:          14,
		ProficiencyBonus:    2,
		WeaponProficiencies: profs,
		Inventory:           inventory,
		EquipmentSlots:      slots,
		Conditions:          conds,
	}
}

// martialHero is the common case: proficient with everything, longsword in
// the main hand.
func (s *CharacterAttackTestSuite) martialHero(conds ...json.RawMessage) *character.Data {
	return s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial},
		map[character.InventorySlot]string{character.SlotMainHand: string(weapons.Longsword)},
		conds...,
	)
}

// load reconstitutes a sheet into the live character the compiler reads.
func (s *CharacterAttackTestSuite) load(data *character.Data) *character.Character {
	c, err := character.Load(s.ctx, data)
	s.Require().NoError(err)

	return c
}

func (s *CharacterAttackTestSuite) compile(
	data *character.Data, in *CharacterAttackInput,
) AttackProfile {
	profile, err := AttackFromCharacter(s.load(data), in)
	s.Require().NoError(err)

	return profile
}

func (s *CharacterAttackTestSuite) mainHand() *CharacterAttackInput {
	return &CharacterAttackInput{Slot: character.SlotMainHand}
}

// world puts the hero next to the wolf. Adjacency matters only to prone's
// range predicate, which nobody here triggers.
func (s *CharacterAttackTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

func (s *CharacterAttackTestSuite) swingAt(
	hero *character.Data, profile AttackProfile, roller *sequenceRoller,
) (*Output, error) {
	return Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: roller,
		World: s.world(),
		Participants: []Participant{
			{Character: hero},
			{Monster: s.wolfData()},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: heroID,
			TargetID:   wolfID,
			Attack:     profile,
			Roller:     roller,
		}),
	})
}

func (s *CharacterAttackTestSuite) wolfData() *monster.Data {
	return monsters.NewWolf(wolfID).ToData()
}

func (s *CharacterAttackTestSuite) outcomeOf(out *Output) StrikeOutcome {
	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok, "a strike produces a StrikeOutcome")

	return outcome
}

func (s *CharacterAttackTestSuite) raging() json.RawMessage {
	raw, err := (&conditions.RagingCondition{
		CharacterID: heroID,
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

// ---------------------------------------------------------------------------
// 1. THE HEADLINE: a hero's own longsword takes a wolf's hit points.
// ---------------------------------------------------------------------------

// The sheet and the equipped weapon are the whole input. One compilation step
// later the wolf has exactly dice-plus-modifier fewer hit points, through the
// same machine the wolf's own bite drives.
func (s *CharacterAttackTestSuite) TestAHeroSwingsTheirLongswordAtAWolf() {
	profile := s.compile(s.martialHero(), s.mainHand())

	// STR 16 (+3) plus proficiency +2 to hit; the +3 remains separately attributable.
	s.Require().Equal(refs.Weapons.Longsword(), profile.Ref)
	s.Require().Equal(3+2, profile.AttackBonus)
	s.Require().Equal([]damage.Damage{{
		Dice:       "1d8",
		Type:       damage.Slashing,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
	}}, profile.Damage)
	s.Require().Equal(abilities.STR, profile.AbilityUsed)
	s.Require().Equal(3, profile.AbilityModifier)
	s.Require().Nil(profile.Gate, "a weapon declares no rider")

	out, err := s.swingAt(s.martialHero(), profile,
		&sequenceRoller{singles: []int{15}, pair: []int{5}, fallback: 2})
	s.Require().NoError(err)

	outcome := s.outcomeOf(out)
	s.Require().True(outcome.Hit, "15 + 5 beats the wolf's AC 13")
	s.Require().False(outcome.Critical)
	s.Require().Equal(5+3, outcome.Damage, "1d8 rolled [5], plus the +3 — exact, not merely positive")
	s.Require().Nil(outcome.Contest, "no gate, no save")

	s.Require().Len(out.DirtyMonsters, 1)
	s.Require().Equal(wolfID, out.DirtyMonsters[0].ID)
	s.Require().Equal(11-8, out.DirtyMonsters[0].HitPoints, "the wolf's 11 hit points, less exactly 8")
}

// ---------------------------------------------------------------------------
// 2. Finesse: the better ability wins, and it shows up in BOTH numbers.
// ---------------------------------------------------------------------------

// One sheet, two weapons. The dagger is finesse and this hero's DEX beats
// their STR, so the dagger derives from DEX while the mace — same sheet, same
// turn — still derives from STR. Asserting one weapon alone cannot tell a
// finesse rule from a hard-coded ability.
func (s *CharacterAttackTestSuite) TestFinesseTakesTheBetterAbilityInBothNumbers() {
	// STR 10 (+0), DEX 18 (+4): a spread no tie can hide inside.
	sheet := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple},
		map[character.InventorySlot]string{
			character.SlotMainHand: string(weapons.Dagger),
			character.SlotOffHand:  string(weapons.Mace),
		},
	)
	sheet.AbilityScores[abilities.STR] = 10
	sheet.AbilityScores[abilities.DEX] = 18

	dagger := s.compile(sheet, &CharacterAttackInput{Slot: character.SlotMainHand})
	s.Require().Equal(4+2, dagger.AttackBonus, "DEX +4 plus proficiency +2")
	s.Require().Equal("1d4", dagger.Damage[0].Dice)
	s.Require().Equal(damage.Piercing, dagger.Damage[0].Type)
	s.Require().Equal(abilities.DEX, dagger.AbilityUsed)
	s.Require().Equal(4, dagger.AbilityModifier, "and the DEX modifier is preserved separately")

	mace := s.compile(sheet, &CharacterAttackInput{Slot: character.SlotOffHand})
	s.Require().Equal(0+2, mace.AttackBonus, "no finesse: STR +0 plus proficiency +2")
	s.Require().Equal("1d6", mace.Damage[0].Dice)
	s.Require().Equal(abilities.STR, mace.AbilityUsed)
	s.Require().Zero(mace.AbilityModifier, "a legitimate +0 modifier remains explicit")

	// The whole point, stated as the difference the property makes.
	s.Require().Equal(4, dagger.AttackBonus-mace.AttackBonus,
		"finesse is worth exactly the DEX-over-STR spread")
}

// Finesse picks the BETTER ability, not simply Dexterity: the same dagger on a
// strong, clumsy hero derives from Strength.
func (s *CharacterAttackTestSuite) TestFinesseKeepsStrengthWhenStrengthIsBetter() {
	sheet := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple},
		map[character.InventorySlot]string{character.SlotMainHand: string(weapons.Dagger)},
	)
	sheet.AbilityScores[abilities.STR] = 18 // +4
	sheet.AbilityScores[abilities.DEX] = 10 // +0

	dagger := s.compile(sheet, s.mainHand())
	s.Require().Equal(4+2, dagger.AttackBonus, "STR +4 wins")
	s.Require().Equal("1d4", dagger.Damage[0].Dice)
	s.Require().Equal(abilities.STR, dagger.AbilityUsed)
	s.Require().Equal(4, dagger.AbilityModifier)
}

// A tie is not a choice: identical modifiers produce identical numbers, so the
// default stands rather than flipping on equality.
func (s *CharacterAttackTestSuite) TestFinesseOnATieIsStillTheSameArithmetic() {
	sheet := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple},
		map[character.InventorySlot]string{character.SlotMainHand: string(weapons.Dagger)},
	)
	sheet.AbilityScores[abilities.STR] = 14 // +2
	sheet.AbilityScores[abilities.DEX] = 14 // +2

	dagger := s.compile(sheet, s.mainHand())
	s.Require().Equal(2+2, dagger.AttackBonus)
	s.Require().Equal("1d4", dagger.Damage[0].Dice)
	s.Require().Equal(abilities.STR, dagger.AbilityUsed)
	s.Require().Equal(2, dagger.AbilityModifier)
}

// ---------------------------------------------------------------------------
// 3. Proficiency: present or absent, and exactly the bonus apart.
// ---------------------------------------------------------------------------

// The same hero with the same longsword, granted martial weapons or not. The
// attack bonus differs by exactly the proficiency bonus and the damage does
// not move at all — proficiency buys accuracy, never harm.
func (s *CharacterAttackTestSuite) TestProficiencyIsWorthExactlyTheProficiencyBonus() {
	equipped := map[character.InventorySlot]string{character.SlotMainHand: string(weapons.Longsword)}

	trained := s.compile(
		s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponMartial}, equipped), s.mainHand())
	// Simple weapons only: a longsword is martial, so this grant does not reach it.
	untrained := s.compile(
		s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponSimple}, equipped), s.mainHand())

	s.Require().Equal(3+2, trained.AttackBonus, "STR +3 plus proficiency +2")
	s.Require().Equal(3, untrained.AttackBonus, "STR +3 and nothing else")
	s.Require().Equal(2, trained.AttackBonus-untrained.AttackBonus,
		"exactly the sheet's proficiency bonus apart")

	s.Require().Equal(untrained.Damage, trained.Damage,
		"proficiency never touches the damage pool")
	s.Require().Equal(3, trained.AbilityModifier)
	s.Require().Equal(3, untrained.AbilityModifier)
}

// A sheet with no weapon grants at all adds nothing.
func (s *CharacterAttackTestSuite) TestNoGrantsAddsNoProficiencyBonus() {
	profile := s.compile(
		s.heroSheet(nil, map[character.InventorySlot]string{
			character.SlotMainHand: string(weapons.Longsword),
		}), s.mainHand())

	s.Require().Equal(3, profile.AttackBonus)
}

func (s *CharacterAttackTestSuite) TestNegativeAbilityModifierRemainsSeparateFromPureDice() {
	sheet := s.martialHero()
	sheet.AbilityScores[abilities.STR] = 8

	profile := s.compile(sheet, s.mainHand())

	s.Require().Equal("1d8", profile.Damage[0].Dice)
	s.Require().Equal(-1, profile.AbilityModifier)
	s.Require().Equal(abilities.STR, profile.AbilityUsed)
}

// ---------------------------------------------------------------------------
// 4. A character's critical hit doubles the dice and not the modifier.
// ---------------------------------------------------------------------------

// The ability modifier remains separate from pure "1d8" notation. The strike
// still applies it once on a critical hit: two d8s, one +3.
func (s *CharacterAttackTestSuite) TestACharacterCritDoublesTheDiceNotTheAbilityModifier() {
	profile := s.compile(s.martialHero(), s.mainHand())
	s.Require().Equal("1d8", profile.Damage[0].Dice)
	s.Require().Equal(3, profile.AbilityModifier)

	out, err := s.swingAt(s.martialHero(), profile,
		&sequenceRoller{singles: []int{20}, pair: []int{5, 4}, fallback: 2})
	s.Require().NoError(err)

	outcome := s.outcomeOf(out)
	s.Require().True(outcome.Hit, "a natural 20 always hits")
	s.Require().True(outcome.Critical)
	s.Require().Equal(5+4+3, outcome.Damage,
		"1d8 rolled twice — [5] then [4] — plus the +3 exactly once, not twice")

	// Spelled out, because this is the arithmetic that would silently be wrong
	// if the modifier rode a separate field the machine doubled.
	s.Require().NotEqual(5+4+3+3, outcome.Damage, "the ability modifier must not double")
}

// ---------------------------------------------------------------------------
// 5. THE LOAD-BEARING ROW: the compiler does not know Rage exists.
// ---------------------------------------------------------------------------

// Rage's damage bonus is situational — its own predicate decides per swing —
// so it must arrive through the damage fold and never through the compiler.
// Two things prove the split: the profile is IDENTICAL whether the hero is
// raging or calm, and the extra damage still lands, attributed to Raging.
func (s *CharacterAttackTestSuite) TestARagingHerosBonusArrivesViaTheChainNotTheCompiler() {
	calm := s.compile(s.martialHero(), s.mainHand())
	raging := s.compile(s.martialHero(s.raging()), s.mainHand())

	s.Require().Equal(calm, raging,
		"the compiler reads static facts only — Rage is invisible to it")
	s.Require().Equal("1d8", raging.Damage[0].Dice, "no arithmetic is fused into canonical dice")
	s.Require().Equal(3, raging.AbilityModifier, "only the static ability modifier compiles")
	s.Require().Equal(3+2, raging.AttackBonus)

	out, err := s.swingAt(s.martialHero(s.raging()), raging,
		&sequenceRoller{singles: []int{15}, pair: []int{5}, fallback: 2})
	s.Require().NoError(err)

	outcome := s.outcomeOf(out)
	s.Require().True(outcome.Hit)
	s.Require().Equal(5+3+2, outcome.Damage,
		"1d8 [5] + STR +3 from the compiler, + Rage's +2 from the fold")

	// And the ledger names who did it, which is what makes the attribution a
	// fact rather than an arithmetic coincidence.
	s.Require().True(s.attachedBy(out, refs.Conditions.Raging()),
		"Raging subscribed to the interaction's bus")
}

// The same swing without Rage lands exactly two less — the difference IS the
// fold's contribution, measured rather than asserted.
func (s *CharacterAttackTestSuite) TestTheSameSwingWithoutRageLandsExactlyTwoLess() {
	profile := s.compile(s.martialHero(), s.mainHand())

	out, err := s.swingAt(s.martialHero(), profile,
		&sequenceRoller{singles: []int{15}, pair: []int{5}, fallback: 2})
	s.Require().NoError(err)

	s.Require().Equal(5+3, s.outcomeOf(out).Damage)
	s.Require().False(s.attachedBy(out, refs.Conditions.Raging()), "nobody is raging here")
}

// The other direction, and the one that proves AbilityUsed is derived rather
// than hard-coded: Rage pays out on melee STRENGTH attacks. The same raging
// hero swinging a finesse dagger off DEX gets the weapon's damage and NOT the
// rage bonus — 5e's rule, and it only works because the compiler reported
// which ability it actually chose.
func (s *CharacterAttackTestSuite) TestRageDoesNotPayOutOnADexterityFinesseSwing() {
	equipped := map[character.InventorySlot]string{character.SlotMainHand: string(weapons.Dagger)}
	grants := []proficiencies.Weapon{proficiencies.WeaponSimple}

	sheet := s.heroSheet(grants, equipped, s.raging())
	sheet.AbilityScores[abilities.STR] = 10 // +0
	sheet.AbilityScores[abilities.DEX] = 18 // +4

	profile := s.compile(sheet, s.mainHand())
	s.Require().Equal(abilities.DEX, profile.AbilityUsed, "finesse chose DEX")
	s.Require().Equal("1d4", profile.Damage[0].Dice)
	s.Require().Equal(4, profile.AbilityModifier)

	freshSheet := s.heroSheet(grants, equipped, s.raging())
	freshSheet.AbilityScores[abilities.STR] = 10
	freshSheet.AbilityScores[abilities.DEX] = 18

	out, err := s.swingAt(freshSheet, profile,
		&sequenceRoller{singles: []int{15}, pair: []int{3}, fallback: 2})
	s.Require().NoError(err)

	outcome := s.outcomeOf(out)
	s.Require().True(outcome.Hit)
	s.Require().Equal(3+4, outcome.Damage,
		"1d4 [3] + DEX +4, and no rage bonus — Rage is a Strength rule")
	s.Require().True(s.attachedBy(out, refs.Conditions.Raging()),
		"Raging was attached and simply declined to contribute")
}

// A Strength swing by the same raging hero DOES pay out, which is what makes
// the case above a rule rather than a broken subscription.
func (s *CharacterAttackTestSuite) TestRagePaysOutOnTheStrengthSwing() {
	profile := s.compile(s.martialHero(s.raging()), s.mainHand())
	s.Require().Equal(abilities.STR, profile.AbilityUsed)

	out, err := s.swingAt(s.martialHero(s.raging()), profile,
		&sequenceRoller{singles: []int{15}, pair: []int{5}, fallback: 2})
	s.Require().NoError(err)

	s.Require().Equal(5+3+2, s.outcomeOf(out).Damage)
}

// attachedBy reports whether any subscription in the ledger was made by the
// named effect.
func (s *CharacterAttackTestSuite) attachedBy(out *Output, effect *core.Ref) bool {
	for _, hook := range out.Hooks {
		if hook.Effect.Module == effect.Module &&
			hook.Effect.Type == effect.Type &&
			hook.Effect.ID == effect.ID {
			return true
		}
	}

	return false
}

// ---------------------------------------------------------------------------
// 6. Determinism.
// ---------------------------------------------------------------------------

// The same sheet compiled and resolved twice produces the same registration
// list, in the same order. Map iteration over equipment or proficiencies would
// show up here as a flake.
func (s *CharacterAttackTestSuite) TestTwoIdenticalRunsRegisterTheSameHooksInOrder() {
	first, err := s.swingAt(s.martialHero(s.raging()), s.compile(s.martialHero(s.raging()), s.mainHand()),
		&sequenceRoller{singles: []int{15}, pair: []int{5}, fallback: 2})
	s.Require().NoError(err)

	second, err := s.swingAt(s.martialHero(s.raging()), s.compile(s.martialHero(s.raging()), s.mainHand()),
		&sequenceRoller{singles: []int{15}, pair: []int{5}, fallback: 2})
	s.Require().NoError(err)

	s.Require().Equal(s.hookShape(first), s.hookShape(second))
	s.Require().NotEmpty(s.hookShape(first), "something was attached, so the comparison means something")
	s.Require().Equal(s.outcomeOf(first).Damage, s.outcomeOf(second).Damage)
}

// hookShape reduces the ledger to what is stable across runs: who attached
// what topic, in order. Subscription IDs are the bus's own counter and are
// deliberately excluded.
func (s *CharacterAttackTestSuite) hookShape(out *Output) []string {
	shape := make([]string, 0, len(out.Hooks))
	for _, hook := range out.Hooks {
		shape = append(shape, hook.Participant+"|"+hook.Effect.String()+"|"+string(hook.Topic))
	}

	return shape
}

// The compiler itself is deterministic: same sheet in, identical profile out.
func (s *CharacterAttackTestSuite) TestTheCompilerIsDeterministic() {
	first := s.compile(s.martialHero(), s.mainHand())
	second := s.compile(s.martialHero(), s.mainHand())

	s.Require().Equal(first, second)
}

func (s *CharacterAttackTestSuite) TestCompiledDamagePropertiesDoNotAliasWeaponCatalogContent() {
	profile := s.compile(s.martialHero(), s.mainHand())
	s.Require().Equal([]damage.Property{damage.AddsAttackAbilityModifier}, profile.Damage[0].Properties)

	original := profile.Damage[0].Properties[0]
	defer func() { profile.Damage[0].Properties[0] = original }()
	profile.Damage[0].Properties[0] = damage.DoesNotCrit

	fresh := s.compile(s.martialHero(), s.mainHand())
	s.Require().Equal([]damage.Property{damage.AddsAttackAbilityModifier}, fresh.Damage[0].Properties)
}

// ---------------------------------------------------------------------------
// Versatile, and the refusals.
// ---------------------------------------------------------------------------

// A versatile weapon gripped in two hands steps its die and moves nothing
// else: same ref, same bonus, same damage type, one bigger die.
func (s *CharacterAttackTestSuite) TestAVersatileWeaponStepsItsDieAndNothingElse() {
	oneHanded := s.compile(s.martialHero(), s.mainHand())
	twoHanded := s.compile(s.martialHero(),
		&CharacterAttackInput{Slot: character.SlotMainHand, TwoHanded: true})

	s.Require().Equal("1d8", oneHanded.Damage[0].Dice)
	s.Require().Equal("1d10", twoHanded.Damage[0].Dice)

	s.Require().Equal(oneHanded.AttackBonus, twoHanded.AttackBonus, "the grip does not change accuracy")
	s.Require().Equal(oneHanded.Ref, twoHanded.Ref)
	s.Require().Equal(oneHanded.Damage[0].Type, twoHanded.Damage[0].Type)
	s.Require().Equal(oneHanded.Damage[0].FlatBonus, twoHanded.Damage[0].FlatBonus)
	s.Require().Equal(oneHanded.Damage[0].Properties, twoHanded.Damage[0].Properties)
	s.Require().Equal(oneHanded.AbilityModifier, twoHanded.AbilityModifier)
}

// A non-versatile weapon in two hands is the same weapon: the flag asks the
// weapon a question, and a greatsword's answer is its own 2d6.
func (s *CharacterAttackTestSuite) TestTwoHandedOnANonVersatileWeaponChangesNothing() {
	sheet := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponMartial},
		map[character.InventorySlot]string{character.SlotMainHand: string(weapons.Greatsword)},
	)

	oneHanded := s.compile(sheet, s.mainHand())
	twoHanded := s.compile(sheet, &CharacterAttackInput{Slot: character.SlotMainHand, TwoHanded: true})

	s.Require().Equal("2d6", oneHanded.Damage[0].Dice)
	s.Require().Equal(3, oneHanded.AbilityModifier)
	s.Require().Equal(oneHanded, twoHanded)
}

func (s *CharacterAttackTestSuite) TestRefusesWhatItCannotCompile() {
	equippedLongsword := map[character.InventorySlot]string{
		character.SlotMainHand: string(weapons.Longsword),
	}

	s.Run("a ranged weapon, because the strike has no range semantics", func() {
		sheet := s.heroSheet(
			[]proficiencies.Weapon{proficiencies.WeaponMartial},
			map[character.InventorySlot]string{character.SlotMainHand: string(weapons.Longbow)},
		)
		_, err := AttackFromCharacter(s.load(sheet), s.mainHand())
		s.Require().ErrorIs(err, ErrBadAttack)
	})

	s.Run("a thrown MELEE weapon is not ranged — a dagger still compiles", func() {
		sheet := s.heroSheet(
			[]proficiencies.Weapon{proficiencies.WeaponSimple},
			map[character.InventorySlot]string{character.SlotMainHand: string(weapons.Dagger)},
		)
		profile, err := AttackFromCharacter(s.load(sheet), s.mainHand())
		s.Require().NoError(err, "a dagger carries a thrown range but is a melee weapon")
		s.Require().Equal("1d4", profile.Damage[0].Dice)
		s.Require().Equal(3, profile.AbilityModifier)
	})

	s.Run("an empty slot, rather than a silent unarmed strike", func() {
		sheet := s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponMartial}, nil)
		_, err := AttackFromCharacter(s.load(sheet), s.mainHand())
		s.Require().ErrorIs(err, ErrBadAttack)
	})

	s.Run("a slot holding nothing this build calls a weapon", func() {
		sheet := s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponMartial}, equippedLongsword)
		_, err := AttackFromCharacter(s.load(sheet), &CharacterAttackInput{Slot: character.SlotArmor})
		s.Require().ErrorIs(err, ErrBadAttack)
	})

	s.Run("no slot named at all", func() {
		sheet := s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponMartial}, equippedLongsword)
		_, err := AttackFromCharacter(s.load(sheet), &CharacterAttackInput{})
		s.Require().ErrorIs(err, ErrBadAttack)
	})

	s.Run("a nil character or nil input", func() {
		_, err := AttackFromCharacter(nil, s.mainHand())
		s.Require().ErrorIs(err, ErrNilInput)

		sheet := s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponMartial}, equippedLongsword)
		_, err = AttackFromCharacter(s.load(sheet), nil)
		s.Require().ErrorIs(err, ErrNilInput)
	})
}
