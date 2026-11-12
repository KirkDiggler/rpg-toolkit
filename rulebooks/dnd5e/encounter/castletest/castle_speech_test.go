// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
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
// (rpg-project#933) over five castle routes — the gate's alarm, the
// stranger's feast, the guest claim admitted at the gate (its interaction
// and recognitions), the same claim whispered past a listener who sees it,
// the cook's thief caught: no
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
		// The guest claim WHISPERED, with a listener seated off the captain
		// who sees it said and catches none of it: the route a line's grant
		// told to its witnesses would fail (rpg-project#933, L4).
		"the claim whispered": func(t *testing.T) *kingScene {
			s := openTheCastleAs(t, castleHost{at: gatehouseSeat, loudness: map[string]string{"guest": "whisper"}})
			s.walkTo(s.at(kingCaptain))
			s.joinListener()
			s.said(kingCaptain, "guest", "persuasion", true)
			require.Equal(t, "admitted", s.entrance(), "precondition: the claim admitted")
			require.Empty(t, s.saidBy(listener, hero), "precondition: the listener caught none of it")
			saw := false
			for _, b := range s.beatsOf(listener, encounter.BeatSpeech) {
				saw = saw || (b.Speaker == string(hero) && b.Words == "")
			}
			require.True(t, saw, "precondition: but saw the hero say it")
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

// spokenLine is one utterance as the whole record holds it: its first seq
// (the id names it), its stamp, who said it and to whom (from any version
// that names them), whether it is a character's line, and each reader's
// version.
type spokenLine struct {
	seq      uint64
	at       uint64
	speaker  string
	to       string
	line     bool
	versions map[encounter.MemberID]map[string]any
}

// requireNoBeatTellsMoreThanItsVersion holds every beat a line's
// consequences are written on to the reader's own version OF THAT LINE
// (rpg-project#933): the governing line of a beat is the last character's
// line said before it, and the utterance id links its versions. Each
// reader may be told the line's id, verdict, topic, holder or fact only if
// their version of that utterance carries it; a pick that spoke names no
// target; and a line's deed is held only by a reader who caught a beaten
// check of its doer's.
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
	type beat struct {
		seq      uint64
		at       uint64
		audience []encounter.MemberID
		body     map[string]any
	}
	utterances := map[string]*spokenLine{}
	var beats []beat
	for _, entry := range log.Entries {
		var b map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		audience := audienceOf(entry.AudienceRef)
		if b["beat"] != encounter.BeatSpeech {
			beats = append(beats, beat{entry.Seq, entry.At, audience, b})
			continue
		}
		id := str(b, "utterance")
		u := utterances[id]
		if u == nil {
			seq, err := strconv.ParseUint(strings.TrimPrefix(id, "u"), 10, 64)
			require.NoError(t, err, "an utterance id names its first seq")
			u = &spokenLine{seq: seq, at: entry.At, versions: map[encounter.MemberID]map[string]any{}}
			utterances[id] = u
		}
		if v := str(b, "speaker"); v != "" {
			u.speaker = v
		}
		if v := str(b, "to"); v != "" {
			u.to = v
		}
		if str(b, "option") != "" {
			u.line = true
		}
		for _, who := range audience {
			_, twice := u.versions[who]
			require.False(t, twice, "seq %d: %s has one version of %s", entry.Seq, who, id)
			u.versions[who] = b
		}
	}
	require.NotEmpty(t, utterances, "the route said things")
	var lines []*spokenLine
	for _, u := range utterances {
		if u.line {
			lines = append(lines, u)
		}
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].seq < lines[j].seq })
	// governing is the line a beat at seq was written for, and the seq the
	// next line begins at (its window's end).
	governing := func(seq uint64) (*spokenLine, uint64) {
		var at *spokenLine
		end := uint64(math.MaxUint64)
		for _, l := range lines {
			if l.seq < seq {
				at = l
				continue
			}
			end = l.seq
			break
		}
		return at, end
	}
	caughtFact := func(who encounter.MemberID, fact string, from, to uint64) bool {
		for _, u := range utterances {
			if u.seq < from || u.seq >= to {
				continue
			}
			if v := u.versions[who]; v != nil && v["words"] != nil && (fact == "" || str(v, "fact") == fact) && v["fact"] != nil {
				return true
			}
		}
		return false
	}

	lineBeats := map[any]bool{
		encounter.BeatAnswered: true, encounter.BeatTable: true, encounter.BeatSpoke: true,
		encounter.BeatRecognized: true, encounter.BeatInteraction: true,
	}
	for _, b := range beats {
		kind := b.body["beat"]
		if !lineBeats[kind] {
			continue
		}
		if kind == encounter.BeatSpoke {
			require.Equal(t, []encounter.MemberID{encounter.MemberID(str(b.body, "actor"))}, b.audience,
				"seq %d: spoke is its speaker's own record", b.seq)
			continue
		}
		require.Empty(t, b.body["say"], "seq %d %s: no words outside a version", b.seq, kind)
		if kind == encounter.BeatAnswered && str(b.body, "target") != "" {
			_, spoke := utterances["u"+strconv.FormatUint(b.seq+1, 10)]
			require.False(t, spoke, "seq %d: a pick that said something names no target", b.seq)
		}
		line, end := governing(b.seq)
		// A recognition stamped with its line's own reading, inside its
		// window, is that line's grant or the interaction it moved.
		lineGrant := kind == encounter.BeatRecognized && line != nil && b.at == line.at
		for _, who := range b.audience {
			var v map[string]any
			if line != nil {
				v = line.versions[who]
			}
			caught := v != nil && v["words"] != nil
			where := fmt.Sprintf("seq %d %s to %s", b.seq, kind, who)
			if o := str(b.body, "option"); o != "" {
				require.True(t, caught && str(v, "option") == o, "%s: told the line %q without catching it", where, o)
			}
			if _, ok := b.body["beaten"]; ok {
				require.True(t, caught && v["check"] != nil, "%s: told a verdict without catching its roll", where)
			}
			if k := str(b.body, "key"); k == string(encounter.AnswerSuccess) || k == string(encounter.AnswerFailure) {
				require.True(t, caught && v["check"] != nil, "%s: told how the line went by its table", where)
			}
			if f := str(b.body, "fact"); f != "" {
				require.True(t, line != nil && caughtFact(who, f, line.seq, end), "%s: told the fact %q without catching it", where, f)
			}
			if str(b.body, "word") == "fact" {
				require.True(t, line != nil && caughtFact(who, "", line.seq, end), "%s: told a fact was taught without catching it", where)
			}
			if lineGrant {
				require.True(t, caught, "%s: told the line's grant (topic %q) without catching the line", where, str(b.body, "topic"))
				if h := str(b.body, "holder"); h != "" && h != string(who) && h == line.to {
					require.Equal(t, h, str(v, "to"), "%s: told who holds it without seeing them", where)
				}
			}
			if to := str(b.body, "to"); to != "" && kind == encounter.BeatAnswered {
				require.Fail(t, "a pick names nobody it was said to", where)
			}
		}
	}

	// THE LINE'S DEEDS: who holds that a doer intimidated or persuaded saw
	// them say a line that worked, and caught its beaten check.
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		holdings, err := enc.View(&encounter.ViewInput{Member: m.ID})
		require.NoError(t, err)
		for _, h := range holdings {
			if h.Channel != deed.Channel {
				continue
			}
			d, err := deed.Decode(h.Payload)
			if err != nil || (d.Verb != encounter.DeedIntimidate && d.Verb != encounter.DeedPersuade) {
				continue
			}
			caught := false
			for _, u := range lines {
				v := u.versions[m.ID]
				// The deed names its doer: only a reader who saw them say it.
				if u.speaker != string(d.Actor) || v == nil || v["words"] == nil || str(v, "speaker") != string(d.Actor) {
					continue
				}
				if check, ok := v["check"].(map[string]any); ok && check["beaten"] == true {
					caught = true
				}
			}
			require.True(t, caught, "%s holds that %s did %s without seeing them say a line that worked", m.ID, d.Actor, d.Verb)
		}
	}
}

