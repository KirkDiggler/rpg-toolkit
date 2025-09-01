package dnd5e_test

import (
	_ "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

func Example_gameplay() {
	// During a game session...

	// Load character from saved data
	charData := loadCharacterData("ragnar")         // You implement this
	raceData := loadRaceData("human")               // You implement this
	classData := loadClassData("fighter")           // You implement this
	backgroundData := loadBackgroundData("soldier") // You implement this

	// TODO: Update this example for the new character system
	_ = charData
	_ = raceData
	_ = classData
	_ = backgroundData
	// character, _ := dnd5e.LoadCharacterFromData(charData, &raceData, &classData, &backgroundData)

	// NOTE: This example needs updating for the new event-driven condition system
	// In the new system, conditions are applied via events:
	//
	// eventBus := events.NewEventBus()
	// character.ApplyToEventBus(ctx, eventBus)
	//
	// // Publish condition applied event
	// topic := dnd5e.ConditionAppliedTopic.On(eventBus)
	// topic.Publish(ctx, dnd5e.ConditionAppliedEvent{
	//     CharacterID: character.GetID(),
	//     Condition:   poisonedCondition,
	//     Source:      "giant_spider_bite",
	// })

	// Check if character has a condition by ref
	// if character.HasCondition("dnd5e:conditions:poisoned") {
	// 	fmt.Println("Character has disadvantage on attack rolls")
	// }

	// Calculate AC (effects system needs updating too)
	// fmt.Printf("AC: %d\n", character.AC())

	// Save character state
	// updatedData := character.ToData()
	// saveCharacterData(updatedData) // You implement this

	// The saved data includes:
	// - Current conditions (poisoned)
	// - Active effects (bless, shield)
	// - All other character state
}

// Helper stubs for the example
func loadCharacterData(_ string) interface{} { return nil }
func loadRaceData(_ string) interface{}      { return nil }
func loadClassData(_ string) interface{}     { return nil }
func loadBackgroundData(_ string) interface{} { return nil }
func saveCharacterData(_ interface{})        {}

// TestEffectStacking demonstrates how effects will work once updated
// func TestEffectStacking(_ *testing.T) {
// 	// Example of how effects work
// 	character := &dnd5e.Character{}
//
// 	// In the new system, effects would be applied via events similar to conditions
// 	// Effects would be permanent modifiers stored as json.RawMessage
//
// 	// TODO: Update this example when effects system is modernized
// }
