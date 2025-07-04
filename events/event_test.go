package events

import (
	"testing"
	"time"
)

// mockEntity implements core.Entity for testing
type mockEntity struct {
	id         string
	entityType string
}

func (m *mockEntity) GetID() string   { return m.id }
func (m *mockEntity) GetType() string { return m.entityType }

func TestGameEvent(t *testing.T) {
	source := &mockEntity{id: "player-1", entityType: "character"}
	target := &mockEntity{id: "goblin-1", entityType: "monster"}

	event := NewGameEvent(EventBeforeAttackRoll, source, target)

	// Test basic properties
	if event.Type() != EventBeforeAttackRoll {
		t.Errorf("Expected event type %s, got %s", EventBeforeAttackRoll, event.Type())
	}

	if event.Source() != source {
		t.Error("Source entity mismatch")
	}

	if event.Target() != target {
		t.Error("Target entity mismatch")
	}

	// Timestamp should be recent
	if time.Since(event.Timestamp()) > time.Second {
		t.Error("Timestamp too old")
	}

	// Context should be initialized
	if event.Context() == nil {
		t.Error("Context is nil")
	}
}

func TestEventContext(t *testing.T) {
	ctx := NewEventContext()

	// Test Get/Set
	ctx.Set("damage", 10)
	val, ok := ctx.Get("damage")
	if !ok {
		t.Error("Expected to find damage value")
	}
	if damage, ok := val.(int); !ok || damage != 10 {
		t.Errorf("Expected damage to be 10, got %v", val)
	}

	// Test missing key
	_, ok = ctx.Get("missing")
	if ok {
		t.Error("Expected missing key to return false")
	}

	// Test modifiers
	mod1 := NewIntModifier("rage", ModifierDamageBonus, 2, 100)
	mod2 := NewIntModifier("bless", ModifierAttackBonus, 1, 50)

	ctx.AddModifier(mod1)
	ctx.AddModifier(mod2)

	mods := ctx.Modifiers()
	if len(mods) != 2 {
		t.Errorf("Expected 2 modifiers, got %d", len(mods))
	}
}

func TestBasicModifier(t *testing.T) {
	mod := NewIntModifier("rage", ModifierDamageBonus, 2, 100)

	if mod.Source() != "rage" {
		t.Errorf("Expected source 'rage', got %s", mod.Source())
	}

	if mod.Type() != ModifierDamageBonus {
		t.Errorf("Expected type %s, got %s", ModifierDamageBonus, mod.Type())
	}

	if mod.ModifierValue().GetValue() != 2 {
		t.Errorf("Expected value 2, got %v", mod.ModifierValue().GetValue())
	}

	if mod.Priority() != 100 {
		t.Errorf("Expected priority 100, got %d", mod.Priority())
	}
}

func TestEventWithNilEntities(t *testing.T) {
	// Event should handle nil source/target gracefully
	event := NewGameEvent(EventOnTurnStart, nil, nil)

	if event.Source() != nil {
		t.Error("Expected nil source")
	}

	if event.Target() != nil {
		t.Error("Expected nil target")
	}
}

func TestContextDataTypes(t *testing.T) {
	ctx := NewEventContext()

	// Test string
	ctx.Set("string", "test")
	if val, ok := ctx.Get("string"); !ok || val != "test" {
		t.Error("String value mismatch")
	}

	// Test int
	ctx.Set("int", 42)
	if val, ok := ctx.Get("int"); !ok || val != 42 {
		t.Error("Int value mismatch")
	}

	// Test float
	ctx.Set("float", 3.14)
	if val, ok := ctx.Get("float"); !ok || val != 3.14 {
		t.Error("Float value mismatch")
	}

	// Test bool
	ctx.Set("bool", true)
	if val, ok := ctx.Get("bool"); !ok || val != true {
		t.Error("Bool value mismatch")
	}

	// Test slice (just verify it's stored and retrieved)
	slice := []int{1, 2, 3}
	ctx.Set("slice", slice)
	if val, ok := ctx.Get("slice"); !ok {
		t.Error("Failed to get slice")
	} else if s, ok := val.([]int); !ok || len(s) != 3 {
		t.Error("Slice type mismatch")
	}

	// Test map (just verify it's stored and retrieved)
	m := map[string]int{"a": 1}
	ctx.Set("map", m)
	if val, ok := ctx.Get("map"); !ok {
		t.Error("Failed to get map")
	} else if mapVal, ok := val.(map[string]int); !ok || mapVal["a"] != 1 {
		t.Error("Map type mismatch")
	}
}

func TestEventCancellation(t *testing.T) {
	source := &mockEntity{id: "player-1", entityType: "character"}
	target := &mockEntity{id: "goblin-1", entityType: "monster"}

	event := NewGameEvent(EventBeforeAttackRoll, source, target)

	// Initially not cancelled
	if event.IsCancelled() {
		t.Error("New event should not be cancelled")
	}

	// Cancel the event
	event.Cancel()

	// Should now be cancelled
	if !event.IsCancelled() {
		t.Error("Event should be cancelled after calling Cancel()")
	}

	// Cancelling again should be idempotent
	event.Cancel()
	if !event.IsCancelled() {
		t.Error("Event should remain cancelled")
	}
}

func TestModifierPriority(t *testing.T) {
	ctx := NewEventContext()

	// Add modifiers with different priorities
	ctx.AddModifier(NewIntModifier("effect1", ModifierDamageBonus, 1, 200))
	ctx.AddModifier(NewIntModifier("effect2", ModifierDamageBonus, 2, 100))
	ctx.AddModifier(NewIntModifier("effect3", ModifierDamageBonus, 3, 300))

	mods := ctx.Modifiers()
	if len(mods) != 3 {
		t.Fatalf("Expected 3 modifiers, got %d", len(mods))
	}

	// Verify they're stored in order added (not sorted by priority)
	// Sorting happens in the event bus
	if mods[0].Priority() != 200 {
		t.Error("Modifiers should maintain insertion order")
	}
}
