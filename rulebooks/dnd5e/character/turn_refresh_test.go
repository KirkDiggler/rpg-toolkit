// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// TurnRefreshTestSuite pins the observable rule the stored TurnNumber exists
// for: the bank is full when your turn begins, and it gets there by being
// noticed as stale at the first ask rather than by someone remembering to
// call a turn-start verb. Nothing in this slice calls it — the door does, in
// E2 — so what ships is the behaviour, tested directly.
type TurnRefreshTestSuite struct {
	suite.Suite

	ctx context.Context
	bus events.EventBus
}

func TestTurnRefreshSuite(t *testing.T) {
	suite.Run(t, new(TurnRefreshTestSuite))
}

func (s *TurnRefreshTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func (s *TurnRefreshTestSuite) sheet() *Data {
	return &Data{
		ID:               "refresh-fighter",
		PlayerID:         "refresh-player",
		Name:             "Fresh",
		Level:            5,
		ProficiencyBonus: 3,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    44,
		MaxHitPoints: 44,
		ArmorClass:   18,
	}
}

// spent is a sheet that took its turn 1 and used all of it: no action, no
// bonus, no reaction, no movement, and a bank that has been emptied. Marked
// clean, so anything dirty afterwards came from the refresh.
func (s *TurnRefreshTestSuite) spent() *Character {
	char, err := LoadFromData(s.ctx, s.sheet(), s.bus)
	s.Require().NoError(err)

	_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	char.SpendSlots(coreCombat.ActionStandard, 1)
	char.SpendSlots(coreCombat.ActionBonus, 1)
	char.SpendSlots(coreCombat.ActionReaction, 1)
	char.SpendCapacity(combat.CapacityMovement, 30)
	char.BankCapacity(combat.CapacityAttack, 2)
	char.SpendCapacity(combat.CapacityAttack, 2)
	char.MarkClean()

	return char
}

// A new turn number finds a stale bank and fills it, and the sheet says so.
func (s *TurnRefreshTestSuite) TestAStaleBankIsFullAgainAtTheNextTurn() {
	char := s.spent()

	out, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 2, Speed: 30})
	s.Require().NoError(err)
	s.True(out.Reseeded)

	s.Equal(1, char.SlotsLeft(coreCombat.ActionStandard))
	s.Equal(1, char.SlotsLeft(coreCombat.ActionBonus))
	s.Equal(1, char.SlotsLeft(coreCombat.ActionReaction))
	s.Equal(30, char.CapacityLeft(combat.CapacityMovement))
	s.Equal(2, char.GetActionEconomy().TurnNumber)
	s.True(char.IsDirty())
}

// The turn-granted bank does not carry across the boundary: attacks banked by
// last turn's Attack action are not this turn's to spend.
func (s *TurnRefreshTestSuite) TestTheNewTurnsBankIsEmptyOfLastTurnsGrants() {
	char := s.spent()
	char.BankCapacity(combat.CapacityAttack, 2)
	char.BankCapacity(combat.CapacityFlurryStrike, 2)

	_, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 2, Speed: 30})
	s.Require().NoError(err)

	s.Equal(0, char.CapacityLeft(combat.CapacityAttack))
	s.Equal(0, char.CapacityLeft(combat.CapacityFlurryStrike))
}

// Mid-turn, the refresh is a no-op. This is the half that makes the helper
// safe to call at every ask: a second swing must not refill the bank the first
// one spent from.
func (s *TurnRefreshTestSuite) TestTheSameTurnIsNeverReseeded() {
	char := s.spent()

	out, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	s.False(out.Reseeded)

	s.Equal(0, char.SlotsLeft(coreCombat.ActionStandard))
	s.Equal(0, char.CapacityLeft(combat.CapacityMovement))
	s.False(char.IsDirty(), "a refresh that changed nothing is not something to save")
}

// Asking repeatedly within the turn stays a no-op — the freshness check reads
// the stored turn number rather than remembering whether it has run.
func (s *TurnRefreshTestSuite) TestRepeatedAsksWithinATurnChangeNothing() {
	char := s.spent()

	for i := 0; i < 3; i++ {
		out, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 1, Speed: 30})
		s.Require().NoError(err)
		s.False(out.Reseeded)
	}
	s.Equal(0, char.SlotsLeft(coreCombat.ActionStandard))
	s.False(char.IsDirty())
}

// Reseeding twice for the same new turn fills the bank once. The second ask is
// now the same-turn no-op, so a spend between the two survives.
func (s *TurnRefreshTestSuite) TestReseedingHappensOncePerTurn() {
	char := s.spent()

	_, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 2, Speed: 30})
	s.Require().NoError(err)
	char.SpendSlots(coreCombat.ActionStandard, 1)

	out, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 2, Speed: 30})
	s.Require().NoError(err)
	s.False(out.Reseeded)
	s.Equal(0, char.SlotsLeft(coreCombat.ActionStandard), "the spend survived the second ask")
}

// Any turn number that is not the stored one is stale, including one that went
// backwards. A bank left empty because a number moved the wrong way is a
// character who cannot act on their own turn.
func (s *TurnRefreshTestSuite) TestATurnNumberThatWentBackwardsIsStale() {
	char := s.spent()
	_, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 7, Speed: 30})
	s.Require().NoError(err)
	char.SpendSlots(coreCombat.ActionStandard, 1)
	char.MarkClean()

	out, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 3, Speed: 30})
	s.Require().NoError(err)
	s.True(out.Reseeded)
	s.Equal(1, char.SlotsLeft(coreCombat.ActionStandard))
	s.Equal(3, char.GetActionEconomy().TurnNumber)
}

// Movement is reseeded from the speed the caller states, not from the sheet's
// base speed — conditions modify speed, and that arithmetic belongs above this.
func (s *TurnRefreshTestSuite) TestMovementIsReseededFromTheStatedSpeed() {
	char := s.spent()

	_, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 2, Speed: 45})
	s.Require().NoError(err)
	s.Equal(45, char.CapacityLeft(combat.CapacityMovement))
}

// A sheet that is not in combat has no turn to be stale, so the refresh
// declines rather than inventing an economy. Combat starts somewhere else.
func (s *TurnRefreshTestSuite) TestASheetOutOfCombatIsNotReseeded() {
	char, err := LoadFromData(s.ctx, s.sheet(), s.bus)
	s.Require().NoError(err)
	char.MarkClean()

	out, err := char.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	s.False(out.Reseeded)
	s.False(char.InCombat())
	s.False(char.IsDirty())
}

// The freshness helper refuses a call with nothing in it rather than reseeding
// to turn zero at zero speed.
func (s *TurnRefreshTestSuite) TestRefreshWithoutInputIsRefused() {
	char := s.spent()

	_, err := char.RefreshForTurn(s.ctx, nil)
	s.Require().Error(err)
	s.Equal(1, char.GetActionEconomy().TurnNumber)
	s.False(char.IsDirty())
}

// Nothing in this slice calls the helper in production: E2's door does. The
// turn-start verb that exists today still works exactly as it did, and the two
// agree about what a fresh turn looks like.
func (s *TurnRefreshTestSuite) TestARefreshedTurnMatchesAStartedOne() {
	started, err := LoadFromData(s.ctx, s.sheet(), s.bus)
	s.Require().NoError(err)
	_, err = started.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 4, Speed: 30})
	s.Require().NoError(err)

	refreshed := s.spent()
	_, err = refreshed.RefreshForTurn(s.ctx, &RefreshForTurnInput{TurnNumber: 4, Speed: 30})
	s.Require().NoError(err)

	s.Equal(started.GetActionEconomy(), refreshed.GetActionEconomy())
}
