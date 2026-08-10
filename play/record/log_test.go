// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/stretchr/testify/suite"
)

type RecordSuite struct {
	suite.Suite
	log *record.Log
}

func (s *RecordSuite) SetupTest() {
	var err error
	s.log, err = record.NewLog()
	s.Require().NoError(err)
}

func (s *RecordSuite) TestAppendAssignsGaplessSeqFromOne() {
	const (
		alice     = "alice"
		bob       = "bob"
		kind      = "kind"
		kindClock = "clock.turn_started"
		actorKey  = "actor"
	)
	out, err := s.log.Append(&record.AppendInput{
		At: 7, Correlation: "act-1",
		Audience: []core.EntityID{alice, bob},
		Tags:     map[string]string{kind: kindClock, actorKey: alice},
		Payload:  []byte(`{"x":1}`),
	})
	s.Require().NoError(err)
	s.Equal(uint64(1), out.Seq)
	out, err = s.log.Append(&record.AppendInput{Audience: []core.EntityID{alice}, Payload: []byte{}})
	s.Require().NoError(err)
	s.Equal(uint64(2), out.Seq)
	next, err := s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(3), next)
}

func (s *RecordSuite) TestAppendValidationOrderAndSentinels() {
	_, err := s.log.Append(nil)
	s.Require().ErrorIs(err, record.ErrNilInput)
	// multi-defect input: audience checked before tags before payload
	_, err = s.log.Append(&record.AppendInput{
		Audience: []core.EntityID{"a", "a"},
		Tags:     map[string]string{"": "x"},
		Payload:  nil,
	})
	s.Require().ErrorIs(err, record.ErrBadAudience)
	_, err = s.log.Append(&record.AppendInput{Tags: map[string]string{"": "x"}, Payload: nil})
	s.Require().ErrorIs(err, record.ErrBadTag)
	_, err = s.log.Append(&record.AppendInput{Payload: nil})
	s.Require().ErrorIs(err, record.ErrNoPayload)
	_, err = s.log.Append(&record.AppendInput{Audience: []core.EntityID{""}, Payload: []byte{}})
	s.Require().ErrorIs(err, record.ErrBadAudience)
	// R5: nothing changed
	next, err := s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(1), next)
	// R5 entry purity: no entry stored by any failed call
	all, err := s.log.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Empty(all)
}

func (s *RecordSuite) TestAppendNormalizesAndCopies() {
	const (
		alice   = "alice"
		mutated = "mutated"
	)
	aud := []core.EntityID{alice}
	tags := map[string]string{"k": "v"}
	payload := []byte("p")
	_, err := s.log.Append(&record.AppendInput{
		At: 42, Correlation: "test-corr", Audience: aud, Tags: tags, Payload: payload,
	})
	s.Require().NoError(err)
	aud[0] = mutated
	tags["k"] = mutated
	payload[0] = 'X'
	all, err := s.log.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	// Envelope-fidelity: Seq, At, Correlation preserved exactly
	s.Equal(uint64(1), all[0].Seq)
	s.Equal(uint64(42), all[0].At)
	s.Equal("test-corr", all[0].Correlation)
	// Audience and tags copied, not aliased
	s.Equal([]core.EntityID{alice}, all[0].Audience)
	s.Equal(map[string]string{"k": "v"}, all[0].Tags)
	// Payload copied, not aliased
	s.Equal([]byte("p"), all[0].Payload)
	// empty-non-nil normalized to nil on store (family convention)
	_, err = s.log.Append(&record.AppendInput{
		Audience: []core.EntityID{}, Tags: map[string]string{}, Payload: []byte("q"),
	})
	s.Require().NoError(err)
	all, err = s.log.All(&record.AllInput{FromSeq: 2})
	s.Require().NoError(err)
	s.Nil(all[0].Audience)
	s.Nil(all[0].Tags)
}

// appendN appends n entries to the log, each with payload []byte("p") and no
// audience or tags. Panics on error.
func (s *RecordSuite) appendN(n int) {
	for i := 0; i < n; i++ {
		_, err := s.log.Append(&record.AppendInput{Payload: []byte("p")})
		s.Require().NoError(err)
	}
}

