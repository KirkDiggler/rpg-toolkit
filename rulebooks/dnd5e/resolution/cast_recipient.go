// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"fmt"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
)

// CastRecipientEligibility projects the profile's condition restrictions from
// persisted sheets. It shares the exact rule used by the pre-payment gate.
func CastRecipientEligibility(profile *combatActions.CastProfile, participants []Participant) (map[string]bool, error) {
	if profile == nil {
		return nil, ErrNilInput
	}
	out := make(map[string]bool, len(participants))
	for _, participant := range participants {
		var stored []json.RawMessage
		switch {
		case participant.Character != nil:
			stored = participant.Character.Conditions
		case participant.Monster != nil:
			stored = participant.Monster.Conditions
		default:
			return nil, ErrBadParticipant
		}
		allowed, err := profile.AllowsRecipient(stored)
		if err != nil {
			return nil, err
		}
		out[participant.ID()] = allowed
	}
	return out, nil
}

func validateRecipientCooldown(profile *combatActions.CastProfile, cast *Participants, targetID string) error {
	if len(profile.RecipientBlockedBy) == 0 {
		return nil
	}
	if _, err := cast.entity(targetID); err != nil {
		return err
	}
	held := heldConditions(cast, targetID)
	stored := make([]json.RawMessage, 0, len(held))
	for _, condition := range held {
		blob, err := condition.ToJSON()
		if err != nil {
			return err
		}
		stored = append(stored, blob)
	}
	allowed, err := profile.AllowsRecipient(stored)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("%w: target %q cannot receive this spell while its cooldown is active", ErrBadAction, targetID)
	}
	return nil
}
