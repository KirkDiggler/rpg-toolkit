// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

// DepartSuite holds the #1983 gate ruling: a member who leaves a run takes
// what was held on them, and what they held, with them.
//   - A TARGET leaving: the effect another's concentration held on them comes
//     off, and the hold carries on for its other targets.
//   - A CASTER leaving: their hold ends, and its effect comes off every
//     remaining target.
//
// Every removal is told on the exit (or ended) beat, and a rest afterwards
// never meets a departed target.
type DepartSuite struct {
	suite.Suite

	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestDepartSuite(t *testing.T) { suite.Run(t, new(DepartSuite)) }

// partyOfThree is alice, bob and carol in one hall, free roaming.
func partyOfThree(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sheets: encStandStill{}, Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: "carol", Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the hall: %v", err)
	}
	data := enc.ToData()
	return &data
}

// SetupTest stands bob concentrating on Bless, its effect on alice and carol.
func (s *DepartSuite) SetupTest() {
	alice, carol := armedFighter("alice"), armedFighter("carol")
	bob := withHitDice(armedFighter("bob"), 1)
	holding := conditions.NewConcentratingCondition("bob", refs.Spells.Bless().String(), "Bless", 10)
	for _, target := range []string{"alice", "carol"} {
		holding.Children = append(holding.Children, dnd5eEvents.ChildRef{
			MemberID: target, ConditionRef: refs.Conditions.Blessed().String(), SourceID: "bob",
		})
	}
	held, err := holding.ToJSON()
	s.Require().NoError(err)
	bob.Conditions = []json.RawMessage{held}
	for _, target := range []*character.Data{alice, carol} {
		bless, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
			MemberID: target.ID, SourceID: "bob", SourceRef: refs.Spells.Bless(),
		})
		s.Require().NoError(err)
		blessed, err := bless.ToJSON()
		s.Require().NoError(err)
		target.Conditions = append(target.Conditions, blessed)
	}

	s.characters = newFakeCharacters(alice, bob, carol)
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: partyOfThree(s.T()),
	})
	s.Require().NoError(err)
	s.stream.published = nil
}

func (s *DepartSuite) holds(member, ref string) bool {
	data, err := s.characters.GetCharacter(context.Background(), member)
	s.Require().NoError(err)
	for _, raw := range data.Conditions {
		var peek struct {
			Ref *core.Ref `json:"ref"`
		}
		if json.Unmarshal(raw, &peek) == nil && peek.Ref != nil && peek.Ref.String() == ref {
			return true
		}
	}
	return false
}

// holdTargets is who bob's stored Bless hold still names.
func (s *DepartSuite) holdTargets() []string {
	data, err := s.characters.GetCharacter(context.Background(), "bob")
	s.Require().NoError(err)
	for _, raw := range data.Conditions {
		var hold struct {
			Ref      *core.Ref              `json:"ref"`
			Children []dnd5eEvents.ChildRef `json:"children"`
		}
		if json.Unmarshal(raw, &hold) != nil || hold.Ref == nil || hold.Ref.String() != refs.Conditions.Concentrating().String() {
			continue
		}
		var out []string
		for _, child := range hold.Children {
			out = append(out, child.MemberID)
		}
		return out
	}
	return nil
}

// exitRemovals is every removal on the exit beat told to recipient.
func (s *DepartSuite) exitRemovals(recipient string) []string {
	var out []string
	for _, event := range s.stream.published {
		if event.Kind != session.EventExited || event.Recipient != recipient {
			continue
		}
		body, ok := event.Body.(session.ExitedBody)
		s.Require().True(ok)
		for _, removed := range body.Ended {
			out = append(out, removed.Target+":"+removed.Ref)
		}
	}
	return out
}

func (s *DepartSuite) TestATargetLeavesAndTheHoldCarriesOnForTheOthers() {
	blessed := refs.Conditions.Blessed().String()

	_, err := s.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	s.False(s.holds("alice", blessed), "the Bless came off alice as she left")
	s.True(s.holds("carol", blessed), "carol is still blessed")
	s.Equal([]string{"carol"}, s.holdTargets(), "bob's hold carries on, naming carol alone")
	s.Contains(s.exitRemovals("carol"), "alice:"+blessed, "the removal is told on the exit beat")

	_, err = s.mgr.Rest(context.Background(), &session.RestInput{
		Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "bob"}},
	})
	s.Require().NoError(err, "bob's rest never meets the target who left")
}

func (s *DepartSuite) TestACasterLeavesAndTheHoldEndsForEveryTarget() {
	blessed := refs.Conditions.Blessed().String()

	_, err := s.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)

	s.False(s.holds("alice", blessed), "alice's Bless ended with its caster's departure")
	s.False(s.holds("carol", blessed), "carol's too")
	s.Nil(s.holdTargets(), "bob holds nothing as he goes")
	told := s.exitRemovals("alice")
	s.Contains(told, "alice:"+blessed)
	s.Contains(told, "carol:"+blessed)
}

func (s *DepartSuite) TestEndTakesEveryHoldWithTheParty() {
	blessed := refs.Conditions.Blessed().String()

	_, err := s.mgr.End(context.Background(), &session.EndInput{Session: "sess", Ending: "withdrawn"})
	s.Require().NoError(err)

	s.False(s.holds("alice", blessed))
	s.False(s.holds("carol", blessed))
	var told []string
	for _, event := range s.stream.published {
		if event.Kind != session.EventEnded || event.Recipient != "alice" {
			continue
		}
		body, ok := event.Body.(session.EndedBody)
		s.Require().True(ok)
		for member, removals := range body.Ended {
			for _, removed := range removals {
				s.Equal(member, removed.Target, "each removal is filed under the member it came off")
				told = append(told, removed.Target+":"+removed.Ref)
			}
		}
	}
	s.Contains(told, "alice:"+blessed)
	s.Contains(told, "carol:"+blessed)
}
