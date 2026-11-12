// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// SegmentSuite pins DataFrom and JoinData: a record a store keeps as a head
// plus appended segments joins back to exactly what ToData writes
// (rpg-project#897 leg c).
type SegmentSuite struct {
	suite.Suite
}

func TestSegmentSuite(t *testing.T) {
	suite.Run(t, new(SegmentSuite))
}

// audiencePool is a handful of overlapping rosters, order significant.
var audiencePool = [][]core.EntityID{
	nil,
	{"alice", "bob", "carol"},
	{"bob", "alice"},
	{"carol"},
	{"dave", "erin"},
	{"alice"},
}

// from is DataFrom at since.
func (s *SegmentSuite) from(log *record.Log, since uint64) record.LogData {
	data, err := log.DataFrom(&record.DataFromInput{Since: since})
	s.Require().NoError(err)
	return data
}

// join is JoinData with no floor: every seq must be present.
func join(head record.LogData, segments ...record.LogData) (record.LogData, error) {
	return joinAt(0, head, segments...)
}

// joinAt is JoinData at floor.
func joinAt(floor uint64, head record.LogData, segments ...record.LogData) (record.LogData, error) {
	return record.JoinData(&record.JoinDataInput{Head: head, Floor: floor, Segments: segments})
}

// floorOf is the log's trim floor: its oldest retained seq, NextSeq when it
// retains none.
func (s *SegmentSuite) floorOf(log *record.Log) uint64 {
	all, err := log.All(&record.AllInput{})
	s.Require().NoError(err)
	if len(all) > 0 {
		return all[0].Seq
	}
	next, err := log.NextSeq()
	s.Require().NoError(err)
	return next
}

func (s *SegmentSuite) newLog() *record.Log {
	log, err := record.NewLog(record.WithIDSource(newFixedSource(1_800_000_000_000)))
	s.Require().NoError(err)
	return log
}

func (s *SegmentSuite) appendRandom(log *record.Log, rng *rand.Rand) {
	n, err := log.NextSeq()
	s.Require().NoError(err)
	_, err = log.Append(&record.AppendInput{
		At:          n * 10,
		Correlation: fmt.Sprintf("c%d", rng.Intn(3)),
		Audience:    audiencePool[rng.Intn(len(audiencePool))],
		Tags:        map[string]string{"kind": fmt.Sprintf("k%d", rng.Intn(4))},
		Payload:     []byte(fmt.Sprintf("beat-%d", n)),
	})
	s.Require().NoError(err)
}

// loadEqual requires data to load to a log that writes exactly want.
func (s *SegmentSuite) loadEqual(want, data record.LogData, msg string) {
	loaded, err := record.LoadLog(data)
	s.Require().NoError(err, msg)
	s.Equal(want, loaded.ToData(), msg)
}

func (s *SegmentSuite) TestToDataIsDataFromZero() {
	log := s.newLog()
	s.Equal(log.ToData(), s.from(log, 0), "fresh")
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // deterministic test
	for range 9 {
		s.appendRandom(log, rng)
	}
	s.Equal(log.ToData(), s.from(log, 0), "populated")
	s.Equal(log.ToData(), s.from(log, 1), "since at the first seq")
	_, err := log.TrimBefore(&record.TrimBeforeInput{Seq: 5})
	s.Require().NoError(err)
	s.Equal(log.ToData(), s.from(log, 0), "trimmed")
	s.Equal(log.ToData(), s.from(log, 3), "since below the oldest retained seq")
}

func (s *SegmentSuite) TestSegmentIsALegalLogDataWithItsOwnTable() {
	log := s.newLog()
	for _, aud := range [][]core.EntityID{audiencePool[1], audiencePool[2], audiencePool[3], audiencePool[1]} {
		_, err := log.Append(&record.AppendInput{Audience: aud, Payload: []byte("x")})
		s.Require().NoError(err)
	}
	seg := s.from(log, 3)
	s.Require().Len(seg.Entries, 2)
	s.Equal(uint64(3), seg.Entries[0].Seq)
	s.Equal(uint64(5), seg.NextSeq)
	s.Equal([][]core.EntityID{audiencePool[3], audiencePool[1]}, seg.Audiences,
		"the table holds only this segment's audiences, first-seen within it")
	s.Equal(uint32(1), seg.Entries[0].AudienceRef)
	s.Equal(uint32(2), seg.Entries[1].AudienceRef)

	loaded, err := record.LoadLog(seg)
	s.Require().NoError(err, "a segment loads on its own")
	got, err := loaded.All(&record.AllInput{FromSeq: 0})
	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Equal(audiencePool[1], got[1].Audience)
}

