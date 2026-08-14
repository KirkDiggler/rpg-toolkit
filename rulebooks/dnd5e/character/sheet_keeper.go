// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// SheetKeeper is a character's own reaction to the world, made attachable.
//
// A sheet does not merely sit there while an encounter happens around it: a
// condition applied to this character has to land on its list, a condition
// removed has to leave it, healing has to move its hit points, and an action
// granted or removed has to show up among the things it can do. Its recoverable
// resources have to hear a rest. That is five subscriptions and a handful of
// resources, and until this type existed they were wired invisibly inside
// LoadFromData — real behaviour that no caller could see, name, or take back.
//
// Here it is an attachable like any other: [SheetKeeper.Apply] takes a bus,
// [SheetKeeper.Remove] gives it back. Whoever owns the bus decides when the
// sheet starts listening, and the subscriptions appear in a registration
// ledger under the participant that made them instead of as anonymous entries
// nobody can attribute (ADR-0038, and rpg-toolkit#985).
//
// Get one from [Character.SheetKeeper]. It is the character's, not a copy of
// it: applying the same character's keeper twice subscribes it twice.
type SheetKeeper struct {
	character *Character

	// subscriptionIDs are the hooks this keeper granted, so Remove can revoke
	// exactly those and nothing else.
	subscriptionIDs []string
}

// SheetKeeper returns the attachable that carries this character's own
// behaviour — the five self-subscriptions and its recoverable resources.
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

// Apply subscribes the sheet's five handlers to bus and puts the character's
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
func (k *SheetKeeper) Apply(ctx context.Context, bus events.EventBus) error {
	if err := k.subscribeSelf(ctx, bus); err != nil {
		return err
	}

	return k.applyResources(ctx, bus)
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

	// Park the bus on the sheet for the verb methods that still read it —
	// MakeSavingThrow, ActivateCombatAbility, EffectiveAC, the rests, Cleanup.
	// They go away with rpg-toolkit#965 and #966, and this line goes with them.
	// Nothing here reads that field: every handler below closes over the bus it
	// was handed.
	c.bus = bus

	appliedID, err := dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(ctx,
		func(ctx context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			return c.onConditionApplied(ctx, bus, event)
		})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to condition applied")
	}
	k.track(appliedID)

	removedID, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx, c.onConditionRemoved)
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to condition removed events")
	}
	k.track(removedID)

	healingID, err := dnd5eEvents.HealingReceivedTopic.On(bus).Subscribe(ctx, c.onHealingReceived)
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to healing received")
	}
	k.track(healingID)

	grantedID, err := dnd5eEvents.ActionGrantedTopic.On(bus).Subscribe(ctx,
		func(ctx context.Context, event dnd5eEvents.ActionGrantedEvent) error {
			return c.onActionGranted(ctx, bus, event)
		})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to action granted")
	}
	k.track(grantedID)

	actionRemovedID, err := dnd5eEvents.ActionRemovedTopic.On(bus).Subscribe(ctx, c.onActionRemoved)
	if err != nil {
		return rpgerr.Wrapf(err, "failed to subscribe to action removed")
	}
	k.track(actionRemovedID)

	return nil
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
func (k *SheetKeeper) applyResources(ctx context.Context, bus events.EventBus) error {
	c := k.character

	for key, resource := range c.resources {
		if err := resource.Apply(ctx, bus); err != nil {
			// Clean up whatever the failed Apply managed to subscribe.
			_ = resource.Remove(ctx, bus)

			if c.policy == strictEffects {
				return rpgerr.Wrapf(err, "failed to apply resource %q", key)
			}

			// Lenient: the legacy path kept only the resources that applied.
			delete(c.resources, key)
		}
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
