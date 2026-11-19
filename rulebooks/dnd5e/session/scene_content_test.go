// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// fieldsOf is a record event's payload.
func (s *LaunchSuite) fieldsOf(e session.RecordEvent) map[string]any {
	var f map[string]any
	s.Require().NoError(json.Unmarshal(e.Fields, &f))
	return f
}

// TestTheRecordNamesTheDungeonItRan (rpg-project#1001): a launched run's
// record says what it is a run of (the dungeon's key, its site hash and
// its content hash) and where each member began, and a member who joins
// later gets its own start cell. No stream carries either beat; a
// catch-up across them loses and repeats nothing; a reload writes no
// second scene_content.
func (s *LaunchSuite) TestTheRecordNamesTheDungeonItRan() {
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/behavior-patrol-consumer.yaml")
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	want, err := compiled.ContentHash()
	s.Require().NoError(err)
	s.mgr, err = session.NewManager(&session.Config{Record: &fakeRecord{}, Acts: s.stream, Seats: s.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &behaviorLaunchDice{}, TurnDriver: session.Driver(),
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters})
	s.Require().NoError(err)
	_, err = s.mgr.Launch(ctx, &session.LaunchInput{Session: "run", DungeonKey: "the-patrol", Dungeon: &compiled, Party: []string{"alice"}})
	s.Require().NoError(err)
	_, err = s.mgr.Join(ctx, &session.JoinInput{Session: "run", Member: "bob", Position: spatial.Position{X: 4, Y: 2}})
	s.Require().NoError(err)
	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "run", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}}})
	s.Require().NoError(err, "a verb after the joins: the run is reloaded and saved")

	read, err := s.mgr.Record(ctx, &session.RecordReadInput{Session: "run"})
	s.Require().NoError(err)
	contents := recordOf(read.Events, encounter.BeatSceneContent)
	s.Require().Len(contents, 1, "one scene_content: a reload writes none")
	content := s.fieldsOf(contents[0])
	s.Equal("the-patrol", content["key"])
	s.Equal(string(want), content["content_hash"], "the compiled dungeon's own content hash")
	s.NotEmpty(content["site_hash"])

	world, err := s.mgr.LoadWorldForTest(ctx, "run")
	s.Require().NoError(err)
	members, err := world.Members()
	s.Require().NoError(err)
	at := map[string]spatial.Position{}
	for _, e := range recordOf(read.Events, encounter.BeatJoinedAt) {
		f := s.fieldsOf(e)
		var cell spatial.Position
		b, _ := json.Marshal(f["cell"])
		s.Require().NoError(json.Unmarshal(b, &cell))
		_, twice := at[f["member"].(string)]
		s.False(twice, "one joined_at per member")
		at[f["member"].(string)] = cell
	}
	s.Len(at, len(members), "every member, the later joiner among them")
	s.Equal(spatial.Position{X: 4, Y: 2}, at["bob"], "the joiner's start cell")
	s.Contains(at, "west")
	s.Contains(at, "court")

	ids := map[string]bool{}
	for _, e := range append(contents, recordOf(read.Events, encounter.BeatJoinedAt)...) {
		ids[e.ID] = true
	}
	for _, act := range s.stream.published {
		s.Empty(act.Undecoded, "no act is undecoded: %+v", act)
		s.False(ids[act.ID], "no stream carries either beat: %+v", act)
	}

	// A CATCH-UP ACROSS THEM, a page at a time, reads every event once.
	var paged []session.RecordEvent
	after := ""
	for {
		page, err := s.mgr.Record(ctx, &session.RecordReadInput{Session: "run", AfterID: after, Limit: 2})
		s.Require().NoError(err)
		if len(page.Events) == 0 {
			break
		}
		paged = append(paged, page.Events...)
		after = page.Events[len(page.Events)-1].ID
	}
	s.Equal(recordIDs(read.Events), recordIDs(paged), "nothing lost, nothing repeated")
}
