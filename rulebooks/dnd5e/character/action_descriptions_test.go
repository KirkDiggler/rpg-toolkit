package character

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// TestAvailableAbilitiesCarryDescriptions pins R12 for the rows this module
// owns: an ability's prose comes from the combat ability or feature itself, and
// a spent ability keeps it, because a card explains what it would do.
func (s *ActionEconomyTestSuite) TestAvailableAbilitiesCarryDescriptions() {
	char := createTestFighterCharacter(s.T(), s.bus)
	_, err := char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	want := combatabilities.NewDodge("dodge").Description()
	s.Require().NotEmpty(want)

	find := func(id string) AvailableAbility {
		for _, ability := range char.AvailableAbilities() {
			if ability.Ref != nil && ability.Ref.ID == id {
				return ability
			}
		}
		s.FailNow("ability not offered", id)
		return AvailableAbility{}
	}

	s.True(find(refs.CombatAbilities.Dodge().ID).CanUse)
	s.Equal(want, find(refs.CombatAbilities.Dodge().ID).Description)

	char.SpendSlots(coreCombat.ActionStandard, 1)
	spent := find(refs.CombatAbilities.Dodge().ID)
	s.False(spent.CanUse)
	s.Equal(want, spent.Description, "a spent ability keeps its text")

	secondWind := find(refs.Features.SecondWind().ID)
	s.NotEmpty(secondWind.Description, "a feature's row carries the feature's prose")
}
