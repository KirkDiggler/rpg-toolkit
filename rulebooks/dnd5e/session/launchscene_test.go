// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// launchscene_test.go is how this package's suites start a run since Launch
// became the only host verb that does (rpg-project#542, R7). A suite that used
// to say "start a session in this world, join these players, spawn these
// monsters" now says one scene and launches it: the same board a host's
// compiled dungeon would give, with nobody placed by a second verb.

// testSession is the run id a scene launches under unless it names another.
const testSession = "sess"

// sceneSeat is one party member and the authored cell they start on.
type sceneSeat struct {
	ID string
	// At is an authored offset [col,row], as [dungeonspec.Seat.At] is.
	At spatial.Position
}

// scene is a run in a form a suite can write down: the floor, the party
// already saved in the fake repository, the monsters on the board, and the
// endings the dungeon declares beside the ones every launch declares.
type scene struct {
	// Session is the run's id; empty means [testSession].
	Session string
	Field   encounter.FieldInput
	// Party are characters already saved in the fake repository, in seat order.
	Party    []sceneSeat
	Monsters []dungeonspec.MonsterPlacement
	// Endings are the dungeon's own. A launch declares [session.EndingWithdrawn]
	// (and [session.EndingBossDown]) itself, so a scene that lists it is read
	// as having listed it once.
	Endings []encounter.EndingInput
}

// launchScene compiles nothing: it builds the dungeonspec.Compiled literal
// (Field, PartyStart from Party, Monsters, Endings) and calls Launch.
func launchScene(t *testing.T, m *session.Manager, sc scene) *session.LaunchOutput {
	t.Helper()
	out, err := m.Launch(context.Background(), sceneInput(sc))
	require.NoError(t, err, "launching the scene")
	return out
}

// sceneInput is the LaunchInput a scene stands for, for a test whose subject
// is the launch itself failing.
func sceneInput(sc scene) *session.LaunchInput {
	id := sc.Session
	if id == "" {
		id = testSession
	}
	seats := make([]dungeonspec.Seat, 0, len(sc.Party))
	party := make([]string, 0, len(sc.Party))
	for _, seat := range sc.Party {
		seats = append(seats, dungeonspec.Seat{At: seat.At})
		party = append(party, seat.ID)
	}
	// A launch declares the party withdrawing itself; a scene that wrote it
	// down too is not declaring it twice.
	endings := make([]encounter.EndingInput, 0, len(sc.Endings))
	for _, ending := range sc.Endings {
		if ending.Key != session.EndingWithdrawn {
			endings = append(endings, ending)
		}
	}
	return &session.LaunchInput{
		Session: id,
		Dungeon: &dungeonspec.Compiled{
			Field:      sc.Field,
			PartyStart: seats,
			Monsters:   sc.Monsters,
			Endings:    endings,
		},
		Party: party,
	}
}

// monsterAt is a placement of a monster ref on an authored cell, member id and
// all, the way the compile mints them.
func monsterAt(id, ref string, col, row int) dungeonspec.MonsterPlacement {
	return dungeonspec.MonsterPlacement{
		Ref: ref, ID: id, MemberID: id, At: spatial.Position{X: float64(col), Y: float64(row)},
	}
}

// seatAt is a party seat on an authored cell.
func seatAt(id string, col, row int) sceneSeat {
	return sceneSeat{ID: id, At: spatial.Position{X: float64(col), Y: float64(row)}}
}

// authoredOf is the authored offset [col,row] of a pointy-top axial cell: the
// frame verbs speak, turned back into the frame a scene is written in.
func authoredOf(axial spatial.Position) spatial.Position {
	r := int(axial.Y)
	return spatial.Position{X: axial.X + float64((r-(r&1))/2), Y: axial.Y}
}

// launchOnClock launches the scene, then moves the named members from the
// world clock into one authored turn clock on the stored run.
//
// A fight forms from contact, and the scenes that use this are not about how
// one forms: they want the swing, the cast or the walk of a turn that is
// already someone's. So the clock is authored directly on the stored world,
// exactly as these suites always did, so initiative never consumes the dice
// whose exact faces they assert. The board itself — who stands where, with
// which sheet — is the launch's own.
func launchOnClock(
	t *testing.T, m *session.Manager, stored *fakeEncounters, sc scene, order []string, active int,
) *session.LaunchOutput {
	t.Helper()
	out := launchScene(t, m, sc)
	authorTurnClock(t, stored, sceneInput(sc).Session, order, active)
	return out
}

// authorTurnClock replaces the stored run's clock for the named members with
// one authored turn clock whose active member is order[active].
func authorTurnClock(t *testing.T, stored *fakeEncounters, run string, order []string, active int) {
	t.Helper()
	data, ok := stored.byID[run]
	require.True(t, ok, "no stored run %q to author a clock on", run)
	ids := make([]core.EntityID, 0, len(order))
	for _, member := range order {
		delete(data.Clock.Budgets, core.EntityID(member))
		ids = append(ids, core.EntityID(member))
	}
	raw, err := json.Marshal([]struct {
		Order     []core.EntityID `json:"order"`
		ActiveIdx int             `json:"active_idx"`
		Round     int             `json:"round"`
	}{{Order: ids, ActiveIdx: active, Round: 1}})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &data.Bubbles))
}

// seedRun writes a run straight into the fake stores, for the few scenes whose
// subject cannot be reached through Launch because it IS a stored shape a
// launch never produces: a bounded retention, a hand-edited log, a record an
// older writer left. Every use names why, beside the call.
func seedRun(t *testing.T, sessions *fakeSessions, encounters *fakeEncounters, run string, world *encounter.EncounterData) {
	t.Helper()
	require.NoError(t, encounters.SaveEncounter(context.Background(), run, world))
	require.NoError(t, sessions.SaveSession(context.Background(), &session.SessionData{ID: run, Encounter: run}))
}
