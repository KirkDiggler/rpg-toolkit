// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ReactionAttacks answers what a reactor swings when its reaction fires.
//
// A CAPABILITY, supplied and never defaulted, for the reason every other seam
// on this package's inputs is: what a member attacks with is read off their
// equipped weapon, and this package holds no equipment rules. The caller that
// owns the sheets owns the answer.
//
// It is asked PER REACTOR at the moment a trigger fires, not handed a
// pre-selected list of who might react. That distinction is the point: a
// caller who had to name the reactors in advance would be deciding which
// effects can notice a step, which is precisely the rule-in-the-wiring this
// machine exists to avoid.
type ReactionAttacks interface {
	// AttackFor returns the attack this reactor swings, or false if it has
	// none. False is an ANSWER, not a failure: an unarmed caster with no
	// melee attack simply does not get an opportunity attack.
	AttackFor(reactorID string) (combatActions.Definition, bool)
}

// MovementInput is one step of a walk, offered to the rules.
type MovementInput struct {
	// Mover is who is stepping. Required.
	Mover encounter.MemberID

	// MoverKind is what the mover is, carried onto the chain event so a
	// subscriber can tell a character's step from a monster's without asking.
	MoverKind string

	// From and To are the step's endpoints, dungeon-absolute — the same
	// coordinates the Atlas draws and every other verb speaks.
	From spatial.Position
	To   spatial.Position

	// Reactions answers what a triggered reactor swings. Required: a movement
	// with no way to resolve a reaction would publish triggers and drop them,
	// which is the failure this whole slice exists to undo.
	Reactions ReactionAttacks

	// Roller rolls the reaction's attack. Required.
	Roller dice.Roller
}

// MovementOutcome is what one step produced.
type MovementOutcome struct {
	// Mover, From and To echo the step, so a caller reading only the outcome
	// can say what happened without holding the input.
	Mover string
	From  spatial.Position
	To    spatial.Position

	// Prevented reports that a chain subscriber stopped the step, with
	// PreventionReason saying which and why. Nothing sets it today — Sentinel
	// is deferred — and it is carried rather than dropped so the day something
	// does, the caller is already reading it.
	Prevented        bool
	PreventionReason string

	// OAPrevented reports that opportunity attacks were suppressed for this
	// step, which is how Disengage reads from out here.
	OAPrevented bool

	// Reactions are what fired, in reactor order.
	Reactions []ReactionOutcome
}

func (MovementOutcome) isOutcome() {}

// ReactionOutcome is one reaction that fired during a step.
type ReactionOutcome struct {
	// ReactorID is who reacted, ConditionRef is what let them, and Against is
	// who they reacted to.
	ReactorID    string
	ConditionRef string
	Against      string

	// Struck is what the reaction's attack produced. A reaction that found no
	// attack to swing does not appear in the outcome at all, so this is always
	// populated.
	Struck StrikeOutcome
}

