// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// The words this package knows how to obey.
//
// DECLARED HERE RATHER THAN IMPORTED FROM CONTENT, and the direction is the
// point: what "approach" MEANS — toward the caster, on this turn's movement,
// provoking — is a rule, and rules are this package's (ADR-0045 puts the
// declaration in content and the execution here). Content declares the MENU:
// which ids a client may choose from and what a person reads beside each one.
//
// The two lists have to agree, and nothing in the type system makes them.
// TestEveryWordOnCommandsMenuIsAWordThisPackageObeys is what makes a
// disagreement loud, because the failure otherwise is silent and looks like
// nothing: a creature under an order this package has no arm for would spend
// its turn standing exactly where a creature under no order stands.
//
// Unexported because Obey is the only door onto them. A caller that could name
// a word would be a second authority on what the word does.
const (
	wordApproach = "approach"
	wordFlee     = "flee"
	wordGrovel   = "grovel"
)

// ObeyInput is one compelled creature's turn: who is compelled, by whom, which
// word, and the interaction the word happens inside.
type ObeyInput struct {
	// Interaction is the cast, the world and the seams, built exactly as a verb
	// builds them for any other resolution. Obey supplies the machine, so
	// Interaction.Machine must be nil: the word IS the machine here.
	//
	// A WHOLE INTERACTION RATHER THAN A BAG OF SHEETS, because one of the three
	// words applies a condition, and a condition is applied by publishing on a
	// bus that only this package may create (ADR-0038). Approach and Flee
	// publish nothing and would be happy with less; a second, thinner door for
	// them would be a second answer to "what does the word do", which is the
	// one thing a closed vocabulary exists to prevent.
	Interaction *Input

	// Member is the compelled creature, and the recipient of anything the word
	// leaves behind.
	Member string

	// CasterID is who gave the order. Approach and Flee are measured from them,
	// which is why the compulsion stored it rather than deriving it later: the
	// caster may be dead, unconscious or across the map by the time the turn
	// comes round, and the anchor is still the anchor.
	CasterID string

	// Word is which order was given: one of the ids the compulsion stored, which
	// are the ids the Command profile's menu lists. An id this package has no arm for is refused rather than passed
	// over, because a compelled creature whose word did nothing would spend its
	// turn looking exactly like a creature that was never compelled.
	Word string

	// Source is what the effects name as their cause — the Commanded condition,
	// for everything that obeys today. It is the caller's to state for the same
	// reason publishCondition's source is: this package does not guess what put
	// somebody under an order.
	Source core.Ref
}

// ObeyOutcome is what one obeyed word produced.
type ObeyOutcome struct {
	// Effects is what the word imposed, in the order the rules imposed them.
	Effects []ImposedEffect
}

func (ObeyOutcome) isOutcome() {}

// ObeyOutput is the obeyed word's effects and the interaction that produced
// them.
type ObeyOutput struct {
	// Effects is what the word imposed. A move is DESCRIBED here and walked by
	// whoever owns the board; a condition is already applied by the time this
	// comes back.
	Effects []ImposedEffect

	// Resolved is the interaction's own output: the world, the dirty sheets and
	// the hooks. The caller saves it exactly as it saves any other verb's, which
	// is how a grovelled creature's prone condition becomes durable.
	Resolved *Output
}

// Obey is what a compelled creature does on its turn, in the same vocabulary a
// cast's consequences travel in.
//
// # It moves nobody
//
// Exactly as imposeMove moves nobody. Approach and Flee come back as an
// [ImposedMove] naming a policy, an anchor and a budget, and the cells are read
// from the field under encounter's own fold — so a compelled walk obeys the
// same refusals a chosen step does and there is no second author of passability
// (rpg-project#431 §2). Grovel is the exception, and it is the one this package
// already owns: a condition is APPLIED here, on the interaction's bus, the way
// a cast's is.
//
// # Every word ends the turn, and that is not one of these effects
//
// The text ends the turn for all three orders, and nothing below says so. Ending
// a turn is the composition's, and it reaches it as the terminal nature of the
// intent the driver returns rather than as a fourth kind of consequence this
// package would then be describing without owning.
//
// # Why the words are a closed switch here and not content
//
// Because each one is a RULE — what "approach" means is "toward the caster, on
// this turn's movement, provoking" — and a rule is this package's to hold
// (ADR-0045 puts the DECLARATION in content, and content declares the menu).
// A word the switch has no arm for is [ErrBadAction] naming the word; Halt is
// deliberately absent (design §7.4).
func Obey(ctx context.Context, in *ObeyInput) (*ObeyOutput, error) {
	machine, err := newObey(in)
	if err != nil {
		return nil, err
	}

	interaction := *in.Interaction
	interaction.Machine = machine
	out, err := Resolve(ctx, &interaction)
	if err != nil {
		return nil, err
	}

	outcome, ok := out.Outcome.(ObeyOutcome)
	if !ok {
		return nil, fmt.Errorf("%w: obeying %q produced %T", ErrBadStep, in.Word, out.Outcome)
	}

	return &ObeyOutput{Effects: outcome.Effects, Resolved: out}, nil
}

