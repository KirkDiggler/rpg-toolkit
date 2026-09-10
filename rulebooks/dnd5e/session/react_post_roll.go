// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// answerPostRoll finishes an attack that stopped after its d20.
//
// # It resumes the SAME strike rather than swinging again
//
// The window froze the machine's own state, and this hands it straight back
// through [resolution.NewStrikeResumed] with the answer. Nothing is re-rolled
// and nothing is re-folded: the d20 the player was shown is the d20 the fight
// is decided on, and re-folding the attack chain would make every subscriber
// that records an attempt record two.
//
// # The action was already paid for
//
// The cost was charged at the door of the FIRST Resolve, before the chain
// folded, so this one passes no Cost at all. A second charge would bill the
// attacker's action twice for one swing, and the second bill would look
// exactly like the first.
//
// # It writes the beat the first call did not
//
// [Manager.Attack] recorded a roll-window beat and no outcome, because there
// was no outcome yet. This records the struck or missed beat, with the same
// presentation id the client has been correlating its own throw against since
// before the dice fell — so the throw the player watched belongs to the beat
// that lands.
//
// # The encounter is not resumed, because it was never paused
//
// [encounter.Encounter.Paused] is false throughout: a player's own attack is
// not a driven turn and has no remaining path. React's shipped guard already
// asks the encounter whether it is paused before resuming it, so the two kinds
// of window need no discrimination there.
func (m *Manager) answerPostRoll(
	ctx context.Context, scope *writeScope, window interrupt.Window, choice ReactChoice,
) (*ReactOutput, error) {
	payload, err := thawPostRollPayload(window.Payload, string(window.Audience))
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	answer := resolution.OfferKeep
	if choice == ReactStrike {
		answer = resolution.OfferSpend
	}

	machine, err := resolution.NewStrikeResumed(&resolution.StrikeResumeInput{
		Frozen: payload.Frozen,
		Answer: answer,
		// The offered die is rolled with the HOST'S dice, through the same seam
		// every other roll in this package takes. There is no process-global
		// fallback for this verb to inherit.
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("react: %w", translateResolution(err))
	}

	roster, err := scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("react: %w", translate(err))
	}

	// EVERYONE IS ATTACHED, the same cast a walk gathers. The resumed half
	// folds the damage chain and applies conditions among exactly the
	// subscribers the first half folded among; loading only the two named
	// members would resolve the second half of one swing in a smaller world
	// than its first half.
	cast := m.walkCast(ctx, scope, roster)

	world := scope.enc.WorldView()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:         world,
		Participants:  cast,
		Initiative:    m.initiative,
		Standing:      scope.standing,
		Sight:         &sightSeam{members: worldMembers(world)},
		Equipment:     equipmentBeside(scope.standing),
		TurnDriver:    m.turnDriver,
		CheckResolver: checkSeam{m: m, scope: scope},
		Witness:       witnessSeam{scope: scope},
		Machine:       machine,
		// Cost is nil ON PURPOSE — see this function's own doc.
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("react: %w", translateResolution(err))
	}
	if out.Posed != nil {
		// One pose per swing is what this slice drives. A machine that posed
		// again would mean the resumed half offered a second die, which
		// nothing builds; storing the second question would be inventing a
		// design nobody wrote.
		return nil, fmt.Errorf("react: %w: the resumed strike posed a second question", ErrInvalidWorld)
	}
	struck, ok := out.Outcome.(resolution.StrikeOutcome)
	if !ok {
		return nil, fmt.Errorf("react: %w: resumed strike produced %T", ErrInvalidWorld, out.Outcome)
	}

	if err := m.adopt(scope, out.World); err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	if err := m.saveDirty(ctx, scope, out); err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}

	// Closed BEFORE the beat, so a failure to record leaves a session whose
	// ledger and story disagree in the direction that fails closed: the window
	// stays open only if nothing was written at all.
	if err := answerWindow(scope, window, choice); err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true

	if _, err := scope.enc.Record(recordStrike(
		payload.Audience, payload.Target, struck, payload.Attack, payload.PresentationID,
		out.ConcentrationChecks, out.ConcentrationBreaks,
	)); err != nil {
		return nil, fmt.Errorf("react: %w", reportUnrecorded(scope, translate(err)))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("react: %w", err)
	}
	return &ReactOutput{Saved: report, Delivery: delivery}, nil
}

