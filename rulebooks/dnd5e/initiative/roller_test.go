package initiative_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
)

func TestRollForOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRoller := mock_dice.NewMockRoller(ctrl)

	// Since map iteration is non-deterministic, we need to set up expectations
	// that work regardless of iteration order. All entities get the same roll (10)
	// so the order will be determined by modifiers alone
	mockRoller.EXPECT().Roll(20).Return(10, nil).Times(3)

	participants := map[core.Entity]int{
		initiative.NewParticipant("ranger", dnd5e.EntityTypeCharacter): +3, // Total: 13
		initiative.NewParticipant("goblin", dnd5e.EntityTypeMonster):   +2, // Total: 12
		initiative.NewParticipant("wizard", dnd5e.EntityTypeCharacter): +1, // Total: 11
	}

	order := initiative.RollForOrder(participants, mockRoller)

	// Should be sorted by total (all rolled 10, so sorted by modifier)
	assert.Equal(t, 3, len(order))

	// Verify the order is correct based on totals
	assert.Equal(t, 13, order[0].Total, "First should have total of 13")
	assert.Equal(t, 12, order[1].Total, "Second should have total of 12")
	assert.Equal(t, 11, order[2].Total, "Third should have total of 11")

	// Verify entities are in the right order
	assert.Equal(t, "ranger", order[0].Entity.GetID())
	assert.Equal(t, "goblin", order[1].Entity.GetID())
	assert.Equal(t, "wizard", order[2].Entity.GetID())
}
