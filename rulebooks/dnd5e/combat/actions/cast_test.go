// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// CastProfileSuite covers the second profile arm: what a valid cast declares,
// what it refuses, and that a definition still populates exactly one profile.
type CastProfileSuite struct {
	suite.Suite
}

type castLedger struct {
	actions int
	pools   map[coreResources.ResourceKey]int
	writes  int
}

func (l *castLedger) InCombat() bool { return true }
func (l *castLedger) SlotsLeft(slot coreCombat.ActionType) int {
	if slot == coreCombat.ActionStandard {
		return l.actions
	}
	return 0
}
func (l *castLedger) CapacityLeft(combat.CapacityType) int           { return 0 }
func (l *castLedger) PoolLeft(key coreResources.ResourceKey) int     { return l.pools[key] }
func (l *castLedger) SpendSlots(_ coreCombat.ActionType, amount int) { l.writes++; l.actions -= amount }
func (l *castLedger) SpendCapacity(combat.CapacityType, int)         {}
func (l *castLedger) SpendPool(key coreResources.ResourceKey, amount int) {
	l.writes++
	l.pools[key] -= amount
}
func (l *castLedger) BankCapacity(combat.CapacityType, int) {}

func TestCastProfileSuite(t *testing.T) {
	suite.Run(t, new(CastProfileSuite))
}

func (s *CastProfileSuite) TestHealingProfileSupportsOneRangedRecipientOnly() {
	p := actions.CastProfile{RangeFeet: 60, Target: actions.CastTargetOneCreature,
		MinTargets: 1, MaxTargets: 1, Healing: &healing.Declaration{Dice: "1d4"},
		Casting: &combat.SpellCasting{Level: 1, Time: combat.SpellCastingBonusAction}}
	s.Require().NoError(p.Validate())
	encoded, err := json.Marshal(p)
	s.Require().NoError(err)
	var loaded actions.CastProfile
	s.Require().NoError(json.Unmarshal(encoded, &loaded))
	s.Equal(p, loaded)
	for _, mutate := range []func(*actions.CastProfile){
		func(p *actions.CastProfile) { p.MaxTargets = 2 },
		func(p *actions.CastProfile) { p.Damage = []damage.Damage{{Dice: "1d4", Type: damage.Fire}} },
		func(p *actions.CastProfile) { p.Target = actions.CastTargetSelf; p.MinTargets = 0; p.MaxTargets = 0 },
		func(p *actions.CastProfile) { p.Casting.Time = "" },
		func(p *actions.CastProfile) { p.Casting.Level = -1 },
	} {
		invalid := p.Clone()
		mutate(&invalid)
		s.Error(invalid.Validate())
	}
	legacy := p.Clone()
	legacy.Casting = nil
	s.NoError(legacy.Validate(), "absence remains unclassified, not a fabricated casting time")
}

func conditionRef(id string) core.Ref {
	return core.Ref{Module: "dnd5e", Type: "conditions", ID: id}
}

// gatelessProfile is True Strike's shape: a named creature, one condition on
// the caster, no roll anywhere.
func gatelessProfile() actions.CastProfile {
	return actions.CastProfile{
		RangeFeet:  30,
		Target:     actions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
		Effects: []actions.CastEffect{{
			Recipient:      actions.CastRecipientCaster,
			Ref:            conditionRef("true_strike"),
			CounterpartKey: "target_id",
		}},
	}
}

// gatedProfile is Vicious Mockery's shape: a save, damage, and a rider.
func gatedProfile() actions.CastProfile {
	return actions.CastProfile{
		RangeFeet:  60,
		Target:     actions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
		Save:       saves.NewSaveGate(abilities.WIS, 13),
		Damage:     []damage.Damage{{Dice: "1d4", Type: damage.Psychic}},
		Effects: []actions.CastEffect{{
			Recipient:      actions.CastRecipientTarget,
			Ref:            conditionRef("vicious_mockery"),
			CounterpartKey: "source_id",
		}},
	}
}

// gatedDamageProfile is Dissonant Whispers' shape without its move: a save, a
// damage pool, and nothing delivered. The shape half-on-a-save is for.
func gatedDamageProfile() actions.CastProfile {
	return actions.CastProfile{
		RangeFeet:  60,
		Target:     actions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
		Save:       saves.NewSaveGate(abilities.WIS, 13),
		Damage:     []damage.Damage{{Dice: "3d6", Type: damage.Psychic}},
	}
}

func (s *CastProfileSuite) TestAGatelessCastValidates() {
	s.Require().NoError(gatelessProfile().Validate())
}

