package encounter

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
)

// knowledge.go is rpg-toolkit#851: the toolkit-owned persistence and
// reconciliation of per-viewer geometry knowledge — the ViewerKnowledge
// seam rpg-api specified before this existed (its
// internal/handlers/dnd5e/v2/encounter/knowledge.go). See
// perception/knowledge.go's file doc for the full contract this
// implements; this file is the WORLD-TRUTH half — perception owns the
// types and the pure Memory data structure, this file owns building a
// HexObservation from e.data and deciding which hexes get refreshed when.

// KnownHexes returns every hex playerID has ever observed, keyed by
// position, each carrying the last AUTHORIZED observation with State set
// to whether that hex is visible to playerID RIGHT NOW or merely
// remembered. Hexes playerID has never observed are absent — never present
// with a zero value (see perception.HexObservation's doc; there is no
// Unseen value).
//
// This is the toolkit's implementation of rpg-api's ViewerKnowledge
// interface (KnownHexes(viewer) map[core.Hex]HexObservation) — same method
// name and shape, deliberately, so rpg-api's adapter becomes a direct
// delegation once it consumes this instead of its own toolkitKnowledge
// stand-in.
//
// State is derived HERE, at read time, from playerID's CURRENT
// VisibleHexesAt — not stored per-observation and trusted. Every write
// path in this file (refreshObservations) only ever writes an observation
// for a hex it has just confirmed is currently visible, so the state
// recorded in Memory at write time is always "this was visible when
// observed" — accurate history, but not the live truth for hexes whose
// visibility has since changed. Deriving state at read time is what makes
// "the viewer walked away and came back" (no code ran, nothing was
// written) still report REMEMBERED correctly.
func (e *Encounter) KnownHexes(playerID core.PlayerID) map[core.Hex]perception.HexObservation {
	p, ok := e.data.Players[playerID]
	if !ok || p.View == nil {
		return nil
	}
	visible := perception.VisibleHexesAt(p.View.Position, p.View.SightRange, e.room)
	out := make(map[core.Hex]perception.HexObservation, len(p.View.Memory))
	for h, obs := range p.View.Memory {
		if visible.Has(h) {
			obs.State = perception.KnowledgeStateVisible
		} else {
			obs.State = perception.KnowledgeStateRemembered
		}
		out[h] = obs
	}
	return out
}

// refreshObservations writes a fresh, current-world-truth HexObservation
// into view.Memory for every hex in hexes — the one place this package
// tells a viewer's memory "this is what you now, authoritatively, see".
// Callers choose WHICH hexes belong in that set (the mover's own
// newly-visible hexes, the specific hexes another mover was actually seen
// crossing, a newly-visible monster's own hex, a door's full post-open
// sightline, ...); this function makes no visibility decision of its own —
// it trusts hexes is already the caller's correct current-visible answer
// for view. Passing a hex the viewer cannot actually see would leak world
// truth, exactly the leak this whole contract exists to close, so callers
// must derive hexes from a real VisibleHexesAt/ProjectMove/ProjectDoorOpen
// result, never invent one.
//
// A hex's Edges/Contents are rebuilt from CURRENT e.data every call, never
// patched — the same "Visible is TOTAL, replace wholesale" rule
// HexObservation's doc states for the wire applies identically to memory:
// Contents comes ENTIRELY from placementsAt's current-world scan, which
// finds every player/monster/obstacle actually AT that hex right now — so
// if something else is already standing on a hex being refreshed here (a
// second monster the caller doesn't know about), it's included, because
// placementsAt scans ALL of e.data, not just the caller's one entity of
// interest.
//
// rpg-toolkit#858: this used to also take an entityID parameter, unioned
// into every written hex's Contents regardless of whether placementsAt
// already found that entity there — meant for the pass-through-move case,
// but its one caller (applyAndPublishMove's per-viewer crossing refresh)
// passed the SAME entity id for every hex in a multi-hex crossing, so a
// mover watched crossing several hexes ended up recorded as present on ALL
// of them, not just the one they actually stopped on — a stale, fabricated
// placement on every hex they'd since left. The fix is that this function
// needs no entity-forcing at all: applyAndPublishMove mutates the mover's
// stored position to their final hex BEFORE calling this, so
// placementsAt's own current-position scan already gets it exactly right —
// present at the hex they actually ended on (if that hex is one this
// viewer refreshes), absent from every other hex, and absent everywhere
// for a pass-through mover whose final hex isn't a hex this viewer saw at
// all. See applyAndPublishMove's call site for the full reasoning.
func (e *Encounter) refreshObservations(view *perception.View, hexes core.HexSet) {
	if view == nil || len(hexes) == 0 {
		return
	}
	edges := e.edgesByHex()
	for h := range hexes {
		// rpg-toolkit#859: hexes is frequently a raw perception.VisibleHexesAt
		// disc (sight-range radius around a position), which has no notion of
		// whether a hex is part of the space at all — it happily includes the
		// void beyond the room's walls. Confine what gets remembered to hexes
		// that are actually part of the space; see isSpaceHex's doc for why
		// this can't be approximated with RegionAt.
		if !e.isSpaceHex(h) {
			continue
		}
		obs := perception.HexObservation{
			Position: h,
			State:    perception.KnowledgeStateVisible,
			Edges:    edges[h],
			Contents: e.placementsAt(h),
		}
		if e.data.Space != nil {
			if zoneID, ok := e.data.Space.RegionAt(h); ok {
				obs.ZoneID = zoneID
			}
		}
		view.Observe(obs)
	}
}

