package monstertraits

import (
	"testing"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

func TestConditionImmunityOnlyAcceptsStandardConditions(t *testing.T) {
	standard := []dnd5eEvents.ConditionType{
		dnd5eEvents.ConditionBlinded,
		dnd5eEvents.ConditionCharmed,
		dnd5eEvents.ConditionDeafened,
		dnd5eEvents.ConditionExhaustion,
		dnd5eEvents.ConditionFrightened,
		dnd5eEvents.ConditionGrappled,
		dnd5eEvents.ConditionIncapacitated,
		dnd5eEvents.ConditionInvisible,
		dnd5eEvents.ConditionParalyzed,
		dnd5eEvents.ConditionPetrified,
		dnd5eEvents.ConditionPoisoned,
		dnd5eEvents.ConditionProne,
		dnd5eEvents.ConditionRestrained,
		dnd5eEvents.ConditionStunned,
		dnd5eEvents.ConditionUnconscious,
	}
	if len(standard) != 15 {
		t.Fatalf("standard condition count = %d, want 15", len(standard))
	}
	for _, conditionType := range standard {
		t.Run(string(conditionType), func(t *testing.T) {
			if _, err := NewConditionImmunity("gray-ooze", conditionType); err != nil {
				t.Fatal(err)
			}
		})
	}

	if _, err := NewConditionImmunity("gray-ooze", dnd5eEvents.ConditionRaging); err == nil {
		t.Fatal("status Raging was accepted as a condition immunity")
	}
}

func TestConditionImmunityTreatsExhaustionAsOneCanonicalCondition(t *testing.T) {
	trait, err := NewConditionImmunity("gray-ooze", dnd5eEvents.ConditionExhaustion)
	if err != nil {
		t.Fatal(err)
	}
	for _, exhaustion := range []dnd5eEvents.ConditionType{
		dnd5eEvents.ConditionExhaustion1,
		dnd5eEvents.ConditionExhaustion2,
		dnd5eEvents.ConditionExhaustion3,
		dnd5eEvents.ConditionExhaustion4,
		dnd5eEvents.ConditionExhaustion5,
		dnd5eEvents.ConditionExhaustion6,
	} {
		if !trait.IsImmuneTo(exhaustion) {
			t.Errorf("exhaustion immunity did not block %s", exhaustion)
		}
	}
}

func TestMonsterUsesItsConditionImmunityTrait(t *testing.T) {
	m := monster.New(monster.Config{ID: "gray-ooze", Name: "Gray Ooze", HP: 22, AC: 8})
	trait, err := NewConditionImmunity(
		m.GetID(),
		dnd5eEvents.ConditionBlinded,
		dnd5eEvents.ConditionCharmed,
		dnd5eEvents.ConditionDeafened,
		dnd5eEvents.ConditionExhaustion,
		dnd5eEvents.ConditionFrightened,
		dnd5eEvents.ConditionProne,
	)
	if err != nil {
		t.Fatal(err)
	}
	m.AddCondition(trait)

	for _, conditionType := range []dnd5eEvents.ConditionType{
		dnd5eEvents.ConditionBlinded,
		dnd5eEvents.ConditionCharmed,
		dnd5eEvents.ConditionDeafened,
		dnd5eEvents.ConditionExhaustion3,
		dnd5eEvents.ConditionFrightened,
		dnd5eEvents.ConditionProne,
	} {
		if !m.IsConditionImmune(conditionType) {
			t.Errorf("Gray Ooze should be immune to %s", conditionType)
		}
	}
	if m.IsConditionImmune(dnd5eEvents.ConditionPoisoned) {
		t.Fatal("Gray Ooze should not be immune to Poisoned")
	}
}
