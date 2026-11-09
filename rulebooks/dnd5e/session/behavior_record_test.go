// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// savedRecordProbe observes the real delivery boundary; it never coordinates
// a machine, manufactures a record, or changes the supplied stream batch.
type savedRecordProbe struct {
	fakeRecord
	check func([]session.RecordEvent)
}

func (p *savedRecordProbe) PublishRecord(ctx context.Context, events []session.RecordEvent) error {
	p.check(events)
	return p.fakeRecord.PublishRecord(ctx, events)
}

func (s *LaunchSuite) TestBehaviorRecordLiveMatchesCatchupAfterReload() {
	raw, err := os.ReadFile("testdata/behavior-patrol-consumer.yaml")
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	die := &behaviorLaunchDice{}
	probe := &savedRecordProbe{check: func(events []session.RecordEvent) {
		stored, readErr := s.encounters.GetEncounter(context.Background(), "run")
		s.Require().NoError(readErr, "publication follows persistence")
		byID := map[string][]byte{}
		for _, entry := range stored.Log.Entries {
			byID[entry.ID] = entry.Payload
		}
		for _, event := range events {
			s.Equal(string(event.Fields), string(byID[event.ID]), "published record %s must already be saved unchanged", event.ID)
		}
	}}
	cfg := &session.Config{Record: probe, Acts: s.stream, Seats: s.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: die, TurnDriver: session.Driver(),
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters}
	s.mgr, err = session.NewManager(cfg)
	s.Require().NoError(err)
	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{Session: "run", DungeonKey: compiled.Key, Dungeon: &compiled, Party: []string{"alice"}})
	s.Require().NoError(err)
	initial, err := s.mgr.Record(context.Background(), &session.RecordReadInput{Session: "run"})
	s.Require().NoError(err)
	s.Require().NotEmpty(initial.Events)
	cursor := initial.Events[len(initial.Events)-1].ID
	// The constructor's scene-opened seed predates the launch write scope.
	// Compare the subsequent committed live suffix, not a fabricated claim
	// that every initialization record was published as a verb delta.
	probe.batches = nil
	for _, path := range [][]spatial.Position{
		{{X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 2}, {X: 4, Y: 2}, {X: 5, Y: 2}, {X: 6, Y: 2}},
		{{X: 5, Y: 2}, {X: 4, Y: 2}, {X: 3, Y: 2}, {X: 2, Y: 2}, {X: 1, Y: 2}, {X: 0, Y: 2}},
	} {
		_, err = s.mgr.Move(context.Background(), &session.MoveInput{Session: "run", Member: "alice", Path: path})
		s.Require().NoError(err)
	}
	var live []session.RecordEvent
	for _, batch := range probe.batches {
		live = append(live, batch...)
	}
	s.Require().Len(recordOf(live, encounter.BeatBehaviorConfig), 2)
	s.NotEmpty(recordOf(live, encounter.BeatBehaviorDecision))
	s.NotEmpty(recordOf(live, encounter.BeatBehaviorWork))
	// A new manager sees only persisted state. Record reads don't choose or
	// manufacture symbol/decision records to reconstruct a trace.
	s.mgr, err = session.NewManager(cfg)
	s.Require().NoError(err)
	var read []session.RecordEvent
	after := cursor
	for {
		page, readErr := s.mgr.Record(context.Background(), &session.RecordReadInput{Session: "run", AfterID: after, Limit: 7})
		s.Require().NoError(readErr)
		s.False(page.Restarted)
		read = append(read, page.Events...)
		if !page.More {
			break
		}
		s.Require().NotEmpty(page.Events)
		after = page.Events[len(page.Events)-1].ID
	}
	s.Equal(live, read, "all record IDs, fields and correlations match live delivery exactly")
	s.Equal(4, die.calls)
	where, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: "run", Member: "west"})
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 2}, where.Position)
}