// appendBeat appends a beat with the given audience, tags, and payload.
// Panics on error.
func (s *RecordSuite) appendBeat(aud []core.EntityID, tags map[string]string, payload string) {
	_, err := s.log.Append(&record.AppendInput{
		Audience: aud,
		Tags:     tags,
		Payload:  []byte(payload),
	})
	s.Require().NoError(err)
}

// seqs maps entries to their Seq values.
func seqs(es []record.Entry) []uint64 {
	result := make([]uint64, len(es))
	for i, e := range es {
		result[i] = e.Seq
	}
	return result
}

func (s *RecordSuite) TestTrimBefore() {
	// Case 1: appendN(4), TrimBefore{Seq: 3} → Removed: 2; All{FromSeq: 1} returns [3, 4]
	s.appendN(4)
	out, err := s.log.TrimBefore(&record.TrimBeforeInput{Seq: 3})
	s.Require().NoError(err)
	s.Equal(2, out.Removed)
	all, err := s.log.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Equal([]uint64{3, 4}, seqs(all))

	// Case 2: TrimBefore{Seq: 3} again → Removed: 0, no error
	out, err = s.log.TrimBefore(&record.TrimBeforeInput{Seq: 3})
	s.Require().NoError(err)
	s.Equal(0, out.Removed)

	// Case 3: TrimBefore{Seq: 5} (== NextSeq) → legal, Removed: 2, log now empty
	out, err = s.log.TrimBefore(&record.TrimBeforeInput{Seq: 5})
	s.Require().NoError(err)
	s.Equal(2, out.Removed)
	all, err = s.log.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Empty(all)
	next, err := s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(5), next)

	// Case 4: TrimBefore{Seq: 6} (> NextSeq) → ErrBadSeq, state unchanged
	_, err = s.log.TrimBefore(&record.TrimBeforeInput{Seq: 6})
	s.Require().ErrorIs(err, record.ErrBadSeq)
	next, err = s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(5), next)
	all, err = s.log.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Empty(all)

	// Case 5: TrimBefore(nil) → ErrNilInput
	_, err = s.log.TrimBefore(nil)
	s.Require().ErrorIs(err, record.ErrNilInput)

	// Case 6: Fresh log, TrimBefore{Seq: 1} → Removed: 0, no error
	freshLog, err := record.NewLog()
	s.Require().NoError(err)
	out, err = freshLog.TrimBefore(&record.TrimBeforeInput{Seq: 1})
	s.Require().NoError(err)
	s.Equal(0, out.Removed)
}

func (s *RecordSuite) TestSliceForProjectsAudienceAndTags() {
	const (
		alice      = "alice"
		bob        = "bob"
		door3      = "door-3"
		kind       = "kind"
		subject    = "subject"
		kindShared = "shared"
		kindIntel  = "intel.first_contact"
		kindGMNote = "gm.note"
	)
	s.appendBeat([]core.EntityID{alice, bob}, map[string]string{kind: kindShared}, "b1")
	s.appendBeat([]core.EntityID{alice}, map[string]string{kind: kindIntel, subject: door3}, "b2")
	s.appendBeat([]core.EntityID{bob}, map[string]string{kind: kindIntel, subject: door3}, "b3")
	s.appendBeat(nil, map[string]string{kind: kindGMNote}, "b4") // empty audience: no viewer

	aliceSlice, err := s.log.SliceFor(&record.SliceForInput{Viewer: alice, FromSeq: 1})
	s.Require().NoError(err)
	s.Equal([]uint64{1, 2}, seqs(aliceSlice))

	firsts, err := s.log.SliceFor(&record.SliceForInput{Viewer: alice, FromSeq: 1,
		Tags: map[string]string{kind: kindIntel}})
	s.Require().NoError(err)
	s.Equal([]uint64{2}, seqs(firsts))

	door, err := s.log.All(&record.AllInput{FromSeq: 1, Tags: map[string]string{subject: door3}})
	s.Require().NoError(err)
	s.Equal([]uint64{2, 3}, seqs(door))

	gm, err := s.log.All(&record.AllInput{FromSeq: 1, Tags: map[string]string{kind: kindGMNote}})
	s.Require().NoError(err)
	s.Equal([]uint64{4}, seqs(gm))
	for _, v := range []core.EntityID{alice, bob} {
		sl, serr := s.log.SliceFor(&record.SliceForInput{Viewer: v, FromSeq: 4})
		s.Require().NoError(serr)
		s.Empty(sl, "empty audience means no viewer")
	}
}

