// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// startfacing_test.go is the seam's half of the start point's optional
// direction (rpg-project#374). Kirk, walking a dungeon: "we always start
// looking the wrong way."
//
// The composition decides everything; this package copies one more fact
// across. What is pinned HERE is the copying — because that is the one thing
// only this layer can get wrong, and it gets it wrong SILENTLY: the seam's
// Atlas is its own struct, so a composition field nobody projected simply
// never reaches a client, with every test still green. The module's own
// completeness audit is what catches that, and it did: pinning the new
// encounter tag failed TestEveryInnerFieldIsCarriedOrJustified before a line
// of this file existed.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type StartFacingSuite struct {
	suite.Suite
	mgr *session.Manager
}

func TestStartFacingSuite(t *testing.T) { suite.Run(t, new(StartFacingSuite)) }

// startWorld is the plain hall with a start declared however the caller says
// — nil for a dungeon that declares none, which is every dungeon stored
// before this field existed.
func startWorld(t fataler, start *encounter.FieldStart) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 6)},
			Start:   start,
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: cell(1, 1)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: cell(2, 1)},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		t.Fatalf("building the start world: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *StartFacingSuite) start(world *encounter.EncounterData) {
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world})
	s.Require().NoError(err)
}

func (s *StartFacingSuite) atlasOf(member string) *session.Atlas {
	s.T().Helper()
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{
		Session: "sess", Member: member})
	s.Require().NoError(err)
	return atlas
}

// TestAnAuthoredStartReachesEveryMembersMap is the mirror's whole claim: the
// fact the composition carries arrives here, in the same frame, for everybody.
func (s *StartFacingSuite) TestAnAuthoredStartReachesEveryMembersMap() {
	s.start(startWorld(s.T(), &encounter.FieldStart{At: cell(1, 1), Facing: "e"}))

	want := &session.AtlasStart{At: hexCell(1, 1), Facing: "e"}
	for _, member := range []string{"alice", "bob"} {
		atlas := s.atlasOf(member)
		s.Require().NotNil(atlas.Start, "%s's map says where the dungeon begins", member)
		s.Equal(want, atlas.Start,
			"a way in is structure: nothing about it varies by who is asking")
	}

	s.Run("the unscoped read carries it too", func() {
		authored, err := s.mgr.AtlasOf(context.Background(), &session.AtlasOfInput{
			World: startWorld(s.T(), &encounter.FieldStart{At: cell(1, 1), Facing: "e"})})
		s.Require().NoError(err)
		s.Require().NotNil(authored.Start)
		s.Equal(want, authored.Start)
	})
}

// TestABareStartCarriesNoFacing is the bare-pair case at the seam. The
// authoring dialect lets a dungeon give a cell and say nothing about
// direction, and that silence has to survive all the way here — a client that
// received "n" for a dungeon whose author never chose one would open the
// camera on a decision nobody made.
func (s *StartFacingSuite) TestABareStartCarriesNoFacing() {
	s.start(startWorld(s.T(), &encounter.FieldStart{At: cell(1, 1)}))

	atlas := s.atlasOf("alice")
	s.Require().NotNil(atlas.Start, "the cell is authored — only the direction was not")
	s.Equal(hexCell(1, 1), atlas.Start.At)
	s.Empty(atlas.Start.Facing, "open the camera however it opened before")
}

// TestAWorldFromBeforeStartsExistedProjectsNone is the compatibility case, and
// the reason Start is a pointer. Every encounter stored before this field
// existed declares none, and must arrive as "nobody said" rather than as a
// party standing at the origin looking nowhere — which is a real dungeon
// somebody could author.
func (s *StartFacingSuite) TestAWorldFromBeforeStartsExistedProjectsNone() {
	s.start(startWorld(s.T(), nil))

	s.Nil(s.atlasOf("alice").Start, "this world declares no way in, and says so by saying nothing")
	s.Nil(s.atlasOf("bob").Start)
}

// TestTheThreeCasesAreDistinguishable is the contract a host reads to decide
// whether to send the wire message at all (rpg-api-protos#292), and it is a
// claim no single scene above makes: that the three answers differ from each
// other, not merely that each is what it is.
//
//	nil                     nobody authored a way in
//	{cell, ""}              a cell, and no direction stated
//	{cell, "e"}             a cell and a direction
//
// The middle case is the one a collapse would eat. A pointer is what keeps it
// separate from the first, and carrying the facing verbatim is what keeps it
// separate from the third.
func (s *StartFacingSuite) TestTheThreeCasesAreDistinguishable() {
	read := func(start *encounter.FieldStart) *session.AtlasStart {
		s.start(startWorld(s.T(), start))
		return s.atlasOf("alice").Start
	}

	none := read(nil)
	bare := read(&encounter.FieldStart{At: cell(1, 1)})
	faced := read(&encounter.FieldStart{At: cell(1, 1), Facing: "e"})

	s.Nil(none, "nobody authored a way in — a host sends no start message at all")
	s.Require().NotNil(bare, "a cell WAS authored, so this is not the first case")
	s.Require().NotNil(faced)

	s.Empty(bare.Facing)
	s.Equal("e", faced.Facing)
	s.Equal(bare.At, faced.At, "the two authored cases differ ONLY in the direction")
	s.NotEqual(*bare, *faced)
}

// TestTheStartWireNamesMatchTheContract pins the JSON names, for
// wirecontract_test.go's own stated reason: a tag regression is invisible to
// every other test here — the Go field keeps its name and every scene passes
// — and the only thing that changes is what a host reading this as JSON sees.
// These names are the proto's (AtlasStart{at, facing}, rpg-api-protos#292).
func TestTheStartWireNamesMatchTheContract(t *testing.T) {
	raw, err := json.Marshal(session.Atlas{
		Start: &session.AtlasStart{At: spatial.Position{X: 1, Y: 3}, Facing: "e"}})
	require.NoError(t, err)

	var wire struct {
		Start map[string]json.RawMessage `json:"start"`
	}
	require.NoError(t, json.Unmarshal(raw, &wire))
	require.Contains(t, wire.Start, "at")
	require.Equal(t, `"e"`, string(wire.Start["facing"]))

	t.Run("a start with no facing spends no key", func(t *testing.T) {
		raw, err := json.Marshal(session.AtlasStart{At: spatial.Position{X: 1, Y: 3}})
		require.NoError(t, err)
		var one map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &one))
		require.NotContains(t, one, "facing",
			"empty is the author saying nothing, and absence says that without a value "+
				"a reader could mistake for a direction")
	})

	t.Run("a map with no start spends no key", func(t *testing.T) {
		raw, err := json.Marshal(session.Atlas{})
		require.NoError(t, err)
		var whole map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &whole))
		require.NotContains(t, whole, "start")
	})
}
