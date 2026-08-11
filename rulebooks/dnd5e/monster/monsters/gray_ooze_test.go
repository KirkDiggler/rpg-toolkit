package monsters

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
)

type grayOozeTestEffect struct{ applied bool }

func (e *grayOozeTestEffect) IsApplied() bool { return e.applied }
func (e *grayOozeTestEffect) Apply(context.Context, events.EventBus) error {
	e.applied = true
	return nil
}
func (e *grayOozeTestEffect) Remove(context.Context, events.EventBus) error {
	e.applied = false
	return nil
}
func (e *grayOozeTestEffect) ToJSON() (json.RawMessage, error) { return json.RawMessage(`{}`), nil }

func TestGrayOozeConditionImmunityApplication(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	data := NewGrayOoze("gray-ooze").ToData()
	ooze, err := monster.LoadFromData(ctx, data, bus)
	if err != nil {
		t.Fatal(err)
	}
	if err := monstertraits.LoadMonsterConditions(ctx, ooze, data.Conditions, bus, nil); err != nil {
		t.Fatal(err)
	}

	prone := &grayOozeTestEffect{}
	err = dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: ooze, Type: dnd5eEvents.ConditionProne, Condition: prone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prone.IsApplied() {
		t.Fatal("Gray Ooze accepted Prone despite condition immunity")
	}

	poisoned := &grayOozeTestEffect{}
	err = dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: ooze, Type: dnd5eEvents.ConditionPoisoned, Condition: poisoned,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !poisoned.IsApplied() {
		t.Fatal("Gray Ooze rejected Poisoned despite not being immune")
	}
}
