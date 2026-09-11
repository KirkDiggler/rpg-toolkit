// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type PureLoadTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *PureLoadTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// immunityBlob is a persisted trait. This package cannot route it to a loader
// — monstertraits imports this one — so it is carried as the bytes it is,
// which is exactly what the round trip below is about.
func (s *PureLoadTestSuite) immunityBlob() json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"ref":         refs.MonsterTraits.Immunity(),
		"monster_id":  "skel-load",
		"damage_type": "poison",
	})
	s.Require().NoError(err)

	return raw
}

func (s *PureLoadTestSuite) sheet() *Data {
	return &Data{
		ID:   "skel-load",
		Name: "Skeleton",
		Ref:  refs.Monsters.Skeleton(),
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 6,
			abilities.WIS: 8,
			abilities.CHA: 5,
		},
		HitPoints:        9,
		MaxHitPoints:     13,
		ArmorClass:       13,
		ProficiencyBonus: 2,
		Speed:            SpeedData{Walk: 30},
		Senses:           SensesData{Darkvision: 60, PassivePerception: 9},
		Actions: []combatActions.Definition{{
			Ref:  *refs.MonsterActions.SkeletonShortsword(),
			Name: "shortsword",
			Attack: &combatActions.AttackProfile{
				Category:    combatActions.AttackCategoryWeapon,
				Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
				AttackBonus: 4,
				Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Piercing, FlatBonus: 2}},
			},
		}},
		Proficiencies: []ProficiencyData{{Skill: "stealth", Bonus: 4}},
		Conditions:    []json.RawMessage{s.immunityBlob()},
		Targeting:     TargetLowestHP,
	}
}

func (s *PureLoadTestSuite) marshal(d *Data) string {
	raw, err := json.Marshal(d)
	s.Require().NoError(err)

	return string(raw)
}

// Data in, the same data out, with no bus in the call — actions and conditions included.
func (s *PureLoadTestSuite) TestRoundTripsByteIdenticalWithNoBus() {
	data := s.sheet()

	m, err := Load(s.ctx, data)
	s.Require().NoError(err)

	s.Require().Equal(s.marshal(data), s.marshal(m.ToData()))
}

// The legacy path throws the conditions away. Its callers are expected to pass
// the same blobs to monstertraits.LoadMonsterConditions themselves, and a
// caller who forgets writes the monster back without them — the trap the
// carried blobs close.
func (s *PureLoadTestSuite) TestActionsReturnsDeepClones() {
	m, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)

	actions := m.Actions()
	s.Require().Len(actions, 1)
	actions[0].Name = "changed"
	actions[0].Attack.Damage[0].Dice = "9d9"
	actions[0].Ref.ID = "changed"

	fresh := m.Actions()
	s.Equal("shortsword", fresh[0].Name)
	s.Equal("1d6", fresh[0].Attack.Damage[0].Dice)
	s.Equal("skeleton-shortsword", fresh[0].Ref.ID)
	s.Equal(s.sheet().Actions, m.ToData().Actions)
}

func (s *PureLoadTestSuite) TestAddActionRejectsInvalidOpaqueConditionParameters() {
	m, err := Load(s.ctx, &Data{
		ID:           "bad-action-monster",
		Name:         "Bad Action Monster",
		HitPoints:    5,
		MaxHitPoints: 5,
		ArmorClass:   10,
	})
	s.Require().NoError(err)

	err = m.AddAction(combatActions.Definition{
		Ref:  *refs.MonsterActions.SkeletonShortsword(),
		Name: "Bad Shortsword",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Piercing}},
			OnHit: []combatActions.ConditionApplication{{
				Ref:        *refs.Conditions.Prone(),
				Parameters: json.RawMessage(`{"duration":`),
			}},
		},
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "invalid monster action")
	s.Contains(err.Error(), "parameters")
	s.Empty(m.Actions())
}

func (s *PureLoadTestSuite) TestLegacyLoadDropsTheConditions() {
	m, err := LoadFromData(s.ctx, s.sheet(), events.NewEventBus())
	s.Require().NoError(err)

	s.Require().Empty(m.ToData().Conditions)
}

func (s *PureLoadTestSuite) TestLoadAppliesNothing() {
	m, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)

	s.Require().Nil(m.bus, "a pure load holds no bus")
	s.Require().Empty(m.subscriptionIDs, "a pure load subscribes to nothing")
	s.Require().Empty(m.GetConditions(), "nothing is applied until a trait loader runs")
}

