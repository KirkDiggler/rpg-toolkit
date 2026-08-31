// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package hostagecamp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/dnd5eresolver"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/hostagecamp"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scripted"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// Rolls, read against the sheets in crew.go. Every rescuer has Stealth +4,
// Insight +3 and Persuasion +3, so the die is the only thing that differs.
const (
	// rescueLands clears DC 12 with +4.
	rescueLands = 20

	// rescueFails does not, and a failed rescue turns the hostage.
	rescueFails = 3

	// readsAsSworn clears the top band of DC 10 with +3 (margin 9).
	readsAsSworn = 16

	// readsAsIndebted clears the middle band (margin 4).
	readsAsIndebted = 11

	// readsAsTalkative clears neither (margin -3).
	readsAsTalkative = 4

	// redemptionLands clears DC 13 with +3.
	redemptionLands = 20

	// redemptionFails does not.
	redemptionFails = 5
)

// UC2Suite is the hostage population: one job, three companies, three names.
type UC2Suite struct {
	suite.Suite

	ctx    context.Context
	sheets map[journal.EntityID]*character.Character

	w *world.World
}

func TestUC2Suite(t *testing.T) {
	suite.Run(t, new(UC2Suite))
}

func (s *UC2Suite) SetupSuite() {
	s.ctx = context.Background()

	crew, err := hostagecamp.Crew(s.ctx)
	s.Require().NoError(err)
	s.sheets = crew
}

// script builds the camp around a written-down sequence of d20 results.
//
// Three lines: declared content, injected dice, the composer assembling them.
// The same three lines banditcamp writes, which is the point of there being a
// second scenario at all.
func (s *UC2Suite) script(rolls ...int) {
	s.T().Helper()

	resolver, err := dnd5eresolver.New(dnd5eresolver.Config{
		Sheets: s.sheets,
		Roller: scripted.NewRoller(rolls...),
		Bus:    events.NewEventBus(),
	})
	s.Require().NoError(err)

	built, err := world.New(world.Config{
		Scenario: hostagecamp.Scenario(),
		Resolver: resolver,
	})
	s.Require().NoError(err)
	s.w = built
}

func (s *UC2Suite) claim(job string, party journal.EntityID) *quest.Instance {
	s.T().Helper()

	instance, _, err := s.w.Claim(job, party)
	s.Require().NoError(err)

	return instance
}

func (s *UC2Suite) act(verb world.VerbName, actor, target journal.EntityID) world.Result {
	s.T().Helper()

	result, err := s.w.Act(s.ctx, world.Act{
		Verb: verb, Actor: actor, Target: target,
		Bystanders: hostagecamp.Witnesses(),
	})
	s.Require().NoError(err)

	return result
}

func (s *UC2Suite) view(observer journal.EntityID) *graph.State {
	return s.w.View(observer)
}

// census counts the rescue population as the world currently stands.
func (s *UC2Suite) census() quest.Tally {
	s.T().Helper()

	board, ok := s.w.Ledger().Board(hostagecamp.RescueJob)
	s.Require().True(ok, "the rescue job is not on the board")

	return board.Tally(s.w.Graph(), s.w.Journal())
}

// claimAll takes all three offers, one per company, and returns each company's
// own instance.
func (s *UC2Suite) claimAll() (a, b, c *quest.Instance) {
	s.T().Helper()

	return s.claim(hostagecamp.RescueJob, hostagecamp.PartyA),
		s.claim(hostagecamp.RescueJob, hostagecamp.PartyB),
		s.claim(hostagecamp.RescueJob, hostagecamp.PartyC)
}

// attempt sends each company's rescuer at that company's own hostage.
func (s *UC2Suite) attempt(instances ...*quest.Instance) {
	s.T().Helper()

	rescuers := hostagecamp.Rescuers()
	for _, instance := range instances {
		s.act(hostagecamp.Rescue, rescuers[instance.Claimant()], instance.Subject())
	}
}

// ---------------------------------------------------------------------------
// 1. Instance isolation: collisions dissolve.
// ---------------------------------------------------------------------------

