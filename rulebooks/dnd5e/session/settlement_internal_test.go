// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// SettlementSuite drives commit's settlement read inside one write scope,
// where a fall and an exit can share an act no public verb puts together.
type SettlementSuite struct {
	suite.Suite

	sessions   *strikeSessions
	encounters *strikeEncounters
	characters *strikeCharacters
	mgr        *Manager
}

func TestSettlementSuite(t *testing.T) { suite.Run(t, new(SettlementSuite)) }

func (s *SettlementSuite) SetupTest() {
	ctx := context.Background()
	s.sessions = &strikeSessions{byID: map[string]*SessionData{}}
	s.encounters = &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	s.characters = &strikeCharacters{byID: map[string]*character.Data{
		"fighter": strikeFixtureFighter("fighter"),
	}}
	mgr, err := NewManager(&Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: &scriptedDice{rolls: []int{10, 10, 10, 10, 10, 10}}, TurnDriver: Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	world, err := encounter.NewEncounter(&encounter.SetupInput{Sheets: encStandStill{},
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: aggregateRecordEveryoneSees{},
		Equipment: encNoHandsObserved{}, Initiative: aggregateRecordOrderAsGiven{},
		TurnDriver: passDriver{}, Standing: aggregateRecordEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)},
		},
		Endings:   []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := world.ToData()
	_, err = mgr.StartSession(ctx, &StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	s.Require().NoError(err)
	_, err = mgr.Join(ctx, &JoinInput{Session: "sess", Member: "fighter",
		Position: encounter.HexCellAt(encounter.HexesArePointyTop(), 2, 0)})
	s.Require().NoError(err)
	_, err = mgr.Spawn(ctx, &SpawnInput{Session: "sess", ID: "goblin", Ref: refs.Monsters.Goblin().String(),
		Position: encounter.HexCellAt(encounter.HexesArePointyTop(), 3, 0)})
	s.Require().NoError(err)
}

// TestAMonsterThatFellAndThenExitedStillGrants: the grant follows the fall,
// and the fall carries the kind the roster held when it happened. A body that
// left the roster later in the same act is still a monster that fell.
func (s *SettlementSuite) TestAMonsterThatFellAndThenExitedStillGrants() {
	ctx := context.Background()
	scope, err := s.mgr.openForChange(ctx, "sess")
	s.Require().NoError(err)

	sheet, ok := npcSheet(scope.data, "goblin")
	s.Require().True(ok)
	s.Require().Positive(sheet.Experience, "the fixture's premise: the goblin is worth something")
	dropped := *sheet
	dropped.HitPoints = 0
	scope.replaceMonsterSheet(&dropped)
	// The composition notices the body on its next consult and tells the fall:
	// any outcome recorded after the sheet says zero is that consult.
	_, err = scope.enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: "fighter", Targets: []encounter.MemberID{"goblin"},
	})
	s.Require().NoError(err)
	_, err = scope.enc.Exit(&encounter.ExitInput{Member: "goblin"})
	s.Require().NoError(err)

	settled, err := scope.enc.Settlement(&encounter.SettlementInput{FromSeq: scope.baseline})
	s.Require().NoError(err)
	fell := false
	for _, fall := range settled.Falls {
		fell = fell || (fall.Member == "goblin" && fall.Kind == encounter.KindMonster)
	}
	s.Require().True(fell, "control: the goblin's fall is in this act's settlement")
	roster, err := scope.enc.Members()
	s.Require().NoError(err)
	for _, member := range roster {
		s.Require().NotEqual(encounter.MemberID("goblin"), member.ID, "control: the goblin has left the roster")
	}

	_, _, err = s.mgr.commit(ctx, scope)
	s.Require().NoError(err)

	s.Equal(sheet.Experience, s.characters.byID["fighter"].Experience,
		"the lone fighter takes the whole worth of the goblin that fell")

	// The beat names the goblin as its actor and its cause: the composition
	// accepts a former member as the actor of an experience beat.
	after, err := s.mgr.openForChange(ctx, "sess")
	s.Require().NoError(err)
	entries, err := after.enc.Story(&encounter.StoryInput{Audience: "fighter"})
	s.Require().NoError(err)
	told := false
	for _, entry := range entries {
		var beat struct {
			Beat       string `json:"beat"`
			Actor      string `json:"actor"`
			Experience struct {
				Member string `json:"member"`
			} `json:"experience"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Beat != string(encounter.OutcomeExperienceGained) {
			continue
		}
		told = true
		s.Equal("goblin", beat.Actor, "the fallen monster acts its own beat")
		s.Equal("goblin", beat.Experience.Member)
	}
	s.True(told, "the grant is told")
}

// TestAnOutputLandsItsAreasOnce: a resumed walk lands its movement output in
// recordMovementResults, and nothing else may land the same output again. An
// opened area landed twice would be refused as already open, so landAreas
// consumes what it applies and a second call for the same output is a no-op.
func (s *SettlementSuite) TestAnOutputLandsItsAreasOnce() {
	ctx := context.Background()
	scope, err := s.mgr.openForChange(ctx, "sess")
	s.Require().NoError(err)
	out := &resolution.Output{OpenedAreas: []encounter.SightAreaInput{{
		ID: "cloud", SourceID: "fighter", Name: "Fog Cloud",
		Center: encounter.HexCellAt(encounter.HexesArePointyTop(), 6, 3), RadiusFeet: 20,
	}}}

	s.Require().NoError(s.mgr.landAreas(scope.enc, scope, out))
	s.Require().NoError(s.mgr.landAreas(scope.enc, scope, out), "the same output landed again changes nothing")
	s.Empty(out.OpenedAreas, "landed changes are consumed")
	clouds := 0
	for _, area := range scope.enc.WorldView().SightAreas {
		if area.ID == "cloud" {
			clouds++
		}
	}
	s.Equal(1, clouds, "the cloud is open once")
}
