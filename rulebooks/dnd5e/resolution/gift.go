// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"maps"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// giftMachine runs an ability that is only its price: a description with no
// profile whose cost gives a turn slot (OnlyItsPrice on the description),
// Action Surge's shape.
//
// # It lands nothing
//
// The charge and the gift both happen at the door, in one [combat.Pay], before
// this machine's step runs; the price is the whole of the ability. The step
// therefore changes no sheet and publishes nothing. It reports what the price
// gave, in the shape an activation that banks something already reports
// ([EffectCapacityGranted], [ActivationOutcome.GrantedCapacity]), so a session
// projects it with no new code.
//
// # Its payment is not optional
//
// A call that sent no payment, or another one, would report a gift nobody was
// given. [Input.Validate] refuses those before anything loads: see
// [Input.validateOnlyItsPrice].
type giftMachine struct {
	ref     string
	actorID string
	// gives is the description's own gift, cloned, which the payment must equal.
	gives map[coreCombat.ActionType]int
	words string
}

// newGift builds the machine for a description that is only its price. The
// actor is [ActionInput.AttackerID]. An empty actor is refused, and so is any
// named target other than the actor: the gift lands on nobody. The actor named
// as the one target is accepted. Refusals are [ErrBadAction]. It needs no
// roller.
func newGift(in *ActionInput, normalizedTargetIDs []string) (Machine, error) {
	definition := in.Definition.Clone()
	ref := definition.Ref.String()
	if in.AttackerID == "" {
		return nil, fmt.Errorf("%w: %s was used by nobody", ErrBadAction, ref)
	}
	if len(normalizedTargetIDs) > 1 || (len(normalizedTargetIDs) == 1 && normalizedTargetIDs[0] != in.AttackerID) {
		return nil, fmt.Errorf("%w: %s gives to its user and names nobody else; got targets %v",
			ErrBadAction, ref, normalizedTargetIDs)
	}

	return &giftMachine{
		ref:     ref,
		actorID: in.AttackerID,
		gives:   maps.Clone(definition.Cost.GrantSlots),
		words:   combat.GrantedWords(definition.Cost),
	}, nil
}

// Start is pure preflight: the actor must be in the run. It yields the one
// step, which lands nothing and reports the gift.
func (m *giftMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	_, isCharacter := cast.Character(m.actorID)
	_, isMonster := cast.Monster(m.actorID)
	if !isCharacter && !isMonster {
		return nil, fmt.Errorf("%w: %s was used by %q, who is not in the run", ErrBadAction, m.ref, m.actorID)
	}

	return Gather{
		name: fmt.Sprintf("give %s for %s", m.ref, m.actorID),
		run: func(context.Context, events.EventBus) (Step, error) {
			return Done{Outcome: ActivationOutcome{
				Ability:         m.ref,
				GrantedCapacity: m.words,
				Effects: []ActivationEffect{{
					Kind: EffectCapacityGranted, TargetID: m.actorID, Description: m.words,
				}},
			}}, nil
		},
	}, nil
}

// validateOnlyItsPrice refuses a payment that is not exactly the gift the
// description states. The gift IS the cost, so a call that carried no cost, a
// cost that charges nothing, another payer's, or a different gift would report
// a gift that was never given. It runs in Validate, before the world is loaded
// and before anything is charged, because Start never sees the cost.
func (in *Input) validateOnlyItsPrice() error {
	m, ok := in.Machine.(*giftMachine)
	if !ok {
		return nil
	}
	if in.Cost == nil || in.Cost.Profile == nil {
		return fmt.Errorf("%w: %s is only its price and carries none", ErrBadAction, m.ref)
	}
	if !chargesSomething(in.Cost.Profile) {
		return fmt.Errorf("%w: %s is only its price and its price charges nothing", ErrBadAction, m.ref)
	}
	if in.Cost.PayerID != m.actorID {
		return fmt.Errorf("%w: %s is paid by the one who uses it: %q uses it, %q was named to pay",
			ErrBadAction, m.ref, m.actorID, in.Cost.PayerID)
	}
	if !maps.Equal(in.Cost.Profile.GrantSlots, m.gives) {
		return fmt.Errorf("%w: %s gives %s and its payment gives %s", ErrBadAction, m.ref,
			givenOrNothing(m.words), givenOrNothing(combat.GrantedWords(in.Cost.Profile)))
	}
	return nil
}

// givenOrNothing words an empty gift as "nothing" so a refusal reads.
func givenOrNothing(words string) string {
	if words == "" {
		return "nothing"
	}
	return words
}
