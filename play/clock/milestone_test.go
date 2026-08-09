package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/assert"
)

func TestMilestoneKindVocabularyIsClosed(t *testing.T) {
	kinds := []clock.MilestoneKind{
		clock.TurnStarted, clock.TurnEnded, clock.RoundStarted, clock.Ticked,
		clock.MemberJoined, clock.MemberLeft, clock.Merged, clock.Dissolved,
	}
	assert.Len(t, kinds, 8)
}