// The blobs are taken, not read: whoever loads them into behaviour clears them,
// so ToData cannot write the same condition twice.
func (s *PureLoadTestSuite) TestTakeUnappliedConditionsDrains() {
	m, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)

	s.Require().Equal([]json.RawMessage{s.immunityBlob()}, m.TakeUnappliedConditions())
	s.Require().Empty(m.TakeUnappliedConditions())
	s.Require().Empty(m.ToData().Conditions)
}

// Strict as far as it can be without a loader: a blob that is not JSON fails
// the load, and the error names it.
func (s *PureLoadTestSuite) TestStrictLoadRefusesAMalformedCondition() {
	data := s.sheet()
	data.Conditions = append(data.Conditions, json.RawMessage(`{"ref":`))

	_, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), `{"ref":`)
}

// A blob with no ref could never be routed to a loader, so accepting it would
// only defer the failure to a place with less to say about it.
func (s *PureLoadTestSuite) TestStrictLoadRefusesAConditionWithNoRef() {
	data := s.sheet()
	data.Conditions = append(data.Conditions, json.RawMessage(`{"monster_id":"skel-load"}`))

	_, err := Load(s.ctx, data)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "names no ref")
}

func (s *PureLoadTestSuite) TestLoadRejectsNilData() {
	_, err := Load(s.ctx, nil)

	s.Require().Error(err)
}

// Two fields of Data survive no loader: a monster has nowhere to hold features
// or inventory and ToData does not write them. Pinned so the round-trip
// guarantee above is read with its actual scope.
// The meter is persisted for the reason SneakAttackData gives for its own
// once-per-turn field: every call reconstructs the sheet from JSON, so a
// runtime-only bool resets on each RPC and meters nothing at all.
func (s *PureLoadTestSuite) TestASpentReactionSurvivesTheRoundTrip() {
	data := s.sheet()
	data.ReactionSpent = true

	m, err := Load(s.ctx, data)
	s.Require().NoError(err)

	s.False(m.CanReact(), "a monster reloaded mid-turn has already spent it")
	s.True(m.ToData().ReactionSpent, "and writes it back out")
}

// The absent field reads as a full meter, which is what every blob written
// before this field existed says.
func (s *PureLoadTestSuite) TestAnOlderBlobWithNoMeterHasItsReaction() {
	m, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)

	s.True(m.CanReact())
	s.False(m.ToData().ReactionSpent)
}

func (s *PureLoadTestSuite) TestKnownRoundTripGaps() {
	data := s.sheet()
	data.Features = []json.RawMessage{json.RawMessage(`{"ref":"whatever"}`)}
	data.Inventory = []InventoryItemData{{ID: "potion", Name: "Potion", Quantity: 1}}

	m, err := Load(s.ctx, data)
	s.Require().NoError(err)

	out := m.ToData()
	s.Require().Empty(out.Features, "Features has no home on a monster")
	s.Require().Empty(out.Inventory, "Inventory has no home on a monster")
}

type liveMonsterCondition struct {
	applied bool
}

func (c *liveMonsterCondition) Ref() *core.Ref  { return refs.Conditions.Prone() }
func (c *liveMonsterCondition) IsApplied() bool { return c.applied }
func (c *liveMonsterCondition) Apply(_ context.Context, _ events.EventBus) error {
	c.applied = true
	return nil
}
func (c *liveMonsterCondition) Remove(_ context.Context, _ events.EventBus) error {
	c.applied = false
	return nil
}
func (c *liveMonsterCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(map[string]any{"ref": refs.Conditions.Prone()})
}

// MonsterKeeperTestSuite drives every row of the monster keeper's
// subscription table. Each one fails outright for a keeper whose Apply
// subscribes nothing, which is what this suite is for: the wiring is invisible,
// so a missing row shows up only as behaviour that quietly stopped happening.
type MonsterKeeperTestSuite struct {
	suite.Suite

	ctx context.Context
	bus events.EventBus
	mon *Monster
}

func (s *MonsterKeeperTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	m, err := Load(s.ctx, &Data{
		ID:           "skel-keeper",
		Name:         "Skeleton",
		HitPoints:    10,
		MaxHitPoints: 13,
		ArmorClass:   13,
	})
	s.Require().NoError(err)
	s.Require().NoError(m.SheetKeeper().Apply(s.ctx, s.bus))

	s.mon = m
}

