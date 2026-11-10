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
	s.Equal(log.ToData(), log.DataFrom(0), "fresh")
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // deterministic test
	for range 9 {
		s.appendRandom(log, rng)
	}
	s.Equal(log.ToData(), log.DataFrom(0), "populated")
	s.Equal(log.ToData(), log.DataFrom(1), "since at the first seq")
	_, err := log.TrimBefore(&record.TrimBeforeInput{Seq: 5})
	s.Require().NoError(err)
	s.Equal(log.ToData(), log.DataFrom(0), "trimmed")
	s.Equal(log.ToData(), log.DataFrom(3), "since below the oldest retained seq")
}

func (s *SegmentSuite) TestSegmentIsALegalLogDataWithItsOwnTable() {
	log := s.newLog()
	for _, aud := range [][]core.EntityID{audiencePool[1], audiencePool[2], audiencePool[3], audiencePool[1]} {
		_, err := log.Append(&record.AppendInput{Audience: aud, Payload: []byte("x")})
		s.Require().NoError(err)
	}
	seg := log.DataFrom(3)
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
	head := log.DataFrom(1)
	s.Equal(record.LogData{}, head, "a fresh head is the zero LogData")

	_, err := log.Append(&record.AppendInput{Audience: audiencePool[1], Payload: []byte("x")})
	s.Require().NoError(err)
	head = log.DataFrom(2)
	s.Empty(head.Entries)
	s.Empty(head.Audiences)
	s.Equal(uint64(2), head.NextSeq)
	s.NotEmpty(head.LastID, "the head carries the last id since no entry in it does")

	head = log.DataFrom(99)
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
			segments = append(segments, log.DataFrom(since))
			next, err := log.NextSeq()
			s.Require().NoError(err)
			since = next
		}
		head := log.DataFrom(since)

		joined, err := record.JoinData(head, segments...)
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
		joined, err := record.JoinData(log.DataFrom(next), older, log.DataFrom(cut))
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
	return re.DataFrom(1)
}

func (s *SegmentSuite) TestTrimmedHead() {
	rng := rand.New(rand.NewSource(5)) //nolint:gosec // deterministic test
	log := s.newLog()
	for range 10 {
		s.appendRandom(log, rng)
	}
	_, err := log.TrimBefore(&record.TrimBeforeInput{Seq: 6})
	s.Require().NoError(err)
	a := log.DataFrom(0)
	s.appendRandom(log, rng)
	b := log.DataFrom(11)
	joined, err := record.JoinData(log.DataFrom(12), a, b)
	s.Require().NoError(err)
	s.Equal(log.ToData(), joined, "a record starting above seq 1 joins")
	s.loadEqual(log.ToData(), joined, "trimmed")

	_, err = log.TrimBefore(&record.TrimBeforeInput{Seq: 12})
	s.Require().NoError(err)
	joined, err = record.JoinData(log.DataFrom(12))
	s.Require().NoError(err)
	s.Equal(log.ToData(), joined, "trimmed-empty: the head alone")
	s.loadEqual(log.ToData(), joined, "trimmed-empty")
}

func (s *SegmentSuite) TestFreshAndEmptySegments() {
	log := s.newLog()
	joined, err := record.JoinData(log.DataFrom(1), log.DataFrom(1), log.DataFrom(1))
	s.Require().NoError(err)
	s.Equal(record.LogData{}, joined, "fresh log, empty segments")

	_, err = log.MintID()
	s.Require().NoError(err)
	joined, err = record.JoinData(log.DataFrom(1))
	s.Require().NoError(err)
	s.Equal(log.ToData(), joined, "a minted id before any append survives the join")
}

func (s *SegmentSuite) TestJoinRefusesByName() {
	rng := rand.New(rand.NewSource(9)) //nolint:gosec // deterministic test
	log := s.newLog()
	for range 6 {
		s.appendRandom(log, rng)
	}
	a, b := s.prefix(log, 3), log.DataFrom(3)
	head := log.DataFrom(7)
	bAt4 := log.DataFrom(4)
	missing := s.prefix(log, 2)

	badRef := log.DataFrom(3)
	badRef.Entries[0].AudienceRef = uint32(len(badRef.Audiences) + 1)
	badRow := log.DataFrom(1)
	badRow.Audiences = append(badRow.Audiences, nil)
	dupRow := log.DataFrom(1)
	dupRow.Audiences = append(dupRow.Audiences, []core.EntityID{"zed"}, []core.EntityID{"zed"})
	badNext := s.prefix(log, 3)
	badNext.NextSeq = 9
	headWithEntries := log.DataFrom(6)
	headWithTable := log.DataFrom(7)
	headWithTable.Audiences = [][]core.EntityID{{"alice"}}
	staleHead := s.prefix(log, 5)
	staleHead.Entries, staleHead.Audiences = nil, nil
	oldHead := log.DataFrom(7)
	oldHead.LastID = a.Entries[0].ID

	cases := []struct {
		name string
		head record.LogData
		segs []record.LogData
		want string
	}{
		{"gap", head, []record.LogData{missing, bAt4}, "(gap)"},
		{"overlap", head, []record.LogData{a, log.DataFrom(1)}, "(overlap)"},
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
		_, err := record.JoinData(tc.head, tc.segs...)
		s.Require().Error(err, tc.name)
		s.ErrorIs(err, record.ErrInvalidData, tc.name)
		s.Contains(err.Error(), tc.want, tc.name)
	}
}
