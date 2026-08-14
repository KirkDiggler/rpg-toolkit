// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import "errors"

var (
	// ErrNilInput indicates a caller defect: Resolve was handed no input at all.
	ErrNilInput = errors.New("resolution: nil input")

	// ErrNoMachine indicates an interaction with nothing to resolve. Distinct
	// from a machine that finishes immediately, which is legal.
	ErrNoMachine = errors.New("resolution: no machine")

	// ErrBadParticipant indicates a participant that is not one sheet: neither
	// a character nor a monster, both at once, or sharing an ID with another.
	ErrBadParticipant = errors.New("resolution: bad participant")

	// ErrBadStep indicates a step this package did not construct, or one it has
	// no case for. Sealed vocabularies are only sealed if the driver says so.
	ErrBadStep = errors.New("resolution: unrecognized step")

	// ErrNoSaver indicates a saving throw naming a participant that was not
	// passed in. Silently rolling without the saver's modifier would be a
	// wrong answer wearing a right one's clothes.
	ErrNoSaver = errors.New("resolution: saver is not a participant")
)