func (s *MonsterKeeperTestSuite) TestDamageReceivedMovesHitPoints() {
	err := dnd5eEvents.DamageReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: s.mon.GetID(),
		Amount:   4,
	})

	s.Require().NoError(err)
	s.Require().Equal(6, s.mon.HP())
	s.Require().True(s.mon.IsDirty(), "a monster that took damage needs saving")
}

func (s *MonsterKeeperTestSuite) TestHealingReceivedMovesHitPoints() {
	err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.mon.GetID(),
		Amount:   2,
	})

	s.Require().NoError(err)
	s.Require().Equal(12, s.mon.HP())
	s.Require().True(s.mon.IsDirty())
}

// validMonsterHealingCalculation builds a valid sourced healing calculation:
// one 1d10 trace plus the Fighter-level modifier, totalling 7.
func validMonsterHealingCalculation() *dnd5eEvents.RollCalculation {
	modifier := 1
	return &dnd5eEvents.RollCalculation{
		Components: []dnd5eEvents.RollComponent{
			{
				Source: dnd5eEvents.RollSource{Ref: refs.Features.SecondWind(), Name: "Second Wind"},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      "1d10",
					DieSize:       10,
					OriginalRolls: []int{6},
					FinalRolls:    []int{6},
					Subtotal:      6,
				},
			},
			{
				Source: dnd5eEvents.RollSource{
					Ref: refs.Classes.Fighter(), Name: "Fighter", Label: "Fighter level",
				},
				Modifier: &modifier,
			},
		},
		Total: 7,
	}
}

func (s *MonsterKeeperTestSuite) TestHealingReceivedCarriesCalculationToApplied() {
	calculation := validMonsterHealingCalculation()

	var got *dnd5eEvents.HealingAppliedEvent
	_, err := dnd5eEvents.HealingAppliedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, event dnd5eEvents.HealingAppliedEvent) error {
			got = &event
			return nil
		})
	s.Require().NoError(err)

	err = dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID:    s.mon.GetID(),
		Amount:      7,
		SourceRef:   refs.Features.SecondWind(),
		SourceName:  "Second Wind",
		Calculation: calculation,
	})

	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Require().Equal(7, got.Requested)
	s.Require().Equal(3, got.Applied, "10/13 heals to the 13 cap")
	s.Require().Equal(10, got.HPBefore)
	s.Require().Equal(13, got.HPAfter)
	s.Require().NotNil(got.Calculation)
	s.Require().Equal(calculation, got.Calculation)
	s.Require().NotSame(calculation, got.Calculation, "the applied fact owns its calculation")

	calculation.Components[0].Dice.FinalRolls[0] = 9
	calculation.Total = 14
	s.Require().Equal(7, got.Calculation.Total,
		"mutating the received calculation cannot rewrite the applied total")
	s.Require().Equal([]int{6}, got.Calculation.Components[0].Dice.FinalRolls,
		"mutating the received calculation cannot rewrite the applied faces")
}

func (s *MonsterKeeperTestSuite) TestHealingReceivedRejectsInvalidCalculation() {
	testCases := []struct {
		name        string
		amount      int
		calculation *dnd5eEvents.RollCalculation
	}{
		{
			name:        "amount does not match calculation total",
			amount:      8,
			calculation: validMonsterHealingCalculation(), // total 7
		},
		{
			name:        "calculation has no components",
			amount:      7,
			calculation: &dnd5eEvents.RollCalculation{Total: 7},
		},
		{
			name:   "calculation arithmetic disagrees with its components",
			amount: 9,
			calculation: func() *dnd5eEvents.RollCalculation {
				calc := validMonsterHealingCalculation()
				calc.Total = 9 // components sum to 7
				return calc
			}(),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			published := 0
			_, err := dnd5eEvents.HealingAppliedTopic.On(s.bus).Subscribe(
				s.ctx, func(_ context.Context, _ dnd5eEvents.HealingAppliedEvent) error {
					published++
					return nil
				})
			s.Require().NoError(err)

			err = dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
				TargetID:    s.mon.GetID(),
				Amount:      tc.amount,
				SourceRef:   refs.Features.SecondWind(),
				SourceName:  "Second Wind",
				Calculation: tc.calculation,
			})

			s.Require().Error(err, "an invalid calculation is refused")
			s.Require().Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
			s.Zero(published, "no applied fact precedes validation")
			s.Equal(10, s.mon.HP(), "HP is untouched")
			s.False(s.mon.IsDirty(), "the sheet is not dirtied")
		})
	}
}

