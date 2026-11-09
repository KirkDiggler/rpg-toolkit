package dice_test

import (
	"testing"
	"time"
)

// TestBudgetProofSleepsSixSeconds is the red half of the CI test budget's
// proof (rpg-project#910). It is reverted in the next commit.
func TestBudgetProofSleepsSixSeconds(t *testing.T) {
	time.Sleep(6 * time.Second)
}
