// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record_test

import (
	"errors"
	"math/rand"
	"sort"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/stretchr/testify/suite"
)

// fixedSource is a deterministic IDSource: a clock that only moves when the
// test moves it, and seeded entropy.
type fixedSource struct {
	ms   uint64
	rng  *rand.Rand
	fail error
}

func newFixedSource(ms uint64) *fixedSource {
	return &fixedSource{ms: ms, rng: rand.New(rand.NewSource(7))} //nolint:gosec // deterministic test entropy
}

func (f *fixedSource) NowMillis() uint64 { return f.ms }

func (f *fixedSource) Entropy(p []byte) error {
	if f.fail != nil {
		return f.fail
	}
	_, _ = f.rng.Read(p)
	return nil
}

type IDSuite struct {
	suite.Suite
	src *fixedSource
	log *record.Log
}

func (s *IDSuite) SetupTest() {
	s.src = newFixedSource(1_700_000_000_000)
	var err error
	s.log, err = record.NewLog(record.WithIDSource(s.src))
	s.Require().NoError(err)
}

func (s *IDSuite) appendID() string {
	out, err := s.log.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	s.Require().Len(out.ID, 26)
	return out.ID
}

// TestAppendMintsIncreasingIDs: a thousand appends inside one millisecond
// get strictly increasing, unique ids, and the entry carries the id Append
// reported.
func (s *IDSuite) TestAppendMintsIncreasingIDs() {
	ids := make([]string, 1000)
	for i := range ids {
		ids[i] = s.appendID()
	}
	seen := make(map[string]bool, len(ids))
	for i, id := range ids {
		s.False(seen[id], "id %d repeats", i)
		seen[id] = true
		if i > 0 {
			s.Less(ids[i-1], id, "id %d does not increase", i)
		}
	}
	all, err := s.log.All(&record.AllInput{})
	s.Require().NoError(err)
	s.Require().Len(all, len(ids))
	for i, e := range all {
		s.Equal(ids[i], e.ID)
	}
	s.True(sort.StringsAreSorted(ids))
}

// TestIDsIncreaseAcrossMilliseconds: a later millisecond sorts after every id
// of an earlier one, and a clock that steps BACK never makes an id decrease.
func (s *IDSuite) TestIDsIncreaseAcrossMilliseconds() {
	a := s.appendID()
	s.src.ms++
	b := s.appendID()
	s.src.ms -= 10
	c := s.appendID()
	s.Less(a, b)
	s.Less(b, c)
}

// TestIDsCarryNoCount: consecutive ids inside one millisecond are not one
// apart, so no viewer can count the beats between two ids it was sent
// (event-stream R27, R30).
func (s *IDSuite) TestIDsCarryNoCount() {
	a := s.appendID()
	b := s.appendID()
	s.Equal(a[:10], b[:10], "same millisecond, same time prefix")
	s.NotEqual(nextULID(a), b, "an id must not be its predecessor plus one")
}

// TestMintIDSharesTheEntryIDSpace: an id minted for something that is not an
// entry (an act) sorts after every entry before it and before every entry
// after it, and is never handed to an entry.
func (s *IDSuite) TestMintIDSharesTheEntryIDSpace() {
	before := s.appendID()
	minted, err := s.log.MintID()
	s.Require().NoError(err)
	after := s.appendID()
	s.Less(before, minted)
	s.Less(minted, after)
	next, err := s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(3), next, "minting appends nothing")
}

// TestFailedMintChangesNothing: an entropy failure fails the Append and
// leaves the log exactly as it was (R5).
func (s *IDSuite) TestFailedMintChangesNothing() {
	s.appendID()
	s.src.ms++
	s.src.fail = errors.New("no entropy")
	_, err := s.log.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().Error(err)
	next, err := s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(2), next)
	_, err = s.log.MintID()
	s.Require().Error(err)
}

// TestIDSurvivesRoundTrip: ToData then LoadLog keeps every id, and the next
// id after the load still sorts after them.
func (s *IDSuite) TestIDSurvivesRoundTrip() {
	ids := []string{s.appendID(), s.appendID(), s.appendID()}
	data := s.log.ToData()
	for i, ed := range data.Entries {
		s.Equal(ids[i], ed.ID)
	}
	loaded, err := record.LoadLog(data, record.WithIDSource(s.src))
	s.Require().NoError(err)
	s.Equal(data, loaded.ToData())
	out, err := loaded.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	s.Less(ids[2], out.ID, "same millisecond after a load still increases")
}