func (s *SegmentSuite) TestHeadCarriesNoEntries() {
	log := s.newLog()
	head := s.from(log, 1)
	s.Equal(record.LogData{}, head, "a fresh head is the zero LogData")

	_, err := log.Append(&record.AppendInput{Audience: audiencePool[1], Payload: []byte("x")})
	s.Require().NoError(err)
	head = s.from(log, 2)
	s.Empty(head.Entries)
	s.Empty(head.Audiences)
	s.Equal(uint64(2), head.NextSeq)
	s.NotEmpty(head.LastID, "the head carries the last id since no entry in it does")

	head = s.from(log, 99)
	s.Empty(head.Entries, "since past NextSeq is still a head")
}

// TestSavesJoinBackToToData is the property: a log saved as a series of
// appended segments (each DataFrom the first seq the store lacks) plus the
// final head joins to exactly ToData, and loads to an equal log.
func (s *SegmentSuite) TestSavesJoinBackToToData() {
	for seed := int64(1); seed <= 200; seed++ {
		rng := rand.New(rand.NewSource(seed)) //nolint:gosec // deterministic test
		log := s.newLog()
		var segments []record.LogData
		since := uint64(1)
		for range rng.Intn(6) + 1 {
			for range rng.Intn(8) { // zero appends is a beatless save
				s.appendRandom(log, rng)
			}
			if rng.Intn(4) == 0 {
				_, err := log.MintID()
				s.Require().NoError(err)
			}
			segments = append(segments, s.from(log, since))
			next, err := log.NextSeq()
			s.Require().NoError(err)
			since = next
		}
		head := s.from(log, since)

		joined, err := join(head, segments...)
		s.Require().NoError(err, "seed %d", seed)
		s.Equal(log.ToData(), joined, "seed %d: joined equals ToData", seed)
		s.loadEqual(log.ToData(), joined, fmt.Sprintf("seed %d", seed))
	}
}

func (s *SegmentSuite) TestSplitAtEverySeq() {
	rng := rand.New(rand.NewSource(3)) //nolint:gosec // deterministic test
	log := s.newLog()
	for range 12 {
		s.appendRandom(log, rng)
	}
	next, err := log.NextSeq()
	s.Require().NoError(err)
	for cut := uint64(1); cut <= next; cut++ {
		// Rebuild the older part as the segment a store would hold, by
		// replaying the same appends up to the cut.
		older := s.prefix(log, cut)
		joined, err := join(s.from(log, next), older, s.from(log, cut))
		s.Require().NoError(err, "cut %d", cut)
		s.Equal(log.ToData(), joined, "cut %d", cut)
	}
}

// prefix returns the segment holding log's entries below cut, as DataFrom
// would have written it when NextSeq was cut.
func (s *SegmentSuite) prefix(log *record.Log, cut uint64) record.LogData {
	all, err := log.All(&record.AllInput{FromSeq: 0})
	s.Require().NoError(err)
	re := s.newLog()
	for _, e := range all {
		if e.Seq >= cut {
			break
		}
		_, err := re.Append(&record.AppendInput{
			At: e.At, Correlation: e.Correlation, Audience: e.Audience, Tags: e.Tags, Payload: e.Payload,
		})
		s.Require().NoError(err)
	}
	return s.from(re, 1)
}

func (s *SegmentSuite) TestTrimmedHead() {
	rng := rand.New(rand.NewSource(5)) //nolint:gosec // deterministic test
	log := s.newLog()
	for range 10 {
		s.appendRandom(log, rng)
	}
	_, err := log.TrimBefore(&record.TrimBeforeInput{Seq: 6})
	s.Require().NoError(err)
	a := s.from(log, 0)
	s.appendRandom(log, rng)
	b := s.from(log, 11)
	_, err = join(s.from(log, 12), a, b)
	s.Require().ErrorIs(err, record.ErrInvalidData, "with no floor, seqs 1-5 are missing")
	joined, err := joinAt(6, s.from(log, 12), a, b)
	s.Require().NoError(err)
	s.Equal(log.ToData(), joined, "a record starting at its floor joins")
	s.loadEqual(log.ToData(), joined, "trimmed")

	_, err = log.TrimBefore(&record.TrimBeforeInput{Seq: 12})
	s.Require().NoError(err)
	joined, err = joinAt(12, s.from(log, 12), a, b)
	s.Require().NoError(err)
	s.Equal(log.ToData(), joined, "trimmed-empty: everything stored is below the floor")
	s.loadEqual(log.ToData(), joined, "trimmed-empty")
}

