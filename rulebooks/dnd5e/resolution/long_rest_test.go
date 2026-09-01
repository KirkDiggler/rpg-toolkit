// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

const (
	longRestFighterID   = "long-rest-fighter"
	longRestBarbarianID = "long-rest-barbarian"
)

var (
	fighterRestPool   = coreResources.ResourceKey("fighter-rest-pool")
	barbarianRestPool = coreResources.ResourceKey("barbarian-rest-pool")
)

// LongRestTestSuite proves the record-in/record-out entry delegates the whole
// recovery to an attached root character and keeps the transient bus sealed.
type LongRestTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *LongRestTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *LongRestTestSuite) TestNilInputAndDataAreRefused() {
	s.Run("nil input", func() {
		out, err := LongRest(s.ctx, nil)
		s.Require().ErrorIs(err, ErrNilInput)
		s.Require().Nil(out)
	})

	s.Run("nil character data", func() {
		out, err := LongRest(s.ctx, &LongRestInput{})
		s.Require().ErrorIs(err, ErrBadParticipant)
		s.Require().Nil(out)
	})

	s.Run("character data with no id", func() {
		out, err := LongRest(s.ctx, &LongRestInput{Character: &character.Data{}})
		s.Require().ErrorIs(err, ErrBadParticipant)
		s.Require().Nil(out)
	})
}

