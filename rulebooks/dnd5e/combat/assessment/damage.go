// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package assessment

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// PrimaryWeaponPool names the marked primary weapon pool, not every damage
// component or the entire attack. Execution already marks that pool explicitly.
const PrimaryWeaponPool = "weapon:primary"

// DamageFrame carries effective facts for an outgoing damage assessment.
// Ability and melee/weapon-pool facts must come after action normalization, not
// from a stale base action. Advantage is effective after keep cancellation.
// Critical names a hypothetical or established branch; it is not a predicted hit.
type DamageFrame struct {
	ActorID       string
	TargetID      string
	Ability       contributions.Fact[abilities.Ability]
	Melee         contributions.Fact[bool]
	HasWeaponPool contributions.Fact[bool]
	Advantage     contributions.Fact[bool]
	Critical      bool
	Context       Context
}

// DamageChange is an unresolved sourced change on an exact pool. Exactly one
// of Fixed or Dice is present; fixed zero remains a real sourced amount.
// DoubleDiceOnCritical declares treatment rather than performing a roll.
type DamageChange struct {
	PoolID               string
	Source               contributions.Source
	Fixed                *int
	Dice                 string
	DoubleDiceOnCritical bool
}

// Clone detaches source and fixed-amount storage.
func (c DamageChange) Clone() DamageChange {
	c.Source = contributions.CloneSource(c.Source)
	if c.Fixed != nil {
		fixed := *c.Fixed
		c.Fixed = &fixed
	}
	return c
}

// AssessDamageInput asks the owning rule about already-normalized damage facts.
type AssessDamageInput struct {
	Frame DamageFrame
}

// AssessDamageOutput separates eligibility from unresolved mechanical changes.
// Non-applicable and pending decisions carry no settled contribution.
type AssessDamageOutput struct {
	Decision contributions.Decision
	Changes  []DamageChange
}

// Validate refuses contradictory answers or empty/ambiguous applicable changes.
// The consuming pool fold additionally validates pool existence and notation.
func (o AssessDamageOutput) Validate() error {
	if err := o.Decision.Validate(); err != nil {
		return err
	}
	if o.Decision.Applicability != contributions.Applies {
		if len(o.Changes) != 0 {
			return fmt.Errorf("unsettled or ineligible damage cannot carry changes")
		}
		return nil
	}
	if len(o.Changes) == 0 {
		return fmt.Errorf("applicable damage requires a change")
	}
	for _, change := range o.Changes {
		if change.PoolID == "" || change.Source.Name == "" ||
			(change.Fixed == nil) == (change.Dice == "") ||
			(change.DoubleDiceOnCritical && change.Dice == "") {
			return fmt.Errorf("damage change requires a pool, source and one amount")
		}
	}
	return nil
}

// DamageRule is a detached owning-rule capability. It cannot roll, spend, publish
// or mutate live state; execution owns those operations at its existing boundary.
type DamageRule interface {
	AssessDamage(*AssessDamageInput) (*AssessDamageOutput, error)
}

// Support declares capability coverage separately from applicability.
type Support string

const (
	// Supported means the owning capability can assess this scoped facet.
	Supported Support = "supported"
	// NotRelevant is an explicit owner declaration, never an inferred default.
	NotRelevant Support = "not_relevant"
	// Unsupported means the affected calculation cannot be presented as complete.
	Unsupported Support = "unsupported"
)

// Coverage declares a rule's support for a named facet with its owner-authored
// reason. Missing entries remain missing coverage, not implicit irrelevance.
type Coverage struct {
	Facet   contributions.Facet
	Support Support
	Reason  string
}

// BindInput supplies evaluation-local identity and deterministic provider order.
// OwnerID is checked against the canonical condition's actual recipient.
type BindInput struct {
	ID      string
	OwnerID string
	Order   int
}

// RuleBinding carries detached source evidence and operation capabilities. It is
// evaluation-local, not a second persisted effect or public command identity.
// Only implemented operations are present; absent capability is not NotRelevant.
type RuleBinding struct {
	ID       string
	Source   contributions.Source
	Address  *dndevents.ConditionAddress
	Order    int
	Coverage []Coverage
	Damage   DamageRule
}

// BindOutput returns one detached rule binding, never a live condition receiver.
type BindOutput struct {
	Binding RuleBinding
}

// SnapshotProvider is implemented at the owning rule, using already-decoded state.
// Registry coverage must still account for rules that do not implement it.
type SnapshotProvider interface {
	AssessmentSnapshot(*BindInput) (*BindOutput, error)
}
