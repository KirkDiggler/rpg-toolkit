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
	const alice = "alice"
	out, err := s.log.Append(&record.AppendInput{
		At: 7, Correlation: "act-1",
		Audience: []core.EntityID{alice, "bob"},
		Tags:     map[string]string{"kind": "clock.turn_started", "actor": alice},
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
	const alice = "alice"
	aud := []core.EntityID{alice}
	tags := map[string]string{"k": "v"}
	payload := []byte("p")
	_, err := s.log.Append(&record.AppendInput{
		At: 42, Correlation: "test-corr", Audience: aud, Tags: tags, Payload: payload,
	})
	s.Require().NoError(err)
	aud[0] = "mutated"
	tags["k"] = "mutated"
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

func TestRecordSuite(t *testing.T) {
	suite.Run(t, new(RecordSuite))
}
