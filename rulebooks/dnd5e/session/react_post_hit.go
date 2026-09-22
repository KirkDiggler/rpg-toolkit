// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

const windowKindPostHit = "post_hit"

// The hit is already durable before this window opens. Frozen belongs to
// resolution; resuming records only the reaction, never the original hit.
type postHitWindowPayload struct {
	Kind     string       `json:"kind"`
	Audience string       `json:"audience"`
	Offer    ReactionRef  `json:"offer"`
	Options  []CastOption `json:"options,omitempty"`
	Frozen   []byte       `json:"frozen"`
}

func thawPostHitPayload(raw []byte, audience string) (postHitWindowPayload, error) {
	var p postHitWindowPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("%w: post-hit window: %v", ErrInvalidSession, err)
	}
	if p.Kind != windowKindPostHit || p.Audience == "" || p.Audience != audience || p.Offer.Ref == "" || p.Offer.Name == "" || len(p.Frozen) == 0 {
		return p, fmt.Errorf("%w: incomplete post-hit window", ErrInvalidSession)
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

func posePostHitWindow(scope *writeScope, posed *resolution.Pose) error {
	ask := posed.Ask
	if ask.Audience == "" || ask.Offer.Ref == nil || ask.Offer.Name == "" {
		return fmt.Errorf("%w: incomplete reaction question", ErrInvalidWorld)
	}
	options := make([]CastOption, 0, len(ask.Choices))
	for _, o := range ask.Choices {
		options = append(options, CastOption{ID: o.ID, Label: o.Label})
	}
	payload, err := json.Marshal(postHitWindowPayload{Kind: windowKindPostHit, Audience: ask.Audience, Offer: ReactionRef{Ref: ask.Offer.Ref.String(), Name: ask.Offer.Name}, Options: options, Frozen: posed.Frozen})
	if err != nil {
		return err
	}
	if _, err = thawPostHitPayload(payload, ask.Audience); err != nil {
		return err
	}
	if _, err = scope.ledger.Pose(&interrupt.PoseInput{Audience: core.EntityID(ask.Audience), Options: []interrupt.Option{interrupt.Option(ReactStrike), interrupt.Option(ReactHold)}, Payload: payload, At: scope.baseline}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true
	return nil
}

func postHitDeclaration(session, member string, window interrupt.Window) (Declaration, error) {
	p, err := thawPostHitPayload(window.Payload, string(window.Audience))
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

func (m *Manager) answerPostHit(ctx context.Context, scope *writeScope, window interrupt.Window, in *ReactInput) (*ReactOutput, error) {
	p, err := thawPostHitPayload(window.Payload, string(window.Audience))
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
	machine, err := resolution.NewStrikeResumed(&resolution.StrikeResumeInput{Frozen: p.Frozen, Answer: answer, Option: in.Option, Roller: &diceSeam{roller: m.dice}})
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
		if err = posePostHitWindow(scope, out.Posed); err != nil {
			return nil, err
		}
	} else {
		struck, ok := out.Outcome.(resolution.StrikeOutcome)
		if !ok {
			return nil, fmt.Errorf("%w: resumed hit produced %T", ErrInvalidWorld, out.Outcome)
		}
		if err = m.recordRetaliation(scope, struck.Retaliation, out); err != nil {
			return nil, reportUnrecorded(scope, err)
		}
		if scope.enc.Paused() {
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

func (m *Manager) recordRetaliation(scope *writeScope, r *resolution.RetaliationOutcome, out *resolution.Output) error {
	if r == nil {
		return nil
	}
	ref := SpellRef{Ref: r.Offer.Ref.String(), Name: r.Offer.Name}
	save, err := castSave(resolution.CastTargetOutcome{TargetID: r.TargetID, Save: &r.Result}, r.Offer.Ref)
	if err != nil {
		return err
	}
	results := make([]encounter.ActivationResult, 0, len(r.Result.Imposed))
	for _, applied := range r.Result.Imposed {
		result, e := imposedResult(applied, ref)
		if e != nil {
			return e
		}
		results = append(results, result)
	}
	_, err = scope.enc.RecordActivation(&encounter.RecordActivationInput{Actor: encounter.MemberID(r.Offer.ReactorID), Target: encounter.MemberID(r.TargetID), Ability: encounter.ActivationIdentity{Ref: ref.Ref, Name: ref.Name}, Save: save, Results: results, ConcentrationChecks: out.ConcentrationChecks, ConcentrationBreaks: out.ConcentrationBreaks})
	return translate(err)
}
