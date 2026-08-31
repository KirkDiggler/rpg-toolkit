// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package banditcamp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/dnd5eresolver"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scenarios/banditcamp"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scripted"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// objectiveID is the one objective on the guild's contract.
const objectiveID = "camp-no-longer-hostile"

// UC1Suite runs the bandit camp: one declaration, nine verbs, five ways
// through, and no code anywhere that knows which one is being taken.
type UC1Suite struct {
	suite.Suite

	ctx    context.Context
	sheets map[journal.EntityID]*character.Character

	w   *world.World
	job *quest.Instance
}

func TestUC1Suite(t *testing.T) {
	suite.Run(t, new(UC1Suite))
}

func (s *UC1Suite) SetupSuite() {
	s.ctx = context.Background()

	crew, err := banditcamp.Crew(s.ctx)
	s.Require().NoError(err)
	s.sheets = crew
}

// script builds the whole camp around a written-down sequence of d20 results
// and takes the contract off the board. Every contested attempt consumes one
// roll.
//
// The camp is declared content; the dice are injected; the composer assembles
// them. Those three lines are the entire wiring, and they are the same three
// lines any rulebook would write.
func (s *UC1Suite) script(rolls ...int) {
	s.T().Helper()

	resolver, err := dnd5eresolver.New(dnd5eresolver.Config{
		Sheets: s.sheets,
		Roller: scripted.NewRoller(rolls...),
		Bus:    events.NewEventBus(),
	})
	s.Require().NoError(err)

	built, err := world.New(world.Config{
		Scenario: banditcamp.Scenario(),
		Resolver: resolver,
	})
	s.Require().NoError(err)
	s.w = built

	job, events, err := s.w.Claim(banditcamp.ContractID, banditcamp.Party)
	s.Require().NoError(err)
	s.Require().Len(events, 1)
	s.Equal(quest.EventQuestClaimed, events[0].Kind)
	s.job = job
}

func (s *UC1Suite) act(
	verb world.VerbName, actor, target journal.EntityID, bystanders ...journal.EntityID,
) world.Result {
	s.T().Helper()

	result, err := s.w.Act(s.ctx, world.Act{
		Verb: verb, Actor: actor, Target: target, Bystanders: bystanders,
	})
	s.Require().NoError(err)

	return result
}

func (s *UC1Suite) do(
	verb world.VerbName, actor, target journal.EntityID, bystanders ...journal.EntityID,
) journal.Fact {
	s.T().Helper()

	return s.act(verb, actor, target, bystanders...).Fact
}

func (s *UC1Suite) view(observer journal.EntityID) *graph.State {
	return s.w.View(observer)
}

func (s *UC1Suite) observe() quest.Report {
	return s.job.Observe(s.w.Graph(), s.w.Journal())
}

func (s *UC1Suite) factsOfKind(kind journal.Kind) []journal.Fact {
	var out []journal.Fact
	for _, f := range s.w.Journal().All() {
		if f.Kind == kind {
			out = append(out, f)
		}
	}

	return out
}

