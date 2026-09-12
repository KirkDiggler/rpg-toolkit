// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// IntelDelta is the encounter-owned projection of one observer's perception
// changes. The encounter deliberately owns this contract so later encounter
// additions can be reported alongside the mind/perception transitions without
// changing perception's payload-opaque API.
type IntelDelta struct {
	FirstContact []perception.Presence
	Refreshed    []core.EntityID
	Faded        []core.EntityID

	// Changed is every subject this observer already held whose TESTIMONY is
	// different this pass — somebody moved, or swapped what is in their
	// hands, where the observer could see it.
	//
	// It REFINES Refreshed and does not partition it, exactly as Reacquired
	// does below: a changed subject appears in both. The comparison is the
	// store's own, made at landing and reported rather than re-derived
	// (mind/perception R7) — which is what keeps it correct for a caller that
	// reuses a clock reading across two passes.
	Changed []core.EntityID

	// Reacquired is every subject who was a GHOST to this observer and is
	// in view again — the exact inverse of Faded, and the moment
	// [Encounter.appendSightedBeats] means by "they came back".
	//
	// It REFINES Refreshed and does not partition it (mind/perception's own
	// doc says why): a re-acquired subject appears in both, so anything
	// reading FirstContact ∪ Refreshed as "everything perceived this pass"
	// keeps the answer it always had.
	Reacquired []core.EntityID
}

// intelDeltaFromPerception projects one observer's [perception.Delta] into
// the encounter-owned shape.
func intelDeltaFromPerception(in *perception.Delta) *IntelDelta {
	if in == nil {
		return nil
	}

	return &IntelDelta{
		FirstContact: clonePresences(in.FirstContact),
		Refreshed:    cloneSubjects(in.Refreshed),
		Faded:        cloneSubjects(in.Faded),
		Changed:      cloneSubjects(in.Changed),
		Reacquired:   cloneSubjects(in.Reacquired),
	}
}

// mergeIntelDeltas merges src into dst by observer. Each category is an
// independent fact stream: duplicates are removed within that category while
// preserving the first occurrence's order and value. In particular, Faded
// and Changed are intentionally not compared with one another.
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
			FirstContact: mergePresences(existing.FirstContact, incoming.FirstContact),
			Refreshed:    mergeSubjects(existing.Refreshed, incoming.Refreshed),
			Faded:        mergeSubjects(existing.Faded, incoming.Faded),
			Changed:      mergeSubjects(existing.Changed, incoming.Changed),
			Reacquired:   mergeSubjects(existing.Reacquired, incoming.Reacquired),
		}
	}

	return dst
}

func cloneIntelDelta(in *IntelDelta) *IntelDelta {
	if in == nil {
		return nil
	}

	return &IntelDelta{
		FirstContact: clonePresences(in.FirstContact),
		Refreshed:    cloneSubjects(in.Refreshed),
		Faded:        cloneSubjects(in.Faded),
		Changed:      cloneSubjects(in.Changed),
		Reacquired:   cloneSubjects(in.Reacquired),
	}
}

func clonePresences(in []perception.Presence) []perception.Presence {
	if in == nil {
		return nil
	}

	out := make([]perception.Presence, len(in))
	for i, presence := range in {
		out[i] = perception.Presence{
			ID:      presence.ID,
			Payload: cloneIntelPayload(presence.Payload),
		}
	}
	return out
}

func cloneSubjects(in []core.EntityID) []core.EntityID {
	if in == nil {
		return nil
	}

	out := make([]core.EntityID, len(in))
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

func mergePresences(dst, src []perception.Presence) []perception.Presence {
	if dst == nil && src == nil {
		return nil
	}

	out := make([]perception.Presence, 0, len(dst)+len(src))
	seen := make(map[core.EntityID]struct{}, len(dst)+len(src))
	for _, presence := range dst {
		if _, exists := seen[presence.ID]; exists {
			continue
		}
		seen[presence.ID] = struct{}{}
		out = append(out, perception.Presence{
			ID:      presence.ID,
			Payload: cloneIntelPayload(presence.Payload),
		})
	}
	for _, presence := range src {
		if _, exists := seen[presence.ID]; exists {
			continue
		}
		seen[presence.ID] = struct{}{}
		out = append(out, perception.Presence{
			ID:      presence.ID,
			Payload: cloneIntelPayload(presence.Payload),
		})
	}
	return out
}

func mergeSubjects(dst, src []core.EntityID) []core.EntityID {
	if dst == nil && src == nil {
		return nil
	}

	out := make([]core.EntityID, 0, len(dst)+len(src))
	seen := make(map[core.EntityID]struct{}, len(dst)+len(src))
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
