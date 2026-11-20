// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/events"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/status"
)

// A status in an author's words that heals 2 once, when given, and otherwise
// grants advantage on Charisma checks until the run ends.
const fedWords = `{"name":"Fed","changes":[{"advantage":"checks","when":{"ability":"charisma"}}],` +
	`"when_given":[{"heal":2}],"maintenance":[{"ends_on":"run"}],"again":"ignored"}`

// The same, and it also raises the holder's maximum hit points by 2.
const feastedWords = `{"name":"Feasted","changes":[{"max_hit_points":2}],` +
	`"when_given":[{"heal":2}],"maintenance":[{"ends_on":"run"}],"again":"ignored"}`

// compileStatus compiles a status the way the session does before giving it.
func (s *GiveStatusTestSuite) compileStatus(id, words string) *status.CompileOutput {
	compiled, err := status.Compile(&status.CompileInput{ID: id, Status: json.RawMessage(words)})
	s.Require().NoError(err)
	s.Require().NotEmpty(compiled.WhenGiven)

	return compiled
}

// giveCompiled gives a compiled status, with its when-given acts, to a member.
func (s *GiveStatusTestSuite) giveCompiled(interaction *Input, compiled *status.CompileOutput, member string) (*GiveStatusOutput, error) {
	return GiveStatus(s.ctx, &GiveStatusInput{
		Interaction: interaction, Member: member,
		Library: compiled.Library, StatusID: compiled.DefinitionID,
		SourceID: "cook", Source: cookRef(), ApplicationID: compiled.DefinitionID + ":" + member,
		WhenGiven: compiled.WhenGiven,
	})
}

// healHero gives the hero a compiled status at the given hit points (of 14)
// and returns the answer and the reloaded sheet.
func (s *GiveStatusTestSuite) healHero(compiled *status.CompileOutput, hp int) (*GiveStatusOutput, *character.Character) {
	fixtures := s.fixtures()
	out, err := s.giveCompiled(s.interaction(fixtures,
		Participant{Character: fixtures.saver(hp)}, Participant{Character: fixtures.bard(1)}), compiled, heroID)
	s.Require().NoError(err)
	s.False(out.AlreadyHeld)
	loaded, err := character.Load(s.ctx, fixtures.sheet(out.Resolved, heroID))
	s.Require().NoError(err)

	return out, loaded
}

// A WOUNDED HERO IS HEALED BY THE AMOUNT, and the heal survives a save and a
// reload. Catches: acts not run at all, run but not marked dirty (the saved
// record would still say 10), or the amount misread.
func (s *GiveStatusTestSuite) TestAWoundedHeroIsHealedAndKeepsItAfterAReload() {
	out, loaded := s.healHero(s.compileStatus("fed", fedWords), 10)
	s.Require().NotEmpty(out.WhenGiven)
	s.Equal(WhenGivenDone{Kind: status.WhenGivenHeal, Amount: 2, Applied: 2, HitPointsBefore: 10, HitPointsAfter: 12}, out.WhenGiven[0])
	s.Equal(12, loaded.GetHitPoints(), "the saved record carries the heal")
	s.Equal(14, loaded.GetMaxHitPoints())
}

// AT FULL, THE HEAL RESTORES NOTHING and says so. Catches: a heal that
// bypasses the sheet's cap (current would read 16 over a maximum of 14).
func (s *GiveStatusTestSuite) TestAHeroAtFullGainsNothing() {
	out, loaded := s.healHero(s.compileStatus("fed", fedWords), 14)
	s.Require().NotEmpty(out.WhenGiven)
	s.Equal(0, out.WhenGiven[0].Applied)
	s.Equal(2, out.WhenGiven[0].Amount)
	s.Equal(14, loaded.GetHitPoints())
}

// ONE BELOW FULL GAINS ONE. Catches: reporting the authored amount as what
// was restored instead of what the sheet actually applied.
func (s *GiveStatusTestSuite) TestOneBelowFullGainsOne() {
	out, loaded := s.healHero(s.compileStatus("fed", fedWords), 13)
	s.Require().NotEmpty(out.WhenGiven)
	s.Equal(1, out.WhenGiven[0].Applied)
	s.Equal(14, loaded.GetHitPoints())
}