// postRollDeclaration compiles one open post-roll window into the row its
// audience sees.
//
// TARGET NONE AND NO CANDIDATES. The mover candidate a reaction window carries
// is a fact about a STEP — who is walking away — and there is no step here.
// The question is about a number the player already rolled, and the only two
// answers are spend and keep.
//
// The SLOT IS NONE, which is the one place this row differs from a reaction's
// by more than its candidates. Answering costs no reaction: the price is the
// thing being offered, and it is not one of the turn's three shapes. A row
// claiming SlotReaction would tell the player they were about to spend
// something they are not.
func postRollDeclaration(session, member string, window interrupt.Window) (Declaration, error) {
	payload, err := thawPostRollPayload(window.Payload, string(window.Audience))
	if err != nil {
		return Declaration{}, err
	}
	id, err := reactDeclarationID(session, member, window.ID)
	if err != nil {
		return Declaration{}, err
	}
	offer := payload.Offer
	return Declaration{
		Verb:       VerbReact,
		Slot:       SlotNone,
		Available:  true,
		ID:         id,
		Reaction:   &offer,
		TargetKind: TargetNone,
		Candidates: []TargetCandidate{},
	}, nil
}

// recordStrike turns a finished strike into the outcome the composition will
// stamp, from the identities a caller holds rather than from a compiled
// definition.
//
// It exists because the RESUMED half of a paused attack has no compiled offer
// to read a definition off — the offer was compiled, priced and selected
// before the pause — and reconstructing one to recover two strings would be a
// second answer to what was swung. [recordFor] is this with an AttackInput and
// a definition in front of it.
// recordStrike builds the record one swing produces, including everything the
// swing's concentration consequences ride in on.
//
// # The breaks and checks are PASSED THROUGH, not built here
//
// Resolution hands over [encounter.ConcentrationCheck] and
// [encounter.ConcentrationBreak] already assembled — the rolls, the reasons,
// the stripped addresses and the names they go out under. This seam copies two
// slice headers onto the record and knows nothing about what is in them, which
// is the whole of Kirk's ruling made structural: *"resolution is the place
// that should resolve things. it shouldn't have to leak out."* An earlier draft
// of this file projected those shapes here, and every line of it was this seam
// having an opinion about a rule.
//
// They are supplied on EVERY strike path — a player's swing, a monster's, and
// a resumed one — because they are written here rather than at the three call
// sites. A path that assembled its own record would be a path where a
// defender's concentration silently survives.
func recordStrike(
	attacker, target string, struck resolution.StrikeOutcome, ref AttackRef, presentationID string,
	checks []encounter.ConcentrationCheck, breaks []encounter.ConcentrationBreak,
) *encounter.RecordInput {
	values := map[encounter.OutcomeValue]int{
		encounter.ValueRoll:    struck.Roll,
		encounter.ValueTotal:   struck.Total,
		encounter.ValueAgainst: struck.TargetAC,
	}
	kind := encounter.OutcomeMissed
	if struck.Hit {
		kind = encounter.OutcomeStruck
		values[encounter.ValueAmount] = struck.Damage
	}

	recorded := &encounter.RecordInput{
		Kind:     kind,
		Actor:    encounter.MemberID(attacker),
		Targets:  []encounter.MemberID{encounter.MemberID(target)},
		Values:   values,
		Critical: struck.Critical,
		Attack: &encounter.AttackIdentity{
			Ref: ref.Ref, Name: ref.Name, DamageType: string(ref.DamageType),
		},

		PresentationID: presentationID,
		Calculation:    rollCalculationFor(struck.Calculation),
	}
	if struck.Hit {
		recorded.DamageComponents = recordDamageComponents(struck.DamageComponents)
		recorded.AdvantageSources = recordAttackModifierSources(struck.Folded.AdvantageSources)
		recorded.DisadvantageSources = recordAttackModifierSources(struck.Folded.DisadvantageSources)
	}

	// Set outside the Hit arm on purpose: whether a miss can end or test a
	// concentration is a rulebook fact, and resolution answers it by handing
	// over empty lists. A guard here would be this seam deciding it.
	recorded.ConcentrationChecks = checks
	recorded.ConcentrationBreaks = breaks
	return recorded
}
