// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"log/slog"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// SheetKeeper is a character's own reaction to the world, made attachable.
//
// A sheet does not merely sit there while an encounter happens around it: a
// condition applied to this character has to land on its list, a condition
// removed has to leave it, and healing has to move its hit points. Its
// recoverable resources have to hear a rest. That is three subscriptions and a handful of
// resources, and until this type existed they were wired invisibly inside
// LoadFromData — real behaviour that no caller could see, name, or take back.
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
}

// SheetKeeper returns the attachable that carries this character's own
// behaviour — the three self-subscriptions and its recoverable resources.
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

// Apply subscribes the sheet's three handlers to bus and puts the character's
// recoverable resources on it.
//
// The handlers close over this bus rather than reading one off the character:
// a sheet that was loaded purely has no bus of its own, and a sheet that does
// must still react on the bus it was attached to and no other.
//
// A resource that fails to apply fails the whole attach on a strictly loaded
// sheet, and is dropped from the sheet on a leniently loaded one — dropping is
// what LoadFromData has always done, and it is exactly how a resource goes
// missing between two saves.
//
// **A failed Apply is a no-op.** Whatever it managed to subscribe before the
// failure is revoked, the sheet is left holding the bus it held before, and the
// keeper is not left in the applied state — so the bus carries no hooks nobody
// asked for, and the caller can retry against another bus. A half-attached
// sheet that still reports itself attached is worse than one that never
// attached: the second is a failure, the first is a leak with an alibi.
func (k *SheetKeeper) Apply(ctx context.Context, bus events.EventBus) error {
	// Read before subscribeSelf parks the new one, so the rollback below can
	// give the sheet back exactly what it was holding.
	var previousBus events.EventBus
	if k.character != nil {
		previousBus = k.character.bus
	}

	if err := k.subscribeSelf(ctx, bus); err != nil {
		return err
	}

	if err := k.applyResources(ctx, bus); err != nil {
		// The handlers went on before the resources did, so they are this
		// function's to take back.
		k.unsubscribeSelf(ctx, bus)
		k.character.bus = previousBus

		return err
	}

	return nil
}

// subscribeSelf makes the five self-subscriptions, and nothing else.
//
// Separate from the resources because a freshly finalized character has always
// had the handlers without its resources on the bus: Character.LongRest
// restores its resources directly and then publishes the rest event, so a
// character whose resources are also subscribed recovers hit dice twice. Load
// has always attached both and inherits that; Finalize has always attached
// neither of the two halves' resources, and this keeps it that way.
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
	// MakeSavingThrow, ActivateCombatAbility, EffectiveAC, the rests, Cleanup.
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

// Remove revokes every subscription this keeper granted and takes the
// character's recoverable resources back off the bus.
//
// It revokes what it granted and nothing else: the sheet may be carrying
// subscriptions from elsewhere, and a keeper that unsubscribed a list it did
// not build would silence hooks it never made.
func (k *SheetKeeper) Remove(ctx context.Context, bus events.EventBus) error {
	if k.character == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "sheet keeper has no character")
	}
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	var firstErr error

	for _, subID := range k.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil && firstErr == nil {
			firstErr = rpgerr.Wrapf(err, "failed to unsubscribe")
		}
	}

	for key, resource := range k.character.resources {
		if err := resource.Remove(ctx, bus); err != nil && firstErr == nil {
			firstErr = rpgerr.Wrapf(err, "failed to remove resource %q", key)
		}
	}

	k.character.forgetSubscriptions(k.subscriptionIDs)
	k.subscriptionIDs = nil

	return firstErr
}

// applyResources puts each recoverable resource on the bus so it hears rests.
//
// On the strict path a resource that will not apply takes the whole attach with
// it, and the ones already applied come back off: a sheet holding some of its
// resources on a bus and the rest off it recovers unevenly on the next rest,
// and nothing about the sheet says which is which.
func (k *SheetKeeper) applyResources(ctx context.Context, bus events.EventBus) error {
	c := k.character
	applied := make([]*combat.RecoverableResource, 0, len(c.resources))

	for key, resource := range c.resources {
		if err := resource.Apply(ctx, bus); err != nil {
			// Clean up whatever the failed Apply managed to subscribe.
			_ = resource.Remove(ctx, bus)

			if c.policy == strictEffects {
				for i := len(applied) - 1; i >= 0; i-- {
					_ = applied[i].Remove(ctx, bus)
				}

				return rpgerr.Wrapf(err, "failed to apply resource %q", key)
			}

			// Lenient: the legacy path kept only the resources that applied.
			warnDropped(c.id, "resource", core.Ref{}, err,
				slog.String("resource", string(key)), slog.String("phase", "apply"))
			delete(c.resources, key)

			continue
		}

		applied = append(applied, resource)
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
