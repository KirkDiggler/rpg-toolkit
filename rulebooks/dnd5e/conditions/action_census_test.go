// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
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
		case actionAnswers, actionNotYetAnswering:
			s.Contains([]contributions.Participation{contributions.ContributesNow, contributions.LaterChoice},
				entry.participation, ref)
		case actionNotBearing:
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

		refs.Conditions.Prone().String():                NewProneCondition("rogue"),
		refs.Conditions.Hidden().String():               NewHiddenCondition("rogue"),
		refs.Conditions.Helped().String():               NewHelpedCondition("rogue", "cleric"),
		refs.Conditions.TrueStrike().String():           NewTrueStrikeCondition("rogue", "goblin", refs.Spells.TrueStrike().String()),
		refs.Conditions.ViciousMockery().String():       NewViciousMockeryCondition("rogue", "bard", refs.Spells.ViciousMockery().String()),
		refs.Conditions.ImprovedCritical().String():     NewImprovedCriticalCondition(ImprovedCriticalInput{MemberID: "rogue", Threshold: 19}),
		refs.Conditions.FightingStyleArchery().String(): NewFightingStyleArcheryCondition("rogue"),
		refs.Conditions.BrutalCritical().String():       NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "rogue", Level: 9}),
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

	var answering []string
	for ref, entry := range actionCensus {
		if entry.class == actionAnswers {
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
		if entry.class == actionNotBearing {
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
		Conditions: []dnd5eEvents.ConditionBehavior{NewFightingStyleDuelingCondition("rogue")},
		Frame:      rogueFrame(false),
	})
	s.Require().NoError(err)
	s.Require().Len(out.Effects, 1)
	effect := out.Effects[0]
	s.Equal(refs.Conditions.FightingStyleDueling().String(), effect.ID)
	s.Equal(contributions.StateUnavailable, effect.State)
	s.Equal("This effect cannot yet say whether it applies to this action", effect.Reason)
	s.Equal(displayCatalog[refs.Conditions.FightingStyleDueling().String()].Detail, effect.Description)
	s.Equal("Dueling", effect.Source.Name)
	s.Equal(contributions.ContributesNow, effect.Participation)
	s.Empty(effect.Benefit)
}

func (s *actionCensusSuite) TestNotBearingYieldsNoRow() {
	out, err := AssessActionEffects(&AssessActionEffectsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{&UnarmoredDefenseCondition{MemberID: "rogue"}},
		Frame:      rogueFrame(false),
	})
	s.Require().NoError(err)
	s.Empty(out.Effects)
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