func (s *SegmentSuite) TestFreshAndEmptySegments() {
	log := s.newLog()
	joined, err := join(s.from(log, 1), s.from(log, 1), s.from(log, 1))
	s.Require().NoError(err)
	s.Equal(record.LogData{}, joined, "fresh log, empty segments")

	_, err = log.MintID()
	s.Require().NoError(err)
	joined, err = join(s.from(log, 1))
	s.Require().NoError(err)
	s.Equal(log.ToData(), joined, "a minted id before any append survives the join")
}

func (s *SegmentSuite) TestJoinRefusesByName() {
	rng := rand.New(rand.NewSource(9)) //nolint:gosec // deterministic test
	log := s.newLog()
	for range 6 {
		s.appendRandom(log, rng)
	}
	a, b := s.prefix(log, 3), s.from(log, 3)
	head := s.from(log, 7)
	bAt4 := s.from(log, 4)
	missing := s.prefix(log, 2)

	badRef := s.from(log, 3)
	badRef.Entries[0].AudienceRef = uint32(len(badRef.Audiences) + 1)
	badRow := s.from(log, 1)
	badRow.Audiences = append(badRow.Audiences, nil)
	dupRow := s.from(log, 1)
	dupRow.Audiences = append(dupRow.Audiences, []core.EntityID{"zed"}, []core.EntityID{"zed"})
	badNext := s.prefix(log, 3)
	badNext.NextSeq = 9
	headWithEntries := s.from(log, 6)
	headWithTable := s.from(log, 7)
	headWithTable.Audiences = [][]core.EntityID{{"alice"}}
	staleHead := s.prefix(log, 5)
	staleHead.Entries, staleHead.Audiences = nil, nil
	oldHead := s.from(log, 7)
	oldHead.LastID = a.Entries[0].ID

	cases := []struct {
		name string
		head record.LogData
		segs []record.LogData
		want string
	}{
		{"gap", head, []record.LogData{missing, bAt4}, "(gap)"},
		{"overlap", head, []record.LogData{a, s.from(log, 1)}, "(overlap)"},
		{"head with entries", headWithEntries, []record.LogData{a}, "head holds 1 entries"},
		{"head with a table", headWithTable, []record.LogData{a}, "head holds 1 audience rows"},
		{"ref past the table", head, []record.LogData{a, badRef}, "past the table"},
		{"empty row", head, []record.LogData{badRow}, "empty (the empty audience is ref 0)"},
		{"repeated row", head, []record.LogData{dupRow}, "repeats audiences"},
		{"segment next_seq", head, []record.LogData{badNext, b}, "segment[0] next_seq 9"},
		{"head next_seq", staleHead, []record.LogData{a, b}, "head next_seq 5"},
		{"head older id", oldHead, []record.LogData{a, b}, "older than the last joined id"},
	}
	for _, tc := range cases {
		_, err := join(tc.head, tc.segs...)
		s.Require().Error(err, tc.name)
		s.ErrorIs(err, record.ErrInvalidData, tc.name)
		s.Contains(err.Error(), tc.want, tc.name)
	}
}

