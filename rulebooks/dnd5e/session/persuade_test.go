// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// persuade_test.go is the second shenanigan across the whole seam
// (rpg-project#458): the twin verb, the social verbs on the WORLD clock where
// no fight exists, the untrained rule reaching a real check, and the author's
// reaction table arriving as a typed beat.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type PersuadeSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters

	// sheet is the checker every scene loads; a scene that is about the
	// untrained rule swaps it before opening the room.
	sheet *character.Data

	// authored is the dungeon author's hand on the spawn, when a scene has
	// one.
	authored func(*session.SpawnInput)

	// turnClock puts alice and the goblin into one authored bubble. False —
	// the default — leaves both in FREE ROAM, which is the front room the
	// whole slice exists for.
	turnClock bool
}

func TestPersuadeSuite(t *testing.T) {
	suite.Run(t, new(PersuadeSuite))
}

func (s *PersuadeSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.sheet, s.authored, s.turnClock = talkingFighter("alice"), nil, false
	s.characters = newFakeCharacters(s.sheet)
}

// front opens a session with alice and a goblin in one hall, NEUTRAL to each
// other so no fight forms — the front room of the design, where a creature
// stands in a doorway and nobody has rolled initiative.
func (s *PersuadeSuite) front(rolls []int) *session.Manager {
	s.characters = newFakeCharacters(s.sheet)
	roller := &sequenceDice{rolls: rolls}
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: roller, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Factions: []encounter.FactionInput{{ID: "goblins"}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	spawn := &session.SpawnInput{
		Session: "sess", ID: "goblin", Ref: refs.Monsters.Goblin().String(),
		Position: spatial.Position{X: 5, Y: 1}, Faction: "goblins",
	}
	if s.authored != nil {
		s.authored(spawn)
	}
	_, err = mgr.Spawn(ctx, spawn)
	s.Require().NoError(err)

	if s.turnClock {
		stored, err := s.encounters.GetEncounter(ctx, "world")
		s.Require().NoError(err)
		s.Require().NoError(s.encounters.SaveEncounter(ctx, "world",
			turnWorld(stored, []string{"alice", "goblin"}, 0)))
	}

	return mgr
}

func (s *PersuadeSuite) persuade(mgr *session.Manager) (*session.PersuadeOutput, error) {
	return mgr.Persuade(context.Background(), &session.PersuadeInput{
		Session: "sess", Member: "alice", Target: "goblin",
	})
}

func (s *PersuadeSuite) rows(mgr *session.Manager) []session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	return out.Declarations
}

func (s *PersuadeSuite) rowFor(mgr *session.Manager, verb session.Verb) session.Declaration {
	for _, decl := range s.rows(mgr) {
		if decl.Verb == verb {
			return decl
		}
	}
	s.Require().Fail("no row", string(verb))

	return session.Declaration{}
}

// THE HEADLINE OF THE SLICE. A player walks up to a neutral goblin in a room
// with no fight in it, talks to it, and it works — no initiative, no turn, no
// action spent, because there is no economy to spend from (R3).
func (s *PersuadeSuite) TestASocialVerbLandsOnTheWorldClock() {
	mgr := s.front([]int{10})

	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "precondition: nothing here is fighting")

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Equal(9, out.DC, "the goblin's own passive Insight, derived not stored")
	s.Equal("persuasion", out.Applied.Ability)
	s.Equal(11, out.Total, "the d20's 10 with CHA 8's -1 and proficiency's +2")
	s.True(out.Beaten)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Nil(stored.ActionEconomy,
		"free roam has no economy: the verb must never ready a sheet, let alone spend from one")
}

