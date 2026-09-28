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
	Kind              string                     `json:"kind"`
	Audience          string                     `json:"audience"`
	Offer             ReactionRef                `json:"offer"`
	Options           []CastOption               `json:"options,omitempty"`
	Frozen            []byte                     `json:"frozen"`
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
	out, err := resolution.Resolve(ctx, &resolution.Input{World: world, Participants: m.walkCast(ctx, scope, roster), Initiative: m.initiative, Standing: scope.standing, Sight: &sightSeam{members: worldMembers(world)}, Equipment: equipmentBeside(scope.standing), TurnDriver: scope.driver, CheckResolver: checkSeam{m: m, scope: scope}, Witness: witnessSeam{scope: scope}, Machine: machine, Roller: &diceSeam{roller: m.dice}})
	if err != nil {
		return nil, translateResolution(err)
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
	if out.Posed != nil {
		if out.Posed.Movement != nil {
			if err = m.recordPendingMovement(ctx, scope, &p, out, *out.Posed.Movement); err != nil {
				return nil, err
			}
		} else if out.Posed.Sequence != nil {
			if err = m.recordPendingSequence(scope, &p, *out.Posed.Sequence); err != nil {
				return nil, err
			}
		} else if out.Posed.SettledStrike != nil && !p.HitRecorded {
			if _, err = scope.enc.Record(recordFor(&AttackInput{Attacker: p.Attacker, Target: p.Target}, *out.Posed.SettledStrike, p.Definition, p.PresentationID, out)); err != nil {
				return nil, translate(err)
			}
			p.HitRecorded = true
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
		open, openErr := scope.ledger.Open()
		if openErr != nil {
			return nil, openErr
		}
		if len(open) == 0 && len(p.WalkPath) > 0 {
			if _, err = m.runWalk(ctx, scope, p.Target, p.WalkPath); err != nil {
				return nil, err
			}
		} else if len(open) == 0 && scope.enc.HeldDirective() {
			if _, err = scope.enc.ResumeDirective(ctx); err != nil {
				return nil, translate(err)
			}
		} else if len(open) == 0 && scope.enc.Paused() {
			if _, err = scope.enc.ResumeTurn(ctx); err != nil {
				return nil, translate(err)
			}
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
