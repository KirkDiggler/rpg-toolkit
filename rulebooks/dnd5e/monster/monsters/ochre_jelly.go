package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
)

// NewOchreJelly creates an Ochre Jelly with its Pseudopod action.
func NewOchreJelly(id string) *monster.Monster {
	m := monster.New(monster.Config{ID: id, Name: "Ochre Jelly"})
	m.AddAction(actions.NewMeleeAction(actions.MeleeConfig{
		Name:        "pseudopod",
		AttackBonus: 4,
		Reach:       1,
		DamageSpec: &damage.DamageSpec{Pools: []damage.Damage{
			{Dice: "2d6", Type: damage.Bludgeoning, FlatBonus: -2, Properties: []damage.Property{damage.PropertyCritEligible}},
			{Dice: "1d6", Type: damage.Acid, Properties: []damage.Property{damage.PropertyCritEligible}},
		}},
	}))
	return m
}
