// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// ResourceReader reads a single owned resource's current and maximum through
// the owner (a Character) — the narrow, non-mutating surface a feature uses to
// report a pool it shares with its owner, such as a monk's Ki or a barbarian's
// RageCharges. It never exposes the resource store or its persistence, so a
// projection built on [Status] never has to serialize a feature's ToJSON to
// discover uses.
type ResourceReader interface {
	// ResourceStatus returns the current and maximum for one resource key.
	// ok is false when the owner does not carry that resource.
	ResourceStatus(key coreResources.ResourceKey) (current, maximum int, ok bool)
}

// StatusInput carries the owner a feature asks about a shared resource. The
// owner is optional for features that own their resource privately
// ([SecondWind], [ActionSurge]) and required for features that share a
// character-owned pool (Rage, Flurry of Blows, Patient Defense, Step of the
// Wind).
type StatusInput struct {
	// Owner reads the shared resource pools the feature consumes from but
	// does not store. Nil is valid only for features with no shared resource.
	Owner ResourceReader
}

// ResourceStatus is the immutable projection of one owned resource: its stable
// key, display name, and current/maximum counts. It is a value, not a live
// handle — mutating the owner after reading does not change a previously
// returned ResourceStatus.
type ResourceStatus struct {
	// Key is the stable, opaque core/resources.ResourceKey the web joins to
	// display the resource. It is never derived from a name.
	Key coreResources.ResourceKey

	// Name is the resource's display name. Never empty.
	Name string

	// Current is the currently available amount.
	Current int

	// Maximum is the resource's capacity.
	Maximum int
}

// Status is the immutable, non-mutating status surface one feature exposes:
// its canonical ref, a display name that is never empty, optional
// server-authored detail, and an optional owned resource. It is the value a
// character StatusView (Task 4) deduplicates by resource key and refuses to
// conflict on. No feature serializes ToJSON to produce one.
type Status struct {
	// Ref is the feature's canonical ref — the same ref its ToJSON embeds and
	// its loader routes on.
	Ref core.Ref

	// Name is the feature's display name. Never empty.
	Name string

	// Detail is server/toolkit-composed display text. May be empty; a name is
	// the minimum a status ever reports.
	Detail string

	// Resource is the optional owned resource the feature reports. Nil for
	// features that own no resource (Reckless Attack, Deflect Missiles).
	Resource *ResourceStatus
}

// StatusOutput wraps [Status] for the [StatusProvider] contract. A pointer so
// the projection can tell "the feature reported nothing" (nil) from "the
// feature reported a status with no resource" (non-nil, Resource nil).
type StatusOutput struct {
	Status *Status
}

// StatusProvider reports a feature's non-mutating status surface: its
// canonical ref, display name, optional server-authored detail, and an
// optional owned resource. It is the composition point a character StatusView
// (Task 4) builds from — and it never serializes ToJSON to read status.
// [Feature] embeds it so every loadable feature must name itself honestly.
type StatusProvider interface {
	// Status returns the feature's immutable status. A feature that shares a
	// character-owned resource requires a non-nil [StatusInput.Owner]; a
	// feature with no shared resource ignores the owner.
	Status(*StatusInput) (*StatusOutput, error)
}

// reportKiStatus is the shared body of every monk Ki feature's Status: it
// reads the single shared Ki pool from the owner and reports it under the
// feature's own ref and name. A Ki feature without an owner, or whose owner
// does not carry Ki, is a malformed sheet — the projection fails loudly
// rather than dropping the feature.
func reportKiStatus(in *StatusInput, ref *core.Ref, name, defaultName string) (*StatusOutput, error) {
	if in == nil || in.Owner == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "ki feature status requires an owner resource reader")
	}
	current, maximum, ok := in.Owner.ResourceStatus(resources.Ki)
	if !ok {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not carry ki")
	}
	if name == "" {
		name = defaultName
	}
	return &StatusOutput{Status: &Status{
		Ref:  *ref,
		Name: name,
		Resource: &ResourceStatus{
			Key:     resources.Ki,
			Name:    "Ki",
			Current: current,
			Maximum: maximum,
		},
	}}, nil
}
