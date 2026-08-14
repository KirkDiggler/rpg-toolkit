// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// unattributed is the key subscriptions land under when they were made through
// the bus itself rather than through an effect's scoped view — the character's
// own machinery, for instance.
var unattributed = core.Ref{}

// scopeRecord is the registration list an attach site would keep: which effect
// subscribed to what. Shared by the root bus and every scoped view it hands out.
type scopeRecord struct {
	asked []core.Ref
	byRef map[core.Ref][]events.Topic
}

// recordingBus delegates every call to a real bus and notes, per subscription,
// which effect was mid-Apply when it was made. It is the shape of the
// instrumented surface resolution will own, reduced to what this test needs.
type recordingBus struct {
	inner  events.EventBus
	ref    core.Ref
	record *scopeRecord
}

func newRecordingBus() *recordingBus {
	return &recordingBus{
		inner:  events.NewEventBus(),
		ref:    unattributed,
		record: &scopeRecord{byRef: make(map[core.Ref][]events.Topic)},
	}
}

func (b *recordingBus) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	id, err := b.inner.Subscribe(ctx, topic, handler)
	if err != nil {
		return "", err
	}

	b.record.byRef[b.ref] = append(b.record.byRef[b.ref], topic)

	return id, nil
}

func (b *recordingBus) Unsubscribe(ctx context.Context, id string) error {
	return b.inner.Unsubscribe(ctx, id)
}

func (b *recordingBus) Publish(ctx context.Context, topic events.Topic, event any) error {
	return b.inner.Publish(ctx, topic, event)
}

// ScopeToEffect implements dnd5eEvents.EffectScoper.
func (b *recordingBus) ScopeToEffect(ref core.Ref) events.EventBus {
	b.record.asked = append(b.record.asked, ref)

	return &recordingBus{inner: b.inner, ref: ref, record: b.record}
}

type EffectScopingTestSuite struct {
	suite.Suite

	ctx       context.Context
	bus       *recordingBus
	ragingRef core.Ref
}

func (s *EffectScopingTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = newRecordingBus()
	s.ragingRef = *refs.Conditions.Raging()
}

// ragingBarbarian returns character data carrying one persisted Raging
// condition — the effect the saving-throw slice is built on.
func (s *EffectScopingTestSuite) ragingBarbarian() *Data {
	raging := &conditions.RagingCondition{
		CharacterID: "char-scope",
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}

	raw, err := raging.ToJSON()
	s.Require().NoError(err)

	return &Data{
		ID:       "char-scope",
		PlayerID: "player-scope",
		Name:     "Scoped Barbarian",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ProficiencyBonus: 2,
		Conditions:       []json.RawMessage{raw},
	}
}

// The load path asks the bus to scope to each effect it is about to apply, and
// names the effect by the ref its loader routed on. This is the moment
// attribution is knowable, and the only one.
func (s *EffectScopingTestSuite) TestLoadScopesEachEffectToItsRef() {
	_, err := LoadFromData(s.ctx, s.ragingBarbarian(), s.bus)
	s.Require().NoError(err)

	s.Require().Equal([]core.Ref{s.ragingRef}, s.bus.record.asked)
}

// The subscriptions Raging makes are attributed to Raging — including the
// saving-throw chain, which is the hook the first resolution slice folds.
func (s *EffectScopingTestSuite) TestRagingsSubscriptionsAreAttributedToRaging() {
	_, err := LoadFromData(s.ctx, s.ragingBarbarian(), s.bus)
	s.Require().NoError(err)

	s.Require().Contains(s.bus.record.byRef, s.ragingRef)
	s.Require().Contains(s.bus.record.byRef[s.ragingRef], events.Topic("dnd5e.saves.chain"))
}

// The character's own machinery is not laundered into an effect's name. Without
// this, "Raging attached N hooks" would quietly include hooks Raging never made.
func (s *EffectScopingTestSuite) TestCharacterMachineryIsNotAttributedToAnEffect() {
	_, err := LoadFromData(s.ctx, s.ragingBarbarian(), s.bus)
	s.Require().NoError(err)

	s.Require().NotEmpty(s.bus.record.byRef[unattributed],
		"the character's own subscriptions should still be recorded, just not under an effect")
	s.Require().NotContains(s.bus.record.byRef[unattributed], events.Topic("dnd5e.saves.chain"))
}

// A character with no persisted effects asks for no scopes at all: the absence
// of an effect is observable rather than indistinguishable from a silent one.
func (s *EffectScopingTestSuite) TestNoEffectsMeansNoScopesRequested() {
	data := s.ragingBarbarian()
	data.Conditions = nil

	_, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	s.Require().Empty(s.bus.record.asked)
	s.Require().NotContains(s.bus.record.byRef, s.ragingRef)
}

// A plain bus is unchanged by any of this — the seam is opt-in, and the ~20
// existing LoadFromData callers keep the behaviour they have.
func (s *EffectScopingTestSuite) TestPlainBusLoadsIdentically() {
	plain := events.NewEventBus()

	char, err := LoadFromData(s.ctx, s.ragingBarbarian(), plain)
	s.Require().NoError(err)

	conds := char.GetConditions()
	s.Require().Len(conds, 1)
	s.Require().True(conds[0].IsApplied())
}

func TestEffectScopingSuite(t *testing.T) {
	suite.Run(t, new(EffectScopingTestSuite))
}
