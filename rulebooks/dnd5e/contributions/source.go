// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

import "github.com/KirkDiggler/rpg-toolkit/core"

// Source identifies a rule-owned fact. Ref names canonical content when present;
// SourceID optionally names its contributing entity. Label describes a narrower
// role within a calculation, not a second identity or an executable predicate.
// This is the common identity behind both unresolved values and resolved traces.
type Source struct {
	Ref      *core.Ref
	Name     string
	Label    string
	SourceID string
}

// CloneSource detaches reference storage while preserving all absence/provenance.
func CloneSource(source Source) Source {
	if source.Ref != nil {
		ref := *source.Ref
		source.Ref = &ref
	}
	return source
}

// DiceContribution is an unresolved homogeneous dice modification. Dice is
// unsigned notation; Subtract preserves the operator without rolling or negating
// that notation. SourceID names the contributing entity, not the recipient.
type DiceContribution struct {
	Source   Source
	Dice     string
	Subtract bool
}

// RollKind names the existing narrow roll-contribution consumer, not a cast DC
// or a prediction of the damage an attack will ultimately inflict.
type RollKind string

const (
	// RollKindAttack identifies an attack roll.
	RollKindAttack RollKind = "attack"
	// RollKindSavingThrow identifies a saving throw.
	RollKindSavingThrow RollKind = "saving_throw"
)
