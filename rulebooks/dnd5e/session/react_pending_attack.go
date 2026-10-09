// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const windowKindPendingAttack = "pending_attack"

// Pending attacks retain their provider continuation and recording identity.
type pendingAttackWindowPayload struct {
	WalkPath          []spatial.Position         `json:"walk_path,omitempty"`
	Movement          bool                       `json:"movement,omitempty"`
	RecordedReactions int                        `json:"recorded_reactions,omitempty"`
	Attacker          string                     `json:"attacker"`
	Target            string                     `json:"target"`
	Definition        combatActions.Definition   `json:"definition"`
	Components        []combatActions.Definition `json:"components,omitempty"`
	PresentationID    string                     `json:"presentation_id,omitempty"`
	RecordedSteps     int                        `json:"recorded_steps,omitempty"`
	HitRecorded       bool                       `json:"hit_recorded,omitempty"`
	// HeldAreas are the area changes of a swing that settled and stopped to
	// ask before its beat was told; they land once that beat is recorded on
	// the resume. See [pendingAttackWindowPayload.holdAreas].
	HeldAreas *heldAreas   `json:"held_areas,omitempty"`
	Kind      string       `json:"kind"`
	Audience  string       `json:"audience"`
	Offer     ReactionRef  `json:"offer"`
	Options   []CastOption `json:"options,omitempty"`
	Frozen    []byte       `json:"frozen"`
}

// heldAreas are area changes waiting on the beat that caused them.
type heldAreas struct {
	Closed []string                   `json:"closed,omitempty"`
	Opened []encounter.SightAreaInput `json:"opened,omitempty"`
}

func thawPendingAttackPayload(raw []byte, audience string) (pendingAttackWindowPayload, error) {
	var p pendingAttackWindowPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("%w: pending attack window: %v", ErrInvalidSession, err)
	}
	if (!p.Movement && (p.Attacker == "" || p.Target == "" || p.Definition.Ref.ID == "")) || p.Kind != windowKindPendingAttack || p.Audience == "" || p.Audience != audience || p.Offer.Ref == "" || p.Offer.Name == "" || len(p.Frozen) == 0 {
		return p, fmt.Errorf("%w: incomplete pending attack window", ErrInvalidSession)
	}
	seen := map[string]bool{}
	for _, o := range p.Options {
		if o.ID == "" || o.Label == "" || seen[o.ID] {
			return p, fmt.Errorf("%w: invalid reaction option", ErrInvalidSession)
		}
		seen[o.ID] = true
	}
	return p, nil
}

