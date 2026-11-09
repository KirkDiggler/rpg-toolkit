// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

type rageItemSuite struct {
	suite.Suite
	ctx    context.Context
	bus    events.EventBus
	ring   *items.Item
	active *rageActivation
	ended  []dndevents.ConditionRemovedEvent
}

func (s *rageItemSuite) SetupTest() {
	s.ctx, s.bus = context.Background(), events.NewEventBus()
	var err error
	s.ring, err = items.New(rageItemConfig("rage-ring-17"))
	s.Require().NoError(err)
	s.Require().NoError(s.ring.Attach(s.ctx, items.AttachInput{Bus: s.bus, HolderID: rageHolder}))
	out, err := useRage(s.ctx, s.ring, rageHolder)
	s.Require().NoError(err)
	s.active = out.Result
	_, err = dndevents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, e dndevents.ConditionRemovedEvent) error {
			s.ended = append(s.ended, e)
			return nil
		})
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		s.NoError(s.active.Effect.Remove(s.ctx, s.bus))
		s.NoError(s.ring.Detach(s.ctx))
	})
}

func (s *rageItemSuite) state() string {
	raw, err := s.active.Effect.ToJSON()
	s.Require().NoError(err)
	var data conditions.ComposedData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Require().NotNil(data.Runtime)
	return data.Runtime.State
}

func (s *rageItemSuite) apply() { s.Require().NoError(s.active.Effect.Apply(s.ctx, s.bus)) }
func (s *rageItemSuite) turn(holder string, round int) {
	s.Require().NoError(dndevents.TurnEndTopic.On(s.bus).Publish(s.ctx,
		dndevents.TurnEndEvent{SubjectID: holder, Round: round}))
}
func (s *rageItemSuite) attack(holder string) {
	_, err := dndevents.PostAttackRollChain.On(s.bus).PublishWithChain(s.ctx,
		&dndevents.PostAttackRollEvent{AttackerID: holder, WouldHit: false},
		events.NewStagedChain[*dndevents.PostAttackRollEvent](combat.ModifierStages))
	s.Require().NoError(err)
}
func (s *rageItemSuite) bonus(actor string, ability abilities.Ability, melee, weapon bool) int {
	s.T().Helper()
	out, err := rageDamage(s.ctx, s.bus, actor, ability, melee, weapon)
	s.Require().NoError(err)
	total := 0
	for _, component := range out.Components {
		total += component.Total()
	}
	return total
}
func (s *rageItemSuite) endedFor(reason string) {
	s.T().Helper()
	s.Require().NotEmpty(s.ended)
	e := s.ended[len(s.ended)-1]
	s.Equal(rageHolder, e.MemberID)
	s.Equal(rageConditionRef.String(), e.ConditionRef)
	s.Equal(reason, e.Reason)
	s.False(s.active.Effect.IsApplied())
	s.Equal(10, s.bonus(rageHolder, abilities.STR, true, true))
	s.Zero(s.ring.Data().Pools[chargePool], "ending an effect is not restoring the item")
}

func (s *rageItemSuite) TestPreparedResultUsesTheCanonicalSharedCreator() {
	s.Equal("dnd5e:conditions:raging", s.active.Effect.Ref().String())
	raw, err := s.active.Effect.ToJSON()
	s.Require().NoError(err)
	var data conditions.ComposedData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Require().NotNil(data.Origin)
	s.Equal("rage-ring-17", data.Origin.EntityID)
	s.Equal(rageHolder, data.MemberID)
	s.JSONEq(`{"damage_bonus":{"fixed":2}}`, string(data.Bindings))
	s.Require().NotNil(data.Runtime)
	s.Equal("fresh", data.Runtime.State)
}

func (s *rageItemSuite) TestRestoredActiveResultSurvivesItemTransfer() {
	s.apply()
	s.turn(rageHolder, 4)
	raw, err := s.active.Effect.ToJSON()
	s.Require().NoError(err)
	s.Require().NoError(s.active.Effect.Remove(s.ctx, s.bus))
	s.Require().NoError(s.ring.Detach(s.ctx))
	s.Require().NoError(s.ring.Attach(s.ctx, items.AttachInput{Bus: s.bus, HolderID: "new-holder"}))
	restored, err := conditions.LoadJSON(raw)
	s.Require().NoError(err)
	s.Require().NoError(restored.Apply(s.ctx, s.bus))
	s.T().Cleanup(func() { s.NoError(restored.Remove(s.ctx, s.bus)) })
	s.Equal(12, s.bonus(rageHolder, abilities.STR, true, true))
	s.Equal(10, s.bonus("new-holder", abilities.STR, true, true))
	s.Zero(s.ring.Data().Pools[chargePool])
	s.turn(rageHolder, 5)
	s.False(restored.IsApplied(), "a restored active effect gets no new grace")
	s.Equal(10, s.bonus(rageHolder, abilities.STR, true, true))
}