func (s *CastProfileSuite) TestAGatedCastValidates() {
	s.Require().NoError(gatedProfile().Validate())
}

// A pointer rather than a bool beside a duration: nil is "no concentration",
// and there is no state where a declared duration means nothing.
func (s *CastProfileSuite) TestAConcentrationCastValidates() {
	profile := gatelessProfile()
	profile.Concentration = &actions.CastConcentration{TurnEnds: 2}

	s.Require().NoError(profile.Validate())
	s.Nil(gatelessProfile().Concentration, "and a profile that declares none carries nothing")
}

func (s *CastProfileSuite) TestItRefusesAConcentrationThatEndsBeforeItBegins() {
	profile := gatelessProfile()
	profile.Concentration = &actions.CastConcentration{}

	s.Require().ErrorContains(profile.Validate(), "at least one turn end")
}

func (s *CastProfileSuite) TestCloningACastProfileCopiesItsConcentration() {
	profile := gatelessProfile()
	profile.Concentration = &actions.CastConcentration{TurnEnds: 10, SkipFirstTurnEnd: true}

	clone := profile.Clone()
	clone.Concentration.TurnEnds = 99
	clone.Concentration.SkipFirstTurnEnd = false

	s.Require().NotNil(profile.Concentration)
	s.Equal(10, profile.Concentration.TurnEnds, "a clone that aliased the duration would rewrite the original")
	s.True(profile.Concentration.SkipFirstTurnEnd)
}

func (s *CastProfileSuite) TestConcentrationSkipFirstTurnEndRoundTrips() {
	profile := gatelessProfile()
	profile.Concentration = &actions.CastConcentration{TurnEnds: 10, SkipFirstTurnEnd: true}

	raw, err := json.Marshal(profile)
	s.Require().NoError(err)
	var back actions.CastProfile
	s.Require().NoError(json.Unmarshal(raw, &back))
	s.Require().NotNil(back.Concentration)
	s.True(back.Concentration.SkipFirstTurnEnd)
}

// TestHalfIsPermittedForADamageOnlyGate — half arrived with Dissonant
// Whispers, and a cast that only deals damage is the whole of what it means:
// the pool is rolled and the successful save takes half of the number.
func (s *CastProfileSuite) TestHalfIsPermittedForADamageOnlyGate() {
	p := gatedDamageProfile()
	p.Save.OnSuccess = saves.Half
	s.NoError(p.Validate())
}

// TestHalfIsRefusedWhenAConditionIsDelivered — half a condition means nothing.
// Vicious Mockery's shape delivers a rider, so the refusal stays exactly where
// there is no arithmetic to halve.
func (s *CastProfileSuite) TestHalfIsRefusedWhenAConditionIsDelivered() {
	p := gatedProfile()
	p.Save.OnSuccess = saves.Half
	err := p.Validate()
	s.Require().Error(err)
	s.Contains(err.Error(), "half on a save is only for damage")
}

func (s *CastProfileSuite) TestItRefusesWhatItCannotResolve() {
	s.Run("no range", func() {
		profile := gatelessProfile()
		profile.RangeFeet = 0
		s.Require().ErrorContains(profile.Validate(), "positive range")
	})

	s.Run("unknown target rule", func() {
		profile := gatelessProfile()
		profile.Target = "everybody"
		s.Require().ErrorContains(profile.Validate(), "unknown cast target rule")
	})

	s.Run("creature target minimum is zero", func() {
		profile := gatelessProfile()
		profile.MinTargets = 0
		s.Require().ErrorContains(profile.Validate(), "at least one target")
	})

	s.Run("maximum is below minimum", func() {
		profile := gatelessProfile()
		profile.MinTargets = 3
		profile.MaxTargets = 2
		s.Require().ErrorContains(profile.Validate(), "at least its minimum")
	})

	s.Run("no consequence at all", func() {
		profile := gatelessProfile()
		profile.Effects = nil
		s.Require().ErrorContains(profile.Validate(), "damage or a delivered condition")
	})

	s.Run("a save word neither half nor negated", func() {
		// The profile stopped naming the permitted words and DELEGATES to the
		// gate, so this is the subtest that proves the delegation happens at
		// all: without the p.Save.Validate() call, a garbage word would reach
		// a machine that has no branch for it.
		profile := gatedDamageProfile()
		profile.Save.OnSuccess = "mostly"
		s.Require().ErrorContains(profile.Validate(), "cast save is invalid")
	})

	s.Run("a save that recurs", func() {
		profile := gatedProfile()
		profile.Save.Recurrence = saves.RecurrenceEndOfTurn
		s.Require().ErrorContains(profile.Validate(), "must not recur")
	})

	s.Run("damage marked with the attack ability", func() {
		profile := gatedProfile()
		profile.Damage[0].Properties = []damage.Property{damage.AddsAttackAbilityModifier}
		s.Require().ErrorContains(profile.Validate(), "attack ability modifier")
	})

	s.Run("an effect that is not a condition", func() {
		profile := gatelessProfile()
		profile.Effects[0].Ref = core.Ref{Module: "dnd5e", Type: "spells", ID: "true-strike"}
		s.Require().ErrorContains(profile.Validate(), "dnd5e:conditions")
	})

	s.Run("an unknown recipient", func() {
		profile := gatelessProfile()
		profile.Effects[0].Recipient = "the room"
		s.Require().ErrorContains(profile.Validate(), "unknown cast effect recipient")
	})

	s.Run("a self cast delivering to a target", func() {
		profile := gatelessProfile()
		profile.Target = actions.CastTargetSelf
		profile.MinTargets = 0
		profile.MaxTargets = 0
		profile.Effects[0].Recipient = actions.CastRecipientTarget
		s.Require().ErrorContains(profile.Validate(), "delivers only to the caster")
	})

	s.Run("a self cast binding a counterpart", func() {
		profile := gatelessProfile()
		profile.Target = actions.CastTargetSelf
		profile.MinTargets = 0
		profile.MaxTargets = 0
		s.Require().ErrorContains(profile.Validate(), "no counterpart to bind")
	})
}