func posePendingAttackWindow(scope *writeScope, posed *resolution.Pose, p pendingAttackWindowPayload) error {
	ask := posed.Ask
	if ask.Audience == "" || ask.Offer.Ref == nil || ask.Offer.Name == "" {
		return fmt.Errorf("%w: incomplete reaction question", ErrInvalidWorld)
	}
	options := make([]CastOption, 0, len(ask.Choices))
	for _, o := range ask.Choices {
		options = append(options, CastOption{ID: o.ID, Label: o.Label})
	}
	p.Kind = windowKindPendingAttack
	p.Audience = ask.Audience
	p.Offer = ReactionRef{Ref: ask.Offer.Ref.String(), Name: ask.Offer.Name}
	p.Options = options
	p.Frozen = posed.Frozen
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if _, err = thawPendingAttackPayload(payload, ask.Audience); err != nil {
		return err
	}
	if _, err = scope.ledger.Pose(&interrupt.PoseInput{Audience: core.EntityID(ask.Audience), Options: []interrupt.Option{interrupt.Option(ReactStrike), interrupt.Option(ReactHold)}, Payload: payload, At: scope.baseline}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true
	return nil
}

func pendingAttackDeclaration(session, member string, window interrupt.Window) (Declaration, error) {
	p, err := thawPendingAttackPayload(window.Payload, string(window.Audience))
	if err != nil {
		return Declaration{}, err
	}
	id, err := reactDeclarationID(session, member, window.ID)
	if err != nil {
		return Declaration{}, err
	}
	slot := SlotReaction
	if len(p.Options) == 0 {
		slot = SlotNone
	}
	return Declaration{Verb: VerbReact, Slot: slot, Available: true, ID: id, Reaction: &p.Offer, TargetKind: TargetNone, Candidates: []TargetCandidate{}, Options: p.Options}, nil
}

func (m *Manager) answerPendingAttack(ctx context.Context, scope *writeScope, window interrupt.Window, in *ReactInput) (*ReactOutput, error) {
	p, err := thawPendingAttackPayload(window.Payload, string(window.Audience))
	if err != nil {
		return nil, err
	}
	answer := resolution.OfferKeep
	if in.Choice == ReactStrike {
		answer = resolution.OfferSpend
	}
	if in.Choice == ReactHold && in.Option != "" {
		return nil, fmt.Errorf("%w: hold carries no option", ErrNotOffered)
	}
	if len(p.Options) > 0 && in.Choice == ReactStrike {
		found := false
		for _, o := range p.Options {
			if o.ID == in.Option {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: unknown reaction option", ErrNotOffered)
		}
	} else if in.Option != "" {
		return nil, fmt.Errorf("%w: this window offers no options", ErrNotOffered)
	}
	resumeInput := &resolution.StrikeResumeInput{Frozen: p.Frozen, Answer: answer, Option: in.Option, Roller: &diceSeam{roller: m.dice}}
	var machine resolution.Machine
	if p.Movement {
		machine, err = resolution.NewMovementResumed(resumeInput)
	} else {
		machine, err = resolution.NewAttackResumed(resumeInput)
	}
	if err != nil {
		return nil, translateResolution(err)
	}
	roster, err := scope.enc.Members()
	if err != nil {
		return nil, translate(err)
	}
	world := scope.enc.WorldView()
	out, err := resolution.Resolve(ctx, m.resolutionInput(ctx, scope, resolutionAsk{World: world, Participants: m.walkCast(ctx, scope, roster), Machine: machine}))
	if err != nil {
		return nil, translateAttack(err)
	}
	l, err := m.pendingAttackLanding(ctx, scope, &p, out)
	if err != nil {
		return nil, err
	}
	l.Answer = &windowAnswer{Window: window, Choice: in.Choice}
	result, err := m.land(ctx, scope, out, l)
	if err != nil {
		return nil, err
	}
	return &ReactOutput{Saved: result.Saved, Delivery: result.Delivery}, nil
}

// pendingAttackLanding chooses how a resumed pending attack lands, one arm
// per shape the resume can leave. It reads Outcome and Posed and nothing the
// landing owns; p is the window's payload, which the record steps advance and
// a re-posed window carries on.
//
// AREAS LAND ONCE PER OUTPUT, after the beats that caused them and before the
// next window opens or the walk resumes. A movement lands its areas now and
// lands no held areas; every other arm lands what an earlier pose held first.
func (m *Manager) pendingAttackLanding(
	ctx context.Context, scope *writeScope, p *pendingAttackWindowPayload, out *resolution.Output,
) (*landing, error) {
	strike := func(struck resolution.StrikeOutcome) func(*encounter.Encounter, concentration) error {
		return func(enc *encounter.Encounter, told concentration) error {
			_, err := enc.Record(recordFor(&AttackInput{Attacker: p.Attacker, Target: p.Target}, struck, p.Definition, p.PresentationID, told))
			return err
		}
	}
	held := areaLanding{Payload: p, LandHeld: true}

	if out.Posed != nil {
		posed := out.Posed
		l := &landing{
			Window: func(*encounter.Encounter) error {
				return posePendingAttackWindow(scope, posed, *p)
			},
		}
		switch {
		case posed.Movement != nil:
			moved := *posed.Movement
			l.Record = func(enc *encounter.Encounter, _ concentration) error {
				return m.recordPendingMovement(enc, p, moved)
			}
		case posed.Sequence != nil:
			// A sequence that pauses AGAIN after a later swing settled holds
			// that swing's areas exactly as the first pause did (strikerSeam):
			// the swing is told only on the next resume. What the swings
			// recorded now told lands.
			sequence := *posed.Sequence
			told := sequence.Steps[min(p.RecordedSteps, len(sequence.Steps)):]
			l.Record = func(*encounter.Encounter, concentration) error {
				return m.recordPendingSequence(scope, p, sequence)
			}
			l.Areas = areaLanding{Payload: p, LandHeld: true, Split: true, Told: told}
		case posed.SettledStrike != nil && !p.HitRecorded:
			record := strike(*posed.SettledStrike)
			l.Record = func(enc *encounter.Encounter, told concentration) error {
				if err := record(enc, told); err != nil {
					return err
				}
				p.HitRecorded = true
				return nil
			}
			l.Areas = held
		default:
			// The settled hit was told on an earlier resume, or the swing has
			// not rolled: nothing is told now (R9).
			l.Untold = true
			l.Areas = held
		}
		return l, nil
	}

	l := &landing{
		Continue: func(*encounter.Encounter) error {
			return m.resumeAfterLastAnswer(ctx, scope, p.Target, p.WalkPath)
		},
	}
	switch produced := out.Outcome.(type) {
	case resolution.MovementOutcome:
		l.Record = func(enc *encounter.Encounter, _ concentration) error {
			return m.recordPendingMovement(enc, p, produced)
		}
	case resolution.StrikeOutcome:
		record := strike(produced)
		l.Record = func(enc *encounter.Encounter, told concentration) error {
			if !p.HitRecorded {
				if err := record(enc, told); err != nil {
					return err
				}
			}
			return m.recordRetaliation(scope, produced.Retaliation, told)
		}
		l.Areas = held
	case resolution.SequenceOutcome:
		l.Record = func(*encounter.Encounter, concentration) error {
			return m.recordPendingSequence(scope, p, produced)
		}
		l.Areas = held
	default:
		// Refused at the record step, where it has always been refused: after
		// the world is adopted and the sheets are written, so the report names
		// what landed.
		l.Record = func(*encounter.Encounter, concentration) error {
			return fmt.Errorf("%w: resumed attack produced %T", ErrInvalidWorld, out.Outcome)
		}
	}
	return l, nil
}

func (m *Manager) recordPendingSequence(scope *writeScope, p *pendingAttackWindowPayload, sequence resolution.SequenceOutcome) error {
	if p.RecordedSteps > len(sequence.Steps) {
		return fmt.Errorf("%w: completed sequence shrank", ErrInvalidWorld)
	}
	remaining := sequence
	remaining.Steps = sequence.Steps[p.RecordedSteps:]
	if len(remaining.Steps) == 0 {
		return nil
	}
	seam := strikerSeam{m: m, scope: scope}
	if err := seam.recordSequence(scope.enc, &AttackInput{Attacker: p.Attacker, Target: p.Target}, remaining, p.Components); err != nil {
		return err
	}
	p.RecordedSteps = len(sequence.Steps)
	return nil
}

func (m *Manager) recordPendingMovement(enc *encounter.Encounter, p *pendingAttackWindowPayload, moved resolution.MovementOutcome) error {
	if p.RecordedReactions > len(moved.Reactions) {
		return fmt.Errorf("%w: completed movement reactions shrank", ErrInvalidWorld)
	}
	count := len(moved.Reactions)
	moved.Reactions = moved.Reactions[p.RecordedReactions:]
	beats, err := movementBeats(moved)
	if err != nil {
		return err
	}
	if err := recordBeats(beats)(enc, concentration{}); err != nil {
		return err
	}
	p.RecordedReactions = count
	return nil
}
