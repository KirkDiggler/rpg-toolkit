// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

// AttackRoll is the selected natural d20 and its total before a post-roll offer.
// It is a rolled fact, not a final outcome: no AC, hit, damage, offer or answer
// belongs here. A resumed strike may change its final total without rolling
// another attack die.
type AttackRoll struct {
	AttackerID string
	TargetID   string
	Roll       int
	Total      int
}

// attackRollReporter exposes the top-level machine's captured data without
// adding a step, bus observer, or callback to the public Resolve boundary.
// Non-strike machines have no new attack roll to report.
type attackRollReporter interface {
	attackRoll() *AttackRoll
}

func reportedAttackRoll(machine Machine) *AttackRoll {
	reporter, ok := machine.(attackRollReporter)
	if !ok {
		return nil
	}
	captured := reporter.attackRoll()
	if captured == nil {
		return nil
	}
	copy := *captured
	return &copy
}

func (m *strikeMachine) attackRoll() *AttackRoll {
	return m.rolled
}
