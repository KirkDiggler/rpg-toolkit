// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// readrecord_test.go pins ReadRecord (rpg-project#628, ideas/event-stream R5,
// R13, R15, R25): the whole retained record, every audience, after an id,
// and a restart when the id is older than retention.

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type ReadRecordSuite struct {
	suite.Suite
}

func TestReadRecordSuite(t *testing.T) {
	suite.Run(t, new(ReadRecordSuite))
}

// facing is the rich golden's two-room scene: g1 and p1 see each other at
// first light, so its opening carries a sighting audienced to g1 alone.
func (s *ReadRecordSuite) facing(retention int) *encounter.Encounter {
	s.T().Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{
				rectRegion("crypt", -10, 7, 8, 8),
				rectRegion("hall", -2, 7, 6, 6),
			},
			Doors: []encounter.DoorInput{openDoorway("door1", -3, 10, -2, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Position: spatial.Position{X: -10, Y: 7}},
			{ID: "g1", Kind: encounter.KindMonster, Position: spatial.Position{X: -2, Y: 7}},
		},
		Endings:   []encounter.EndingInput{{Key: "leave", Trigger: encounter.TriggerExternal{}}},
		Retention: retention,
		IDs:       newFixedIDs(),
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

func (s *ReadRecordSuite) read(enc *encounter.Encounter, after string) *encounter.ReadRecordOutput {
	s.T().Helper()
	out, err := enc.ReadRecord(&encounter.ReadRecordInput{AfterID: after})
	s.Require().NoError(err)
	return out
}

func entryIDs(entries []record.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

// TestRecordIncludesEveryAudience: a beat audienced to one member is in the
// record, which is the GM view — more than any one member's story.
func (s *ReadRecordSuite) TestRecordIncludesEveryAudience() {
	enc := s.facing(encounter.RetentionUnbounded)

	whole := s.read(enc, "").Entries
	var gOnly bool
	for _, e := range whole {
		if len(e.Audience) == 1 && e.Audience[0] == core.EntityID("g1") {
			gOnly = true
		}
	}
	s.True(gOnly, "the sighting told only to g1 is in the record")

	p1, err := enc.Story(&encounter.StoryInput{Audience: "p1"})
	s.Require().NoError(err)
	s.Greater(len(whole), len(p1))
}

// TestRecordAfterID: only entries after the given id, ascending.
func (s *ReadRecordSuite) TestRecordAfterID() {
	enc := s.facing(encounter.RetentionUnbounded)
	_, err := aRound(enc)
	s.Require().NoError(err)

	whole := s.read(enc, "").Entries
	s.Require().Greater(len(whole), 3)
	s.True(sort.StringsAreSorted(entryIDs(whole)))

	after := s.read(enc, whole[2].ID)
	s.False(after.Restarted)
	s.Equal(entryIDs(whole[3:]), entryIDs(after.Entries))

	last := s.read(enc, whole[len(whole)-1].ID)
	s.Empty(last.Entries, "caught up")
	s.False(last.Restarted)
}

// TestRecordOlderThanRetentionRestarts: with a window of 5 and at least ten
// beats written, an id from before the window answers with the five retained
// beats and Restarted; the oldest retained id answers normally.
func (s *ReadRecordSuite) TestRecordOlderThanRetentionRestarts() {
	enc := s.facing(5)
	for i := 0; i < 5; i++ {
		next, err := enc.NextStorySeq()
		s.Require().NoError(err)
		if next > 10 {
			break
		}
		_, err = aRound(enc)
		s.Require().NoError(err)
	}
	next, err := enc.NextStorySeq()
	s.Require().NoError(err)
	s.Require().Greater(next, uint64(10), "at least ten beats")

	first := s.read(enc, "").Entries[0].ID
	_ = enc.ToData() // retention is enforced at the storage boundary

	out := s.read(enc, first)
	s.True(out.Restarted)
	s.Len(out.Entries, 5)

	oldest := out.Entries[0].ID
	again := s.read(enc, oldest)
	s.False(again.Restarted)
	s.Len(again.Entries, 4)
}

// TestReadRecordRefusesAMalformedAfterID: an AfterID that is not a record id
// is refused, never read as caught up — the three probes from review.
func (s *ReadRecordSuite) TestReadRecordRefusesAMalformedAfterID() {
	enc := s.facing(encounter.RetentionUnbounded)
	held := s.read(enc, "").Entries[0].ID
	for _, after := range []string{"zzz", strings.ToLower(held), "not-an-id"} {
		_, err := enc.ReadRecord(&encounter.ReadRecordInput{AfterID: after})
		s.ErrorIs(err, encounter.ErrInvalidData, "after %q", after)
	}
	s.read(enc, held) // a real id still reads
}

// TestReadRecordNilInput is the module's nil-input law.
func (s *ReadRecordSuite) TestReadRecordNilInput() {
	enc := s.facing(encounter.RetentionUnbounded)
	_, err := enc.ReadRecord(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
}