// isSpaceHex reports whether h is actually part of this encounter's space —
// not merely within sight-range distance of some viewer position.
// rpg-toolkit#859: three live encounters measured ~80% of a viewer's stored
// Memory as hexes with no floor beneath them at all — perception.
// VisibleHexesAt returns every hex within sightRange that isn't
// wall-blocked, with no notion of whether a hex belongs to the space, so
// refreshObservations was faithfully recording the void beyond the room's
// walls right alongside real floor.
//
// RegionAt alone is NOT this test: a door's own cell deliberately belongs to
// no region (RegionAt(door.Position) is always ("", false) by construction —
// see doorPassageNeighbor's doc), and corridors may be similarly unregioned
// in future generators. Filtering on "has a region" would silently drop
// doorways from memory — a worse bug than the one being fixed, since the
// player would lose the doorway they are standing in.
//
// The real test is the room's own grid bounds. rebuildRoomFromData builds
// e.room's HexGrid from exactly SpaceData.Width/Height (spatial.HexGridConfig
// {Width: sd.Width, Height: sd.Height}), and every generator this package
// ships lays hexes out with NO gaps inside that rectangle:
//   - InitRoom: a single implicit region spanning the whole grid — the
//     entire rectangle IS the floor.
//   - InitDungeon (InitTwoChamberRoom included, generalized to N=2):
//     generateDungeonLayout places each region's columns contiguously
//     (starts[i] = x; x += r.Width + 1) with the boundary/door column
//     immediately after — the shared rectangle is exactly regions plus
//     their connecting door columns, tiled with zero slack.
//
// So "within the room's grid bounds" IS "part of the space" for every shape
// this package generates, doors included, with no per-generator
// special-casing and no dependency on region tagging at all — precisely the
// real membership answer the space itself already has, not an approximation.
//
// A nil e.room (InitRoom/LoadFromData-with-Space never ran) means there is
// no room to be outside of: every hex is accepted, preserving this
// package's pre-wave-1 unbounded-radius behavior for non-spatial encounters.
func (e *Encounter) isSpaceHex(h core.Hex) bool {
	if e.room == nil {
		return true
	}
	return e.room.GetGrid().IsValidPosition(h.ToPosition())
}