// TestSavesThatTrimJoinBackToToData is the trim property (rpg-project#897
// review of #2352): a log trimmed to a retention window before every save,
// with windows smaller than one save's appends, so a segment starts above the
// store's last seq + 1. The store keeps the floor of its last save and drops
// the segments wholly below it; the join at that floor is exactly ToData.
func (s *SegmentSuite) TestSavesThatTrimJoinBackToToData() {
	for seed := int64(1); seed <= 200; seed++ {
		rng := rand.New(rand.NewSource(seed)) //nolint:gosec // deterministic test
		window := uint64(rng.Intn(6) + 1)
		log := s.newLog()
		var stored []record.LogData
		since := uint64(1)
		for save := range rng.Intn(8) + 1 {
			appends := rng.Intn(12) // often more than the window
			for range appends {
				s.appendRandom(log, rng)
			}
			if rng.Intn(4) == 0 {
				_, err := log.MintID()
				s.Require().NoError(err)
			}
			// The save: trim to the window, as the encounter does at its
			// storage boundary, then append the segment from since.
			next, err := log.NextSeq()
			s.Require().NoError(err)
			if next > window {
				_, err := log.TrimBefore(&record.TrimBeforeInput{Seq: next - window})
				s.Require().NoError(err)
			}
			floor := s.floorOf(log)
			stored = append(stored, s.from(log, since))
			kept := stored[:0]
			for _, seg := range stored {
				if n := len(seg.Entries); n > 0 && seg.Entries[n-1].Seq < floor {
					continue // the store drops a segment wholly below the floor
				}
				kept = append(kept, seg)
			}
			stored = kept
			since = next

			joined, err := joinAt(floor, s.from(log, next), stored...)
			s.Require().NoError(err, "seed %d save %d window %d appends %d", seed, save, window, appends)
			s.Equal(log.ToData(), joined, "seed %d save %d: joined equals ToData", seed, save)
			s.loadEqual(log.ToData(), joined, fmt.Sprintf("seed %d save %d", seed, save))
		}
	}
}

// TestASegmentStartingAboveTheStoreJoinsAtTheFloor is the review's probe: a
// verb appends more than the window, so the segment starts above the store's
// last seq + 1. Without the floor that is a gap; at the floor it joins.
func (s *SegmentSuite) TestASegmentStartingAboveTheStoreJoinsAtTheFloor() {
	rng := rand.New(rand.NewSource(11)) //nolint:gosec // deterministic test
	log := s.newLog()
	for range 3 {
		s.appendRandom(log, rng)
	}
	first := s.from(log, 1) // the store holds 1-3
	for range 10 {
		s.appendRandom(log, rng)
	}
	_, err := log.TrimBefore(&record.TrimBeforeInput{Seq: 10}) // a window of 4
	s.Require().NoError(err)
	second := s.from(log, 4) // starts at 10
	s.Require().Equal(uint64(10), second.Entries[0].Seq)

	_, err = join(s.from(log, 14), first, second)
	s.Require().ErrorIs(err, record.ErrInvalidData)
	s.Contains(err.Error(), "(gap)")

	joined, err := joinAt(10, s.from(log, 14), first, second)
	s.Require().NoError(err)
	s.Equal(log.ToData(), joined, "the trimmed entries do not come back")
}

func (s *SegmentSuite) TestFloorRefusesByName() {
	rng := rand.New(rand.NewSource(13)) //nolint:gosec // deterministic test
	log := s.newLog()
	for range 8 {
		s.appendRandom(log, rng)
	}
	head := s.from(log, 9)
	upToFour := s.prefix(log, 5)
	fromSeven := s.from(log, 7)

	_, err := joinAt(10, head, s.from(log, 1))
	s.Require().ErrorIs(err, record.ErrInvalidData)
	s.Contains(err.Error(), "floor 10 is past the head's next_seq 9")

	_, err = joinAt(3, head, upToFour, fromSeven)
	s.Require().ErrorIs(err, record.ErrInvalidData)
	s.Contains(err.Error(), "seq 5 is missing at or above floor 3 (gap)", "a gap above the floor refuses")

	_, err = joinAt(5, head, upToFour)
	s.Require().ErrorIs(err, record.ErrInvalidData)
	s.Contains(err.Error(), "seqs 5 through 8 are missing", "everything stored is below the floor, but the floor is not NextSeq")

	_, err = joinAt(3, head, s.from(log, 1), s.from(log, 3))
	s.Require().ErrorIs(err, record.ErrInvalidData)
	s.Contains(err.Error(), "repeats seq 3 (overlap)", "an overlap at or above the floor refuses")

	joined, err := joinAt(5, head, s.from(log, 1), s.from(log, 3))
	s.Require().ErrorIs(err, record.ErrInvalidData, "an overlap above the floor still refuses")
	s.Empty(joined.Entries)

	_, err = joinAt(7, head, upToFour, s.prefix(log, 7), fromSeven)
	s.Require().NoError(err, "overlaps wholly below the floor are the store's straddlers, dropped")
}

func (s *SegmentSuite) TestNilInputs() {
	_, err := record.JoinData(nil)
	s.ErrorIs(err, record.ErrNilInput)
	_, err = s.newLog().DataFrom(nil)
	s.ErrorIs(err, record.ErrNilInput)
}
