// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/stretchr/testify/suite"
)

// RestSuite covers slice 4's rest done-when: a short rest inside a run spends
// the asked hit dice and tells a beat; asked in a fight it refuses; the hour
// is one clock jump for the party; a broken concentration's area closes.
type RestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestRestSuite(t *testing.T) { suite.Run(t, new(RestSuite)) }

// woundedFighter has three hit dice to spend and room to heal.
func woundedFighter(id string) *character.Data {
	data := withHitDice(armedFighter(id), 3)
	data.HitPoints = 10
	return data
}

func (s *RestSuite) start(world *encounter.EncounterData, alice, bob *character.Data) {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(alice, bob)
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world,
	})
	s.Require().NoError(err)
	s.stream.published = nil
}

func (s *RestSuite) stored(id string) *character.Data {
	data, err := s.characters.GetCharacter(context.Background(), id)
	s.Require().NoError(err)
	return data
}

func (s *RestSuite) clock() int { return s.encounters.byID["world"].Clock.HighWater }

func (s *RestSuite) restedTo(recipient string) []session.RestedBody {
	var out []session.RestedBody
	for _, event := range s.stream.published {
		if event.Kind != session.EventRested || event.Recipient != recipient {
			continue
		}
		body, ok := event.Body.(session.RestedBody)
		s.Require().True(ok, "a rest event carries its typed body")
		out = append(out, body)
	}
	return out
}

func (s *RestSuite) TestAShortRestSpendsTheAskedHitDiceAndTellsABeat() {
	s.start(freeRoamDuelWorld(s.T()), woundedFighter("alice"), armedFighter("bob"))
	before := s.clock()

	out, err := s.mgr.Rest(context.Background(), &session.RestInput{
		Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "alice", HitDice: 2}},
	})
	s.Require().NoError(err)

	alice := s.stored("alice")
	s.Equal(1, alice.Resources[resources.HitDice].Current, "exactly two of three dice spent")
	s.Greater(alice.HitPoints, 10)
	s.Require().Len(out.Rested, 1)
	s.Equal(2, out.Rested[0].HitDiceSpent)
	s.Equal(1, out.Rested[0].HitDiceRemaining)
	s.Equal(alice.HitPoints-10, out.Rested[0].HitPointsRestored)
	s.Require().NotNil(out.Rested[0].Calculation, "the dice's roll is carried")
	s.Contains(out.Saved.Written, "character:alice")
	s.Contains(out.Saved.Written, "encounter:world")
	s.Equal(before+encounter.RoundsPerHour, s.clock(), "the hour is one jump on the world clock")

	toBob := s.restedTo("bob")
	s.Require().Len(toBob, 1, "the witness hears the rest")
	s.Equal("alice", toBob[0].Member)
	s.Equal(string(session.RestShort), toBob[0].Kind)
	s.Equal(2, toBob[0].HitDiceSpent)
	s.Len(s.restedTo("alice"), 1)
}

func (s *RestSuite) TestThePartyRestsTogetherInOneHour() {
	s.start(freeRoamDuelWorld(s.T()), woundedFighter("alice"), woundedFighter("bob"))
	before := s.clock()

	out, err := s.mgr.Rest(context.Background(), &session.RestInput{
		Session: "sess", Kind: session.RestShort,
		Resters: []session.Rester{{Member: "alice", HitDice: 1}, {Member: "bob", HitDice: 0}},
	})
	s.Require().NoError(err)

	s.Require().Len(out.Rested, 2)
	s.Equal(before+encounter.RoundsPerHour, s.clock(), "two resters, one hour")
	s.Equal(3, s.stored("bob").Resources[resources.HitDice].Current, "bob spent none")
	s.Equal(10, s.stored("bob").HitPoints)
	s.Len(s.restedTo("alice"), 2, "one beat per rester")
}

func (s *RestSuite) TestARestInAFightRefusesAndWritesNothing() {
	s.start(duelWorld(s.T()), woundedFighter("alice"), armedFighter("bob"))
	charSaves, worldSaves := s.characters.saves, s.encounters.saves

	_, err := s.mgr.Rest(context.Background(), &session.RestInput{
		Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "alice", HitDice: 1}},
	})
	s.Require().ErrorIs(err, session.ErrInBubble)

	s.Equal(charSaves, s.characters.saves)
	s.Equal(worldSaves, s.encounters.saves)
	s.Equal(3, s.stored("alice").Resources[resources.HitDice].Current)
	s.Empty(s.stream.published)
}

