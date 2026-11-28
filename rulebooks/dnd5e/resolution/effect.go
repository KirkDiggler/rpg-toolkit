// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// effectMachine runs an action whose body is an effect on its user: the heal
// its declaration states, delivered by the deliverer a healing spell uses.
//
// # It pays nothing
//
// Payment is the runner's, at the one door every verb pays at. This machine is
// NOT a cast: it is not a [*castMachine], so [payForMachine] hands its cost to
// [payAtTheDoor] alone and no spell rule applies to it.
//
// # Delivery is the activation machine's gateless arm
//
// The heal is a [preparedHealing] handed to that arm, so the effect is
// collected and published exactly as a healing spell's is, and the caller gets
// an [ActivationOutcome] carrying the healing-applied fact: the same shape a
// feature activation reports, so a session projects it without a new one.
type effectMachine struct {
	delivery Machine
}

// newEffect builds the machine for a definition whose body is an effect on its
// user. The actor is [ActionInput.AttackerID]. An empty actor is refused, and
// so is any named target other than the actor: a self effect names nobody, and
// the actor named as the one target is accepted as naming itself. Refusals are
// [ErrBadAction].
func newEffect(in *ActionInput, normalizedTargetIDs []string) (Machine, error) {
	definition := in.Definition.Clone()
	profile := definition.Effect
	if profile.To != combatActions.EffectSelf || (profile.Healing == nil && profile.Condition == nil) {
		return nil, fmt.Errorf("%w: %s effect has no supported body", ErrBadAction, definition.Ref.String())
	}
	actorID := in.AttackerID
	if actorID == "" {
		return nil, fmt.Errorf("%w: %s was used by nobody", ErrBadAction, definition.Ref.String())
	}
	if len(normalizedTargetIDs) > 1 || (len(normalizedTargetIDs) == 1 && normalizedTargetIDs[0] != actorID) {
		return nil, fmt.Errorf("%w: %s lands on its user and names nobody else; got targets %v",
			ErrBadAction, definition.Ref.String(), normalizedTargetIDs)
	}

	ref := definition.Ref
	prepared := &preparedCast{source: ref}
	if profile.Healing != nil {
		if in.Roller == nil {
			return nil, fmt.Errorf("%w: healing requires a roller", ErrBadAction)
		}
		prepared.healing = &preparedHealing{
			declaration: profile.Healing.Clone(),
			targetID:    actorID,
			source:      dnd5eEvents.RollSource{Ref: &ref, Name: definition.Name, SourceID: actorID},
			roller:      in.Roller,
		}
	} else {
		// The status is built HERE, before anything is charged: its ref and
		// values as written, on its user, with the ability and its user as its
		// origin. A status that cannot be built is refused with the catalog's
		// own sentence.
		built, err := prepareConditionFrom(*profile.Condition, actorID, "",
			&dnd5eEvents.ConditionOrigin{Ref: ref, EntityID: actorID})
		if err != nil {
			if !errors.Is(err, ErrBadAction) {
				err = fmt.Errorf("%w: %w", ErrBadAction, err)
			}
			return nil, err
		}
		prepared.conditions = []preparedDelivery{{condition: built, recipientID: actorID}}
		prepared.conditionSource = dnd5eEvents.ConditionSourceFeature
	}
	delivery, err := NewActivation(&ActivationInput{MemberID: actorID, TargetID: actorID, cast: prepared})
	if err != nil {
		return nil, err
	}

	return &effectMachine{delivery: delivery}, nil
}

// Start is pure preflight: the actor must be in the run and may receive
// healing, by the rule healing spells use. It yields the step that delivers.
func (m *effectMachine) Start(ctx context.Context, cast *Participants) (Step, error) {
	return m.delivery.Start(ctx, cast)
}
