// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// SneakAttackTestSuite tests the SneakAttackCondition behavior
type SneakAttackTestSuite struct {
	suite.Suite
	ctrl   *gomock.Controller
	ctx    context.Context
	bus    events.EventBus
	roller *mock_dice.MockRoller
}

func (s *SneakAttackTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.roller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *SneakAttackTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestSneakAttackTestSuite(t *testing.T) {
	suite.Run(t, new(SneakAttackTestSuite))
}

// damageChainInput holds parameters for executeDamageChain
type damageChainInput struct {
	attackerID       string
	targetID         string
	abilityUsed      abilities.Ability
	hasAdvantage     bool
	isCritical       bool
	componentType    damage.Type
	weaponDamageType damage.Type
	weaponRef        *core.Ref
}

// executeDamageChain creates a damage chain event and executes it.
func (s *SneakAttackTestSuite) executeDamageChain(input damageChainInput) (*dnd5eEvents.DamageChainEvent, error) {
	componentType := input.componentType
	if componentType == damage.None {
		componentType = damage.Piercing
	}
	weaponDamageType := input.weaponDamageType
	if weaponDamageType == damage.None {
		weaponDamageType = damage.Piercing
	}

	weaponComp := dnd5eEvents.DamageComponent{
		Source:     dnd5eEvents.DamageSourceWeapon,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		Roll: dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Shortsword(), Name: "Shortsword"},
			Dice:   testDiceTrace(6, 5),
		},
		DamageType: componentType,
		IsCritical: false,
	}

	weaponRef := input.weaponRef
	if weaponRef == nil {
		weaponRef = refs.Weapons.Shortsword()
	}

	targetID := input.targetID
	if targetID == "" {
		targetID = "goblin-1"
	}

	damageEvent := swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID:       input.attackerID,
		TargetID:         targetID,
		Components:       []dnd5eEvents.DamageComponent{weaponComp},
		WeaponDamageType: weaponDamageType,
		IsCritical:       input.isCritical,
	}, swing{HasAdvantage: input.hasAdvantage, AbilityUsed: input.abilityUsed, WeaponRef: weaponRef})

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(s.bus)

	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, withEventFrame(damageEvent), chain)
	if err != nil {
		return nil, err
	}

	return modifiedChain.Execute(s.ctx, damageEvent)
}

func (s *SneakAttackTestSuite) TestSneakAttackUsesMarkedWeaponType() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
		Roller:   s.roller,
	})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))
	s.roller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{4}, nil)

	finalEvent, err := s.executeDamageChain(damageChainInput{
		attackerID:       "rogue-1",
		abilityUsed:      abilities.DEX,
		hasAdvantage:     true,
		componentType:    damage.Slashing,
		weaponDamageType: damage.Fire,
	})
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2)
	s.Equal(damage.Fire, finalEvent.Components[1].DamageType)
}

// executeDamageChainSimple is a convenience wrapper for simple test cases
//
//nolint:unparam // abilityUsed is intentionally fixed to DEX for most tests
func (s *SneakAttackTestSuite) executeDamageChainSimple(
	attackerID string,
	abilityUsed abilities.Ability,
) (*dnd5eEvents.DamageChainEvent, error) {
	return s.executeDamageChain(damageChainInput{
		attackerID:   attackerID,
		abilityUsed:  abilityUsed,
		hasAdvantage: true, // Default to true so existing tests pass
	})
}

func (s *SneakAttackTestSuite) TestSneakAttackAddsDiceLevel1() {
	// Level 1 rogue gets 1d6 sneak attack
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
		Roller:   s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Expect 1d6 to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	// Execute damage chain with DEX (finesse weapon) and advantage
	finalEvent, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	// Should have weapon + sneak attack components
	s.Require().Len(finalEvent.Components, 2, "Should have weapon and sneak attack components")

	// Verify sneak attack component uses DamageSourceFeature
	sneakComp := finalEvent.Components[1]
	s.Equal(dnd5eEvents.DamageSourceFeature, sneakComp.Source)
	s.Equal([]int{4}, sneakComp.Roll.Dice.FinalRolls, "Should have rolled 1d6")
	s.Equal(4, sneakComp.Total(), "Sneak attack should add 4 damage")
}

