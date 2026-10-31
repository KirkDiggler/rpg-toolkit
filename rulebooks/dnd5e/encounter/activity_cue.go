// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "github.com/KirkDiggler/rpg-toolkit/mind/behavior"

// ActivityCueData preserves a spoken answer's optional presentation intent in
// the saved table and selected record. The asset provider owns its meaning.
type ActivityCueData struct {
	// Ref is the captured, provider-owned activity reference.
	Ref string `json:"ref"`
	// Cycles is the positive sustain count, excluding entry and exit.
	Cycles uint32 `json:"cycles"`
}

func activityDataFrom(cue *behavior.ActivityCue) *ActivityCueData {
	if cue == nil {
		return nil
	}
	return &ActivityCueData{Ref: cue.Ref, Cycles: cue.Cycles}
}

func activityFromData(data *ActivityCueData) *behavior.ActivityCue {
	if data == nil {
		return nil
	}
	return &behavior.ActivityCue{Ref: data.Ref, Cycles: data.Cycles}
}