// obeyMachine is one word, as steps over data.
type obeyMachine struct {
	in *ObeyInput
}

// newObey validates the order and refuses a word nothing can obey, before any
// interaction is opened.
func newObey(in *ObeyInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.Interaction == nil {
		return nil, fmt.Errorf("%w: obeying a word happens inside an interaction", ErrNilInput)
	}
	if in.Interaction.Machine != nil {
		return nil, fmt.Errorf("%w: an obeyed word is its own machine", ErrBadAction)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("%w: nobody was compelled", ErrBadAction)
	}
	if in.CasterID == "" {
		return nil, fmt.Errorf("%w: %q was compelled by nobody", ErrBadAction, in.Member)
	}
	switch in.Word {
	case wordApproach, wordFlee, wordGrovel:
	default:
		return nil, fmt.Errorf("%w: %q is not a word anything knows how to obey", ErrBadAction, in.Word)
	}

	return &obeyMachine{in: in}, nil
}

// Start is pure: the two walking words describe themselves and finish, and the
// one that leaves something behind builds it here and publishes it in a step.
func (m *obeyMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	switch m.in.Word {
	case wordApproach:
		return Done{Outcome: ObeyOutcome{Effects: []ImposedEffect{m.walk(combatActions.MoveToward)}}}, nil
	case wordFlee:
		return Done{Outcome: ObeyOutcome{Effects: []ImposedEffect{m.walk(combatActions.MoveAway)}}}, nil
	case wordGrovel:
		return m.grovel(cast)
	default:
		// newObey closed the vocabulary before the interaction opened, so this
		// is a defect in this file rather than a bad order. It refuses instead
		// of finishing empty, because a compelled turn that imposed nothing
		// looks exactly like a turn nobody was compelled on.
		return nil, fmt.Errorf("%w: %q reached the machine with no arm", ErrBadAction, m.in.Word)
	}
}

// walk describes the move the word asks for.
//
// The budget is the TURN, because the whole of a compelled turn is the walk:
// "moves toward you by the shortest and most direct route" and "spends its turn
// moving away from you" are both the creature's movement, not a shove's fixed
// distance and not a second speed's worth of it.
//
// Provokes, because the creature is walking on its own legs. Compelled is not
// forced in the sense the opportunity-attack fold means: a creature marched out
// of a reach by an order is leaving that reach the way a frightened creature
// does, and the reach gets its swing.
//
// Pays nothing, because the turn IS the price. There is no PaysTurn and no arm
// in payForMove for one.
func (m *obeyMachine) walk(policy combatActions.MovePolicy) ImposedEffect {
	source := m.in.Source
	directive := &MoveDirective{
		Policy:   policy,
		AnchorID: m.in.CasterID,
		Turn:     true,
		Pays:     combatActions.PaysNothing,
		Provokes: true,
	}

	return ImposedEffect{
		Kind:        ImposedMove,
		Ref:         &source,
		Description: describeMove(*directive),
		RecipientID: m.in.Member,
		Move:        directive,
	}
}

// grovel puts the creature prone, here, on this interaction's bus.
//
// APPLIED RATHER THAN DESCRIBED, and the asymmetry with the two walking words
// is the division this package already draws: a condition is finished when this
// package is done with it and a move is not. "Falls prone and then ends its
// turn" is a condition landing, so it lands the way every other condition
// lands — through the same publish, with the same replacement rule, on the same
// bus.
//
// The SOURCE handed to the factory is the compulsion rather than Command
// itself, because the order is what put the creature on the floor: the spell
// chose the word a turn earlier and may have ended by now. Prone keeps no
// source of its own today, so nothing on the sheet reads it back — the
// statement is what the next word to apply a condition that DOES keep one will
// inherit, rather than a guarantee this one can make.
func (m *obeyMachine) grovel(cast *Participants) (Step, error) {
	prepared, err := prepareCondition(
		combatActions.ConditionApplication{Ref: *refs.Conditions.Prone()},
		m.in.Member, m.in.Source.String(),
	)
	if err != nil {
		return nil, err
	}
	if _, err := cast.entity(m.in.Member); err != nil {
		return nil, err
	}

	return publishPreparedCondition(
		prepared, cast, m.in.Member, dnd5eEvents.ConditionSourceSpell,
		func(replaced []ImposedEffect) (Step, error) {
			effects := append([]ImposedEffect(nil), replaced...)

			return Done{Outcome: ObeyOutcome{
				Effects: append(effects, prepared.atStake(m.in.Member)),
			}}, nil
		},
	), nil
}
