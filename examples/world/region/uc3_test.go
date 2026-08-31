// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package region_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/dnd5eresolver"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/region"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scenarios/banditcamp"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scenarios/hostagecamp"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scripted"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/goal"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// The weekend starts here.
var weekend = time.Date(2026, 9, 4, 17, 0, 0, 0, time.UTC)

// Rolls for one full run of the region, in the order the acts consume them.
// Persuasion +3 against DC 13 lands on a 10; Stealth +4 against DC 12 lands on
// anything from 8 up.
var regionRolls = []int{10, 10, 10, 20, 20, 3}

// UC3Suite is the region: two camps, three companies, one needle, one weekend.
type UC3Suite struct {
	suite.Suite

	ctx      context.Context
	sheets   map[journal.EntityID]*character.Character
	scenario world.Scenario

	w     *world.World
	clock *scripted.Clock
}

func TestUC3Suite(t *testing.T) {
	suite.Run(t, new(UC3Suite))
}

func (s *UC3Suite) SetupSuite() {
	s.ctx = context.Background()

	crew, err := region.Crew(s.ctx)
	s.Require().NoError(err)
	s.sheets = crew

	scenario, err := region.Scenario()
	s.Require().NoError(err)
	s.scenario = scenario
}

// build assembles the region with the clock stopped at a chosen moment.
func (s *UC3Suite) build(now time.Time, rolls ...int) {
	s.T().Helper()

	resolver, err := dnd5eresolver.New(dnd5eresolver.Config{
		Sheets: s.sheets,
		Roller: scripted.NewRoller(rolls...),
		Bus:    events.NewEventBus(),
	})
	s.Require().NoError(err)

	s.clock = scripted.NewClock(now)

	built, err := world.New(world.Config{
		Scenario: s.scenario,
		Resolver: resolver,
		Goals:    []goal.Goal{region.WeekendGoal(weekend)},
		Clock:    s.clock,
	})
	s.Require().NoError(err)
	s.w = built
}

func (s *UC3Suite) act(verb world.VerbName, actor, target journal.EntityID) world.Result {
	s.T().Helper()

	result, err := s.w.Act(s.ctx, world.Act{
		Verb: verb, Actor: actor, Target: target,
		Bystanders: hostagecamp.Witnesses(),
	})
	s.Require().NoError(err)

	return result
}

func (s *UC3Suite) claim(party journal.EntityID) *quest.Instance {
	s.T().Helper()

	instance, _, err := s.w.Claim(hostagecamp.RescueJob, party)
	s.Require().NoError(err)

	return instance
}

// hand is one company and the person it sends.
type hand struct {
	party  journal.EntityID
	worker journal.EntityID
}

// pacify runs the whole region: one company talks the camp round, and the other
// two work the hostages — two freed, one lost and then put down.
//
// It returns the result of the act that finished it.
func (s *UC3Suite) pacify(diplomat hand, hostageWork [3]hand) world.Result {
	s.T().Helper()

	s.act(banditcamp.Parley, diplomat.worker, banditcamp.Camp)
	for range banditcamp.ConversionThreshold {
		s.act(banditcamp.Persuade, diplomat.worker, banditcamp.Camp)
	}

	var last world.Result
	for i, h := range hostageWork {
		instance := s.claim(h.party)
		last = s.act(hostagecamp.Rescue, h.worker, instance.Subject())

		// The third attempt is scripted to fail, which turns that hostage.
		if i == len(hostageWork)-1 {
			last = s.act(hostagecamp.PutDown, h.worker, instance.Subject())
		}
	}

	return last
}

func (s *UC3Suite) census() quest.Tally {
	s.T().Helper()

	board, ok := s.w.Ledger().Board(hostagecamp.RescueJob)
	s.Require().True(ok)

	return board.Tally(s.w.Graph(), s.w.Journal())
}

// ---------------------------------------------------------------------------
// 1. Three companies, three methods, one needle.
// ---------------------------------------------------------------------------