func (s *UC2Suite) TestOneCompanysFailureTouchesOnlyItsOwnHostage() {
	s.script(rescueFails, rescueLands, rescueLands)

	ember, quill, thorn := s.claimAll()

	s.Run("each company was given a different person", func() {
		s.Equal(hostagecamp.Deryn, ember.Subject())
		s.Equal(hostagecamp.Moss, quill.Subject())
		s.Equal(hostagecamp.Tallow, thorn.Subject())
	})

	s.attempt(ember, quill, thorn)

	s.Run("Ember lost theirs and nobody else lost anything", func() {
		seen := s.view(hostagecamp.Village)
		s.True(seen.Flagged(hostagecamp.Turned, hostagecamp.Deryn))
		s.False(seen.Flagged(hostagecamp.Turned, hostagecamp.Moss))
		s.False(seen.Flagged(hostagecamp.Turned, hostagecamp.Tallow))
		s.True(seen.Flagged(hostagecamp.Freed, hostagecamp.Moss))
		s.True(seen.Flagged(hostagecamp.Freed, hostagecamp.Tallow))
	})

	s.Run("the instances settled independently", func() {
		s.Equal(quest.StatusFailed, ember.Status())
		s.Equal(quest.StatusCompleted, quill.Status())
		s.Equal(quest.StatusCompleted, thorn.Status())
	})

	s.Run("only the turned hostage changed sides", func() {
		seen := s.view(hostagecamp.Village)
		s.True(seen.HasEdge(hostagecamp.Deryn, hostagecamp.AlliedWith, hostagecamp.Captors))
		s.False(seen.HasEdge(hostagecamp.Deryn, hostagecamp.AlliedWith, hostagecamp.Village))
		s.True(seen.HasEdge(hostagecamp.Moss, hostagecamp.AlliedWith, hostagecamp.Village))
		s.True(seen.HasEdge(hostagecamp.Tallow, hostagecamp.AlliedWith, hostagecamp.Village))
	})

	s.Run("nothing declined to fold", func() {
		s.Empty(s.view(hostagecamp.Village).Refusals())
	})
}

// ---------------------------------------------------------------------------
// 2. Claims come off the board and nothing puts them back.
// ---------------------------------------------------------------------------

func (s *UC2Suite) TestAClaimedOfferIsGoneForGood() {
	s.script(rescueLands, rescueFails)

	board, ok := s.w.Ledger().Board(hostagecamp.RescueJob)
	s.Require().True(ok)
	s.Equal(3, board.Available())

	ember, quill, thorn := s.claimAll()
	s.Zero(board.Available())

	s.Run("a fourth company is told why, in words it can act on", func() {
		_, _, err := s.w.Claim(hostagecamp.RescueJob, "party-latecomer")
		s.Require().ErrorIs(err, quest.ErrBoardExhausted)
		s.Contains(err.Error(), "add more names")
	})

	s.Run("finishing one and losing one puts neither back", func() {
		s.attempt(ember, quill)
		s.Equal(quest.StatusCompleted, ember.Status())
		s.Equal(quest.StatusFailed, quill.Status())
		s.Zero(board.Available())
	})

	s.Run("giving up does not put one back either", func() {
		_, err := thorn.Abandon()
		s.Require().NoError(err)
		s.Equal(quest.StatusAbandoned, thorn.Status())
		s.Zero(board.Available(), "Thorn walked away and Tallow is still not on offer")
	})
}

// ---------------------------------------------------------------------------
// 3. The follow-up opens exactly when the whole population settles.
// ---------------------------------------------------------------------------

func (s *UC2Suite) TestTheFollowUpOpensOnTheShapeOfThePopulation() {
	s.script(rescueFails, rescueFails, rescueFails, redemptionFails)

	ember, quill, thorn := s.claimAll()

	s.Run("two lost and one still captive opens nothing", func() {
		s.attempt(ember, quill)

		census := s.census()
		s.Equal(3, census.Total())
		s.Equal(2, census.Count(hostagecamp.BucketTurned))
		s.Equal(1, census.Count(hostagecamp.BucketCaptive))

		_, open := s.w.Ledger().Board(hostagecamp.ReckoningJob)
		s.False(open, "the reckoning opened while somebody was still captive")
	})

	s.Run("the third loss opens it, about exactly those three", func() {
		result := s.act(hostagecamp.Rescue, hostagecamp.Sable, thorn.Subject())

		s.Require().Contains(kinds(result.Quests.Events), quest.EventBoardOpened)

		reckoning, open := s.w.Ledger().Board(hostagecamp.ReckoningJob)
		s.Require().True(open)
		s.Equal(3, reckoning.Available())
		s.Empty(result.Quests.Refusals)
	})

	s.Run("and it does not open again while the shape still holds", func() {
		before := len(s.w.Ledger().Boards())

		// A redemption that misses writes an inert fact, so the population is
		// still exactly the shape that opened the follow-up. The only thing
		// stopping a second one is that the first is remembered.
		instance := s.claim(hostagecamp.ReckoningJob, hostagecamp.PartyA)
		result := s.act(hostagecamp.Redeem, hostagecamp.Wren, instance.Subject())

		s.Equal(hostagecamp.FactSpurned, result.Fact.Kind)
		s.Equal(3, s.census().Count(hostagecamp.BucketTurned), "the shape has not moved")
		s.Equal(before, len(s.w.Ledger().Boards()))
		s.NotContains(kinds(result.Quests.Events), quest.EventBoardOpened)
		s.Empty(result.Quests.Refusals, "a second opening was attempted and turned away")
	})
}

