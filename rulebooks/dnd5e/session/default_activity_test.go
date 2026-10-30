package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/stretchr/testify/suite"
)

const thoughtfulActivity = "dnd5e:activities:thoughtful"
const relaxedActivity = "dnd5e:activities:relaxed"

type DefaultActivityRosterSuite struct{ suite.Suite }

func TestDefaultActivityRosterSuite(t *testing.T) { suite.Run(t, new(DefaultActivityRosterSuite)) }

func (s *DefaultActivityRosterSuite) roster(f *rosterFixture, observer string) []session.PublicMember {
	out, err := f.manager.Roster(context.Background(), &session.RosterInput{Session: "sess", Member: observer, Player: "player-" + observer})
	s.Require().NoError(err)
	return out.Members
}

func (s *DefaultActivityRosterSuite) TestDefaultActivityMemoryDiffersFromAnotherObserversCurrentSight() {
	f := newRosterFixture(s.T())
	data := rosterWorldWithPresentation(s.T(), rosterPresentation{appearanceRef: merchantAppearance, defaultActivityRef: thoughtfulActivity}, encEveryoneSees{})
	for i := range data.Members {
		if data.Members[i].ID == "skel-1" {
			data.Members[i].DefaultActivityRef = relaxedActivity
		}
	}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: *data, Capabilities: encounter.Capabilities{
		Sheets: encStandStill{}, Sight: aliceCannotSee{}, Equipment: encNoHandsObserved{}, Conditions: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, Driver: encPassDriver{}, Participation: encEveryoneStanding{},
		Actors: encounter.Actors{Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}},
	}})
	s.Require().NoError(err)
	_, err = enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{"skel-1"}})
	s.Require().NoError(err)
	encoded, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var saved encounter.EncounterData
	s.Require().NoError(json.Unmarshal(encoded, &saved))
	f.encounters.byID["world"] = &saved
	for observer, expected := range map[string]string{"alice": thoughtfulActivity, "bob": relaxedActivity} {
		found := false
		for _, member := range s.roster(f, observer) {
			if member.ID == "skel-1" {
				found = true
				s.Equal(expected, member.DefaultActivityRef)
				s.Equal(merchantAppearance, member.AppearanceRef)
				s.Equal("dnd5e:monsters:skeleton", member.MonsterRef)
				s.Equal("monsters", member.Faction)
			}
		}
		s.True(found)
	}
}

func (s *DefaultActivityRosterSuite) TestDefaultActivityNeverSeenAndLegacyAbsenceAreNotEnriched() {
	f := newRosterFixture(s.T())
	f.encounters.byID["world"] = rosterWorldWithPresentation(s.T(), rosterPresentation{defaultActivityRef: thoughtfulActivity}, aliceCannotSee{})
	ids := []string{}
	for _, member := range s.roster(f, "alice") {
		ids = append(ids, member.ID)
	}
	s.Contains(ids, "alice")
	s.NotContains(ids, "skel-1")
	legacy := rosterWorld(s.T())
	for i := range legacy.Members {
		if legacy.Members[i].ID == "skel-1" {
			legacy.Members[i].DefaultActivityRef = thoughtfulActivity
		}
	}
	f.encounters.byID["world"] = legacy
	found := false
	for _, member := range s.roster(f, "alice") {
		if member.ID == "skel-1" {
			found = true
			s.Empty(member.DefaultActivityRef, "legacy absence must not join to the live default")
		}
	}
	s.True(found)
}

func (s *LaunchSuite) TestDefaultActivityLaunchReachesActiveAndReservedMembers() {
	dungeon := s.camp()
	expected := make(map[string]string)
	for i := range dungeon.Monsters {
		dungeon.Monsters[i].DefaultActivityRef = thoughtfulActivity
		dungeon.Monsters[i].AppearanceRef = merchantAppearance
		expected[dungeon.Monsters[i].MemberID] = dungeon.Monsters[i].Ref
	}
	_, err := s.launch(dungeon, "alice")
	s.Require().NoError(err)
	world := s.encounters.byID["run"]
	s.Require().NotNil(world)
	encoded, err := json.Marshal(world)
	s.Require().NoError(err)
	var saved encounter.EncounterData
	s.Require().NoError(json.Unmarshal(encoded, &saved))
	found := make(map[string]bool)
	for _, member := range saved.Members {
		if _, exists := expected[string(member.ID)]; exists {
			s.Equal(thoughtfulActivity, member.DefaultActivityRef)
			s.Equal(merchantAppearance, member.AppearanceRef)
			found[string(member.ID)] = true
		}
	}
	for _, member := range saved.Reserve {
		if _, exists := expected[string(member.ID)]; exists {
			s.Equal(thoughtfulActivity, member.DefaultActivityRef)
			s.Equal(merchantAppearance, member.AppearanceRef)
			found[string(member.ID)] = true
		}
	}
	for id := range expected {
		s.True(found[id], "placement %s survives launch", id)
	}
	for _, npc := range s.sessions.byID["run"].NPCs {
		s.Equal(expected[npc.ID], npc.Ref.String())
	}
}
