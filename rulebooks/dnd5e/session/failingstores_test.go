// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// errBroken stands for a store that is down — distinct from a store that
// simply does not hold the key.
var errBroken = errors.New("store unavailable")

// failingSessions fails whichever operations a test arms.
type failingSessions struct {
	*fakeSessions
	getErr  error
	saveErr error
}

func (f *failingSessions) GetSession(ctx context.Context, id string) (*session.SessionData, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.fakeSessions.GetSession(ctx, id)
}

func (f *failingSessions) SaveSession(ctx context.Context, data *session.SessionData) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.fakeSessions.SaveSession(ctx, data)
}

// failingEncounters fails saves on demand.
type failingEncounters struct {
	*fakeEncounters
	saveErr error
}

func (f *failingEncounters) SaveEncounter(
	ctx context.Context, id string, data *encounter.EncounterData,
) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.fakeEncounters.SaveEncounter(ctx, id, data)
}

// authoredWorld is a small valid dungeon floor standing in for content from an
// authoring pipeline: one hall, alice in it, and one ending of its own.
func authoredWorld() scene {
	return scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)}},
		Party: []sceneSeat{seatAt("alice", 1, 1)},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 4, Y: 4},
			}},
		},
	}
}

// fataler is the sliver of *testing.T the older fixtures need.
type fataler interface {
	Fatalf(format string, args ...any)
}