// NewMovement returns the machine that offers one step of a walk to the rules.
//
// # It is a sibling of NewBoundary and NewActivation
//
// The three share the shape that matters: attach everyone, do ONE thing on the
// interaction's own bus, collect dirty sheets. A boundary is "time happened",
// an activation is "a member used something they carry", and this is "a member
// entered a cell" — none of them is a declared action with a compiled profile,
// so none of them is an arm of [NewAction].
//
// # Why a step needs its own machine at all
//
// Because the alternative was a bus at the call site with a hand-picked cast,
// and that is a rule in the wiring. Kirk ruled it out directly (rpg-project#316,
// 2026-08-28): "a walk should load everything. if there is a trap along the
// path it will need to be loaded. i think the idea we know what to load on the
// bus will hamstring us as we want to add new things in."
//
// Running here means attachment is [attachAll] — the same one every other
// machine uses — so a trap, a hazard aura, a Sentinel feat and an ally's
// Protection are each attached and each answer for themselves. The acceptance
// test is that the NEXT thing to notice a step subscribes to MovementChain and
// needs no change here to be heard.
//
// # It publishes, and encounter does not
//
// The natural-looking seam is [encounter.Step], which holds both endpoints. It
// is the wrong one: that module never imports events and publishes nothing at
// all, because determinism is its module law. So the step is walked there and
// announced here, which is the same division [announcerSeam] already makes for
// a clock boundary the composition noticed and cannot publish.
//
// # The reaction resolves INSIDE this interaction
//
// A trigger is answered with [Request], on this interaction's own bus and over
// its own cast, rather than handed back for the caller to run separately. That
// sameness is load-bearing twice over: the reactor's condition marked itself
// used during the fold, and a separate interaction would reload that sheet from
// data and find the meter empty; and an effect attached for this step
// contributes to the reaction's attack exactly as it would to a declared one.
//
// # It runs BEFORE the step is taken, and that is a contract not a preference
//
// The reactor swings through the ordinary strike path, which enforces melee
// reach against where the target IS. An opportunity attack fires precisely
// because the mover is LEAVING reach, so a caller that walks the member first
// and announces afterwards hands the strike a target who has already gone: the
// swing is refused as out of range and, because a refused reaction fails the
// interaction, the whole walk dies with it.
//
// So the caller announces the step, then takes it. That is also the order
// combat.MoveEntity used on the old stack — "fires a MovementChain event before
// each step" — and the order MovementPrevented needs to mean anything, since a
// step already taken cannot be prevented.
//
// Refuses a nil input, a mover with no ID, a step that goes nowhere, and a
// missing capability — all before the world is loaded.
func NewMovement(in *MovementInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.Mover == "" {
		return nil, fmt.Errorf("%w: no member is moving", ErrBadMovement)
	}
	if in.From == in.To {
		return nil, fmt.Errorf("%w: member %q stepped nowhere", ErrBadMovement, in.Mover)
	}
	if in.Reactions == nil {
		return nil, fmt.Errorf("%w: member %q has no way to resolve a reaction", ErrBadMovement, in.Mover)
	}
	if in.Roller == nil {
		return nil, fmt.Errorf("%w: member %q has no roller", ErrBadMovement, in.Mover)
	}

	// Copied rather than kept by pointer, for the reason NewActivation copies
	// its ref: a caller reusing the struct must not be able to change which
	// step this machine announces after it was constructed.
	cloned := *in

	return &movementMachine{in: &cloned}, nil
}

type movementMachine struct {
	in *MovementInput

	folded    *dnd5eEvents.MovementChainEvent
	triggers  []dnd5eEvents.ReactionTriggerEvent
	reactions []ReactionOutcome
}

// Start is pure preflight — NewMovement already refused what it could — and
// yields the fold without publishing.
func (m *movementMachine) Start(_ context.Context, _ *Participants) (Step, error) {
	return m.announce(), nil
}

// announce publishes the step and folds what the rules did with it.
func (m *movementMachine) announce() Step {
	return Gather{
		name: fmt.Sprintf("move %s from (%v,%v) to (%v,%v)",
			m.in.Mover, m.in.From.X, m.in.From.Y, m.in.To.X, m.in.To.Y),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			// Collect triggers for the duration of the fold only. A reaction
			// condition PUBLISHES rather than returning, because one step can
			// trigger several reactors and a chain fold has one return value;
			// subscribing here is what turns those publishes into an ordered
			// list this machine can answer one at a time.
			stop, collected, err := m.collectTriggers(ctx, bus)
			if err != nil {
				return nil, err
			}
			defer stop()

			event := &dnd5eEvents.MovementChainEvent{
				EntityID:            string(m.in.Mover),
				EntityType:          m.in.MoverKind,
				FromPosition:        dnd5eEvents.Position{X: m.in.From.X, Y: m.in.From.Y},
				ToPosition:          dnd5eEvents.Position{X: m.in.To.X, Y: m.in.To.Y},
				OAPreventionSources: make([]dnd5eEvents.MovementModifierSource, 0),
			}

			chain := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
			modified, err := dnd5eEvents.MovementChain.On(bus).PublishWithChain(ctx, event, chain)
			if err != nil {
				return nil, fmt.Errorf("publish movement chain for %q: %w", m.in.Mover, err)
			}

			folded, err := modified.Execute(ctx, event)
			if err != nil {
				return nil, fmt.Errorf("fold movement chain for %q: %w", m.in.Mover, err)
			}
			m.folded = folded

			// Sorted by reactor, then by what let them react, so identical
			// inputs produce identical stories (C8). Subscriber order is
			// attach order today and that is already sorted — this does not
			// depend on it staying that way.
			m.triggers = append(m.triggers, *collected...)
			sort.SliceStable(m.triggers, func(i, j int) bool {
				if m.triggers[i].ReactorID != m.triggers[j].ReactorID {
					return m.triggers[i].ReactorID < m.triggers[j].ReactorID
				}
				return m.triggers[i].ConditionRef < m.triggers[j].ConditionRef
			})

			return m.react(0), nil
		},
	}
}

