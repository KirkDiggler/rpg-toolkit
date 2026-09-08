package resolution

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ActionInput identifies a shared action definition and the participants it targets.
//
// AttackerID is whoever acts — the swinger of a strike, the CASTER of a cast.
// The name is the strike's and stays the strike's: renaming it is a mechanical
// change across every call site and it should ride a third profile arm rather
// than this one.
type ActionInput struct {
	Definition combatActions.Definition
	AttackerID string
	TargetID   string
	Roller     dice.Roller
}

// NewAction validates an inert definition and dispatches by populated profile arm.
//
// # The word "cast" does not reach a machine
//
// This is the single place content chooses a sequence, and the cast arm adds no
// machine to choose from. A definition whose cast profile carries a gate is a
// [NewContest] — a save, and what a failed one delivers. One without a gate has
// nothing to resolve, so it is a DELIVERY: the conditions are built here and
// published on the interaction's bus by the activation machine's arm, which is
// the collector every activation result already travels through.
//
// Nothing below this function names a spell, a cantrip or a school. It sees a
// profile with a gate or without one (ADR-0045).
func NewAction(in *ActionInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if err := in.Definition.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadAction, err)
	}
	if in.Definition.Attack != nil {
		return NewStrike(&StrikeInput{
			AttackerID: in.AttackerID,
			TargetID:   in.TargetID,
			Definition: in.Definition.Clone(),
			Roller:     in.Roller,
		}), nil
	}
	if in.Definition.Cast != nil {
		return newCast(in)
	}
	return nil, fmt.Errorf("%w: definition %q has no supported profile", ErrBadAction, in.Definition.Ref.String())
}

// newCast reads a cast profile and returns the machine that already exists for
// the shape it declares.
//
// The definition is CLONED first and everything below reads the clone: the
// save cause holds a pointer to its ref, and a caller reusing its definition
// must not be able to rewrite what a running interaction says caused the save.
func newCast(in *ActionInput) (Machine, error) {
	definition := in.Definition.Clone()
	profile := definition.Cast
	casterID, targetID := in.AttackerID, in.TargetID

	if casterID == "" {
		return nil, fmt.Errorf("%w: %s was cast by nobody", ErrBadAction, definition.Ref.String())
	}
	if err := checkCastTarget(profile.Target, definition.Ref, targetID); err != nil {
		return nil, err
	}

	// The cause is the spell, and it is what every downstream record reads to
	// say WHAT was saved against and WHO cast it.
	cause := dnd5eEvents.SaveCause{
		Trigger:      dnd5eEvents.SaveTriggerSpell,
		EffectRef:    &definition.Ref,
		InstigatorID: casterID,
	}

	if profile.Save != nil {
		return newGatedCast(definition, casterID, targetID, cause, in.Roller)
	}

	return newGatelessCast(definition, casterID, targetID)
}

// checkCastTarget enforces what the profile's target rule promises. A rule the
// call cannot satisfy is a caller defect: a self-targeted cast handed a target
// is a client that believes it aimed something that aims at nobody, and a
// one-creature cast handed none would resolve against an empty id.
func checkCastTarget(rule combatActions.CastTargetRule, ref core.Ref, targetID string) error {
	switch rule {
	case combatActions.CastTargetSelf:
		if targetID != "" {
			return fmt.Errorf("%w: %s reaches only its caster, but %q was named",
				ErrBadAction, ref.String(), targetID)
		}
	case combatActions.CastTargetOneCreature:
		if targetID == "" {
			return fmt.Errorf("%w: %s names one creature and none was named",
				ErrBadAction, ref.String())
		}
	default:
		return fmt.Errorf("%w: %s declares an unknown target rule %q",
			ErrBadAction, ref.String(), rule)
	}

	return nil
}

