package core_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

func TestTypedRef(t *testing.T) {
	t.Run("String with valid ref", func(t *testing.T) {
		ref := core.MustNewRef(core.RefInput{
			Module: "combat",
			Type:   "attack",
			Value:  "melee",
		})
		typed := core.TypedRef[AttackEvent]{Ref: ref}

		got := typed.String()
		want := "combat:attack:melee"

		if got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("String with nil ref", func(t *testing.T) {
		typed := core.TypedRef[AttackEvent]{Ref: nil}

		got := typed.String()
		want := ""

		if got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})

	t.Run("Type safety maintains different refs", func(t *testing.T) {
		// TypedRef allows the same ref to be typed differently
		// This is useful when the same ref needs different type associations
		attackRef := core.TypedRef[AttackEvent]{
			Ref: core.MustNewRef(core.RefInput{
				Module: "combat",
				Type:   "event",
				Value:  "attack",
			}),
		}

		damageRef := core.TypedRef[DamageEvent]{
			Ref: core.MustNewRef(core.RefInput{
				Module: "combat",
				Type:   "event",
				Value:  "damage",
			}),
		}

		// Verify they maintain their separate string representations
		if attackRef.String() != "combat:event:attack" {
			t.Errorf("attackRef.String() = %q, want %q", attackRef.String(), "combat:event:attack")
		}

		if damageRef.String() != "combat:event:damage" {
			t.Errorf("damageRef.String() = %q, want %q", damageRef.String(), "combat:event:damage")
		}

		// Verify they are not equal (different underlying refs)
		if attackRef.String() == damageRef.String() {
			t.Error("attackRef and damageRef should have different string representations")
		}
	})

	t.Run("Same ref with different types", func(t *testing.T) {
		// This shows how the same ref can be associated with different types
		// Useful for event systems where the same event ID might have different payload types
		sharedRef := core.MustNewRef(core.RefInput{
			Module: "game",
			Type:   "event",
			Value:  "turn_end",
		})

		// Same ref, but typed for different event structures
		playerTurnEnd := core.TypedRef[PlayerTurnEndEvent]{Ref: sharedRef}
		monsterTurnEnd := core.TypedRef[MonsterTurnEndEvent]{Ref: sharedRef}

		// Both have the same string representation
		if playerTurnEnd.String() != "game:event:turn_end" {
			t.Errorf("playerTurnEnd.String() = %q, want %q", playerTurnEnd.String(), "game:event:turn_end")
		}

		if monsterTurnEnd.String() != "game:event:turn_end" {
			t.Errorf("monsterTurnEnd.String() = %q, want %q", monsterTurnEnd.String(), "game:event:turn_end")
		}

		// They refer to the same underlying ref
		if playerTurnEnd.String() != monsterTurnEnd.String() {
			t.Error("Both typed refs should have the same string representation when using the same underlying ref")
		}
	})
}

// Test types for demonstration
type AttackEvent struct {
	AttackerID string
	Damage     int
}

type DamageEvent struct {
	TargetID string
	Amount   int
}

type PlayerTurnEndEvent struct {
	PlayerID string
	Actions  int
}

type MonsterTurnEndEvent struct {
	MonsterID  string
	Initiative int
}