func (s *CastProfileSuite) TestLevelOneCastPriceRefusalPreservesActionAndPoolAtomically() {
	price := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Pools: map[coreResources.ResourceKey]int{resources.SpellSlotLevel1: 1},
	}

	for _, test := range []struct {
		name    string
		actions int
		pool    int
	}{
		{name: "action shortage", actions: 0, pool: 1},
		{name: "pool shortage", actions: 1, pool: 0},
	} {
		s.Run(test.name, func() {
			ledger := &castLedger{actions: test.actions, pools: map[coreResources.ResourceKey]int{
				resources.SpellSlotLevel1: test.pool,
			}}
			s.False(combat.CanPay(ledger, price))
			s.Require().Error(combat.Pay(ledger, price))
			s.Zero(ledger.writes)
			s.Equal(test.actions, ledger.actions)
			s.Equal(test.pool, ledger.pools[resources.SpellSlotLevel1])
		})
	}
}

func (s *CastProfileSuite) TestADefinitionWithTwoProfilesIsRefused() {
	definition := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "vicious-mockery"},
		Name: "Vicious Mockery",
		Cast: ptr(gatedProfile()),
		Attack: &actions.AttackProfile{
			Category: actions.AttackCategorySpell,
			Delivery: actions.AttackDelivery{Ranged: &actions.RangedDelivery{NormalFeet: 60}},
			Damage:   []damage.Damage{{Dice: "1d4", Type: damage.Psychic}},
		},
	}

	s.Require().ErrorContains(definition.Validate(), "exactly one profile")
}

func (s *CastProfileSuite) TestADefinitionWithNoProfileIsStillRefused() {
	definition := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "mage-hand"},
		Name: "Mage Hand",
	}

	s.Require().ErrorContains(definition.Validate(), "exactly one profile")
}

func (s *CastProfileSuite) TestADefinitionWithACastProfileValidates() {
	definition := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "vicious-mockery"},
		Name: "Vicious Mockery",
		Cast: ptr(gatedProfile()),
	}

	s.Require().NoError(definition.Validate())
}

func (s *CastProfileSuite) TestAnInvalidCastProfileFailsTheDefinition() {
	profile := gatedProfile()
	profile.RangeFeet = 0

	definition := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "vicious-mockery"},
		Name: "Vicious Mockery",
		Cast: &profile,
	}

	s.Require().ErrorContains(definition.Validate(), "cast profile is invalid")
}

func (s *CastProfileSuite) TestCloningACastDefinitionAliasesNothing() {
	profile := gatedProfile()
	profile.Effects[0].Parameters = json.RawMessage(`{"note":"original"}`)
	original := actions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "spells", ID: "vicious-mockery"},
		Name: "Vicious Mockery",
		Cast: &profile,
	}

	clone := original.Clone()

	s.Require().NotSame(original.Cast, clone.Cast)
	clone.Cast.RangeFeet = 5
	clone.Cast.Damage[0].Dice = "9d9"
	clone.Cast.Effects[0].Ref.ID = "rewritten"
	clone.Cast.Effects[0].Parameters[2] = 'X'
	clone.Cast.Save.Abilities[0] = abilities.STR

	s.Equal(60, original.Cast.RangeFeet)
	s.Equal("1d4", original.Cast.Damage[0].Dice)
	s.Equal("vicious_mockery", original.Cast.Effects[0].Ref.ID)
	s.JSONEq(`{"note":"original"}`, string(original.Cast.Effects[0].Parameters))
	s.Equal(abilities.WIS, original.Cast.Save.Abilities[0])
}

