// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// plainBus is an events.EventBus that knows nothing about effect scoping —
// the shape every caller outside an attach site has.
type plainBus struct {
	name string
}

func (b *plainBus) Subscribe(_ context.Context, _ events.Topic, _ any) (string, error) {
	return b.name, nil
}

func (b *plainBus) Unsubscribe(_ context.Context, _ string) error { return nil }

func (b *plainBus) Publish(_ context.Context, _ events.Topic, _ any) error { return nil }

// scopingBus records what it was asked to scope to and returns whatever the
// test told it to, including nil.
type scopingBus struct {
	plainBus
	returns events.EventBus
	asked   []core.Ref
}

func (b *scopingBus) ScopeToEffect(ref core.Ref) events.EventBus {
	b.asked = append(b.asked, ref)
	return b.returns
}

type ScopeTestSuite struct {
	suite.Suite

	ref core.Ref
}

func (s *ScopeTestSuite) SetupTest() {
	s.ref = core.Ref{Module: "dnd5e", Type: "conditions", ID: "raging"}
}

// A bus that does not implement EffectScoper is handed back untouched, which is
// what keeps this seam free for every existing caller.
func (s *ScopeTestSuite) TestPlainBusIsReturnedUnchanged() {
	bus := &plainBus{name: "plain"}

	got := dnd5eEvents.BusForEffect(bus, s.ref)

	s.Require().Same(bus, got)
}

// The whole point: a scoper is consulted, with the effect's own ref, and its
// answer is what the effect gets applied with.
func (s *ScopeTestSuite) TestScoperIsConsultedAndItsBusUsed() {
	scoped := &plainBus{name: "scoped"}
	bus := &scopingBus{returns: scoped}

	got := dnd5eEvents.BusForEffect(bus, s.ref)

	s.Require().Same(scoped, got)
	s.Require().Equal([]core.Ref{s.ref}, bus.asked)
}

// Bookkeeping must never be able to break an effect: a scoper that answers nil
// leaves the effect on the bus it would have had anyway.
func (s *ScopeTestSuite) TestNilScopeFallsBackToTheBus() {
	bus := &scopingBus{returns: nil}

	got := dnd5eEvents.BusForEffect(bus, s.ref)

	s.Require().Same(bus, got)
	s.Require().Len(bus.asked, 1)
}

// A blob with no ref still applies; it is only unattributed. Asserting the
// scoper is asked (rather than skipped) keeps the zero ref a decision the
// attach site makes, not one this helper makes for it.
func (s *ScopeTestSuite) TestZeroRefStillConsultsTheScoper() {
	scoped := &plainBus{name: "scoped"}
	bus := &scopingBus{returns: scoped}

	got := dnd5eEvents.BusForEffect(bus, core.Ref{})

	s.Require().Same(scoped, got)
	s.Require().Equal([]core.Ref{{}}, bus.asked)
}

func TestScopeSuite(t *testing.T) {
	suite.Run(t, new(ScopeTestSuite))
}