func (s *RecordSuite) TestSliceForAndAllErrorHandling() {
	const alice = "alice"
	// All(nil) → ErrNilInput
	_, err := s.log.All(nil)
	s.Require().ErrorIs(err, record.ErrNilInput)

	// SliceFor(nil) → ErrNilInput
	_, err = s.log.SliceFor(nil)
	s.Require().ErrorIs(err, record.ErrNilInput)

	// Empty viewer → ErrNoViewer
	_, err = s.log.SliceFor(&record.SliceForInput{Viewer: "", FromSeq: 1})
	s.Require().ErrorIs(err, record.ErrNoViewer)

	// SliceFor with empty filter key → ErrBadTag
	_, err = s.log.SliceFor(&record.SliceForInput{Viewer: alice, FromSeq: 1,
		Tags: map[string]string{"": "x"}})
	s.Require().ErrorIs(err, record.ErrBadTag)

	// Viewer-before-tags precedence: {Viewer: "", Tags: {"": "x"}} → ErrNoViewer
	_, err = s.log.SliceFor(&record.SliceForInput{Viewer: "", FromSeq: 1,
		Tags: map[string]string{"": "x"}})
	s.Require().ErrorIs(err, record.ErrNoViewer)

	// All with empty filter key → ErrBadTag
	_, err = s.log.All(&record.AllInput{FromSeq: 1, Tags: map[string]string{"": "x"}})
	s.Require().ErrorIs(err, record.ErrBadTag)

	// R5: none of these mutate (NextSeq still 1)
	next, err := s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(1), next)
}

func (s *RecordSuite) TestEmptyValueTagFilter() {
	const (
		alice = "alice"
		flag  = "flag"
	)
	// Append beats: one with empty value, one with non-empty, one with nil tags, one missing the key
	s.appendBeat([]core.EntityID{alice}, map[string]string{flag: ""}, "b1")
	s.appendBeat([]core.EntityID{alice}, map[string]string{flag: "y"}, "b2")
	s.appendBeat([]core.EntityID{alice}, nil, "b3")                                 // nil tags
	s.appendBeat([]core.EntityID{alice}, map[string]string{"other": "value"}, "b4") // missing the key

	// Filter {flag: ""} returns only the first
	result, err := s.log.SliceFor(&record.SliceForInput{Viewer: alice, FromSeq: 1,
		Tags: map[string]string{flag: ""}})
	s.Require().NoError(err)
	s.Equal([]uint64{1}, seqs(result), "only entry with flag='' should match")

	// Verify nil-tags entry does NOT match filter
	s.NotContains(seqs(result), uint64(3), "entry with nil tags must not match {flag: ''} filter")

	// Verify missing-key entry does NOT match filter
	s.NotContains(seqs(result), uint64(4), "entry with tags missing the key must not match {flag: ''} filter")
}

func (s *RecordSuite) TestCopyOutImmunity() {
	const (
		alice   = "alice"
		mutated = "mutated"
	)
	// Append a beat
	s.appendBeat([]core.EntityID{alice}, map[string]string{"k": "v"}, "payload")

	// Query and mutate the result
	results, err := s.log.SliceFor(&record.SliceForInput{Viewer: alice, FromSeq: 1})
	s.Require().NoError(err)
	s.Len(results, 1)

	// Mutate the result's Audience, Tags, and Payload
	if results[0].Audience != nil {
		results[0].Audience[0] = mutated
	}
	results[0].Tags["k"] = mutated
	results[0].Payload[0] = 'X'

	// Re-query and verify the original is unchanged
	results2, err := s.log.SliceFor(&record.SliceForInput{Viewer: alice, FromSeq: 1})
	s.Require().NoError(err)
	s.Len(results2, 1)
	s.Equal([]core.EntityID{alice}, results2[0].Audience)
	s.Equal(map[string]string{"k": "v"}, results2[0].Tags)
	s.Equal([]byte("payload"), results2[0].Payload)
}

