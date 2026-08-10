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
}

func (s *RecordSuite) TestAppendNormalizesAndCopies() {
	const alice = "alice"
	aud := []core.EntityID{alice}
	tags := map[string]string{"k": "v"}
	_, err := s.log.Append(&record.AppendInput{Audience: aud, Tags: tags, Payload: []byte("p")})
	s.Require().NoError(err)
	aud[0] = "mutated"
	tags["k"] = "mutated"
	all, err := s.log.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{alice}, all[0].Audience)
	s.Equal(map[string]string{"k": "v"}, all[0].Tags)
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
