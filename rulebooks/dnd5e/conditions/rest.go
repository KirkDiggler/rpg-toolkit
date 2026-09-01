// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func subscribeRemoveOnLongRest(
	ctx context.Context,
	bus events.EventBus,
	memberID string,
	ref *core.Ref,
	remove func(context.Context, events.EventBus) error,
) (string, error) {
	return dnd5eEvents.RestTopic.On(bus).Subscribe(ctx,
		func(ctx context.Context, event dnd5eEvents.RestEvent) error {
			if event.CharacterID != memberID || event.RestType != coreResources.ResetLongRest {
				return nil
			}

			if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx,
				dnd5eEvents.ConditionRemovedEvent{
					MemberID:     memberID,
					ConditionRef: ref.String(),
					Reason:       "long rest",
				}); err != nil {
				return err
			}

			return remove(ctx, bus)
		})
}
