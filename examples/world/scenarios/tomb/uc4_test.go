// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package tomb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/dnd5eresolver"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scenarios/tomb"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scripted"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// The config this suite builds every tomb from, and the rolls read against
// it. Finch has Investigation +4 and Sleight of Hand +4 (DEX 14, INT 14,
// proficient in both, +2 proficiency bonus), so the die is the only thing
// that differs between a landed check and a missed one.
const (
	artifactID journal.EntityID = "the-crown"
	captainID  journal.EntityID = "the-captain"

	viaInvestigation journal.Approach = "investigation"
	viaSleightOfHand journal.Approach = "sleight-of-hand"

	findDC = 15
	openDC = 13

	// findLands clears DC 15 with +4 (margin 4).
	findLands = 15
	// findFails does not (margin -6).
	findFails = 5

	// openLands clears DC 13 with +4 (margin 6).
	openLands = 15
	// openFails does not (margin -6).
	openFails = 3
)

// UC4Suite is the tomb: a config that is the builder form, a door that is
// real from birth, and one fact two different routes can write.
type UC4Suite struct {
	suite.Suite

	ctx    context.Context
	sheets map[journal.EntityID]*character.Character
}

func TestUC4Suite(t *testing.T) {
	suite.Run(t, new(UC4Suite))
}

func (s *UC4Suite) SetupSuite() {
	s.ctx = context.Background()

	crew, err := tomb.Crew(s.ctx)
	s.Require().NoError(err)
	s.sheets = crew
}

// config is the one valid form this suite builds a tomb from.
func (s *UC4Suite) config() tomb.Config {
	return tomb.Config{
		Artifact: artifactID,
		Captain:  captainID,
		Find:     tomb.Check{Approach: viaInvestigation, Difficulty: findDC},
		Open:     tomb.Check{Approach: viaSleightOfHand, Difficulty: openDC},
	}
}

// build assembles a tomb world around a written-down sequence of d20 results.
func (s *UC4Suite) build(rolls ...int) *world.World {
	s.T().Helper()

	resolver, err := dnd5eresolver.New(dnd5eresolver.Config{
		Sheets: s.sheets,
		Roller: scripted.NewRoller(rolls...),
		Bus:    events.NewEventBus(),
	})
	s.Require().NoError(err)

	scenario, err := tomb.New(s.config())
	s.Require().NoError(err)

	built, err := world.New(world.Config{
		Scenario: scenario,
		Resolver: resolver,
		Witness:  scripted.NewWitness(tomb.Party),
	})
	s.Require().NoError(err)

	return built
}