func (s *UC3Suite) TestThreeCompaniesByThreeMethodsMoveOneNeedle() {
	s.build(weekend.Add(-48*time.Hour), regionRolls...)

	last := s.pacify(
		hand{hostagecamp.PartyA, hostagecamp.Wren},
		[3]hand{
			{hostagecamp.PartyB, hostagecamp.Marek},
			{hostagecamp.PartyB, hostagecamp.Marek},
			{hostagecamp.PartyC, hostagecamp.Sable},
		},
	)

	s.Run("Ember talked the camp round", func() {
		camp := s.w.View(banditcamp.Camp)
		s.False(camp.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
		s.True(camp.HasEdge(banditcamp.Camp, banditcamp.AlliedWith, banditcamp.Party))
	})

	s.Run("Quill freed two and Thorn lost one and finished it", func() {
		count := s.census()
		s.Equal(3, count.Total())
		s.Equal(2, count.Count(hostagecamp.BucketRescued))
		s.Equal(1, count.Count(hostagecamp.BucketDead))
		s.Zero(count.Count(hostagecamp.BucketCaptive))
		s.Zero(count.Count(hostagecamp.BucketTurned))
	})

	s.Run("and one needle moved, on the act that finished it", func() {
		s.Require().Len(last.Goals.Events, 1)
		s.Equal(goal.EventGoalMet, last.Goals.Events[0].Kind)
		s.Equal(region.WeekendGoalID, last.Goals.Events[0].GoalID)

		status, watched := s.w.GoalStatus(region.WeekendGoalID)
		s.Require().True(watched)
		s.Equal(goal.StatusMet, status)
	})

	s.Run("the journal remembers who did what, and the needle never asked", func() {
		actors := map[journal.EntityID]int{}
		for _, f := range s.w.Journal().All() {
			actors[f.Actor]++
		}
		s.NotZero(actors[hostagecamp.Wren], "Ember's diplomat left no trace")
		s.NotZero(actors[hostagecamp.Marek], "Quill's rescuer left no trace")
		s.NotZero(actors[hostagecamp.Sable], "Thorn's left no trace")

		// The goal is two conditions about the region. Neither mentions anyone.
		s.Equal("Pacify the Region: bandit-camp is not hostile-to guild-crew "+
			"(as bandit-camp sees it) and on rescue-the-hostage, none are captive and none are turned",
			region.WeekendGoal(weekend).Describe())
	})
}

func (s *UC3Suite) TestSwappingWhichCompanyDoesWhatChangesNothing() {
	s.build(weekend.Add(-48*time.Hour), regionRolls...)

	// Thorn talks this time and Ember does the rescuing. Same rolls, same
	// needle, and nothing anywhere is keeping score by company.
	last := s.pacify(
		hand{hostagecamp.PartyC, hostagecamp.Sable},
		[3]hand{
			{hostagecamp.PartyA, hostagecamp.Wren},
			{hostagecamp.PartyA, hostagecamp.Wren},
			{hostagecamp.PartyB, hostagecamp.Marek},
		},
	)

	s.Require().Len(last.Goals.Events, 1)
	s.Equal(goal.EventGoalMet, last.Goals.Events[0].Kind)
	s.False(s.w.View(banditcamp.Camp).HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
	s.Zero(s.census().Count(hostagecamp.BucketCaptive))
}

func (s *UC3Suite) TestTheCampHalfDoesNotCareHowItWasSettled() {
	// Diplomacy above; force here. The needle reads the same either way, which
	// is method-indifference surviving all the way up to guild scale.
	s.build(weekend.Add(-48*time.Hour), 20, 20, 3)

	s.act(banditcamp.Approach, banditcamp.Brann, banditcamp.Camp)
	s.act(banditcamp.Assault, banditcamp.Brann, banditcamp.Camp)
	s.act(banditcamp.Defeat, banditcamp.Brann, banditcamp.Camp)

	var last world.Result
	for i, h := range [3]hand{
		{hostagecamp.PartyA, hostagecamp.Wren},
		{hostagecamp.PartyB, hostagecamp.Marek},
		{hostagecamp.PartyC, hostagecamp.Sable},
	} {
		instance := s.claim(h.party)
		last = s.act(hostagecamp.Rescue, h.worker, instance.Subject())
		if i == 2 {
			last = s.act(hostagecamp.PutDown, h.worker, instance.Subject())
		}
	}

	s.Require().Len(last.Goals.Events, 1)
	s.Equal(goal.EventGoalMet, last.Goals.Events[0].Kind)
	s.True(s.w.View(banditcamp.Camp).Flagged(banditcamp.Defeated, banditcamp.Camp),
		"this run settled the camp by force, not by talking")
}

// ---------------------------------------------------------------------------
// 2. The needle is derived, never stored.
// ---------------------------------------------------------------------------

func (s *UC3Suite) TestTheNeedleIsAFreshFoldOfTheWholeJournal() {
	s.build(weekend.Add(-48*time.Hour), regionRolls...)
	s.pacify(
		hand{hostagecamp.PartyA, hostagecamp.Wren},
		[3]hand{
			{hostagecamp.PartyB, hostagecamp.Marek},
			{hostagecamp.PartyB, hostagecamp.Marek},
			{hostagecamp.PartyC, hostagecamp.Sable},
		},
	)

	// A graph that watched none of it, a ledger nobody claimed from, and a
	// tracker that has never been observed — over the same journal.
	fresh, err := graph.New(s.scenario.Graph)
	s.Require().NoError(err)
	ledger, err := quest.NewLedger(s.scenario.Quests...)
	s.Require().NoError(err)

	s.Run("a stranger reading the same log reads the same needle", func() {
		tracker, err := goal.NewTracker(goal.TrackerConfig{
			Clock: s.clock, Goals: []goal.Goal{region.WeekendGoal(weekend)},
		})
		s.Require().NoError(err)

		report := tracker.Observe(goal.Reading{
			Graph: fresh, Log: s.w.Journal(), Ledger: ledger,
		})
		s.Require().Len(report.Goals, 1)
		s.True(report.Goals[0].Holds)
		s.Equal(goal.StatusMet, report.Goals[0].Status)
	})

	s.Run("rewinding the journal rewinds the needle", func() {
		tracker, err := goal.NewTracker(goal.TrackerConfig{
			Clock: s.clock, Goals: []goal.Goal{region.WeekendGoal(weekend)},
		})
		s.Require().NoError(err)

		report := tracker.Observe(goal.Reading{
			Graph: fresh, Log: journal.New(), Ledger: ledger,
		})
		s.False(report.Goals[0].Holds)
		s.Equal(goal.StatusOpen, report.Goals[0].Status)
	})
}

// ---------------------------------------------------------------------------
// 3. The unlock fires once.
// ---------------------------------------------------------------------------

func (s *UC3Suite) TestTheUnlockFiresExactlyOnce() {
	s.build(weekend.Add(-48*time.Hour), regionRolls...)

	last := s.pacify(
		hand{hostagecamp.PartyA, hostagecamp.Wren},
		[3]hand{
			{hostagecamp.PartyB, hostagecamp.Marek},
			{hostagecamp.PartyB, hostagecamp.Marek},
			{hostagecamp.PartyC, hostagecamp.Sable},
		},
	)
	s.Require().Len(last.Goals.Events, 1)

	s.Run("looking again is silent", func() {
		s.Empty(s.w.ObserveGoals().Events)
		s.Empty(s.w.ObserveGoals().Events)
	})

	s.Run("and it still reads as met", func() {
		report := s.w.ObserveGoals()
		s.Require().Len(report.Goals, 1)
		s.Equal(goal.StatusMet, report.Goals[0].Status)
		s.True(report.Goals[0].Holds)
	})
}

// ---------------------------------------------------------------------------
// 4. Deadlines are honest.
// ---------------------------------------------------------------------------

func (s *UC3Suite) TestAMissedDeadlineIsNeverRetroUnlocked() {
	// The companies start after the weekend has already begun.
	s.build(weekend.Add(time.Hour), regionRolls...)

	s.Run("the first act notices the deadline has passed, once", func() {
		first := s.act(banditcamp.Parley, hostagecamp.Wren, banditcamp.Camp)
		s.Require().Len(first.Goals.Events, 1)
		s.Equal(goal.EventGoalMissed, first.Goals.Events[0].Kind)

		second := s.act(banditcamp.Persuade, hostagecamp.Wren, banditcamp.Camp)
		s.Empty(second.Goals.Events, "a miss is announced once")
	})

	s.Run("finishing the whole region afterwards unlocks nothing", func() {
		for range banditcamp.ConversionThreshold - 1 {
			s.act(banditcamp.Persuade, hostagecamp.Wren, banditcamp.Camp)
		}
		for i, h := range [3]hand{
			{hostagecamp.PartyB, hostagecamp.Marek},
			{hostagecamp.PartyB, hostagecamp.Marek},
			{hostagecamp.PartyC, hostagecamp.Sable},
		} {
			instance := s.claim(h.party)
			result := s.act(hostagecamp.Rescue, h.worker, instance.Subject())
			s.Empty(result.Goals.Events)
			if i == 2 {
				result = s.act(hostagecamp.PutDown, h.worker, instance.Subject())
				s.Empty(result.Goals.Events)
			}
		}

		report := s.w.ObserveGoals()
		s.Equal(goal.StatusMissed, report.Goals[0].Status)
		s.True(report.Goals[0].Holds, "the region really is pacified — it is just too late")
		s.Empty(report.Events)
	})
}

func (s *UC3Suite) TestADeadlineThatPassesWhileNobodyIsActingIsStillNoticed() {
	// A miss is the absence of an act, so an act can never be the thing that
	// notices one. Somebody has to look.
	s.build(weekend.Add(-time.Hour), regionRolls...)

	s.Empty(s.w.ObserveGoals().Events)

	s.clock.Advance(2 * time.Hour)
	report := s.w.ObserveGoals()
	s.Require().Len(report.Events, 1)
	s.Equal(goal.EventGoalMissed, report.Events[0].Kind)
	s.Equal(s.clock.Now(), report.Events[0].At)
}

// ---------------------------------------------------------------------------
// 5. The clock is injected, and refused when absent.
// ---------------------------------------------------------------------------

func (s *UC3Suite) TestAWorldWithGoalsAndNoClockRefuses() {
	resolver, err := dnd5eresolver.New(dnd5eresolver.Config{
		Sheets: s.sheets,
		Roller: scripted.NewRoller(),
		Bus:    events.NewEventBus(),
	})
	s.Require().NoError(err)

	s.Run("goals without a clock are refused, in words an author can act on", func() {
		_, err := world.New(world.Config{
			Scenario: s.scenario,
			Resolver: resolver,
			Goals:    []goal.Goal{region.WeekendGoal(weekend)},
		})
		s.Require().ErrorIs(err, goal.ErrNoClock)
		s.Contains(err.Error(), "what time it is now")
	})

	s.Run("a region with no goals needs no clock", func() {
		built, err := world.New(world.Config{Scenario: s.scenario, Resolver: resolver})
		s.Require().NoError(err)
		s.Empty(built.ObserveGoals().Goals)

		_, watched := built.GoalStatus(region.WeekendGoalID)
		s.False(watched)
	})
}

// ---------------------------------------------------------------------------
// Composition itself.
// ---------------------------------------------------------------------------

func (s *UC3Suite) TestComposingRefusesRegionsItCannotBuild() {
	s.Run("nothing to compose", func() {
		_, err := world.Compose()
		s.Require().ErrorIs(err, world.ErrNothingToCompose)
	})

	s.Run("the same content twice collides on its own names", func() {
		_, err := world.Compose(banditcamp.Scenario(), banditcamp.Scenario())
		s.Require().ErrorIs(err, world.ErrCollision)
		s.Contains(err.Error(), "rename it in one of them")
	})

	s.Run("content that disagrees about belonging is refused", func() {
		odd := hostagecamp.Scenario()
		odd.Graph.Membership = "part-of"
		_, err := world.Compose(banditcamp.Scenario(), odd)
		s.Require().ErrorIs(err, world.ErrMembershipDisagrees)
		s.Contains(err.Error(), "how news reaches a group's members")
	})
}

func (s *UC3Suite) TestTheRegionIsNotAThirdScenario() {
	// Mechanically: it adds no people, no actions and no jobs. Everything in
	// the composed region came from one of the two camps, and the only thing
	// this package declares is the edges that make three companies one guild.
	camp := banditcamp.Scenario()
	hostages := hostagecamp.Scenario()

	s.Len(s.scenario.Graph.Entities, len(camp.Graph.Entities)+len(hostages.Graph.Entities))
	s.Len(s.scenario.Verbs, len(camp.Verbs)+len(hostages.Verbs))
	s.Len(s.scenario.Quests, len(camp.Quests)+len(hostages.Quests))
	s.Len(s.scenario.Graph.Slots, len(camp.Graph.Slots)+len(hostages.Graph.Slots))

	ties := len(s.scenario.Graph.Edges) - len(camp.Graph.Edges) - len(hostages.Graph.Edges)
	s.Equal(len(region.Companies()), ties, "the region's only declaration is one tie per company")
}

func (s *UC3Suite) TestTheRegionIsBothCampsAndTheTiesBetweenThem() {
	s.build(weekend.Add(-48*time.Hour), regionRolls...)

	s.Run("both camps' jobs are on one board", func() {
		_, hasCamp := s.w.Ledger().Board(banditcamp.ContractID)
		_, hasHostages := s.w.Ledger().Board(hostagecamp.RescueJob)
		s.True(hasCamp)
		s.True(hasHostages)
	})

	s.Run("the companies belong to the guild the camp is hostile to", func() {
		truth := s.w.Truth()
		for _, company := range region.Companies() {
			s.Equalf(banditcamp.Party, truth.FactionOf(company),
				"%s does not act for the guild", company)
		}
	})
}