func (s *SneakAttackTestSuite) TestCriticalRollsSneakDiceTwice() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
		Roller:   s.roller,
	})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))

	s.roller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{4}, nil)
	s.roller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{5}, nil)

	finalEvent, err := s.executeDamageChain(damageChainInput{
		attackerID:       "rogue-1",
		abilityUsed:      abilities.DEX,
		hasAdvantage:     true,
		isCritical:       true,
		componentType:    damage.Piercing,
		weaponDamageType: damage.Piercing,
	})
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2)

	sneakComp := finalEvent.Components[1]
	s.Equal([]int{4, 5}, sneakComp.Roll.Dice.FinalRolls)
	s.Equal(damage.Piercing, sneakComp.DamageType)
	s.True(sneakComp.IsCritical)
}

func (s *SneakAttackTestSuite) TestSneakAttackAddsDiceLevel5() {
	// Level 5 rogue gets 3d6 sneak attack
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    5,
		Roller:   s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Expect 3d6 to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 3, 6).
		Return([]int{3, 5, 6}, nil)

	finalEvent, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.Components, 2)

	sneakComp := finalEvent.Components[1]
	s.Equal(dnd5eEvents.DamageSourceFeature, sneakComp.Source)
	s.Equal([]int{3, 5, 6}, sneakComp.Roll.Dice.FinalRolls, "Should have rolled 3d6")
	s.Equal(14, sneakComp.Total(), "Sneak attack should add 14 damage (3+5+6)")
}

func (s *SneakAttackTestSuite) TestSneakAttackOnlyOncePerTurn() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
		Roller:   s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// First attack - expect sneak attack
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	finalEvent, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2, "First attack should have sneak attack")

	// Second attack - NO sneak attack (already used this turn)
	// No roller expectation - RollN should NOT be called

	finalEvent2, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent2.Components, 1, "Second attack should NOT have sneak attack")
}

func (s *SneakAttackTestSuite) TestSneakAttackResetsOnTurnEnd() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
		Roller:   s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// First attack
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	_, err = s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)

	// End turn
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		SubjectID: "rogue-1",
		Round:     1,
	})
	s.Require().NoError(err)

	// Next turn - sneak attack should work again
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{6}, nil)

	finalEvent, err := s.executeDamageChainSimple("rogue-1", abilities.DEX)
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2, "Should have sneak attack after turn reset")
}

// TestSneakAttackUsedThisTurnPersistsAcrossJSONRoundTrip is the regression
// guard for the bug fixed in this PR. Before the fix, ToJSON / loadJSON dropped
// UsedThisTurn, so an Encounter.TakeAction RPC that did Character.LoadFromData
// would always start with UsedThisTurn=false — letting a rogue sneak-attack on
// every TakeAction within a turn instead of once per turn.
func (s *SneakAttackTestSuite) TestSneakAttackUsedThisTurnPersistsAcrossJSONRoundTrip() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    3,
	})
	sneak.UsedThisTurn = true

	raw, err := sneak.ToJSON()
	s.Require().NoError(err)

	loaded := &SneakAttackCondition{}
	err = loaded.loadJSON(raw)
	s.Require().NoError(err)

	s.True(loaded.UsedThisTurn,
		"UsedThisTurn must survive ToJSON → loadJSON; otherwise the once-per-turn gate is silently broken across LoadFromData cycles")
	s.Equal("rogue-1", loaded.CharacterID, "CharacterID still round-trips")
	s.Equal(3, loaded.Level, "Level still round-trips")
	s.Equal(2, loaded.DamageDice, "DamageDice still round-trips (level 3 → 2d6)")
}

