// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestMadKingNoBeatTellsAPartItsReaderDidNotPerceive is perceived speech
// (rpg-project#933) held over a castle route's whole record: the hero at the
// gatehouse, the gate's hail, a swing at gate-1 and the garrison's shouted
// alarm and answers. (The facets of a player's own line are the speech
// suite's.) Every beat is checked against what its readers can have
// perceived, as far as the record itself can say:
//
//   - a creature's `answered` beat and every `table` row carry no words;
//   - `spoke` is its speaker's alone;
//   - a speech version carries a check only with the words, a distance
//     whenever it names no speaker, and no member is written two versions
//     of one utterance;
//   - no player is told a voice by a `heard` beat: they are told a version.
func TestMadKingNoBeatTellsAPartItsReaderDidNotPerceive(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, gatehouseSeat)
	s.struck(kingGate1, false)
	require.NotEmpty(t, s.learners("alarm-raised"), "precondition: the gate raised the alarm")

	versions := map[string][]encounter.MemberID{}
	speech := 0
	log := s.enc.ToData().Log
	audienceOf := func(ref uint32) []encounter.MemberID {
		if ref == 0 {
			return nil
		}
		var out []encounter.MemberID
		for _, id := range log.Audiences[ref-1] {
			out = append(out, encounter.MemberID(id))
		}
		return out
	}
	for _, entry := range log.Entries {
		audience := audienceOf(entry.AudienceRef)
		var b map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		switch b["beat"] {
		case encounter.BeatAnswered:
			if b["creature"] != "" {
				require.Empty(t, b["say"], "seq %d: a creature's answered beat carries no words", entry.Seq)
				require.Empty(t, b["to"], "seq %d: nor whom they were said to", entry.Seq)
			}
		case encounter.BeatTable:
			for _, row := range b["rows"].([]any) {
				require.Empty(t, row.(map[string]any)["say"], "seq %d: a table row carries no words", entry.Seq)
			}
		case encounter.BeatSpoke:
			require.Equal(t, []encounter.MemberID{encounter.MemberID(b["actor"].(string))}, audience,
				"seq %d: spoke is its speaker's own record", entry.Seq)
		case encounter.BeatHeard:
			require.NotEqual(t, encounter.SoundVoice, b["kind"], "seq %d: a player is written a version, not a voice", entry.Seq)
		case encounter.BeatSpeech:
			speech++
			if _, ok := b["check"]; ok {
				require.Contains(t, b, "words", "seq %d: the roll travels only with the words", entry.Seq)
			}
			if _, ok := b["speaker"]; !ok {
				require.Contains(t, b, "hops", "seq %d: an unnamed voice says how far it came", entry.Seq)
			}
			id := b["utterance"].(string)
			for _, who := range audience {
				require.False(t, slices.Contains(versions[id], who), "seq %d: %s has one version of %s", entry.Seq, who, id)
			}
			versions[id] = append(versions[id], audience...)
		}
	}
	require.NotZero(t, speech, "the route said things")
}

// TestMadKingNoBeatTellsAReaderMoreThanTheirVersion is the leak property
// (rpg-project#933) over four castle routes — the gate's alarm, the
// stranger's feast, the guest claim admitted at the gate (its interaction
// and recognitions), the cook's thief caught: no
// beat of any kind gives any reader a line id, a verdict, a fact, a speaker
// or an addressee their own speech versions do not. What a reader may know
// of a line is what their versions say; every other beat is held to it.
func TestMadKingNoBeatTellsAReaderMoreThanTheirVersion(t *testing.T) {
	t.Parallel()
	routes := map[string]func(t *testing.T) *kingScene{
		"the alarm": func(t *testing.T) *kingScene {
			s := openTheCastle(t, gatehouseSeat)
			s.struck(kingGate1, false)
			return s
		},
		"the feast": func(t *testing.T) *kingScene {
			s := openTheScript(t, partyStart(t), nil)
			s.heardTheStranger(true)
			return s
		},
		"the claim at the gate": func(t *testing.T) *kingScene {
			s := openTheCastle(t, gatehouseSeat)
			s.walkTo(s.at(kingCaptain))
			s.said(kingCaptain, "guest", "persuasion", true)
			require.Equal(t, "admitted", s.entrance(), "precondition: the claim admitted")
			return s
		},
		"the thief": func(t *testing.T) *kingScene {
			s := openTheCastle(t, besideTheCook)
			s.said(kingCook, "pilfer", "sleight-of-hand", false)
			return s
		},
	}
	for name, route := range routes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			requireNoBeatTellsMoreThanItsVersion(t, route(t).enc)
		})
	}
}

