// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestPartitions_DerivedOnce pins the field's partitions as DERIVED WHEN
// ASKED (rpg-project#762 T13; partitions.go's [partitionMemo]): a load runs
// no partition pass, plain steps and a door change ask none, the first sound
// derives them once, and nothing afterwards derives them again.
//
// THE DOOR CHANGE DERIVES NOTHING, by the division's own contract: every door
// is closed to the partition pass whatever its state ([PartitionFloor]), so
// opening one cannot change a partition and invalidating on it would be a
// second pass that answers the same. What the open door changes is the hop a
// sound pays to cross it, read live by [Encounter.hopsFrom].
func TestPartitions_DerivedOnce(t *testing.T) {
	const door DoorID = "derived-once-door"
	caps := func() Capabilities {
		return Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     UnobservedEquipment{},
			Conditions:    UnobservedEquipment{},
			Sheets:        sheetFacts{"talker": {SpeedFeet: 30}, "listener": {SpeedFeet: 30, PassivePerception: 30}},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		}
	}
	base, err := NewEncounter(&SetupInput{
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("derived-once", 0, 0, 10, 10)},
			Doors: []DoorInput{{
				ID:    door,
				Edges: []DoorEdge{{From: spatial.Position{X: 0, Y: 3}, To: spatial.Position{X: 1, Y: 3}}},
				State: DoorIsClosed(),
			}},
		},
		Members: []MemberInput{
			{ID: "talker", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 3}},
			{ID: "listener", Kind: KindPlayer, Position: spatial.Position{X: 8, Y: 8}},
		},
		Endings:      []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: caps(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, base.field.partitions.derivations, "Setup derives at the door that builds the place")

	enc, err := LoadEncounter(&LoadEncounterInput{Data: base.ToData(), Capabilities: caps()})
	require.NoError(t, err)
	derivations := func() int { return enc.field.partitions.derivations }
	require.Zero(t, derivations(), "a load derives nothing")

	// N plain steps: no sound, no door.
	start := spatial.Position{X: 0, Y: 3}
	beside := spatial.Position{X: 0, Y: 4}
	for i := 0; i < 4; i++ {
		_, err := enc.Step(&StepInput{Member: "talker", To: beside})
		require.NoError(t, err)
		_, err = enc.Step(&StepInput{Member: "talker", To: start, EndWalk: true})
		require.NoError(t, err)
	}
	require.Zero(t, derivations(), "plain steps that make no sound and open no door derive nothing")

	_, err = enc.OpenDoor(&OpenDoorInput{Door: door, Actor: "talker"})
	require.NoError(t, err)
	require.Zero(t, derivations(), "a door change cannot change a partition, so it derives nothing")

	require.NoError(t, enc.makeSound(noiseSay, "talker", 0))
	require.Equal(t, 1, derivations(), "a sound derives once")

	require.NoError(t, enc.makeSound(noiseSay, "talker", 0))
	_, err = enc.Step(&StepInput{Member: "talker", To: beside, EndWalk: true})
	require.NoError(t, err)
	require.Equal(t, 1, derivations(), "and never again for the instance's life")

	// The lazily derived division answers exactly what Setup's eager one does.
	for _, cell := range []spatial.Position{start, beside, {X: 8, Y: 8}, {X: 20, Y: 20}} {
		want, wantOK, err := base.field.partitionAt(cell)
		require.NoError(t, err)
		got, gotOK, err := enc.field.partitionAt(cell)
		require.NoError(t, err)
		require.Equal(t, wantOK, gotOK, "owned at %v", cell)
		require.Equal(t, want, got, "partition at %v", cell)
	}
}
