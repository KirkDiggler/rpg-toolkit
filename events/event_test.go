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

func TestTypedContextAccessors(t *testing.T) {
	ctx := NewEventContext()

	// Test GetInt
	ctx.Set("damage", 10)
	if val, ok := ctx.GetInt("damage"); !ok || val != 10 {
		t.Errorf("GetInt failed: got %d, ok=%v", val, ok)
	}

	// Test GetInt with wrong type
	ctx.Set("name", "test")
	if _, ok := ctx.GetInt("name"); ok {
		t.Error("GetInt should return false for wrong type")
	}

	// Test GetString
	ctx.Set("ability", "STR")
	if val, ok := ctx.GetString("ability"); !ok || val != "STR" {
		t.Errorf("GetString failed: got %s, ok=%v", val, ok)
	}

	// Test GetBool
	ctx.Set("advantage", true)
	if val, ok := ctx.GetBool("advantage"); !ok || val != true {
		t.Errorf("GetBool failed: got %v, ok=%v", val, ok)
	}

	// Test GetFloat64
	ctx.Set("multiplier", 1.5)
	if val, ok := ctx.GetFloat64("multiplier"); !ok || val != 1.5 {
		t.Errorf("GetFloat64 failed: got %f, ok=%v", val, ok)
	}

	// Test GetEntity
	entity := &mockEntity{id: "player-1", entityType: "character"}
	ctx.Set("attacker", entity)
	if val, ok := ctx.GetEntity("attacker"); !ok || val != entity {
		t.Errorf("GetEntity failed: got %v, ok=%v", val, ok)
	}

	// Test GetDuration
	duration := &RoundsDuration{Rounds: 3, StartRound: 1}
	ctx.Set("duration", duration)
	if val, ok := ctx.GetDuration("duration"); !ok || val != duration {
		t.Errorf("GetDuration failed: got %v, ok=%v", val, ok)
	}

	// Test missing keys
	if _, ok := ctx.GetInt("missing"); ok {
		t.Error("GetInt should return false for missing key")
	}
	if _, ok := ctx.GetString("missing"); ok {
		t.Error("GetString should return false for missing key")
	}
	if _, ok := ctx.GetBool("missing"); ok {
		t.Error("GetBool should return false for missing key")
	}
	if _, ok := ctx.GetFloat64("missing"); ok {
		t.Error("GetFloat64 should return false for missing key")
	}
	if _, ok := ctx.GetEntity("missing"); ok {
		t.Error("GetEntity should return false for missing key")
	}
	if _, ok := ctx.GetDuration("missing"); ok {
		t.Error("GetDuration should return false for missing key")
	}
}

func TestEventBuilderPattern(t *testing.T) {
	source := &mockEntity{id: "player-1", entityType: "character"}
	target := &mockEntity{id: "goblin-1", entityType: "monster"}

	// Test builder pattern
	event := NewGameEvent(EventBeforeAttackRoll, nil, nil).
		WithSource(source).
		WithTarget(target).
		WithContext("weapon", "longsword").
		WithContext("damage_type", "slashing").
		WithModifier(NewIntModifier("strength", ModifierAttackBonus, 3, 100))

	// Verify source and target
	if event.Source() != source {
		t.Error("Source not set correctly via builder")
	}
	if event.Target() != target {
		t.Error("Target not set correctly via builder")
	}

	// Verify context values
	weapon, ok := event.Context().GetString("weapon")
	if !ok || weapon != "longsword" {
		t.Error("Weapon context not set correctly")
	}

	damageType, ok := event.Context().GetString("damage_type")
	if !ok || damageType != "slashing" {
		t.Error("Damage type context not set correctly")
	}

	// Verify modifier
	mods := event.Context().Modifiers()
	if len(mods) != 1 {
		t.Fatalf("Expected 1 modifier, got %d", len(mods))
	}
	if mods[0].Source() != "strength" {
		t.Error("Modifier not added correctly via builder")
	}
}

func TestTypedEventTypes(t *testing.T) {
	// Test creating event with typed event type
	event := NewTypedGameEvent(EventTypeBeforeAttackRoll, nil, nil)

	// Should have both string and typed access
	if event.Type() != EventBeforeAttackRoll {
		t.Errorf("String type mismatch: got %s, want %s", event.Type(), EventBeforeAttackRoll)
	}

	if event.TypedType() != EventTypeBeforeAttackRoll {
		t.Errorf("Typed type mismatch: got %v, want %v", event.TypedType(), EventTypeBeforeAttackRoll)
	}

	// Test creating event with string type
	event2 := NewGameEvent(EventOnDamageRoll, nil, nil)

	if event2.Type() != EventOnDamageRoll {
		t.Errorf("String type mismatch: got %s, want %s", event2.Type(), EventOnDamageRoll)
	}

	if event2.TypedType() != EventTypeOnDamageRoll {
		t.Errorf("Typed type mismatch: got %v, want %v", event2.TypedType(), EventTypeOnDamageRoll)
	}

	// Test custom event type
	customEvent := NewGameEvent("custom.event.type", nil, nil)

	if customEvent.Type() != "custom.event.type" {
		t.Error("Custom event type not preserved")
	}

	if customEvent.TypedType() != EventTypeCustom {
		t.Error("Custom event should map to EventTypeCustom")
	}
}

