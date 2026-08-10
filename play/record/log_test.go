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

func TestRecordSuite(t *testing.T) {
	suite.Run(t, new(RecordSuite))
}
