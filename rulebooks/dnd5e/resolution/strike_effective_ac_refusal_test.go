// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// acRefusingTarget is a defender whose sheet cannot derive an AC — the shape a
// character loaded off the bus-free Load has.
//
// Its AC() deliberately returns a perfectly plausible 13. That number is the
// whole point: it is what the old fallback would have handed the strike, and it
// is indistinguishable from a real answer, so a test that only checked "an
// error happened" would not notice the strike swinging at it anyway.
type acRefusingTarget struct{ id string }

func (t *acRefusingTarget) GetID() string        { return t.id }
func (t *acRefusingTarget) GetHitPoints() int    { return 10 }
func (t *acRefusingTarget) GetMaxHitPoints() int { return 10 }
func (t *acRefusingTarget) AC() int              { return 13 }
func (t *acRefusingTarget) ApplyDamage(
	_ context.Context, _ *combat.ApplyDamageInput,
) *combat.ApplyDamageResult {
	return nil
}
func (t *acRefusingTarget) IsDirty() bool                       { return false }
func (t *acRefusingTarget) AbilityScores() shared.AbilityScores { return shared.AbilityScores{} }
func (t *acRefusingTarget) ProficiencyBonus() int               { return 2 }
func (t *acRefusingTarget) PassivePerception() int              { return 10 }
func (t *acRefusingTarget) HasShieldEquipped() bool             { return false }

// CanReact is true because this fake carries no action economy at all, and
// false would claim one refused. Nothing in this test asks; the method is here
// because combat.Member asks it of every participant.
func (t *acRefusingTarget) CanReact() bool { return true }

var errNoBus = errors.New("sheet is on no bus")

func (t *acRefusingTarget) EffectiveAC(_ context.Context) (*combat.ACBreakdown, error) {
	return nil, errNoBus
}

// A defender that cannot report its AC stops the strike.
//
// Before rpg-toolkit#1276, combat.GetEffectiveAC fell back to the flat sheet
// number whenever the fold could not run, so this target would have been
// attacked at AC 13 and the outcome would have reported 13 as though it had
// been derived. The strike now refuses instead.
//
// The positive half of this contract is already measured end to end by
// EffectiveACTestSuite (TestUnarmoredDefenseReachesTheStrike and
// TestUnarmoredDefenseDecidesTheHit): a defender that CAN fold has the folded
// number reach the roll. So this test does not need to re-prove that, and a
// blanket "EffectiveAC always errors" regression could not hide behind it.
func TestStrikeRefusesATargetThatCannotDeriveAC(t *testing.T) {
	// attack is populated so the gather can run to completion if it ever stops
	// refusing. Without it the machine panics further down on a nil profile,
	// and this test would "pass" on a crash instead of on its own assertions —
	// verified by mutation: restoring the old target.AC() fallback must fail
	// HERE, on the outcome, not incidentally somewhere after it.
	m := &strikeMachine{
		in:     &StrikeInput{AttackerID: "attacker-1", TargetID: "defender-1"},
		attack: &combatActions.AttackProfile{AttackBonus: 5},
	}
	target := &acRefusingTarget{id: "defender-1"}

	step, err := m.effectiveACStep(target, false).run(context.Background(), events.NewEventBus())

	require.Error(t, err, "a target that cannot derive its AC must stop the strike")
	require.ErrorIs(t, err, errNoBus, "the defender's own reason must survive to the caller")
	require.Nil(t, step, "a refused gather yields no step to continue the machine with")
	require.Contains(t, err.Error(), "defender-1", "the error names which combatant could not answer")
	require.NotEqual(t, 13, m.outcome.TargetAC,
		"the strike must not fall back to the flat AC() the old code would have used")
	require.Zero(t, m.outcome.TargetAC, "a refused read leaves the outcome untouched")
}