// A rest writes the sheet back, so its load must be strict. Dropping this blob
// would turn an unrelated rest into a persisted condition deletion.
func (s *LongRestTestSuite) TestMalformedPersistedEffectIsRefusedStrictly() {
	data := s.barbarian()
	data.Conditions = append(data.Conditions,
		json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"raging"},"x":`))

	out, err := LongRest(s.ctx, &LongRestInput{Character: data})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "condition")
	s.Require().Contains(err.Error(), "raging")
	s.Require().Nil(out)
}

// This is the complete Fighter recovery as independent values: no assertion
// asks a second root LongRest call what the first one should have done.
func (s *LongRestTestSuite) TestFighterRecoveryIsCompleteAndIndependent() {
	input := s.fighter()
	before, err := json.Marshal(input)
	s.Require().NoError(err)

	out, err := LongRest(s.ctx, &LongRestInput{Character: input})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Require().NotNil(out.Character)
	got := out.Character

	s.Require().Equal(36, got.HitPoints)
	s.Require().Equal(36, got.MaxHitPoints)
	s.Require().NotNil(got.DeathSaveState)
	s.Require().Zero(got.DeathSaveState.Successes)
	s.Require().Zero(got.DeathSaveState.Failures)
	s.Require().False(got.DeathSaveState.Stabilized)
	s.Require().False(got.DeathSaveState.Dead)

	s.Require().Len(got.Resources, 2)
	s.Require().Equal(3, got.Resources[resources.HitDice].Current,
		"one spent die plus half of four recovers to three")
	s.Require().Equal(4, got.Resources[resources.HitDice].Maximum)
	s.Require().Equal(2, got.Resources[fighterRestPool].Current,
		"a character-owned short-rest pool also refills on a long rest")
	s.Require().Len(got.SpellSlots, 2)
	s.Require().Zero(got.SpellSlots[1].Used)
	s.Require().Equal(3, got.SpellSlots[1].Max)
	s.Require().Zero(got.SpellSlots[2].Used)
	s.Require().Equal(2, got.SpellSlots[2].Max)

	s.Require().Len(got.Features, 2)
	var secondWind features.SecondWindData
	s.Require().NoError(json.Unmarshal(featureWithRef(s.T(), got.Features, refs.Features.SecondWind()), &secondWind))
	s.Require().Equal(1, secondWind.Uses)
	s.Require().Equal(1, secondWind.MaxUses)

	var actionSurge features.ActionSurgeData
	s.Require().NoError(json.Unmarshal(featureWithRef(s.T(), got.Features, refs.Features.ActionSurge()), &actionSurge))
	s.Require().Equal(1, actionSurge.Uses)
	s.Require().Equal(1, actionSurge.MaxUses)

	s.Require().Len(got.Conditions, 1)
	var opportunity conditions.OpportunityAttackConditionData
	s.Require().NoError(json.Unmarshal(
		conditionWithRef(s.T(), got.Conditions, refs.Conditions.OpportunityAttack()), &opportunity))
	s.Require().False(opportunity.UsedThisTurn,
		"the retained reaction meter resets on a long rest")
	s.Require().Nil(conditionWithRefOrNil(got.Conditions, refs.Conditions.Prone()),
		"the temporary prone condition ends on a long rest")

	after, err := json.Marshal(input)
	s.Require().NoError(err)
	s.Require().JSONEq(string(before), string(after), "the persisted input remains unchanged")
	s.Require().NotSame(input, got)

	// Mutate fields that the root sheet historically held by reference. If the
	// boundary merely echoed those references, these writes would reach input.
	got.AbilityScores[abilities.STR] = 3
	got.Languages[0] = languages.Elvish
	got.Skills[skills.Athletics] = shared.NotProficient
	got.Inventory[0].Quantity = 99
	got.EquipmentSlots[character.SlotMainHand] = "scribbled"
	slot := got.SpellSlots[1]
	slot.Used = 2
	got.SpellSlots[1] = slot

	s.Require().Equal(16, input.AbilityScores[abilities.STR])
	s.Require().Equal(languages.Common, input.Languages[0])
	s.Require().Equal(shared.Proficient, input.Skills[skills.Athletics])
	s.Require().Equal(1, input.Inventory[0].Quantity)
	s.Require().Equal("longsword", input.EquipmentSlots[character.SlotMainHand])
	s.Require().Equal(3, input.SpellSlots[1].Used)
}

// The Barbarian pins the other class-owned pool and both retained/removed
// condition outcomes, including odd-level hit-die rounding.
func (s *LongRestTestSuite) TestBarbarianRecoveryIsComplete() {
	out, err := LongRest(s.ctx, &LongRestInput{Character: s.barbarian()})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	got := out.Character

	s.Require().Equal(55, got.HitPoints)
	s.Require().Equal(55, got.MaxHitPoints)
	s.Require().NotNil(got.DeathSaveState)
	s.Require().Zero(got.DeathSaveState.Successes)
	s.Require().Zero(got.DeathSaveState.Failures)
	s.Require().False(got.DeathSaveState.Stabilized)
	s.Require().False(got.DeathSaveState.Dead)

	s.Require().Len(got.Resources, 3)
	s.Require().Equal(2, got.Resources[resources.HitDice].Current,
		"half of five rounds down to two")
	s.Require().Equal(5, got.Resources[resources.HitDice].Maximum)
	s.Require().Equal(3, got.Resources[resources.RageCharges].Current)
	s.Require().Equal(3, got.Resources[resources.RageCharges].Maximum)
	s.Require().Equal(2, got.Resources[barbarianRestPool].Current)
	s.Require().Len(got.SpellSlots, 1)
	s.Require().Zero(got.SpellSlots[1].Used)

	s.Require().Len(got.Conditions, 1)
	s.Require().NotNil(conditionWithRefOrNil(got.Conditions, refs.Conditions.UnarmoredDefense()),
		"the passive condition is retained")
	s.Require().Nil(conditionWithRefOrNil(got.Conditions, refs.Conditions.Raging()),
		"the temporary raging condition ends")
}

// R5 at the new entry: a retained condition is live during the rest and no
// longer registered after the call returns.
func (s *LongRestTestSuite) TestHeldSurfaceSuccessLeavesNoRegistrations() {
	bus := newLongRestFaultBus()
	surf := newSurface(bus)

	out, err := longRestOn(s.ctx, &LongRestInput{Character: s.retainedSheet()}, surf)

	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Require().NotEmpty(surf.registrations(), "the held surface observed real attachment")
	s.Require().Empty(bus.active, "every granted registration was revoked")
	s.assertNoUnarmoredDefense(bus)
}

// A root rule error and a teardown error stay jointly reachable. The fault bus
// delegates to a real bus and only injects the two deliberate failures.
func (s *LongRestTestSuite) TestHeldSurfaceRuleErrorJoinsTeardownAndLeavesNoRegistrations() {
	ruleErr := errors.New("rest publication failed")
	teardownErr := errors.New("teardown reported failure")
	bus := newLongRestFaultBus()
	bus.publishErr = ruleErr
	bus.unsubscribeErr = teardownErr
	surf := newSurface(bus)

	out, err := longRestOn(s.ctx, &LongRestInput{Character: s.retainedSheet()}, surf)

	s.Require().ErrorIs(err, ruleErr)
	s.Require().ErrorIs(err, teardownErr)
	s.Require().Nil(out)
	s.Require().NotEmpty(surf.registrations(), "the rule failed only after attachment")
	s.Require().Empty(bus.active,
		"the injected teardown error is reported after the real registration was removed")
	s.assertNoUnarmoredDefense(bus)
}

func (s *LongRestTestSuite) TestHeldSurfaceTeardownErrorRefusesAnOtherwiseSuccessfulResult() {
	teardownErr := errors.New("teardown reported failure")
	bus := newLongRestFaultBus()
	bus.unsubscribeErr = teardownErr
	surf := newSurface(bus)

	out, err := longRestOn(s.ctx, &LongRestInput{Character: s.retainedSheet()}, surf)

	s.Require().ErrorIs(err, teardownErr)
	s.Require().Contains(err.Error(), "resolution: teardown")
	s.Require().Nil(out, "a call that failed to tear down is not reported as successful")
	s.Require().Empty(bus.active,
		"the injected error is returned after each real registration was removed")
	s.assertNoUnarmoredDefense(bus)
}

// Refusing a Subscribe after the keeper has partially attached exercises the
// attach error path rather than the strict loader path. Everything granted
// before the refusal is still revoked.
func (s *LongRestTestSuite) TestHeldSurfaceAttachErrorLeavesNoRegistrations() {
	attachErr := errors.New("subscription refused")
	bus := newLongRestFaultBus()
	bus.failSubscribeAt = 3
	bus.subscribeErr = attachErr
	surf := newSurface(bus)

	out, err := longRestOn(s.ctx, &LongRestInput{Character: s.retainedSheet()}, surf)

	s.Require().ErrorIs(err, attachErr)
	s.Require().Nil(out)
	s.Require().NotEmpty(surf.registrations(), "two keeper registrations landed before refusal")
	s.Require().Empty(bus.active, "attach rollback and surface teardown leave nothing registered")
	s.assertNoUnarmoredDefense(bus)
}

func (s *LongRestTestSuite) fighter() *character.Data {
	secondWind, err := json.Marshal(features.SecondWindData{
		Ref: refs.Features.SecondWind(), ID: "fighter-second-wind", Name: "Second Wind",
		Level: 4, CharacterID: longRestFighterID, Uses: 0, MaxUses: 1,
	})
	s.Require().NoError(err)
	actionSurge, err := json.Marshal(features.ActionSurgeData{
		Ref: refs.Features.ActionSurge(), ID: "fighter-action-surge", Name: "Action Surge",
		CharacterID: longRestFighterID, Uses: 0, MaxUses: 1,
	})
	s.Require().NoError(err)
	opportunity, err := (&conditions.OpportunityAttackCondition{
		MemberID: longRestFighterID, UsedThisTurn: true,
	}).ToJSON()
	s.Require().NoError(err)
	prone, err := conditions.NewProneCondition(longRestFighterID).ToJSON()
	s.Require().NoError(err)

	return &character.Data{
		ID: longRestFighterID, PlayerID: "rest-player", Name: "Spent Fighter",
		Level: 4, ProficiencyBonus: 2, RaceID: races.Human, ClassID: classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 7, MaxHitPoints: 36, ArmorClass: 16,
		DeathSaveState: &saves.DeathSaveState{Successes: 1, Failures: 2, Stabilized: true},
		Skills:         map[skills.Skill]shared.ProficiencyLevel{skills.Athletics: shared.Proficient},
		Languages:      []languages.Language{languages.Common},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: "longsword", Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: "longsword"},
		SpellSlots: map[int]character.SpellSlotData{
			1: {Max: 3, Used: 3},
			2: {Max: 2, Used: 1},
		},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice: {Current: 1, Maximum: 4, ResetType: coreResources.ResetLongRest},
			fighterRestPool:   {Current: 0, Maximum: 2, ResetType: coreResources.ResetShortRest},
		},
		Features:   []json.RawMessage{secondWind, actionSurge},
		Conditions: []json.RawMessage{opportunity, prone},
	}
}

func (s *LongRestTestSuite) barbarian() *character.Data {
	unarmored, err := (&conditions.UnarmoredDefenseCondition{
		MemberID: longRestBarbarianID,
		Type:     conditions.UnarmoredDefenseBarbarian,
		Source:   "dnd5e:classes:barbarian",
	}).ToJSON()
	s.Require().NoError(err)
	raging, err := (&conditions.RagingCondition{
		CharacterID: longRestBarbarianID,
		DamageBonus: 2,
		Level:       5,
		Source:      "dnd5e:features:rage",
	}).ToJSON()
	s.Require().NoError(err)

	return &character.Data{
		ID: longRestBarbarianID, PlayerID: "rest-player", Name: "Spent Barbarian",
		Level: 5, ProficiencyBonus: 3, RaceID: races.Human, ClassID: classes.Barbarian,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 18, abilities.DEX: 14, abilities.CON: 16,
			abilities.INT: 8, abilities.WIS: 12, abilities.CHA: 10,
		},
		HitPoints: 0, MaxHitPoints: 55, ArmorClass: 15,
		DeathSaveState: &saves.DeathSaveState{Successes: 2, Failures: 1, Dead: true},
		SpellSlots:     map[int]character.SpellSlotData{1: {Max: 2, Used: 2}},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice:     {Current: 0, Maximum: 5, ResetType: coreResources.ResetLongRest},
			resources.RageCharges: {Current: 0, Maximum: 3, ResetType: coreResources.ResetLongRest},
			barbarianRestPool:     {Current: 0, Maximum: 2, ResetType: coreResources.ResetShortRest},
		},
		Conditions: []json.RawMessage{unarmored, raging},
	}
}

func (s *LongRestTestSuite) retainedSheet() *character.Data {
	data := s.barbarian()
	data.HitPoints = 11
	data.Conditions = data.Conditions[:1]
	return data
}

func (s *LongRestTestSuite) assertNoUnarmoredDefense(bus events.EventBus) {
	s.T().Helper()

	event := &combat.ACChainEvent{
		CharacterID: longRestBarbarianID,
		Breakdown:   &combat.ACBreakdown{Components: []combat.ACComponent{}},
	}
	chain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	modified, err := combat.ACChain.On(bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	s.Require().Empty(folded.Breakdown.Components,
		"the retained condition no longer answers after the entry returns")
}

func TestLongRestSuite(t *testing.T) {
	suite.Run(t, new(LongRestTestSuite))
}

func featureWithRef(t *testing.T, blobs []json.RawMessage, want *core.Ref) json.RawMessage {
	t.Helper()
	if got := effectWithRefOrNil(t, blobs, want); got != nil {
		return got
	}
	t.Fatalf("feature %s not found", want.String())
	return nil
}

func conditionWithRef(t *testing.T, blobs []json.RawMessage, want *core.Ref) json.RawMessage {
	t.Helper()
	if got := effectWithRefOrNil(t, blobs, want); got != nil {
		return got
	}
	t.Fatalf("condition %s not found", want.String())
	return nil
}

func conditionWithRefOrNil(blobs []json.RawMessage, want *core.Ref) json.RawMessage {
	for _, raw := range blobs {
		var envelope struct {
			Ref core.Ref `json:"ref"`
		}
		if json.Unmarshal(raw, &envelope) == nil && envelope.Ref.Equals(want) {
			return raw
		}
	}
	return nil
}

func effectWithRefOrNil(t *testing.T, blobs []json.RawMessage, want *core.Ref) json.RawMessage {
	t.Helper()
	for _, raw := range blobs {
		var envelope struct {
			Ref core.Ref `json:"ref"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatalf("read persisted effect ref: %v", err)
		}
		if envelope.Ref.Equals(want) {
			return raw
		}
	}
	return nil
}

