// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const sheetID = "barbarian"

// AppliedTwiceSuite applies each bearing condition twice through a real
// character sheet's door — a ConditionAppliedEvent on its bus, the path a
// directly activated ability takes — and lists effect rows afterwards.
type AppliedTwiceSuite struct {
	suite.Suite
	ctx  context.Context
	bus  events.EventBus
	char *character.Character
}

func TestAppliedTwiceSuite(t *testing.T) { suite.Run(t, new(AppliedTwiceSuite)) }

func (s *AppliedTwiceSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	char, err := character.LoadFromData(s.ctx, &character.Data{
		ID: sheetID, PlayerID: "player-1", Name: "Twice", Level: 1, ProficiencyBonus: 2,
		ClassID: classes.Barbarian, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 14, MaxHitPoints: 14, ArmorClass: 14,
	}, s.bus)
	s.Require().NoError(err)
	s.char = char
}

func (s *AppliedTwiceSuite) must(condition dnd5eEvents.ConditionBehavior, err error) dnd5eEvents.ConditionBehavior {
	s.Require().NoError(err)
	return condition
}

// fixtures builds one instance of each bearing condition, keyed by its loader
// ref. TestEveryBearingCensusEntryHasAFixture holds this set equal to the
// census, so a new bearing loader without a fixture fails.
func (s *AppliedTwiceSuite) fixtures() map[string]func() dnd5eEvents.ConditionBehavior {
	id := sheetID
	return map[string]func() dnd5eEvents.ConditionBehavior{
		refs.Conditions.Raging().String(): func() dnd5eEvents.ConditionBehavior {
			return &conditions.RagingCondition{CharacterID: id, DamageBonus: 2, Level: 3}
		},
		refs.Features.SneakAttack().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewSneakAttackCondition(conditions.SneakAttackInput{MemberID: id, Level: 1})
		},
		refs.Conditions.Blessed().String(): func() dnd5eEvents.ConditionBehavior {
			return s.must(conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
				MemberID: id, SourceID: "cleric", SourceRef: refs.Spells.Bless()}))
		},
		refs.Conditions.Baned().String(): func() dnd5eEvents.ConditionBehavior {
			return s.must(conditions.NewBanedCondition(conditions.NewBanedConditionInput{
				MemberID: id, SourceID: "cultist", SourceRef: refs.Spells.Bane()}))
		},
		refs.Conditions.Inspired().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewInspiredCondition(id, "bard", "")
		},
		refs.Conditions.Shillelagh().String(): func() dnd5eEvents.ConditionBehavior {
			return s.must(conditions.NewShillelaghCondition(id, conditions.ShillelaghConfig{
				Weapons:    []conditions.HeldWeapon{{Slot: "main_hand", ItemID: "club"}},
				WeaponSlot: "main_hand", Ability: abilities.WIS}))
		},
		refs.Conditions.BrutalCritical().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewBrutalCriticalCondition(conditions.BrutalCriticalInput{MemberID: id, Level: 9})
		},
		refs.Conditions.FightingStyleArchery().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewFightingStyleArcheryCondition(id)
		},
		refs.Conditions.FightingStyleDueling().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewFightingStyleDuelingCondition(id)
		},
		refs.Conditions.FightingStyleGreatWeaponFighting().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewFightingStyleGreatWeaponFightingCondition(id, nil)
		},
		refs.Conditions.FightingStyleTwoWeaponFighting().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewFightingStyleTwoWeaponFightingCondition(id)
		},
		refs.Conditions.ImprovedCritical().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewImprovedCriticalCondition(conditions.ImprovedCriticalInput{MemberID: id, Threshold: 19})
		},
		refs.Conditions.RecklessAttack().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewRecklessAttackCondition(id)
		},
		refs.Conditions.MartialArts().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewMartialArtsCondition(conditions.MartialArtsInput{MemberID: id, MonkLevel: 1})
		},
		refs.Conditions.Prone().String():  func() dnd5eEvents.ConditionBehavior { return conditions.NewProneCondition(id) },
		refs.Conditions.Hidden().String(): func() dnd5eEvents.ConditionBehavior { return conditions.NewHiddenCondition(id) },
		refs.Conditions.Helped().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewHelpedCondition(id, "cleric")
		},
		refs.Conditions.TrueStrike().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewTrueStrikeCondition(id, "goblin", refs.Spells.TrueStrike().String())
		},
		refs.Conditions.ViciousMockery().String(): func() dnd5eEvents.ConditionBehavior {
			return conditions.NewViciousMockeryCondition(id, "bard", refs.Spells.ViciousMockery().String())
		},
		refs.Conditions.DivineFavor().String(): func() dnd5eEvents.ConditionBehavior {
			return s.must(conditions.NewDivineFavorCondition(conditions.NewDivineFavorConditionInput{
				MemberID: id, SourceID: id, SourceRef: refs.Spells.DivineFavor()}))
		},
		refs.Conditions.Sanctuary().String(): func() dnd5eEvents.ConditionBehavior {
			return s.must(conditions.NewSanctuaryCondition(conditions.NewSanctuaryConditionInput{
				MemberID: id, SourceID: "cleric", SourceRef: refs.Spells.Sanctuary()}))
		},
	}
}

func (s *AppliedTwiceSuite) TestEveryBearingCensusEntryHasAFixture() {
	s.ElementsMatch(conditions.ActionBearingRefs(), slices.Collect(maps.Keys(s.fixtures())),
		"every condition the action census classes as answering or not yet answering needs a fixture here, and no other")
}

// TestEveryBearingConditionAppliedTwiceLeavesOne: applying each bearing
// condition twice through the sheet's door leaves one, the first instance is
// detached, and the effect rows still list.
func (s *AppliedTwiceSuite) TestEveryBearingConditionAppliedTwiceLeavesOne() {
	fixtures := s.fixtures()
	for _, ref := range conditions.ActionBearingRefs() {
		build, ok := fixtures[ref]
		s.Require().True(ok, "%s has no fixture", ref)
		s.SetupTest()
		first, second := build(), build()
		s.apply(first)
		s.apply(second)

		address := conditions.ConditionAddressOf(sheetID, second)
		held := 0
		for _, condition := range s.char.GetConditions() {
			if conditions.ConditionAddressOf(sheetID, condition) == address {
				held++
			}
		}
		s.Equal(1, held, ref)
		s.False(first.IsApplied(), "%s: the replaced instance is detached", ref)

		out, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{
			Conditions: s.char.GetConditions(),
			Frame: contributions.Frame{
				Actor:  sheetID,
				Target: contributions.Known("goblin"),
				Action: contributions.ActionFacts{
					Roll:       contributions.Known(contributions.RollKindAttack),
					Ability:    contributions.Known(abilities.STR),
					Melee:      contributions.Known(true),
					WeaponPool: contributions.Known(true),
					Advantage:  contributions.Known(false),
				},
			},
		})
		s.Require().NoError(err, "%s: effect rows list without a duplicate id", ref)
		s.Require().NotNil(out, ref)
	}
}

func (s *AppliedTwiceSuite) apply(condition dnd5eEvents.ConditionBehavior) {
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: s.char, Type: dnd5eEvents.ConditionType(condition.Ref().ID),
		Source: dnd5eEvents.ConditionSourceCombatAbility, Condition: condition,
	}))
}