func (s *RecordSuite) TestPersistenceIdleConvention() {
	// Fresh log: NewLog().ToData() → zero LogData
	freshLog, err := record.NewLog()
	s.Require().NoError(err)
	data := freshLog.ToData()
	s.Equal(record.LogData{}, data, "fresh log ToData must equal zero LogData")

	// Zero LogData marshals to {}
	jsonBytes, err := json.Marshal(data)
	s.Require().NoError(err)
	s.Equal([]byte("{}"), jsonBytes, "zero LogData must marshal to {}")

	// LoadLog(LogData{}) → fresh log with NextSeq() == 1
	loaded, err := record.LoadLog(record.LogData{})
	s.Require().NoError(err)
	next, err := loaded.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(1), next, "loaded fresh log must have NextSeq == 1")

	// Append to fresh loaded log gets Seq 1
	out, err := loaded.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	s.Equal(uint64(1), out.Seq, "first append after LoadLog(zero) must get Seq 1")
}

func (s *RecordSuite) TestPersistenceRoundTrips() {
	const (
		alice   = "alice"
		bob     = "bob"
		kind    = "kind"
		intelK  = "intel.first_contact"
		subject = "subject"
		door3   = "door-3"
	)

	// Case (a): populated round-trip
	s.appendBeat([]core.EntityID{alice, bob}, map[string]string{kind: intelK}, "b1")
	s.appendBeat([]core.EntityID{alice}, map[string]string{kind: intelK, subject: door3}, "b2")
	s.appendBeat([]core.EntityID{bob}, map[string]string{subject: door3}, "b3")

	// ToData, LoadLog, ToData again → identical
	data1 := s.log.ToData()
	loaded, err := record.LoadLog(data1)
	s.Require().NoError(err)
	data2 := loaded.ToData()
	s.Equal(data1, data2, "round-trip ToData/LoadLog/ToData must preserve data")

	// Append after reload continues sequence at Seq 4
	out, err := loaded.Append(&record.AppendInput{Payload: []byte("b4")})
	s.Require().NoError(err)
	s.Equal(uint64(4), out.Seq, "append after reload must continue sequence")

	// SliceFor identical pre/post
	alicePre, err := s.log.SliceFor(&record.SliceForInput{Viewer: alice, FromSeq: 1})
	s.Require().NoError(err)
	alicePost, err := loaded.SliceFor(&record.SliceForInput{Viewer: alice, FromSeq: 1})
	s.Require().NoError(err)
	s.Equal(seqs(alicePre), seqs(alicePost), "SliceFor must be identical pre/post load")

	// Case (b): trimmed-empty
	log2, err := record.NewLog()
	s.Require().NoError(err)
	_, err = log2.Append(&record.AppendInput{Payload: []byte("a")})
	s.Require().NoError(err)
	_, err = log2.Append(&record.AppendInput{Payload: []byte("b")})
	s.Require().NoError(err)
	_, err = log2.TrimBefore(&record.TrimBeforeInput{Seq: 3})
	s.Require().NoError(err)

	trimData := log2.ToData()
	s.Equal(uint64(3), trimData.NextSeq, "trimmed-empty must store NextSeq 3")
	s.Nil(trimData.Entries, "trimmed-empty must have nil entries")

	trimLoaded, err := record.LoadLog(trimData)
	s.Require().NoError(err)
	next, err := trimLoaded.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(3), next, "loaded trimmed-empty must have NextSeq 3")

	// Next append gets Seq 3
	out, err = trimLoaded.Append(&record.AppendInput{Payload: []byte("p")})
	s.Require().NoError(err)
	s.Equal(uint64(3), out.Seq, "append after trimmed-empty load must get Seq 3")

	// Case (c): post-trim POPULATED (natural-mistake catcher)
	log3, err := record.NewLog()
	s.Require().NoError(err)
	for i := 0; i < 4; i++ {
		_, err := log3.Append(&record.AppendInput{Payload: []byte("p")})
		s.Require().NoError(err)
	}
	_, err = log3.TrimBefore(&record.TrimBeforeInput{Seq: 3})
	s.Require().NoError(err)

	trimPopData := log3.ToData()
	// Must have entries [3, 4], NextSeq 5
	s.NotNil(trimPopData.Entries)
	s.Len(trimPopData.Entries, 2)
	s.Equal(uint64(3), trimPopData.Entries[0].Seq)
	s.Equal(uint64(4), trimPopData.Entries[1].Seq)
	s.Equal(uint64(5), trimPopData.NextSeq)

	// LoadLog MUST accept (no contiguity check demanding start at 1)
	trimPopLoaded, err := record.LoadLog(trimPopData)
	s.Require().NoError(err)
	next, err = trimPopLoaded.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(5), next)

	// Behavior identical after reload
	all3Pre, err := log3.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	allLoaded, err := trimPopLoaded.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Equal(seqs(all3Pre), seqs(allLoaded), "post-trim populated must be behavior-identical after reload")
}

