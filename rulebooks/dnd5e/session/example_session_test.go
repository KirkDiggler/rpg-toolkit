// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Example_theSession is the acceptance scenes told as one continuous story.
//
// It is the wave's headline claim, made executable: a host implements two
// repositories, and from then on holds no domain object at all — it names
// things and calls verbs. Every line of the Output block below is re-proved by
// go test, so this narration cannot drift from what the SDK actually returns.
//
// What to watch for, in order:
//
//   - the party enters an authored world nobody had to hydrate by hand;
//   - a walk reports the cells it ACTUALLY entered, not the ones requested;
//   - an ending fires underfoot, the walk stops there, and the remaining steps
//     never happen — visible as a Steps count shorter than the path;
//   - a closed encounter refuses every change and answers every read.
func Example_theSession() {
	ctx := context.Background()

	// The host's entire integration: two stores.
	mgr, err := session.NewManager(&session.Config{Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions:   newFakeSessions(),
		Encounters: newFakeEncounters(), Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	if err != nil {
		panic(err)
	}

	// -- the party enters the tomb --
	if _, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "tomb-run", Encounter: "tomb", World: authoredTomb(),
	}); err != nil {
		panic(err)
	}
	fmt.Println("-- the party enters --")

	if _, err := mgr.Join(ctx, &session.JoinInput{
		Session: "tomb-run", Member: "bob",
		Position: spatial.Position{X: 1, Y: 3},
	}); err != nil {
		panic(err)
	}

	atlas, err := mgr.Atlas(ctx, &session.AtlasInput{Session: "tomb-run"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("  the map is %d cells on a %s grid\n", len(atlas.Cells), atlas.Grid)

	// -- alice walks, and gets exactly as far as the world allows --
	fmt.Println("-- alice walks toward the stairs --")
	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "tomb-run", Member: "alice",
		Path: []spatial.Position{
			{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}, // the stairs
			{X: 5, Y: 1}, {X: 6, Y: 1}, // never walked
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("  asked for %d cells, walked %d\n", 5, len(out.Steps))
	for _, step := range out.Steps {
		fmt.Printf("    alice enters (%v,%v)\n", step.Position.X, step.Position.Y)
	}
	if out.Outcome != nil {
		fmt.Printf("  the encounter ends underfoot: %q\n", out.Outcome.Ending)
	}

	// -- a finished encounter is a record --
	fmt.Println("-- afterwards --")
	_, err = mgr.Move(ctx, &session.MoveInput{
		Session: "tomb-run", Member: "bob", Path: []spatial.Position{{X: 1, Y: 2}},
	})
	fmt.Printf("  bob tries to move: %v\n", err != nil)

	status, err := mgr.Status(ctx, &session.StatusInput{Session: "tomb-run"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("  but the story still reads: open=%v, ended by %q\n",
		status.Open, status.Outcome.Ending)

	final := make([]string, 0, len(status.Outcome.Members))
	for _, m := range status.Outcome.Members {
		final = append(final, fmt.Sprintf("%s at (%v,%v)", m.ID, m.Position.X, m.Position.Y))
	}
	sort.Strings(final)
	for _, line := range final {
		fmt.Printf("    %s\n", line)
	}

	// Output:
	// -- the party enters --
	//   the map is 64 cells on a square grid
	// -- alice walks toward the stairs --
	//   asked for 5 cells, walked 3
	//     alice enters (2,1)
	//     alice enters (3,1)
	//     alice enters (4,1)
	//   the encounter ends underfoot: "stairs"
	// -- afterwards --
	//   bob tries to move: true
	//   but the story still reads: open=false, ended by "stairs"
	//     alice at (4,1)
	//     bob at (1,3)
}

// Example_theFightThatStartsItself is the host's whole "a fight broke out"
// loop, and it is short on purpose: there is no loop.
//
// A verb returns having done less than it was asked and says why in the same
// response. The host renders it. There is no second call, no window to answer,
// and — the point — no decision the host has to make about what a sighting
// means. It used to be a suspension loop, back when THIS package decided that
// seeing something stops a walk; the composition decides now
// (rpg-toolkit#964), so what reaches the host is news rather than a question.
func Example_theFightThatStartsItself() {
	ctx := context.Background()
	mgr, err := session.NewManager(&session.Config{Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	if err != nil {
		panic(err)
	}
	if _, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "run", Encounter: "tomb", World: ambushWorld(panicFataler{}),
	}); err != nil {
		panic(err)
	}

	path := []spatial.Position{{X: 2, Y: 1}, {X: 2, Y: 2}, {X: 2, Y: 3}, {X: 2, Y: 4}}
	out, err := mgr.Move(ctx, &session.MoveInput{Session: "run", Member: "alice", Path: path})
	if err != nil {
		panic(err)
	}

	if out.Formed == nil {
		fmt.Printf("walked all %d cells\n", len(out.Steps))
		return
	}
	fmt.Printf("stopped after %d/%d: a fight starts, in order %v\n",
		len(out.Steps), len(path), out.Formed.Order)

	// She is not free to WANDER — Transfer to the world clock is still refused
	// while a bubble holds her — but it is her own turn (she is the fight's
	// first name in initiative), so Move still lets her walk: on the turn
	// clock a walk spends movement rather than being refused outright
	// (rpg-toolkit#1169). This cell costs 5 of the feet her speed banked at
	// the top of the fight.
	out, err = mgr.Move(ctx, &session.MoveInput{
		Session: "run", Member: "alice", Path: []spatial.Position{{X: 2, Y: 3}},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("she can still walk on her own turn: %d step(s)\n", len(out.Steps))

	// Output:
	// stopped after 2/4: a fight starts, in order [alice ogre]
	// she can still walk on her own turn: 1 step(s)
}

// panicFataler adapts the test fixtures for use from an Example, which has no
// *testing.T to fail through.
type panicFataler struct{}

func (panicFataler) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

// authoredTomb is content, not a live encounter: the blob a host would have
// sitting in storage from an authoring pipeline.
func authoredTomb() *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 4, Y: 1},
			}},
		},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		panic(err)
	}
	data := enc.ToData()
	return &data
}
