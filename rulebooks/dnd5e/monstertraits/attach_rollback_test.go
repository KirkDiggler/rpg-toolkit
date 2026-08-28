// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// errRefused is what the stub bus answers with.
var errRefused = errors.New("bus refused the subscription")

// failingBus is a real bus that refuses the nth Subscribe and behaves normally
// otherwise, so a test can make a trait's Apply fail after the keeper's has
// succeeded.
type failingBus struct {
	inner  events.EventBus
	failOn int // 1-based; 0 never fails
	calls  int
}

func newFailingBus(failOn int) *failingBus {
	return &failingBus{inner: events.NewEventBus(), failOn: failOn}
}

func (b *failingBus) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	b.calls++
	if b.calls == b.failOn {
		return "", errRefused
	}

	return b.inner.Subscribe(ctx, topic, handler)
}

func (b *failingBus) Unsubscribe(ctx context.Context, id string) error {
	return b.inner.Unsubscribe(ctx, id)
}

func (b *failingBus) Publish(ctx context.Context, topic events.Topic, event any) error {
	return b.inner.Publish(ctx, topic, event)
}

// AttachMonsterRollbackTestSuite is the sharpest version of the contract: the
// trait blobs come off the monster before anything is known to work, so a
// failure that did not put them back would write the monster to storage
// without conditions nobody removed. Same three assertions as everywhere else —
// same data out, nothing listening, retry works.
type AttachMonsterRollbackTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *AttachMonsterRollbackTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AttachMonsterRollbackTestSuite) immunityBlob() json.RawMessage {
	raw, err := json.Marshal(ImmunityData{
		Ref:        refs.MonsterTraits.Immunity(),
		OwnerID:    "gob-rollback",
		DamageType: damage.Poison,
	})
	s.Require().NoError(err)

	return raw
}

// goblin carries two trait blobs so that a failure on the second can be told
// apart from a failure that lost the lot.
func (s *AttachMonsterRollbackTestSuite) goblin() *monster.Data {
	data := goblinData("gob-rollback")
	data.Conditions = []json.RawMessage{s.immunityBlob(), s.vulnerabilityBlob()}

	return data
}

func (s *AttachMonsterRollbackTestSuite) vulnerabilityBlob() json.RawMessage {
	raw, err := json.Marshal(VulnerabilityData{
		Ref:        refs.MonsterTraits.Vulnerability(),
		OwnerID:    "gob-rollback",
		DamageType: damage.Bludgeoning,
	})
	s.Require().NoError(err)

	return raw
}

func (s *AttachMonsterRollbackTestSuite) marshal(d *monster.Data) string {
	raw, err := json.Marshal(d)
	s.Require().NoError(err)

	return string(raw)
}

// assertNothingListening checks the keeper came back off with everything else.
func (s *AttachMonsterRollbackTestSuite) assertNothingListening(m *monster.Monster, bus events.EventBus) {
	before := m.HP()

	s.Require().NoError(dnd5eEvents.DamageReceivedTopic.On(bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: m.GetID(),
		Amount:   3,
	}))

	s.Require().Equal(before, m.HP(), "the monster is not listening")
	s.Require().False(m.IsDirty())
}

// A ref no loader here can route fails the attach — and every blob, including
// the ones never reached, is back on the monster. Without this, ToData writes
// the goblin back with no conditions at all and nothing anywhere says so.
func (s *AttachMonsterRollbackTestSuite) TestAnUnroutableTraitLosesNothing() {
	data := s.goblin()
	data.Conditions = append(data.Conditions,
		json.RawMessage(`{"ref":{"module":"dnd5e","type":"monster_traits","id":"nope"}}`))

	m, err := LoadMonster(s.ctx, data)
	s.Require().NoError(err)
	before := s.marshal(m.ToData())

	bus := events.NewEventBus()
	err = AttachMonster(s.ctx, m, bus, nil)

	s.Require().Error(err)
	s.Require().Contains(err.Error(), `"id":"nope"`, "the error names the blob")
	s.Require().Equal(before, s.marshal(m.ToData()), "the monster writes back exactly what it did before")
	s.Require().Empty(m.GetConditions(), "and nothing was half-added to it")
	s.assertNothingListening(m, bus)

	// The blobs are where they were, in order, ready for another attempt.
	s.Require().Len(m.TakeUnappliedConditions(), 3)
}

// The keeper attaches, then the first trait's Apply is refused — the third
// subscription. Everything unwinds, and the same monster attaches cleanly to a
// working bus afterwards.
func (s *AttachMonsterRollbackTestSuite) TestARefusedTraitRollsTheAttachBack() {
	data := s.goblin()

	m, err := LoadMonster(s.ctx, data)
	s.Require().NoError(err)
	before := s.marshal(m.ToData())

	bus := newFailingBus(3)
	err = AttachMonster(s.ctx, m, bus, nil)

	s.Require().Error(err)
	s.Require().ErrorIs(err, errRefused)
	s.Require().Equal(before, s.marshal(m.ToData()))
	s.Require().Empty(m.GetConditions())
	s.assertNothingListening(m, bus)

	good := events.NewEventBus()
	s.Require().NoError(AttachMonster(s.ctx, m, good, nil))
	s.Require().Len(m.GetConditions(), 3, "the retry attached both traits, plus the carried reaction")
	s.Require().Contains(s.marshal(m.ToData()), refs.Conditions.OpportunityAttack().ID,
		"an attached monster carries its reaction, which is how a spent meter survives")

	s.Require().NoError(dnd5eEvents.DamageReceivedTopic.On(good).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: m.GetID(),
		Amount:   3,
	}))
	s.Require().Equal(4, m.HP(), "and the keeper attached on the retry")
}

// A trait that fails after another has already applied takes that one off with
// it: half a monster's traits attached is a monster with rules nobody wrote.
func (s *AttachMonsterRollbackTestSuite) TestASecondTraitFailingRemovesTheFirst() {
	m, err := LoadMonster(s.ctx, s.goblin())
	s.Require().NoError(err)
	before := s.marshal(m.ToData())

	// 1-2 are the keeper's, 3 is the immunity trait's; the vulnerability trait
	// is refused after immunity has applied.
	bus := newFailingBus(4)
	err = AttachMonster(s.ctx, m, bus, nil)

	s.Require().Error(err)
	s.Require().Equal(before, s.marshal(m.ToData()))
	s.Require().Empty(m.GetConditions())
	s.assertNothingListening(m, bus)
	s.Require().Len(m.TakeUnappliedConditions(), 2, "both blobs are back")
}

func TestAttachMonsterRollbackSuite(t *testing.T) {
	suite.Run(t, new(AttachMonsterRollbackTestSuite))
}