func (s *UC2Suite) TestTheFollowUpStaysShutWhenAnyoneWasSaved() {
	// Nobody is captive any more, but not everybody turned. The shape the
	// follow-up waits for is both halves, and one half is not it.
	s.script(rescueFails, rescueFails, rescueLands)

	s.attempt(s.claimAll())

	census := s.census()
	s.Zero(census.Count(hostagecamp.BucketCaptive))
	s.Equal(2, census.Count(hostagecamp.BucketTurned))
	s.Equal(1, census.Count(hostagecamp.BucketRescued))

	_, open := s.w.Ledger().Board(hostagecamp.ReckoningJob)
	s.False(open)
}

// ---------------------------------------------------------------------------
// 4. Rolled dispositions land as attributed facts.
// ---------------------------------------------------------------------------

func (s *UC2Suite) TestRescuedHostagesTakeItAsTheDiceSay() {
	s.script(
		rescueLands, rescueLands, rescueLands,
		readsAsSworn, readsAsIndebted, readsAsTalkative,
	)

	ember, quill, thorn := s.claimAll()
	s.attempt(ember, quill, thorn)

	rescuers := hostagecamp.Rescuers()
	expected := []struct {
		instance *quest.Instance
		kind     journal.Kind
		flag     graph.Flag
	}{
		{ember, hostagecamp.FactGuardsOath, hostagecamp.Sworn},
		{quill, hostagecamp.FactRepayment, hostagecamp.Indebted},
		{thorn, hostagecamp.FactRumour, hostagecamp.Talkative},
	}

	for _, want := range expected {
		rescuer := rescuers[want.instance.Claimant()]
		result := s.act(hostagecamp.Take, rescuer, want.instance.Subject())

		s.Equalf(want.kind, result.Fact.Kind, "%s read %s", rescuer, want.instance.Subject())
		s.Equalf(rescuer, result.Fact.Actor, "the reading lost its attribution")
		s.Equal(want.instance.Subject(), result.Fact.Subject)
		s.True(result.Fact.Outcome.Contested)
		s.Truef(s.view(hostagecamp.Village).Flagged(want.flag, want.instance.Subject()),
			"%s did not end up %s", want.instance.Subject(), want.flag)
	}

	s.Run("one roll, three results, and no branch anywhere that knows which", func() {
		s.False(s.view(hostagecamp.Village).Flagged(hostagecamp.Sworn, hostagecamp.Moss))
		s.False(s.view(hostagecamp.Village).Flagged(hostagecamp.Talkative, hostagecamp.Deryn))
	})
}

// ---------------------------------------------------------------------------
// 5. The repeatable flip, readable in every view.
// ---------------------------------------------------------------------------

