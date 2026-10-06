// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

// attackRollText is a rule's player-facing wording. Benefit states what the
// effect does to the roll when it applies.
type attackRollText struct {
	NotOwner    string
	OnlyAttacks string
	NotTarget   string
	Applies     string
	Benefit     string
}

// attackRollRule answers for an effect on its holder's own attack rolls:
// advantage, disadvantage or a lower critical threshold. It applies to every
// attack roll its owner makes, or, when target is set, only to attacks against
// that target. It holds only the facts its predicate reads, not the
// condition, so asking it cannot spend or end the effect.
type attackRollRule struct {
	name   string
	owner  string
	target string
	text   attackRollText
	// mode is the advantage or disadvantage an applying answer carries, for
	// a rule whose handler applies it through applyAttackMode; empty for one
	// whose handler contributes something else.
	mode contributions.AttackMode
}

func (r attackRollRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, r.name)
	if err != nil {
		return nil, err
	}
	if frame.Actor != r.owner {
		return assessed(contributions.DoesNotApply, r.text.NotOwner), nil
	}
	if roll, _ := frame.Action.Roll.Get(); roll != contributions.RollKindAttack {
		return assessed(contributions.DoesNotApply, r.text.OnlyAttacks), nil
	}
	if r.target != "" {
		target, known := frame.Target.Get()
		if !known {
			return assessed(contributions.Depends, "Depends on the target"), nil
		}
		if target != r.target {
			return assessed(contributions.DoesNotApply, r.text.NotTarget), nil
		}
	}
	out := assessed(contributions.Applies, r.text.Applies)
	out.Answer.Benefit = r.text.Benefit
	out.Answer.AttackMode = r.mode
	return out, nil
}

// assessed is a contributes-now answer with no benefit or contribution.
func assessed(state contributions.Applicability, reason string) *contributions.AssessActionOutput {
	return &contributions.AssessActionOutput{Answer: contributions.Answer{
		Decision:      contributions.Decision{Applicability: state, Reason: reason},
		Participation: contributions.ContributesNow,
	}}
}