func (s *RestSuite) TestRequestsARunDoesNotTakeAreRefused() {
	s.start(freeRoamDuelWorld(s.T()), woundedFighter("alice"), armedFighter("bob"))
	for name, in := range map[string]*session.RestInput{
		"long":     {Session: "sess", Kind: session.RestLong, Resters: []session.Rester{{Member: "alice"}}},
		"nobody":   {Session: "sess", Kind: session.RestShort},
		"twice":    {Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "alice"}, {Member: "alice"}}},
		"too many": {Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "alice", HitDice: 4}}},
	} {
		s.Run(name, func() {
			charSaves := s.characters.saves
			_, err := s.mgr.Rest(context.Background(), in)
			s.Require().ErrorIs(err, session.ErrBadRest)
			s.Equal(charSaves, s.characters.saves)
		})
	}
}

func (s *RestSuite) TestAStrangerCannotRest() {
	s.start(freeRoamDuelWorld(s.T()), woundedFighter("alice"), armedFighter("bob"))
	_, err := s.mgr.Rest(context.Background(), &session.RestInput{
		Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "carol"}},
	})
	s.Require().ErrorIs(err, session.ErrNoMember)
}

// TestARestEndsConcentrationAndClosesItsArea is the rest's area law: a
// concentration an hour outlasts ends on the rest, and the sight area the
// caster opened is closed through the encounter's own verb after the rest is
// told.
func (s *RestSuite) TestARestEndsConcentrationAndClosesItsArea() {
	bob := withHitDice(armedFighter("bob"), 1)
	holding := conditions.NewConcentratingCondition("bob", refs.Spells.FogCloud().String(), "Fog Cloud", 10)
	blob, err := holding.ToJSON()
	s.Require().NoError(err)
	bob.Conditions = []json.RawMessage{blob}

	world := freeRoamDuelWorld(s.T())
	world.SightAreas = []encounter.SightAreaData{{
		ID: "fog-bob", SourceID: "bob", Name: "Fog Cloud",
		Center: encounter.PositionData{X: 5, Y: 5}, RadiusFeet: 10,
	}}
	s.start(world, armedFighter("alice"), bob)
	areas, err := s.mgr.Areas(context.Background(), &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Len(areas, 1, "the fog stands before the rest")

	_, err = s.mgr.Rest(context.Background(), &session.RestInput{
		Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "bob"}},
	})
	s.Require().NoError(err)

	areas, err = s.mgr.Areas(context.Background(), &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Empty(areas, "the caster's area closed with the concentration")
	for _, raw := range s.stored("bob").Conditions {
		s.NotContains(string(raw), "concentrat", "the concentration ended")
	}
}

// TestTheRestBeatTellsWhatItRefilledAndWhatItEnded is the rest beat's whole
// account (rpg-project#542): the resources the rest refilled by full ref, the
// concentration it ended with its reason, and the condition it took off the
// rester — all on the rester's own beat, carried as the rulebook answered.
func (s *RestSuite) TestTheRestBeatTellsWhatItRefilledAndWhatItEnded() {
	bob := withHitDice(secondWindFighter(s.T(), "bob", 10, 28), 1)
	feature := map[string]interface{}{}
	s.Require().NoError(json.Unmarshal(bob.Features[0], &feature))
	feature["uses"] = 0
	raw, err := json.Marshal(feature)
	s.Require().NoError(err)
	bob.Features[0] = raw
	holding := conditions.NewConcentratingCondition("bob", refs.Spells.FogCloud().String(), "Fog Cloud", 10)
	held, err := holding.ToJSON()
	s.Require().NoError(err)
	dodging, err := (&conditions.DodgingCondition{MemberID: "bob"}).ToJSON()
	s.Require().NoError(err)
	bob.Conditions = []json.RawMessage{held, dodging}
	s.start(freeRoamDuelWorld(s.T()), armedFighter("alice"), bob)

	out, err := s.mgr.Rest(context.Background(), &session.RestInput{
		Session: "sess", Kind: session.RestShort, Resters: []session.Rester{{Member: "bob"}},
	})
	s.Require().NoError(err)

	toAlice := s.restedTo("alice")
	s.Require().Len(toAlice, 1)
	beat := toAlice[0]
	s.Contains(beat.ResourcesRefilled, refs.Features.SecondWind().String(), "Second Wind refilled")
	s.Equal(beat.ResourcesRefilled, out.Rested[0].ResourcesRefilled)
	s.Require().Len(beat.ConcentrationEnded, 1)
	s.Equal(refs.Spells.FogCloud().String(), beat.ConcentrationEnded[0].Spell.Ref)
	s.Equal("rest", beat.ConcentrationEnded[0].Reason)
	endedRefs := make([]string, 0, len(beat.Ended))
	for _, removed := range beat.Ended {
		endedRefs = append(endedRefs, removed.Ref)
		s.Equal("bob", removed.Target)
	}
	s.Contains(endedRefs, refs.Conditions.Dodging().String(), "the rest took dodging off bob")
}