// ---------------------------------------------------------------------------
// 1. The config struct IS the builder form.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestNewRefusesAConfigAnAuthorHasNotFinished() {
	cases := []struct {
		name   string
		change func(*tomb.Config)
		want   error
	}{
		{
			name:   "no artifact",
			change: func(c *tomb.Config) { c.Artifact = "" },
			want:   tomb.ErrNoArtifact,
		},
		{
			name:   "no captain",
			change: func(c *tomb.Config) { c.Captain = "" },
			want:   tomb.ErrNoCaptain,
		},
		{
			name:   "no find check",
			change: func(c *tomb.Config) { c.Find = tomb.Check{} },
			want:   tomb.ErrNoFindCheck,
		},
		{
			name:   "no open check",
			change: func(c *tomb.Config) { c.Open = tomb.Check{} },
			want:   tomb.ErrNoOpenCheck,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			broken := s.config()
			tc.change(&broken)

			_, err := tomb.New(broken)
			s.Require().ErrorIs(err, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Availability is an audience fold, not a lock.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestAPartyHoldingNoLocationFactGetsAViewWithNoDoorInIt() {
	w := s.build()

	s.False(tomb.Knows(w, tomb.Bram), "Bram has not been told anything and has not searched")
	s.False(tomb.Knows(w, tomb.Finch), "nor has Finch, before she does anything")

	// This is now literally true, not a journal workaround standing in for
	// it (rpg-toolkit#1342): a stranger's own structural view contains
	// neither the door edge nor the hidden room it leads to. Concealment,
	// not the absence of a fact, is why. Truth still sees both — the GM's
	// view bypasses concealment entirely.
	stranger := w.View(tomb.Finch)
	s.False(stranger.HasEdge(tomb.BossRoom, tomb.LeadsTo, tomb.HiddenRoom))
	s.False(stranger.Visible(tomb.HiddenRoom))

	truth := w.Truth()
	s.True(truth.HasEdge(tomb.BossRoom, tomb.LeadsTo, tomb.HiddenRoom))
	s.True(truth.Visible(tomb.HiddenRoom))
}

// ---------------------------------------------------------------------------
// 3. Fight path: knowledge is loot, and the chest is a separate reward.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestFightPathTransfersKnowledgeAndPaysTwice() {
	w := s.build(openLands)

	instance, _, err := w.Claim(tomb.QuestID, tomb.Party)
	s.Require().NoError(err)

	s.Run("defeating the captain teaches whoever was there", func() {
		result, err := w.Act(s.ctx, world.Act{Verb: tomb.Defeat, Actor: tomb.Thane, Target: captainID})
		s.Require().NoError(err)
		s.Equal(tomb.FactLocationKnown, result.Fact.Kind)

		s.True(tomb.Knows(w, tomb.Thane))
		s.True(tomb.Knows(w, tomb.Bram), "the captain's knowledge became loot for everyone present")
	})

	s.Run("the boss room's chest is a reward of its own", func() {
		result, err := w.Act(s.ctx, world.Act{Verb: tomb.Loot, Actor: tomb.Thane, Target: tomb.BossRoom})
		s.Require().NoError(err)
		s.Equal(tomb.FactLooted, result.Fact.Kind)
		s.True(w.View(tomb.Thane).Flagged(tomb.Looted, tomb.BossRoom))
	})

	s.Run("a knower still has to get through the door", func() {
		// Thane's fighting arm does not pick locks — Finch is the one with
		// Sleight of Hand, and the captain's defeat already taught her too.
		result, err := w.Act(s.ctx, world.Act{Verb: tomb.Open, Actor: tomb.Finch, Target: artifactID})
		s.Require().NoError(err)
		s.Equal(tomb.FactDoorOpened, result.Fact.Kind)
		s.True(w.View(tomb.Finch).Flagged(tomb.Recovered, artifactID))
	})

	s.Run("the fight paid twice: the artifact and the chest", func() {
		s.True(w.Truth().Flagged(tomb.Recovered, artifactID))
		s.True(w.Truth().Flagged(tomb.Looted, tomb.BossRoom))
	})

	s.Run("and the single-run quest completed", func() {
		report := instance.Observe(w.Graph(), w.Journal())
		s.Equal(quest.StatusCompleted, report.Status)
	})
}

// ---------------------------------------------------------------------------
// 4. Search path: the same fact, a different writer, zero combat facts.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestSearchPathRecoversWithZeroCombatFacts() {
	w := s.build(findLands, openLands)

	instance, _, err := w.Claim(tomb.QuestID, tomb.Party)
	s.Require().NoError(err)

	s.Run("a successful search tells the searcher alone", func() {
		result, err := w.Act(s.ctx, world.Act{Verb: tomb.Search, Actor: tomb.Finch, Target: tomb.HiddenRoom})
		s.Require().NoError(err)
		s.Equal(tomb.FactLocationKnown, result.Fact.Kind)

		s.True(tomb.Knows(w, tomb.Finch))
		s.False(tomb.Knows(w, tomb.Bram), "nobody told him and nobody opened anything yet")
	})

	s.Run("opening it recovers the artifact", func() {
		result, err := w.Act(s.ctx, world.Act{Verb: tomb.Open, Actor: tomb.Finch, Target: artifactID})
		s.Require().NoError(err)
		s.Equal(tomb.FactDoorOpened, result.Fact.Kind)
		s.True(w.View(tomb.Finch).Flagged(tomb.Recovered, artifactID))
	})

	s.Run("the quest completed and not one fact belongs to the fighter", func() {
		report := instance.Observe(w.Graph(), w.Journal())
		s.Equal(quest.StatusCompleted, report.Status)

		for _, f := range w.Journal().All() {
			s.NotEqualf(tomb.Thane, f.Actor, "fact %d is a combat fact on the silent-search path", f.Seq)
			s.NotEqual(tomb.FactLooted, f.Kind, "the chest was never touched")
		}
	})
}

// ---------------------------------------------------------------------------
// 5. Party-mates learn only when a knower opens the door in front of them.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestPartyMatesSeeNoDoorUntilAKnowerOpensIt() {
	w := s.build(findLands, openLands)

	_, err := w.Act(s.ctx, world.Act{Verb: tomb.Search, Actor: tomb.Finch, Target: tomb.HiddenRoom})
	s.Require().NoError(err)

	s.Run("Bram still knows nothing after a solo search", func() {
		s.False(tomb.Knows(w, tomb.Bram))
	})

	s.Run("Finch opening it in front of him shares it", func() {
		result, err := w.Act(s.ctx, world.Act{Verb: tomb.Open, Actor: tomb.Finch, Target: artifactID})
		s.Require().NoError(err)
		s.Equal(tomb.FactDoorOpened, result.Fact.Kind)

		s.True(tomb.Knows(w, tomb.Bram), "the open check succeeding is what broadcasts, not the knowing")
	})
}

