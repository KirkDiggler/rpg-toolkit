// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record_test

import (
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

func TestRecordSuite(t *testing.T) {
	suite.Run(t, new(RecordSuite))
}
