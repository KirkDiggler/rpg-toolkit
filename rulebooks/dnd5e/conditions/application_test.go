package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type applicationTarget struct {
	immuneTo map[dnd5eEvents.ConditionType]bool
	added    int
}

func (t *applicationTarget) IsConditionImmune(kind dnd5eEvents.ConditionType) bool {
	return t.immuneTo[kind]
}
func (t *applicationTarget) AddConditionEffect(dnd5eEvents.ConditionBehavior) { t.added++ }

type testEffect struct{ applied bool }

func (e *testEffect) IsApplied() bool                               { return e.applied }
func (e *testEffect) Apply(context.Context, events.EventBus) error  { e.applied = true; return nil }
func (e *testEffect) Remove(context.Context, events.EventBus) error { e.applied = false; return nil }
func (e *testEffect) ToJSON() (json.RawMessage, error)              { return json.RawMessage(`{}`), nil }

func TestApplyEffectBlocksImmuneStandardCondition(t *testing.T) {
	target := &applicationTarget{immuneTo: map[dnd5eEvents.ConditionType]bool{dnd5eEvents.ConditionProne: true}}
	effect := &testEffect{}
	applied, err := ApplyEffect(context.Background(), events.NewEventBus(), target, dnd5eEvents.ConditionProne, effect)
	if err != nil || applied || effect.applied || target.added != 0 {
		t.Fatalf("immune result = applied:%v effect:%v added:%d err:%v", applied, effect.applied, target.added, err)
	}
}

func TestApplyEffectAllowsNonImmuneCondition(t *testing.T) {
	target := &applicationTarget{immuneTo: map[dnd5eEvents.ConditionType]bool{}}
	effect := &testEffect{}
	applied, err := ApplyEffect(context.Background(), events.NewEventBus(), target, dnd5eEvents.ConditionPoisoned, effect)
	if err != nil || !applied || !effect.applied || target.added != 1 {
		t.Fatalf("non-immune result = applied:%v effect:%v added:%d err:%v", applied, effect.applied, target.added, err)
	}
}
