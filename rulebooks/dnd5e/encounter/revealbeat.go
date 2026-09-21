// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// revealbeat.go is THE RECIPIENT-SCOPED REVEAL BEAT — the wire's
// CONCEALMENT_REVEALED, as the composition records it. It is BUILT FOR ITS
// RECIPIENT rather than shared: a non-knower held no honest trace of the
// structure, so the beat carries everything their cached view was
// withholding, and the audience is exactly one member — the
// detection-beats-per-player-from-birth ruling.
//
// # ONE BEAT, WHERE THERE WERE TWO (rpg-project#490, E4)
//
// A door's find wrote DOOR_REVEALED and the room behind it wrote
// REGION_REVEALED, because finding a door and learning what it guarded were
// two knowledge moments. A concealment is ONE noun and one moment, so the
// patch is one message carrying the whole of what was withheld: the cells,
// the props standing on them, the doors with their live state and their
// doorways, the boundaries, the segments the recipient did not have, and the
// sealed cells the reveal replaces.

// BeatConcealmentRevealed is the beat kind a concealment's reveal writes.
//
// EXPORTED because a session-side decoder reads it — [BeatSighted]'s reason,
// and the same shape: the word on the wire is a contract with the host, so
// it is a constant here rather than a string literal each writer spells for
// itself.
const BeatConcealmentRevealed = "concealment_revealed"