// longRestFaultBus is a real bus with narrow, opt-in failure points and an
// active-registration view for lifecycle assertions.
type longRestFaultBus struct {
	inner events.EventBus

	active map[string]struct{}

	failSubscribeAt int
	subscribeCalls  int
	subscribeErr    error
	publishErr      error
	unsubscribeErr  error
}

func newLongRestFaultBus() *longRestFaultBus {
	return &longRestFaultBus{
		inner:  events.NewEventBus(),
		active: make(map[string]struct{}),
	}
}

func (b *longRestFaultBus) Subscribe(
	ctx context.Context, topic events.Topic, handler any,
) (string, error) {
	b.subscribeCalls++
	if b.failSubscribeAt != 0 && b.subscribeCalls == b.failSubscribeAt {
		return "", b.subscribeErr
	}

	id, err := b.inner.Subscribe(ctx, topic, handler)
	if err == nil {
		b.active[id] = struct{}{}
	}
	return id, err
}

func (b *longRestFaultBus) Unsubscribe(ctx context.Context, id string) error {
	if err := b.inner.Unsubscribe(ctx, id); err != nil {
		return err
	}
	delete(b.active, id)
	return b.unsubscribeErr
}

func (b *longRestFaultBus) Publish(
	ctx context.Context, topic events.Topic, event any,
) error {
	if topic == events.Topic("dnd5e.rest") && b.publishErr != nil {
		return b.publishErr
	}
	return b.inner.Publish(ctx, topic, event)
}
