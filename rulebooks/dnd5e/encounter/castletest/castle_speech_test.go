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
