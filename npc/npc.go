// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package npc

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// NPC is a reusable generic non-player-character content record.
type NPC struct {
	ref               *core.Ref
	displayName       string
	capabilities      []Capability
	combatPolicy      CombatPolicy
	observationPolicy ObservationPolicy
	dispositionPolicy DispositionPolicy
	movementPolicy    MovementPolicy
	inventory         json.RawMessage
}

// Config provides authoring values for a new NPC.
//
// New applies authoring defaults for omitted policies. Load is stricter because
// persisted data should already contain the concrete chosen values.
type Config struct {
	Ref               *core.Ref
	DisplayName       string
	Capabilities      []Capability
	CombatPolicy      CombatPolicy
	ObservationPolicy ObservationPolicy
	DispositionPolicy DispositionPolicy
	MovementPolicy    MovementPolicy

	// Inventory is opaque rulebook-owned content this package never reads —
	// see [Data.Inventory]'s own doc for why.
	Inventory json.RawMessage
}

// Data is the serializable form of an NPC.
type Data struct {
	Ref               *core.Ref         `json:"ref,omitempty"`
	DisplayName       string            `json:"display_name"`
	Capabilities      []Capability      `json:"capabilities,omitempty"`
	CombatPolicy      CombatPolicy      `json:"combat_policy"`
	ObservationPolicy ObservationPolicy `json:"observation_policy"`
	DispositionPolicy DispositionPolicy `json:"disposition_policy"`
	MovementPolicy    MovementPolicy    `json:"movement_policy"`

	// Inventory is opaque rulebook-owned content — a vendor's stock, or
	// whatever else a future NPC role needs to carry — that this package
	// never reads, validates, or requires. The same JSON-in/JSON-out
	// convention this toolkit already uses for conditions and features:
	// the rulebook package that knows what "inventory" means marshals its
	// own typed shape into these bytes and unmarshals it back out. This
	// package treats it exactly as it already treats [Capability] — an
	// opaque label another package may route on, carried and never
	// interpreted. Nil is legal and means "no inventory content," which is
	// every NPC that isn't a vendor (or whatever future role uses this
	// slot).
	Inventory json.RawMessage `json:"inventory,omitempty"`
}

// New creates a generic NPC content record.
func New(config Config) (*NPC, error) {
	data := &Data{
		Ref:               config.Ref,
		DisplayName:       config.DisplayName,
		Capabilities:      config.Capabilities,
		CombatPolicy:      defaultCombatPolicy(config.CombatPolicy),
		ObservationPolicy: defaultObservationPolicy(config.ObservationPolicy),
		DispositionPolicy: defaultDispositionPolicy(config.DispositionPolicy),
		MovementPolicy:    defaultMovementPolicy(config.MovementPolicy),
		Inventory:         config.Inventory,
	}

	return load(data)
}

// Load turns serialized NPC data into an NPC.
func Load(data *Data) (*NPC, error) {
	if data == nil {
		return nil, ErrNoData
	}

	return load(data)
}

// Ref returns the NPC's stable content reference.
func (n *NPC) Ref() *core.Ref {
	if n == nil {
		return nil
	}
	return cloneRef(n.ref)
}

// DisplayName returns the player-facing NPC name.
func (n *NPC) DisplayName() string {
	if n == nil {
		return ""
	}
	return n.displayName
}

// Capabilities returns opaque labels that another package may route on.
func (n *NPC) Capabilities() []Capability {
	if n == nil {
		return nil
	}
	return slices.Clone(n.capabilities)
}

// Inventory returns the NPC's opaque rulebook-owned content, unread and
// unvalidated by this package — see [Data.Inventory]'s own doc. Copy-out,
// the same as [NPC.Capabilities]: mutating the returned bytes does not
// mutate the NPC's own stored value.
func (n *NPC) Inventory() json.RawMessage {
	if n == nil {
		return nil
	}
	return slices.Clone(n.inventory)
}

// CombatPolicy returns the NPC's authored combat participation policy.
func (n *NPC) CombatPolicy() CombatPolicy {
	if n == nil {
		return ""
	}
	return n.combatPolicy
}

// ObservationPolicy returns the NPC's authored observation policy.
func (n *NPC) ObservationPolicy() ObservationPolicy {
	if n == nil {
		return ""
	}
	return n.observationPolicy
}

