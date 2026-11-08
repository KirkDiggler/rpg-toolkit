// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// AudienceTableSuite pins the persisted audience table: each distinct
// audience is stored once in LogData.Audiences and every entry names its
// audience by reference (rpg-project#896, finding F20).
type AudienceTableSuite struct {
	suite.Suite
}

func TestAudienceTableSuite(t *testing.T) {
	suite.Run(t, new(AudienceTableSuite))
}

// roster returns n distinct member ids shaped like the garrison keep's.
func roster(n int) []core.EntityID {
	out := make([]core.EntityID, n)
	for i := range out {
		out[i] = core.EntityID(fmt.Sprintf("monster-garrison-%04d", i))
	}
	return out
}

func (s *AudienceTableSuite) TestThreeAudiencesInterleavedRoundTrip() {
	everyone := []core.EntityID{"alice", "bob", "carol"}
	pair := []core.EntityID{"bob", "alice"} // order is part of the audience
	solo := []core.EntityID{"carol"}
	// Ten entries over three distinct audiences, interleaved, plus GM-only
	// beats that name no audience at all.
	plan := [][]core.EntityID{everyone, pair, everyone, nil, solo, everyone, pair, solo, nil, everyone}

	log, err := record.NewLog()
	s.Require().NoError(err)
	for i, aud := range plan {
		_, err := log.Append(&record.AppendInput{
			Audience: aud,
			Tags:     map[string]string{"n": fmt.Sprint(i)},
			Payload:  []byte(fmt.Sprintf("beat-%d", i)),
		})
		s.Require().NoError(err)
	}
	before, err := log.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)

	data := log.ToData()
	s.Len(data.Audiences, 3, "each distinct audience is stored once; the GM-only beat takes no row")

	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	var back record.LogData
	s.Require().NoError(json.Unmarshal(raw, &back))

	loaded, err := record.LoadLog(back)
	s.Require().NoError(err)
	after, err := loaded.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Equal(before, after, "same entries, same audiences, same order")

	for i, e := range after {
		if plan[i] == nil {
			s.Nil(e.Audience, "entry %d is GM-only", i)
			continue
		}
		s.Equal(plan[i], e.Audience, "entry %d resolves to its own audience", i)
	}

	// Membership still projects per viewer after the load.
	carol, err := loaded.SliceFor(&record.SliceForInput{Viewer: "carol", FromSeq: 1})
	s.Require().NoError(err)
	seqs := make([]uint64, 0, len(carol))
	for _, e := range carol {
		seqs = append(seqs, e.Seq)
	}
	s.Equal([]uint64{1, 3, 5, 6, 8, 10}, seqs)
}

func (s *AudienceTableSuite) TestTwoAudiencesInterleavedResolveToTheirOwnRow() {
	a := []core.EntityID{"alice"}
	b := []core.EntityID{"bob"}
	log, err := record.NewLog()
	s.Require().NoError(err)
	for _, aud := range [][]core.EntityID{a, b, a, b} {
		_, err := log.Append(&record.AppendInput{Audience: aud, Payload: []byte("p")})
		s.Require().NoError(err)
	}
	data := log.ToData()
	s.Equal([][]core.EntityID{a, b}, data.Audiences, "first-seen order")
	refs := make([]uint32, 0, len(data.Entries))
	for _, e := range data.Entries {
		refs = append(refs, e.AudienceRef)
	}
	s.Equal([]uint32{1, 2, 1, 2}, refs, "a reference is the 1-based table row")
}

