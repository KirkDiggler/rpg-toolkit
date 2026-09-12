// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// CastContentSuite covers supported cast profiles and unsupported catalog entries.
type CastContentSuite struct {
	suite.Suite
}

func TestCastContentSuite(t *testing.T) {
	suite.Run(t, new(CastContentSuite))
}

func (s *CastContentSuite) TestBaneCompilesItsCompleteLevelOneProfile() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bane, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Equal(*refs.Spells.Bane(), definition.Ref)
	s.Require().NotNil(definition.Cost)
	s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard])
	s.Equal(1, definition.Cost.Pools[resources.SpellSlotLevel1])

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(30, profile.RangeFeet)
	s.Equal(actions.CastTargetOneCreature, profile.Target)
	s.Equal(1, profile.MinTargets)
	s.Equal(3, profile.MaxTargets)
	s.Require().NotNil(profile.Save)
	s.Equal([]abilities.Ability{abilities.CHA}, profile.Save.Abilities)
	s.Equal(13, profile.Save.DC.DC(saves.DCInput{}))
	s.Equal(saves.Negated, profile.Save.OnSuccess)
	s.Equal(saves.RecurrenceNone, profile.Save.Recurrence)
	s.Require().Len(profile.Effects, 1)
	s.Equal(actions.CastRecipientTarget, profile.Effects[0].Recipient)
	s.Equal(*refs.Conditions.Baned(), profile.Effects[0].Ref)
	s.Require().NotNil(profile.Concentration)
	s.Equal(10, profile.Concentration.TurnEnds)
	s.True(profile.Concentration.SkipFirstTurnEnd)
	s.True(spells.HasCastProfile(spells.Bane))
	s.Equal([]spells.Spell{spells.Bane}, spells.Castable([]spells.Spell{spells.Bless, spells.Bane}))
}

func (s *CastContentSuite) TestSacredFlameCarriesOnlyItsSaveAndRadiantDamage() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.SacredFlame, SpellSaveDC: 14})
	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Equal(refs.Spells.SacredFlame().String(), definition.Ref.String())
	s.Equal("Sacred Flame", definition.Name)
	s.Nil(definition.Attack)
	s.Require().NotNil(definition.Cost)
	s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard])
	s.Empty(definition.Cost.Pools, "cantrips spend no spell-slot pool")

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(60, profile.RangeFeet)
	s.Equal(actions.CastTargetOneCreature, profile.Target)
	s.Require().NotNil(profile.Save)
	s.Equal([]abilities.Ability{abilities.DEX}, profile.Save.Abilities)
	s.Equal(14, profile.Save.DC.DC(saves.DCInput{}))
	s.Equal(saves.Negated, profile.Save.OnSuccess)
	s.Equal(saves.RecurrenceNone, profile.Save.Recurrence)
	s.Require().Len(profile.Damage, 1)
	s.Equal("1d8", profile.Damage[0].Dice)
	s.Equal(damage.Radiant, profile.Damage[0].Type)
	s.Empty(profile.Effects, "Sacred Flame leaves no condition behind")
	s.Nil(profile.Concentration)
}

func (s *CastContentSuite) TestClericCastableSubsetDoesNotEnableOtherKnownCantrips() {
	s.Equal([]spells.Spell{spells.SacredFlame},
		spells.Castable([]spells.Spell{spells.Guidance, spells.SacredFlame, spells.Light}))
}

func (s *CastContentSuite) TestViciousMockeryCarriesItsSaveGateAndDamage() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Equal(refs.Spells.ViciousMockery().String(), definition.Ref.String())
	s.Equal("Vicious Mockery", definition.Name)

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(spells.ViciousMockeryRangeFeet, profile.RangeFeet)
	s.Equal(actions.CastTargetOneCreature, profile.Target)

	s.Require().NotNil(profile.Save, "the target contests it")
	s.Equal([]abilities.Ability{abilities.WIS}, profile.Save.Abilities)
	s.Equal(saves.Negated, profile.Save.OnSuccess)
	s.Equal(saves.RecurrenceNone, profile.Save.Recurrence)
	s.Equal(saves.DCKindStatic, profile.Save.DC.Kind())
	s.Equal(13, profile.Save.DC.DC(saves.DCInput{}), "the caster's own DC, written in")

	s.Require().Len(profile.Damage, 1)
	s.Equal(spells.ViciousMockeryDamage, profile.Damage[0].Dice)
	s.Equal(damage.Psychic, profile.Damage[0].Type)

	s.Require().Len(profile.Effects, 1)
	s.Equal(actions.CastRecipientTarget, profile.Effects[0].Recipient)
	s.Equal(refs.Conditions.ViciousMockery().String(), profile.Effects[0].Ref.String())
	s.Equal(spells.ViciousMockeryCasterParameter, profile.Effects[0].CounterpartKey)
}

