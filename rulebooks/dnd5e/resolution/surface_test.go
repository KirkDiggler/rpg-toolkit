// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// quietEffect is an effect that subscribes to nothing.
//
// No shipped condition does this today, which is precisely why it is worth
// pinning: "this effect attached zero hooks" has to be a readable fact rather
// than something indistinguishable from an effect that was never applied. When
// a real one arrives — a purely declarative immunity, say — this is the
// behaviour it will rely on.
type quietEffect struct{}

func (quietEffect) IsApplied() bool                                   { return true }
func (quietEffect) Apply(_ context.Context, _ events.EventBus) error  { return nil }
func (quietEffect) Remove(_ context.Context, _ events.EventBus) error { return nil }
func (quietEffect) ToJSON() (json.RawMessage, error)                  { return json.RawMessage(`{}`), nil }

type SurfaceTestSuite struct {
	suite.Suite

	ctx   context.Context
	inner events.EventBus
	surf  *surface
	topic events.Topic
}

func (s *SurfaceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.inner = events.NewEventBus()
	s.surf = newSurface(s.inner)
	s.topic = events.Topic("test.topic")
}

func (s *SurfaceTestSuite) TestSubscriptionsAreStampedWithParticipantAndEffect() {
	ref := core.Ref{Module: "dnd5e", Type: "conditions", ID: "raging"}

	view := s.surf.forParticipant("hero")
	_, err := view.Subscribe(s.ctx, s.topic, func(any) error { return nil })
	s.Require().NoError(err)

	scoped := view.ScopeToEffect(ref)
	_, err = scoped.Subscribe(s.ctx, s.topic, func(any) error { return nil })
	s.Require().NoError(err)

	got := s.surf.registrations()
	s.Require().Len(got, 2)

	s.Require().Equal("hero", got[0].Participant)
	s.Require().Equal(core.Ref{}, got[0].Effect, "the participant's own machinery names no effect")

	s.Require().Equal("hero", got[1].Participant)
	s.Require().Equal(ref, got[1].Effect)
}

// The claim ADR-0038 makes that nothing else can: an effect's silence is
// observable. Attaching a participant with a quiet effect leaves a ledger that
// says so, rather than one that simply omits it.
func (s *SurfaceTestSuite) TestAnEffectThatAttachesNothingIsAnAssertableFact() {
	loud := core.Ref{Module: "dnd5e", Type: "conditions", ID: "loud"}
	quiet := core.Ref{Module: "dnd5e", Type: "conditions", ID: "quiet"}

	view := s.surf.forParticipant("hero")

	_, err := view.ScopeToEffect(loud).Subscribe(s.ctx, s.topic, func(any) error { return nil })
	s.Require().NoError(err)

	s.Require().NoError(quietEffect{}.Apply(s.ctx, view.ScopeToEffect(quiet)))

	s.Require().Len(hooksFor(s.surf.registrations(), loud), 1)
	s.Require().Empty(hooksFor(s.surf.registrations(), quiet),
		"the quiet effect ran and attached nothing, and the ledger says exactly that")
}

// R5. Teardown revokes what resolution granted, and it does not ask the effects
// whether they would like to clean up after themselves.
func (s *SurfaceTestSuite) TestTeardownRevokesEverythingGranted() {
	fired := 0
	handler := func(any) error {
		fired++
		return nil
	}

	view := s.surf.forParticipant("hero")
	_, err := view.Subscribe(s.ctx, s.topic, handler)
	s.Require().NoError(err)
	_, err = view.ScopeToEffect(core.Ref{ID: "x"}).Subscribe(s.ctx, s.topic, handler)
	s.Require().NoError(err)

	s.Require().NoError(s.inner.Publish(s.ctx, s.topic, "before"))
	s.Require().Equal(2, fired, "both subscriptions are live before teardown")

	s.Require().NoError(s.surf.teardown(s.ctx))

	s.Require().NoError(s.inner.Publish(s.ctx, s.topic, "after"))
	s.Require().Equal(2, fired, "nothing survived the teardown")
}

// The ledger is the record of what was *granted*, so it keeps an entry an
// effect later revoked itself. Teardown stays safe because revoking twice is a
// no-op, and rewriting history to hide the first revocation would make the
// pre-execution picture a lie.
func (s *SurfaceTestSuite) TestTeardownIsIdempotent() {
	view := s.surf.forParticipant("hero")
	id, err := view.Subscribe(s.ctx, s.topic, func(any) error { return nil })
	s.Require().NoError(err)

	s.Require().NoError(view.Unsubscribe(s.ctx, id))
	s.Require().Len(s.surf.registrations(), 1)

	s.Require().NoError(s.surf.teardown(s.ctx))
	s.Require().NoError(s.surf.teardown(s.ctx))
}

// Every view writes into one ledger, because the registration list is a
// property of the interaction rather than of any one participant.
func (s *SurfaceTestSuite) TestViewsShareOneLedger() {
	_, err := s.surf.forParticipant("a").Subscribe(s.ctx, s.topic, func(any) error { return nil })
	s.Require().NoError(err)
	_, err = s.surf.forParticipant("b").Subscribe(s.ctx, s.topic, func(any) error { return nil })
	s.Require().NoError(err)

	got := s.surf.registrations()
	s.Require().Len(got, 2)
	s.Require().Equal("a", got[0].Participant)
	s.Require().Equal("b", got[1].Participant)
}

// registrations() hands out a copy: a caller reading the list cannot rewrite
// what teardown is about to revoke.
func (s *SurfaceTestSuite) TestRegistrationsAreACopy() {
	_, err := s.surf.forParticipant("a").Subscribe(s.ctx, s.topic, func(any) error { return nil })
	s.Require().NoError(err)

	got := s.surf.registrations()
	got[0].Participant = "tampered"

	s.Require().Equal("a", s.surf.registrations()[0].Participant)
}

func TestSurfaceSuite(t *testing.T) {
	suite.Run(t, new(SurfaceTestSuite))
}

// hooksFor is the question the registration list exists to answer: what did
// this effect attach?
func hooksFor(hooks []Registration, effect core.Ref) []Registration {
	var out []Registration
	for _, h := range hooks {
		if h.Effect == effect {
			out = append(out, h)
		}
	}

	return out
}