func (s *MonsterKeeperTestSuite) TestConditionAppliedLandsOnTheMonster() {
	condition := &liveMonsterCondition{}

	err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    s.mon,
		Type:      dnd5eEvents.ConditionProne,
		Condition: condition,
	})

	s.Require().NoError(err)
	s.Require().True(condition.applied)
	s.Require().Len(s.mon.GetConditions(), 1)
	s.Require().True(s.mon.IsDirty())
}

func (s *MonsterKeeperTestSuite) TestEventsForOtherMonstersAreIgnored() {
	err := dnd5eEvents.DamageReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: "someone-else",
		Amount:   4,
	})

	s.Require().NoError(err)
	s.Require().Equal(10, s.mon.HP())
	s.Require().False(s.mon.IsDirty())
}

func (s *MonsterKeeperTestSuite) TestRemoveStopsTheMonsterListening() {
	s.Require().NoError(s.mon.SheetKeeper().Remove(s.ctx, s.bus))

	err := dnd5eEvents.DamageReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: s.mon.GetID(),
		Amount:   4,
	})

	s.Require().NoError(err)
	s.Require().Equal(10, s.mon.HP())
	s.Require().Empty(s.mon.subscriptionIDs)
}

func (s *MonsterKeeperTestSuite) TestApplyingTwiceIsRefused() {
	s.Require().Error(s.mon.SheetKeeper().Apply(s.ctx, s.bus))
}

// A monster's condition removal reaches its sheet, which it did not before:
// the keeper had rows for damage, healing and condition-applied, and none for
// removal. Nothing in production removes a monster's condition yet, so this row
// is the gap closed before its first caller rather than after.
func (s *MonsterKeeperTestSuite) TestConditionRemovedLeavesTheMonster() {
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    s.mon,
		Condition: &liveMonsterCondition{},
	}))
	markSaved(s.mon)

	err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     s.mon.GetID(),
		ConditionRef: refs.Conditions.Prone().String(),
		Reason:       "test",
	})

	s.Require().NoError(err)
	s.Require().Empty(s.mon.GetConditions(), "the condition that ended is off the sheet")
	s.Require().True(s.mon.IsDirty(), "a sheet that lost a condition needs saving")
}

// A removal naming a condition this monster is not carrying changes nothing,
// dirtiness included. Every sheet on the bus hears every removal, so marking on
// a miss would persist every monster in the fight each time one of them lost
// something.
func (s *MonsterKeeperTestSuite) TestConditionRemovedThatMatchesNothingLeavesTheSheetClean() {
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    s.mon,
		Condition: &liveMonsterCondition{},
	}))
	markSaved(s.mon)

	err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     s.mon.GetID(),
		ConditionRef: refs.Conditions.Raging().String(),
	})

	s.Require().NoError(err)
	s.Require().Len(s.mon.GetConditions(), 1, "the condition it does carry is untouched")
	s.Require().False(s.mon.IsDirty(), "nothing changed, so nothing needs saving")
}

func (s *MonsterKeeperTestSuite) TestConditionRemovedFromSomeoneElseIsIgnored() {
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    s.mon,
		Condition: &liveMonsterCondition{},
	}))
	markSaved(s.mon)

	err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     "someone-else",
		ConditionRef: refs.Conditions.Prone().String(),
	})

	s.Require().NoError(err)
	s.Require().Len(s.mon.GetConditions(), 1)
	s.Require().False(s.mon.IsDirty())
}

// A condition reporting a change to its OWN persisted state marks the monster.
// The condition's fields serialize as part of this sheet and nothing else can
// see them move, so without this row a wolf that spent its reaction reloads
// having spent nothing.
func (s *MonsterKeeperTestSuite) TestConditionStateChangedMarksTheMonster() {
	markSaved(s.mon)

	err := dnd5eEvents.ConditionStateChangedTopic.On(s.bus).Publish(
		s.ctx, dnd5eEvents.ConditionStateChangedEvent{
			MemberID:     s.mon.GetID(),
			ConditionRef: refs.Conditions.OpportunityAttack(),
		})

	s.Require().NoError(err)
	s.Require().True(s.mon.IsDirty(), "a condition's own state is this sheet's state")
}

func (s *MonsterKeeperTestSuite) TestConditionStateChangedForSomeoneElseIsIgnored() {
	markSaved(s.mon)

	err := dnd5eEvents.ConditionStateChangedTopic.On(s.bus).Publish(
		s.ctx, dnd5eEvents.ConditionStateChangedEvent{
			MemberID:     "someone-else",
			ConditionRef: refs.Conditions.OpportunityAttack(),
		})

	s.Require().NoError(err)
	s.Require().False(s.mon.IsDirty(), "every sheet hears it; only one of them is it")
}

