// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// launch_template_test.go is the session's half of "stat blocks as content"
// (rpg-project#555, plan T3): a placement whose ref names a template the
// dungeon authored spawns through monster.FromTemplate, and the member it
// makes is an ordinary monster from then on.
//
// THE FIXTURE IS THE AUTHORING DIALECT'S OWN castle kitchen, copied into this
// module's testdata because the encounter module's testdata is not this
// module's to read. Each test that needs a variant edits the source text, so
// what is launched is always a file an author could have written.
//
// THE NUMBERS ARE DERIVED, NEVER PASSED. The file says STR 13, CON 12, 2d8 and
// a chain shirt. It never says 11 hit points or armor class 13, so a sheet
// that reads 11 and 13 can only have come from the rulebook's assembly.

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

const castleKitchenFixture = "testdata/world-builder-v4-castle-kitchen.yaml"

// faceDice hands every die the same face, and the face can change between
// verbs: ordinary 10s while the board stands, then a 1 for a swing that must
// miss.
type faceDice struct{ face int }

func (d *faceDice) Roll(_ context.Context, _ int) (int, error) { return d.face, nil }

// LaunchTemplateSuite launches the castle kitchen.
type LaunchTemplateSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	stream     *fakeStream
	dice       *faceDice
	mgr        *session.Manager
}

func TestLaunch_TemplateSuite(t *testing.T) { suite.Run(t, new(LaunchTemplateSuite)) }

func (s *LaunchTemplateSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.stream = &fakeStream{}
	s.dice = &faceDice{face: 10}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: s.dice, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(armedFighter("alice")), Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// castle is the fixture compiled, after each edit replaces its first text
// with its second. An edit that matches nothing fails the test rather than
// launching the unedited file.
func (s *LaunchTemplateSuite) castle(edits ...[2]string) *dungeonspec.Compiled {
	s.T().Helper()
	raw, err := os.ReadFile(castleKitchenFixture)
	s.Require().NoError(err)
	source := string(raw)
	for _, edit := range edits {
		s.Require().Contains(source, edit[0], "the edit must match the fixture")
		source = strings.Replace(source, edit[0], edit[1], 1)
	}
	compiled, err := dungeonspec.Load([]byte(source))
	s.Require().NoError(err)
	return &compiled
}

func (s *LaunchTemplateSuite) launch(dungeon *dungeonspec.Compiled) error {
	_, err := s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: testSession, Dungeon: dungeon, Party: []string{"alice"},
	})
	return err
}

// sheet is a monster's STORED sheet — what a rehydrated run reads.
func (s *LaunchTemplateSuite) sheet(id string) *monster.Data {
	s.T().Helper()
	stored, ok := s.sessions.byID[testSession]
	s.Require().True(ok, "the launch stored no session")
	for i := range stored.NPCs {
		if stored.NPCs[i].ID == id {
			return &stored.NPCs[i]
		}
	}
	s.Require().Failf("no sheet", "the launch recorded no sheet for %q", id)
	return nil
}

func (s *LaunchTemplateSuite) actionRefs(id string) []string {
	s.T().Helper()
	var out []string
	for _, action := range s.sheet(id).Actions {
		s.Require().NotNil(action.Ref, "%s: every action names its weapon", id)
		out = append(out, action.Ref.String())
	}
	return out
}

func (s *LaunchTemplateSuite) assertNothingSpawned() {
	s.T().Helper()
	_, err := s.sessions.GetSession(context.Background(), testSession)
	s.ErrorIs(err, session.ErrNotFound, "no session is written")
	s.Zero(s.encounters.saves, "no world is written")
	s.Empty(s.stream.published, "nobody is told anything arrived")
}

// TestLaunch_TemplateGuardSpawnsWithDerivedNumbers is the headline: the
// guard's numbers are the rulebook's derivation of what the author wrote,
// and the sheet keeps the author's ref.
func (s *LaunchTemplateSuite) TestLaunch_TemplateGuardSpawnsWithDerivedNumbers() {
	s.Require().NoError(s.launch(s.castle()))

	guard := s.sheet("guard-1")
	s.Equal(11, guard.MaxHitPoints, "2d8 averages 9, plus CON 12's +1 per die")
	s.Equal(11, guard.HitPoints, "a guard arrives unhurt")
	s.Equal(13, guard.ArmorClass, "a chain shirt is 13 plus DEX 10's +0")
	s.Require().NotNil(guard.Ref)
	s.Equal("dnd5e:monsters:guard", guard.Ref.String(),
		"the sheet names the template the author placed, not the human it derives from")
	s.Equal([]string{"dnd5e:weapons:spear"}, s.actionRefs("guard-1"),
		"the template's spear, and nothing of the base's fists")

	cook := s.sheet("cook-1")
	s.Equal(4, cook.MaxHitPoints, "the human base's 1d8 at CON 10: an unstated field is the base's")
	s.Equal(10, cook.ArmorClass, "nothing worn")
	s.Equal([]string{"dnd5e:weapons:dagger"}, s.actionRefs("cook-1"))
	s.Equal("dnd5e:monsters:cook", cook.Ref.String())
}