// appendConcealmentRevealedBeat records that a concealment entered ONE
// RECIPIENT's knowledge, carrying its whole atlas slice — the cells, THE
// ROOMS THEY WERE CUT OUT OF as the recipient now sees them, the props
// standing on them and the props it hides wherever they stand, the doors it
// hides with their LIVE state and their doorways, every boundary touching
// its cells, THE WALLS IT IS DRAWN WITH and THE CELLS OF IT NOBODY STANDS
// ON. Border walls and the frontier with any still-hidden neighbour are
// included, since the never-authored yardstick withheld them all. The beat
// is the patch for the recipient's cached atlas: the load-once,
// beat-refreshed shape (rpg-project#264).
//
// # Segments, and why they are a difference rather than a slice
//
// A client draws walls from SEGMENTS since rpg-project#360, so a reveal that
// carried a room's boundaries and not its segments would open the secret onto
// a room with no walls — the tell the masquerade exists to remove, arriving at
// the moment it matters most (rpg-toolkit#1480).
//
// Props and boundaries are sliced by asking which of the concealment's cells
// they touch. A segment cannot be asked that: [AtlasSegment] carries no
// footprint, deliberately, because a segment that named the cells it stood on
// would say through the back door what the doorway list withholds. So the
// segments this beat carries are the ones the recipient DID NOT HAVE AND NOW
// DOES — the difference between their atlas before the knowledge landed and
// after. That is a truer reading of a patch anyway: a border wall the
// recipient could already see is not news, and the walls inside the room are
// exactly what was withheld.
//
// # What it does NOT carry, named so it is not mistaken for an omission
//
// A hidden PLACED footprint — a v4 door, a v4 bookcase — is not in this
// payload. [Atlas.Placed] is not on the wire at all (rpg-api-protos#351): the
// session SDK drops the field, so a beat key for it would be inventing a wire
// shape no consumer speaks. Such a thing reaches a recipient when they
// re-read their own atlas, which is what the reveal tells them to do. The
// design names this as the World Builder lane's landing item rather than
// this wave's (rpg-project#490, "The World Builder lane's landing item").
//
// BUILT FROM THE RECIPIENT'S OWN [Encounter.AtlasFor], deliberately: the
// beat documents itself as the patch for that answer, so it is derived from
// that answer rather than recomputed beside it — two computations of one
// truth is how a patch and an atlas learn to disagree (PR #1373 review,
// Minor 1: the first version rebuilt the list from the unscoped Atlas and
// omitted the masquerade mask at the recipient's own still-unfound door
// seam). Everything member-scoped falls out for free: a boundary shared with
// a still-hidden neighbour now PRESENTS, ordinary as any other border wall of
// the space being revealed (rpg-toolkit#1419), and the synthetic mask at a
// door the recipient has NOT found rides the slice exactly as their atlas
// draws it. The recipient's knowledge fact is already written when this runs,
// so the concealment being revealed is present in its own patch.
func (e *Encounter) appendConcealmentRevealedBeat(
	recipient MemberID, c *concealment, before Atlas, at uint64,
) (uint64, error) {
	scoped, err := e.AtlasFor(recipient)
	if err != nil {
		return 0, fmt.Errorf("concealment reveal %q: %w", c.id, err)
	}

	cells := e.hiddenCellsOf(c)
	sortCells(cells)
	owned := make(map[spatial.Position]bool, len(cells))
	for _, cell := range cells {
		owned[cell] = true
	}
	member := make(map[PropID]bool, len(c.props))
	for _, id := range c.props {
		member[id] = true
	}

	props := make([]AtlasProp, 0)
	for _, p := range scoped.Props {
		if owned[p.At] || member[p.ID] {
			props = append(props, p)
		}
	}

	boundaries := make([]AtlasBoundary, 0)
	for _, b := range scoped.Boundaries {
		if owned[b.From] || owned[b.To] {
			boundaries = append(boundaries, b)
		}
	}

	// THE CELLS OF THIS SECRET NOBODY STANDS ON. A sealed cell keeps its
	// region, its lighting and its archetype and loses only feet, so a
	// recipient who has the cells still needs telling which of them are not a
	// place to stand.
	//
	// SCOPED TO THE CONCEALMENT, AND A REPLACEMENT RATHER THAN AN ADDITION —
	// which is why it is not a difference the way the segments are. Cells
	// LEAVE a recipient's sealed set on a reveal: the footing of a wall
	// presented to a non-knower reaches them as ownerless floor, which is
	// floor nobody stands on, and is ordinary standable floor once the secret
	// is theirs. A difference could only ever add. Pinned by
	// TestSealedIsAReplacementWithinTheRoomAndNotAnAddition.
	sealed := make([]spatial.Position, 0)
	for _, cell := range scoped.Sealed {
		if owned[cell] {
			sealed = append(sealed, cell)
		}
	}

	// THE ROOMS THE SECRET WAS CUT OUT OF, as the recipient sees them NOW.
	//
	// A concealment hides CELLS, and those cells sit inside an authored
	// region, so a non-knower's region entry is the authored one with the
	// hidden cells taken out of it — and withheld entirely when none survive
	// (projection.go). A reveal therefore does not only ADD floor, it
	// restores the room that floor belongs to, and a recipient who was
	// handed cells with no region to file them under would be holding floor
	// with no lighting, no archetype and no name.
	//
	// A REPLACEMENT, not a difference — [Atlas.Sealed]'s law on this beat,
	// for the same reason. An entry the recipient already had comes back
	// LARGER, so a difference could only ever say "here is a room you have",
	// which is not the news. The whole entry, as it now stands, is.
	regions := make([]AtlasRegion, 0)
	for _, r := range scoped.Regions {
		if regionTouches(r, owned) {
			regions = append(regions, r)
		}
	}

	payload := map[string]interface{}{
		"beat":        BeatConcealmentRevealed,
		"concealment": c.id,
		"cells":       cells,
		"regions":     revealRegionsPayload(regions),
		"props":       revealPropsPayload(props),
		"doors":       e.revealDoorsPayload(c),
		"boundaries":  revealBoundariesPayload(boundaries),
		"segments":    revealSegmentsPayload(newSegments(before.Segments, scoped.Segments)),
		"sealed":      sealed,
	}

	return e.appendRevealBeat(recipient, payload, at)
}

