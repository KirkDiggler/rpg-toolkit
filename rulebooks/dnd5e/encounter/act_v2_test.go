// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// act_v2_test.go pins the act's v1alpha2 name (rpg-project#628,
// ideas/event-stream R27): every act is minted with both `<verb>:<first
// seq>` and `<verb>:<id>`, the second drawn from the record's own id space
// at open, and every beat of the act carries it as the record tag
// encounter.ActTag. Both names travel together through a reopen and a pause.

import (
	"context"
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

// TestOpenActMintsBothNames: one OpenAct mints the v1 id and the v2 id; the
// act's beats carry the first as their correlation and the second as their
// act tag; the v2 id is the act's own, sorting before its first beat's id.
func (s *ActTestSuite) TestOpenActMintsBothNames() {
	enc := s.pair(&downList{})

	out, err := enc.OpenAct(&encounter.OpenActInput{Verb: "attack"})
	s.Require().NoError(err)
	s.True(strings.HasPrefix(out.ID, "attack:"))
	s.Len(strings.TrimPrefix(out.ID, "attack:"), 26)
	s.Equal(out.ID, enc.CurrentActID())

	s.missAt(enc)
	s.Require().NoError(enc.CloseAct())
	s.Empty(enc.CurrentActID())

	missed := beatsNamed(s.whole(enc), string(encounter.OutcomeMissed))
	s.Require().Len(missed, 1)
	s.Equal(out.Act, missed[0].Correlation)
	s.Equal(out.ID, missed[0].Tags[encounter.ActTag])
	s.Less(strings.TrimPrefix(out.ID, "attack:"), missed[0].ID, "the act is named before its first beat is written")
}

// TestBeatOutsideActCarriesNoActTag: the scene opening is nobody's act.
func (s *ActTestSuite) TestBeatOutsideActCarriesNoActTag() {
	enc := s.pair(&downList{})

	opened := beatsNamed(s.whole(enc), encounter.BeatSceneOpened)
	s.Require().Len(opened, 1)
	_, tagged := opened[0].Tags[encounter.ActTag]
	s.False(tagged)
}

// TestReopenCarriesBothNames: a reopen names the act under both forms; one
// reopened without its v2 id (saved before v2 ids) stamps no act tag; a v2
// id with no act is refused.
func (s *ActTestSuite) TestReopenCarriesBothNames() {
	enc := s.pair(&downList{})

	_, err := enc.OpenAct(&encounter.OpenActInput{Act: "move:41", ID: "move:01HF7YAT003RVQKEBZ99Y1AF3E"})
	s.Require().NoError(err)
	s.missAt(enc)
	s.Require().NoError(enc.CloseAct())

	_, err = enc.OpenAct(&encounter.OpenActInput{Act: "move:42"})
	s.Require().NoError(err)
	s.missAt(enc)
	s.Require().NoError(enc.CloseAct())

	missed := beatsNamed(s.whole(enc), string(encounter.OutcomeMissed))
	s.Require().Len(missed, 2)
	s.Equal("move:41", missed[0].Correlation)
	s.Equal("move:01HF7YAT003RVQKEBZ99Y1AF3E", missed[0].Tags[encounter.ActTag])
	s.Equal("move:42", missed[1].Correlation)
	_, tagged := missed[1].Tags[encounter.ActTag]
	s.False(tagged, "a legacy reopen has no v2 name to stamp")

	_, err = enc.OpenAct(&encounter.OpenActInput{ID: "move:01HF7YAT003RVQKEBZ99Y1AF3E"})
	s.ErrorIs(err, encounter.ErrInvalidData)
}

// TestPauseKeepsV2Act: a walk held by a window keeps its v2 id and its outer
// act's v2 id through a save, and the cells walked after the answer carry
// the same act tag as the cells before it.
func (s *ActTestSuite) TestPauseKeepsV2Act() {
	loaded, _, _ := s.pausedWalk()
	walkID := loaded.PausedActID()
	s.True(strings.HasPrefix(walkID, "move:"), "the reloaded pause names its v2 act: %q", walkID)

	data := loaded.ToData()
	s.Require().NotNil(data.Pause)
	s.Equal(walkID, data.Pause.ActID)
	s.Require().Len(data.Pause.OuterIDs, 1)
	s.True(strings.HasPrefix(data.Pause.OuterIDs[0], "end_turn:"))

	_, err := loaded.OpenAct(&encounter.OpenActInput{Act: "react:999", ID: "react:01HF7YAT003RVQKEBZ99Y1AF3E"})
	s.Require().NoError(err)
	_, err = loaded.Resume(context.Background())
	s.Require().NoError(err)
	s.Require().NoError(loaded.CloseAct())

	moved := beatsNamed(s.whole(loaded), encounter.BeatMoved)
	var walked int
	for _, m := range moved {
		if strings.Contains(string(m.Payload), `"member":"`+string(goblin)+`"`) {
			walked++
			s.Equal(walkID, m.Tags[encounter.ActTag], "every cell of the walk, before and after the window")
		}
	}
	s.Equal(len(closeIn), walked)
}

// TestPauseWithoutV2IDsStillLoads: a pause saved before v2 ids resumes with
// no act tag (degraded, not refused); outer v2 ids that do not match the
// outer acts are refused.
func (s *ActTestSuite) TestPauseWithoutV2IDsStillLoads() {
	loaded, _, _ := s.pausedWalk()
	data := loaded.ToData()
	s.Require().NotNil(data.Pause)

	legacy := data
	pause := *data.Pause
	pause.ActID, pause.OuterIDs = "", nil
	legacy.Pause = &pause
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: legacy, Capabilities: s.pauseCapabilities()})
	s.Require().NoError(err)

	bad := data
	mismatched := *data.Pause
	mismatched.OuterIDs = append(append([]string(nil), mismatched.OuterIDs...), "extra:01HF7YAT003RVQKEBZ99Y1AF3E")
	bad.Pause = &mismatched
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: bad, Capabilities: s.pauseCapabilities()})
	s.ErrorIs(err, encounter.ErrInvalidData)

	orphan := data
	noAct := *data.Pause
	noAct.Act, noAct.Outer, noAct.OuterIDs = "", nil, nil
	orphan.Pause = &noAct
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: orphan, Capabilities: s.pauseCapabilities()})
	s.ErrorIs(err, encounter.ErrInvalidData)
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

// TestRecordEquipWithoutActMintsV2: an equip with no act open names itself
// under both forms, as every other act does.
func (s *ActTestSuite) TestRecordEquipWithoutActMintsV2() {
	enc := s.equipScene()

	out, err := enc.RecordEquip(swap)
	s.Require().NoError(err)
	beats := beatsNamed(s.whole(enc), encounter.BeatEquipmentChanged)
	s.Require().Len(beats, 2)
	act := beats[0].Tags[encounter.ActTag]
	s.True(strings.HasPrefix(act, "equip:"), "got %q", act)
	for _, b := range beats {
		s.Equal(out.Correlation, b.Correlation)
		s.Equal(act, b.Tags[encounter.ActTag])
	}
}