// TestLaunch_PlacementActionsReplaceTemplateWeapons is R9: a binding's
// `actions:` replaces a template's weapons wholesale, exactly as it replaces
// a rulebook monster's, and touches nobody else placed from the same block.
func (s *LaunchTemplateSuite) TestLaunch_PlacementActionsReplaceTemplateWeapons() {
	s.Require().NoError(s.launch(s.castle([2]string{
		"guard-2: {faction: watch}",
		"guard-2: {faction: watch, actions: ['dnd5e:weapons:club']}",
	})))

	s.Equal([]string{"dnd5e:weapons:club"}, s.actionRefs("guard-2"), "club only: the spear is gone")
	s.Equal([]string{"dnd5e:weapons:spear"}, s.actionRefs("guard-1"),
		"the other guard keeps the template's spear")
	s.Equal(11, s.sheet("guard-2").MaxHitPoints, "re-arming changes the weapons and nothing else")
}

// TestLaunch_TemplateShadowingRulebookIsRefused is R2 at launch: a template
// named `goblin` would make `dnd5e:monsters:goblin` mean two things, and the
// launch refuses before anything stands.
func (s *LaunchTemplateSuite) TestLaunch_TemplateShadowingRulebookIsRefused() {
	err := s.launch(s.castle(
		[2]string{"  cook:\n", "  goblin:\n"},
		[2]string{"ref: 'dnd5e:monsters:cook'", "ref: 'dnd5e:monsters:goblin'"},
	))

	s.Require().ErrorIs(err, session.ErrShadowedRef)
	s.Contains(err.Error(), `template "goblin" shadows rulebook monster "dnd5e:monsters:goblin"; rename the template`)
	s.assertNothingSpawned()
}

// TestLaunch_TemplateWithUnknownBaseIsRefused is R8: a base nothing ships is
// refused by its name, never assembled from nothing.
func (s *LaunchTemplateSuite) TestLaunch_TemplateWithUnknownBaseIsRefused() {
	err := s.launch(s.castle([2]string{
		"  cook:\n    base: dnd5e:monsters:human",
		"  cook:\n    base: dnd5e:monsters:elf",
	}))

	s.Require().ErrorIs(err, session.ErrUnknownContent)
	s.Contains(err.Error(), "elf", "the author is told which base")
	s.Contains(err.Error(), "cook-1", "and which placement it stopped on")
	s.assertNothingSpawned()
}

// TestLaunch_TemplateMemberIsAttackableAndTurnsHostile is the R1 pin: a
// creature placed from a template is a monster to every rule downstream. A
// swing at the neutral cook is not refused as a non-target, the aggression
// law turns the kitchen hostile to the party, and the kitchen is in the
// fight.
//
// THE COOK STANDS NEXT TO THE PARTY'S SEAT, one cell from where the fixture
// puts it, so a sword can reach. ALICE'S TURN IS AUTHORED: this seam has no
// free-roam attack offer (bothways_test.go says why), and the castle is civil,
// so nothing forms a fight on its own. THE SWING MISSES ON PURPOSE (every die
// a 1): the claim is that the kitchen turns, not that the cook dies, and a
// cook at 4 hit points would fall out of the fight it is claimed to join.
func (s *LaunchTemplateSuite) TestLaunch_TemplateMemberIsAttackableAndTurnsHostile() {
	s.Require().NoError(s.launch(s.castle([2]string{
		"{id: cook-1, ref: 'dnd5e:monsters:cook', startingCell: { location: { q: -1, r: 2 } }}",
		"{id: cook-1, ref: 'dnd5e:monsters:cook', startingCell: { location: { q: -1, r: 1 } }}",
	})))
	s.Require().Empty(eventsOfKind(s.stream.published, "alice", session.EventFightStarted),
		"precondition: the castle is civil and nothing is fighting")
	authorTurnClock(s.T(), s.encounters, testSession, []string{"alice"}, 0)
	s.stream.published = nil
	s.dice.face = 1

	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: testSession, Attacker: "alice", Target: "cook-1",
		DeclarationID: currentAttackID(s.T(), s.mgr, testSession, "alice"),
	})
	s.Require().NotErrorIs(err, session.ErrNotATarget, "a template member is a monster, and a monster can be attacked")
	s.Require().NoError(err)

	s.Run("the kitchen and the party are hostile", func() {
		events := eventsOfKind(s.stream.published, "alice", session.EventStanceChanged)
		s.Require().Len(events, 1, "alice heard the pair turn exactly once")
		body, ok := events[0].Body.(session.StanceChangedBody)
		s.Require().True(ok)
		s.ElementsMatch([]string{"kitchen", encounter.FactionParty}, body.Between)
		s.Equal(string(encounter.StanceHostile), body.Stance)
	})

	s.Run("the cook is in the fight", func() {
		turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: testSession, Member: "cook-1"})
		s.Require().NoError(err)
		s.Equal(session.ClockTurn, turn.Clock)
	})

	s.Run("the watch stays out of it", func() {
		turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: testSession, Member: "guard-1"})
		s.Require().NoError(err)
		s.Equal(session.ClockWorld, turn.Clock, "the watch was never attacked and is still neutral to the party")
	})
}
