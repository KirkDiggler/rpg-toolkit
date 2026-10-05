// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// attackChainFrame is the frame an attack-chain handler asks its rule with.
//
// The attack chain carries no frame yet — resolution builds one for the damage
// fold and the post-roll offers, after the attack chain has folded — so this
// reads only the facts the event itself carries: attacker, target and whether
// the attack is melee. Every other fact stays unknown, so a rule that needs
// one answers Depends and executeRule fails the attack instead of switching
// the rule off. It goes away when resolution hands the attack chain its frame.
func attackChainFrame(event dnd5eEvents.AttackChainEvent) contributions.Frame {
	frame := contributions.Frame{
		Actor:  event.AttackerID,
		Target: contributions.Unknown[string](),
		Action: contributions.ActionFacts{
			Roll:  contributions.Known(contributions.RollKindAttack),
			Melee: contributions.Known(event.IsMelee),
		},
	}
	if event.TargetID != "" {
		frame.Target = contributions.Known(event.TargetID)
	}
	return frame
}

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
	return out, nil
}

// assessed is a contributes-now answer with no benefit or contribution.
func assessed(state contributions.Applicability, reason string) *contributions.AssessActionOutput {
	return &contributions.AssessActionOutput{Answer: contributions.Answer{
		Decision:      contributions.Decision{Applicability: state, Reason: reason},
		Participation: contributions.ContributesNow,
	}}
}
