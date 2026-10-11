package session_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/stretchr/testify/suite"
)

const merchantAppearance = "dnd5e:npcs:kingdom:merchant-01"
const kingAppearance = "dnd5e:npcs:kingdom:king-01"

type AppearanceRosterSuite struct{ suite.Suite }

func TestAppearanceRosterSuite(t *testing.T) { suite.Run(t, new(AppearanceRosterSuite)) }

type aliceCannotSee struct{}

func (aliceCannotSee) Sight(ids []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(ids))
	for _, id := range ids {
		if id != "alice" {
			out[id] = 100
		} else {
			out[id] = 0
		}
	}
	return out, nil
}

func (s *AppearanceRosterSuite) roster(f *rosterFixture, observer string) []session.PublicMember {
	out, err := f.manager.Roster(context.Background(), &session.RosterInput{Session: "sess", Member: observer, Player: "player-" + observer})
	s.Require().NoError(err)
	return out.Members
}

func (s *AppearanceRosterSuite) TestMemoryUsesCapturedAppearanceWhileAnotherObserverSeesTheNewOne() {
	f := newRosterFixture(s.T())
	data := rosterWorldWithAppearance(s.T(), merchantAppearance, encEveryoneSees{})
	for i := range data.Members {
		if data.Members[i].ID == "skel-1" {
			data.Members[i].AppearanceRef = kingAppearance
		}
	}
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: *data, Capabilities: encounter.Capabilities{
		Sheets: encStandStill{}, Sight: aliceCannotSee{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, Driver: encPassDriver{}, Standing: encEveryoneStanding{},
		Actors: encounter.Actors{Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}},
	}})
	s.Require().NoError(err)
	_, err = enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{"skel-1"}})
	s.Require().NoError(err)
	saved := enc.ToData()
	f.encounters.byID["world"] = &saved
	for observer, expected := range map[string]string{"alice": merchantAppearance, "bob": kingAppearance} {
		found := false
		for _, member := range s.roster(f, observer) {
			if member.ID == "skel-1" {
				found = true
				s.Equal(expected, member.AppearanceRef)
				s.Equal("dnd5e:monsters:skeleton", member.MonsterRef)
				s.Equal("monsters", member.Faction)
			}
		}
		s.True(found)
	}
}

func (s *AppearanceRosterSuite) TestNeverSeenAndLegacyAbsenceAreNotEnriched() {
	f := newRosterFixture(s.T())
	f.encounters.byID["world"] = rosterWorldWithAppearance(s.T(), merchantAppearance, aliceCannotSee{})
	for _, member := range s.roster(f, "alice") {
		s.NotEqual("skel-1", member.ID)
	}
	legacy := rosterWorld(s.T())
	for i := range legacy.Members {
		if legacy.Members[i].ID == "skel-1" {
			legacy.Members[i].AppearanceRef = kingAppearance
		}
	}
	f.encounters.byID["world"] = legacy
	for _, member := range s.roster(f, "alice") {
		if member.ID == "skel-1" {
			s.Empty(member.AppearanceRef, "missing observed appearance is not a request for live appearance")
		}
	}
}

func (s *LaunchSuite) TestAppearanceReachesActiveAndReservedMembersWithoutChangingRules() {
	dungeon := s.camp()
	expected := make(map[string]string)
	for i := range dungeon.Monsters {
		dungeon.Monsters[i].AppearanceRef = merchantAppearance
		expected[dungeon.Monsters[i].MemberID] = dungeon.Monsters[i].Ref
	}
	_, err := s.launch(dungeon, "alice")
	s.Require().NoError(err)
	world := s.encounters.byID["run"]
	s.Require().NotNil(world)
	count := 0
	for _, member := range world.Members {
		if _, exists := expected[string(member.ID)]; exists {
			s.Equal(merchantAppearance, member.AppearanceRef)
			count++
		}
	}
	for _, member := range world.Reserve {
		if _, exists := expected[string(member.ID)]; exists {
			s.Equal(merchantAppearance, member.AppearanceRef)
			count++
		}
	}
	s.Equal(len(expected), count)
	for _, npc := range s.sessions.byID["run"].NPCs {
		s.Equal(expected[npc.ID], npc.Ref.String())
	}
}
