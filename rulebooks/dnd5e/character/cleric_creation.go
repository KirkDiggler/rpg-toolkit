package character

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// validateLifeEquipment checks the conditional warhammer choice against the
// combined grants finalization uses. Life grants heavy armor, but not martial
// weapons. This creation-only restriction does not prohibit owning a weapon
// acquired from a background or later in play. Other domains remain unmigrated.
func (d *Draft) validateLifeEquipment() error {
	if d.class != classes.Cleric || d.subclass != classes.LifeDomain {
		return nil
	}
	_, weaponProfs, _ := d.compileProficiencies(backgrounds.GetGrants(d.background))
	for _, choice := range d.choices {
		if choice.Source != shared.SourceClass || choice.Category != shared.ChoiceEquipment ||
			choice.ChoiceID != choices.ClericWeapons || !slices.Contains(choice.EquipmentSelection, weapons.Warhammer) {
			continue
		}
		weapon, err := weapons.GetByID(weapons.Warhammer)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(weaponProfs, weapon.CoveredBy) {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "starting warhammer requires weapon proficiency")
		}
	}
	return nil
}
