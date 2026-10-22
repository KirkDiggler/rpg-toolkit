// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// act_v2_test.go pins the act id (rpg-project#628, #631, ideas/event-stream
// R27): every act is minted as `<verb>:<id>`, drawn from the record's own id
// space at open, and every beat of the act carries it both as its record
// correlation and as the record tag encounter.ActTag. E1's
// `<verb>:<first seq>` form is gone. The id travels through a reopen and a
// pause.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// whole is every retained entry of the record, every audience included.
func (s *ActTestSuite) whole(enc *encounter.Encounter) []record.Entry {
	s.T().Helper()
	out, err := enc.ReadRecord(&encounter.ReadRecordInput{})
	s.Require().NoError(err)
	return out.Entries
}

// beatsNamed is every entry whose payload beat is name.
func beatsNamed(entries []record.Entry, name string) []record.Entry {
	var out []record.Entry
	for _, e := range entries {
		if strings.Contains(string(e.Payload), `"beat":"`+name+`"`) {
			out = append(out, e)
		}
	}
	return out
}

// TestCorrelationIsTheV2ActID: one OpenAct mints `<verb>:<id>`, and the
// act's beats carry that one id as their correlation and as their act tag.
// The id is the act's own, sorting before its first beat's id, and is not
// E1's `<verb>:<first seq>`.
func (s *ActTestSuite) TestCorrelationIsTheV2ActID() {
	enc := s.pair(&downList{})
	next, err := enc.NextStorySeq()
	s.Require().NoError(err)

	out, err := enc.OpenAct(&encounter.OpenActInput{Verb: "attack"})
	s.Require().NoError(err)
	s.True(strings.HasPrefix(out.Act, "attack:"))
	s.Len(strings.TrimPrefix(out.Act, "attack:"), 26, "a record id, not a seq")
	s.Equal(out.Act, enc.CurrentAct())

	s.missAt(enc)
	s.Require().NoError(enc.CloseAct())
	s.Empty(enc.CurrentAct())

	missed := beatsNamed(s.whole(enc), string(encounter.OutcomeMissed))
	s.Require().Len(missed, 1)
	s.Equal(out.Act, missed[0].Correlation)
	s.Equal(out.Act, missed[0].Tags[encounter.ActTag])
	s.NotEqual(fmt.Sprintf("attack:%d", next), missed[0].Correlation, "the seq form is gone")
	s.Less(strings.TrimPrefix(out.Act, "attack:"), missed[0].ID, "the act is named before its first beat is written")
}

// TestBeatOutsideActCarriesNoActTag: the scene opening is nobody's act.
func (s *ActTestSuite) TestBeatOutsideActCarriesNoActTag() {
	enc := s.pair(&downList{})

	opened := beatsNamed(s.whole(enc), encounter.BeatSceneOpened)
	s.Require().Len(opened, 1)
	s.Empty(opened[0].Correlation)
	_, tagged := opened[0].Tags[encounter.ActTag]
	s.False(tagged)
}

// TestReopenCarriesItsID: a reopen names the act by the id it was given, as
// correlation and act tag alike.
func (s *ActTestSuite) TestReopenCarriesItsID() {
	enc := s.pair(&downList{})

	const walk = "move:01HF7YAT003RVQKEBZ99Y1AF3E"
	_, err := enc.OpenAct(&encounter.OpenActInput{Act: walk})
	s.Require().NoError(err)
	s.missAt(enc)
	s.Require().NoError(enc.CloseAct())

	missed := beatsNamed(s.whole(enc), string(encounter.OutcomeMissed))
	s.Require().Len(missed, 1)
	s.Equal(walk, missed[0].Correlation)
	s.Equal(walk, missed[0].Tags[encounter.ActTag])
}

// TestPauseKeepsV2Act: a walk held by a window keeps its id and its outer
// act's id through a save, and the cells walked after the answer carry the
// same act as the cells before it.
func (s *ActTestSuite) TestPauseKeepsV2Act() {
	loaded, _, walkID := s.pausedWalk()
	s.True(strings.HasPrefix(walkID, "move:"), "the reloaded pause names its act: %q", walkID)

	data := loaded.ToData()
	s.Require().NotNil(data.Pause)
	s.Equal(walkID, data.Pause.ActID)
	s.Require().Len(data.Pause.OuterIDs, 1)
	s.True(strings.HasPrefix(data.Pause.OuterIDs[0], "end_turn:"))

	s.resumeUnder(loaded, "react:01HF7YAT003RVQKEBZ99Y1AF3E")
	s.Equal(len(closeIn), s.walkedUnder(loaded, walkID))
}

