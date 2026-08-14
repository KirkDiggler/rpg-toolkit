// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// Registration is one subscription resolution granted during an interaction.
//
// The list of these is the answer to three questions that were previously
// unanswerable: what is attached (before anything runs), what a given effect
// attached (including nothing at all), and what has to be revoked at the end.
type Registration struct {
	// Participant is the ID of the sheet being attached when the subscription
	// was made.
	Participant string

	// Effect names the effect that made it. The zero Ref means the
	// subscription is the participant's own machinery — a character listening
	// for conditions applied to it, say — rather than an effect's, and keeping
	// those apart is what makes "this effect attached nothing" falsifiable.
	Effect core.Ref

	// Topic is what was subscribed to.
	Topic events.Topic

	// ID is the bus's own subscription ID, which is what teardown revokes.
	ID string
}

// ledger is the shared record every view of the surface writes into.
type ledger struct {
	mu      sync.Mutex
	entries []Registration
}

func (l *ledger) record(r Registration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, r)
}

func (l *ledger) all() []Registration {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]Registration, len(l.entries))
	copy(out, l.entries)

	return out
}

// surface is the instrumented events.EventBus everything is attached through.
//
// It is a delegating wrapper: every call reaches the real bus unchanged, and
// the only thing added is that a Subscribe is written down. Because
// events.EventBus is three methods, this costs nothing at the ~26 existing
// Apply sites — they keep their exact signature and never learn they are
// talking to a surface rather than a bus.
//
// A surface is also a *view*: scoping to a participant or to an effect returns
// another surface over the same bus and the same ledger, differing only in
// what it stamps on what it records. Attribution is therefore by construction
// — whoever routed a ref to build a behaviour hands the surface that ref
// before calling Apply — rather than by interrogation, which a
// ConditionBehavior cannot answer anyway.
type surface struct {
	inner       events.EventBus
	ledger      *ledger
	participant string
	effect      core.Ref
}

// newSurface returns the root surface over a fresh bus.
func newSurface(inner events.EventBus) *surface {
	return &surface{inner: inner, ledger: &ledger{}}
}

// forParticipant returns the view that attributes subscriptions to a
// participant and to no particular effect.
func (s *surface) forParticipant(id string) *surface {
	return &surface{inner: s.inner, ledger: s.ledger, participant: id}
}

// ScopeToEffect implements dnd5eEvents.EffectScoper: it returns the view a
// loader should apply one effect through, so everything that effect subscribes
// to is recorded under its ref.
func (s *surface) ScopeToEffect(ref core.Ref) events.EventBus {
	return &surface{inner: s.inner, ledger: s.ledger, participant: s.participant, effect: ref}
}

// Subscribe registers a handler and writes down who asked for it.
func (s *surface) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	id, err := s.inner.Subscribe(ctx, topic, handler)
	if err != nil {
		return "", err
	}

	s.ledger.record(Registration{
		Participant: s.participant,
		Effect:      s.effect,
		Topic:       topic,
		ID:          id,
	})

	return id, nil
}

// Unsubscribe delegates. The ledger entry is deliberately left in place: it
// records that the subscription was granted, which stays true, and teardown is
// idempotent so a revoked ID costs nothing at the end.
func (s *surface) Unsubscribe(ctx context.Context, id string) error {
	return s.inner.Unsubscribe(ctx, id)
}

// Publish delegates unchanged.
func (s *surface) Publish(ctx context.Context, topic events.Topic, event any) error {
	return s.inner.Publish(ctx, topic, event)
}

// registrations returns everything granted so far, in the order it was granted.
func (s *surface) registrations() []Registration {
	return s.ledger.all()
}

// teardown revokes every subscription resolution granted, newest first.
//
// Newest first so an effect that layered subscriptions comes apart in the
// reverse of the order it went together. Every revocation is attempted even if
// one fails, because a bus that refuses one Unsubscribe is no reason to leak
// the rest.
func (s *surface) teardown(ctx context.Context) error {
	entries := s.ledger.all()

	var errs []error
	for i := len(entries) - 1; i >= 0; i-- {
		if err := s.inner.Unsubscribe(ctx, entries[i].ID); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// The surface is the scoping bus the dnd5e load paths look for; if that ever
// stops being true, attribution silently degrades to per-participant, so the
// compiler is asked to care.
var _ dnd5eEvents.EffectScoper = (*surface)(nil)
var _ events.EventBus = (*surface)(nil)
