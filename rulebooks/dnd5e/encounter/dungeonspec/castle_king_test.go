// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_test.go is THE MAD KING'S CASTLE, PLAYED (rpg-project#605):
// the one studio document, compiled, opened the way a host opens it, and
// driven one route at a time. The routes live beside this file
// (castle_king_*_test.go); this file is the host they share.
//
// THE COMPILED CONTENT IS WHAT PLAYS. Every creature enters with the table,
// options, arrival, temper and holdings the file compiled for it. No test
// swaps a table or adds a line: a route that needs one the file does not
// carry is a finding, not a fixture edit.
//
// THE HOST IS DELIBERATELY PLAIN. Everyone sees sixty feet and hears with a
// passive Perception of 10; everyone walks thirty feet and swings a spear
// at five. The world's die always rolls 1, so the first row a table offers is
// the one that fires and the first answer a line has is the one given. A
// temper is a soldier whatever word was dealt: what a temper means is the
// rulebook's, and no route here is about one.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	kingFixture = "testdata/world-builder-v4-castle.yaml"

	// hero is the party's one player.
	hero = encounter.MemberID("hero")
)

// The castle's cast, by the ids the file gives them.
const (
	kingCaptain = encounter.MemberID("captain")
	kingGate1   = encounter.MemberID("gate-1")
	kingGate2   = encounter.MemberID("gate-2")
	kingHall1   = encounter.MemberID("hall-1")
	kingCook    = encounter.MemberID("cook")
	kingKeep1   = encounter.MemberID("keep-1")
	kingKeep2   = encounter.MemberID("keep-2")
	theKing     = encounter.MemberID("king")
	kingReserve = encounter.MemberID("reserve-1")
)

// The castle's factions, as the file names them.
const (
	garrison  = encounter.FactionID("garrison")
	loyalists = encounter.FactionID("loyalists")
)

// kingHallDoor is the courtyard's door into the great hall, by the compiled
// id of the studio's wall door. Shut, as the file authors it.
const kingHallDoor = encounter.DoorID("the-mad-king/hall-wall-door")

// castleCell is the dungeon-absolute cell of an offset [col, row] on the
// studio's floor: the frame the keep fixture places its rooms in, and the
// frame [encounter.Encounter.Members] answers positions in.
func castleCell(col, row int) spatial.Position {
	return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
}

// Where routes begin, as offset cells for [encounter.MemberInput.Position].
var (
	// besideTheCook is the kitchen floor beside the cook, where every route
	// that starts with the kitchen's gossip begins.
	besideTheCook = spatial.Position{X: -19, Y: -12}

	// besideTheKing is the throne room floor beside the king.
	besideTheKing = spatial.Position{X: 1, Y: -22}
)

// kingSpear is the one weapon everybody in this host swings.
var kingSpear = core.Ref{Module: "dnd5e", Type: "weapons", ID: "spear"}

// kingScene is one run of the castle and what the host recorded of it.
type kingScene struct {
	t        *testing.T
	enc      *encounter.Encounter
	compiled dungeonspec.Compiled
	strikes  *[]cellsStrike
	standing *castleStanding
}

// openTheCastle compiles the file and opens it with the hero at a cell and
// every creature where the file posts it, carrying what the file compiled.
func openTheCastle(t *testing.T, at spatial.Position) *kingScene {
	t.Helper()
	compiled := loadContent(t, kingFixture)

	members := []encounter.MemberInput{{ID: hero, Kind: encounter.KindPlayer, Position: at}}
	for _, m := range compiled.Monsters {
		var holds []encounter.IntelID
		for _, h := range m.Holds {
			holds = append(holds, encounter.IntelID(h))
		}
		members = append(members, encounter.MemberInput{
			ID:       encounter.MemberID(m.MemberID),
			Kind:     encounter.KindMonster,
			Position: m.At,
			Faction:  encounter.FactionID(m.Faction),
			Table:    m.Table,
			Options:  m.Options,
			Arrives:  m.Arrives,
			Temper:   soldierProfiles(m.Temper),
			Holds:    holds,
		})
	}

	strikes := &[]cellsStrike{}
	standing := &castleStanding{down: map[encounter.MemberID]bool{}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     compiled.Field,
		Members:   members,
		Endings:   append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: encounter.Capabilities{
			Sight:         castleSight{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        castleSheets{},
			Participation: standing,
			Initiative:    orderAsGiven{},
			Driver:        encounter.TableDriver{Roller: rollsOne{}},
			Roller:        rollsOne{},
			CheckResolver: nothingIsEverFound{},
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   cellsStriker{calls: strikes},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return &kingScene{t: t, enc: enc, compiled: compiled, strikes: strikes, standing: standing}
}

// soldierProfiles completes a compiled temper the way a host does, with the
// profile every dealt word means — here, a soldier's for every word.
func soldierProfiles(temper encounter.Temper) encounter.Temper {
	if len(temper.Mix) == 0 {
		return temper
	}
	temper.Profiles = make(map[string]encounter.TemperProfile, len(temper.Mix))
	for word := range temper.Mix {
		temper.Profiles[word] = encounter.TemperProfile{}
	}

	return temper
}

// speak says one of a creature's lines with the hero's check already rolled.
func (s *kingScene) speak(target encounter.MemberID, option, skill string, beaten bool) (*encounter.SpeakOutput, error) {
	s.t.Helper()

	return s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Actor: hero, Target: target, OptionID: option, Skill: skill,
		Beaten: beaten, DC: 15, Total: map[bool]int{true: 20, false: 3}[beaten], Roller: rollsOne{},
	})
}

// said is speak that must be accepted.
func (s *kingScene) said(target encounter.MemberID, option, skill string, beaten bool) {
	s.t.Helper()
	_, err := s.speak(target, option, skill, beaten)
	require.NoError(s.t, err, "%s offers %q", target, option)
}

// walkTo walks the hero one cell at a time along the pathfinder's own route
// toward a cell, until the hero stands beside it or a fight takes the walk.
func (s *kingScene) walkTo(to spatial.Position) {
	s.t.Helper()
	route, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveToward, Anchor: to, Budget: 200})
	require.NoError(s.t, err)
	require.Empty(s.t, route.StoppedBy, "the hero has a way to %v", to)
	for i, cell := range route.Path {
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: i == len(route.Path)-1})
		require.NoError(s.t, err)
		if clockKind(s.t, s.enc, hero) == encounter.ClockTurn {
			return
		}
	}
}

