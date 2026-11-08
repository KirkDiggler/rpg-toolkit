// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidLifetime marks invalid lifetime definitions, restoration or operations.
var ErrInvalidLifetime = errors.New("invalid lifetime")

// LifetimePhase reports whether an application can still change membership.
type LifetimePhase string

const (
	// LifetimeLive admits member-loss notices and the first authorized end.
	LifetimeLive LifetimePhase = "live"
	// LifetimeEnding holds the first accepted end while its owner delivers the plan.
	LifetimeEnding LifetimePhase = "ending"
	// LifetimeEnded records successful completion; it produces no further plan.
	LifetimeEnded LifetimePhase = "ended"
)

// LifetimeInput declares opaque identity, ordered member keys and authorized enders.
// Empty membership is valid. All supplied keys must be nonblank and unique within
// their list. The caller owns the meaning of each key and its external uniqueness.
type LifetimeInput struct {
	ID      string
	Members []string
	Enders  []string
}

// LifetimeSnapshot is an owned observation of phase and surviving member keys.
// Only a live snapshot can be restored: a completed application is removed by its
// owner rather than persisted as a fresh or partially ended lifetime.
type LifetimeSnapshot struct {
	Phase       LifetimePhase `json:"phase"`
	LiveMembers []string      `json:"live_members"`
}

// RestoreLifetimeInput binds immutable construction data to explicit saved state.
// A nil snapshot is missing state, not an instruction to create fresh membership.
type RestoreLifetimeInput struct {
	Definition LifetimeInput
	Snapshot   *LifetimeSnapshot
}

// EndRequest names the bound capability asking to end and its nonblank reason.
type EndRequest struct {
	Requestor string
	Reason    string
}

// EndPlan is the first accepted end's ordered member list. A repeated valid
// request returns Accepted=false, the original reason and no members to remove.
// The caller delivers the plan and calls CompleteEnd only after delivery succeeds.
type EndPlan struct {
	Accepted bool
	Reason   string
	Members  []string
}

// MemberRemovedInput names a declared member that has ended independently.
type MemberRemovedInput struct{ Key string }

// MemberRemovedOutput distinguishes a real loss from a duplicate notification.
// Before and After let the owner distinguish last-member loss from empty creation.
type MemberRemovedOutput struct {
	Removed bool
	Before  int
	After   int
}

// Lifetime owns one application's live membership and end acceptance. It returns
// values and never publishes, rolls, loads a holder or interprets a member key.
// Like Composed, it is interaction-local and not safe for concurrent mutation.
type Lifetime struct {
	id      string
	members []string
	enders  []string
	live    map[string]struct{}
	phase   LifetimePhase
	reason  string
}

// NewLifetime constructs live membership with defensive copies. It refuses a
// blank identity or blank/duplicate member or ender keys.
func NewLifetime(input LifetimeInput) (*Lifetime, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidLifetime)
	}
	for _, group := range []struct {
		name string
		keys []string
	}{{"member", input.Members}, {"ender", input.Enders}} {
		seen := make(map[string]struct{}, len(group.keys))
		for _, key := range group.keys {
			if strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("%w: %s: blank %s key", ErrInvalidLifetime, input.ID, group.name)
			}
			if _, exists := seen[key]; exists {
				return nil, fmt.Errorf("%w: %s: duplicate %s %q", ErrInvalidLifetime, input.ID, group.name, key)
			}
			seen[key] = struct{}{}
		}
	}
	live := make(map[string]struct{}, len(input.Members))
	for _, key := range input.Members {
		live[key] = struct{}{}
	}
	return &Lifetime{
		id: input.ID, members: slices.Clone(input.Members), enders: slices.Clone(input.Enders),
		live: live, phase: LifetimeLive,
	}, nil
}

// RestoreLifetime restores an explicitly live subset of the declared membership.
// It refuses missing/non-live state and duplicate or foreign keys; it never
// restores omitted members from construction defaults. Input order cannot change
// the declared order of future end plans.
func RestoreLifetime(input RestoreLifetimeInput) (*Lifetime, error) {
	lifetime, err := NewLifetime(input.Definition)
	if err != nil {
		return nil, err
	}
	if input.Snapshot == nil || input.Snapshot.Phase != LifetimeLive {
		return nil, fmt.Errorf("%w: %s: explicit live snapshot is required", ErrInvalidLifetime, lifetime.id)
	}
	clear(lifetime.live)
	for _, key := range input.Snapshot.LiveMembers {
		if !slices.Contains(lifetime.members, key) {
			return nil, fmt.Errorf("%w: %s: foreign member %q", ErrInvalidLifetime, lifetime.id, key)
		}
		if _, exists := lifetime.live[key]; exists {
			return nil, fmt.Errorf("%w: %s: duplicate live member %q", ErrInvalidLifetime, lifetime.id, key)
		}
		lifetime.live[key] = struct{}{}
	}
	return lifetime, nil
}

// Snapshot returns an owned membership slice in declared order, never map order.
func (l *Lifetime) Snapshot() LifetimeSnapshot {
	members := make([]string, 0, len(l.live))
	for _, key := range l.members {
		if _, live := l.live[key]; live {
			members = append(members, key)
		}
	}
	return LifetimeSnapshot{Phase: l.phase, LiveMembers: members}
}

// RequestEnd accepts the first authorized request and freezes membership for its
// delivery. Invalid authority/reason always fails; valid repeats preserve the
// first reason and cannot obtain another plan, including after CompleteEnd.
func (l *Lifetime) RequestEnd(input EndRequest) (EndPlan, error) {
	if !slices.Contains(l.enders, input.Requestor) {
		return EndPlan{}, fmt.Errorf("%w: %s: unknown ender %q", ErrInvalidLifetime, l.id, input.Requestor)
	}
	if strings.TrimSpace(input.Reason) == "" {
		return EndPlan{}, fmt.Errorf("%w: %s: end reason is required", ErrInvalidLifetime, l.id)
	}
	if l.phase != LifetimeLive {
		return EndPlan{Reason: l.reason}, nil
	}
	l.phase = LifetimeEnding
	l.reason = input.Reason
	return EndPlan{Accepted: true, Reason: l.reason, Members: l.Snapshot().LiveMembers}, nil
}

// CompleteEnd records successful delivery of an accepted end plan. Repetition is
// idempotent; completion before any accepted request is refused.
func (l *Lifetime) CompleteEnd() error {
	switch l.phase {
	case LifetimeEnding:
		l.phase = LifetimeEnded
		return nil
	case LifetimeEnded:
		return nil
	default:
		return fmt.Errorf("%w: %s: no accepted end to complete", ErrInvalidLifetime, l.id)
	}
}

// MemberRemoved removes one known live member and reports cardinality before and
// after. A known duplicate is idempotent; unknown keys and any membership change
// after end acceptance are refused without altering state.
func (l *Lifetime) MemberRemoved(input MemberRemovedInput) (MemberRemovedOutput, error) {
	if l.phase != LifetimeLive {
		return MemberRemovedOutput{}, fmt.Errorf("%w: %s: membership is frozen while %s", ErrInvalidLifetime, l.id, l.phase)
	}
	if !slices.Contains(l.members, input.Key) {
		return MemberRemovedOutput{}, fmt.Errorf("%w: %s: unknown member %q", ErrInvalidLifetime, l.id, input.Key)
	}
	before := len(l.live)
	_, present := l.live[input.Key]
	delete(l.live, input.Key)
	return MemberRemovedOutput{Removed: present, Before: before, After: len(l.live)}, nil
}
