// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// DissolveKind names why a fight ended, in the form the story carries it.
type DissolveKind string

const (
	// DissolveByDecision is a fight its members chose to leave.
	DissolveByDecision DissolveKind = "decision"

	// DissolveByDefeat is a fight that ran out of one side.
	DissolveByDefeat DissolveKind = "defeat"
)

// DissolveCause is why a fight ended: a closed set, sealed the way
// [saves.DCSource] is and for the same reason.
type DissolveCause interface {
	// Kind names which cause this is.
	Kind() DissolveKind

	// isDissolveCause seals the set. See the type's godoc.
	isDissolveCause()
}

// ByDecision is a fight its members chose to walk away from.
func ByDecision() DissolveCause { return byDecision{} }

type byDecision struct{}

func (byDecision) Kind() DissolveKind { return DissolveByDecision }
func (byDecision) isDissolveCause()   {}

// ByDefeat is a fight that ended because one side had nobody left standing in
// it.
func ByDefeat() DissolveCause { return byDefeat{} }

type byDefeat struct{}

func (byDefeat) Kind() DissolveKind { return DissolveByDefeat }
func (byDefeat) isDissolveCause()   {}
