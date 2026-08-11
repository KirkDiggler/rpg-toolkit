package conditions

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ApplicationTarget is an entity that can receive a condition or other active
// effect. The technical ConditionBehavior name is retained for compatibility;
// immunity itself is limited to the fifteen standard D&D conditions.
type ApplicationTarget interface {
	IsConditionImmune(dnd5eEvents.ConditionType) bool
	AddConditionEffect(dnd5eEvents.ConditionBehavior)
}

// ApplyEffect is the shared front door for applying an effect to a target.
// It checks condition immunity before any behavior subscribes to game events.
// It returns false, nil when a standard condition is blocked by immunity.
func ApplyEffect(
	ctx context.Context,
	bus events.EventBus,
	target ApplicationTarget,
	conditionType dnd5eEvents.ConditionType,
	effect dnd5eEvents.ConditionBehavior,
) (bool, error) {
	if target == nil {
		return false, rpgerr.New(rpgerr.CodeInvalidArgument, "condition target is required")
	}
	if effect == nil {
		return false, rpgerr.New(rpgerr.CodeInvalidArgument, "effect is required")
	}
	if dnd5eEvents.IsStandardCondition(conditionType) && target.IsConditionImmune(conditionType) {
		return false, nil
	}
	if err := effect.Apply(ctx, bus); err != nil {
		_ = effect.Remove(ctx, bus)
		return false, rpgerr.Wrap(err, "apply effect")
	}
	target.AddConditionEffect(effect)
	return true, nil
}
