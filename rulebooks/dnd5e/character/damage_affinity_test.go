package character

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage/affinity"
)

func TestCharacterCanCreateAndPersistDamageImmunityEffect(t *testing.T) {
	effect, err := NewDamageAffinityEffect("hero", affinity.Immunity, damage.Fire, "feature:fire-soul")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := effect.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := conditions.LoadJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.Apply(context.Background(), events.NewEventBus()); err != nil {
		t.Fatal(err)
	}
}
