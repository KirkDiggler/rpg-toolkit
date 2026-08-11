package character

import (
	"testing"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func TestCharacterUsesSharedConditionImmunityEffect(t *testing.T) {
	c := &Character{id: "hero"}
	effect, err := NewConditionImmunityEffect(
		c.id,
		dnd5eEvents.ConditionCharmed,
		dnd5eEvents.ConditionExhaustion,
	)
	if err != nil {
		t.Fatal(err)
	}
	c.AddConditionEffect(effect)

	if !c.IsConditionImmune(dnd5eEvents.ConditionCharmed) {
		t.Fatal("character should be immune to Charmed")
	}
	if !c.IsConditionImmune(dnd5eEvents.ConditionExhaustion5) {
		t.Fatal("character should be immune to every exhaustion level")
	}
	if c.IsConditionImmune(dnd5eEvents.ConditionPoisoned) {
		t.Fatal("character should not be immune to Poisoned")
	}
}