func (s *CastContentSuite) TestTheDCIsTheCastersRatherThanTheSpells() {
	twelve := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: 12})
	fifteen := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: 15})

	s.Require().NotNil(twelve)
	s.Require().NotNil(fifteen)
	s.Equal(12, twelve.Cast.Save.DC.DC(saves.DCInput{}))
	s.Equal(15, fifteen.Cast.Save.DC.DC(saves.DCInput{}))
}

func (s *CastContentSuite) TestTrueStrikeCarriesNoSave() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.TrueStrike, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Require().NoError(definition.Validate())
	s.Equal(refs.Spells.TrueStrike().String(), definition.Ref.String())

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Nil(profile.Save, "nobody resists True Strike")
	s.Empty(profile.Damage, "and it deals none")
	s.Equal(spells.TrueStrikeRangeFeet, profile.RangeFeet)

	s.Require().Len(profile.Effects, 1)
	s.Equal(actions.CastRecipientCaster, profile.Effects[0].Recipient,
		"the advantage lands on the caster, not the creature named")
	s.Equal(refs.Conditions.TrueStrike().String(), profile.Effects[0].Ref.String())
	s.Equal(spells.TrueStrikeTargetParameter, profile.Effects[0].CounterpartKey)
}

// The retrofit (rpg-project#407 R8): True Strike IS a concentration cantrip,
// and the two turn ends it used to count for itself now live on the profile,
// which is where the owning condition reads them.
func (s *CastContentSuite) TestTrueStrikeDeclaresConcentration() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.TrueStrike, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Require().NotNil(definition.Cast.Concentration)
	s.Equal(spells.TrueStrikeTurnEnds, definition.Cast.Concentration.TurnEnds)
	s.Equal(2, spells.TrueStrikeTurnEnds,
		"the end of the casting turn, then the end of the next one")
}

func (s *CastContentSuite) TestViciousMockeryDeclaresNone() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: 13})

	s.Require().NotNil(definition)
	s.Nil(definition.Cast.Concentration, "an insult that landed needs nobody to hold it")
}

func (s *CastContentSuite) TestACantripWithNoContentMintsNothing() {
	for _, id := range []spells.Spell{spells.MageHand, spells.Light, spells.Prestidigitation} {
		s.Nil(spells.CastDefinition(spells.CastDefinitionInput{Spell: id, SpellSaveDC: 13}), "%s has no cast content in this build", id)
		s.False(spells.HasCastProfile(id))
	}
}

func (s *CastContentSuite) TestASpellThisBuildNeverHeardOfMintsNothing() {
	s.Nil(spells.CastDefinition(spells.CastDefinitionInput{Spell: "song-of-nothing", SpellSaveDC: 13}))
	s.False(spells.HasCastProfile("song-of-nothing"))
}

// TestTheBardsListHoldsWhatThisBuildOffers deliberately does NOT pin the list's
// length.
//
// It used to (s.Len(BardCantrips, 11)), and that assertion tested nothing about
// the list while making every future addition an edit to a test that was never
// about the addition. What is worth pinning is that a named cantrip is present.
func (s *CastContentSuite) TestTheBardsListHoldsWhatThisBuildOffers() {
	s.Contains(spells.BardCantrips, spells.TrueStrike)
	s.Contains(spells.BardCantrips, spells.ViciousMockery)
	s.Contains(spells.BardCantrips, spells.MageHand)
	s.Contains(spells.BardCantrips, spells.Thunderclap)
}

// Castable is the subset of a class's list that has cast content, in the order
// the list gave. Named for the property rather than for a count: the count is
// the part that changes every time a cantrip grows a profile, and a test whose
// NAME goes stale is a test people stop trusting.
//
// Blade Ward leads because spells.BardCantrips opens in book order, and its
// arrival is the moment the bard's cantrip pick stops being "choose 2 of 2".
// Thunderclap trails because it is not on the 2014 book list at all: the list
// is what THIS BUILD offers a bard, book order first and additions after.
func (s *CastContentSuite) TestCastableIsTheSubsetWithProfilesInListOrder() {
	s.Equal([]spells.Spell{spells.BladeWard, spells.TrueStrike, spells.ViciousMockery, spells.Thunderclap},
		spells.Castable(spells.BardCantrips))
}

