// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sightSweepVersion names the sight test a stored sweep key was taken
// under. BUMP IT WHENEVER WHAT [sightReach.seesCell] ANSWERS CHANGES FOR THE
// SAME INPUTS — a new lane rule, a new kind of sight blocker, a change to
// what a face teaches — so every stored key stops matching and every
// observer sweeps once under the new rule. A key from an older rule that
// still matched would hold an observer to what the old rule let it see.
const sightSweepVersion = "sweep/1"

// THE SIGHT SWEEP SKIP (rpg-project#664, rpg-toolkit#2097).
//
// [Encounter.discoverCells] asks the sight test of every floor cell an
// observer does not know yet. Its answer depends only on the inputs a
// [sightSweepKey] names, and knowledge is a union that never forgets, so an
// observer whose key is the key of its last sweep would learn nothing from
// sweeping again: every cell the test reaches under those inputs was learned
// by that sweep. Such an observer skips. On the castle that is the guards
// who stand at their posts while somebody else walks — about 93% of the
// sweeps a walk used to run, each learning nothing.
//
// The key is the observer's origin cell, its sight range this pass, the
// field's opacity ([Encounter.sightOpacity]) and the observer's own
// concealment view (the hidden doors and props [sightReach.seesCell] reads
// a cover's face through). Persisted on [EncounterData.SightSweeps], so a
// guard who stands still does not re-sweep on the next verb either; a save
// written before the keys existed carries none, and every observer sweeps
// once and then holds.
//
// THE KEY IS A FINGERPRINT OF WHAT IS READ, NOT A COUNTER OF WHAT WAS
// WRITTEN. A version bumped by each mutator is only as right as the list of
// mutators, and a missed one skips a sweep that should have learned
// something. The reads are few and live in one place — the lane facts
// ([canvasSightObstructions]), the placed footprints and door leaves
// ([field.sightBlocksAlong], [field.screensOf]), the obscuring areas, the
// observer's concealment view — so the fingerprint is taken of exactly
// those, and a change nobody remembered to announce still changes it.
type sightSweepKey string

// sightOpacity is THE FIELD'S OPACITY NOW: a digest of every input to the
// sight test that a verb can change mid-scene. Taken once per sight pass;
// the pass changes none of them.
//
//   - Each door's blocking answer — the one fact its edges and its
//     footprint leaf read ([doorRecord.blocks]). A lock is not sight.
//   - Where each movable placed footprint stands now, or that it is off the
//     floor ([placedFold.stands]): held, in reserve, gone, dropped
//     elsewhere. A fixed placement is construction truth and the field's.
//   - Every canvas entity that blocks sight, and where it stands — the
//     authored props, an arrived one included. Members block nothing.
//   - Every obscuring area standing now, its centre and radius.
//
// The walls, the floor, the void and every fixed placement are the field's
// construction truth: the field is persisted with the run, so a key read
// back against the same blob reads the same field.
func (e *Encounter) sightOpacity() string {
	var b strings.Builder
	b.WriteString(sightSweepVersion)

	doors := make([]*doorRecord, len(e.doors))
	copy(doors, e.doors)
	sort.Slice(doors, func(i, j int) bool { return doors[i].id < doors[j].id })
	for _, d := range doors {
		fmt.Fprintf(&b, "|door:%s=%t", d.id, d.blocks())
	}

	fold := e.field.placedNow()
	for i := range e.field.placed {
		p := &e.field.placed[i]
		if !p.mutable() {
			continue
		}
		placement, stands := fold.stands(p)
		fmt.Fprintf(&b, "|placed:%s=%t@%g,%g", p.id, stands, placement.Origin.X, placement.Origin.Y)
	}

	type blocker struct {
		id  string
		pos spatial.Position
	}
	var blockers []blocker
	for id, ent := range e.canvas.GetAllEntities() {
		placeable, ok := ent.(spatial.Placeable)
		if !ok || !placeable.BlocksLineOfSight() {
			continue
		}
		pos, _ := e.canvas.GetEntityPosition(id)
		blockers = append(blockers, blocker{id: id, pos: pos})
	}
	sort.Slice(blockers, func(i, j int) bool { return blockers[i].id < blockers[j].id })
	for _, bl := range blockers {
		fmt.Fprintf(&b, "|entity:%s@%g,%g", bl.id, bl.pos.X, bl.pos.Y)
	}

	areas := make([]string, 0, len(e.sightAreas))
	for id := range e.sightAreas {
		areas = append(areas, id)
	}
	sort.Strings(areas)
	for _, id := range areas {
		a := e.sightAreas[id]
		fmt.Fprintf(&b, "|area:%s@%g,%g r%d", id, a.Center.X, a.Center.Y, a.RadiusFeet)
	}

	sum := sha256.Sum256([]byte(b.String()))

	return hex.EncodeToString(sum[:])
}

// sweepKeyOf is one observer's key for this pass: where it stands, how far
// it sees, the field's opacity, and which doors and props are hidden from it.
func sweepKeyOf(origin spatial.Position, cells int, opacity string, hidden hiddenView) sightSweepKey {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|at:%g,%g|cells:%d", opacity, origin.X, origin.Y, cells)

	doors := make([]string, 0, len(hidden.doors))
	for id, hid := range hidden.doors {
		if hid {
			doors = append(doors, string(id))
		}
	}
	sort.Strings(doors)
	for _, id := range doors {
		fmt.Fprintf(&b, "|hidden-door:%s", id)
	}

	props := make([]string, 0, len(hidden.props))
	for id, hid := range hidden.props {
		if hid {
			props = append(props, string(id))
		}
	}
	sort.Strings(props)
	for _, id := range props {
		fmt.Fprintf(&b, "|hidden-prop:%s", id)
	}

	sum := sha256.Sum256([]byte(b.String()))

	return sightSweepKey(hex.EncodeToString(sum[:16]))
}

// copySightSweeps is the persisted keys, copied across the data boundary.
// Nil when nobody has swept, so a blob with no sight pass is unchanged.
func copySightSweeps(in map[MemberID]string) map[MemberID]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[MemberID]string, len(in))
	for member, key := range in {
		out[member] = key
	}

	return out
}