// TestSneakAttackUsedThisTurnFalseRoundTrips guards against the inverse bug —
// if loadJSON forgot to read the field, a fresh condition would always have
// UsedThisTurn=false regardless of the persisted value.
func (s *SneakAttackTestSuite) TestSneakAttackUsedThisTurnFalseRoundTrips() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
	})
	// UsedThisTurn defaults to false; explicit for the test.
	sneak.UsedThisTurn = false

	raw, err := sneak.ToJSON()
	s.Require().NoError(err)

	loaded := &SneakAttackCondition{}
	err = loaded.loadJSON(raw)
	s.Require().NoError(err)

	s.False(loaded.UsedThisTurn, "UsedThisTurn=false must round-trip as false")
}

func TestSneakAttackMeterResetsOnLongRest(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	raw := json.RawMessage(`{
		"ref":{"module":"dnd5e","type":"features","id":"sneak_attack"},
		"member_id":"rogue-1","level":3,"damage_dice":2,"used_this_turn":true
	}`)

	loaded, err := LoadJSON(raw)
	require.NoError(t, err)
	sneak, ok := loaded.(*SneakAttackCondition)
	require.True(t, ok)
	require.True(t, sneak.UsedThisTurn)

	var changed []dnd5eEvents.ConditionStateChangedEvent
	_, err = dnd5eEvents.ConditionStateChangedTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.ConditionStateChangedEvent) error {
			changed = append(changed, event)
			return nil
		})
	require.NoError(t, err)
	require.NoError(t, sneak.Apply(ctx, bus))

	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "other-rogue",
	}))
	require.True(t, sneak.UsedThisTurn, "another character's rest must not reset this meter")
	require.Empty(t, changed)

	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetShortRest,
		CharacterID: "rogue-1",
	}))
	require.True(t, sneak.UsedThisTurn, "a short rest must not reset this meter")
	require.Empty(t, changed)

	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "rogue-1",
	}))
	require.False(t, sneak.UsedThisTurn)
	require.Len(t, changed, 1)
	require.Equal(t, "rogue-1", changed[0].MemberID)
	require.Equal(t, refs.Features.SneakAttack().String(), changed[0].ConditionRef.String())

	serialized, err := sneak.ToJSON()
	require.NoError(t, err)
	var data SneakAttackData
	require.NoError(t, json.Unmarshal(serialized, &data))
	require.False(t, data.UsedThisTurn)
	require.True(t, sneak.IsApplied(), "long rest resets but retains sneak attack")

	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "rogue-1",
	}))
	require.Len(t, changed, 1, "an already-clear meter must not publish a state change")

	sneak.UsedThisTurn = true
	require.NoError(t, sneak.Remove(ctx, bus))
	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "rogue-1",
	}))
	require.True(t, sneak.UsedThisTurn, "a removed condition must no longer hear rests")
	require.Len(t, changed, 1)
}

func TestSneakAttackMeterLongRestSubscribeFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	bus := &failAfterBus{EventBus: events.NewEventBus(), allow: 2}
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue-1", Level: 3})

	err := sneak.Apply(ctx, bus)
	require.ErrorIs(t, err, errRefusedSubscribe)
	require.False(t, sneak.IsApplied())
	require.Len(t, bus.unsubscribed, 2, "both earlier subscriptions must be rolled back")

	retryBus := events.NewEventBus()
	require.NoError(t, sneak.Apply(ctx, retryBus), "a rolled-back condition must be reusable")
	require.NoError(t, sneak.Remove(ctx, retryBus))
}

func (s *SneakAttackTestSuite) TestSneakAttackRequiresFinesseWeapon() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
		Roller:   s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation: a weapon that is neither finesse nor ranged
	// never sneak attacks, whichever ability swings it (rpg-toolkit#1929).
	for _, ability := range []abilities.Ability{abilities.STR, abilities.DEX} {
		finalEvent, err := s.executeDamageChain(damageChainInput{
			attackerID:   "rogue-1",
			abilityUsed:  ability,
			hasAdvantage: true,
			weaponRef:    refs.Weapons.Club(),
		})
		s.Require().NoError(err)
		s.Require().Len(finalEvent.Components, 1, "a club attack should NOT have sneak attack")
	}
}

