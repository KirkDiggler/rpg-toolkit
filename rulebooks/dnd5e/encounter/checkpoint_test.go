// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// checkpointStore is the store ToCheckpoint is built for (rpg-project#897
// leg c): the rest rewritten whole, the record appended a segment at a time,
// segments wholly below the floor dropped. It round-trips everything through
// JSON, as a store does.
type checkpointStore struct {
	rest     []byte
	segments [][]byte
	since    uint64 // the first seq the store does not hold
	floor    uint64 // the floor of the last save, kept beside the rest
}

func (s *checkpointStore) save(t *testing.T, enc *encounter.Encounter) *encounter.ToCheckpointOutput {
	t.Helper()
	out, err := enc.ToCheckpoint(&encounter.ToCheckpointInput{Since: s.since})
	require.NoError(t, err)
	require.Empty(t, out.Rest.Log.Entries, "the rest carries no entries")
	require.Empty(t, out.Rest.Log.Audiences, "the rest carries no table")
	if len(out.Appended.Entries) > 0 {
		want := max(s.since, out.Floor) // a window may have trimmed past the store
		require.Equal(t, want, out.Appended.Entries[0].Seq, "the segment starts where the store stops")
	}

	s.rest = mustJSON(t, out.Rest)
	s.segments = append(s.segments, mustJSON(t, out.Appended))
	kept := s.segments[:0]
	for _, raw := range s.segments {
		var seg record.LogData
		require.NoError(t, json.Unmarshal(raw, &seg))
		if n := len(seg.Entries); n > 0 && seg.Entries[n-1].Seq < out.Floor {
			continue
		}
		kept = append(kept, raw)
	}
	s.segments = kept
	s.floor = out.Floor
	s.since = out.Rest.Log.NextSeq
	if s.since == 0 {
		s.since = 1
	}
	return out
}

// whole joins what the store holds back into one EncounterData.
func (s *checkpointStore) whole(t *testing.T) encounter.EncounterData {
	t.Helper()
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(s.rest, &data))
	segs := make([]record.LogData, len(s.segments))
	for i, raw := range s.segments {
		require.NoError(t, json.Unmarshal(raw, &segs[i]))
	}
	joined, err := record.JoinData(&record.JoinDataInput{Head: data.Log, Floor: s.floor, Segments: segs})
	require.NoError(t, err)
	data.Log = joined
	return data
}

// asStored is data as a store hands it back: through JSON.
func asStored(t *testing.T, data encounter.EncounterData) encounter.EncounterData {
	t.Helper()
	var back encounter.EncounterData
	require.NoError(t, json.Unmarshal(mustJSON(t, data), &back))
	return back
}

// checkpointCapabilities is what the keep is opened and loaded with: the
// observers' keep row's, sight scaled to the keep.
func checkpointCapabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         &sightList{fallback: 8},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        walksThirty{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        encounter.BehaviorDriver{},
		Roller:        rollsLowest{},
		CheckResolver: findsNothing{},
		Witness:       nobodyPerceives{},
		Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

// checkpointParty is how many of [keepPlayers] stepSome walks; the small
// keep seats at least this many.
const checkpointParty = 2

// loadWrites loads data and returns what the loaded encounter writes.
func loadWrites(t *testing.T, data encounter.EncounterData) encounter.EncounterData {
	t.Helper()
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: asStored(t, data), Capabilities: checkpointCapabilities(),
	})
	require.NoError(t, err)
	return asStored(t, loaded.ToData())
}

// openCheckpointKeep opens the small keep with its party and no garrison:
// the record needs beats, and the party's steps write them.
func openCheckpointKeep(t *testing.T, retention int) *encounter.Encounter {
	t.Helper()
	compiled := smallKeep(t, false)
	members := seatedParty(compiled)
	require.GreaterOrEqual(t, len(members), checkpointParty)
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Options: m.Options,
			Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees, Behaviors: m.Behaviors,
			Machine: m.Machine, BlockingStates: m.BlockingStates, Posts: m.Posts,
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        compiled.Field,
		Members:      members,
		Endings:      append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: checkpointCapabilities(),
		Retention:    retention,
	})
	require.NoError(t, err)
	return enc
}

// stepSome lands up to n seeded single steps by the party.
func stepSome(t *testing.T, enc *encounter.Encounter, rng *rand.Rand, n int) {
	t.Helper()
	for range n {
		who := keepPlayers[rng.IntN(checkpointParty)]
		at := whereIs(t, enc, who)
		d := hexSteps[rng.IntN(len(hexSteps))]
		_, _ = enc.Step(&encounter.StepInput{Member: who, To: spatial.Position{X: at.X + d.X, Y: at.Y + d.Y}, EndWalk: true})
	}
}

