package selection_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/selection"
)

type exampleDie struct{}

func (exampleDie) Roll(context.Context, int) (int, error) { return 6, nil }

// Profiles and knowledge are supplied by the caller. The package has no guard,
// personality catalog or dialogue policy of its own; it only chooses values.
func Example_guardAndDialogue() {
	for _, profile := range []struct {
		name   string
		factor int
	}{{"persistent", 3}, {"cautious", 1}} {
		pool, err := selection.Evaluate(&selection.EvaluateInput{Candidates: []selection.Candidate{
			{ID: "search", Weight: 2, Requirements: []selection.Requirement{{ID: "lead", Verdict: selection.Met}}, Factors: []selection.Factor{{ID: profile.name, Multiplier: profile.factor}}},
			{ID: "hold", Weight: 4},
		}})
		if err != nil {
			panic(err)
		}
		picked, err := selection.Draw(context.Background(), &selection.DrawInput{Evaluation: pool, Die: exampleDie{}})
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s guard: %s (face %d of %d)\n", profile.name, picked.Choice.Row.ID, picked.Roll, picked.Total)
	}
	menu, err := selection.Evaluate(&selection.EvaluateInput{Candidates: []selection.Candidate{
		{ID: "ask-purpose"},
		{ID: "repeat-rumor", Requirements: []selection.Requirement{{ID: "heard-rumor", Verdict: selection.Unknown}}},
	}})
	if err != nil {
		panic(err)
	}
	chosen, err := selection.Choose(&selection.ChooseInput{Evaluation: menu, ID: "ask-purpose"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("player: %s (no roll)\n", chosen.Row.ID)
	evidence, err := menu.Inspect()
	if err != nil {
		panic(err)
	}
	// Diagnostic evidence is private until the caller applies disclosure policy.
	fmt.Printf("rumor eligible: %t; unknown: %t\n", evidence.Rows[1].Eligible, evidence.Rows[1].Requirements[0].Verdict == selection.Unknown)
	// Output:
	// persistent guard: search (face 6 of 10)
	// cautious guard: hold (face 6 of 6)
	// player: ask-purpose (no roll)
	// rumor eligible: false; unknown: true
}