// placementsAt returns a Placement for every player, monster, and static
// obstacle CURRENTLY at position h — the complete, TOTAL current occupancy
// (HexObservation's doc: an empty result is a positive claim the hex is
// empty, not "not computed"). Sorted by entity id for deterministic output
// — Go randomizes map iteration, and without this two calls against
// unchanged data could disagree on order, breaking Memory.Observe's
// idempotency guarantee (see its doc).
//
// Facing is always the Placement zero value: nothing in the toolkit tracks
// entity facing today (perception.Placement's own doc says so plainly).
func (e *Encounter) placementsAt(h core.Hex) []perception.Placement {
	var out []perception.Placement
	for _, p := range e.data.Players {
		if p.View != nil && p.View.Position == h {
			out = append(out, perception.Placement{EntityID: p.EntityID})
		}
	}
	for _, m := range e.data.Monsters {
		if m.Position == h {
			out = append(out, perception.Placement{EntityID: m.ID})
		}
	}
	if e.data.Space != nil {
		for _, o := range e.data.Space.Obstacles {
			if o.Position == h {
				out = append(out, perception.Placement{EntityID: o.ID})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i].EntityID) < string(out[j].EntityID) })
	return out
}

// edgesByHex indexes every wall and door segment in the encounter's
// persisted room by the hex it sits on, so refreshObservations can attach
// a hex's own edges without re-scanning every wall/door per hex. Mirrors
// rpg-api's OWN edgesByHex (knowledge_adapter.go) — that copy becomes dead
// code once rpg-api consumes this method instead of re-deriving it from
// world truth it has no business reading directly.
//
// Each hex's edge list is sorted by (To.Q, To.R, To.S) then DoorID for
// deterministic output — the same idempotency reasoning as placementsAt.
func (e *Encounter) edgesByHex() map[core.Hex][]perception.Edge {
	out := make(map[core.Hex][]perception.Edge)
	if e.data.Space != nil {
		for _, w := range e.data.Space.Walls {
			if !w.BlocksMovement && !w.BlocksLoS {
				// Not a barrier worth remembering — matches the wire
				// boundary's own "not a wall worth putting on the wire" gate.
				continue
			}
			from := core.HexFromCube(w.Start)
			out[from] = append(out[from], perception.Edge{
				From:           from,
				To:             core.HexFromCube(w.End),
				BlocksMovement: w.BlocksMovement,
				BlocksLoS:      w.BlocksLoS,
			})
		}
	}
	for id, door := range e.data.Doors {
		if door == nil {
			continue
		}
		out[door.Position] = append(out[door.Position], perception.Edge{
			From: door.Position,
			To:   e.doorPassageNeighbor(door),
			// A closed door blocks both; an open one blocks neither — the
			// observer reads DoorOpen/DoorLocked, so these flags describe
			// the barrier rather than re-deriving the door's kind.
			BlocksMovement: !door.Open,
			BlocksLoS:      !door.Open,
			DoorID:         string(id),
			DoorOpen:       door.Open,
			DoorLocked:     door.Locked,
		})
	}
	for h, edges := range out {
		sort.Slice(edges, func(i, j int) bool {
			a, b := edges[i].To, edges[j].To
			if a.Q != b.Q {
				return a.Q < b.Q
			}
			if a.R != b.R {
				return a.R < b.R
			}
			if a.S != b.S {
				return a.S < b.S
			}
			return edges[i].DoorID < edges[j].DoorID
		})
		out[h] = edges
	}
	return out
}

// doorPassageNeighbor returns door's designated passage-edge neighbor: the
// door's own cell belongs to NEITHER region (RegionAt(door.Position) is
// always ("", false) by construction — doors sit between two regions'
// tagged hex sets, never inside either), so this walks HexNeighbors and
// returns the first neighbor RegionAt tags into ANY region — the one
// designated passage edge, so a renderer draws a single door frame on that
// edge instead of an ambiguous isolated-hex wall. Falls back to the door's
// own position (a degenerate Start==To segment, matching a solid wall's
// shape) when no tagged neighbor exists, including when e.data.Space is
// nil — SpaceData.RegionAt is itself nil-receiver safe, so this guard is
// belt-and-suspenders documentation as much as a runtime necessity.
func (e *Encounter) doorPassageNeighbor(door *DoorData) core.Hex {
	if e.data.Space == nil {
		return door.Position
	}
	for _, n := range perception.HexNeighbors(door.Position) {
		if _, ok := e.data.Space.RegionAt(n); ok {
			return n
		}
	}
	return door.Position
}