// TestTrimmedEmptyKeepsTheLastID: a log trimmed to nothing remembers the last
// id it issued, so an id minted after its load still sorts after it.
func (s *IDSuite) TestTrimmedEmptyKeepsTheLastID() {
	s.appendID()
	last := s.appendID()
	_, err := s.log.TrimBefore(&record.TrimBeforeInput{Seq: 3})
	s.Require().NoError(err)
	data := s.log.ToData()
	s.Equal(last, data.LastID)
	loaded, err := record.LoadLog(data, record.WithIDSource(s.src))
	s.Require().NoError(err)
	out, err := loaded.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	s.Less(last, out.ID)
}

// TestLoadLogFillsLegacyIDs: entries persisted before ids existed load with
// deterministic ids in seq order, every one below an id minted afterwards.
func (s *IDSuite) TestLoadLogFillsLegacyIDs() {
	data := record.LogData{
		NextSeq: 6,
		Entries: []record.EntryData{
			{Seq: 3, Payload: []byte("a")},
			{Seq: 4, Payload: []byte("b")},
			{Seq: 5, Payload: []byte("c")},
		},
	}
	first, err := record.LoadLog(data, record.WithIDSource(s.src))
	s.Require().NoError(err)
	second, err := record.LoadLog(data, record.WithIDSource(s.src))
	s.Require().NoError(err)
	a, err := first.All(&record.AllInput{})
	s.Require().NoError(err)
	b, err := second.All(&record.AllInput{})
	s.Require().NoError(err)
	s.Require().Len(a, 3)
	for i := range a {
		s.Len(a[i].ID, 26)
		s.Equal(a[i].ID, b[i].ID, "two loads give the same legacy id")
		if i > 0 {
			s.Less(a[i-1].ID, a[i].ID)
		}
	}
	out, err := first.Append(&record.AppendInput{Payload: []byte("d")})
	s.Require().NoError(err)
	s.Less(a[2].ID, out.ID, "a minted id sorts after every legacy id")
}

// TestLoadLogRejectsBadIDs: ids that are malformed or out of order are an
// unreachable state (R9).
func (s *IDSuite) TestLoadLogRejectsBadIDs() {
	good := s.appendID()
	later := s.appendID()
	cases := map[string]record.LogData{
		"malformed": {NextSeq: 2, Entries: []record.EntryData{{Seq: 1, ID: "not-an-id", Payload: []byte("p")}}},
		"decreasing": {NextSeq: 3, Entries: []record.EntryData{
			{Seq: 1, ID: later, Payload: []byte("p")},
			{Seq: 2, ID: good, Payload: []byte("p")},
		}},
		"repeated": {NextSeq: 3, Entries: []record.EntryData{
			{Seq: 1, ID: good, Payload: []byte("p")},
			{Seq: 2, ID: good, Payload: []byte("p")},
		}},
		"minted then legacy": {NextSeq: 3, Entries: []record.EntryData{
			{Seq: 1, ID: good, Payload: []byte("p")},
			{Seq: 2, Payload: []byte("p")},
		}},
		"malformed last id": {NextSeq: 3, LastID: "zzz"},
	}
	for name, data := range cases {
		_, err := record.LoadLog(data)
		s.ErrorIs(err, record.ErrInvalidData, name)
	}
}

// TestDefaultSourceMints: a log built with no source mints valid, increasing
// ids from the wall clock.
func (s *IDSuite) TestDefaultSourceMints() {
	log, err := record.NewLog()
	s.Require().NoError(err)
	a, err := log.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	b, err := log.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	s.Len(a.ID, 26)
	s.Less(a.ID, b.ID)
}

// nextULID is the id one above id in Crockford base32 — what a counting id
// would be.
func nextULID(id string) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	b := []byte(id)
	for i := len(b) - 1; i >= 0; i-- {
		j := 0
		for j < len(alphabet) && alphabet[j] != b[i] {
			j++
		}
		if j+1 < len(alphabet) {
			b[i] = alphabet[j+1]
			return string(b)
		}
		b[i] = alphabet[0]
	}
	return string(b)
}

func TestIDSuite(t *testing.T) {
	suite.Run(t, new(IDSuite))
}