// collectTriggers subscribes to reaction triggers and returns the buffer plus
// the unsubscribe. The buffer is a pointer because the handler appends to it
// after this returns.
func (m *movementMachine) collectTriggers(
	ctx context.Context, bus events.EventBus,
) (func(), *[]dnd5eEvents.ReactionTriggerEvent, error) {
	buffered := &[]dnd5eEvents.ReactionTriggerEvent{}
	subID, err := dnd5eEvents.ReactionTriggerTopic.On(bus).Subscribe(
		ctx, func(_ context.Context, e dnd5eEvents.ReactionTriggerEvent) error {
			*buffered = append(*buffered, e)
			return nil
		})
	if err != nil {
		return nil, nil, fmt.Errorf("subscribe reaction triggers for %q: %w", m.in.Mover, err)
	}

	return func() { _ = bus.Unsubscribe(ctx, subID) }, buffered, nil
}

// react answers trigger i, or finishes when they are exhausted.
//
// One step per trigger rather than one step resolving all of them, for the
// reason boundaryMachine yields one per crossing: each is a separate thing that
// happened, and every yield point is a legal suspension point — which is what
// the reaction WINDOW will need when a player is asked rather than told
// (rpg-project#316 defers the prompt; this shape is what it returns to).
func (m *movementMachine) react(i int) Step {
	for ; i < len(m.triggers); i++ {
		trigger := m.triggers[i]
		definition, ok := m.in.Reactions.AttackFor(trigger.ReactorID)
		if !ok {
			// No attack to swing is an ANSWER. The reactor's condition already
			// spent its meter deciding to react, and that is not refunded here:
			// this machine does not know why the capability declined, and
			// unwinding another package's economy on a guess is worse than a
			// reactor who swung at nothing.
			continue
		}

		return Request{
			name: fmt.Sprintf("%s reacts to %s", trigger.ReactorID, trigger.SourceEntity),
			machine: NewStrike(&StrikeInput{
				AttackerID: trigger.ReactorID,
				TargetID:   trigger.SourceEntity,
				Definition: definition,
				Roller:     m.in.Roller,
			}),
			next: func(_ context.Context, out Outcome) (Step, error) {
				struck, ok := out.(StrikeOutcome)
				if !ok {
					return nil, fmt.Errorf("%w: reaction by %q produced %T, not a strike",
						ErrBadMovement, trigger.ReactorID, out)
				}
				m.reactions = append(m.reactions, ReactionOutcome{
					ReactorID:    trigger.ReactorID,
					ConditionRef: trigger.ConditionRef,
					Against:      trigger.SourceEntity,
					Struck:       struck,
				})

				return m.react(i + 1), nil
			},
		}
	}

	return Done{Outcome: m.outcome()}
}

// outcome reads the step's result off the FOLDED event rather than the input,
// so a modifier that changed something is reported rather than echoed over.
//
// THE ENDPOINTS TOO, not only the flags. They are the same values today —
// nothing in the rulebook moves a step's From or To — and that is exactly what
// makes echoing them survive the whole suite while being wrong. runWalk's own
// loop carries the same warning about the same mistake one layer up: the day a
// step can land somewhere other than where it was aimed (a shove, a slide, a
// door that opens onto a different cell), echoing the input reports a movement
// that did not happen, and reading the answer keeps being right without anyone
// noticing it had to change.
//
// The input is the fallback ONLY for a machine whose fold never ran, which is
// the Start-without-drive path a test can reach and production cannot.
func (m *movementMachine) outcome() MovementOutcome {
	out := MovementOutcome{
		Mover:     string(m.in.Mover),
		From:      m.in.From,
		To:        m.in.To,
		Reactions: m.reactions,
	}
	if m.folded != nil {
		out.From = spatial.Position{X: m.folded.FromPosition.X, Y: m.folded.FromPosition.Y}
		out.To = spatial.Position{X: m.folded.ToPosition.X, Y: m.folded.ToPosition.Y}
		out.Prevented = m.folded.MovementPrevented
		out.PreventionReason = m.folded.PreventionReason
		out.OAPrevented = m.folded.IsOAPrevented()
	}

	return out
}