// revealDoorsPayload renders the doors a concealment hid: identity and LIVE
// state, every doorway (a wide door's edges arrive together), and — when
// locked — the lock's authored approaches. Exactly what a knower's
// [Encounter.DoorsFor] and [Encounter.AtlasFor] now list, which is what the
// retired DOOR_REVEALED carried for one door.
//
// A FOOTPRINT DOOR CARRIES NO DOORWAYS, because it stands in no crossing —
// the wire's empty-is-the-ordinary-case, made structural. Its rectangle
// reaches the recipient through the atlas, for the reason the beat's own doc
// gives.
//
// In the concealment's authored door order, which is the order the author
// wrote them in and the order every other read of this list uses.
func (e *Encounter) revealDoorsPayload(c *concealment) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(c.doors))
	for _, id := range c.doors {
		d, ok := e.doorsByID[id]
		if !ok {
			continue
		}
		doorways := make([]map[string]spatial.Position, 0, len(d.edges))
		for _, edge := range d.edges {
			doorways = append(doorways, map[string]spatial.Position{"from": edge.From, "to": edge.To})
		}
		entry := map[string]interface{}{
			"door":     d.id,
			"state":    string(d.state.Kind()),
			"doorways": doorways,
		}
		if lock, locked := d.state.Lock(); locked {
			entry["approaches"] = approachesDataFrom(lock.Approaches)
		}
		out = append(out, entry)
	}

	return out
}

// newSegments is every wall in `after` that was not in `before`, in after's
// own order.
//
// `BEFORE` MUST BE THE RECIPIENT'S ACTUAL PRIOR PROJECTION — the same
// [Encounter.AtlasFor], with their knowledge as it stood a moment earlier —
// and never a recomputation under some other rule. A wall can already be
// presented to somebody for more than one reason: it stands partly on floor
// they own, it foots on a cell they can see, it is the seam their own door
// hides in. A rule written to pick out "the room's new walls" would have to
// rediscover every one of those reasons and would be wrong the day a new one
// appears; a difference against the answer they actually had cannot be, because
// it asks the same question that produced the answer.
//
// A segment is identified by ITS TWO ENDS, compared BY VALUE, which is the
// whole of what one is on the wire — no name, no footprint, no doors. Both
// ends are exact halves of axial steps, so they compare without a tolerance
// (rpg-project#360's note on why a corner needs no epsilon) and a map key over
// the pair is honest rather than a rounding hazard.
func newSegments(before, after []AtlasSegment) []AtlasSegment {
	had := make(map[[2]AxialPointF]bool, len(before))
	for _, seg := range before {
		had[[2]AxialPointF{seg.From, seg.To}] = true
	}

	out := make([]AtlasSegment, 0)
	for _, seg := range after {
		if had[[2]AxialPointF{seg.From, seg.To}] {
			continue
		}
		out = append(out, seg)
	}

	return out
}

// regionTouches reports whether a region entry holds any of the cells a
// concealment hides — the slice test the props and the boundaries already
// use, asked of a room.
func regionTouches(r AtlasRegion, owned map[spatial.Position]bool) bool {
	for _, cell := range r.Cells {
		if owned[cell] {
			return true
		}
	}

	return false
}

// revealRegionsPayload renders the rooms a reveal restores, each as the whole
// entry the recipient's atlas now carries: the id, the name, every cell they
// now have of it, and the two per-area facts a client dresses it with.
//
// The same shape the retired REGION_REVEALED wrote for its one room, carried
// for however many rooms a secret was cut out of — a concealment is cells,
// and cells can span two rooms where a region flag could only ever mean one.
func revealRegionsPayload(regions []AtlasRegion) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(regions))
	for _, r := range regions {
		out = append(out, map[string]interface{}{
			"id":        r.ID,
			"name":      r.Name,
			"cells":     r.Cells,
			"archetype": r.Archetype,
			"lighting":  map[string]float64{"intensity": r.Lighting.Intensity},
		})
	}

	return out
}

// revealSegmentsPayload renders the walls a reveal newly presents, ends in
// fractional axial exactly as the atlas carries them.
func revealSegmentsPayload(segments []AtlasSegment) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(segments))
	for _, seg := range segments {
		out = append(out, map[string]interface{}{
			"from":   seg.From,
			"to":     seg.To,
			"height": seg.Height,
		})
	}

	return out
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

// appendRevealBeat is the reveal beat's writer: audience of exactly the
// recipient, tagged as a reveal.
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
