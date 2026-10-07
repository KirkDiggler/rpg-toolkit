// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type actionCensusSuite struct{ suite.Suite }

func TestActionCensusSuite(t *testing.T) { suite.Run(t, new(actionCensusSuite)) }

func (s *actionCensusSuite) TestEveryConditionLoaderIsClassified() {
	loaders := slices.Sorted(maps.Keys(conditionLoaders))
	census := slices.Sorted(maps.Keys(actionCensus))
	s.Equal(loaders, census, "the census and the loaders name exactly the same refs")

	for ref, entry := range actionCensus {
		switch entry.class {
		case censusAnswers, censusNotYetAnswering:
			s.Contains([]contributions.Participation{contributions.ContributesNow, contributions.LaterChoice},
				entry.participation, ref)
		case censusNotBearing:
			s.Empty(entry.participation, ref)
		default:
			s.Failf("unknown census class", "%s: %q", ref, entry.class)
		}
	}
}

func (s *actionCensusSuite) TestAnsweringLoadersImplementActionAssessor() {
	fixtures := map[string]dnd5eEvents.ConditionBehavior{
		refs.Conditions.Raging().String():    &RagingCondition{CharacterID: "barb", DamageBonus: 2, Level: 1},
		refs.Features.SneakAttack().String(): NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue", Level: 1}),
		refs.Conditions.Inspired().String():  NewInspiredCondition("rogue", "bard", ""),

		refs.Conditions.Prone().String():                            NewProneCondition("rogue"),
		refs.Conditions.Hidden().String():                           NewHiddenCondition("rogue"),
		refs.Conditions.Helped().String():                           NewHelpedCondition("rogue", "cleric"),
		refs.Conditions.TrueStrike().String():                       NewTrueStrikeCondition("rogue", "goblin", refs.Spells.TrueStrike().String()),
		refs.Conditions.ViciousMockery().String():                   NewViciousMockeryCondition("rogue", "bard", refs.Spells.ViciousMockery().String()),
		refs.Conditions.ImprovedCritical().String():                 NewImprovedCriticalCondition(ImprovedCriticalInput{MemberID: "rogue", Threshold: 19}),
		refs.Conditions.FightingStyleArchery().String():             NewFightingStyleArcheryCondition("rogue"),
		refs.Conditions.BrutalCritical().String():                   NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "rogue", Level: 9}),
		refs.Conditions.MartialArts().String():                      NewMartialArtsCondition(MartialArtsInput{MemberID: "rogue", MonkLevel: 1}),
		refs.Conditions.FightingStyleDueling().String():             NewFightingStyleDuelingCondition("rogue"),
		refs.Conditions.FightingStyleGreatWeaponFighting().String(): NewFightingStyleGreatWeaponFightingCondition("rogue", nil),
		refs.Conditions.FightingStyleTwoWeaponFighting().String():   NewFightingStyleTwoWeaponFightingCondition("rogue"),
		refs.Conditions.RecklessAttack().String():                   NewRecklessAttackCondition("rogue"),
	}
	blessed, err := NewBlessedCondition(NewBlessedConditionInput{
		MemberID: "rogue", SourceID: "cleric", SourceRef: refs.Spells.Bless(),
	})
	s.Require().NoError(err)
	fixtures[refs.Conditions.Blessed().String()] = blessed
	baned, err := NewBanedCondition(NewBanedConditionInput{
		MemberID: "rogue", SourceID: "cultist", SourceRef: refs.Spells.Bane(),
	})
	s.Require().NoError(err)
	fixtures[refs.Conditions.Baned().String()] = baned
	favored, err := NewDivineFavorCondition(NewDivineFavorConditionInput{
		MemberID: "rogue", SourceID: "rogue", SourceRef: refs.Spells.DivineFavor(),
	})
	s.Require().NoError(err)
	fixtures[refs.Conditions.DivineFavor().String()] = favored
	enchanted, err := NewShillelaghCondition("rogue", testShillelaghConfig())
	s.Require().NoError(err)
	fixtures[refs.Conditions.Shillelagh().String()] = enchanted

	var answering []string
	for ref, entry := range actionCensus {
		if entry.class == censusAnswers {
			answering = append(answering, ref)
		}
	}
	s.ElementsMatch(answering, slices.Collect(maps.Keys(fixtures)), "one fixture per answering ref")

	for ref, fixture := range fixtures {
		data, err := fixture.ToJSON()
		s.Require().NoError(err)
		loaded, err := LoadJSON(data)
		s.Require().NoError(err, ref)
		s.Equal(ref, loaded.Ref().String())
		_, answers := loaded.(contributions.ActionAssessor)
		s.True(answers, "%s is classified as answering but the loaded condition is not an ActionAssessor", ref)
	}
}

