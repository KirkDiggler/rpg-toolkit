// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// subscribeRemoveOnRestInput names the condition a rest removes and how it
// removes itself.
type subscribeRemoveOnRestInput struct {
	Address dnd5eEvents.ConditionAddress
	Remove  func(context.Context, events.EventBus) error

	// LongRestOnly keeps the condition through a short rest. Every
	// combat-scoped condition leaves it false: a short rest is an hour, and
	// an hour ends a fight's conditions (rpg-project#542). Only an effect
	// that genuinely outlasts an hour's rest sets it.
	LongRestOnly bool
}

// subscribeRemoveOnRest removes the condition when its owner rests: on any
// rest, short or long, with the reason "rest"; or, for LongRestOnly, on a long
// rest alone, with the reason "long rest".
func subscribeRemoveOnRest(
	ctx context.Context,
	bus events.EventBus,
	input subscribeRemoveOnRestInput,
) (string, error) {
	reason := "rest"
	if input.LongRestOnly {
		reason = "long rest"
	}

	return dnd5eEvents.RestTopic.On(bus).Subscribe(ctx,
		func(ctx context.Context, event dnd5eEvents.RestEvent) error {
			if event.CharacterID != input.Address.MemberID {
				return nil
			}
			if input.LongRestOnly && event.RestType != coreResources.ResetLongRest {
				return nil
			}

			if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx,
				dnd5eEvents.ConditionRemovedEvent{
					MemberID:     input.Address.MemberID,
					ConditionRef: input.Address.ConditionRef,
					SourceID:     input.Address.SourceID,
					Reason:       reason,
				}); err != nil {
				return err
			}

			return input.Remove(ctx, bus)
		})
}
