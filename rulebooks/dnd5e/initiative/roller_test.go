package initiative_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
)

func TestRollForOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRoller := mock_dice.NewMockRoller(ctrl)

	// Set up deterministic rolls
	// Ranger rolls 15 + 3 = 18
	// Goblin rolls 10 + 2 = 12
	// Wizard rolls 8 + 1 = 9
	mockRoller.EXPECT().Roll(20).Return(15, nil) // Ranger
	mockRoller.EXPECT().Roll(20).Return(10, nil) // Goblin
	mockRoller.EXPECT().Roll(20).Return(8, nil)  // Wizard

	participants := map[core.Entity]int{
		initiative.NewParticipant("ranger", "character"): +3,
		initiative.NewParticipant("goblin", "monster"):   +2,
		initiative.NewParticipant("wizard", "character"): +1,
	}

	order := initiative.RollForOrder(participants, mockRoller)

	// Should be sorted by total: ranger (18), goblin (12), wizard (9)
	assert.Equal(t, 3, len(order))
	assert.Equal(t, "ranger", order[0].Entity.GetID())
	assert.Equal(t, "goblin", order[1].Entity.GetID())
	assert.Equal(t, "wizard", order[2].Entity.GetID())
}
