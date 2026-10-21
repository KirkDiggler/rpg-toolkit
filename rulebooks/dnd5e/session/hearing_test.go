// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// hearing_test.go is hearing through the session (rpg-project#569 R4–R5,
// castle plan Task B3): the sheet seam supplies every member's passive
// Perception on every pull, the composition decides who a sound reaches, and
// the per-recipient Story carries the `heard` beat to its one listener.
//
// THE SCENE. Two rooms joined by one open door. In the hall, alice swings at
// a thug — a clang, DC 5 where it is made. Through the door, one hop and so
// DC 10, stand bob (passive 14) and carol (passive 9), and two castle guards
// authored as templates: a keen one trained in Perception (passive 10,
// exactly the DC) and a dull one with no Perception skill (passive 9, 10 +
// WIS 8's -1). Both guards carry the same standing order: when a clang was
// heard, walk toward it.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type HearingSuite struct {
	suite.Suite

	stream     *fakeStream
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestHearingSuite(t *testing.T) { suite.Run(t, new(HearingSuite)) }

// The two factions are NEUTRAL to the party, the composition's own hearing
// hall's reason: a creature in no faction is opposed to the party, and a
// fight that formed on first sight would put every listener on the turn
// clock before anybody made a sound. The swing turns the thugs hostile and
// leaves the castle at peace, on the world clock, with time to walk.
const (
	hearingCastle encounter.FactionID = "castle"
	hearingThugs  encounter.FactionID = "thugs"
)

// listener is a fighter whose sheet answers the given passive Perception:
// WIS 14 trained in Perception is 10 + 2 + 2 = 14; WIS 8 untrained is 9.
func listener(id string, wis int, trained bool) *character.Data {
	sheet := armedFighter(id)
	sheet.AbilityScores[abilities.WIS] = wis
	if trained {
		sheet.Skills = map[skills.Skill]shared.ProficiencyLevel{skills.Perception: shared.Proficient}
	}
	return sheet
}

// guardOrders is both guards' standing order: a clang heard in the last few
// rounds sends them toward where it rang. One row, so the `time` key is the
// author's wholly and a guard who heard nothing has no row to pick.
func guardOrders() encounter.Table {
	return encounter.Table{encounter.AnswerTime: {{
		Weight: 1,
		When:   []encounter.When{{Heard: &encounter.HeardWhen{Kind: encounter.SoundClang, Within: 6}}},
		Toward: &encounter.Selector{Word: encounter.SelectorSound},
	}}}
}

func (s *HearingSuite) SetupTest() {
	s.stream = &fakeStream{}
	s.encounters = newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{},
		Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: newFakeSessions(), Encounters: s.encounters,
		Characters: newFakeCharacters(
			armedFighter("alice"),
			listener("bob", 14, true),
			listener("carol", 8, false),
		),
		Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	keen := monsterAt("keen", "dnd5e:monsters:keen-guard", 9, 3)
	dull := monsterAt("dull", "dnd5e:monsters:dull-guard", 11, 1)
	thug := monsterAt("thug", refs.Monsters.Goblin().String(), 2, 1)
	keen.Faction, dull.Faction, thug.Faction = string(hearingCastle), string(hearingCastle), string(hearingThugs)
	keen.Table, dull.Table = guardOrders(), guardOrders()

	sc := scene{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 5), rectRegion("barracks", 6, 0, 6, 5)},
			Walls:   hexSeamWallsFrom(6, 0, 5, 1),
			Doors: []encounter.DoorInput{{
				ID: "door", Edges: []encounter.DoorEdge{{From: hexCell(5, 1), To: hexCell(6, 1)}}, State: encounter.DoorIsOpen(),
			}},
			Factions: []encounter.FactionInput{{ID: hearingCastle}, {ID: hearingThugs}},
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{hearingCastle, encounter.FactionParty}, Stance: encounter.StanceNeutral},
				{Between: [2]encounter.FactionID{hearingThugs, encounter.FactionParty}, Stance: encounter.StanceNeutral},
			},
		},
		Party:    []sceneSeat{seatAt("alice", 1, 1), seatAt("bob", 8, 0), seatAt("carol", 6, 4)},
		Monsters: []dungeonspec.MonsterPlacement{keen, dull, thug},
		Endings:  []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
	in := sceneInput(sc)
	in.Dungeon.Templates = map[string]dungeonspec.TemplateSpec{
		// Passive 10: 10, WIS 6's -2, trained Perception's +2.
		"keen-guard": {
			Base: "dnd5e:monsters:human", Abilities: map[string]int{"wis": 6},
			Skills: []string{"perception"}, Actions: []string{"dnd5e:weapons:spear"},
		},
		// Passive 9: no Perception skill, so 10 + WIS 8's -1 — the
		// stat block's own answer, read by the session and never computed.
		"dull-guard": {
			Base: "dnd5e:monsters:human", Abilities: map[string]int{"wis": 8},
			Actions: []string{"dnd5e:weapons:spear"},
		},
	}
	_, err = s.mgr.Launch(context.Background(), in)
	s.Require().NoError(err)
	// alice and the thug on one authored turn clock, alice active, for
	// launchOnClock's reason: the scene is about the swing's noise, not how a
	// fight forms. Everyone else stays on the world clock.
	authorTurnClock(s.T(), s.encounters, "sess", []string{"alice", "thug"}, 0)
	s.stream.published = nil
}

