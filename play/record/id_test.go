// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record_test

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"strings"
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

// TestIDsCarryNoCount: inside one millisecond the steps between ids are
// heavy-tailed — their spread covers many orders of magnitude — so a distance
// carries only log-scale information about the ids between, not a count
// (event-stream R27, R30). A constant step, or a narrow uniform one, fails.
func (s *IDSuite) TestIDsCarryNoCount() {
	const n = 400
	prev := s.appendID()
	steps := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		next := s.appendID()
		s.Equal(prev[:10], next[:10], "same millisecond, same time prefix")
		steps = append(steps, idDistance(prev, next))
		prev = next
	}
	sort.Float64s(steps)
	median, largest := steps[n/2], steps[n-1]
	s.Greater(largest/median, math.Pow(2, 16),
		"the largest step must dwarf the median: one draw dominates a sum")
	s.Greater(steps[n/2]/steps[n/10], 16.0, "the steps spread over orders of magnitude, not a constant")
}

// TestMintedIDSurvivesSave is the review's probe: an id minted after the last
// append, then a save and load with nothing appended between, must still sort
// below the next entry's id — under a source whose entropy never changes,
// the case a composition pinning bytes would hand the log.
func (s *IDSuite) TestMintedIDSurvivesSave() {
	src := &constantSource{ms: 1_700_000_000_000}
	log, err := record.NewLog(record.WithIDSource(src))
	s.Require().NoError(err)
	_, err = log.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	minted, err := log.MintID()
	s.Require().NoError(err)

	data := log.ToData()
	s.Equal(minted, data.LastID, "a mint above the last entry is written")
	loaded, err := record.LoadLog(data, record.WithIDSource(src))
	s.Require().NoError(err)
	out, err := loaded.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	s.Greater(out.ID, minted)
}

// TestMintBeforeAnyAppendSurvivesSave: the same on a log that has appended
// nothing yet — its snapshot is the zero LogData but for the last id.
func (s *IDSuite) TestMintBeforeAnyAppendSurvivesSave() {
	src := &constantSource{ms: 1_700_000_000_000}
	log, err := record.NewLog(record.WithIDSource(src))
	s.Require().NoError(err)
	minted, err := log.MintID()
	s.Require().NoError(err)

	data := log.ToData()
	s.Equal(record.LogData{LastID: minted}, data)
	loaded, err := record.LoadLog(data, record.WithIDSource(src))
	s.Require().NoError(err)
	out, err := loaded.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	s.Equal(uint64(1), out.Seq)
	s.Greater(out.ID, minted)
}

// TestNoMintWritesNoLastID: a log whose last issue was an entry writes no
// last_id — the entry carries it.
func (s *IDSuite) TestNoMintWritesNoLastID() {
	s.appendID()
	s.Empty(s.log.ToData().LastID)
}

// TestExhaustedMillisecondIsRefused: a burst that would carry the entropy past
// 80 bits fails the Append and changes nothing; it never wraps.
func (s *IDSuite) TestExhaustedMillisecondIsRefused() {
	src := &maxSource{ms: 1_700_000_000_000}
	log, err := record.NewLog(record.WithIDSource(src))
	s.Require().NoError(err)
	var appended uint64
	for i := 0; i < 1<<17; i++ {
		if _, err = log.Append(&record.AppendInput{Payload: []byte("p")}); err != nil {
			break
		}
		appended++
	}
	s.Require().Error(err, "maximal steps exhaust the millisecond")
	next, nerr := log.NextSeq()
	s.Require().NoError(nerr)
	s.Equal(appended+1, next, "the refused append changed nothing")
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

// constantSource is a legal deterministic IDSource whose entropy is always
// zero: every same-millisecond step is the smallest.
type constantSource struct{ ms uint64 }

func (c *constantSource) NowMillis() uint64 { return c.ms }

func (c *constantSource) Entropy(p []byte) error {
	for i := range p {
		p[i] = 0
	}
	return nil
}

// maxSource draws the largest step every time.
type maxSource struct{ ms uint64 }

func (m *maxSource) NowMillis() uint64 { return m.ms }

func (m *maxSource) Entropy(p []byte) error {
	for i := range p {
		p[i] = 0xff
	}
	return nil
}

// idDistance is b minus a as a float, from the ids' 80 entropy bits.
func idDistance(a, b string) float64 {
	return idEntropy(b) - idEntropy(a)
}

// idEntropy decodes an id's low 80 bits as a float (enough precision for a
// ratio test).
func idEntropy(id string) float64 {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	v := new(big.Int)
	for _, ch := range id {
		v.Mul(v, big.NewInt(32))
		v.Add(v, big.NewInt(int64(strings.IndexRune(alphabet, ch))))
	}
	v.And(v, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 80), big.NewInt(1)))
	f, _ := new(big.Float).SetInt(v).Float64()
	return f
}

func TestIDSuite(t *testing.T) {
	suite.Run(t, new(IDSuite))
}
