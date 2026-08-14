// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// EffectScoper is an optional interface an events.EventBus may implement so
// that the subscriptions an effect makes can be attributed to that effect.
//
// It exists because attribution is only knowable at one moment. An effect is
// self-contained: it names its own topics inside Apply, and nothing holds a
// registry of what any effect does (ADR-0038 §4). A ConditionBehavior cannot
// name itself either — it is IsApplied/Apply/Remove/ToJSON and nothing more.
// So the only party that knows "the subscriptions about to be made belong to
// this effect" is the loader that just routed a ref to build the behaviour,
// in the instant before it calls Apply. A loader that consults this interface
// hands that knowledge to the bus instead of discarding it.
//
// This is an optional interface rather than a parameter on purpose. Adding it
// to events.EventBus would break every implementer for a concern only an
// attach site has; adding it as a parameter would fork every load path into a
// scoped and an unscoped variant. A bus that does not implement it is used
// exactly as before, so every existing caller is unaffected.
type EffectScoper interface {
	// ScopeToEffect returns the bus an effect's Apply should be given, so that
	// subscriptions made during that call are attributed to ref. The returned
	// bus must behave as the bus it came from; scoping is bookkeeping, not a
	// change in delivery.
	ScopeToEffect(ref core.Ref) events.EventBus
}

// BusForEffect returns the bus that the effect named by ref should be applied
// with: a scoped view when bus is an EffectScoper, and bus itself otherwise.
//
// A nil return from ScopeToEffect falls back to bus rather than propagating a
// nil bus into Apply, because a bookkeeping implementation must never be able
// to stop an effect from working.
func BusForEffect(bus events.EventBus, ref core.Ref) events.EventBus {
	scoper, ok := bus.(EffectScoper)
	if !ok {
		return bus
	}

	scoped := scoper.ScopeToEffect(ref)
	if scoped == nil {
		return bus
	}

	return scoped
}