// newGatedCast is a save and what failing it costs, which is exactly
// [NewContest]. The saver is the creature the cast named; the damage and the
// condition are the profile's, handed over as the contest's two consequences.
//
// # One condition, on the saver
//
// The contest's scope is one saver, one gate, one condition and one damage set,
// so a profile declaring more than one effect, or one that lands on the caster,
// is REFUSED rather than half-delivered. Neither exists in content today and
// both are real shapes — a spell that damages its target and buffs its caster
// on the same failed save arrives with its own customer, and it will widen the
// contest rather than being smuggled through this branch.
func newGatedCast(
	definition combatActions.Definition, casterID, targetID string,
	cause dnd5eEvents.SaveCause, roller dice.Roller,
) (Machine, error) {
	profile := definition.Cast

	var application combatActions.ConditionApplication
	switch len(profile.Effects) {
	case 0:
	case 1:
		effect := profile.Effects[0]
		if effect.Recipient != combatActions.CastRecipientTarget {
			return nil, fmt.Errorf(
				"%w: %s contests a save and delivers to its caster, which no contest can do",
				ErrBadAction, definition.Ref.String())
		}
		parameters, err := bindCounterpart(effect, casterID)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrBadAction, definition.Ref.String(), err)
		}
		application = combatActions.ConditionApplication{Ref: effect.Ref, Parameters: parameters}
	default:
		return nil, fmt.Errorf("%w: %s declares %d conditions on one save, and a contest delivers one",
			ErrBadAction, definition.Ref.String(), len(profile.Effects))
	}

	return NewContest(&ContestInput{
		Gate:        profile.Save,
		SaverID:     targetID,
		Application: application,
		Damage:      profile.Damage,
		Cause:       cause,
		Roller:      roller,
	}), nil
}

// newGatelessCast is a delivery, not a resolution. Nobody resists it, so there
// is nothing to roll and no policy to apply: the conditions are built here —
// pure, before the door charges anybody — and published on the interaction's
// bus by the activation machine's cast arm.
//
// Damage with no gate is refused. Undefended damage is a real shape (a magic
// missile has one) and it is not this one: every consequence in this slice is
// either contested or a condition, and a branch that silently dropped a
// declared damage pool would be the affordance-with-nothing-behind-it this
// stack keeps finding.
func newGatelessCast(definition combatActions.Definition, casterID, targetID string) (Machine, error) {
	profile := definition.Cast
	if len(profile.Damage) > 0 {
		return nil, fmt.Errorf("%w: %s deals damage with no save, which this module cannot yet deliver",
			ErrBadAction, definition.Ref.String())
	}

	deliveries := make([]preparedDelivery, 0, len(profile.Effects))
	for _, effect := range profile.Effects {
		recipientID, counterpartID := casterID, targetID
		if effect.Recipient == combatActions.CastRecipientTarget {
			recipientID, counterpartID = targetID, casterID
		}
		parameters, err := bindCounterpart(effect, counterpartID)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrBadAction, definition.Ref.String(), err)
		}
		prepared, err := prepareCondition(
			combatActions.ConditionApplication{Ref: effect.Ref, Parameters: parameters},
			recipientID, definition.Ref.String(),
		)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, preparedDelivery{condition: prepared, recipientID: recipientID})
	}

	return NewActivation(&ActivationInput{
		MemberID: casterID,
		TargetID: targetID,
		cast:     &preparedCast{source: definition.Ref, conditions: deliveries},
	})
}

// bindCounterpart writes the cast's OTHER party into the parameter the effect
// names, and hands back the condition's complete configuration.
//
// The key is DECLARED by content rather than conventional here, because the two
// conditions this slice ships spell it differently and mean different things by
// it: True Strike is keyed to the target its advantage is good against, Vicious
// Mockery records the bard who imposed it. Both factories refuse a config
// without it, so a binding this function skipped would fail at build time
// rather than resolving into a condition bound to nobody.
func bindCounterpart(effect combatActions.CastEffect, counterpartID string) (json.RawMessage, error) {
	if effect.CounterpartKey == "" {
		return append(json.RawMessage(nil), effect.Parameters...), nil
	}
	if counterpartID == "" {
		return nil, fmt.Errorf("condition %s binds %q to the cast's other party, and there is none",
			effect.Ref.String(), effect.CounterpartKey)
	}

	fields := map[string]json.RawMessage{}
	if len(effect.Parameters) > 0 {
		if err := json.Unmarshal(effect.Parameters, &fields); err != nil {
			return nil, fmt.Errorf("condition %s parameters are not an object: %w", effect.Ref.String(), err)
		}
	}
	encoded, err := json.Marshal(counterpartID)
	if err != nil {
		return nil, fmt.Errorf("condition %s counterpart %q: %w", effect.Ref.String(), counterpartID, err)
	}
	fields[effect.CounterpartKey] = encoded

	bound, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("condition %s parameters: %w", effect.Ref.String(), err)
	}

	return bound, nil
}
