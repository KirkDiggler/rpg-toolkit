package resolution

import (
	"context"
	"fmt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

func deliveryRangeState(
	ctx context.Context, attackerID, targetID string, delivery combatActions.AttackDelivery,
) (longRange bool, err error) {
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrBadWorld, err)
	}
	attacker, ok := room.GetEntityPosition(attackerID)
	if !ok {
		return false, fmt.Errorf("%w: attacker %q has no position", ErrBadWorld, attackerID)
	}
	target, ok := room.GetEntityPosition(targetID)
	if !ok {
		return false, fmt.Errorf("%w: target %q has no position", ErrBadWorld, targetID)
	}
	distance := room.GetGrid().Distance(attacker, target)
	maximum := float64(encounter.CellsFromFeet(delivery.MaxRangeFeet()))
	if distance > maximum {
		return false, fmt.Errorf("%w: distance %.0f cells exceeds %d feet", ErrOutOfRange, distance, delivery.MaxRangeFeet())
	}
	if delivery.Ranged != nil && delivery.Ranged.LongFeet > 0 {
		normal := float64(encounter.CellsFromFeet(delivery.NormalRangeFeet()))
		return distance > normal, nil
	}
	return false, nil
}