func (s *rageItemSuite) TestPreparedResultIsNotAppliedUntilTheCallerActs() {
	s.Zero(s.ring.Data().Pools[chargePool])
	s.False(s.active.Effect.IsApplied())
	s.Equal(10, s.bonus(rageHolder, abilities.STR, true, true))
	s.apply()
	s.Equal(12, s.bonus(rageHolder, abilities.STR, true, true))
	s.Equal("rage-ring-17", s.active.Source.ItemID)
}

func (s *rageItemSuite) TestDamageBonusUsesTheActualActionFactsAndItemProvenance() {
	s.apply()
	s.Equal(12, s.bonus(rageHolder, abilities.STR, true, true))
	s.Equal(10, s.bonus(rageHolder, abilities.DEX, true, true))
	s.Equal(10, s.bonus(rageHolder, abilities.STR, false, true))
	s.Equal(10, s.bonus(rageHolder, abilities.STR, true, false))
	s.Equal(10, s.bonus("other", abilities.STR, true, true))
	out, err := rageDamage(s.ctx, s.bus, rageHolder, abilities.STR, true, true)
	s.Require().NoError(err)
	s.Require().Len(out.Components, 2)
	s.Equal("rage-ring-17", out.Components[1].Roll.Source.SourceID)
}

func (s *rageItemSuite) TestPhysicalResistanceUsesTheRealIncomingFoldAndSettlement() {
	s.apply()
	for _, kind := range []damage.Type{damage.Bludgeoning, damage.Piercing, damage.Slashing} {
		settled, err := rageIncoming(s.ctx, s.bus, rageHolder, kind)
		s.Require().NoError(err)
		s.Require().Len(settled.Types, 1)
		s.Equal(4, settled.Types[0].Taken, "9 physical damage is halved and rounded down")
		s.Require().NotNil(settled.Types[0].DecidedBy)
		s.Equal("rage-ring-17", settled.Types[0].DecidedBy.Source.SourceID)
	}
	fire, err := rageIncoming(s.ctx, s.bus, rageHolder, damage.Fire)
	s.Require().NoError(err)
	s.Equal(9, fire.Types[0].Taken)
	other, err := rageIncoming(s.ctx, s.bus, "other", damage.Slashing)
	s.Require().NoError(err)
	s.Equal(9, other.Types[0].Taken)
}

func (s *rageItemSuite) TestStrengthSavesAndChecksGainAdvantageOnlyForTheHolder() {
	s.apply()
	for _, tc := range []struct {
		holder  string
		ability abilities.Ability
		want    bool
	}{
		{rageHolder, abilities.STR, true}, {rageHolder, abilities.DEX, false}, {"other", abilities.STR, false},
	} {
		out, err := rageSave(s.ctx, s.bus, tc.holder, tc.ability)
		s.Require().NoError(err)
		s.Equal(tc.want, out.HasAdvantage())
	}
	for _, tc := range []struct {
		holder string
		skill  skills.Skill
		want   bool
	}{
		{rageHolder, skills.Athletics, true}, {rageHolder, skills.Stealth, false}, {"other", skills.Athletics, false},
	} {
		out, err := rageCheck(s.ctx, s.bus, tc.holder, tc.skill)
		s.Require().NoError(err)
		s.Equal(tc.want, out.HasAdvantage())
	}
}

func (s *rageItemSuite) TestGraceMissedAttackAndDamageSustainThenInactivityEnds() {
	s.apply()
	s.turn("other", 1)
	s.Equal(rageFresh, s.state())
	s.turn(rageHolder, 1)
	s.Equal(rageLive, s.state())
	s.attack(rageHolder) // A miss is still an attack attempt.
	s.turn(rageHolder, 2)
	s.Equal(rageLive, s.state())
	s.Require().NoError(dndevents.DamageTakenTopic.On(s.bus).Publish(s.ctx,
		&dndevents.DamageTakenEvent{MemberID: rageHolder, Amount: 1}))
	s.turn(rageHolder, 3)
	s.Equal(rageLive, s.state())
	s.turn(rageHolder, 4)
	s.endedFor("no_combat_activity")
}

func (s *rageItemSuite) TestTenRoundsEndEvenWhenAttacksSustainIt() {
	s.apply()
	s.turn(rageHolder, 1)
	for round := 2; round < 10; round++ {
		s.attack(rageHolder)
		s.turn(rageHolder, round)
	}
	s.Equal(rageLive, s.state())
	s.attack(rageHolder)
	s.turn(rageHolder, 10)
	s.endedFor("duration_expired")
}

func (s *rageItemSuite) TestClockResetEndsTheEffect() {
	s.apply()
	s.turn(rageHolder, 5)
	s.attack(rageHolder)
	s.turn(rageHolder, 1)
	s.endedFor("clock_reset")
}

