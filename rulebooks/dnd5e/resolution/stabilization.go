// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type preparedStabilization struct {
	targetID string
	source   core.Ref
	name     string
}

func (s *preparedStabilization) prepare(cast *Participants) error {
	target, ok := cast.Character(s.targetID)
	if !ok || !target.CanStabilize() {
		return fmt.Errorf("%w: stabilization requires a living character at zero hit points", ErrBadAction)
	}
	return nil
}

func (s *preparedStabilization) deliver(cast *Participants) (ActivationEffect, error) {
	target, ok := cast.Character(s.targetID)
	if !ok {
		return ActivationEffect{}, fmt.Errorf("%w: no stabilization recipient %q", ErrBadAction, s.targetID)
	}
	result, err := target.Stabilize()
	if err != nil {
		return ActivationEffect{}, err
	}
	return ActivationEffect{Kind: EffectStabilized, TargetID: s.targetID,
		Ref: s.source.String(), Name: s.name, Stabilization: *result}, nil
}

// StabilizationTargetsInput supplies physical reach and known candidate identities.
// It does not discover targets or require current sight. Monsters are unsupported.
type StabilizationTargetsInput struct {
	Room         spatial.Room
	CasterID     string
	Candidates   []string
	Participants []Participant
}

// StabilizationTargets asks the character provider for eligibility, then checks
// touch reach. It neither spends resources nor changes the supplied sheets.
func StabilizationTargets(ctx context.Context, input *StabilizationTargetsInput) (map[string]bool, error) {
	if input == nil {
		return nil, ErrNilInput
	}
	sheets := make(map[string]*character.Data)
	for _, participant := range input.Participants {
		if participant.Character != nil {
			sheets[participant.Character.ID] = participant.Character
		}
	}
	out := make(map[string]bool, len(input.Candidates))
	for _, id := range input.Candidates {
		data, ok := sheets[id]
		if !ok {
			continue
		}
		sheet, err := character.Load(ctx, data)
		if err != nil {
			return nil, err
		}
		if !sheet.CanStabilize() {
			continue
		}
		reach, err := TouchReach(input.Room, input.CasterID, id)
		if err != nil {
			return nil, err
		}
		out[id] = reach
	}
	return out, nil
}
