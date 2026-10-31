// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// CharacterConfigReader is the host's character configuration, read by
// character (rpg-project#753 R2). What a player chooses for a character across
// every session lives there, under the character's own guard, and is written
// by character verbs the host owns. The session only reads it: no session verb
// writes character configuration, so no session lock needs to reach wider than
// its own session.
//
// DiscoverySharing is the character's discovery-sharing choice. A nil value
// with a nil error means the character has not chosen; ErrNotFound means the
// same. Any other error refuses the verb that asked. A host migrating an older
// preference record resolves it into this one answer before returning it.
type CharacterConfigReader interface {
	DiscoverySharing(ctx context.Context, character string) (*bool, error)
}

// resolveDiscoverySharing is the one place a configured preference becomes
// the audience a member discovers for (rpg-project#595, R8): an explicit
// choice wins, and a character who has not chosen discovers privately. It
// reads only, so a default is never written down as a choice. A legacy record
// without a choice is the host's to resolve before the reader answers.
func resolveDiscoverySharing(choice *bool) bool {
	if choice != nil {
		return *choice
	}
	return false
}

func (m *Manager) prepareExploration(ctx context.Context, scope *writeScope, extra ...string) error {
	roster, err := scope.enc.Members()
	if err != nil {
		return translate(err)
	}
	ids := map[string]bool{}
	for _, member := range roster {
		if member.Kind == encounter.KindPlayer {
			ids[string(member.ID)] = true
		}
	}
	for _, id := range extra {
		if id != "" {
			ids[id] = true
		}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	scope.explorationSharing = map[string]bool{}
	for _, id := range ordered {
		choice, loadErr := m.characterConfig.DiscoverySharing(ctx, id)
		if errors.Is(loadErr, ErrNotFound) {
			choice = nil
		} else if loadErr != nil {
			return fmt.Errorf("character configuration %q: %w", id, loadErr)
		}
		// Read at every write verb, so a change the host made under the
		// character's guard takes effect at this member's next verb.
		sharing := resolveDiscoverySharing(choice)
		scope.explorationSharing[id] = sharing
		// EVERY PLAYER IS STAGED, ON EVERY WRITE VERB, and that is also what
		// a sneaker's roll needs (rpg-project#606, design R10): the
		// encounter asks the check seam for a sneaker's Stealth inside
		// whatever verb produced the entry — another player's walk, a
		// monster's turn, a door opening — so the sneaker's record must be
		// staged even when the verb is somebody else's. No second staging
		// path: a sneaker is a player, and this loop already holds them all.
		if err = m.stageCheck(ctx, scope, "discovery observer", id); err != nil {
			return err
		}
		// Restore only the preference. Existing attempt/knowledge state comes
		// from the loaded encounter; no character-owned checks enter this call.
		for _, member := range roster {
			if string(member.ID) == id {
				restore := &encounter.RestoreDiscoveryInput{Member: member.ID, Private: !sharing}
				if _, err = scope.enc.RestoreDiscovery(restore); err != nil {
					return translate(err)
				}
				break
			}
		}
	}
	return nil
}