// swing is alice's attack on the thug: struck or missed, it is a clang.
func (s *HearingSuite) swing() {
	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "thug",
		DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "alice"),
	})
	s.Require().NoError(err)
}

// positionOf is where a member stands now, off the stored world.
func (s *HearingSuite) positionOf(member string) spatial.Position {
	s.T().Helper()
	return persistedMonsterPosition(s.T(), s.encounters, member)
}

// heardBy is every heard beat in one member's Story.
func (s *HearingSuite) heardBy(member string) []session.HeardBody {
	s.T().Helper()
	story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	var out []session.HeardBody
	for _, event := range story {
		if event.Kind != session.EventHeard {
			continue
		}
		body, ok := event.Body.(session.HeardBody)
		s.Require().True(ok, "a heard beat decodes to its typed body, got %T", event.Body)
		out = append(out, body)
	}
	return out
}

// aRound makes one round of the world pass: carol paces the barracks' bottom
// row, end to end, until the world clock has had a round — six cells at a
// Human's thirty feet — which is the world's only way to move (somebody
// acts). She makes no sound.
func (s *HearingSuite) aRound() {
	s.T().Helper()
	before := s.encounters.byID[testSession].Clock.HighWater
	for s.encounters.byID[testSession].Clock.HighWater == before {
		at := s.positionOf("carol")
		west, east := hexCell(6, 4), hexCell(10, 4)
		step, end := 1.0, east.X
		if at.X == east.X {
			step, end = -1, west.X
		}
		var path []spatial.Position
		for at.X != end {
			at = spatial.Position{X: at.X + step, Y: at.Y}
			path = append(path, at)
		}
		out, err := s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "carol", Path: path})
		s.Require().NoError(err)
		s.Require().Len(out.Steps, len(path), "the whole pace happened")
	}
}

// timePick is the answered beat a creature's last `time` pick wrote, as
// alice's Story carries it.
func (s *HearingSuite) timePick(creature string) session.AnsweredBody {
	s.T().Helper()
	story, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	var last *session.AnsweredBody
	for _, event := range story {
		body, ok := event.Body.(session.AnsweredBody)
		if ok && body.Creature == creature && body.Key == string(encounter.AnswerTime) {
			last = &body
		}
	}
	s.Require().NotNil(last, "%s had time and picked", creature)
	return *last
}

// THE HEADLINE (castle plan B3; the composition's own first case, now through
// the session): the keen guard's passive 10 reaches DC 5 + 5 for the hop,
// so its next `time` pick is the `heard` row and it walks toward the clang
// through the open door. The dull guard's 9 does not, so it has no row to
// pick and stands where it stood. Both numbers came off the stat blocks the
// session holds — the composition was told nothing else.
func (s *HearingSuite) TestAClangCarriesOneOpenDoorToTheKeenEarOnly() {
	keenFrom, dullFrom := s.positionOf("keen"), s.positionOf("dull")
	clang := s.positionOf("alice")

	s.swing()
	s.aRound()

	s.Equal("toward", s.timePick("keen").Word, "the keen guard picked its heard row")
	s.Less(hexSteps(s.positionOf("keen"), clang), hexSteps(keenFrom, clang),
		"and walked toward where the clang rang")

	s.Equal("hold", s.timePick("dull").Word, "the dull guard heard nothing, so no row was eligible")
	s.Equal(-1, s.timePick("dull").Entry)
	s.Equal(dullFrom, s.positionOf("dull"), "and it did not move")
}

// The keen guard keeps walking on the next round too — the sound is held,
// not a one-round nudge — until it stands at the clang.
func (s *HearingSuite) TestTheKeenGuardWalksAllTheWayToTheClang() {
	clang := s.positionOf("alice")

	s.swing()
	s.aRound()
	s.aRound()

	s.LessOrEqual(hexSteps(s.positionOf("keen"), clang), 1, "it walked to where the clang rang")
}

// THE BEAT REACHES ITS LISTENER ONLY. bob (passive 14) stands through the
// door: one hop, DC 10, heard — and his Story carries the beat naming the kind,
// the cell it rang on and no maker. carol (passive 9) did not hear it and
// never sees it; alice made it and never sees bob's beat either. Live
// delivery says the same: the stream handed one heard event, to bob.
func (s *HearingSuite) TestOnlyTheListenerWhoHeardIsTold() {
	clang := s.positionOf("alice")

	s.swing()

	s.Equal([]session.HeardBody{{Listener: "bob", Kind: encounter.SoundClang, Position: clang, Hops: 1}},
		s.heardBy("bob"))
	s.Empty(s.heardBy("carol"), "passive 9 does not reach DC 10 through the door")
	s.Empty(s.heardBy("alice"), "the maker hears nothing, and another listener's beat is not hers")

	var recipients []string
	for _, event := range s.stream.published {
		if event.Kind == session.EventHeard {
			recipients = append(recipients, event.Recipient)
		}
	}
	s.Equal([]string{"bob"}, recipients, "delivered live to the listener alone")
}
