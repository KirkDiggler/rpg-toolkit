package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// NewGrayOoze creates the first Gray Ooze layer: its core statistics and
// innate standard-condition immunities. Pseudopod and its other traits are
// intentionally added in later focused layers.
func NewGrayOoze(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Gray Ooze",
		Ref:  refs.Monsters.GrayOoze(),
		Size: dnd5e.SizeMedium,
		HP:   22,
		AC:   8,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12,
			abilities.DEX: 6,
			abilities.CON: 16,
			abilities.INT: 1,
			abilities.WIS: 6,
			abilities.CHA: 2,
		},
	})
	m.SetSpeed(monster.SpeedData{Walk: 10, Climb: 10})
	m.AddTraitData(monstertraits.MustConditionImmunityJSON(
		id,
		dnd5eEvents.ConditionBlinded,
		dnd5eEvents.ConditionCharmed,
		dnd5eEvents.ConditionDeafened,
		dnd5eEvents.ConditionExhaustion,
		dnd5eEvents.ConditionFrightened,
		dnd5eEvents.ConditionProne,
	))
	return m
}