// TestThunderclapDeclaresAShapeRatherThanATarget is the first content in this
// build that names a region of space instead of a creature.
//
// Everything else the toolkit can cast, it casts at somebody a caller picked.
// This one says WHERE and lets the engine work out WHO — which is the whole
// capability, and the reason the spell is here at all.
func (s *CastContentSuite) TestThunderclapDeclaresAShapeRatherThanATarget() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{
		Spell: spells.Thunderclap, SpellSaveDC: 13,
	})
	s.Require().NotNil(definition, "a spell missing from the byID map mints a nil definition, silently")
	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Require().NoError(profile.Validate())

	s.Equal(actions.CastTargetArea, profile.Target)
	s.Zero(profile.MinTargets, "the caller names nobody")
	s.Zero(profile.MaxTargets)

	s.Require().NotNil(profile.Area)
	s.Equal(actions.AreaRadius, profile.Area.Footprint.Shape)
	s.Equal(spells.ThunderclapRadiusFeet, profile.Area.Footprint.SizeFeet)
	s.Equal(actions.AreaOriginCaster, profile.Area.Footprint.Origin)
	s.Equal(actions.AreaCatchesOthers, profile.Area.Catches,
		`"each creature other than you" — the caster stands in their own burst`)

	s.Require().NotNil(profile.Save)
	s.Equal([]abilities.Ability{abilities.CON}, profile.Save.Abilities)
	s.Require().Len(profile.Damage, 1)
	s.Equal(damage.Thunder, profile.Damage[0].Type)
	s.Equal(spells.ThunderclapDamage, profile.Damage[0].Dice)
}

func (s *CastContentSuite) TestEveryCastableCantripHasAValidDefinition() {
	for _, id := range spells.Castable(spells.BardCantrips) {
		definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: id, SpellSaveDC: 13})
		s.Require().NotNil(definition, "%s", id)
		s.Require().NoError(definition.Validate(), "%s", id)
		s.Nil(definition.Attack, "%s declares a cast, not an attack", id)
		s.Require().NotNil(definition.Cost, "%s", id)
		s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard], "%s", id)
		s.Empty(definition.Cost.Pools, "%s is a cantrip", id)
	}
}

// TestThunderwaveDeclaresACubeAndAPush is the first content in this build that
// MOVES a creature that did not choose to move.
//
// Thunderclap proved a cast can say WHERE and let the engine work out WHO.
// This one proves a cast can say what happens to them afterwards without
// naming a single cell: a policy, a budget, and the layer that owns the map
// deciding where the body actually stops.
func (s *CastContentSuite) TestThunderwaveDeclaresACubeAndAPush() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{
		Spell: spells.Thunderwave, SpellSaveDC: 13,
	})
	s.Require().NotNil(definition, "a spell missing from the byID map mints a nil definition, silently")
	s.Equal(*refs.Spells.Thunderwave(), definition.Ref)
	s.Require().NoError(definition.Validate())

	s.Require().NotNil(definition.Cost)
	s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard])
	s.Equal(1, definition.Cost.Pools[resources.SpellSlotLevel1], "a levelled spell spends a level-1 slot")

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(actions.CastTargetArea, profile.Target)
	s.Zero(profile.MinTargets, "the caller names nobody")
	s.Zero(profile.MaxTargets)

	s.Require().NotNil(profile.Area)
	s.Equal(actions.AreaBox, profile.Area.Footprint.Shape)
	s.Equal(spells.ThunderwaveCubeFeet, profile.Area.Footprint.SizeFeet)
	s.Equal(actions.AreaOriginCasterEdge, profile.Area.Footprint.Origin,
		`"a 15-foot cube originating from you" — the caster is never under it`)
	s.Equal(actions.AreaCatchesOthers, profile.Area.Catches)

	s.Require().NotNil(profile.Save)
	s.Equal([]abilities.Ability{abilities.CON}, profile.Save.Abilities)
	s.Equal(13, profile.Save.DC.DC(saves.DCInput{}))
	s.Equal(saves.Half, profile.Save.OnSuccess, "half exists now; the row promised to flip")

	s.Require().Len(profile.Damage, 1)
	s.Equal(damage.Thunder, profile.Damage[0].Type)
	s.Equal(spells.ThunderwaveDamage, profile.Damage[0].Dice)

	s.Require().NotNil(profile.Move)
	s.Equal(actions.MoveLine, profile.Move.Policy, "straight away from the caster; a shove looks for nothing better")
	s.Equal(spells.ThunderwavePushCells, profile.Move.Cells)
	s.False(profile.Move.Speed, "a fixed ten feet, not the mover's own legs")
	s.Equal(actions.PaysNothing, profile.Move.Pays, "being shoved costs the creature nothing")
	s.False(profile.Move.Provokes, "and fires nobody's reaction on the way")
}

