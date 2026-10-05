// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// OneConditionPerIdentitySuite applies conditions through the sheet's own
// door — a ConditionAppliedEvent on its bus, the path a directly activated
// ability (Hide, Help, a feature) takes without resolution in between.
type OneConditionPerIdentitySuite struct {
	suite.Suite
	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func TestOneConditionPerIdentitySuite(t *testing.T) {
	suite.Run(t, new(OneConditionPerIdentitySuite))
}

func (s *OneConditionPerIdentitySuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	data := fullSheet(&s.Suite)
	data.Conditions = nil
	char, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.char = char
}

func (s *OneConditionPerIdentitySuite) apply(condition dnd5eEvents.ConditionBehavior) {
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: s.char, Type: dnd5eEvents.ConditionType(condition.Ref().ID),
		Source: dnd5eEvents.ConditionSourceCombatAbility, Condition: condition,
	}))
}

func (s *OneConditionPerIdentitySuite) held(address dnd5eEvents.ConditionAddress) []dnd5eEvents.ConditionBehavior {
	var found []dnd5eEvents.ConditionBehavior
	for _, condition := range s.char.GetConditions() {
		if conditions.ConditionAddressOf(s.char.GetID(), condition) == address {
			found = append(found, condition)
		}
	}
	return found
}

func (s *OneConditionPerIdentitySuite) attackFrame() contributions.Frame {
	return contributions.Frame{
		Actor:  s.char.GetID(),
		Target: contributions.Known("goblin"),
		Action: contributions.ActionFacts{
			Roll:       contributions.Known(contributions.RollKindAttack),
			Ability:    contributions.Known(abilities.STR),
			Melee:      contributions.Known(true),
			WeaponPool: contributions.Known(true),
			Advantage:  contributions.Known(false),
		},
	}
}

func (s *OneConditionPerIdentitySuite) TestHidingAgainReplacesTheFirstHidden() {
	first := conditions.NewHiddenCondition(s.char.GetID())
	second := conditions.NewHiddenCondition(s.char.GetID())

	s.apply(first)
	s.apply(second)

	held := s.held(conditions.ConditionAddressOf(s.char.GetID(), second))
	s.Require().Len(held, 1, "exactly one Hidden on the sheet")
	s.Same(second, held[0], "the sheet carries the newer Hidden")
	s.False(first.IsApplied(), "the replaced Hidden's handlers are detached")
	s.True(second.IsApplied())
}

func (s *OneConditionPerIdentitySuite) TestASecondHelpReplacesTheFirst() {
	for name, helpers := range map[string][2]string{
		"same helper":       {"cleric", "cleric"},
		"different helpers": {"cleric", "fighter"},
	} {
		s.SetupTest()
		first := conditions.NewHelpedCondition(s.char.GetID(), helpers[0])
		second := conditions.NewHelpedCondition(s.char.GetID(), helpers[1])

		s.apply(first)
		s.apply(second)

		held := s.held(conditions.ConditionAddressOf(s.char.GetID(), second))
		s.Require().Len(held, 1, "%s: two Helps do not stack", name)
		s.Same(second, held[0], name)
		s.Equal(helpers[1], held[0].(*conditions.HelpedCondition).HelperID, name)
		s.False(first.IsApplied(), "%s: the replaced Helped is detached", name)
	}
}

func (s *OneConditionPerIdentitySuite) TestDifferentSourcesAreDifferentIdentities() {
	a, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
		MemberID: s.char.GetID(), SourceID: "cleric-a", SourceRef: refs.Spells.Bless()})
	s.Require().NoError(err)
	b, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
		MemberID: s.char.GetID(), SourceID: "cleric-b", SourceRef: refs.Spells.Bless()})
	s.Require().NoError(err)

	s.apply(a)
	s.apply(b)

	s.Len(s.held(a.ConditionAddress()), 1)
	s.Len(s.held(b.ConditionAddress()), 1, "a second caster's Bless keeps its own instance")
	s.True(a.IsApplied())
}