// heardOf is what one reader's speech versions told them.
type heardOf struct {
	options  map[string]bool
	facts    map[string]bool
	checked  bool
	speakers map[string]bool
	to       map[string]bool
}

// requireNoBeatTellsMoreThanItsVersion walks enc's whole record twice: once
// to gather what each reader's speech versions told them, and once to hold
// every other beat to it.
func requireNoBeatTellsMoreThanItsVersion(t *testing.T, enc *encounter.Encounter) {
	t.Helper()
	log := enc.ToData().Log
	audienceOf := func(ref uint32) []encounter.MemberID {
		if ref == 0 {
			return nil
		}
		var out []encounter.MemberID
		for _, id := range log.Audiences[ref-1] {
			out = append(out, encounter.MemberID(id))
		}
		return out
	}
	str := func(b map[string]any, key string) string {
		v, _ := b[key].(string)
		return v
	}
	told := map[encounter.MemberID]*heardOf{}
	of := func(id encounter.MemberID) *heardOf {
		if told[id] == nil {
			told[id] = &heardOf{options: map[string]bool{}, facts: map[string]bool{}, speakers: map[string]bool{}, to: map[string]bool{}}
		}
		return told[id]
	}
	type beat struct {
		seq      uint64
		audience []encounter.MemberID
		body     map[string]any
	}
	var beats []beat
	speech := 0
	for _, entry := range log.Entries {
		var b map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		audience := audienceOf(entry.AudienceRef)
		if b["beat"] != encounter.BeatSpeech {
			beats = append(beats, beat{entry.Seq, audience, b})
			continue
		}
		speech++
		for _, who := range audience {
			h := of(who)
			if v := str(b, "option"); v != "" {
				h.options[v] = true
			}
			if v := str(b, "fact"); v != "" {
				h.facts[v] = true
			}
			if _, ok := b["check"]; ok {
				h.checked = true
			}
			if v := str(b, "speaker"); v != "" {
				h.speakers[v] = true
			}
			if v := str(b, "to"); v != "" {
				h.to[v] = true
			}
		}
	}
	require.NotZero(t, speech, "the route said things")

	// The beats a line's consequences are written on: what each may carry
	// about the line, only to a reader whose versions say it.
	lineBeats := map[any]bool{
		encounter.BeatAnswered: true, encounter.BeatTable: true, encounter.BeatSpoke: true,
		encounter.BeatRecognized: true, encounter.BeatInteraction: true,
	}
	for _, b := range beats {
		if !lineBeats[b.body["beat"]] {
			continue
		}
		for _, who := range b.audience {
			h := of(who)
			if v := str(b.body, "option"); v != "" {
				require.True(t, h.options[v], "seq %d %s: %s is told the line %q without catching it", b.seq, b.body["beat"], who, v)
			}
			if v := str(b.body, "fact"); v != "" {
				require.True(t, h.facts[v], "seq %d %s: %s is told the fact %q without catching it", b.seq, b.body["beat"], who, v)
			}
			if _, ok := b.body["beaten"]; ok {
				require.True(t, h.checked, "seq %d %s: %s is told a verdict without catching a roll", b.seq, b.body["beat"], who)
			}
			if k := str(b.body, "key"); k == string(encounter.AnswerSuccess) || k == string(encounter.AnswerFailure) {
				require.True(t, h.checked, "seq %d %s: %s is told how a line went by its table", b.seq, b.body["beat"], who)
			}
			require.Empty(t, b.body["say"], "seq %d %s: no words outside a version", b.seq, b.body["beat"])
			// An interaction's `to` is the state it stepped to, not a member.
			if v := str(b.body, "to"); v != "" && b.body["beat"] != encounter.BeatInteraction {
				require.True(t, h.to[v], "seq %d %s: %s is told whom it was said to without seeing them", b.seq, b.body["beat"], who)
			}
			if v := str(b.body, "speaker"); v != "" {
				require.True(t, h.speakers[v], "seq %d %s: %s is told who spoke without seeing them", b.seq, b.body["beat"], who)
			}
		}
	}
}
