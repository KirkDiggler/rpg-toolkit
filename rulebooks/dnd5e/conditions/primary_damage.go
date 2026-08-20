package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// primaryWeaponComponent returns the canonical weapon pool marked as carrying
// the attack ability modifier. Attack-specific metadata belongs to this
// component, not to the damage-chain envelope.
func primaryWeaponComponent(event *dnd5eEvents.DamageChainEvent) *dnd5eEvents.DamageComponent {
	for i := range event.Components {
		component := &event.Components[i]
		if component.Source == dnd5eEvents.DamageSourceWeapon &&
			component.HasProperty(damage.AddsAttackAbilityModifier) {
			return component
		}
	}
	return nil
}
