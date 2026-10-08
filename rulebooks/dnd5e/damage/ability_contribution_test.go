package damage_test

import (
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/stretchr/testify/suite"
)

type AbilityModifierInformationSuite struct{ suite.Suite }

func TestAbilityModifierInformationSuite(t *testing.T) {
	suite.Run(t, new(AbilityModifierInformationSuite))
}

func (s *AbilityModifierInformationSuite) TestNormalAndOffHandBaseContributions() {
	for _, modifier := range []int{-3, 0, 3} {
		for _, offHand := range []bool{false, true} {
			s.Run(fmt.Sprintf("modifier=%d,offhand=%t", modifier, offHand), func() {
				included := damage.IncludesAbilityModifier(damage.AbilityModifierInput{Modifier: modifier, OffHand: offHand})
				if offHand && modifier >= 0 {
					s.False(included, "a later fighting-style contribution is not part of the base")
				} else {
					s.True(included, "zero on a normal attack and negative off-hand modifiers participate")
				}
			})
		}
	}
}
