// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// conceal_test.go is the concealment capabilities, CARRIED AND NEVER ASKED
// (rpg-toolkit#1378).
//
// The two are not the sixth and seventh entries in Validate's list, and the
// difference is the subject here. Standing, Sight and the rest are required
// on every input because every world needs them; CheckResolver and Witness
// are required exactly when the blob carries concealed structure, and only
// the composition's load door can read the blob to say so. So this package
// carries them without a gate of its own in either direction — a plain world
// loads with both nil, a concealed world is refused by encounter's own
// sentinels — and these tests pin each half separately.

// neverResolves and neverWitnesses satisfy Setup's capability requirement
// while the concealed FIXTURE world is authored. Nothing searches during
// authoring, so a consult during the build is the fixture's own bug — and it
// says so, the way noAttacksExpected does.
type neverResolves struct{}

func (neverResolves) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, errors.New("resolution fixtures: no fixture build resolves a check")
}

type neverWitnesses struct{}

func (neverWitnesses) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return nil, errors.New("resolution fixtures: no fixture build asks who perceives")
}

// concealedWorld is the smallest field that makes the two capabilities
// required: a visible hall and a concealed vault beside it. No door and no
// walls, because fieldHasConcealment is the composition's question and one
// concealed region already answers it — what this fixture owes the tests
// below is a blob whose load door demands the capabilities, nothing more.
func concealedWorld(t *testing.T) encounter.EncounterData {
	t.Helper()

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{},
		Sight:         everyoneSeesTheWholeMap{},
		CheckResolver: neverResolves{},
		Witness:       neverWitnesses{},
		Field: encounter.FieldInput{
			Canvas: hexCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("hall", 0, 0, 6, 6),
				concealRegion(rectRegion("vault", 6, 0, 6, 6)),
			},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	return enc.ToData()
}

// concealRegion marks an authored region as hidden space.
func concealRegion(r encounter.RegionInput) encounter.RegionInput {
	r.Concealed = true
	return r
}

// countingCheckResolver refuses like neverResolves and remembers being asked.
type countingCheckResolver struct{ asks int }

func (c *countingCheckResolver) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	c.asks++

	return nil, errors.New("resolution fixtures: no scene here searches")
}

// countingWitness refuses like neverWitnesses and remembers being asked.
type countingWitness struct{ asks int }

func (c *countingWitness) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	c.asks++

	return nil, errors.New("resolution fixtures: no scene here perceives a door")
}

// TestTheConcealmentCapabilitiesAreCarriedAndNeverAsked is
// TestTheStandingCapability's twin for the pair that arrived with
// rpg-toolkit#1371, and it pins the same two halves.
//
// CARRIED: a concealed field refuses to load without both capabilities, so a
// Resolve that succeeds at all is proof that WHAT THIS INPUT WAS HANDED
// reached LoadEncounter — drop either pass-through and this fails at the
// door rather than in an assertion. That identity is the whole of
// rpg-toolkit#1378's first contract point: before it, fight formation
// reloaded a concealed world without them and every fight on a concealed
// dungeon failed closed at exactly that door.
//
// NEVER ASKED: the composition consults a CheckResolver in Search and a
// Witness where doors open, and this package calls no verbs — it loads a
// world, runs one interaction, reads the world back out. The zeros are the
// assertion rather than an accident, because a later change that started
// consulting either would be this package answering a rules question it
// holds no opinion on.
func TestTheConcealmentCapabilitiesAreCarriedAndNeverAsked(t *testing.T) {
	resolver := &countingCheckResolver{}
	witness := &countingWitness{}

	probe := &worldProbe{}
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight:         everyoneSeesTheWholeMap{},
		Roller:        dice.NewRoller(),
		CheckResolver: resolver,
		Witness:       witness,
		World:         concealedWorld(t),
		Participants:  []Participant{{Character: probeSheet(heroID)}},
		Machine:       probe,
	})

	require.NoError(t, err, "the concealed world loads, which it cannot do without both capabilities")
	require.NotNil(t, out)
	require.Zero(t, resolver.asks, "this package never resolves a find check")
	require.Zero(t, witness.asks, "and never asks who perceives a door")
}

// TestAConcealedWorldIsRefusedAtEncountersOwnDoor pins that this package
// adds NO SOFTENING LAYER: absence travels down as the nil it is, and what
// comes back is the composition's own sentinel wrapped by the load-world
// prefix — reachable by errors.Is, exactly as the session's tripwire found
// it (rpg-toolkit#1377). A default here, or a friendlier refusal of this
// package's own, would each be resolution deciding a question about
// concealment it holds none of.
//
// The third subtest is the other half of the same rule, and it is the half
// Validate's silence buys: nil is the TRUTHFUL zero for a world that
// conceals nothing, so a plain world resolves without either capability —
// which is every other scene in this suite, pinned here once by name.
func TestAConcealedWorldIsRefusedAtEncountersOwnDoor(t *testing.T) {
	input := func(world encounter.EncounterData) *Input {
		return &Input{
			Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
			Sight:        everyoneSeesTheWholeMap{},
			Roller:       dice.NewRoller(),
			World:        world,
			Participants: []Participant{{Character: probeSheet(heroID)}},
			Machine:      &worldProbe{},
		}
	}

	t.Run("no check resolver", func(t *testing.T) {
		in := input(concealedWorld(t))

		_, err := Resolve(context.Background(), in)
		require.ErrorIs(t, err, encounter.ErrNoCheckResolver)
	})

	t.Run("no witness", func(t *testing.T) {
		in := input(concealedWorld(t))
		in.CheckResolver = &countingCheckResolver{}

		_, err := Resolve(context.Background(), in)
		require.ErrorIs(t, err, encounter.ErrNoWitness)
	})

	t.Run("a plain world needs neither", func(t *testing.T) {
		in := input(walledWorld(t))

		_, err := Resolve(context.Background(), in)
		require.NoError(t, err)
	})
}
