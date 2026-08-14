// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// unattributed is the key subscriptions land under when they were made through
// the bus itself rather than through a trait's scoped view.
var unattributed = core.Ref{}

type scopeRecord struct {
	asked []core.Ref
	byRef map[core.Ref][]events.Topic
}

// recordingBus delegates to a real bus and notes which trait was mid-Apply when
// each subscription was made.
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

type TraitScopingTestSuite struct {
	suite.Suite

	ctx context.Context
	bus *recordingBus
}

func (s *TraitScopingTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = newRecordingBus()
}

func (s *TraitScopingTestSuite) skeleton() *monster.Monster {
	m, err := monster.LoadFromData(s.ctx, &monster.Data{
		ID:            "skeleton-scope",
		Name:          "Skeleton",
		Ref:           refs.Monsters.Skeleton(),
		HitPoints:     13,
		MaxHitPoints:  13,
		ArmorClass:    13,
		AbilityScores: shared.AbilityScores{},
	}, s.bus)
	s.Require().NoError(err)

	return m
}

func (s *TraitScopingTestSuite) immunityBlob() json.RawMessage {
	raw, err := json.Marshal(ImmunityData{
		Ref:        refs.MonsterTraits.Immunity(),
		OwnerID:    "skeleton-scope",
		DamageType: damage.Piercing,
	})
	s.Require().NoError(err)

	return raw
}

// Monsters get the same attribution characters do. Their loader was already
// caller-driven; what it lacked was a way to say which trait it was applying.
func (s *TraitScopingTestSuite) TestTraitSubscriptionsAreAttributedToTheirRef() {
	m := s.skeleton()
	immunityRef := *refs.MonsterTraits.Immunity()

	err := LoadMonsterConditions(s.ctx, m, []json.RawMessage{s.immunityBlob()}, s.bus, nil)
	s.Require().NoError(err)

	s.Require().Equal([]core.Ref{immunityRef}, s.bus.record.asked)
	s.Require().NotEmpty(s.bus.record.byRef[immunityRef],
		"the immunity trait subscribed to something, and it should be recorded under immunity")
}

// The monster's own machinery, subscribed while it loaded, stays unattributed
// rather than being folded into whichever trait is applied next.
func (s *TraitScopingTestSuite) TestMonsterMachineryIsNotAttributedToATrait() {
	m := s.skeleton()

	s.Require().NotEmpty(s.bus.record.byRef[unattributed],
		"loading the monster subscribes its own hooks")
	s.Require().Empty(s.bus.record.asked,
		"loading a monster applies no traits, so nothing should have been scoped yet")

	err := LoadMonsterConditions(s.ctx, m, []json.RawMessage{s.immunityBlob()}, s.bus, nil)
	s.Require().NoError(err)

	s.Require().Len(s.bus.record.asked, 1)
}

// A plain bus is unaffected — the seam is opt-in.
func (s *TraitScopingTestSuite) TestPlainBusLoadsIdentically() {
	plain := events.NewEventBus()

	m, err := monster.LoadFromData(s.ctx, &monster.Data{
		ID:            "skeleton-plain",
		Name:          "Skeleton",
		Ref:           refs.Monsters.Skeleton(),
		HitPoints:     13,
		MaxHitPoints:  13,
		ArmorClass:    13,
		AbilityScores: shared.AbilityScores{},
	}, plain)
	s.Require().NoError(err)

	raw, err := json.Marshal(ImmunityData{
		Ref:        refs.MonsterTraits.Immunity(),
		OwnerID:    "skeleton-plain",
		DamageType: damage.Piercing,
	})
	s.Require().NoError(err)

	s.Require().NoError(LoadMonsterConditions(s.ctx, m, []json.RawMessage{raw}, plain, nil))
	s.Require().Len(m.GetConditions(), 1)
}

func TestTraitScopingSuite(t *testing.T) {
	suite.Run(t, new(TraitScopingTestSuite))
}
