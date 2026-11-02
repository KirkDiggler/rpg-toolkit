// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

// world_test.go is ONE HOST, TWO DICE: the verbs a run is played with and
// the verbs a replay re-issues go through the same methods here, so the only
// thing that differs between a played run and its replay is the die — the
// host's crypto die for the run, the record's own rolls for the replay.
//
// THE HOST RELOADS where a deployed host does (castletest's rule,
// rpg-project#667): before every EndTurn, and after a step that ended a
// world round. The replay reloads on the same rule, so a fact the blob drops
// would show here as a replay that lands another log.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// speakReach is the host's interaction reach, three hexes (the session's,
// rpg-project#753 R4). The reach is the host's constant, not a fact of the
// run, so the record does not carry it and the replay uses the host's own.
const speakReach = 3

// opener builds a run's SetupInput on a die: the same content and the same
// host every time, so a replay opens exactly what the run opened.
type opener func(t testing.TB, h host, down *standing) *encounter.SetupInput

// world is one run of an encounter on one die.
type world struct {
	t    testing.TB
	enc  *encounter.Encounter
	die  dice.Roller
	down *standing
	caps encounter.Capabilities
}

// openWorld opens a run on a die.
func openWorld(t testing.TB, open opener, h host) *world {
	t.Helper()
	down := &standing{down: map[encounter.MemberID]bool{}}
	setup := open(t, h, down)
	enc, err := encounter.NewEncounter(setup)
	require.NoError(t, err)

	return &world{t: t, enc: enc, die: h.die, down: down, caps: setup.Capabilities}
}

// reload saves the encounter the way a host does — ToData, through JSON —
// and plays on from what loads back, on the same die.
func (w *world) reload() error {
	blob, err := json.Marshal(w.enc.ToData())
	if err != nil {
		return err
	}
	var data encounter.EncounterData
	if err := json.Unmarshal(blob, &data); err != nil {
		return err
	}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: w.caps})
	if err != nil {
		return err
	}
	w.enc = enc

	return nil
}

// step is one Step, and a reload when it ended a world round.
func (w *world) step(member encounter.MemberID, to spatial.Position) error {
	round := w.enc.ToData().Clock.HighWater
	if _, err := w.enc.Step(&encounter.StepInput{Member: member, To: to}); err != nil {
		return err
	}
	if w.enc.ToData().Clock.HighWater != round {
		return w.reload()
	}

	return nil
}

// speak is one Speak, the answer picked on the run's die.
func (w *world) speak(in encounter.SpeakInput) error {
	in.Range, in.Roller = speakReach, w.die
	_, err := w.enc.Speak(context.Background(), &in)

	return err
}

// openDoor is one OpenDoor.
func (w *world) openDoor(door encounter.DoorID, actor encounter.MemberID) error {
	_, err := w.enc.OpenDoor(&encounter.OpenDoorInput{Door: door, Actor: actor})

	return err
}

// strike lands one of a player's outcomes, the host first putting down
// whoever it scored a fall for.
func (w *world) strike(outcome encounter.RecordInput, falls []encounter.MemberID) error {
	for _, id := range falls {
		w.down.down[id] = true
	}
	_, err := w.enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &outcome}}})

	return err
}

// endTurn reloads, then ends a player's turn; every driven turn after it
// runs before this returns.
func (w *world) endTurn(member encounter.MemberID) error {
	if err := w.reload(); err != nil {
		return err
	}
	_, err := w.enc.EndTurn(&encounter.EndTurnInput{Member: member})

	return err
}

// at is where a member stands now.
func (w *world) at(id encounter.MemberID) spatial.Position {
	w.t.Helper()
	members, err := w.enc.Members()
	require.NoError(w.t, err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	require.FailNowf(w.t, "not in the run", "%s", id)

	return spatial.Position{}
}

// fighting is whether a member is on a fight's clock.
func (w *world) fighting(id encounter.MemberID) bool {
	w.t.Helper()
	on, err := w.enc.ClockOf(&encounter.ClockOfInput{Member: id})
	require.NoError(w.t, err)

	return on.Kind == encounter.ClockTurn
}

// round is the world clock's high water.
func (w *world) round() int { return w.enc.ToData().Clock.HighWater }

// walkTo walks a member along the pathfinder's own route toward a cell,
// until it stands beside it or a fight takes the walk.
func (w *world) walkTo(member encounter.MemberID, to spatial.Position) {
	w.t.Helper()
	route, err := w.enc.Route(encounter.RouteInput{Mover: member, Policy: encounter.MoveToward, Anchor: to, Budget: 200})
	require.NoError(w.t, err)
	require.Empty(w.t, route.StoppedBy, "%s has a way to %v", member, to)
	for _, cell := range route.Path {
		require.NoError(w.t, w.step(member, cell))
		if w.fighting(member) {
			return
		}
	}
}

// pace walks a member out and back between two adjacent cells until the
// world clock has run some rounds on, or a fight takes the walk.
func (w *world) pace(member encounter.MemberID, a, b spatial.Position, rounds int) {
	w.t.Helper()
	require.Equal(w.t, 1.0, w.enc.Distance(a, b), "pace walks between adjacent cells")
	until := w.round() + rounds
	for guard := 0; w.round() < until && !w.fighting(member); guard++ {
		require.Less(w.t, guard, 50*rounds, "the world clock runs on under a pacing walker")
		to := a
		if w.at(member) == a {
			to = b
		}
		require.NoError(w.t, w.step(member, to))
	}
}
