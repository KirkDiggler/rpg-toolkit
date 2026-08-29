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
	"github.com/KirkDiggler/rpg-toolkit/events"
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
	s.mon.MarkClean()

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
	s.mon.MarkClean()

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
	s.mon.MarkClean()

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
	s.mon.MarkClean()

	err := dnd5eEvents.ConditionStateChangedTopic.On(s.bus).Publish(
		s.ctx, dnd5eEvents.ConditionStateChangedEvent{
			MemberID:     s.mon.GetID(),
			ConditionRef: refs.Conditions.OpportunityAttack(),
		})

	s.Require().NoError(err)
	s.Require().True(s.mon.IsDirty(), "a condition's own state is this sheet's state")
}

func (s *MonsterKeeperTestSuite) TestConditionStateChangedForSomeoneElseIsIgnored() {
	s.mon.MarkClean()

	err := dnd5eEvents.ConditionStateChangedTopic.On(s.bus).Publish(
		s.ctx, dnd5eEvents.ConditionStateChangedEvent{
			MemberID:     "someone-else",
			ConditionRef: refs.Conditions.OpportunityAttack(),
		})

	s.Require().NoError(err)
	s.Require().False(s.mon.IsDirty(), "every sheet hears it; only one of them is it")
}

// A spend request passes the monster keeper by, and that absence is the D&D
// asymmetry rather than an oversight: monsters carry no action economy here, so
// there is no ledger to debit and nothing to refuse. The keeper says so by
// having no row for the topic — the sheet does not even go dirty.
func (s *MonsterKeeperTestSuite) TestSpendRequestedPassesTheMonsterBy() {
	s.mon.MarkClean()

	err := dnd5eEvents.SpendRequestedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.SpendRequestedEvent{
		MemberID:   s.mon.GetID(),
		ActionType: coreCombat.ActionReaction,
		Amount:     1,
		SourceRef:  refs.Conditions.OpportunityAttack(),
	})

	s.Require().NoError(err)
	s.Require().False(s.mon.IsDirty(), "nothing on this sheet could have paid")
}

// CanReact is true, and true is the answer rather than a stub. False would mean
// "my economy refuses"; a monster has no economy to do the refusing, which is
// the same thing the reacting rules say today in a nil check.
func (s *MonsterKeeperTestSuite) TestAMonsterNeverRefusesAReaction() {
	s.Require().True(s.mon.CanReact())
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