// TestSneakAttackWithStrengthFinesse: a finesse weapon swung with Strength
// still sneak attacks (rpg-toolkit#1929).
func (s *SneakAttackTestSuite) TestSneakAttackWithStrengthFinesse() {
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue-1", Level: 1, Roller: s.roller})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))
	s.roller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{4}, nil)

	finalEvent, err := s.executeDamageChain(damageChainInput{
		attackerID:   "rogue-1",
		abilityUsed:  abilities.STR,
		hasAdvantage: true,
		weaponRef:    refs.Weapons.Rapier(),
	})
	s.Require().NoError(err)
	s.Require().Len(finalEvent.Components, 2, "a Strength rapier attack sneak attacks")
}

func (s *SneakAttackTestSuite) TestSneakAttackOnlyAffectsOwnAttacks() {
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
		Roller:   s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// No roller expectation - different attacker should not trigger

	// Different character attacks
	finalEvent, err := s.executeDamageChainSimple("rogue-2", abilities.DEX)
	s.Require().NoError(err)

	// Should only have weapon component
	s.Require().Len(finalEvent.Components, 1, "Other character's attack should NOT have sneak attack")
}

func (s *SneakAttackTestSuite) TestCalculateSneakAttackDice() {
	testCases := []struct {
		level      int
		damageDice int
	}{
		{level: 1, damageDice: 1},
		{level: 2, damageDice: 1},
		{level: 3, damageDice: 2},
		{level: 4, damageDice: 2},
		{level: 5, damageDice: 3},
		{level: 6, damageDice: 3},
		{level: 7, damageDice: 4},
		{level: 9, damageDice: 5},
		{level: 11, damageDice: 6},
		{level: 13, damageDice: 7},
		{level: 15, damageDice: 8},
		{level: 17, damageDice: 9},
		{level: 19, damageDice: 10},
		{level: 20, damageDice: 10},
	}

	for _, tc := range testCases {
		sneak := NewSneakAttackCondition(SneakAttackInput{
			MemberID: "rogue-1",
			Level:    tc.level,
			Roller:   s.roller,
		})
		s.Equal(tc.damageDice, sneak.DamageDice, "Level %d should have %dd6", tc.level, tc.damageDice)
	}
}

// =============================================================================
// Sneak Attack Condition Tests (Advantage OR Ally Adjacent)
// =============================================================================

func (s *SneakAttackTestSuite) TestSneakAttackTriggersWithAdvantage() {
	// Sneak attack should trigger when attacker has advantage
	sneak := NewSneakAttackCondition(SneakAttackInput{
		MemberID: "rogue-1",
		Level:    1,
		Roller:   s.roller,
	})

	err := sneak.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Expect sneak attack dice to be rolled
	s.roller.EXPECT().
		RollN(gomock.Any(), 1, 6).
		Return([]int{4}, nil)

	// Attack with advantage (no ally nearby)
	finalEvent, err := s.executeDamageChain(damageChainInput{
		attackerID:   "rogue-1",
		abilityUsed:  abilities.DEX,
		hasAdvantage: true,
	})
	s.Require().NoError(err)

	// Should have sneak attack
	s.Require().Len(finalEvent.Components, 2, "Should have sneak attack with advantage")
}

// The positional half of the rule is answered from the frame's target→X
// pairs, which resolution measures on the compiled hex grid and stances from
// the disposition graph. No room or cast is consulted here.

func (s *SneakAttackTestSuite) TestSneakAttackTriggersWithAllyAdjacent() {
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue-1", Level: 1, Roller: s.roller})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))
	s.roller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{5}, nil)

	// The fighter qualifies not by being a "character" but by being an enemy
	// OF THE TARGET, adjacent to it.
	finalEvent := s.runDamageChain("rogue-1", "goblin-1",
		knownPair("goblin-1", "fighter-1", 1.0, contributions.StanceHostile))
	s.Require().Len(finalEvent.Components, 2, "Should have sneak attack with ally adjacent")
}