// TestDissonantWhispersRunsTheTargetAwayAndChargesItForIt is the first content
// in this build that makes a creature MOVE ITSELF.
//
// Thunderwave proved a cast can shove a body along a line for free. This one
// proves the other kind: the target is not thrown, it runs — as far as its own
// legs carry it, away from the caster by the ruler, paying its reaction for the
// privilege and drawing every opportunity attack on the way out. Three fields
// Thunderwave left at zero, all non-zero here, and each of them a rule the
// spell's text actually states.
func (s *CastContentSuite) TestDissonantWhispersRunsTheTargetAwayAndChargesItForIt() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{
		Spell: spells.DissonantWhispers, SpellSaveDC: 13,
	})
	s.Require().NotNil(definition, "a spell missing from the byID map mints a nil definition, silently")
	s.Equal(*refs.Spells.DissonantWhispers(), definition.Ref)
	s.Require().NoError(definition.Validate())

	s.Require().NotNil(definition.Cost)
	s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard])
	s.Equal(1, definition.Cost.Pools[resources.SpellSlotLevel1], "a levelled spell spends a level-1 slot")

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(spells.DissonantWhispersRangeFeet, profile.RangeFeet)
	s.Equal(actions.CastTargetOneCreature, profile.Target)
	s.Equal(1, profile.MinTargets, "one creature, named by the caster")
	s.Equal(1, profile.MaxTargets)
	s.Nil(profile.Area, "a named creature is not an area")

	s.Require().NotNil(profile.Save)
	s.Equal([]abilities.Ability{abilities.WIS}, profile.Save.Abilities)
	s.Equal(13, profile.Save.DC.DC(saves.DCInput{}))
	s.Equal(saves.Half, profile.Save.OnSuccess, "a made save still hears the whisper, just quieter")
	s.Equal(saves.RecurrenceNone, profile.Save.Recurrence, "one save, at the moment it lands")

	s.Require().Len(profile.Damage, 1)
	s.Equal(damage.Psychic, profile.Damage[0].Type)
	s.Equal(spells.DissonantWhispersDamage, profile.Damage[0].Dice)
	s.Empty(profile.Effects, "it delivers no condition, which is what lets its save buy half")

	s.Require().NotNil(profile.Move)
	s.Equal(actions.MoveAway, profile.Move.Policy, "as far from the caster as the ruler measures")
	s.True(profile.Move.Speed, "the budget is the mover's own legs, which content cannot know")
	s.Zero(profile.Move.Cells, "and therefore not a fixed count")
	s.Equal(actions.PaysReaction, profile.Move.Pays, "the flight costs the target its reaction")
	s.True(profile.Move.Provokes, "running is running: everyone in reach gets their swing")
}

// TestEveryLevelOneSpellSpendsTheSameSlot — the slot cost is a function of the
// spell's LEVEL and nothing else, so unrelated level-1 spells declare it
// through one call rather than through hand-written profiles that could drift
// apart.
func (s *CastContentSuite) TestEveryLevelOneSpellSpendsTheSameSlot() {
	for _, id := range []spells.Spell{spells.Bane, spells.Thunderwave, spells.DissonantWhispers} {
		definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: id, SpellSaveDC: 13})
		s.Require().NotNil(definition, "%s", id)
		s.Require().NotNil(definition.Cost, "%s", id)
		s.Equal(1, definition.Cost.Pools[resources.SpellSlotLevel1], "%s", id)
		s.Len(definition.Cost.Pools, 1, "%s spends one pool and no other", id)
	}
}