// Both social verbs are OFFERED there too, and the panel says they cost
// nothing — the row and the door agree.
func (s *PersuadeSuite) TestBothSocialVerbsAreOfferedFreeInFreeRoam() {
	mgr := s.front([]int{10})

	verbs := make([]session.Verb, 0, 2)
	for _, decl := range s.rows(mgr) {
		verbs = append(verbs, decl.Verb)
		s.True(decl.Available, "%s: the goblin is standing right there", decl.Verb)
		s.Equal(session.SlotNone, decl.Slot, "%s: nothing is spent in free roam", decl.Verb)
		s.Nil(decl.Why, "%s: there is no budget to fall short of", decl.Verb)
		s.Require().Len(decl.Candidates, 1, "%s: one witness in the room", decl.Verb)
		s.Equal("goblin", decl.Candidates[0].Member, "%s: aimed at the witness", decl.Verb)
		s.True(decl.Candidates[0].Available, "%s: speech carries as far as sight", decl.Verb)
	}
	s.Equal([]session.Verb{session.VerbIntimidate, session.VerbPersuade}, verbs)
}

// On the turn clock it is still an action, and still the actor's own turn —
// the world clock changed what free roam offers, not what a fight costs.
func (s *PersuadeSuite) TestOnTheTurnClockItStillCostsAnAction() {
	s.turnClock = true
	mgr := s.front([]int{10})

	row := s.rowFor(mgr, session.VerbPersuade)
	s.Equal(session.SlotAction, row.Slot, "in a fight it draws from the action")
	s.True(row.Available)

	_, err := s.persuade(mgr)
	s.Require().NoError(err)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Require().NotNil(stored.ActionEconomy, "a turn-clock appeal readies and spends")
	s.Zero(stored.ActionEconomy.ActionsRemaining, "the standard action is gone")

	// And a second one has nothing left to spend.
	_, err = s.persuade(mgr)
	s.Require().ErrorIs(err, session.ErrCannotAfford)
}

// A beaten appeal lands its OWN deed, not the threat's. A mind reads the verb,
// and the coward's fear must not key on being talked round.
func (s *PersuadeSuite) TestABeatenAppealLandsItsOwnDeed() {
	mgr := s.front([]int{10})

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	s.True(s.heldBy(mgr, encounter.DeedPersuade), "the goblin remembers being talked round")
	s.False(s.heldBy(mgr, encounter.DeedIntimidate), "and was never threatened")
}

// heldBy reads whether the goblin holds a deed with this verb from alice.
func (s *PersuadeSuite) heldBy(mgr *session.Manager, verb string) bool {
	sightings, err := mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "goblin"})
	s.Require().NoError(err)
	for _, holding := range sightings {
		if holding.Channel != string(deed.Channel) {
			continue
		}
		saw, err := deed.Decode(holding.Payload)
		s.Require().NoError(err)
		if saw.Verb == verb && string(saw.Actor) == "alice" {
			return true
		}
	}

	return false
}

// THE UNTRAINED RULE, at the seam that sets it (rpg-project#457 R2). A
// character who never took Persuasion rolls the verb at disadvantage, and the
// difference is visible in the total the beat reports.
//
// TWO SCENES, ONE DIE SCRIPT. The trained checker rolls once and takes 10; the
// untrained one rolls twice and takes the lower of 10 and 3. A single-die
// assertion could not tell the rule from a bad roll.
func (s *PersuadeSuite) TestAnUntrainedCheckerRollsTheVerbAtDisadvantage() {
	s.Run("trained: one die", func() {
		s.SetupTest()
		out, err := s.persuade(s.front([]int{10, 3}))
		s.Require().NoError(err)
		s.Equal(11, out.Total, "one d20 of 10, CHA -1, proficiency +2")
		s.True(out.Beaten)
	})

	s.Run("untrained: two dice, lower kept", func() {
		s.SetupTest()
		s.sheet = armedFighter("alice") // no Persuasion at all
		out, err := s.persuade(s.front([]int{10, 3}))
		s.Require().NoError(err)
		s.Equal(2, out.Total, "two d20s, the lower kept: 3 with CHA 8's -1 and no proficiency")
		s.False(out.Beaten, "the same script that beat the DC trained misses it untrained")
	})
}

