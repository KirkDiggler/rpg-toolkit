// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"bytes"
	"context"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestASecondRolesStepIsRecordOnly: the patrol consumer's guards run
// `investigate` beside their patrol body; here its first step fires on
// `time`, so walking makes it step (rpg-project#1001). The record carries
// the guard's role_state beat; no act on any stream comes of it, and none
// is undecoded; the walk is refused nothing.
func (s *LaunchSuite) TestASecondRolesStepIsRecordOnly() {
	raw, err := os.ReadFile("testdata/behavior-patrol-consumer.yaml")
	s.Require().NoError(err)
	heard := []byte("            - on: heard\n              when: {fact: {name: heard.kind, op: eq, value: clang}}\n              to: active\n")
	s.Require().Equal(1, bytes.Count(raw, heard), "precondition: investigate waits on a clang")
	raw = bytes.Replace(raw, heard, []byte("            - on: time\n              to: active\n"), 1)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	record := &fakeRecord{}
	s.mgr, err = session.NewManager(&session.Config{Record: record, Acts: s.stream, Seats: s.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &behaviorLaunchDice{}, TurnDriver: session.Driver(),
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters})
	s.Require().NoError(err)
	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{Session: "run", DungeonKey: compiled.Key, Dungeon: &compiled, Party: []string{"alice"}})
	s.Require().NoError(err)
	for _, path := range [][]spatial.Position{
		{{X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 2}, {X: 4, Y: 2}, {X: 5, Y: 2}, {X: 6, Y: 2}},
		{{X: 5, Y: 2}, {X: 4, Y: 2}, {X: 3, Y: 2}, {X: 2, Y: 2}, {X: 1, Y: 2}, {X: 0, Y: 2}},
	} {
		_, err = s.mgr.Move(context.Background(), &session.MoveInput{Session: "run", Member: "alice", Path: path})
		s.Require().NoError(err, "a role's step refuses nothing")
	}

	read, err := s.mgr.Record(context.Background(), &session.RecordReadInput{Session: "run"})
	s.Require().NoError(err)
	steps := recordOf(read.Events, encounter.BeatRoleState)
	s.Require().NotEmpty(steps, "the record carries the role's step")
	ids := map[string]bool{}
	for _, e := range steps {
		ids[e.ID] = true
	}
	for _, act := range s.stream.published {
		s.Empty(act.Undecoded, "no act is undecoded: %+v", act)
		s.False(ids[act.ID], "no stream carries an act of the role's step: %+v", act)
	}
}