func TestConditionalModifier(t *testing.T) {
	// Create a modifier that only applies to attack rolls
	attackOnlyModifier := &BasicModifier{
		source:   "weapon_focus",
		modType:  ModifierAttackBonus,
		modValue: NewRawValue(2, "weapon_focus"),
		priority: 100,
		condition: func(event Event) bool {
			return event.Type() == EventBeforeAttackRoll || event.Type() == EventOnAttackRoll
		},
	}

	// Test with attack roll event
	attackEvent := NewGameEvent(EventBeforeAttackRoll, nil, nil)
	if !attackOnlyModifier.Condition(attackEvent) {
		t.Error("Modifier should apply to attack roll events")
	}

	// Test with damage roll event
	damageEvent := NewGameEvent(EventOnDamageRoll, nil, nil)
	if attackOnlyModifier.Condition(damageEvent) {
		t.Error("Modifier should not apply to damage roll events")
	}

	// Test modifier with nil condition (always applies)
	alwaysApplies := &BasicModifier{
		source:   "bless",
		modType:  ModifierAttackBonus,
		modValue: NewRawValue(1, "bless"),
		priority: 50,
		// condition is nil
	}

	if !alwaysApplies.Condition(attackEvent) {
		t.Error("Modifier with nil condition should always apply")
	}
	if !alwaysApplies.Condition(damageEvent) {
		t.Error("Modifier with nil condition should always apply")
	}
}

func TestModifierWithDuration(t *testing.T) {
	// Create a modifier with a duration
	tempModifier := &BasicModifier{
		source:   "shield_of_faith",
		modType:  ModifierACBonus,
		modValue: NewRawValue(2, "shield_of_faith"),
		priority: 100,
		duration: &MinutesDuration{Minutes: 10, StartTime: time.Now()},
	}

	// Check duration
	if tempModifier.Duration() == nil {
		t.Fatal("Modifier should have a duration")
	}

	if tempModifier.Duration().Type() != DurationTypeMinutes {
		t.Error("Duration type mismatch")
	}

	// Test permanent modifier (no duration)
	permModifier := &BasicModifier{
		source:   "ring_of_protection",
		modType:  ModifierACBonus,
		modValue: NewRawValue(1, "ring_of_protection"),
		priority: 50,
		// duration is nil
	}

	if permModifier.Duration() != nil {
		t.Error("Permanent modifier should have nil duration")
	}
}

func TestModifierSourceDetails(t *testing.T) {
	sourceDetails := &ModifierSource{
		Type:        "spell",
		Name:        "Bless",
		Description: "You bless up to three creatures of your choice within range",
		Entity:      &mockEntity{id: "cleric-1", entityType: "character"},
	}

	modifier := &BasicModifier{
		source:        "bless",
		modType:       ModifierAttackBonus,
		modValue:      NewDiceValue(1, 4, "bless"),
		priority:      100,
		sourceDetails: sourceDetails,
	}

	// Check source details
	details := modifier.SourceDetails()
	if details == nil {
		t.Fatal("Modifier should have source details")
	}

	if details.Type != "spell" {
		t.Errorf("Source type mismatch: got %s, want spell", details.Type)
	}

	if details.Name != "Bless" {
		t.Errorf("Source name mismatch: got %s, want Bless", details.Name)
	}

	if details.Entity.GetID() != "cleric-1" {
		t.Error("Source entity mismatch")
	}
}

func TestModifierConfig(t *testing.T) {
	// Create a modifier using ModifierConfig
	cfg := ModifierConfig{
		Source:   "divine_favor",
		Type:     ModifierDamageBonus,
		Value:    NewRawValue(3, "divine_favor"),
		Priority: 150,
		Condition: func(e Event) bool {
			// Only applies to weapon attacks
			weapon, ok := e.Context().GetString("weapon")
			return ok && weapon != ""
		},
		Duration: &RoundsDuration{
			Rounds:       10,
			StartRound:   1,
			IncludeStart: true,
		},
		SourceDetails: &ModifierSource{
			Type:        "spell",
			Name:        "Divine Favor",
			Description: "Your prayer empowers you with divine radiance",
			Entity:      &mockEntity{id: "paladin-1", entityType: "character"},
		},
	}

	modifier := NewModifierWithConfig(cfg)

	// Verify all properties
	if modifier.Source() != "divine_favor" {
		t.Error("Source mismatch")
	}

	if modifier.Type() != ModifierDamageBonus {
		t.Error("Type mismatch")
	}

	if modifier.ModifierValue().GetValue() != 3 {
		t.Error("Value mismatch")
	}

	if modifier.Priority() != 150 {
		t.Error("Priority mismatch")
	}

	// Test condition
	weaponEvent := NewGameEvent(EventOnDamageRoll, nil, nil)
	weaponEvent.Context().Set("weapon", "longsword")
	if !modifier.Condition(weaponEvent) {
		t.Error("Modifier should apply to weapon attacks")
	}

	spellEvent := NewGameEvent(EventOnDamageRoll, nil, nil)
	spellEvent.Context().Set("spell", "fireball")
	if modifier.Condition(spellEvent) {
		t.Error("Modifier should not apply to spell attacks")
	}

	// Verify duration
	if modifier.Duration() == nil {
		t.Fatal("Modifier should have duration")
	}
	if modifier.Duration().Type() != DurationTypeRounds {
		t.Error("Duration type mismatch")
	}

	// Verify source details
	if modifier.SourceDetails() == nil {
		t.Fatal("Modifier should have source details")
	}
	if modifier.SourceDetails().Name != "Divine Favor" {
		t.Error("Source details name mismatch")
	}
}