// TestRosterWideAudienceIsUnder300BytesPerEntry is the F20 bound: on a
// 100-member keep with 200 roster-wide beats, the serialised record stays
// under 300 bytes per entry. The old inline shape is computed in-test so
// the saving is visible in -v output.
func (s *AudienceTableSuite) TestRosterWideAudienceIsUnder300BytesPerEntry() {
	const (
		members = 100
		entries = 200
		bound   = 300
	)
	everyone := roster(members)
	payload := []byte(`{"moved":{"actor":"monster-garrison-0007","from":[12,4],"to":[13,4]}}`)

	log, err := record.NewLog()
	s.Require().NoError(err)
	for i := 0; i < entries; i++ {
		_, err := log.Append(&record.AppendInput{
			Correlation: fmt.Sprintf("act-%d", i),
			Audience:    everyone,
			Tags:        map[string]string{"kind": "encounter.moved"},
			Payload:     payload,
		})
		s.Require().NoError(err)
	}
	raw, err := json.Marshal(log.ToData())
	s.Require().NoError(err)
	perEntry := len(raw) / entries

	// The old shape repeated the roster in every entry.
	inline, err := json.Marshal(everyone)
	s.Require().NoError(err)
	oldPerEntry := perEntry + len(`,"audience":`) + len(inline) - len(`,"audience":1`)

	s.T().Logf("bytes per entry: %d now, %d with the audience inline", perEntry, oldPerEntry)
	s.Less(perEntry, bound, "a roster-wide entry must cost under %d bytes", bound)
}

func (s *AudienceTableSuite) TestLoadLogFailsClosedOnABadTable() {
	entry := func(ref uint32) []record.EntryData {
		return []record.EntryData{{Seq: 1, AudienceRef: ref, Payload: []byte("p")}}
	}
	cases := map[string]struct {
		data record.LogData
		want string
	}{
		"reference past the table": {
			data: record.LogData{NextSeq: 2, Audiences: [][]core.EntityID{{"alice"}}, Entries: entry(2)},
			want: "audience 2 past the table of 1",
		},
		"reference with no table": {
			data: record.LogData{NextSeq: 2, Entries: entry(1)},
			want: "audience 1 past the table of 0",
		},
		"table row with a duplicate id": {
			data: record.LogData{NextSeq: 2, Audiences: [][]core.EntityID{{"alice", "alice"}}, Entries: entry(1)},
			want: `audiences[0] audience: duplicate "alice"`,
		},
		"table row with an empty id": {
			data: record.LogData{NextSeq: 2, Audiences: [][]core.EntityID{{"alice", ""}}, Entries: entry(1)},
			want: "audiences[0] audience",
		},
		"empty table row": {
			data: record.LogData{NextSeq: 2, Audiences: [][]core.EntityID{{}}, Entries: entry(0)},
			want: "audiences[0] empty",
		},
		"the same audience stored twice": {
			data: record.LogData{
				NextSeq:   2,
				Audiences: [][]core.EntityID{{"alice", "bob"}, {"alice", "bob"}},
				Entries:   entry(1),
			},
			want: "audiences[1] repeats audiences[0]",
		},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			got, err := record.LoadLog(tc.data)
			s.Nil(got)
			s.Require().ErrorIs(err, record.ErrInvalidData)
			s.Contains(err.Error(), tc.want)
		})
	}
}

func (s *AudienceTableSuite) TestAnOldInlineAudienceFailsToDecode() {
	// No backward-compatible load path: the inline array no longer fits the
	// reference, so a stale blob is refused rather than loaded audience-less.
	old := `{"next_seq":2,"entries":[{"seq":1,"audience":["alice"],"payload":"cA=="}]}`
	var data record.LogData
	s.Error(json.Unmarshal([]byte(old), &data))
}

func (s *AudienceTableSuite) TestLoadedEntriesDoNotAliasTheTable() {
	data := record.LogData{
		NextSeq:   3,
		Audiences: [][]core.EntityID{{"alice", "bob"}},
		Entries: []record.EntryData{
			{Seq: 1, AudienceRef: 1, Payload: []byte("p")},
			{Seq: 2, AudienceRef: 1, Payload: []byte("q")},
		},
	}
	loaded, err := record.LoadLog(data)
	s.Require().NoError(err)
	data.Audiences[0][0] = "mallory"

	all, err := loaded.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	all[0].Audience[1] = "mallory" // a caller's copy, never the log's
	again, err := loaded.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	for _, e := range again {
		s.Equal([]core.EntityID{"alice", "bob"}, e.Audience)
	}
}
