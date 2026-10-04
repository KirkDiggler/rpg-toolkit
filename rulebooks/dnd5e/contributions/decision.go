// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

import (
	"fmt"
	"slices"
	"strings"
)

// Facet identifies the scoped mechanical question, not the predicted outcome.
type Facet string

const (
	// AttackRoll describes the attack's terms and keep/threshold policies.
	AttackRoll Facet = "attack_roll"
	// DamageNormal describes normal-hit damage before target defenses.
	DamageNormal Facet = "damage_normal"
	// DamageCritical describes critical-hit damage before target defenses.
	DamageCritical Facet = "damage_critical"
	// SpellDC describes a caster's save DC, not the target's saving throw.
	SpellDC Facet = "spell_dc"
	// Healing describes potential healing, not guaranteed HP restored.
	Healing Facet = "healing"
)

// Valid reports whether the facet is one of the supported bounded questions.
func (f Facet) Valid() bool {
	switch f {
	case AttackRoll, DamageNormal, DamageCritical, SpellDC, Healing:
		return true
	default:
		return false
	}
}

// Applicability is an owning rule's answer. Unsupported evaluation and failed
// validation are not eligibility answers; neither may become DoesNotApply.
type Applicability string

const (
	// Applies means the supplied facts establish this rule's eligibility.
	Applies Applicability = "applies"
	// DoesNotApply means known facts establish ineligibility.
	DoesNotApply Applicability = "does_not_apply"
	// NeedsContext means permitted facts cannot yet settle eligibility.
	NeedsContext Applicability = "needs_context"
)

// NeedKind identifies a factual input, never a rule expression to interpret.
type NeedKind string

const (
	// NeedTarget requests a selected target identity.
	NeedTarget NeedKind = "target"
	// NeedDistance requests distance between two known subjects.
	NeedDistance NeedKind = "distance"
	// NeedRelationship requests a directed relationship for two known subjects.
	NeedRelationship NeedKind = "relationship"
	// NeedSight requests whether Subject sees Other, not the reverse direction.
	NeedSight NeedKind = "sight"
	// NeedUniverse requests sufficient neighborhood knowledge for Subject.
	NeedUniverse NeedKind = "observed_universe"
	// NeedTargetEffects marks absent permitted effect knowledge about Subject.
	NeedTargetEffects NeedKind = "target_effects"
	// NeedReactionReadiness marks absent permitted readiness for Subject.
	NeedReactionReadiness NeedKind = "reaction_readiness"
	// NeedActionFacts marks effective action facts not yet settled by normalization.
	NeedActionFacts NeedKind = "action_facts"
)

// Need names missing information about known subjects. A consumer must not
// populate these identities by enumerating hidden members or hidden providers.
type Need struct {
	Kind    NeedKind
	Subject string
	Other   string
}

// Validate refuses unknown categories and incomplete/contradictory identities.
// It does not establish whether a caller is entitled to the subjects it names.
func (n Need) Validate() error {
	switch n.Kind {
	case NeedTarget:
		if n.Other != "" {
			return fmt.Errorf("target need cannot name an other subject")
		}
	case NeedDistance, NeedRelationship, NeedSight:
		if n.Subject == "" || n.Other == "" || n.Subject == n.Other {
			return fmt.Errorf("%s need requires two distinct subjects", n.Kind)
		}
	case NeedUniverse, NeedTargetEffects, NeedReactionReadiness, NeedActionFacts:
		if n.Subject == "" || n.Other != "" {
			return fmt.Errorf("%s need requires one subject", n.Kind)
		}
	default:
		return fmt.Errorf("unknown information need %q", n.Kind)
	}
	return nil
}

// Decision is the explanation shared by execution and information consumers.
// An operation's output validates its own typed changes/opportunities separately;
// this value does not grant permission to act or decide a stacking winner.
type Decision struct {
	Applicability Applicability
	Reason        string
	Needs         []Need
}

// Validate requires one truthful answer and an owner-authored reason. A pending
// answer needs factual requirements; a settled answer cannot also be pending.
func (d Decision) Validate() error {
	if strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("rule decision requires a reason")
	}
	switch d.Applicability {
	case Applies, DoesNotApply:
		if len(d.Needs) != 0 {
			return fmt.Errorf("settled rule decision cannot carry context needs")
		}
	case NeedsContext:
		if len(d.Needs) == 0 {
			return fmt.Errorf("pending rule decision requires context needs")
		}
		for _, need := range d.Needs {
			if err := need.Validate(); err != nil {
				return fmt.Errorf("invalid rule decision: %w", err)
			}
		}
	default:
		return fmt.Errorf("unknown applicability %q", d.Applicability)
	}
	return nil
}

// Clone detaches a decision's needs from the rule's evaluation-local storage.
func (d Decision) Clone() Decision {
	d.Needs = slices.Clone(d.Needs)
	return d
}