// A RAISED MAXIMUM IS ROOM TO HEAL INTO: at full 14 of 14, a status that
// raises the maximum by 2 and heals 2 ends at 16 of 16. Catches: running the
// acts before the status lands (the heal would be capped at the old 14).
func (s *GiveStatusTestSuite) TestAHealLandsAgainstTheMaximumTheStatusRaised() {
	out, loaded := s.healHero(s.compileStatus("feasted", feastedWords), 14)
	s.Require().NotEmpty(out.WhenGiven)
	s.Equal(2, out.WhenGiven[0].Applied)
	s.Equal(16, out.WhenGiven[0].HitPointsAfter)
	s.Equal(16, loaded.GetMaxHitPoints())
	s.Equal(16, loaded.GetHitPoints(), "old current 14 + the heal of 2, under the new maximum")
}

// GIVEN AGAIN, IT HEALS NOTHING: the second give is already held, runs no act,
// and changes no sheet. Catches: running the acts on the already-held path.
func (s *GiveStatusTestSuite) TestGivingAgainHealsNothing() {
	compiled := s.compileStatus("fed", fedWords)
	fixtures := s.fixtures()
	first, err := s.giveCompiled(s.interaction(fixtures,
		Participant{Character: fixtures.saver(8)}, Participant{Character: fixtures.bard(1)}), compiled, heroID)
	s.Require().NoError(err)
	held := fixtures.sheet(first.Resolved, heroID)
	s.Equal(10, held.HitPoints)

	again, err := s.giveCompiled(s.interaction(fixtures,
		Participant{Character: held}, Participant{Character: fixtures.bard(1)}), compiled, heroID)
	s.Require().NoError(err)
	s.True(again.AlreadyHeld)
	s.Empty(again.WhenGiven)
	for _, dirty := range again.Resolved.DirtyCharacters {
		s.NotEqual(heroID, dirty.ID, "a repeated give heals no one")
	}
	s.Equal(10, held.HitPoints)
}

// A MONSTER IS HEALED THE SAME WAY, once, and keeps it across a reload.
// Catches: a heal wired only to character sheets.
func (s *GiveStatusTestSuite) TestAMonsterIsHealedOnceAndKeepsIt() {
	compiled := s.compileStatus("fed", fedWords)
	fixtures := s.fixtures()
	wounded := fixtures.wolfData()
	wounded.HitPoints = 5
	out, err := s.giveCompiled(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Monster: wounded}), compiled, wolfID)
	s.Require().NoError(err)
	s.Require().NotEmpty(out.WhenGiven)
	s.Equal(WhenGivenDone{Kind: status.WhenGivenHeal, Amount: 2, Applied: 2, HitPointsBefore: 5, HitPointsAfter: 7}, out.WhenGiven[0])
	saved := s.monsterSheet(out.Resolved, wolfID)
	reloaded, err := monster.Load(s.ctx, saved)
	s.Require().NoError(err)
	s.Equal(7, reloaded.GetHitPoints())

	again, err := s.giveCompiled(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Monster: saved}), compiled, wolfID)
	s.Require().NoError(err)
	s.True(again.AlreadyHeld)
	s.Empty(again.WhenGiven)
	for _, dirty := range again.Resolved.DirtyMonsters {
		s.NotEqual(wolfID, dirty.ID, "a repeated give heals no one")
	}
}

// A HERO AT ZERO, DYING, IS BROUGHT BACK: the engine's ordinary healing raises
// a dying character above zero and clears their death saves, and a status's
// heal is that same healing. Catches: a separate heal path that skips the
// sheet's handler (death saves would stay failed).
func (s *GiveStatusTestSuite) TestADyingHeroAtZeroIsHealedAndTheirDeathSavesClear() {
	compiled := s.compileStatus("fed", fedWords)
	fixtures := s.fixtures()
	dying := fixtures.saver(0)
	dying.DeathSaveState = &saves.DeathSaveState{Failures: 1}
	out, err := s.giveCompiled(s.interaction(fixtures,
		Participant{Character: dying}, Participant{Character: fixtures.bard(1)}), compiled, heroID)
	s.Require().NoError(err)
	s.Require().NotEmpty(out.WhenGiven)
	s.Equal(2, out.WhenGiven[0].Applied)
	saved := fixtures.sheet(out.Resolved, heroID)
	s.Equal(2, saved.HitPoints)
	s.Require().NotNil(saved.DeathSaveState)
	s.Zero(saved.DeathSaveState.Failures, "healing above zero clears the death saves")
}

