// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import "errors"

// ErrBadActivity reports a spoken activity cue without a line, reference or
// positive sustain count. The primitive validates shape, never asset support.
var ErrBadActivity = errors.New("behavior: activity cue cannot be read")

// ActivityCue is presentation intent carried with a selected spoken answer.
// It affects neither eligibility nor weight; the consumer owns playback.
type ActivityCue struct {
	// Ref is an opaque, nonempty provider-owned activity reference.
	Ref string
	// Cycles counts complete sustain cycles, excluding entry and exit. It is
	// positive; zero is not an infinite-playback spelling.
	Cycles uint32
}