// DispositionPolicy returns the NPC's authored default stance.
func (n *NPC) DispositionPolicy() DispositionPolicy {
	if n == nil {
		return ""
	}
	return n.dispositionPolicy
}

// MovementPolicy returns the NPC's authored movement-occupancy policy.
func (n *NPC) MovementPolicy() MovementPolicy {
	if n == nil {
		return ""
	}
	return n.movementPolicy
}

// ToData returns a serializable copy of the NPC.
func (n *NPC) ToData() *Data {
	if n == nil {
		return nil
	}
	return &Data{
		Ref:               cloneRef(n.ref),
		DisplayName:       n.displayName,
		Capabilities:      slices.Clone(n.capabilities),
		CombatPolicy:      n.combatPolicy,
		ObservationPolicy: n.observationPolicy,
		DispositionPolicy: n.dispositionPolicy,
		MovementPolicy:    n.movementPolicy,
		Inventory:         slices.Clone(n.inventory),
	}
}

func load(data *Data) (*NPC, error) {
	if err := validate(data); err != nil {
		return nil, err
	}

	return &NPC{
		ref:               cloneRef(data.Ref),
		displayName:       strings.TrimSpace(data.DisplayName),
		capabilities:      slices.Clone(data.Capabilities),
		combatPolicy:      data.CombatPolicy,
		observationPolicy: data.ObservationPolicy,
		dispositionPolicy: data.DispositionPolicy,
		movementPolicy:    data.MovementPolicy,
		inventory:         slices.Clone(data.Inventory),
	}, nil
}

func validate(data *Data) error {
	if data.Ref == nil {
		return ErrNoRef
	}
	if err := data.Ref.IsValid(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRef, err)
	}
	if strings.TrimSpace(data.DisplayName) == "" {
		return ErrNoDisplayName
	}
	for i, capability := range data.Capabilities {
		if capability == "" {
			return fmt.Errorf("%w: capability %d", ErrEmptyCapability, i)
		}
	}
	if err := validateCombatPolicy(data.CombatPolicy); err != nil {
		return err
	}
	if err := validateObservationPolicy(data.ObservationPolicy); err != nil {
		return err
	}
	if err := validateDispositionPolicy(data.DispositionPolicy); err != nil {
		return err
	}
	if err := validateMovementPolicy(data.MovementPolicy); err != nil {
		return err
	}
	return nil
}

func defaultCombatPolicy(policy CombatPolicy) CombatPolicy {
	if policy == "" {
		return CombatPolicyNonCombatant
	}
	return policy
}

func defaultObservationPolicy(policy ObservationPolicy) ObservationPolicy {
	if policy == "" {
		return ObservationPolicySubjectOnly
	}
	return policy
}

func defaultDispositionPolicy(policy DispositionPolicy) DispositionPolicy {
	if policy == "" {
		return DispositionPolicyNeutral
	}
	return policy
}

func defaultMovementPolicy(policy MovementPolicy) MovementPolicy {
	if policy == "" {
		return MovementPolicyBlocking
	}
	return policy
}

func validateCombatPolicy(policy CombatPolicy) error {
	switch policy {
	case "":
		return ErrNoCombatPolicy
	case CombatPolicyNonCombatant:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownCombatPolicy, policy)
	}
}

func validateObservationPolicy(policy ObservationPolicy) error {
	switch policy {
	case "":
		return ErrNoObservationPolicy
	case ObservationPolicySubjectOnly, ObservationPolicyObserver:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownObservationPolicy, policy)
	}
}

func validateDispositionPolicy(policy DispositionPolicy) error {
	switch policy {
	case "":
		return ErrNoDispositionPolicy
	case DispositionPolicyNeutral:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownDispositionPolicy, policy)
	}
}

func validateMovementPolicy(policy MovementPolicy) error {
	switch policy {
	case "":
		return ErrNoMovementPolicy
	case MovementPolicyBlocking, MovementPolicyPassable:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownMovementPolicy, policy)
	}
}

func cloneRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}
	return &core.Ref{
		Module: ref.Module,
		Type:   ref.Type,
		ID:     ref.ID,
	}
}