func (s *UC2Suite) TestARedeemedHostageReadsAlliedAgainInEveryView() {
	s.script(
		rescueFails, rescueFails, rescueFails,
		redemptionFails, redemptionLands,
	)

	s.attempt(s.claimAll())

	instance := s.claim(hostagecamp.ReckoningJob, hostagecamp.PartyA)
	s.Equal(hostagecamp.Deryn, instance.Subject())

	s.Run("a redemption that does not take changes nothing and leaves the door open", func() {
		result := s.act(hostagecamp.Redeem, hostagecamp.Wren, instance.Subject())

		s.Equal(hostagecamp.FactSpurned, result.Fact.Kind)
		s.True(s.view(hostagecamp.Village).HasEdge(
			hostagecamp.Deryn, hostagecamp.AlliedWith, hostagecamp.Captors))
		s.Equal(quest.StatusClaimed, instance.Status())
	})

	s.Run("trying again lands it", func() {
		result := s.act(hostagecamp.Redeem, hostagecamp.Wren, instance.Subject())
		s.Equal(hostagecamp.FactRedemption, result.Fact.Kind)
		s.Equal(quest.StatusCompleted, instance.Status())
	})

	s.Run("everyone reads them as allied again", func() {
		observers := []journal.EntityID{
			hostagecamp.Village, hostagecamp.Captors,
			hostagecamp.PartyA, hostagecamp.PartyB, hostagecamp.PartyC,
			hostagecamp.Deryn,
		}
		for _, observer := range observers {
			seen := s.view(observer)
			s.Truef(seen.HasEdge(hostagecamp.Deryn, hostagecamp.AlliedWith, hostagecamp.Village),
				"%s does not read Deryn as allied again", observer)
			s.Falsef(seen.HasEdge(hostagecamp.Deryn, hostagecamp.AlliedWith, hostagecamp.Captors),
				"%s still reads Deryn with the bandits", observer)
		}
	})

	s.Run("the flag that says they turned is still up — outranked, not erased", func() {
		seen := s.view(hostagecamp.Village)
		s.True(seen.Flagged(hostagecamp.Turned, hostagecamp.Deryn))
		s.True(seen.Flagged(hostagecamp.Redeemed, hostagecamp.Deryn))
		s.Equal(1, s.census().Count(hostagecamp.BucketRedeemed))
		s.Equal(2, s.census().Count(hostagecamp.BucketTurned))
	})
}

func (s *UC2Suite) TestTheOtherWayThatJobEnds() {
	s.script(rescueFails, rescueFails, rescueFails)

	s.attempt(s.claimAll())
	instance := s.claim(hostagecamp.ReckoningJob, hostagecamp.PartyA)

	result := s.act(hostagecamp.PutDown, hostagecamp.Wren, instance.Subject())

	s.Equal(hostagecamp.FactExecution, result.Fact.Kind)
	s.Equal(quest.StatusCompleted, instance.Status(), "settled is settled, either way")

	seen := s.view(hostagecamp.Village)
	s.True(seen.Flagged(hostagecamp.Dead, hostagecamp.Deryn))
	s.False(seen.HasEdge(hostagecamp.Deryn, hostagecamp.AlliedWith, hostagecamp.Captors))
	s.False(seen.HasEdge(hostagecamp.Deryn, hostagecamp.HostileTo, hostagecamp.Village))
}

// ---------------------------------------------------------------------------
// 6. Every change arrives by fold over the population.
// ---------------------------------------------------------------------------

func (s *UC2Suite) TestTheCensusIsAFoldOverTheWorldAndNothingIsStored() {
	s.script(rescueFails, rescueLands, rescueFails)

	s.attempt(s.claimAll())

	played := s.census()
	s.Equal(3, played.Total())
	s.Equal(2, played.Count(hostagecamp.BucketTurned))
	s.Equal(1, played.Count(hostagecamp.BucketRescued))

	s.Run("a board that watched none of it counts the same", func() {
		fresh, err := quest.NewBoard(hostagecamp.Contract())
		s.Require().NoError(err)
		s.Zero(fresh.Available()-3, "a fresh board has taken no claims")
		s.Equal(played, fresh.Tally(s.w.Graph(), s.w.Journal()))
	})

	s.Run("a world that watched none of it derives the same", func() {
		fresh, err := graph.New(hostagecamp.Declaration())
		s.Require().NoError(err)

		board, err := quest.NewBoard(hostagecamp.Contract())
		s.Require().NoError(err)
		s.Equal(played, board.Tally(fresh, s.w.Journal()))
	})

	s.Run("rewinding the journal puts everyone back in the cell", func() {
		board, err := quest.NewBoard(hostagecamp.Contract())
		s.Require().NoError(err)

		start := board.Tally(s.w.Graph(), journal.New())
		s.Equal(3, start.Count(hostagecamp.BucketCaptive))
		s.Zero(start.Count(hostagecamp.BucketTurned))
	})

	s.Run("replaying a prefix reproduces that moment", func() {
		prefix := journal.New()
		for _, f := range s.w.Journal().All()[:1] {
			_, err := prefix.Append(f)
			s.Require().NoError(err)
		}

		board, err := quest.NewBoard(hostagecamp.Contract())
		s.Require().NoError(err)

		partway := board.Tally(s.w.Graph(), prefix)
		s.Equal(1, partway.Count(hostagecamp.BucketTurned))
		s.Equal(2, partway.Count(hostagecamp.BucketCaptive))
	})
}

func kinds(events []quest.Event) []quest.EventKind {
	out := make([]quest.EventKind, 0, len(events))
	for _, e := range events {
		out = append(out, e.Kind)
	}

	return out
}
