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
	mgr, err := session.NewManager(&session.Config{
		Sessions:   newFakeSessions(),
		Encounters: newFakeEncounters(),
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
		Session: "tomb-run", Member: "bob", Kind: session.KindPlayer,
		Room: "hall", Position: spatial.Position{X: 1, Y: 3},
	}); err != nil {
		panic(err)
	}

	atlas, err := mgr.Atlas(ctx, &session.AtlasInput{Session: "tomb-run"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("  the hall is %dx%d on a %s grid\n",
		atlas.Rooms[0].Width, atlas.Rooms[0].Height, atlas.Rooms[0].Grid)

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
	//   the hall is 8x8 on a square grid
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

// Example_thePending shows the shape a suspension will take from the host's
// side, using the only pause that exists today — an ending.
//
// The point is the CALLER'S code, not the mechanism. A verb comes back having
// done less than it was asked, says so in its own return value, and the host
// reacts. When resolutions become genuinely suspendable, the caller learns
// about it through the same channel: a verb that returns having stopped, plus a
// field saying who owes an answer. Nothing about this loop changes shape.
func Example_thePending() {
	ctx := context.Background()
	mgr, err := session.NewManager(&session.Config{
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
	})
	if err != nil {
		panic(err)
	}
	if _, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "run", Encounter: "tomb", World: authoredTomb(),
	}); err != nil {
		panic(err)
	}

	path := []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}, {X: 5, Y: 1}}
	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "run", Member: "alice", Path: path,
	})
	if err != nil {
		panic(err)
	}

	switch {
	case out.Outcome != nil:
		fmt.Printf("stopped after %d/%d: the encounter ended (%s)\n",
			len(out.Steps), len(path), out.Outcome.Ending)
	case len(out.Steps) < len(path):
		fmt.Printf("stopped after %d/%d: something interrupted\n", len(out.Steps), len(path))
	default:
		fmt.Printf("walked all %d cells\n", len(out.Steps))
	}

	// Output:
	// stopped after 3/4: the encounter ended (stairs)
}

// authoredTomb is content, not a live encounter: the blob a host would have
// sitting in storage from an authoring pipeline.
func authoredTomb() *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Room: "hall", Position: spatial.Position{X: 4, Y: 1},
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
