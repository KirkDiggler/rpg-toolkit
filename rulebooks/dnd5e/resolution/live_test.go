// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// lend loads the suite's world the way a host holds it, and turns in into the
// same interaction over that instance instead of its data.
func (s *MovementTestSuite) lend(in *Input) *encounter.Encounter {
	live, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: in.World, Capabilities: in.Capabilities,
	})
	s.Require().NoError(err)
	in.World = encounter.EncounterData{}
	in.Live = live
	return live
}

// A LENT ENCOUNTER RESOLVES EXACTLY AS ITS DATA DOES (rpg-project#753, T12):
// the same step, the same cast, the same scripted dice, once over the data
// this package loads and once over an instance the host already holds. The
// outcome, the dirty sheets and the hooks are the same; only Output.World
// differs, empty for the lent one because nothing here wrote to it.
func (s *MovementTestSuite) TestALentEncounterResolvesAsItsDataDoes() {
	var lentBefore string
	// ONE WORLD FOR BOTH RUNS: a world built twice mints its log ids twice,
	// and the comparison below is of what Resolve did to the same data.
	world := s.world()
	run := func(lent bool) (*Output, *encounter.Encounter) {
		step := s.stepInput()
		step.Roller = fixedRoller{face: 6}
		machine, err := NewMovement(step)
		s.Require().NoError(err)
		in := s.input(machine, heroID)
		in.World = world
		in.Roller = fixedRoller{face: 6}
		var live *encounter.Encounter
		if lent {
			live = s.lend(in)
			lentBefore = jsonOf(s, live.ToData())
		}
		out, err := resolveOn(s.ctx, in, newSurface(events.NewEventBus()))
		s.Require().NoError(err)
		return out, live
	}

	loaded, _ := run(false)
	lentOut, live := run(true)

	moved, ok := lentOut.Outcome.(MovementOutcome)
	s.Require().True(ok)
	s.Require().Len(moved.Reactions, 1, "the lent world still provokes: sides and reach were read off it")

	s.Equal(loaded.Outcome, lentOut.Outcome)
	s.Equal(loaded.Hooks, lentOut.Hooks)
	s.Equal(sheetsOf(s, loaded), sheetsOf(s, lentOut))
	s.Equal(loaded.OpenedAreas, lentOut.OpenedAreas)
	s.Equal(loaded.ClosedAreas, lentOut.ClosedAreas)

	s.Equal(jsonOf(s, encounter.EncounterData{}), jsonOf(s, lentOut.World),
		"a lent encounter comes back as nothing to store")
	s.Equal(lentBefore, jsonOf(s, live.ToData()),
		"RESOLUTION NEVER WRITES THE WORLD: the lent instance's data is unchanged across Resolve")
	s.Equal(jsonOf(s, loaded.World), jsonOf(s, live.ToData()),
		"and it is the world the loaded run handed back")
}

// ONE WORLD: a lent encounter beside world data is refused before anything
// runs, rather than this package choosing which one is true.
func (s *MovementTestSuite) TestALentEncounterBesideWorldDataIsRefused() {
	machine, err := NewMovement(s.stepInput())
	s.Require().NoError(err)
	in := s.input(machine)
	world := in.World
	s.lend(in)
	in.World = world

	out, err := Resolve(s.ctx, in)
	s.Nil(out)
	s.Require().Error(err)
	s.True(errors.Is(err, ErrBadWorld), "got %v", err)
}

// NO WORLD AT ALL is refused too, before anything runs: an interaction
// happens somewhere.
func (s *MovementTestSuite) TestNeitherALentWorldNorWorldDataIsRefused() {
	machine, err := NewMovement(s.stepInput())
	s.Require().NoError(err)
	in := s.input(machine)
	in.World = encounter.EncounterData{}

	out, err := Resolve(s.ctx, in)
	s.Nil(out)
	s.Require().ErrorIs(err, ErrBadWorld)
}

// A NIL ENCOUNTER IS NO WORLD: a typed-nil *encounter.Encounter lent as Live
// is a non-nil interface, and it is refused like no world at all rather than
// panicking on its first read.
func (s *MovementTestSuite) TestANilLentEncounterIsRefusedAsNoWorld() {
	machine, err := NewMovement(s.stepInput())
	s.Require().NoError(err)
	in := s.input(machine)
	in.World = encounter.EncounterData{}
	var none *encounter.Encounter
	in.Live = none

	out, err := Resolve(s.ctx, in)
	s.Nil(out)
	s.Require().ErrorIs(err, ErrBadWorld)
}

func jsonOf(s *MovementTestSuite, v any) string {
	raw, err := json.Marshal(v)
	s.Require().NoError(err)
	return string(raw)
}

// sheetsOf is an output's dirty sheets as JSON with the write clock cleared:
// two runs a few milliseconds apart stamp different update times and nothing
// else.
func sheetsOf(s *MovementTestSuite, out *Output) string {
	for _, sheet := range out.DirtyCharacters {
		sheet.UpdatedAt = time.Time{}
	}
	return jsonOf(s, out.DirtyCharacters)
}
