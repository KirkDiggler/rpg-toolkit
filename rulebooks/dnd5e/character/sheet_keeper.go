// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"log/slog"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// SheetKeeper is a character's own reaction to the world, made attachable.
//
// A sheet does not merely sit there while an encounter happens around it: a
// condition applied to this character has to land on its list, a condition
// removed has to leave it, healing has to move its hit points, a condition
// that changed its OWN persisted state has to leave this sheet needing a save,
// and an effect that cannot reach the ledger has to be able to ask this sheet
// to pay. That is five subscriptions, and until this type existed they were
// wired invisibly inside LoadFromData — real behaviour that no caller could
// see, name, or take back.
//
// Character-owned recoverable resources deliberately do not subscribe here.
// Character.LongRest and Character.ShortRest own those pools directly; putting
// them on RestTopic as well would recover attached hit dice twice. Feature- and
// condition-owned state listens through those effects' own lifecycle instead.
//
// Here it is an attachable like any other: [SheetKeeper.Apply] takes a bus,
// [SheetKeeper.Remove] gives it back. Whoever owns the bus decides when the
// sheet starts listening, and the subscriptions appear in a registration
// ledger under the participant that made them instead of as anonymous entries
// nobody can attribute (ADR-0038, and rpg-toolkit#985).
//
// Get one from [Character.SheetKeeper]. It is the character's own, not a fresh
// one per call, and an already-applied keeper refuses a second [SheetKeeper.Apply]
// with [rpgerr.CodeAlreadyExists] rather than quietly subscribing the sheet
// twice. A failed Apply leaves nothing behind and can be retried.
type SheetKeeper struct {
	character *Character

	// subscriptionIDs are the hooks this keeper granted, so Remove can revoke
	// exactly those and nothing else.
	subscriptionIDs []string

	// attachedFeatures are the optional feature lifecycles this keeper put on a
	// bus, paired with the exact scoped bus each Apply received so Remove uses
	// the same attribution view.
	attachedFeatures []attachedFeature
}

type featureLifecycle interface {
	Apply(context.Context, events.EventBus) error
	Remove(context.Context, events.EventBus) error
}

type attachedFeature struct {
	lifecycle featureLifecycle
	bus       events.EventBus
}

// SheetKeeper returns the attachable that carries this character's five
// self-subscriptions and optional feature lifecycles.
//
// The keeper is created once and kept, so that two callers asking a character
// for its keeper get the same one and cannot accidentally subscribe the sheet
// twice.
func (c *Character) SheetKeeper() *SheetKeeper {
	if c.keeper == nil {
		c.keeper = &SheetKeeper{character: c}
	}

	return c.keeper
}

// Apply subscribes the sheet's five handlers and every feature that exposes an
// Apply/Remove bus lifecycle.
//
// The handlers close over this bus rather than reading one off the character:
// a sheet that was loaded purely has no bus of its own, and a sheet that does
// must still react on the bus it was attached to and no other. Each attachable
// feature receives a bus scoped to its own Ref so registration attribution is
// preserved without widening the Feature interface or switching on refs.
//
// **A failed Apply is a no-op.** Whatever it managed to subscribe before the
// failure is revoked, the sheet is left holding the bus it held before, and the
// keeper is not left in the applied state — so the bus carries no hooks nobody
// asked for, and the caller can retry against another bus. A half-attached
// sheet that still reports itself attached is worse than one that never
// attached: the second is a failure, the first is a leak with an alibi.
func (k *SheetKeeper) Apply(ctx context.Context, bus events.EventBus) error {
	var previousBus events.EventBus
	if k.character != nil {
		previousBus = k.character.bus
	}

	if err := k.subscribeSelf(ctx, bus); err != nil {
		return err
	}

	if err := k.applyFeatures(ctx, bus); err != nil {
		k.unsubscribeSelf(ctx, bus)
		k.character.bus = previousBus
		return err
	}

	return nil
}