// TestCheckpointJoinsBackToToData: a keep saved after every verb as a rest
// plus appended segments joins, after each save, to exactly ToData, and the
// joined data loads to an encounter that writes it again.
func TestCheckpointJoinsBackToToData(t *testing.T) {
	enc := openCheckpointKeep(t, encounter.RetentionUnbounded)
	store := &checkpointStore{since: 1}
	rng := rand.New(rand.NewPCG(897, 3)) //nolint:gosec // a seeded walk, not a secret

	grew := 0
	for save := 0; save < 5; save++ {
		before := store.since
		out := store.save(t, enc)
		require.LessOrEqual(t, out.Floor, uint64(1), "unbounded keeps everything")
		if len(out.Appended.Entries) > 0 {
			grew++
		}
		require.Equal(t, asStored(t, enc.ToData()), store.whole(t), "save %d (since %d): rest + segments is ToData", save, before)
		stepSome(t, enc, rng, 3)
	}
	require.Greater(t, grew, 1, "the verbs appended beats across several saves")
	store.save(t, enc)

	require.Equal(t, loadWrites(t, enc.ToData()), loadWrites(t, store.whole(t)),
		"the joined checkpoint loads to the encounter ToData loads to")
}

func TestCheckpointSinceOneIsTheWholeRecord(t *testing.T) {
	enc := openCheckpointKeep(t, encounter.RetentionUnbounded)
	stepSome(t, enc, rand.New(rand.NewPCG(1, 1)), 4) //nolint:gosec // a seeded walk
	out, err := enc.ToCheckpoint(&encounter.ToCheckpointInput{Since: 1})
	require.NoError(t, err)
	whole := enc.ToData()
	require.NotEmpty(t, whole.Log.Entries)
	require.Equal(t, whole.Log.Entries, out.Appended.Entries)
	require.Equal(t, whole.Log.Audiences, out.Appended.Audiences)
	require.Equal(t, whole.Log.NextSeq, out.Rest.Log.NextSeq)

	rest := out.Rest
	rest.Log = whole.Log
	require.Equal(t, whole, rest, "the rest is ToData but for the log")
}

func TestCheckpointBeatlessVerbAppendsNothing(t *testing.T) {
	enc := openCheckpointKeep(t, encounter.RetentionUnbounded)
	store := &checkpointStore{since: 1}
	store.save(t, enc)
	out := store.save(t, enc)
	require.Empty(t, out.Appended.Entries, "nothing happened between the saves")
	require.Equal(t, asStored(t, enc.ToData()), store.whole(t))
}

// TestCheckpointBoundedReportsTheFloor: under a window smaller than one
// save's beats, Floor is the story's oldest retained seq, the segment starts
// there rather than at the store's last seq + 1, the store drops segments
// wholly below it, and the join at the floor is exactly ToData after every
// save.
func TestCheckpointBoundedReportsTheFloor(t *testing.T) {
	const window = 4
	enc := openCheckpointKeep(t, window)
	store := &checkpointStore{since: 1}
	rng := rand.New(rand.NewPCG(897, 9)) //nolint:gosec // a seeded walk

	jumped := false
	for save := 0; save < 8; save++ {
		stepSome(t, enc, rng, 3)
		before := store.since
		out := store.save(t, enc)
		want := asStored(t, enc.ToData())
		if len(want.Log.Entries) > 0 {
			require.Equal(t, want.Log.Entries[0].Seq, out.Floor, "save %d: the floor is the oldest retained seq", save)
		}
		if out.Floor > before {
			jumped = true
		}
		require.Equal(t, want, store.whole(t), "save %d (since %d, floor %d): rest + segments is ToData", save, before, out.Floor)
	}
	require.True(t, jumped, "a save trimmed past the store's last seq + 1")
	require.Equal(t, loadWrites(t, enc.ToData()), loadWrites(t, store.whole(t)))
}

func TestCheckpointRefusesSincePastTheStory(t *testing.T) {
	enc := openCheckpointKeep(t, encounter.RetentionUnbounded)
	next, err := enc.NextStorySeq()
	require.NoError(t, err)
	before := enc.ToData()

	_, err = enc.ToCheckpoint(&encounter.ToCheckpointInput{Since: next + 1})
	require.ErrorIs(t, err, encounter.ErrBadSince)
	_, err = enc.ToCheckpoint(nil)
	require.ErrorIs(t, err, encounter.ErrNilInput)

	out, err := enc.ToCheckpoint(&encounter.ToCheckpointInput{Since: next})
	require.NoError(t, err, "since at NextSeq is the store holding everything")
	require.Empty(t, out.Appended.Entries)
	require.Equal(t, before, enc.ToData())
}