func (s *RecordSuite) TestPersistenceGoldenJSON() {
	const (
		alice = "alice"
		kind  = "kind"
		kindX = "x"
	)
	// Populated log with ONE entry
	_, err := s.log.Append(&record.AppendInput{
		At: 7, Correlation: "c1",
		Audience: []core.EntityID{alice},
		Tags:     map[string]string{kind: kindX},
		Payload:  []byte("p"),
	})
	s.Require().NoError(err)

	data := s.log.ToData()
	jsonBytes, err := json.Marshal(data)
	s.Require().NoError(err)

	// Exact golden JSON format with payload base64-encoded.
	expectedJSON := `{"next_seq":2,"entries":[{"seq":1,"at":7,"correlation":"c1",` +
		`"audience":["alice"],"tags":{"kind":"x"},"payload":"cA=="}]}`
	s.JSONEq(expectedJSON, string(jsonBytes), "populated log must marshal to exact golden JSON")

	// Trimmed-empty marshals {"next_seq":3}
	trimLog, err := record.NewLog()
	s.Require().NoError(err)
	_, err = trimLog.Append(&record.AppendInput{Payload: []byte("a")})
	s.Require().NoError(err)
	_, err = trimLog.Append(&record.AppendInput{Payload: []byte("b")})
	s.Require().NoError(err)
	_, err = trimLog.TrimBefore(&record.TrimBeforeInput{Seq: 3})
	s.Require().NoError(err)

	trimData := trimLog.ToData()
	trimJSON, err := json.Marshal(trimData)
	s.Require().NoError(err)
	trimExpected := `{"next_seq":3}`
	s.JSONEq(trimExpected, string(trimJSON), "trimmed-empty must marshal to {\"next_seq\":3}")

	// Unmarshal populated string → LoadLog → behavior-identical
	var loadedData record.LogData
	err = json.Unmarshal([]byte(expectedJSON), &loadedData)
	s.Require().NoError(err)
	loadedLog, err := record.LoadLog(loadedData)
	s.Require().NoError(err)
	all, err := loadedLog.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Len(all, 1)
	s.Equal(uint64(1), all[0].Seq)
	s.Equal(uint64(7), all[0].At)
	s.Equal("c1", all[0].Correlation)
	s.Equal([]core.EntityID{alice}, all[0].Audience)
	s.Equal(map[string]string{kind: kindX}, all[0].Tags)
	s.Equal([]byte("p"), all[0].Payload)
}