// subscribeSelf makes the five self-subscriptions, and nothing else.
// Character-owned resources are restored by the Character rest verbs, not by
// subscribing the same pools to the RestTopic those verbs publish.
func (k *SheetKeeper) subscribeSelf(ctx context.Context, bus events.EventBus) error {
	if k.character == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "sheet keeper has no character")
	}
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if len(k.subscriptionIDs) > 0 {
		// Attaching a sheet twice would double every handler it has: a healing
		// event would heal twice, a granted action would be added twice. The
		// recoverable resources refuse a second Apply for the same reason, so
		// this was already an error for any character carrying one — it is an
		// error for all of them now, and it says so.
		return rpgerr.New(rpgerr.CodeAlreadyExists, "sheet keeper is already applied")
	}

	c := k.character
	previousBus := c.bus

	// Park the bus on the sheet for the verb methods that still read it —
	// MakeSavingThrow, EffectiveAC, the rests, Cleanup.
	// They go away with rpg-toolkit#965 and #966, and this line goes with them.
	// Nothing here reads that field: every handler below closes over the bus it
	// was handed.
	c.bus = bus

	// One table rather than five near-identical blocks, so that the rollback
	// on a failed subscription is written once and cannot be forgotten for one
	// of them.
	handlers := []struct {
		what      string
		subscribe func() (string, error)
	}{
		{"condition applied", func() (string, error) {
			return dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(ctx,
				func(ctx context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
					return c.onConditionApplied(ctx, bus, event)
				})
		}},
		{"condition removed events", func() (string, error) {
			return dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx, c.onConditionRemoved)
		}},
		{"healing received", func() (string, error) {
			return dnd5eEvents.HealingReceivedTopic.On(bus).Subscribe(ctx, c.onHealingReceived)
		}},
		{"condition state changed", func() (string, error) {
			return dnd5eEvents.ConditionStateChangedTopic.On(bus).Subscribe(ctx, c.onConditionStateChanged)
		}},
		{"spend requested", func() (string, error) {
			return dnd5eEvents.SpendRequestedTopic.On(bus).Subscribe(ctx, c.onSpendRequested)
		}},
	}

	for _, handler := range handlers {
		subID, err := handler.subscribe()
		if err != nil {
			// Take back the ones that did land, and give the sheet back the bus
			// it was holding: a sheet subscribed to two of its five handlers
			// reacts to some of the world and not the rest, which is a harder
			// bug to see than not attaching at all.
			k.unsubscribeSelf(ctx, bus)
			c.bus = previousBus

			return rpgerr.Wrapf(err, "failed to subscribe to %s", handler.what)
		}

		k.track(subID)
	}

	return nil
}

// unsubscribeSelf revokes the subscriptions this keeper granted and forgets
// them, leaving the keeper unapplied and therefore appliable again.
func (k *SheetKeeper) unsubscribeSelf(ctx context.Context, bus events.EventBus) {
	for _, subID := range k.subscriptionIDs {
		_ = bus.Unsubscribe(ctx, subID)
	}

	k.character.forgetSubscriptions(k.subscriptionIDs)
	k.subscriptionIDs = nil
}

// Remove revokes every subscription this keeper granted.
//
// It revokes what it granted and nothing else: the sheet may be carrying
// subscriptions from elsewhere, and a keeper that unsubscribed a list it did
// not build would silence hooks it never made. Features come off newest first,
// reversing their persisted-order Apply calls.
func (k *SheetKeeper) Remove(ctx context.Context, bus events.EventBus) error {
	if k.character == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "sheet keeper has no character")
	}
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	var firstErr error

	for i := len(k.attachedFeatures) - 1; i >= 0; i-- {
		attached := k.attachedFeatures[i]
		if err := attached.lifecycle.Remove(ctx, attached.bus); err != nil && firstErr == nil {
			firstErr = rpgerr.Wrapf(err, "failed to remove feature")
		}
	}
	k.attachedFeatures = nil

	for _, subID := range k.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil && firstErr == nil {
			firstErr = rpgerr.Wrapf(err, "failed to unsubscribe")
		}
	}

	k.character.forgetSubscriptions(k.subscriptionIDs)
	k.subscriptionIDs = nil

	return firstErr
}

// applyFeatures attaches only features that opt into the existing Apply/Remove
// bus lifecycle. Non-reactive features remain ordinary values. A strict
// failure removes earlier features and leaves the sheet unchanged for retry;
// the lenient path drops only the feature that could not attach and continues.
func (k *SheetKeeper) applyFeatures(ctx context.Context, bus events.EventBus) error {
	c := k.character
	kept := make([]features.Feature, 0, len(c.features))

	for _, feature := range c.features {
		lifecycle, ok := feature.(featureLifecycle)
		if !ok {
			kept = append(kept, feature)
			continue
		}

		featureBus := dnd5eEvents.BusForEffect(bus, *feature.Ref())
		if err := lifecycle.Apply(ctx, featureBus); err != nil {
			_ = lifecycle.Remove(ctx, featureBus)

			if c.policy == strictEffects {
				for i := len(k.attachedFeatures) - 1; i >= 0; i-- {
					attached := k.attachedFeatures[i]
					_ = attached.lifecycle.Remove(ctx, attached.bus)
				}
				k.attachedFeatures = nil
				return rpgerr.Wrapf(err, "failed to apply feature %s", feature.Ref().String())
			}

			warnDropped(c.id, "feature", *feature.Ref(), err, slog.String("phase", "apply"))
			continue
		}

		k.attachedFeatures = append(k.attachedFeatures, attachedFeature{
			lifecycle: lifecycle,
			bus:       featureBus,
		})
		kept = append(kept, feature)
	}

	if c.policy == lenientEffects {
		c.features = kept
	}
	return nil
}

// track records a subscription on the keeper and on the character.
//
// Both, because the two have different questions to answer: the keeper needs
// to know what to revoke in Remove, and Cleanup — which predates the keeper
// and is still what most callers use to tear a character down — unsubscribes
// whatever the character is carrying.
func (k *SheetKeeper) track(subID string) {
	k.subscriptionIDs = append(k.subscriptionIDs, subID)
	k.character.subscriptionIDs = append(k.character.subscriptionIDs, subID)
}