// TestSneakAttackDoesNotTriggerWhenAdjacentCreatureIsTheTargetsAlly pins the
// half of the rule the old entity-type proxy could not express at all.
//
// RAW is "another ENEMY OF THE TARGET is within 5 feet of it". A second goblin
// standing beside the first is the target's ally, and grants the rogue nothing.
func (s *SneakAttackTestSuite) TestSneakAttackDoesNotTriggerWhenAdjacentCreatureIsTheTargetsAlly() {
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue-1", Level: 1, Roller: s.roller})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))

	finalEvent := s.runDamageChain("rogue-1", "goblin-1",
		knownPair("goblin-1", "goblin-2", 1.0, contributions.StanceAllied))
	s.Require().Len(finalEvent.Components, 1, "the target's own ally must not enable sneak attack")
}

// TestSneakAttackTriggersWhenAThirdFactionIsAdjacentToTheTarget: the rogue
// stabs a duergar while a hobgoblin — hostile to the party AND to the duergar —
// stands next to it. That hobgoblin is an enemy of the target, so RAW the rogue
// sneak attacks, though it is nobody's ally.
func (s *SneakAttackTestSuite) TestSneakAttackTriggersWhenAThirdFactionIsAdjacentToTheTarget() {
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue-1", Level: 1, Roller: s.roller})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))
	s.roller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{5}, nil)

	finalEvent := s.runDamageChain("rogue-1", "duergar-1",
		knownPair("duergar-1", "rogue-1", 4.0, contributions.StanceHostile),
		knownPair("duergar-1", "hobgoblin-1", 1.0, contributions.StanceHostile))
	s.Require().Len(finalEvent.Components, 2,
		"an enemy of the target enables sneak attack even when it is nobody's ally")
}

// runDamageChain folds one no-advantage DEX weapon hit under a complete frame
// carrying the given pairs and returns the result.
func (s *SneakAttackTestSuite) runDamageChain(
	attackerID, targetID string, pairs ...contributions.PairFacts,
) *dnd5eEvents.DamageChainEvent {
	s.T().Helper()

	damageEvent := withEventFrame(swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID: attackerID,
		TargetID:   targetID,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Shortsword(), Name: "Shortsword"},
				Dice:   testDiceTrace(6, 5),
			},
			DamageType: damage.Piercing,
		}},
	}, swing{IsMelee: true, HasAdvantage: false, AbilityUsed: abilities.DEX, WeaponRef: refs.Weapons.Shortsword()}), pairs...)

	c := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modifiedChain, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(s.ctx, framedDamage(damageEvent), c)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)
	return finalEvent
}

func (s *SneakAttackTestSuite) TestSneakAttackDoesNotTriggerWithoutConditions() {
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue-1", Level: 1, Roller: s.roller})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))

	// No roller expectation - sneak attack should NOT be rolled.
	finalEvent := s.runDamageChain("rogue-1", "goblin-1",
		knownPair("goblin-1", "rogue-1", 4.0, contributions.StanceHostile))
	s.Require().Len(finalEvent.Components, 1, "Should NOT have sneak attack without conditions")
	s.False(sneak.UsedThisTurn)
}

func (s *SneakAttackTestSuite) TestSneakAttackDoesNotTriggerWhenAllyTooFar() {
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue-1", Level: 1, Roller: s.roller})
	s.Require().NoError(sneak.Apply(s.ctx, s.bus))

	finalEvent := s.runDamageChain("rogue-1", "goblin-1",
		knownPair("goblin-1", "fighter-1", 5.0, contributions.StanceHostile))
	s.Require().Len(finalEvent.Components, 1, "Should NOT have sneak attack when ally too far")
}

// Suppress unused import warning
var _ dice.Roller = (*mock_dice.MockRoller)(nil)
