package monsters

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/attack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/stretchr/testify/require"
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

func TestGrayOozePseudopod(t *testing.T) {
	def := grayOozePseudopod(t)
	require.Equal(t, 3, def.Bonus.Fixed)
	require.Equal(t, -1, def.Damage.Pools[0].FlatBonus)
	require.Equal(t, damage.Acid, def.Damage.Pools[1].Type)
}

func TestGrayOozeCriticalPseudopod(t *testing.T) {
	result := resolveGrayOozeCritical(t)
	require.Equal(t, -1, result.Breakdown.Components[0].FlatBonus)
	require.Len(t, result.Breakdown.Components[0].FinalDiceRolls, 2)
	require.Len(t, result.Breakdown.Components[1].FinalDiceRolls, 4)
}

func resolveGrayOozeCritical(t *testing.T) *combat.AttackResult {
	t.Helper()
	attacker := NewGrayOoze("gray-ooze")
	target := NewGrayOoze("target")
	ctx := combat.WithCombatantLookup(context.Background(), monsterLookup{
		attacker.GetID(): attacker,
		target.GetID():   target,
	})
	result, err := combat.ResolveAttack(ctx, &combat.AttackInput{
		AttackerID: attacker.GetID(), TargetID: target.GetID(), Attack: ptr(grayOozePseudopod(t)),
		EventBus: events.NewEventBus(), Roller: criticalRoller{},
	})
	require.NoError(t, err)
	return result
}

func ptr(def attack.Definition) *attack.Definition { return &def }

type monsterLookup map[string]combat.Combatant

func (l monsterLookup) Get(id string) (combat.Combatant, error) {
	combatant, ok := l[id]
	if !ok {
		return nil, errors.New("combatant not found")
	}
	return combatant, nil
}

type criticalRoller struct{}

var _ dice.Roller = criticalRoller{}

func (criticalRoller) Roll(_ context.Context, size int) (int, error) {
	if size != 20 {
		return 0, errors.New("unexpected die size")
	}
	return 20, nil
}

func (criticalRoller) RollN(_ context.Context, count, size int) ([]int, error) {
	if size != 6 {
		return nil, errors.New("unexpected die size")
	}
	rolls := make([]int, count)
	for i := range rolls {
		rolls[i] = 1
	}
	return rolls, nil
}

func grayOozePseudopod(t *testing.T) attack.Definition {
	t.Helper()
	return emittedPseudopod(t, NewGrayOoze("gray-ooze"))
}

func emittedPseudopod(t *testing.T, attacker *monster.Monster) attack.Definition {
	t.Helper()
	actions := attacker.Actions()
	require.Len(t, actions, 1)

	ctx := context.Background()
	bus := events.NewEventBus()
	var received dnd5eEvents.AttackEvent
	_, err := dnd5eEvents.AttackTopic.On(bus).Subscribe(ctx, func(_ context.Context, event dnd5eEvents.AttackEvent) error {
		received = event
		return nil
	})
	require.NoError(t, err)

	target := NewGrayOoze("target")
	err = actions[0].Activate(ctx, attacker, monster.MonsterActionInput{
		Bus: bus, Target: target,
		Perception: &monster.PerceptionData{Enemies: []monster.PerceivedEntity{{Entity: target, Distance: 1}}},
	})
	require.NoError(t, err)
	return received.Definition
}
