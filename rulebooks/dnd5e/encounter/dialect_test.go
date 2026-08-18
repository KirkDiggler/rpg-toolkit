// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// dialect_test.go pins what happens to a save written before this composition
// spoke one map (rpg-toolkit#1053, #1068).
//
// Kirk's ruling, 2026-08-17: FAIL LOUDLY. Nothing real exists to migrate — the
// only blobs in the world are dev and workbench saves — so a blob written in
// the room-local dialect is REFUSED at load with a clear error, never quietly
// reinterpreted. The cost of the alternative is not a crash: it is a party
// rendered at cells that belong to some other room, in a load that reported
// success.
//
// Two dialects are detectable, and both are checked here:
//
//   - a sight payload carrying a "room" key, which is what every payload
//     looked like before #1044 made them dungeon-absolute;
//   - an outcome member carrying "position" instead of "cell", which is the
//     shape #1068 replaced. A bare pair of numbers cannot be told apart by
//     inspection, which is exactly why the new wire key is a different NAME:
//     the old one no longer lands anywhere, and its absence is the signal.
//
// The room here is anchored at (30,10) so that the two frames are different
// numbers. A room at the origin would make the whole distinction invisible.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type DialectSuite struct {
	suite.Suite
}

func TestDialectSuite(t *testing.T) {
	suite.Run(t, new(DialectSuite))
}

var dialectOrigin = spatial.Position{X: 30, Y: 10}

// closedBlob is one small scene, closed, saved: two members standing in plain
// sight of each other in an off-origin room, so the blob carries both a sight
// holding and an outcome — the two things this file ages.
//
// Both are players on purpose. A player and a monster in sight of each other
// start a fight, and a bubble in the blob is noise for a test about
// coordinates.
func (s *DialectSuite) closedBlob() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{},
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{
			{ID: "hall", Width: 8, Height: 8, Origin: dialectOrigin},
		}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 2, Y: 3}},
			{ID: "bob", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 2, Y: 5}},
		},
		Endings:   []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)

	_, err = enc.End(&encounter.EndInput{Ending: "done"})
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().NotNil(data.Outcome, "the scene closed, so the blob has an outcome to age")
	s.Require().NotEmpty(data.Intel.Holdings, "the two of them are in plain sight, so somebody holds something")
	return data
}

// load is the host's side of the seam: hand the loader a blob and see what it says.
func (s *DialectSuite) load(data encounter.EncounterData) error {
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Initiative: orderAsGiven{},
	})
	return err
}

// TestTodaysBlobLoads is the control. Every refusal below is only meaningful if
// the dialect the composition writes today is the one it accepts.
func (s *DialectSuite) TestTodaysBlobLoads() {
	s.Require().NoError(s.load(s.closedBlob()))
}

// TestARoomBearingSightPayloadIsRefused.
//
// Intel holds opaque bytes and never reads them — that is the leaf's whole
// contract — so nothing beneath this composition can notice that a stored
// payload means something different than it used to. A stale sighting at
// hall-local (2,5) would render at absolute (2,5): a cell in a different room,
// or no room at all.
func (s *DialectSuite) TestARoomBearingSightPayloadIsRefused() {
	data := s.closedBlob()

	aged, err := json.Marshal(map[string]any{"room": "hall", "x": 2, "y": 5})
	s.Require().NoError(err)
	holding := data.Intel.Holdings[core.EntityID("alice")][intel.Subject("bob")]
	holding.Payload = aged
	data.Intel.Holdings[core.EntityID("alice")][intel.Subject("bob")] = holding

	err = s.load(data)
	s.Require().Error(err, "a room-local sighting must not load")
	s.ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "alice", "the refusal names whose holding it is")
	s.Contains(err.Error(), "bob", "and who it is about")
	s.Contains(err.Error(), "room-local", "and says what is wrong with it")
}

// TestARoomLocalOutcomeIsRefused.
//
// The old outcome shape is a room plus a room-local "position". Loading it
// against the new field would leave the outcome's cell nil — an absent
// placement reading as the map's origin, which is a legal cell — so absence is
// checked and named rather than defaulted, the same call RoomData.Origin makes.
func (s *DialectSuite) TestARoomLocalOutcomeIsRefused() {
	err := s.load(s.agedOutcome(s.closedBlob()))
	s.Require().Error(err, "a room-local outcome must not load")
	s.ErrorIs(err, encounter.ErrInvalidData)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.Contains(err.Error(), "alice", "the refusal names the member it could not place")
	s.Contains(err.Error(), "room-local", "and says what is wrong with the blob")
}

// agedOutcome rewrites a fresh blob's outcome members back into the wire shape
// #1068 replaced — the room-LOCAL "position" key — by editing the marshalled
// bytes, because the current Go type has nowhere to put one.
//
// Going through JSON is the point rather than an inconvenience: this is
// exactly the path a host takes with a save written by the old code, unknown
// keys silently dropped and all. Building the aged blob by transforming
// today's keeps the fixture honest as the rest of the shape moves on.
func (s *DialectSuite) agedOutcome(data encounter.EncounterData) encounter.EncounterData {
	raw, err := json.Marshal(data)
	s.Require().NoError(err)

	var blob map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(raw, &blob))

	var outcome map[string]any
	s.Require().NoError(json.Unmarshal(blob["outcome"], &outcome))

	members, ok := outcome["members"].([]any)
	s.Require().True(ok, "the blob's outcome lists its members")
	for _, entry := range members {
		member, isObject := entry.(map[string]any)
		s.Require().True(isObject)
		cell, hasCell := member["cell"].(map[string]any)
		s.Require().True(hasCell, "today's outcome carries a cell, which is what ages into a position")
		delete(member, "cell")
		member["position"] = map[string]any{
			"x": cell["x"].(float64) - dialectOrigin.X,
			"y": cell["y"].(float64) - dialectOrigin.Y,
		}
	}

	outcomeRaw, err := json.Marshal(outcome)
	s.Require().NoError(err)
	blob["outcome"] = outcomeRaw

	agedRaw, err := json.Marshal(blob)
	s.Require().NoError(err)

	var aged encounter.EncounterData
	s.Require().NoError(json.Unmarshal(agedRaw, &aged))
	return aged
}
