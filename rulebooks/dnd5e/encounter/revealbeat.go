// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// revealbeat.go is the two recipient-scoped reveal beats — the wire's
// DOOR_REVEALED and REGION_REVEALED (rpg-api-protos v0.1.149), as the
// composition records them. Each is BUILT FOR ITS RECIPIENT rather than
// shared: a non-knower held no honest trace of the structure, so the beat
// carries everything their cached view was withholding, and the audience is
// exactly one member — the detection-beats-per-player-from-birth ruling.

// appendDoorRevealedBeat records that a door entered ONE RECIPIENT's
// knowledge.
//
// The payload is the door's identity and LIVE state, its doorways (every
// edge of a wide door arrives together), and — when locked — the lock's
// authored approaches: exactly what a knower's [Encounter.DoorsFor] and
// [Encounter.AtlasFor] would now list. There is deliberately no boundaries
// key: a door's edges carry no authored boundary (validateDoorInputs
// refuses a door standing in a wall), so the truth that replaces a mask is
// nothing standing there — the wire's empty-is-the-ordinary-case, made
// structural.
func (e *Encounter) appendDoorRevealedBeat(recipient MemberID, d *doorRecord, at uint64) (uint64, error) {
	doorways := make([]map[string]spatial.Position, 0, len(d.edges))
	for _, edge := range d.edges {
		doorways = append(doorways, map[string]spatial.Position{"from": edge.From, "to": edge.To})
	}
	payload := map[string]interface{}{
		"beat":     "door_revealed",
		"door":     d.id,
		"state":    string(d.state.Kind()),
		"doorways": doorways,
	}
	if lock, locked := d.state.Lock(); locked {
		payload["approaches"] = approachesDataFrom(lock.Approaches)
	}

	return e.appendRevealBeat(recipient, payload, at)
}

// appendRegionRevealedBeat records that a concealed region entered ONE
// RECIPIENT's knowledge, carrying the region's whole atlas slice — the
// region entry, the props standing in it, and every boundary touching its
// cells, border walls included, since the never-authored yardstick withheld
// them all. The beat is the patch for the recipient's cached atlas: the
// load-once, beat-refreshed shape (rpg-project#264).
//
// BUILT FROM THE RECIPIENT'S OWN [Encounter.AtlasFor], deliberately: the
// beat documents itself as the patch for that answer, so it is derived from
// that answer rather than recomputed beside it — two computations of one
// truth is how a patch and an atlas learn to disagree (PR #1373 review,
// Minor 1: the first version rebuilt the list from the unscoped Atlas and
// omitted the masquerade mask at the recipient's own still-unfound door
// seam). Everything member-scoped falls out for free: a boundary shared
// with a still-hidden neighbour stays withheld (the Wave 1b interpretation
// pin), and the synthetic mask at a concealed door the recipient has NOT
// found rides the slice exactly as their atlas draws it. The recipient's
// knowledge fact is already written when this runs, so the region being
// revealed is present in its own patch.
func (e *Encounter) appendRegionRevealedBeat(recipient MemberID, region RegionID, at uint64) (uint64, error) {
	scoped, err := e.AtlasFor(recipient)
	if err != nil {
		return 0, fmt.Errorf("region reveal %q: %w", region, err)
	}

	var entry *AtlasRegion
	for i := range scoped.Regions {
		if scoped.Regions[i].ID == region {
			entry = &scoped.Regions[i]
			break
		}
	}
	if entry == nil {
		return 0, fmt.Errorf("region reveal %q: %w", region, ErrNoRegion)
	}

	owned := make(map[spatial.Position]bool, len(entry.Cells))
	for _, c := range entry.Cells {
		owned[c] = true
	}

	props := make([]AtlasProp, 0)
	for _, p := range scoped.Props {
		if owned[p.At] {
			props = append(props, p)
		}
	}

	boundaries := make([]AtlasBoundary, 0)
	for _, b := range scoped.Boundaries {
		if owned[b.From] || owned[b.To] {
			boundaries = append(boundaries, b)
		}
	}

	payload := map[string]interface{}{
		"beat": "region_revealed",
		"region": map[string]interface{}{
			"id":        entry.ID,
			"name":      entry.Name,
			"cells":     entry.Cells,
			"archetype": entry.Archetype,
			"lighting":  map[string]float64{"intensity": entry.Lighting.Intensity},
		},
		"props":      revealPropsPayload(props),
		"boundaries": revealBoundariesPayload(boundaries),
	}

	return e.appendRevealBeat(recipient, payload, at)
}

// revealPropsPayload renders props for a reveal beat, cells dungeon-absolute.
func revealPropsPayload(props []AtlasProp) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(props))
	for _, p := range props {
		out = append(out, map[string]interface{}{
			"ref":                  p.Ref,
			"at":                   p.At,
			"blocks_movement":      p.BlocksMovement,
			"blocks_line_of_sight": p.BlocksLineOfSight,
			"facing":               p.Facing,
			"offset":               p.Offset,
		})
	}
	return out
}

// revealBoundariesPayload renders boundaries for a reveal beat, endpoints
// dungeon-absolute and normalized, exactly as the atlas carries them.
func revealBoundariesPayload(boundaries []AtlasBoundary) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(boundaries))
	for _, b := range boundaries {
		out = append(out, map[string]interface{}{
			"from":                 b.From,
			"to":                   b.To,
			"blocks_movement":      b.BlocksMovement,
			"blocks_line_of_sight": b.BlocksLineOfSight,
			"height":               b.Height,
		})
	}
	return out
}

// appendRevealBeat is the one writer both reveal beats share: audience of
// exactly the recipient, tagged as a reveal.
func (e *Encounter) appendRevealBeat(recipient MemberID, payload map[string]interface{}, at uint64) (uint64, error) {
	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal reveal beat: %w", err)
	}

	out, err := e.appendBeat(&record.AppendInput{
		Audience: []MemberID{recipient},
		Tags:     map[string]string{"tag": "reveal"},
		Payload:  beatBytes,
		At:       at,
	})
	if err != nil {
		return 0, fmt.Errorf("append reveal beat: %w", err)
	}

	return out.Seq, nil
}
