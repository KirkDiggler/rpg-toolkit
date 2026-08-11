package affinity

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func TestAffinitiesAddTheCorrectTypeSpecificMultiplier(t *testing.T) {
	tests := []struct {
		kind       Kind
		multiplier float64
	}{
		{kind: Resistance, multiplier: 0.5},
		{kind: Vulnerability, multiplier: 2},
		{kind: Immunity, multiplier: 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			bus := events.NewEventBus()
			condition, err := New(tt.kind, "target", damage.Fire, "ring-1")
			if err != nil {
				t.Fatal(err)
			}
			if err := condition.Apply(context.Background(), bus); err != nil {
				t.Fatal(err)
			}

			event := &dnd5eEvents.DamageChainEvent{TargetID: "target", Components: []dnd5eEvents.DamageComponent{
				{FlatBonus: 8, DamageType: damage.Fire},
				{FlatBonus: 5, DamageType: damage.Slashing},
			}}
			chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
			modified, err := dnd5eEvents.DamageChain.On(bus).PublishWithChain(context.Background(), event, chain)
			if err != nil {
				t.Fatal(err)
			}
			result, err := modified.Execute(context.Background(), event)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Components) != 3 {
				t.Fatalf("component count = %d, want 3", len(result.Components))
			}
			modifier := result.Components[2]
			if modifier.DamageType != damage.Fire || modifier.Multiplier != tt.multiplier {
				t.Fatalf("modifier = %#v, want %s multiplier %v", modifier, damage.Fire, tt.multiplier)
			}
		})
	}
}

func TestAffinityCanBeRemoved(t *testing.T) {
	bus := events.NewEventBus()
	condition, err := New(Resistance, "target", damage.Fire, "ring-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := condition.Apply(context.Background(), bus); err != nil {
		t.Fatal(err)
	}
	if err := condition.Remove(context.Background(), bus); err != nil {
		t.Fatal(err)
	}
	if condition.IsApplied() {
		t.Fatal("condition remains applied after removal")
	}
}

func TestEveryDamageTypeIsValidForEveryAffinityKind(t *testing.T) {
	allTypes := append(damage.Physical(), damage.Elemental()...)
	allTypes = append(allTypes, damage.Magical()...)
	if len(allTypes) != 13 {
		t.Fatalf("damage type count = %d, want 13", len(allTypes))
	}

	for _, kind := range []Kind{Resistance, Vulnerability, Immunity} {
		for _, damageType := range allTypes {
			t.Run(string(kind)+"/"+string(damageType), func(t *testing.T) {
				condition, err := New(kind, "target", damageType, "")
				if err != nil {
					t.Fatal(err)
				}
				if condition.damageType != damageType {
					t.Fatalf("damage type = %s, want %s", condition.damageType, damageType)
				}
			})
		}
	}
}