func (s *actionCensusSuite) TestBearingLoadersHaveDescriptions() {
	for ref, entry := range actionCensus {
		if entry.class == censusNotBearing {
			continue
		}
		parsed, err := core.ParseString(ref)
		s.Require().NoError(err, ref)
		display, found := DisplayFor(*parsed)
		s.True(found, "%s bears on actions but has no catalog entry", ref)
		s.NotEmpty(display.Detail, "%s bears on actions but has no description", ref)
	}
}

func (s *actionCensusSuite) TestNotYetAnsweringYieldsUnavailableRow() {
	out, err := AssessActionEffects(&AssessActionEffectsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{s.sanctuary()},
		Frame:      rogueFrame(false),
	})
	s.Require().NoError(err)
	s.Require().Len(out.Effects, 1)
	effect := out.Effects[0]
	s.Equal(refs.Conditions.Sanctuary().String()+"@cleric", effect.ID)
	s.Equal(contributions.StateUnavailable, effect.State)
	s.Equal("This effect cannot yet say whether it applies to this action", effect.Reason)
	s.Equal(displayCatalog[refs.Conditions.Sanctuary().String()].Detail, effect.Description)
	s.Equal(SanctuaryName, effect.Source.Name)
	s.Equal(contributions.ContributesNow, effect.Participation)
	s.Empty(effect.Benefit)
}

// sanctuary is a ward on the rogue, an effect whose rule cannot yet answer.
func (s *actionCensusSuite) sanctuary() *SanctuaryCondition {
	ward, err := NewSanctuaryCondition(NewSanctuaryConditionInput{
		MemberID: "rogue", SourceID: "cleric", SourceRef: refs.Spells.Sanctuary(),
	})
	s.Require().NoError(err)
	return ward
}

// testShillelaghConfig enchants the club in the main hand with Wisdom.
func testShillelaghConfig() ShillelaghConfig {
	return ShillelaghConfig{
		Weapons:    []HeldWeapon{{Slot: "main_hand", ItemID: "club"}},
		WeaponSlot: "main_hand",
		Ability:    abilities.WIS,
	}
}

func (s *actionCensusSuite) TestNotBearingYieldsNoRow() {
	fog, err := NewInFogCondition(NewInFogConditionInput{MemberID: "rogue", SourceID: "area-1", SourceRef: refs.Spells.FogCloud()})
	s.Require().NoError(err)
	out, err := AssessActionEffects(&AssessActionEffectsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{&UnarmoredDefenseCondition{MemberID: "rogue"}, fog},
		Frame:      rogueFrame(false),
	})
	s.Require().NoError(err)
	s.Empty(out.Effects, "In Fog owns no rule, so it shows no row (R20)")

	frame := rogueFrame(false)
	frame.Held = []contributions.MemberHeld{{Member: "goblin", Conditions: []contributions.HeldCondition{
		{Ref: refs.Conditions.InFog().String(), SourceID: "area-1"},
	}}}
	held, err := AssessTargetHeldEffects(&AssessTargetHeldEffectsInput{Frame: frame})
	s.Require().NoError(err)
	s.Empty(held.Effects, "a target in the fog shows no row either (R20)")
}

func (s *actionCensusSuite) TestEffectIDsUniqueAndDeterministic() {
	held := func() []dnd5eEvents.ConditionBehavior {
		a, err := NewBlessedCondition(NewBlessedConditionInput{MemberID: "rogue", SourceID: "a", SourceRef: refs.Spells.Bless()})
		s.Require().NoError(err)
		b, err := NewBlessedCondition(NewBlessedConditionInput{MemberID: "rogue", SourceID: "b", SourceRef: refs.Spells.Bless()})
		s.Require().NoError(err)
		return []dnd5eEvents.ConditionBehavior{
			a, b,
			NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue", Level: 1}),
			NewFightingStyleArcheryCondition("rogue"),
		}
	}
	ids := func() []string {
		out, err := AssessActionEffects(&AssessActionEffectsInput{Conditions: held(), Frame: rogueFrame(false)})
		s.Require().NoError(err)
		var listed []string
		for _, effect := range out.Effects {
			listed = append(listed, effect.ID)
		}
		return listed
	}

	first := ids()
	s.Equal([]string{
		"dnd5e:conditions:blessed@a",
		"dnd5e:conditions:blessed@b",
		refs.Features.SneakAttack().String(),
		refs.Conditions.FightingStyleArchery().String(),
	}, first)
	s.Equal(first, ids(), "the same conditions give the same ids in the same order")
}