// TestMadKingGateGuardsAnswerShoutsNotTalk is the operator's ruling on
// rpg-project#930 at the gate (rpg-project#933): the hero greets the captain
// in gate-1's earshot and gate-1 stays posted — talk is not a disturbance;
// the same line shouted turns him alert, as any clang would.
func TestMadKingGateGuardsAnswerShoutsNotTalk(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		loudness string
		state    string
	}{{"", "posted"}, {"shout", "alert"}} {
		t.Run("greeted at "+tc.loudness, func(t *testing.T) {
			t.Parallel()
			s := openTheCastleAs(t, castleHost{at: gatehouseSeat, loudness: map[string]string{"greet": tc.loudness}})
			s.walkTo(s.at(kingCaptain))
			require.Equal(t, "posted", machineState(t, s.enc, kingGate1), "precondition: gate-1 on his post")
			_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
				Range: speakReach, Actor: hero, Target: kingCaptain, OptionID: "greet", Roller: rollsOne{},
			})
			require.NoError(t, err, "a plain line")
			s.aRoundPasses() // what he heard is answered on the world's round
			heard := false
			for _, b := range s.beatsOf(kingGate1, encounter.BeatSpeech) {
				heard = heard || b.Speaker == string(hero) || b.Words != ""
			}
			require.True(t, heard, "precondition: gate-1 perceived the hero's line")
			require.Equal(t, tc.state, machineState(t, s.enc, kingGate1))
		})
	}
}
