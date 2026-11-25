// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// prayer_status_test.go is the Keep chapel's third benefit at the session
// door (rpg-project#1036): a status whose change is `advantage: checks`,
// given by one line, makes the holder roll the NEXT line's ability check with
// advantage. Same site as kitchen_status_test.go: a neutral priest beside
// alice's start, plain line to give the status, scripted dice.
//
// ALICE'S PERSUASION MODIFIER IS -1: armedFighter has CHA 8 (-1) and no
// Persuasion proficiency. DC 12 therefore needs a d20 of 13 or more. The
// scripted d20s are 5 (total 4, a miss) and 18 (total 17, a pass).

const prayed = `{"name":"Prayed","changes":[{"advantage":"checks"}],"maintenance":[{"ends_on":"run"}],"again":"ignored"}`

const (
	prayerLow  = 5
	prayerHigh = 18
)

// d20Script scripts only the d20s, in order. The answer pick draws its own
// die on a smaller size, which must not eat a scripted face: it gets 1.
type d20Script struct {
	faces []int
	next  int
}

func (d *d20Script) Roll(_ context.Context, size int) (int, error) {
	if size != 20 {
		return 1, nil
	}
	if d.next >= len(d.faces) {
		return 0, fmt.Errorf("d20Script: asked for d20 number %d of %d", d.next+1, len(d.faces))
	}
	face := d.faces[d.next]
	d.next++
	return face, nil
}

type PrayerStatusSuite struct {
	suite.Suite
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestPrayerStatusSuite(t *testing.T) { suite.Run(t, new(PrayerStatusSuite)) }

// manager builds a Manager over the suite's own stores, so a second one is a
// reload of everything saved.
func (s *PrayerStatusSuite) manager(rolls ...int) *session.Manager {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: s.stream, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &d20Script{faces: rolls},
		TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	return mgr
}

func (s *PrayerStatusSuite) SetupTest() {
	s.characters = newFakeCharacters(armedFighter("alice"))
	s.sessions, s.encounters, s.stream = newFakeSessions(), newFakeEncounters(), &fakeStream{}
	s.mgr = s.manager(prayerLow, prayerHigh)

	priest := monsterAt("priest", refs.Monsters.Goblin().String(), 0, 0)
	priest.Faction = "chapel"
	priest.Options = []encounter.Option{
		cookLine("pray", "prayed", prayed),
		{
			ID: "ask", Say: "Will you vouch for us?",
			Check:   []encounter.CheckApproach{{Ability: "persuasion", DC: 12}},
			Success: []encounter.Answer{{Weight: 1, Say: "I will."}},
			Failure: []encounter.Answer{{Weight: 1, Say: "I will not."}},
		},
	}
	sc := ambushWorld(priest)
	sc.Monsters[0].ID, sc.Monsters[0].MemberID = "a-goblin", "a-goblin"
	sc.Field.Factions = []encounter.FactionInput{{ID: "chapel"}}
	sc.Field.Dispositions = []encounter.DispositionInput{{
		Between: [2]encounter.FactionID{"chapel", encounter.FactionParty}, Stance: encounter.StanceNeutral,
	}}
	launched := launchScene(s.T(), s.mgr, sc)
	s.Require().Empty(launched.Formed, "nobody hostile is in sight at the start")
}

func (s *PrayerStatusSuite) say(mgr *session.Manager, option string) *session.SpeakOutput {
	out, err := mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", Target: "priest", OptionID: option})
	s.Require().NoError(err)
	return out
}

// lastLineRoll is the roll on alice's own last line, as the act stream tells it.
func (s *PrayerStatusSuite) lastLineRoll() *session.Roll {
	var roll *session.Roll
	for _, f := range where[session.SpeechAct](s.stream.published, func(a session.SpeechAct) bool {
		return a.Speaker.ID == "alice" && a.Option == "ask" && a.Roll != nil
	}) {
		if f.act.Recipient == "alice" {
			roll = f.body.Roll
		}
	}
	s.Require().NotNil(roll, "alice is told the roll of her line")
	return roll
}

func (s *PrayerStatusSuite) praysFrom(roll *session.Roll) []string {
	var names []string
	for _, in := range roll.Influences {
		if in.Kind == session.InfluenceAdvantage {
			names = append(names, in.Name)
		}
	}
	return names
}

func (s *PrayerStatusSuite) d20Faces(roll *session.Roll) (rolled []int, kept []int) {
	s.Require().NotEmpty(roll.Terms)
	term := roll.Terms[0]
	s.Equal(20, term.DieSize)
	return term.Rolled, term.Faces
}

// Catches advantage leaking onto a character who never prayed: one d20, no
// advantage source, and the low die misses.
func (s *PrayerStatusSuite) TestWithoutTheStatusTheCheckIsOneDie() {
	out := s.say(s.mgr, "ask")
	s.False(out.Beaten)
	s.Equal(prayerLow-1, out.Total)

	roll := s.lastLineRoll()
	rolled, kept := s.d20Faces(roll)
	s.Equal([]int{prayerLow}, rolled, "one d20")
	s.Equal([]int{prayerLow}, kept)
	s.Empty(s.praysFrom(roll), "no advantage source")
}

// Catches the status never reaching a line's check: after the pray line the
// same two scripted dice roll as advantage, the higher is kept and the roll
// names Prayed.
func (s *PrayerStatusSuite) TestTheStatusGivesAdvantageOnTheNextLinesCheck() {
	s.say(s.mgr, "pray")
	out := s.say(s.mgr, "ask")
	s.True(out.Beaten, "the high die was kept")
	s.Equal(prayerHigh-1, out.Total)
	s.assertAdvantageFromPrayer()
}

func (s *PrayerStatusSuite) assertAdvantageFromPrayer() {
	roll := s.lastLineRoll()
	rolled, kept := s.d20Faces(roll)
	s.Equal([]int{prayerLow, prayerHigh}, rolled, "two d20")
	s.Equal([]int{prayerHigh}, kept, "the higher is kept")
	s.Contains(s.praysFrom(roll), "Prayed", "the roll names the status as its advantage source")
}

// Catches the status living only in the manager's memory: a second Manager
// over the same stores still rolls the check with advantage.
func (s *PrayerStatusSuite) TestTheStatusSurvivesAReloadAndStillGivesAdvantage() {
	s.say(s.mgr, "pray")
	reloaded := s.manager(prayerLow, prayerHigh)
	out := s.say(reloaded, "ask")
	s.True(out.Beaten)
	s.assertAdvantageFromPrayer()
}
