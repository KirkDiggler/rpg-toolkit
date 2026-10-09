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
	Character          string `json:"character"`
	PrivateDiscoveries bool   `json:"private,omitempty"`
}

// ExplorationRepository is the host's opaque key-value preference store.
// Hosts must coordinate operations touching the same profile, including across
// sessions. A session-ID-only lock does not serialize those shared records.
type ExplorationRepository interface {
	GetExploration(context.Context, string) (*ExplorationData, error)
	SaveExploration(context.Context, *ExplorationData) error
}

func (m *Manager) prepareExploration(ctx context.Context, scope *writeScope, extra ...string) error {
	if m.explorations == nil {
		return nil
	} // older hosts retain explicit search
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
	for _, id := range ordered {
		profile, loadErr := m.explorations.GetExploration(ctx, id)
		if errors.Is(loadErr, ErrNotFound) {
			profile = &ExplorationData{Character: id}
		} else if loadErr != nil {
			return loadErr
		}
		if profile == nil || profile.Character != id {
			return fmt.Errorf("exploration %q: %w", id, ErrBadRepository)
		}
		copied := cloneExploration(profile)
		scope.exploration[id] = copied
		scope.explorationBefore[id] = *cloneExploration(profile)
		if err = m.stageCheck(ctx, scope, "discovery observer", id); err != nil {
			return err
		}
		// Restore only the preference. Existing attempt/knowledge state comes
		// from the loaded encounter; no character-owned checks enter this call.
		for _, member := range roster {
			if string(member.ID) == id {
				if _, err = scope.enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{Member: member.ID, Private: profile.PrivateDiscoveries}); err != nil {
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
	return &out
}

func (m *Manager) saveExploration(ctx context.Context, scope *writeScope) error {
	if m.explorations == nil {
		return nil
	}
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
		sharing, err := scope.enc.DiscoverySharing(&encounter.DiscoveryMemoryInput{Member: member.ID})
		if err != nil {
			return translate(err)
		}
		profile.PrivateDiscoveries = !sharing.Sharing
		if *profile == scope.explorationBefore[id] {
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
	if m.explorations == nil {
		return nil, fmt.Errorf("%s: unavailable: %w", verb, ErrIncompleteConfig)
	}
	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, err)
	}
	out, err := scope.enc.SetDiscoverySharing(&encounter.SetDiscoverySharingInput{Member: encounter.MemberID(in.Member), Sharing: in.Sharing})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, translate(err))
	}
	saved, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, err)
	}
	return &SetDiscoverySharingOutput{Sharing: out.Sharing, Saved: saved, Delivery: delivery}, nil
}

// automaticCheckSeam explicitly supplies proximity checks for adopting hosts.
// Authoring/read loaders and older hosts do not acquire this capability.
type automaticCheckSeam struct{ checkSeam }

func (c automaticCheckSeam) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	staged := c.scope.checks[string(in.Member)]
	if staged == nil {
		return nil, ErrNoSheet
	}
	if c.scope.walker != nil && c.scope.walker.ID == string(in.Member) {
		staged.data = c.scope.walker
	} else {
		// Staging precedes the verb's own writes, not only other requests.
		// Join, for example, stages the incoming member and then persists a
		// LongRest before placement triggers this check. Reusing that staged
		// record would reattach pre-rest effects (such as Rage). The session
		// guard does not make the old record current after an in-verb save.
		// The walker above is the exception: its unpaid/in-flight movement
		// state lives on the scope and must not be replaced from storage.
		fresh, err := c.m.sheetsFor(nil).load(staged.ctx, "discovery observer", string(in.Member))
		if err != nil {
			return nil, err
		}
		staged.data = fresh
	}
	return c.ResolveCheck(in)
}
func (m *Manager) checkResolverFor(scope *writeScope) encounter.CheckResolver {
	base := checkSeam{m: m, scope: scope}
	if m.explorations != nil {
		return automaticCheckSeam{base}
	}
	return base
}