// spendReaction publishes the bill a reacting condition publishes, addressed
// to whichever member the caller names.
func (s *MonsterKeeperTestSuite) spendReaction(memberID string) {
	s.Require().NoError(dnd5eEvents.SpendRequestedTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.SpendRequestedEvent{
			MemberID:   memberID,
			ActionType: coreCombat.ActionReaction,
			Amount:     1,
			SourceRef:  refs.Conditions.OpportunityAttack(),
		}))
}

// A monster has exactly one reaction, and this keeper is what meters it.
//
// The bill used to pass a monster by: the keeper had no row for the topic, so
// a wolf could swing an opportunity attack every time anybody moved. Kirk
// ruled it on 2026-09-11 — "monsters should have reaction and it should
// replace that used once hack" — because Dissonant Whispers spends a monster's
// reaction from outside any condition the monster carries, and there was
// nothing for it to spend.
func (s *MonsterKeeperTestSuite) TestASpentReactionIsMeteredOnThisSheet() {
	markSaved(s.mon)
	s.Require().True(s.mon.CanReact(), "a monster starts its turn with its reaction")

	s.spendReaction(s.mon.GetID())

	s.False(s.mon.CanReact(), "the bill lands on the sheet that has to pay it")
	s.True(s.mon.IsDirty(), "a spent reaction that is not written down is not spent")
}

// Every sheet hears the bill; only one of them is the one being billed.
func (s *MonsterKeeperTestSuite) TestAReactionSomebodyElseSpentIsNotThisMonstersBill() {
	markSaved(s.mon)

	s.spendReaction("someone-else")

	s.True(s.mon.CanReact())
	s.False(s.mon.IsDirty())
}

// Only the reaction is metered here. A monster has no action and no bonus
// action to run out of, so a request for one is not this sheet's business and
// must not quietly empty the one meter it does keep.
func (s *MonsterKeeperTestSuite) TestASpentActionIsNotTheReactionMeter() {
	markSaved(s.mon)

	s.Require().NoError(dnd5eEvents.SpendRequestedTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.SpendRequestedEvent{
			MemberID:   s.mon.GetID(),
			ActionType: coreCombat.ActionStandard,
			Amount:     1,
		}))

	s.True(s.mon.CanReact())
	s.False(s.mon.IsDirty())
}

// TURN START, NOT TURN END. A reaction is spent on somebody else's turn, so a
// meter cleared at the end of its holder's turn would be full again for the
// whole window it governs. 2014 PHB: "you regain a spent reaction at the start
// of each of your turns."
func (s *MonsterKeeperTestSuite) TestTheReactionComesBackAtThisMonstersTurnStart() {
	s.spendReaction(s.mon.GetID())
	s.Require().False(s.mon.CanReact())
	markSaved(s.mon)

	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnStartEvent{SubjectID: s.mon.GetID(), Round: 2}))

	s.True(s.mon.CanReact())
	s.True(s.mon.IsDirty(), "a refreshed meter is a sheet worth saving")
}

// Somebody ELSE's turn beginning is exactly the window a reaction is spent in.
// Refreshing on it would make the meter meaningless.
func (s *MonsterKeeperTestSuite) TestAnotherSubjectsTurnStartLeavesTheMeterSpent() {
	s.spendReaction(s.mon.GetID())
	markSaved(s.mon)

	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnStartEvent{SubjectID: "someone-else", Round: 2}))

	s.False(s.mon.CanReact())
	s.False(s.mon.IsDirty(), "a turn start that changed nothing is not a write")
}

// A turn start with nothing to clear is not a write either. A boundary runs
// for every participant of every round, so marking unconditionally would flag
// every monster in the fight dirty on every turn of it.
func (s *MonsterKeeperTestSuite) TestATurnStartWithAFullMeterWritesNothing() {
	markSaved(s.mon)

	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnStartEvent{SubjectID: s.mon.GetID(), Round: 2}))

	s.True(s.mon.CanReact())
	s.False(s.mon.IsDirty())
}