// TestEveryBearingConditionAppliedTwiceLeavesOne covers each condition the
// action census classes as answering or not yet answering: applying it twice
// through the sheet's door leaves one, and the effect rows still list.
func (s *OneConditionPerIdentitySuite) TestEveryBearingConditionAppliedTwiceLeavesOne() {
	id := "char-load"
	must := func(c dnd5eEvents.ConditionBehavior, err error) dnd5eEvents.ConditionBehavior {
		s.Require().NoError(err)
		return c
	}
	fixtures := map[string]func() dnd5eEvents.ConditionBehavior{
		"raging": func() dnd5eEvents.ConditionBehavior {
			return &conditions.RagingCondition{CharacterID: id, DamageBonus: 2, Level: 3}
		},
		"sneak attack": func() dnd5eEvents.ConditionBehavior {
			return conditions.NewSneakAttackCondition(conditions.SneakAttackInput{MemberID: id, Level: 1})
		},
		"blessed": func() dnd5eEvents.ConditionBehavior {
			return must(conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
				MemberID: id, SourceID: "cleric", SourceRef: refs.Spells.Bless()}))
		},
		"baned": func() dnd5eEvents.ConditionBehavior {
			return must(conditions.NewBanedCondition(conditions.NewBanedConditionInput{
				MemberID: id, SourceID: "cultist", SourceRef: refs.Spells.Bane()}))
		},
		"inspired": func() dnd5eEvents.ConditionBehavior { return conditions.NewInspiredCondition(id, "bard", "") },
		"shillelagh": func() dnd5eEvents.ConditionBehavior {
			return must(conditions.NewShillelaghCondition(id, conditions.ShillelaghConfig{
				Weapons:    []conditions.HeldWeapon{{Slot: string(SlotMainHand), ItemID: "club"}},
				WeaponSlot: string(SlotMainHand), Ability: abilities.WIS}))
		},
		"brutal critical": func() dnd5eEvents.ConditionBehavior {
			return conditions.NewBrutalCriticalCondition(conditions.BrutalCriticalInput{MemberID: id, Level: 9})
		},
		"archery": func() dnd5eEvents.ConditionBehavior { return conditions.NewFightingStyleArcheryCondition(id) },
		"dueling": func() dnd5eEvents.ConditionBehavior { return conditions.NewFightingStyleDuelingCondition(id) },
		"great weapon": func() dnd5eEvents.ConditionBehavior {
			return conditions.NewFightingStyleGreatWeaponFightingCondition(id, nil)
		},
		"two weapon": func() dnd5eEvents.ConditionBehavior { return conditions.NewFightingStyleTwoWeaponFightingCondition(id) },
		"reckless":   func() dnd5eEvents.ConditionBehavior { return conditions.NewRecklessAttackCondition(id) },
		"prone":      func() dnd5eEvents.ConditionBehavior { return conditions.NewProneCondition(id) },
		"hidden":     func() dnd5eEvents.ConditionBehavior { return conditions.NewHiddenCondition(id) },
		"helped":     func() dnd5eEvents.ConditionBehavior { return conditions.NewHelpedCondition(id, "cleric") },
		"true strike": func() dnd5eEvents.ConditionBehavior {
			return conditions.NewTrueStrikeCondition(id, "goblin", refs.Spells.TrueStrike().String())
		},
		"mockery": func() dnd5eEvents.ConditionBehavior {
			return conditions.NewViciousMockeryCondition(id, "bard", refs.Spells.ViciousMockery().String())
		},
		"martial arts": func() dnd5eEvents.ConditionBehavior {
			return conditions.NewMartialArtsCondition(conditions.MartialArtsInput{MemberID: id, MonkLevel: 1})
		},
		"improved critical": func() dnd5eEvents.ConditionBehavior {
			return conditions.NewImprovedCriticalCondition(conditions.ImprovedCriticalInput{MemberID: id, Threshold: 19})
		},
		"divine favor": func() dnd5eEvents.ConditionBehavior {
			return must(conditions.NewDivineFavorCondition(conditions.NewDivineFavorConditionInput{
				MemberID: id, SourceID: id, SourceRef: refs.Spells.DivineFavor()}))
		},
		"sanctuary": func() dnd5eEvents.ConditionBehavior {
			return must(conditions.NewSanctuaryCondition(conditions.NewSanctuaryConditionInput{
				MemberID: id, SourceID: "cleric", SourceRef: refs.Spells.Sanctuary()}))
		},
		"in fog": func() dnd5eEvents.ConditionBehavior {
			return must(conditions.NewInFogCondition(conditions.NewInFogConditionInput{
				MemberID: id, SourceID: "area-1", SourceRef: refs.Spells.FogCloud()}))
		},
	}
	s.Len(fixtures, 22, "one fixture per answering or not-yet-answering census entry")

	for name, build := range fixtures {
		s.SetupTest()
		first, second := build(), build()
		s.apply(first)
		s.apply(second)

		s.Require().Len(s.held(conditions.ConditionAddressOf(id, second)), 1, name)
		s.False(first.IsApplied(), "%s: the replaced instance is detached", name)

		out, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{
			Conditions: s.char.GetConditions(), Frame: s.attackFrame(),
		})
		s.Require().NoError(err, "%s: effect rows list without a duplicate id", name)
		s.Require().NotNil(out, name)
	}
}
