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

// ExplorationData stores character-owned discovery preferences only.
// Attempts and learned facts belong to EncounterData, never this profile.
// Legacy JSON may contain a checks member; ordinary decoding ignores that
// obsolete duplicate rather than importing it into a new playthrough.
type ExplorationData struct {
	Character string `json:"character"`
	// PrivateDiscoveries is the legacy preference, read only when
	// DiscoverySharing is nil: a record written before the choice existed
	// keeps its meaning (absent meant shared). SetDiscoverySharing keeps it
	// the complement of DiscoverySharing so the two never disagree.
	PrivateDiscoveries bool `json:"private,omitempty"`
	// DiscoverySharing is the character's explicit choice. Nil means the
	// character has not chosen; only SetDiscoverySharing sets it.
	DiscoverySharing *bool `json:"discovery_sharing,omitempty"`
}

// resolveDiscoverySharing is the one place a stored preference becomes the
// audience a member discovers for (rpg-project#595, R8): an explicit choice
// wins; a record without one keeps its legacy meaning; no record at all is
// private. It reads only, so a default is never written down as a choice.
func resolveDiscoverySharing(profile *ExplorationData, found bool) bool {
	if profile != nil && profile.DiscoverySharing != nil {
		return *profile.DiscoverySharing
	}
	if profile == nil || !found {
		return false
	}
	return !profile.PrivateDiscoveries
}

// ExplorationRepository is the host's opaque key-value preference store.
// Hosts must coordinate operations touching the same profile, including across
// sessions. A session-ID-only lock does not serialize those shared records.
type ExplorationRepository interface {
	GetExploration(context.Context, string) (*ExplorationData, error)
	SaveExploration(context.Context, *ExplorationData) error
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
	scope.exploration = map[string]*ExplorationData{}
	scope.explorationBefore = map[string]ExplorationData{}
	scope.explorationSharing = map[string]bool{}
	for _, id := range ordered {
		profile, loadErr := m.explorations.GetExploration(ctx, id)
		found := true
		if errors.Is(loadErr, ErrNotFound) {
			profile, found = &ExplorationData{Character: id}, false
		} else if loadErr != nil {
			return loadErr
		}
		if profile == nil || profile.Character != id {
			return fmt.Errorf("exploration %q: %w", id, ErrBadRepository)
		}
		copied := cloneExploration(profile)
		scope.exploration[id] = copied
		scope.explorationBefore[id] = *cloneExploration(profile)
		// Resolve once, here, where ErrNotFound is still visible: the staged
		// copy of a missing record is a bare {Character: id}, which reads
		// exactly like a legacy shared record.
		sharing := resolveDiscoverySharing(profile, found)
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

func cloneExploration(in *ExplorationData) *ExplorationData {
	out := *in
	if in.DiscoverySharing != nil {
		sharing := *in.DiscoverySharing
		out.DiscoverySharing = &sharing
	}
	return &out
}

func sameExploration(a, b *ExplorationData) bool {
	if a.Character != b.Character || a.PrivateDiscoveries != b.PrivateDiscoveries {
		return false
	}
	if a.DiscoverySharing == nil || b.DiscoverySharing == nil {
		return a.DiscoverySharing == nil && b.DiscoverySharing == nil
	}
	return *a.DiscoverySharing == *b.DiscoverySharing
}

func (m *Manager) saveExploration(ctx context.Context, scope *writeScope) error {
	members, err := scope.enc.Members()
	if err != nil {
		return translate(err)
	}
	for _, member := range members {
		id := string(member.ID)
		profile := scope.exploration[id]
		if profile == nil {
			continue
		}
		// Only a staged change is saved. The preference is never derived from
		// the encounter: SetDiscoverySharing is its one writer, so a member who
		// never chose leaves no record behind.
		before := scope.explorationBefore[id]
		if sameExploration(profile, &before) {
			continue
		}
		if err = m.explorations.SaveExploration(ctx, profile); err != nil {
			return saveErrorAfterWrites(scope, "exploration:"+id, err)
		}
		scope.written = append(scope.written, "exploration:"+id)
	}
	return nil
}

// SetDiscoverySharingInput changes the seated character's future discovery audience.
type SetDiscoverySharingInput struct {
	Session string
	Member  string
	Sharing bool
}

// SetDiscoverySharingOutput is the new preference and ordinary persistence reports.
type SetDiscoverySharingOutput struct {
	Sharing  bool
	Saved    SaveReport
	Delivery DeliveryReport
}

// SetDiscoverySharing neither replays earlier discoveries nor removes knowledge.
// Refusals name the verb while preserving sentinels and partial-save reports.
func (m *Manager) SetDiscoverySharing(ctx context.Context, in *SetDiscoverySharingInput) (*SetDiscoverySharingOutput, error) {
	const verb = "set discovery sharing"
	if in == nil {
		return nil, fmt.Errorf("%s: %w", verb, ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, fmt.Errorf("%s: %w", verb, lockErr)
	}
	defer release()
	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, err)
	}
	out, err := scope.enc.SetDiscoverySharing(&encounter.SetDiscoverySharingInput{Member: encounter.MemberID(in.Member), Sharing: in.Sharing})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, translate(err))
	}
	// The explicit choice is staged here and nowhere else. Choosing the value
	// the default already resolves to is still a choice and is saved.
	if profile := scope.exploration[in.Member]; profile != nil {
		sharing := out.Sharing
		profile.DiscoverySharing = &sharing
		profile.PrivateDiscoveries = !sharing
	}
	saved, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, err)
	}
	return &SetDiscoverySharingOutput{Sharing: out.Sharing, Saved: saved, Delivery: delivery}, nil
}