// TestCommandCarriesItsMenuAndBindsBothKeys — Command is the first spell whose
// caster makes a choice at cast time, and the first whose delivered condition
// needs two things filled in by the engine: who cast it, and which word was
// picked.
func (s *CastContentSuite) TestCommandCarriesItsMenuAndBindsBothKeys() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Command, SpellSaveDC: 13})

	s.Require().NotNil(definition, "a spell missing from the byID map mints a nil definition, silently")
	s.Equal(*refs.Spells.Command(), definition.Ref)
	s.Require().NoError(definition.Validate())

	s.Require().NotNil(definition.Cost)
	s.Equal(1, definition.Cost.Slots[coreCombat.ActionStandard])
	s.Equal(1, definition.Cost.Pools[resources.SpellSlotLevel1], "a levelled spell spends a level-1 slot")

	profile := definition.Cast
	s.Require().NotNil(profile)
	s.Equal(spells.CommandRangeFeet, profile.RangeFeet)
	s.Equal(actions.CastTargetOneCreature, profile.Target)
	s.Equal(1, profile.MinTargets, "one creature, named by the caster")
	s.Equal(1, profile.MaxTargets)
	s.Nil(profile.Area, "a named creature is not an area")
	s.Nil(profile.Move, "the spell moves nobody: the condition it leaves is what drives the turn")
	s.Empty(profile.Damage, "a one-word command deals none")

	s.Require().NotNil(profile.Save)
	s.Equal([]abilities.Ability{abilities.WIS}, profile.Save.Abilities)
	s.Equal(13, profile.Save.DC.DC(saves.DCInput{}))
	s.Equal(saves.Negated, profile.Save.OnSuccess, "you obey or you do not; there is no half a word")
	s.Equal(saves.RecurrenceNone, profile.Save.Recurrence, "one save, at the moment it lands")

	// THE PAIRING, not just the presence. An id and the label beside it are
	// two halves of one word: the picker draws the label and the condition
	// stores the id, which the layer driving the compelled turn switches on.
	// Swap two ids and every other assertion here still passes, while a player
	// pressing Approach gets a creature that runs. Asserted as the whole slice
	// because the ORDER is what a picker draws top to bottom.
	s.Equal([]actions.CastOption{
		{ID: spells.CommandWordApproach, Label: "Approach"},
		{ID: spells.CommandWordFlee, Label: "Flee"},
		{ID: spells.CommandWordGrovel, Label: "Grovel"},
	}, profile.Options)
	s.Equal("approach", spells.CommandWordApproach, "the id the request sends back and the condition stores")
	s.Equal("flee", spells.CommandWordFlee)
	s.Equal("grovel", spells.CommandWordGrovel)

	s.True(profile.HasOption(spells.CommandWordApproach))
	s.True(profile.HasOption(spells.CommandWordFlee))
	s.True(profile.HasOption(spells.CommandWordGrovel))
	s.False(profile.HasOption("halt"),
		"Halt is a one-row addition and is not in this slice: it is the word with no route and no effect")

	s.Require().Len(profile.Effects, 1)
	effect := profile.Effects[0]
	s.Equal(actions.CastRecipientTarget, effect.Recipient, "the compulsion lands on whoever failed")
	s.Equal(*refs.Conditions.Commanded(), effect.Ref)
	s.Equal(spells.CommandCasterParameter, effect.CounterpartKey,
		"resolution writes the caster here, because Approach and Flee measure from them")
	s.Equal(spells.CommandWordParameter, effect.OptionKey,
		"and the door writes the chosen word here")
}

// TestEveryCommandOptionIsLabelledForAPersonToRead — the client draws what it
// was sent and infers nothing, so a missing label is a blank button and a
// derived one would be this spell's words written somewhere that does not own
// them.
func (s *CastContentSuite) TestEveryCommandOptionIsLabelledForAPersonToRead() {
	profile := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Command, SpellSaveDC: 13}).Cast

	for _, option := range profile.Options {
		s.NotEmpty(option.ID, "an option the request could not name")
		s.NotEmpty(option.Label, "%s has nothing to draw on a button", option.ID)
	}
}

// TestOnlyCommandOffersAMenu — the option is a cast-time input that every other
// profile leaves at its zero value, and this is the assertion that would catch
// a menu leaking into a spell by a shared helper or a copied row.
func (s *CastContentSuite) TestOnlyCommandOffersAMenu() {
	for id := range spells.SpellData {
		definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: id, SpellSaveDC: 13})
		if definition == nil || id == spells.Command {
			continue
		}
		s.Require().NotNil(definition.Cast, "%s minted a definition with no cast profile", id)
		s.Empty(definition.Cast.Options, "%s declares a menu nobody asked it for", id)
		for _, effect := range definition.Cast.Effects {
			s.Empty(effect.OptionKey, "%s reads an option it never offers", id)
		}
	}
}