func (s *RecordSuite) TestLoadLogRejectsInvalid() {
	const alice = "alice"
	// Table of rejection cases; each must return ErrInvalidData
	testCases := []struct {
		name string
		data record.LogData
	}{
		{
			name: "non-contiguous seqs [1,3]",
			data: record.LogData{
				NextSeq: 4,
				Entries: []record.EntryData{
					{Seq: 1, Payload: []byte("p")},
					{Seq: 3, Payload: []byte("p")},
				},
			},
		},
		{
			name: "duplicate seqs [1,1]",
			data: record.LogData{
				NextSeq: 2,
				Entries: []record.EntryData{
					{Seq: 1, Payload: []byte("p")},
					{Seq: 1, Payload: []byte("q")},
				},
			},
		},
		{
			name: "out-of-order [2,1]",
			data: record.LogData{
				NextSeq: 3,
				Entries: []record.EntryData{
					{Seq: 2, Payload: []byte("p")},
					{Seq: 1, Payload: []byte("q")},
				},
			},
		},
		{
			name: "zero seq",
			data: record.LogData{
				NextSeq: 1,
				Entries: []record.EntryData{
					{Seq: 0, Payload: []byte("p")},
				},
			},
		},
		{
			name: "non-empty with NextSeq == lastSeq (shortfall)",
			data: record.LogData{
				NextSeq: 1, // last seq is 1
				Entries: []record.EntryData{
					{Seq: 1, Payload: []byte("p")},
				},
			},
		},
		{
			name: "non-empty with NextSeq == lastSeq+2 (slack)",
			data: record.LogData{
				NextSeq: 4, // last seq is 1
				Entries: []record.EntryData{
					{Seq: 1, Payload: []byte("p")},
				},
			},
		},
		{
			name: "empty with NextSeq 1",
			data: record.LogData{
				NextSeq: 1,
				Entries: nil,
			},
		},
		{
			name: "audience with empty ID",
			data: record.LogData{
				NextSeq: 2,
				Entries: []record.EntryData{
					{Seq: 1, Audience: []core.EntityID{""}, Payload: []byte("p")},
				},
			},
		},
		{
			name: "audience with duplicates",
			data: record.LogData{
				NextSeq: 2,
				Entries: []record.EntryData{
					{Seq: 1, Audience: []core.EntityID{alice, alice}, Payload: []byte("p")},
				},
			},
		},
		{
			name: "tag map with empty key",
			data: record.LogData{
				NextSeq: 2,
				Entries: []record.EntryData{
					{Seq: 1, Tags: map[string]string{"": "x"}, Payload: []byte("p")},
				},
			},
		},
		{
			name: "nil payload entry",
			data: record.LogData{
				NextSeq: 2,
				Entries: []record.EntryData{
					{Seq: 1, Payload: nil},
				},
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			_, err := record.LoadLog(tc.data)
			s.ErrorIs(err, record.ErrInvalidData, "must return ErrInvalidData for %s", tc.name)
		})
	}
}

func (s *RecordSuite) TestSnapshotImmunity() {
	const alice = "alice"

	// ToData then Append → snapshot unchanged
	s.appendBeat([]core.EntityID{alice}, map[string]string{"k": "v"}, "b1")
	data1 := s.log.ToData()

	_, err := s.log.Append(&record.AppendInput{Payload: []byte("b2")})
	s.Require().NoError(err)

	data2 := s.log.ToData()
	s.NotEqual(data1, data2, "ToData snapshot must change after Append")
	// Verify data1 still has only 1 entry
	s.Len(data1.Entries, 1)
	s.Equal(uint64(2), data1.NextSeq)

	// Load-side aliasing: mutate the caller's LogData after LoadLog → log unchanged
	log2, err := record.NewLog()
	s.Require().NoError(err)
	_, err = log2.Append(&record.AppendInput{
		At:       42,
		Audience: []core.EntityID{alice},
		Tags:     map[string]string{"k": "v"},
		Payload:  []byte("p"),
	})
	s.Require().NoError(err)
	data3 := log2.ToData()

	// LoadLog first (this makes a deep copy)
	loaded, err := record.LoadLog(data3)
	s.Require().NoError(err)

	// NOW mutate the caller's data3
	data3.NextSeq = 99
	if len(data3.Entries) > 0 {
		data3.Entries[0].Audience = append(data3.Entries[0].Audience, "mutated")
		data3.Entries[0].Tags["mutated"] = "yes"
		data3.Entries[0].Payload = []byte("mutated")
	}

	// Re-query loaded and verify unchanged (deep-copy immunity)
	all, err := loaded.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Len(all, 1)
	s.Equal(uint64(1), all[0].Seq)
	s.Equal([]core.EntityID{alice}, all[0].Audience,
		"loaded entry must have original audience (not mutated)")
	s.Equal(map[string]string{"k": "v"}, all[0].Tags,
		"loaded entry must have original tags (not mutated)")
	s.Equal([]byte("p"), all[0].Payload,
		"loaded entry must have original payload (not mutated)")
}

func TestRecordSuite(t *testing.T) {
	suite.Run(t, new(RecordSuite))
}