// open opens a door the hero is standing within reach of.
func (s *kingScene) open(door encounter.DoorID) {
	s.t.Helper()
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: door, Actor: hero})
	require.NoError(s.t, err, "the hero reaches %s", door)
}

// partyStart is the file's own party start, as an offset cell.
func partyStart(t *testing.T) spatial.Position {
	t.Helper()
	compiled := loadContent(t, kingFixture)
	require.NotEmpty(t, compiled.PartyStart)

	return compiled.PartyStart[0].At
}

// at is where a member stands now.
func (s *kingScene) at(id encounter.MemberID) spatial.Position {
	s.t.Helper()
	members, err := s.enc.Members()
	require.NoError(s.t, err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	require.FailNowf(s.t, "not in the run", "%s", id)

	return spatial.Position{}
}

// learners is every member who has learned a fact, by the journal's subject.
func (s *kingScene) learners(fact string) map[encounter.MemberID]bool {
	s.t.Helper()
	out := map[encounter.MemberID]bool{}
	for _, f := range s.enc.ToData().Journal.Facts {
		if f.Kind == "known:fact:"+fact {
			out[encounter.MemberID(f.Subject)] = true
		}
	}

	return out
}

// answeredWith is every creature the audience saw answer with a row that
// teaches a fact, in story order: who SAID it, which the journal of who
// learned it does not record.
func (s *kingScene) answeredWith(fact string, audience encounter.MemberID) []encounter.MemberID {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: audience})
	require.NoError(s.t, err)
	var out []encounter.MemberID
	for _, beat := range story {
		var body struct {
			Beat     string `json:"beat"`
			Creature string `json:"creature"`
			Fact     string `json:"fact"`
		}
		require.NoError(s.t, json.Unmarshal(beat.Payload, &body))
		if body.Beat == encounter.BeatAnswered && body.Fact == fact {
			out = append(out, encounter.MemberID(body.Creature))
		}
	}

	return out
}

// stance is the declared-plus-known stance between two factions now.
func (s *kingScene) stance(a, b encounter.FactionID) encounter.Stance {
	s.t.Helper()
	out, err := s.enc.Stance(a, b)
	require.NoError(s.t, err)

	return out
}

// hostile is whether the graph holds two members enemies now.
func (s *kingScene) hostile(a, b encounter.MemberID) bool {
	s.t.Helper()
	out, known := s.enc.IsHostile(a, b)
	require.True(s.t, known, "%s and %s are both in the run", a, b)

	return out
}

// ending is the run's ending, or "" while it is open.
func (s *kingScene) ending() string {
	s.t.Helper()
	status, err := s.enc.Status()
	require.NoError(s.t, err)
	if status.Open || status.Outcome == nil {
		return ""
	}

	return status.Outcome.Ending
}

// struck lands one swing of the hero's: a hit, and a fall when it drops.
func (s *kingScene) struck(target encounter.MemberID, drops bool) {
	s.t.Helper()
	if drops {
		s.standing.down[target] = true
	}
	_, err := s.enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: hero, Targets: []encounter.MemberID{target},
		Attack: &encounter.AttackIdentity{Ref: kingSpear.String(), Name: "Spear", DamageType: "piercing"},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 6},
	}}}})
	require.NoError(s.t, err)
}

// endTurn ends the hero's turn in a fight, and every driven turn after it
// runs before this returns.
func (s *kingScene) endTurn() {
	s.t.Helper()
	_, err := s.enc.EndTurn(&encounter.EndTurnInput{Member: hero})
	require.NoError(s.t, err)
}

// castleSpeedFeet is everybody's walking speed in this host.
const castleSpeedFeet = 30

// castleSight is a torchlit castle: everybody sees sixty feet.
type castleSight struct{}

func (castleSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = encounter.CellsFromFeet(60)
	}

	return out, nil
}

// castleSheets is everybody's sheet in this host: thirty feet, a spear at
// five, and a passive Perception of 10.
type castleSheets struct{}

func (castleSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{
			SpeedFeet:         castleSpeedFeet,
			Actions:           []encounter.ActionView{{Ref: kingSpear, RangeFeet: 5}},
			PassivePerception: 10,
		}
	}

	return out, nil
}

// castleStanding is the host's answer to who is standing: everybody, until
// a swing the host scored as a fall puts somebody down.
type castleStanding struct{ down map[encounter.MemberID]bool }

func (c *castleStanding) Standing(members []encounter.MemberID) ([]encounter.MemberID, error) {
	var out []encounter.MemberID
	for _, id := range members {
		if c.down[id] {
			out = append(out, id)
		}
	}

	return out, nil
}

func (c *castleStanding) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		assessment.Members = append(assessment.Members, encounter.MemberParticipation{
			Member: id, Contact: !c.down[id], Conscious: !c.down[id], Down: c.down[id],
			Turn: encounter.TurnParticipationWait,
		})
	}

	return assessment, nil
}