// A long rest clears it too, which is where the opportunity attack's flag used
// to clear. A short rest does not: reactions are not a short-rest resource.
func (s *MonsterKeeperTestSuite) TestALongRestClearsTheMeterAndAShortRestDoesNot() {
	s.spendReaction(s.mon.GetID())
	markSaved(s.mon)

	s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetShortRest,
		CharacterID: s.mon.GetID(),
	}))
	s.Require().False(s.mon.CanReact(), "a short rest does not give a reaction back")

	s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "someone-else",
	}))
	s.Require().False(s.mon.CanReact(), "and neither does somebody else's long rest")

	s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: s.mon.GetID(),
	}))
	s.True(s.mon.CanReact())
	s.True(s.mon.IsDirty())
}

func TestPureLoadSuite(t *testing.T) {
	suite.Run(t, new(PureLoadTestSuite))
}

func TestMonsterKeeperSuite(t *testing.T) {
	suite.Run(t, new(MonsterKeeperTestSuite))
}

// namelessMonsterCondition breaks [dnd5eEvents.ConditionBehavior]'s contract
// the same way character's namelessCondition does: its ToJSON embeds a
// perfectly good ref, but Ref() returns nil.
type namelessMonsterCondition struct{}

func (c *namelessMonsterCondition) Ref() *core.Ref { return nil }

func (c *namelessMonsterCondition) IsApplied() bool { return true }

func (c *namelessMonsterCondition) Apply(_ context.Context, _ events.EventBus) error { return nil }

func (c *namelessMonsterCondition) Remove(_ context.Context, _ events.EventBus) error { return nil }

func (c *namelessMonsterCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(map[string]any{"ref": refs.Conditions.Prone()})
}

// TestARefLessConditionIsRefusedAtTheDoor covers the BUS path onto a monster's
// sheet — the twin of the character keeper's test of the same name.
//
// It also retires a panic. core.Ref.String has a pointer receiver that
// dereferences id.Module unguarded, so before rpg-project#319 Phase 6 a
// nameless condition reaching the removal path took the whole bus publish down
// rather than returning an error.
//
// The bus is NOT the only way onto this sheet, which review of that phase
// caught after a first attempt guarded only here: see
// TestALoadedRefLessConditionIsRefusedAtTheDoor for the load path, which
// monstertraits drives directly through Monster.AddLoadedCondition and which
// never touches a bus handler at all.
//
// namelessMonsterCondition's ToJSON returns a valid ref, which no longer
// matters to either keeper — both stopped reading it in Phase 6 — and is kept
// only so the fake is well-formed in every respect except the one under test.
func (s *MonsterKeeperTestSuite) TestARefLessConditionIsRefusedAtTheDoor() {
	err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    s.mon,
		Condition: &namelessMonsterCondition{},
	})

	s.Require().Error(err, "a condition that cannot name itself must not reach the sheet")
	s.Require().Empty(s.mon.GetConditions(), "and nothing was admitted")
	s.Require().False(s.mon.IsDirty(), "a refused application changes nothing to save")
}

// TestALoadedRefLessConditionIsRefusedAtTheDoor covers the LOAD path, which is
// the one a first attempt at this door missed.
//
// monstertraits calls Monster.AddLoadedCondition directly — from
// LoadMonsterConditions and from AttachMonster — so a persisted trait reaches
// the sheet without ever passing a bus handler. Guarding only the handler left
// this path open while the removal side had already given up its own nil
// check, which is a worse state than either half alone: a nameless trait would
// have loaded onto the sheet and then PANICKED the first removal that swept
// past it.
//
// So the door is the sheet's own Add methods, where every path converges,
// rather than the handler in front of one of them.
func (s *MonsterKeeperTestSuite) TestALoadedRefLessConditionIsRefusedAtTheDoor() {
	err := s.mon.AddLoadedCondition(&namelessMonsterCondition{})

	s.Require().Error(err, "a trait that cannot name itself must not reach the sheet")
	s.Require().Empty(s.mon.GetConditions(), "and nothing was admitted")
	s.Require().Contains(err.Error(), "namelessMonsterCondition",
		"the type is the only identification available for something that cannot name itself")
	s.Require().Contains(err.Error(), s.mon.GetID(), "and the error says whose sheet refused it")
}

// TestALiveRefLessConditionIsRefusedAtTheDoor is the same door, entered by the
// method the bus handler uses. Both Add methods ask, because both are exported
// and either can be called with anything.
func (s *MonsterKeeperTestSuite) TestALiveRefLessConditionIsRefusedAtTheDoor() {
	err := s.mon.AddCondition(&namelessMonsterCondition{})

	s.Require().Error(err)
	s.Require().Empty(s.mon.GetConditions())
	s.Require().False(s.mon.IsDirty(), "a refused application changes nothing to save")
}