// ---------------------------------------------------------------------------
// 6. Knowing is not entering: a failed open recovers nothing either.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestAFailedOpenDoesNotRecoverTheArtifact() {
	w := s.build(findLands, openFails)

	_, err := w.Act(s.ctx, world.Act{Verb: tomb.Search, Actor: tomb.Finch, Target: tomb.HiddenRoom})
	s.Require().NoError(err)
	s.True(tomb.Knows(w, tomb.Finch), "she knows where it is")

	result, err := w.Act(s.ctx, world.Act{Verb: tomb.Open, Actor: tomb.Finch, Target: artifactID})
	s.Require().NoError(err)

	s.Equal(tomb.FactOpenFailed, result.Fact.Kind)
	s.False(w.Truth().Flagged(tomb.Recovered, artifactID), "knowing where it is did not open it")
}

// ---------------------------------------------------------------------------
// 7. A failed search writes a fact and reveals nothing; nothing rewinds.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestAFailedSearchWritesAFactAndRevealsNothing() {
	w := s.build(findFails)

	result, err := w.Act(s.ctx, world.Act{Verb: tomb.Search, Actor: tomb.Finch, Target: tomb.HiddenRoom})
	s.Require().NoError(err)

	s.Equal(tomb.FactSearchFailed, result.Fact.Kind)
	s.Equal(1, w.Journal().Len(), "the attempt is recorded — the world does not rewind it")

	s.False(tomb.Knows(w, tomb.Finch), "a miss reveals nothing, not even to the searcher")
	s.False(tomb.Knows(w, tomb.Bram))
}

// ---------------------------------------------------------------------------
// 8. Piercing at the structural level, not just through Knows.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestPiercingIsVisibleInTheStructuralViewToo() {
	w := s.build(findLands)

	_, err := w.Act(s.ctx, world.Act{Verb: tomb.Search, Actor: tomb.Finch, Target: tomb.HiddenRoom})
	s.Require().NoError(err)

	s.Run("the finder's own structural view has both the edge and the room", func() {
		found := w.View(tomb.Finch)
		s.True(found.HasEdge(tomb.BossRoom, tomb.LeadsTo, tomb.HiddenRoom))
		s.True(found.Visible(tomb.HiddenRoom))
	})

	s.Run("a party-mate who was not in the audience sees neither", func() {
		mate := w.View(tomb.Bram)
		s.False(mate.HasEdge(tomb.BossRoom, tomb.LeadsTo, tomb.HiddenRoom))
		s.False(mate.Visible(tomb.HiddenRoom))
	})
}

// ---------------------------------------------------------------------------
// 9. A reveal holds for someone who was never even in this run.
// ---------------------------------------------------------------------------

func (s *UC4Suite) TestALateArrivalSeesTheRevealedDoorWithZeroWitnessedFacts() {
	w := s.build(openLands)

	_, err := w.Act(s.ctx, world.Act{Verb: tomb.Defeat, Actor: tomb.Thane, Target: captainID})
	s.Require().NoError(err)
	_, err = w.Act(s.ctx, world.Act{Verb: tomb.Open, Actor: tomb.Finch, Target: artifactID})
	s.Require().NoError(err)

	// Nobody this content declared — a new hire who joined the party after
	// the tomb was already cleared. Not a party member, not audienced to
	// anything, ever.
	const lateArrival journal.EntityID = "someone-who-joined-later"
	s.Empty(w.Journal().WitnessedBy(w.Graph().AudienceOf(lateArrival)...),
		"confirm this observer truly witnessed nothing before trusting what they see")

	late := w.View(lateArrival)
	s.True(late.HasEdge(tomb.BossRoom, tomb.LeadsTo, tomb.HiddenRoom),
		"perceiving present state is not witnessing past events")
	s.True(late.Visible(tomb.HiddenRoom))
	s.True(tomb.Knows(w, lateArrival))
}