// The author's table arrives as a typed beat: the creature, the verb, the die
// it was rolled with, the entry that fired and the line the author wrote.
func (s *PersuadeSuite) TestTheReactionReachesTheStreamAsATypedBeat() {
	s.authored = func(in *session.SpawnInput) {
		in.Reactions = map[string][]session.Reaction{
			"persuaded": {
				{Weight: 3, Say: "Bandits took the cellar. Go left at the rope."},
				{Weight: 1, Say: "Follow me."},
			},
		}
	}
	// The d20, then the world's own die: 3 + 1 weights make a d4, and a face
	// of 2 lands inside the first entry's share.
	mgr := s.front([]int{10, 2})

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	var found *session.Event
	events := s.events(mgr, "alice")
	for i := range events {
		if events[i].Kind == session.EventReacted {
			found = &events[i]
		}
	}
	s.Require().NotNil(found, "the reaction reached alice's stream as its own kind")
	s.Equal(session.ReactedBody{
		Creature: "goblin", Verb: encounter.DeedPersuade, Beaten: true,
		Roll: 2, Of: 4, Entry: 0, Word: "",
		Say: "Bandits took the cellar. Go left at the rope.", Fact: "",
	}, found.Body, "the die, the weights it was rolled against, and the author's line verbatim")
}

// A verdict the author wrote no table for produces NO reaction beat at all.
// Absent means absent, which is what makes an entry that fires and does
// nothing distinguishable from nothing being authored.
func (s *PersuadeSuite) TestAnUnauthoredOutcomeRollsNothing() {
	s.authored = func(in *session.SpawnInput) {
		in.Reactions = map[string][]session.Reaction{
			"persuade_failed": {{Weight: 1, Say: "Nothing down there, friend."}},
		}
	}
	// ONE face. A reaction roll would ask for a second and fail the verb; the
	// appeal succeeding is the assertion that the beaten half never rolled.
	mgr := s.front([]int{10})

	out, err := s.persuade(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)

	for _, event := range s.events(mgr, "alice") {
		s.NotEqual(session.EventReacted, event.Kind, "nothing was authored for a beaten appeal")
	}
}

// The refusals are the threat's, with the threat's sentinels.
func (s *PersuadeSuite) TestRefusals() {
	mgr := s.front([]int{10})

	_, err := mgr.Persuade(context.Background(), nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = mgr.Persuade(context.Background(), &session.PersuadeInput{Session: "sess", Member: "alice"})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = mgr.Persuade(context.Background(), &session.PersuadeInput{
		Session: "sess", Member: "alice", Target: "nobody"})
	s.ErrorIs(err, session.ErrNoMember)
}

// events reads one member's delivered stream.
func (s *PersuadeSuite) events(mgr *session.Manager, member string) []session.Event {
	story, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	return story
}

// INTIMIDATE GOT THE SAME DOOR, and this scene is why the change is a ruling
// and not a convenience: the verb used to refuse a world-clock actor outright
// with ErrNotYourTurn, on the reasoning that it costs an action and an action
// belongs to a turn. R3 (rpg-project#457) ruled the other way — a front room
// has no turn to be out of.
func (s *PersuadeSuite) TestIntimidateAlsoLandsOnTheWorldClock() {
	mgr := s.front([]int{10})

	out, err := mgr.Intimidate(context.Background(), &session.IntimidateInput{
		Session: "sess", Member: "alice", Target: "goblin",
	})
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.Equal(9, out.DC)

	stored, ok := s.characters.byID["alice"]
	s.Require().True(ok)
	s.Nil(stored.ActionEconomy, "free roam charged nothing, exactly as Move does there")
}

// And the turn clock's own gate is UNCHANGED: somebody else's turn still
// refuses, which is the half of the old rule that was always right.
func (s *PersuadeSuite) TestSomebodyElsesTurnStillRefuses() {
	s.turnClock = true
	mgr := s.front([]int{10})

	stored, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	s.Require().NoError(s.encounters.SaveEncounter(context.Background(), "world",
		turnWorld(stored, []string{"alice", "goblin"}, 1)))

	_, err = s.persuade(mgr)
	s.ErrorIs(err, session.ErrNotYourTurn, "it is the goblin's turn")
}
