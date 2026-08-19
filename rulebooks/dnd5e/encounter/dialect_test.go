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

// closedBlob is one small scene, closed, saved: three members standing in
// plain sight of one another in an off-origin room, so the blob carries sight
// holdings and an outcome — the two things this file ages.
//
// Three rather than two so that ONE observer holds more than one sighting:
// alice holds both bob and carl. With two members every observer's holdings
// are a walk of length one, and the order they are walked in pins nothing.
//
// All three are players on purpose. A player and a monster in sight of each
// other start a fight, and a bubble in the blob is noise for a test about
// coordinates.
func (s *DialectSuite) closedBlob() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{
			{ID: "hall", Width: 8, Height: 8, Origin: dialectOrigin},
		}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 2, Y: 3}},
			{ID: "bob", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 2, Y: 5}},
			{ID: "carl", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 4, Y: 4}},
		},
		Endings:   []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)

	_, err = enc.End(&encounter.EndInput{Ending: "done"})
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().NotNil(data.Outcome, "the scene closed, so the blob has an outcome to age")
	s.Require().Len(data.Intel.Holdings, 3, "each of them is in plain sight of the other two")
	return data
}

// aRoomBearingSighting is one payload in the dialect #1044 replaced.
const aRoomBearingSighting = `{"room":"hall","x":2,"y":5}`

// load is the host's side of the seam: hand the loader a blob and see what it says.
func (s *DialectSuite) load(data encounter.EncounterData) error {
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
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
	// The KEY is what marks the dialect, whatever it holds. Decoding "room"
	// into a typed field would let a null through as absent and a non-string
	// through as unparseable (raised by Copilot on #1072) — and this
	// composition is the only writer of sight payloads, so a payload that
	// names a room AT ALL is not one it wrote today.
	for _, payload := range []string{
		aRoomBearingSighting,
		`{"room":null,"x":32,"y":15}`,
		`{"room":7,"x":32,"y":15}`,
	} {
		s.Run(payload, func() {
			data := s.closedBlob()
			holding := data.Intel.Holdings[core.EntityID("alice")][intel.Subject("bob")]
			holding.Payload = []byte(payload)
			data.Intel.Holdings[core.EntityID("alice")][intel.Subject("bob")] = holding

			err := s.load(data)
			s.Require().Error(err, "a sighting that names a room must not load")
			s.ErrorIs(err, encounter.ErrInvalidData)
			s.Contains(err.Error(), "alice", "the refusal names whose holding it is")
			s.Contains(err.Error(), "bob", "and who it is about")
			s.Contains(err.Error(), "room-local", "and says what is wrong with it")
		})
	}
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

// TestAnotherChannelsPayloadIsNotOurs pins the guard's NARROWNESS, which is a
// policy rather than an oversight.
//
// Intel carries testimony for any channel a composition cares to invent, and
// the room key this one used to write is the only thing in those bytes that is
// ours to recognize. A holding on some other channel is left alone whatever it
// says — including saying "room", which in another channel's vocabulary might
// mean something entirely reasonable.
func (s *DialectSuite) TestAnotherChannelsPayloadIsNotOurs() {
	data := s.closedBlob()
	holding := data.Intel.Holdings[core.EntityID("alice")][intel.Subject("bob")]
	holding.Channel = intel.Channel("hearsay")
	holding.CurrentVia = []intel.Channel{intel.Channel("hearsay")}
	holding.Payload = []byte(aRoomBearingSighting)
	data.Intel.Holdings[core.EntityID("alice")][intel.Subject("bob")] = holding

	s.Require().NoError(s.load(data),
		"a channel this composition never writes is not this composition's business")
}

// TestTheRefusalNamesTheSameSightingEveryTime.
//
// Go randomizes map iteration, so a guard that walked its holdings in map
// order would name a different stale sighting from one run to the next: a
// rejection nobody can write a test against, and a bug report nobody can
// reproduce. Sorted, the answer is alice's sighting of bob every time —
// first observer, first subject — and both orderings have to hold for that
// to be true.
func (s *DialectSuite) TestTheRefusalNamesTheSameSightingEveryTime() {
	data := s.closedBlob()
	for observer, subjects := range data.Intel.Holdings {
		for subject, holding := range subjects {
			holding.Payload = []byte(aRoomBearingSighting)
			data.Intel.Holdings[observer][subject] = holding
		}
	}

	first := s.load(data)
	s.Require().Error(first)
	s.Contains(first.Error(), `"alice"`, "the first observer")
	s.Contains(first.Error(), `"bob"`, "and their first subject")

	// Fifty runs: an unsorted walk would have to pick the same one out of six
	// holdings fifty times running to slip through here.
	for i := range 50 {
		again := s.load(data)
		s.Require().Error(again)
		s.Require().Equal(first.Error(), again.Error(), "run %d named a different sighting", i)
	}
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