func (s *rageItemSuite) TestUnconsciousnessEndsOnlyTheAffectedHolder() {
	s.apply()
	s.Require().NoError(dndevents.DamageTakenTopic.On(s.bus).Publish(s.ctx,
		&dndevents.DamageTakenEvent{MemberID: rageHolder, DroppedToZero: false}))
	s.Require().NoError(dndevents.DamageTakenTopic.On(s.bus).Publish(s.ctx,
		&dndevents.DamageTakenEvent{MemberID: "other", DroppedToZero: true}))
	s.True(s.active.Effect.IsApplied())
	s.Require().NoError(dndevents.DamageTakenTopic.On(s.bus).Publish(s.ctx,
		&dndevents.DamageTakenEvent{MemberID: rageHolder, DroppedToZero: true}))
	s.endedFor("unconscious")
}

func (s *rageItemSuite) TestCombatEndAndChargeAreDifferentLifetimes() {
	s.apply()
	s.Require().NoError(dndevents.CombatEndTopic.On(s.bus).Publish(s.ctx, dndevents.CombatEndEvent{SubjectID: "other"}))
	s.True(s.active.Effect.IsApplied())
	s.Require().NoError(dndevents.CombatEndTopic.On(s.bus).Publish(s.ctx, dndevents.CombatEndEvent{SubjectID: rageHolder}))
	s.endedFor("combat_ended")
}

func (s *rageItemSuite) TestSourceDetachmentDoesNotImplicitlyRemoveOrMoveTheResult() {
	s.apply()
	s.Require().NoError(s.ring.Detach(s.ctx))
	s.Require().NoError(s.ring.Attach(s.ctx, items.AttachInput{Bus: s.bus, HolderID: "new-holder"}))
	s.Equal(12, s.bonus(rageHolder, abilities.STR, true, true))
	s.Equal(10, s.bonus("new-holder", abilities.STR, true, true))
	_, err := useRage(s.ctx, s.ring, "new-holder")
	s.ErrorIs(err, items.ErrUnavailable)
	// The caller can explicitly end the result; source detachment did not do it.
	s.Require().NoError(s.active.Effect.Remove(s.ctx, s.bus))
	s.Equal(10, s.bonus(rageHolder, abilities.STR, true, true))
	s.Zero(s.ring.Data().Pools[chargePool])
}

func (s *rageItemSuite) TestRestEndsTheEffectAndRecoveryDoesNotReactivateIt() {
	s.apply()
	s.Require().NoError(dndevents.RestTopic.On(s.bus).Publish(s.ctx,
		dndevents.RestEvent{CharacterID: rageHolder, RestType: resources.ResetShortRest}))
	s.endedFor("rest")
	s.Require().NoError(items.RecoveryTopic.On(s.bus).Publish(s.ctx,
		items.RecoveryEvent{HolderID: rageHolder, Trigger: shortRest}))
	s.Zero(s.ring.Data().Pools[chargePool], "short rest ended Rage but did not refill this fixture's long-rest pool")
	s.Require().NoError(items.RecoveryTopic.On(s.bus).Publish(s.ctx,
		items.RecoveryEvent{HolderID: rageHolder, Trigger: longRest}))
	s.Equal(1, s.ring.Data().Pools[chargePool])
	s.False(s.active.Effect.IsApplied())
	s.Equal(10, s.bonus(rageHolder, abilities.STR, true, true))
}

func (s *rageItemSuite) TestEndNotificationPrecedesOutputDetachment() {
	s.apply()
	bonusAtRemoval := 0
	_, err := dndevents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, _ dndevents.ConditionRemovedEvent) error {
			bonusAtRemoval = s.bonus(rageHolder, abilities.STR, true, true)
			return nil
		})
	s.Require().NoError(err)
	s.turn(rageHolder, 1)
	s.turn(rageHolder, 2)
	s.Equal(12, bonusAtRemoval)
	s.Equal(10, s.bonus(rageHolder, abilities.STR, true, true))
}

func (s *rageItemSuite) TestUnknownAttackFactsRefuseTheFoldRatherThanDisableTheBonus() {
	s.apply()
	frame := rageAttackFrame(rageHolder, "target")
	frame.Action.WeaponPool = contributions.Fact[bool]{}
	event := dndevents.NewDamageChainEvent(dndevents.DamageChainInput{
		AttackerID: rageHolder, TargetID: "target", Frame: frame,
		Components: baseDamage(10, damage.Slashing), WeaponDamageType: damage.Slashing,
	})
	_, err := dndevents.DamageChain.On(s.bus).PublishWithChain(s.ctx, event,
		events.NewStagedChain[*dndevents.DamageChainEvent](combat.ModifierStages))
	s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
}

func (s *rageItemSuite) TestOtherActorsActivityDoesNotSustainTheHolder() {
	s.apply()
	s.turn(rageHolder, 1)
	s.attack("other")
	s.Require().NoError(dndevents.DamageTakenTopic.On(s.bus).Publish(s.ctx,
		&dndevents.DamageTakenEvent{MemberID: "other", Amount: 1}))
	s.turn(rageHolder, 2)
	s.endedFor("no_combat_activity")
}

func TestRageGrantingItem(t *testing.T) { suite.Run(t, new(rageItemSuite)) }
