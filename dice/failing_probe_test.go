package dice_test

import "testing"

// TestFailingProbe is the red half of the pipe's proof (rpg-project#910): CI
// must fail through go test -json | tee | test-budget.sh log. Removed in the
// next commit.
func TestFailingProbe(t *testing.T) {
	t.Fatal("probe")
}
