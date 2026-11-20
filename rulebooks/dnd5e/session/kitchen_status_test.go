// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// kitchen_status_test.go is the Keep's kitchen rewards at the session door,
// on the smallest site that has both a line to say and a fight to walk into:
// the ambush hall (fight_starts_test.go) with a neutral cook beside alice's
// start. A plain line to the cook gives a status; walking north meets the
// goblin waiting past the gap, and the fight forms with the session's own
// dice (a flat 10 for everyone).
//
// The goblin is "a-goblin" so its ID sorts BEFORE alice's, and both have
// Dexterity 14: on a tie the goblin goes first, so alice going first can
// only be the status's +1.

const (
	warmedUp = `{"name":"Warmed up","changes":[{"initiative":1}],"maintenance":[{"ends_on":"rest"}],"again":"ignored"}`
	feasted  = `{"name":"Feasted","changes":[{"max_hit_points":2}],"when_given":[{"heal":2}],` +
		`"maintenance":[{"ends_on":"run"}],"again":"ignored"}`
)

type KitchenStatusSuite struct {
	suite.Suite
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestKitchenStatusSuite(t *testing.T) { suite.Run(t, new(KitchenStatusSuite)) }

// cookLine is a plain line (no check) whose one answer gives a status.
func cookLine(id, statusID, status string) encounter.Option {
	return encounter.Option{
		ID: id, Say: "Can I help?",
		Success: []encounter.Answer{{Weight: 1, Say: "Here, have some."}},
		Grants: []encounter.RowGrant{{Row: 0, Consequence: encounter.Consequence{
			ApplyStatus: &encounter.StatusGrant{ID: statusID, Status: json.RawMessage(status), To: "speaker"},
		}}},
	}
}

func (s *KitchenStatusSuite) SetupTest() {
	s.characters = newFakeCharacters(armedFighter("alice"))
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: s.characters,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	cook := monsterAt("cook", refs.Monsters.Goblin().String(), 0, 0)
	cook.Faction = "kitchen"
	cook.Options = []encounter.Option{cookLine("warm", "warmed-up", warmedUp), cookLine("feast", "feasted", feasted)}
	sc := ambushWorld(cook)
	sc.Monsters[0].ID, sc.Monsters[0].MemberID = "a-goblin", "a-goblin"
	sc.Field.Factions = []encounter.FactionInput{{ID: "kitchen"}}
	sc.Field.Dispositions = []encounter.DispositionInput{{
		Between: [2]encounter.FactionID{"kitchen", encounter.FactionParty}, Stance: encounter.StanceNeutral,
	}}
	launched := launchScene(s.T(), s.mgr, sc)
	s.Require().Empty(launched.Formed, "nobody hostile is in sight at the start")
}

func (s *KitchenStatusSuite) say(option string) {
	_, err := s.mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", Target: "cook", OptionID: option})
	s.Require().NoError(err)
}

// walk walks north into the goblin and returns the fight's order.
func (s *KitchenStatusSuite) walk() []string {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: ambushPath()})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "the walk meets the goblin")
	return out.Formed.Order
}

// Catches the seam ignoring a held initiative effect: tied on the die and on
// Dexterity, alice acts first only because she holds Warmed up.
func (s *KitchenStatusSuite) TestAHeldInitiativeStatusActsFirst() {
	s.say("warm")
	s.Equal([]string{"alice", "a-goblin"}, s.walk())
}

// Catches the bonus outliving its status: after a rest ends Warmed up, the
// tie goes back to the goblin's lower ID. Without the status the order is
// the same as if she had never eaten.
func (s *KitchenStatusSuite) TestAnEndedInitiativeStatusAddsNothing() {
	s.say("warm")
	before, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Contains(string(mustJSON(s.T(), before.Conditions)), "Warmed up", "she holds it before the rest")
	_, err = s.mgr.Rest(context.Background(), &session.RestInput{Session: "sess", Kind: session.RestShort,
		Resters: []session.Rester{{Member: "alice"}}})
	s.Require().NoError(err)
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	for _, held := range stored.Conditions {
		s.NotContains(string(held), "Warmed up", "the rest ended Warmed up")
	}
	s.Equal([]string{"a-goblin", "alice"}, s.walk())
}

// wound sets alice's saved hit points, after the launch's first-admission
// long rest has filled them.
func (s *KitchenStatusSuite) wound(hp int) {
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	stored.HitPoints = hp
	s.Require().NoError(s.characters.SaveCharacter(context.Background(), stored))
}

// Catches the heal not passed through (or not saved), run on every saying, or
// capped at the old maximum: a wounded alice is healed by 2 once, the saved
// sheet says so, and saying the line again heals nothing.
func (s *KitchenStatusSuite) TestTheFeastHealsOnceAndIsSaved() {
	s.wound(20)
	s.say("feast")
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Equal(22, stored.HitPoints, "healed by 2, on the saved sheet")
	s.Equal(28, stored.MaxHitPoints, "the saved maximum is the base")

	s.say("feast")
	again, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Equal(22, again.HitPoints, "a second saying heals nothing")
}

// Catches the projection reading the stored base: the session's character
// projection of the saved sheet shows the raised maximum, and a heal from
// full reaches it.
func (s *KitchenStatusSuite) TestTheFeastRaisesTheProjectedMaximum() {
	s.say("feast")
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	s.Equal(30, stored.HitPoints, "full (28) healed by 2 reaches the raised maximum")
	projected, err := resolution.ProjectCharacter(context.Background(), &resolution.ProjectCharacterInput{Character: stored})
	s.Require().NoError(err)
	s.Equal(30, projected.Sheet.MaxHitPoints)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
