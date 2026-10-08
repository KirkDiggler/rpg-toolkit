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

// holdAreas moves out's area changes onto the window, to land when the
// resume has told the swing that caused them. out keeps none, so nothing
// lands them twice.
//
// THE RESUME IS THE ONLY PLACE THEY LAND. A future path that closes this
// window without resuming the sequence (an expiry, a dissolve) must land
// HeldAreas itself, or the area stands with no concentration behind it.
func (p *pendingAttackWindowPayload) holdAreas(out *resolution.Output) {
	if len(out.ClosedAreas) == 0 && len(out.OpenedAreas) == 0 {
		return
	}
	held := p.HeldAreas
	if held == nil {
		held = &heldAreas{}
	}
	held.Closed = append(held.Closed, out.ClosedAreas...)
	held.Opened = append(held.Opened, out.OpenedAreas...)
	p.HeldAreas = held
	out.ClosedAreas, out.OpenedAreas = nil, nil
}

// landHeldAreas lands what an earlier pose held, once the resume has recorded
// the swing that caused it, and clears it from the window.
// landToldAreas lands the area changes a paused sequence has already told
// and holds the rest. told are the swings recorded for this output: a closed
// area whose caster's break rides one of them is told and lands; any other
// change belongs to the swing that settled and paused, untold until the next
// resume, and waits on the window. A pose before the roll has no such swing,
// so everything lands.
func (m *Manager) landToldAreas(
	enc *encounter.Encounter, scope *writeScope, p *pendingAttackWindowPayload,
	out *resolution.Output, told []resolution.SequenceStepOutcome,
) error {
	if out.Posed == nil || out.Posed.BeforeRoll {
		return m.landAreas(enc, scope, out)
	}
	broken := map[string]bool{}
	for _, step := range told {
		for _, b := range step.ConcentrationBreaks {
			broken[string(b.Caster)] = true
		}
	}
	landing := &resolution.Output{}
	var waiting []string
	for _, caster := range out.ClosedAreas {
		if broken[caster] {
			landing.ClosedAreas = append(landing.ClosedAreas, caster)
		} else {
			waiting = append(waiting, caster)
		}
	}
	out.ClosedAreas = waiting
	p.holdAreas(out)
	return m.landAreas(enc, scope, landing)
}

func (m *Manager) landHeldAreas(scope *writeScope, p *pendingAttackWindowPayload) error {
	if p.HeldAreas == nil {
		return nil
	}
	held := &resolution.Output{ClosedAreas: p.HeldAreas.Closed, OpenedAreas: p.HeldAreas.Opened}
	p.HeldAreas = nil
	return m.landAreas(scope.enc, scope, held)
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
	out, err := resolution.Resolve(ctx, &resolution.Input{World: world, Participants: m.walkCast(ctx, scope, roster), Initiative: m.initiative, Standing: scope.standing, Sight: sheetsBeside(scope.standing), Equipment: equipmentBeside(scope.standing), Sheets: sheetsBeside(scope.standing), TurnDriver: scope.driver, CheckResolver: checkSeam{m: m, scope: scope}, Witness: witnessSeam{scope: scope}, Machine: machine, Roller: &diceSeam{roller: m.dice}})
	if err != nil {
		return nil, translateAttack(err)
	}
	if err = m.adopt(ctx, scope, out.World); err != nil {
		return nil, err
	}
	if err = m.saveDirty(ctx, scope, out); err != nil {
		return nil, err
	}
	if err = answerWindow(scope, window, in.Choice); err != nil {
		return nil, err
	}
	// AREAS LAND ONCE PER OUTPUT, after the beats that caused them and before
	// the next window opens or the walk resumes. A movement output lands in
	// recordMovementResults (the same call a walk's own step makes), so only
	// the other arms land here: landing a movement output twice would re-open
	// an opened area, which the encounter refuses.
	var told []resolution.SequenceStepOutcome
	if out.Posed != nil {
		if out.Posed.Movement != nil {
			if err = m.recordPendingMovement(ctx, scope, &p, out, *out.Posed.Movement); err != nil {
				return nil, err
			}
		} else if out.Posed.Sequence != nil {
			told = out.Posed.Sequence.Steps[min(p.RecordedSteps, len(out.Posed.Sequence.Steps)):]
			if err = m.recordPendingSequence(scope, &p, *out.Posed.Sequence); err != nil {
				return nil, err
			}
		} else if out.Posed.SettledStrike != nil && !p.HitRecorded {
			if _, err = scope.enc.Record(recordFor(&AttackInput{Attacker: p.Attacker, Target: p.Target}, *out.Posed.SettledStrike, p.Definition, p.PresentationID, out)); err != nil {
				return nil, translate(err)
			}
			p.HitRecorded = true
		}
		if out.Posed.Movement == nil {
			if err = m.landHeldAreas(scope, &p); err != nil {
				return nil, reportUnrecorded(scope, err)
			}
			// A sequence that pauses AGAIN after a later swing settled holds
			// that swing's areas exactly as the first pause did (strikerSeam):
			// the swing is told only on the next resume. What the swings
			// recorded now told lands.
			if out.Posed.Sequence != nil {
				err = m.landToldAreas(scope.enc, scope, &p, out, told)
			} else {
				err = m.landAreas(scope.enc, scope, out)
			}
			if err != nil {
				return nil, reportUnrecorded(scope, err)
			}
		}
		if err = posePendingAttackWindow(scope, out.Posed, p); err != nil {
			return nil, err
		}
	} else {
		switch produced := out.Outcome.(type) {
		case resolution.MovementOutcome:
			if err = m.recordPendingMovement(ctx, scope, &p, out, produced); err != nil {
				return nil, err
			}
		case resolution.StrikeOutcome:
			if !p.HitRecorded {
				if _, err = scope.enc.Record(recordFor(&AttackInput{Attacker: p.Attacker, Target: p.Target}, produced, p.Definition, p.PresentationID, out)); err != nil {
					return nil, translate(err)
				}
			}
			if err = m.recordRetaliation(scope, produced.Retaliation, out); err != nil {
				return nil, reportUnrecorded(scope, err)
			}
		case resolution.SequenceOutcome:
			if err = m.recordPendingSequence(scope, &p, produced); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%w: resumed attack produced %T", ErrInvalidWorld, out.Outcome)
		}
		if _, moved := out.Outcome.(resolution.MovementOutcome); !moved {
			if err = m.landHeldAreas(scope, &p); err != nil {
				return nil, reportUnrecorded(scope, err)
			}
			if err = m.landAreas(scope.enc, scope, out); err != nil {
				return nil, reportUnrecorded(scope, err)
			}
		}
		if err = m.resumeAfterLastAnswer(ctx, scope, p.Target, p.WalkPath); err != nil {
			return nil, err
		}
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true
	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, err
	}
	return &ReactOutput{Saved: report, Delivery: delivery}, nil
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

func (m *Manager) recordPendingMovement(ctx context.Context, scope *writeScope, p *pendingAttackWindowPayload, out *resolution.Output, moved resolution.MovementOutcome) error {
	if p.RecordedReactions > len(moved.Reactions) {
		return fmt.Errorf("%w: completed movement reactions shrank", ErrInvalidWorld)
	}
	count := len(moved.Reactions)
	moved.Reactions = moved.Reactions[p.RecordedReactions:]
	if err := (moverSeam{m: m, scope: scope}).recordMovementResults(ctx, scope.enc, out, moved); err != nil {
		return err
	}
	p.RecordedReactions = count
	return nil
}