func ptr(profile actions.CastProfile) *actions.CastProfile { return &profile }

// commandProfile is Command's shape: a gate, a menu, and one condition whose
// parameters are filled from the word the caster chose.
func commandProfile() actions.CastProfile {
	return actions.CastProfile{
		RangeFeet:  60,
		Target:     actions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
		Save:       saves.NewSaveGate(abilities.WIS, 13),
		Options: []actions.CastOption{
			{ID: "approach", Label: "Approach"},
			{ID: "flee", Label: "Flee"},
		},
		Effects: []actions.CastEffect{{
			Recipient:      actions.CastRecipientTarget,
			Ref:            conditionRef("commanded"),
			CounterpartKey: "caster_id",
			OptionKey:      "word",
		}},
	}
}

// TestACastWithAMenuValidates — the menu is a cast-time input the way an aimed
// cell is, so a profile carrying one is an ordinary profile with one more
// field, not a new kind of cast.
func (s *CastProfileSuite) TestACastWithAMenuValidates() {
	s.Require().NoError(commandProfile().Validate())
	s.Empty(gatelessProfile().Options, "and a spell with no choice to make declares nothing")
}

// TestTheMenuAndTheKeyAreBoundInBothDirections — each half is useless without
// the other, and each failure is silent rather than loud without this refusal.
// A menu no effect reads is an affordance with nothing behind it: the client
// would draw a picker and the choice would land nowhere. A key on a profile
// with no menu is a parameter that can never be filled, so the condition would
// be built with the field the content promised left empty.
func (s *CastProfileSuite) TestTheMenuAndTheKeyAreBoundInBothDirections() {
	s.Run("a menu nothing reads is refused", func() {
		profile := commandProfile()
		profile.Effects[0].OptionKey = ""
		s.Require().ErrorContains(profile.Validate(), "no effect reads it")
	})

	s.Run("a key with no menu is refused", func() {
		profile := commandProfile()
		profile.Options = nil
		s.Require().ErrorContains(profile.Validate(), "option key")
	})
}

// TestTheMenuIsRefusedWhenTheClientCouldNotDrawIt — every id is what the
// request sends back and every label is what a person reads, so neither may be
// missing and no two rows may answer to the same id.
func (s *CastProfileSuite) TestTheMenuIsRefusedWhenTheClientCouldNotDrawIt() {
	s.Run("an empty id", func() {
		profile := commandProfile()
		profile.Options[1].ID = ""
		s.Require().ErrorContains(profile.Validate(), "id")
	})

	s.Run("an empty label", func() {
		profile := commandProfile()
		profile.Options[1].Label = ""
		s.Require().ErrorContains(profile.Validate(), "label")
	})

	s.Run("two rows with one id", func() {
		profile := commandProfile()
		profile.Options[1].ID = "approach"
		s.Require().ErrorContains(profile.Validate(), "duplicate")
	})
}

// TestHasOptionAnswersOnlyForIdsTheProfileListed is the check the door makes
// before it binds a request's word: an id nobody declared must not reach the
// parameters, because the condition would then be built around a word the
// spell never offered.
func (s *CastProfileSuite) TestHasOptionAnswersOnlyForIdsTheProfileListed() {
	profile := commandProfile()
	s.True(profile.HasOption("approach"))
	s.True(profile.HasOption("flee"))
	s.False(profile.HasOption("grovel"), "a word this profile did not list is not one of its options")
	s.False(profile.HasOption(""), "and neither is nothing at all")
	s.False(gatelessProfile().HasOption("approach"), "a profile with no menu offers no option")
}

// TestCloningACastProfileCopiesItsMenu — Clone exists so a running interaction
// cannot be rewritten through the definition it came from, and a slice left
// shared would be exactly that hole.
func (s *CastProfileSuite) TestCloningACastProfileCopiesItsMenu() {
	original := commandProfile()
	clone := original.Clone()

	clone.Options[0].Label = "Grovel"
	s.Equal("Approach", original.Options[0].Label, "editing a clone must not reach the original")
}