// AN ACT THE ENGINE DOES NOT KNOW, or an amount outside 1 to 1000, is refused
// before anything is given. Catches: a malformed act slipping through and
// landing the status without its heal.
func (s *GiveStatusTestSuite) TestABadActIsRefusedBeforeAnythingIsGiven() {
	compiled := s.compileStatus("fed", fedWords)
	fixtures := s.fixtures()
	for _, tc := range []struct {
		name string
		act  status.WhenGivenAct
	}{
		{"an unknown kind", status.WhenGivenAct{Kind: "teleport", Amount: 2}},
		{"an amount of zero", status.WhenGivenAct{Kind: status.WhenGivenHeal, Amount: 0}},
		{"a negative amount", status.WhenGivenAct{Kind: status.WhenGivenHeal, Amount: -2}},
		{"an amount over 1000", status.WhenGivenAct{Kind: status.WhenGivenHeal, Amount: 1001}},
	} {
		s.Run(tc.name, func() {
			out, err := GiveStatus(s.ctx, &GiveStatusInput{
				Interaction: s.interaction(fixtures,
					Participant{Character: fixtures.saver(10)}, Participant{Character: fixtures.bard(1)}),
				Member: heroID, Library: compiled.Library, StatusID: compiled.DefinitionID,
				SourceID: "cook", Source: cookRef(), ApplicationID: "fed:" + heroID,
				WhenGiven: []status.WhenGivenAct{{Kind: status.WhenGivenHeal, Amount: 1}, tc.act},
			})
			s.Require().ErrorIs(err, ErrBadAction)
			s.Nil(out)
		})
	}
}

// giveWatched gives a compiled status on a bus the test holds, and counts
// every condition and heal published on it — what reached the in-memory
// sheets, whether or not anything came back to be saved.
func (s *GiveStatusTestSuite) giveWatched(interaction *Input, compiled *status.CompileOutput, member string) (given, heals int, out *Output, err error) {
	machine, err := newGiveStatus(&GiveStatusInput{
		Interaction: interaction, Member: member,
		Library: compiled.Library, StatusID: compiled.DefinitionID,
		SourceID: "cook", Source: cookRef(), ApplicationID: compiled.DefinitionID + ":" + member,
		WhenGiven: compiled.WhenGiven,
	})
	s.Require().NoError(err)
	bus := events.NewEventBus()
	_, subErr := dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(s.ctx, func(context.Context, dnd5eEvents.ConditionAppliedEvent) error {
		given++
		return nil
	})
	s.Require().NoError(subErr)
	_, subErr = dnd5eEvents.HealingReceivedTopic.On(bus).Subscribe(s.ctx, func(context.Context, dnd5eEvents.HealingReceivedEvent) error {
		heals++
		return nil
	})
	s.Require().NoError(subErr)
	interaction.Machine = machine
	out, err = resolveOn(s.ctx, interaction, newSurface(bus))

	return given, heals, out, err
}

// A DEFEATED MONSTER IS NOT REVIVED: ordinary healing refuses a monster at 0,
// so a status that heals is refused whole before anything is given — no
// condition and no heal reach its sheet. Catches: a status heal that skips the
// rule ordinary healing applies (the monster's own handler would heal it).
func (s *GiveStatusTestSuite) TestADefeatedMonsterIsRefusedAndStaysDown() {
	compiled := s.compileStatus("fed", fedWords)
	fixtures := s.fixtures()
	down := fixtures.wolfData()
	down.HitPoints = 0
	given, heals, out, err := s.giveWatched(s.interaction(fixtures,
		Participant{Character: fixtures.saver(14)}, Participant{Monster: down}), compiled, wolfID)
	s.Require().ErrorIs(err, ErrBadAction)
	s.Nil(out)
	s.Zero(given, "no part of the status reached the in-memory sheet")
	s.Zero(heals, "no heal was published")
	s.Equal(0, down.HitPoints)
	s.Empty(down.Conditions)
}

// A DEAD CHARACTER IS REFUSED THE SAME WAY, up front: nothing reaches the
// in-memory sheet and nothing comes back to save. Catches: refusing only when
// the heal lands, after the status is already on the sheet.
func (s *GiveStatusTestSuite) TestADeadCharacterIsRefusedBeforeAnythingIsGiven() {
	compiled := s.compileStatus("fed", fedWords)
	fixtures := s.fixtures()
	dead := fixtures.saver(0)
	dead.DeathSaveState = &saves.DeathSaveState{Failures: 3, Dead: true}
	given, heals, out, err := s.giveWatched(s.interaction(fixtures,
		Participant{Character: dead}, Participant{Character: fixtures.bard(1)}), compiled, heroID)
	s.Require().ErrorIs(err, ErrBadAction)
	s.Nil(out, "no output, so no dirty sheet for the caller to save")
	s.Zero(given, "no part of the status reached the in-memory sheet")
	s.Zero(heals)
	s.Empty(dead.Conditions, "the record handed in is untouched")
}