// TestResumedTurnEndsInsideItsOuterActV2: after a paused and resumed driven
// walk, the goblin's turn-ended and the tick carry the outer end_turn's id
// (Pause.OuterIDs[0]) as their act, whoever resumes.
func (s *ActTestSuite) TestResumedTurnEndsInsideItsOuterActV2() {
	for _, resumer := range []string{"", "react:01HF7YAT003RVQKEBZ99Y1AF3E"} {
		loaded, _, _ := s.pausedWalk()
		data := loaded.ToData()
		s.Require().NotNil(data.Pause)
		s.Require().Len(data.Pause.OuterIDs, 1)
		outer := data.Pause.OuterIDs[0]
		s.Require().True(strings.HasPrefix(outer, "end_turn:"), "got %q", outer)
		before := len(s.whole(loaded))

		s.resumeUnder(loaded, resumer)

		tail := s.whole(loaded)[before:]
		var ended int
		for _, e := range beatsNamed(tail, encounter.BeatTurnEnded) {
			if strings.Contains(string(e.Payload), `"member":"`+string(goblin)+`"`) {
				ended++
				s.Equal(outer, e.Correlation, "resumer %q", resumer)
				s.Equal(outer, e.Tags[encounter.ActTag], "resumer %q", resumer)
			}
		}
		s.Equal(1, ended, "the goblin's paused turn ends on the resume (resumer %q)", resumer)
		ticks := beatsNamed(tail, encounter.BeatTick)
		s.Require().NotEmpty(ticks)
		for _, t := range ticks {
			s.Equal(outer, t.Tags[encounter.ActTag], "the round wraps inside the end_turn (resumer %q)", resumer)
		}
	}
}

// TestPauseRoundTripWithoutSeqForm (R27): a saved pause writes its acts as
// `act_id` and `outer_ids` only, never E1's `act` and `outer`. A pause
// saved before rpg-project#631, which carries both, loads, and its seq
// fields are ignored: the resumed walk carries the act_id. A pause saved
// with no act id at all still loads and resumes with none (degraded, not
// refused).
func (s *ActTestSuite) TestPauseRoundTripWithoutSeqForm() {
	loaded, _, walkID := s.pausedWalk()

	blob, err := json.Marshal(loaded.ToData())
	s.Require().NoError(err)
	var saved map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(blob, &saved))
	var pause map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(saved["pause"], &pause))
	s.Contains(pause, "act_id")
	s.Contains(pause, "outer_ids")
	s.NotContains(pause, "act", "the seq form is never written")
	s.NotContains(pause, "outer", "the seq form is never written")

	// An older save: the seq forms beside the ids.
	pause["act"] = json.RawMessage(`"move:41"`)
	pause["outer"] = json.RawMessage(`["end_turn:40"]`)
	older := s.loadJSON(saved, pause)
	s.Equal(walkID, older.PausedAct(), "the seq form is ignored")
	s.resumeUnder(older, "")
	s.Equal(len(closeIn), s.walkedUnder(older, walkID))

	// A save with no act id at all.
	delete(pause, "act_id")
	delete(pause, "outer_ids")
	bare := s.loadJSON(saved, pause)
	s.Empty(bare.PausedAct())
	s.resumeUnder(bare, "")
}

// loadJSON loads the encounter saved as JSON, with its pause replaced.
func (s *ActTestSuite) loadJSON(saved, pause map[string]json.RawMessage) *encounter.Encounter {
	s.T().Helper()
	p, err := json.Marshal(pause)
	s.Require().NoError(err)
	saved["pause"] = p
	blob, err := json.Marshal(saved)
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(blob, &data))
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.pauseCapabilities()})
	s.Require().NoError(err)
	return enc
}

// resumeUnder resumes the held walk, inside resumer when it names an act.
func (s *ActTestSuite) resumeUnder(enc *encounter.Encounter, resumer string) {
	s.T().Helper()
	if resumer != "" {
		_, err := enc.OpenAct(&encounter.OpenActInput{Act: resumer})
		s.Require().NoError(err)
	}
	_, err := enc.Resume(context.Background())
	s.Require().NoError(err)
	if resumer != "" {
		s.Require().NoError(enc.CloseAct())
	}
}

// walkedUnder counts the goblin's moved beats, asserting each carries act.
func (s *ActTestSuite) walkedUnder(enc *encounter.Encounter, act string) int {
	s.T().Helper()
	var walked int
	for _, m := range beatsNamed(s.whole(enc), encounter.BeatMoved) {
		if strings.Contains(string(m.Payload), `"member":"`+string(goblin)+`"`) {
			walked++
			s.Equal(act, m.Correlation, "every cell of the walk, before and after the window")
			s.Equal(act, m.Tags[encounter.ActTag], "every cell of the walk, before and after the window")
		}
	}
	return walked
}

// pauseCapabilities is what PauseTestSuite.reload loads with.
func (s *ActTestSuite) pauseCapabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        pauseSheets,
		Participation: &downList{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Actors: encounter.Actors{
			Striker:   passStriker{},
			Mover:     &pausingMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

// TestRecordEquipWithoutActMintsV2: an equip with no act open opens its own
// act, `equip:<id>`, as every other act does.
func (s *ActTestSuite) TestRecordEquipWithoutActMintsV2() {
	enc := s.equipScene()

	out, err := enc.RecordEquip(swap)
	s.Require().NoError(err)
	s.True(strings.HasPrefix(out.Correlation, "equip:"), "got %q", out.Correlation)
	s.Len(strings.TrimPrefix(out.Correlation, "equip:"), 26)
	beats := beatsNamed(s.whole(enc), encounter.BeatEquipmentChanged)
	s.Require().Len(beats, 2)
	for _, b := range beats {
		s.Equal(out.Correlation, b.Correlation)
		s.Equal(out.Correlation, b.Tags[encounter.ActTag])
	}
}
