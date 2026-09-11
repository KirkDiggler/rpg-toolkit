// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "github.com/KirkDiggler/rpg-toolkit/play/intel"

// IntelDelta is the encounter-owned projection of one observer's intel
// changes. The encounter deliberately owns this contract so later encounter
// corrections can be reported alongside the play/intel transitions without
// changing play/intel's payload-opaque API.
type IntelDelta struct {
	FirstContact []intel.Report
	Refreshed    []intel.Subject
	Faded        []intel.Subject

	// Reacquired is every subject who was a GHOST to this observer and is
	// in view again — the exact inverse of Faded, and the moment
	// [Encounter.appendSightedBeats] means by "they came back".
	//
	// It REFINES Refreshed and does not partition it (play/intel's own doc
	// says why): a re-acquired subject appears in both, so anything reading
	// FirstContact ∪ Refreshed as "everything perceived this pass" — which
	// correctArrivedLocations does — keeps the answer it always had.
	Reacquired []intel.Subject

	Corrected []intel.Subject
}

func intelDeltaFromSurveil(in *intel.SurveilOutput) *IntelDelta {
	if in == nil {
		return nil
	}

	return &IntelDelta{
		FirstContact: cloneIntelReports(in.FirstContact),
		Refreshed:    cloneIntelSubjects(in.Refreshed),
		Faded:        cloneIntelSubjects(in.Faded),
		Reacquired:   cloneIntelSubjects(in.Reacquired),
	}
}

// mergeIntelDeltas merges src into dst by observer. Each category is an
// independent fact stream: duplicates are removed within that category while
// preserving the first occurrence's order and value. In particular, Faded
// and Corrected are intentionally not compared with one another.
func mergeIntelDeltas(dst, src map[MemberID]*IntelDelta) map[MemberID]*IntelDelta {
	if dst == nil {
		if src == nil {
			return nil
		}
		dst = make(map[MemberID]*IntelDelta, len(src))
	}

	// Clone existing values before composing so the returned map owns all
	// slices and payload bytes, including values already present in dst.
	for observer, delta := range dst {
		dst[observer] = cloneIntelDelta(delta)
	}

	for observer, incoming := range src {
		if incoming == nil {
			if _, exists := dst[observer]; !exists {
				dst[observer] = nil
			}
			continue
		}

		existing := dst[observer]
		if existing == nil {
			dst[observer] = cloneIntelDelta(incoming)
			continue
		}

		dst[observer] = &IntelDelta{
			FirstContact: mergeIntelReports(existing.FirstContact, incoming.FirstContact),
			Refreshed:    mergeIntelSubjects(existing.Refreshed, incoming.Refreshed),
			Faded:        mergeIntelSubjects(existing.Faded, incoming.Faded),
			Reacquired:   mergeIntelSubjects(existing.Reacquired, incoming.Reacquired),
			Corrected:    mergeIntelSubjects(existing.Corrected, incoming.Corrected),
		}
	}

	return dst
}

func cloneIntelDelta(in *IntelDelta) *IntelDelta {
	if in == nil {
		return nil
	}

	return &IntelDelta{
		FirstContact: cloneIntelReports(in.FirstContact),
		Refreshed:    cloneIntelSubjects(in.Refreshed),
		Faded:        cloneIntelSubjects(in.Faded),
		Reacquired:   cloneIntelSubjects(in.Reacquired),
		Corrected:    cloneIntelSubjects(in.Corrected),
	}
}

func cloneIntelReports(in []intel.Report) []intel.Report {
	if in == nil {
		return nil
	}

	out := make([]intel.Report, len(in))
	for i, report := range in {
		out[i] = intel.Report{
			Subject: report.Subject,
			Payload: cloneIntelPayload(report.Payload),
		}
	}
	return out
}

func cloneIntelSubjects(in []intel.Subject) []intel.Subject {
	if in == nil {
		return nil
	}

	out := make([]intel.Subject, len(in))
	copy(out, in)
	return out
}

func cloneIntelPayload(in []byte) []byte {
	if in == nil {
		return nil
	}

	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func mergeIntelReports(dst, src []intel.Report) []intel.Report {
	if dst == nil && src == nil {
		return nil
	}

	out := make([]intel.Report, 0, len(dst)+len(src))
	seen := make(map[intel.Subject]struct{}, len(dst)+len(src))
	for _, report := range dst {
		if _, exists := seen[report.Subject]; exists {
			continue
		}
		seen[report.Subject] = struct{}{}
		out = append(out, intel.Report{
			Subject: report.Subject,
			Payload: cloneIntelPayload(report.Payload),
		})
	}
	for _, report := range src {
		if _, exists := seen[report.Subject]; exists {
			continue
		}
		seen[report.Subject] = struct{}{}
		out = append(out, intel.Report{
			Subject: report.Subject,
			Payload: cloneIntelPayload(report.Payload),
		})
	}
	return out
}

func mergeIntelSubjects(dst, src []intel.Subject) []intel.Subject {
	if dst == nil && src == nil {
		return nil
	}

	out := make([]intel.Subject, 0, len(dst)+len(src))
	seen := make(map[intel.Subject]struct{}, len(dst)+len(src))
	for _, subject := range dst {
		if _, exists := seen[subject]; exists {
			continue
		}
		seen[subject] = struct{}{}
		out = append(out, subject)
	}
	for _, subject := range src {
		if _, exists := seen[subject]; exists {
			continue
		}
		seen[subject] = struct{}{}
		out = append(out, subject)
	}
	return out
}