// campIsHostile is the state every path starts from.
func (s *UC1Suite) assertStartsHostile() {
	s.T().Helper()

	camp := s.view(banditcamp.Camp)
	s.True(camp.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
	s.Equal(banditcamp.Leader, camp.Occupant(banditcamp.Leads, banditcamp.Camp))
	s.False(s.observe().Met[objectiveID])
}

// ---------------------------------------------------------------------------
// Path 1 — the front door.
// approach, attack; assault witnessed by the camp; alerted and formed up;
// objective reached by defeat.
// ---------------------------------------------------------------------------

func (s *UC1Suite) TestFrontDoor() {
	s.script() // Nothing here is contested. Kicking a door needs no roll.
	s.assertStartsHostile()

	s.do(banditcamp.Approach, banditcamp.Brann, banditcamp.Camp)
	assault := s.do(banditcamp.Assault, banditcamp.Brann, banditcamp.Camp)

	s.Run("the camp witnessed the assault", func() {
		s.True(assault.Audience.Includes(banditcamp.Camp))
		s.Equal(banditcamp.Brann, assault.Actor)
	})

	s.Run("so it is alerted and forms up", func() {
		camp := s.view(banditcamp.Camp)
		s.True(camp.Flagged(banditcamp.Alerted, banditcamp.Camp))
		s.Equal(banditcamp.FormedUp, camp.Label(banditcamp.Posture, banditcamp.Camp))
	})

	s.Run("a fight in progress has not met the contract", func() {
		s.True(s.view(banditcamp.Camp).HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
		s.False(s.observe().Met[objectiveID])
	})

	s.Run("defeat meets it", func() {
		result := s.act(banditcamp.Defeat, banditcamp.Brann, banditcamp.Camp)

		camp := s.view(banditcamp.Camp)
		s.True(camp.Flagged(banditcamp.Defeated, banditcamp.Camp))
		s.False(camp.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))

		// The act that changed the world is the act that closed the job: the
		// composer lets the jobs look before it hands the result back.
		s.Require().Len(result.Quests.Events, 1)
		s.Equal(quest.EventQuestCompleted, result.Quests.Events[0].Kind)

		report := s.observe()
		s.True(report.Met[objectiveID])
		s.Equal(quest.StatusCompleted, report.Status)
		s.Empty(report.Events, "a job completes once")
	})
}

// ---------------------------------------------------------------------------
// Path 2 — the back way.
// sneak past; the entry facts carry no camp audience; the camp is surprised.
// ---------------------------------------------------------------------------

func (s *UC1Suite) TestBackWay() {
	s.script(18) // Rook, Stealth +7: 25 against DC 13.
	s.assertStartsHostile()

	infiltration := s.do(banditcamp.Sneak, banditcamp.Rook, banditcamp.Camp)
	entry := s.do(banditcamp.Enter, banditcamp.Rook, banditcamp.Camp)

	s.Run("the sneak landed and the camp is in neither audience", func() {
		s.True(infiltration.Outcome.Contested)
		s.True(infiltration.Outcome.Succeeded)
		s.Equal(journal.Audience{banditcamp.Rook}, infiltration.Audience)
		s.Equal(journal.Audience{banditcamp.Rook}, entry.Audience)
	})

	s.Run("the camp witnessed nothing at all", func() {
		s.Empty(s.w.Journal().WitnessedBy(s.w.Graph().AudienceOf(banditcamp.Camp)...))
	})

	s.Run("so it is unsuspecting, and a fight would start surprised", func() {
		camp := s.view(banditcamp.Camp)
		s.False(camp.Flagged(banditcamp.Alerted, banditcamp.Camp))
		s.Equal(banditcamp.Surprised, camp.Label(banditcamp.Posture, banditcamp.Camp))
		s.True(camp.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
	})

	s.Run("a botched sneak writes the same fact to a different audience", func() {
		s.script(2) // Rook, Stealth +7: 9 against DC 13.

		heard := s.do(banditcamp.Sneak, banditcamp.Rook, banditcamp.Camp)
		s.False(heard.Outcome.Succeeded)
		s.Equal(banditcamp.FactInfiltration, heard.Kind)
		s.True(heard.Audience.Includes(banditcamp.Camp))

		camp := s.view(banditcamp.Camp)
		s.True(camp.Flagged(banditcamp.Alerted, banditcamp.Camp))
		s.Equal(banditcamp.FormedUp, camp.Label(banditcamp.Posture, banditcamp.Camp))
	})
}

// ---------------------------------------------------------------------------
// Path 3 — the changeling.
// an unwitnessed kill and a believed claim; the camp's allegiance follows the
// impostor; the contract closes with no camp combat.
// ---------------------------------------------------------------------------

func (s *UC1Suite) TestChangeling() {
	s.script(19, 15) // Assassinate: 26 vs DC 15. Impersonate: Deception +4 → 19 vs DC 12.
	s.assertStartsHostile()

	kill := s.do(banditcamp.Assassinate, banditcamp.Rook, banditcamp.Leader)
	claim := s.do(banditcamp.Impersonate, banditcamp.Rook, banditcamp.Camp)

	s.Run("the kill reached nobody in the camp", func() {
		s.True(kill.Outcome.Succeeded)
		s.Equal(banditcamp.FactKilling, kill.Kind)
		// The assassin and nobody else. Not the camp, not the man he killed.
		s.Equal(journal.Audience{banditcamp.Rook}, kill.Audience)
		for _, witnessed := range s.w.Journal().WitnessedBy(s.w.Graph().AudienceOf(banditcamp.Camp)...) {
			s.NotEqual(banditcamp.FactKilling, witnessed.Kind)
		}
	})

	s.Run("the claim reached all of it", func() {
		s.True(claim.Outcome.Succeeded)
		s.Equal(banditcamp.FactImpersonation, claim.Kind)
		s.True(claim.Audience.Includes(banditcamp.Camp))
	})

	s.Run("allegiance follows whoever the camp thinks leads it", func() {
		camp := s.view(banditcamp.Camp)
		s.Equal(banditcamp.Rook, camp.Occupant(banditcamp.Leads, banditcamp.Camp))
		s.False(camp.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
		s.True(camp.HasEdge(banditcamp.Camp, banditcamp.AlliedWith, banditcamp.Party))
	})

	s.Run("and no blow was struck", func() {
		s.Empty(s.factsOfKind(banditcamp.FactAssault))
		s.Empty(s.factsOfKind(banditcamp.FactScuffle))
		s.Empty(s.factsOfKind(banditcamp.FactRout))

		camp := s.view(banditcamp.Camp)
		s.False(camp.Flagged(banditcamp.Alerted, banditcamp.Camp))
		s.Equal(banditcamp.Surprised, camp.Label(banditcamp.Posture, banditcamp.Camp))
	})

	s.Run("the contract closes on the same predicate defeat closed", func() {
		report := s.observe()
		s.True(report.Met[objectiveID])
		s.Equal(quest.StatusCompleted, report.Status)
	})

	s.Run("nothing declined to fold", func() {
		s.Empty(s.view(banditcamp.Camp).Refusals())
	})
}

// ---------------------------------------------------------------------------
// Path 4 — diplomacy.
// persuasion facts cross the declared threshold; hostile-to becomes
// allied-with; the camp then counts as fighting for the party.
// ---------------------------------------------------------------------------

func (s *UC1Suite) TestDiplomacy() {
	s.script(10, 10, 10) // Sela, Persuasion +5: 15 against DC 13, three times.
	s.assertStartsHostile()

	s.do(banditcamp.Parley, banditcamp.Sela, banditcamp.Camp)
	for range banditcamp.ConversionThreshold {
		s.do(banditcamp.Persuade, banditcamp.Sela, banditcamp.Camp)
	}

	s.Run("regard accrued to the guild, not to the paladin personally", func() {
		camp := s.view(banditcamp.Camp)
		s.Equal(banditcamp.ConversionThreshold, camp.Count(banditcamp.Regard, banditcamp.Camp, banditcamp.Party))
		s.Zero(camp.Count(banditcamp.Regard, banditcamp.Camp, banditcamp.Sela))
	})

	s.Run("crossing the threshold converts the relationship", func() {
		camp := s.view(banditcamp.Camp)
		s.False(camp.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
		s.True(camp.HasEdge(banditcamp.Camp, banditcamp.AlliedWith, banditcamp.Party))
	})

	s.Run("the camp fights for the guild afterwards, as far as anyone in it knows", func() {
		// Ally behaviour is behaviour reading this fold. Every bandit reads the
		// same edge, because group grain put the same facts in front of them.
		for _, member := range []journal.EntityID{banditcamp.Bandits, banditcamp.Lieutenant} {
			view := s.w.View(member)
			s.True(view.HasEdge(banditcamp.Camp, banditcamp.AlliedWith, banditcamp.Party))
			s.False(view.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
		}
	})

	s.Run("the contract closes without a fight or a body", func() {
		s.Empty(s.factsOfKind(banditcamp.FactRout))
		s.Empty(s.factsOfKind(banditcamp.FactKilling))
		s.True(s.observe().Met[objectiveID])
	})

	s.Run("a botched argument costs ground and holds the line", func() {
		s.script(10, 10, 2) // Two land; the third comes in at 7 against DC 13.

		for range 3 {
			s.do(banditcamp.Persuade, banditcamp.Sela, banditcamp.Camp)
		}

		camp := s.view(banditcamp.Camp)
		s.Equal(1, camp.Count(banditcamp.Regard, banditcamp.Camp, banditcamp.Party))
		s.True(camp.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
		s.False(s.observe().Met[objectiveID])
	})
}

// ---------------------------------------------------------------------------
// Path 5 — the blown disguise.
// the changeling, then one failed check seen by one lieutenant. Audience grain:
// the lieutenant folds a different present from the camp he is standing in.
// ---------------------------------------------------------------------------

func (s *UC1Suite) TestBlownDisguise() {
	s.script(19, 15, 3) // Kill lands, claim lands, and the second claim comes apart.
	s.assertStartsHostile()

	s.do(banditcamp.Assassinate, banditcamp.Rook, banditcamp.Leader)
	s.do(banditcamp.Impersonate, banditcamp.Rook, banditcamp.Camp)

	// Only the lieutenant is close enough to see this one fail.
	reveal := s.do(banditcamp.Impersonate, banditcamp.Rook, banditcamp.Camp, banditcamp.Lieutenant)

	s.Run("the reveal is about the impostor and reaches one witness", func() {
		s.False(reveal.Outcome.Succeeded)
		s.Equal(banditcamp.FactUnmasking, reveal.Kind)
		s.Equal(banditcamp.Rook, reveal.Subject)
		s.True(reveal.Audience.Includes(banditcamp.Lieutenant))
		s.False(reveal.Audience.Includes(banditcamp.Camp))
		s.False(reveal.Audience.Includes(banditcamp.Bandits))
	})

	s.Run("the lieutenant sees through it and is hostile again", func() {
		seen := s.view(banditcamp.Lieutenant)
		s.Empty(seen.Occupant(banditcamp.Leads, banditcamp.Camp))
		s.True(seen.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
		s.False(seen.HasEdge(banditcamp.Camp, banditcamp.AlliedWith, banditcamp.Party))
	})

	s.Run("the camp's own state is untouched", func() {
		camp := s.view(banditcamp.Camp)
		s.Equal(banditcamp.Rook, camp.Occupant(banditcamp.Leads, banditcamp.Camp))
		s.True(camp.HasEdge(banditcamp.Camp, banditcamp.AlliedWith, banditcamp.Party))
		s.False(camp.HasEdge(banditcamp.Camp, banditcamp.HostileTo, banditcamp.Party))
		s.False(camp.Flagged(banditcamp.Alerted, banditcamp.Camp))
	})

	s.Run("and so is every other bandit's", func() {
		bandits := s.w.View(banditcamp.Bandits)
		s.Equal(banditcamp.Rook, bandits.Occupant(banditcamp.Leads, banditcamp.Camp))
		s.True(bandits.HasEdge(banditcamp.Camp, banditcamp.AlliedWith, banditcamp.Party))
	})

	s.Run("the same objective is met in one view and unmet in the other", func() {
		s.True(s.observe().Met[objectiveID])

		doubted := banditcamp.Contract()
		doubted.ID = "quiet-the-bandit-camp-doubted"
		doubted.Objectives[0].Observer = banditcamp.Lieutenant
		board, err := quest.NewBoard(doubted)
		s.Require().NoError(err)
		second, _, err := board.Claim(banditcamp.Party)
		s.Require().NoError(err)
		s.False(second.Observe(s.w.Graph(), s.w.Journal()).Met[objectiveID])
	})
}
