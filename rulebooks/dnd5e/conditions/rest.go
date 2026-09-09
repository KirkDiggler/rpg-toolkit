// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type subscribeRemoveOnLongRestInput struct {
	Address dnd5eEvents.ConditionAddress
	Remove  func(context.Context, events.EventBus) error
}

func subscribeRemoveOnLongRest(
	ctx context.Context,
	bus events.EventBus,
	input subscribeRemoveOnLongRestInput,
) (string, error) {
	return dnd5eEvents.RestTopic.On(bus).Subscribe(ctx,
		func(ctx context.Context, event dnd5eEvents.RestEvent) error {
			if event.CharacterID != input.Address.MemberID || event.RestType != coreResources.ResetLongRest {
				return nil
			}

			if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx,
				dnd5eEvents.ConditionRemovedEvent{
					MemberID:     input.Address.MemberID,
					ConditionRef: input.Address.ConditionRef,
					SourceID:     input.Address.SourceID,
					Reason:       "long rest",
				}); err != nil {
				return err
			}

			return input.Remove(ctx, bus)
		})
}
