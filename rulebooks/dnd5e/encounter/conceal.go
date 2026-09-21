// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// conceal.go is THE RUN COMPOSES ITS WORLD (living-world slice 1, wave 1b —
// rpg-toolkit#1371; ruled on rpg-project#350 and #351, reshaped onto the one
// concealment primitive by rpg-project#490).
//
// The dungeon carries CONCEALMENTS ([ConcealmentInput], concealment.go) —
// each an id, the checks that find it, and the cells, doors and props it
// hides — and until this file, the composition carried them opaquely. Here
// they stop being opaque: at construction each concealment is seeded into a
// world/graph declaration, and WHO KNOWS WHAT is a journal of
// audience-scoped facts folded per member (world v0.3.0's own concealment
// machinery, the kernel the tomb example proved).
//
// One knowledge model, three consequences, each in its own file:
//
//   - [Encounter.Search] rolls the checks of every concealment touching a
//     region (search.go);
//   - [Encounter.AtlasFor] and [Encounter.DoorsFor] answer as one member,
//     under the never-authored yardstick and the masquerade wall
//     (projection.go);
//   - the probe law and the move law make a hidden door unnameable and
//     uncrossable-without-a-trace (doorverbs.go, step.go).
//
// # Knowledge arrives as facts, never as flags
//
// Every way a member comes to know a concealment — their own search, a door
// of it opened in their presence, walking up to one standing open, crossing
// into it, standing inside it from frame one, looting a record that names
// it — writes the SAME fact kind, audienced to the learner alone, and a
// [graph.Pierce] declared per concealment folds it into that member's view.
// The enumerated causes are examples of knowledge arriving, not a closed set
// (ruled on rpg-project#350); a new cause is a new writer of an existing
// fact, never a new mechanism. The fold is recomputed from the journal on
// every question, so there is no second copy of "who knows" anywhere to
// disagree with it.
//
// # ONE knowledge moment, where there were two (rpg-project#490, R1)
//
// Finding a door used to reveal the DOOR alone, and the room behind it
// arrived only on perceiving that door OPEN. Two moments, two fact kinds,
// and an index tying each concealed door to the concealed regions it
// guarded. A concealment is ONE noun: finding it gives you the cells, the
// doors and the props together, because they are one authored secret. And
// PRESENCE PIERCES still: a member standing on a concealment's cell
// perceives it — you cannot occupy a secret you do not know exists — so a
// party start inside one is legal authoring and the occupants begin
// knowing.
//
// # The capabilities are supplied, never defaulted
//
// [CheckResolver] rolls a find check; [Witness] answers who currently
// perceives a door. Both are rules this module is not allowed to know
// (rpg-toolkit#1033's law, the same move as Standing and Sight): what a DC
// means is the rulebook's, and how far perception reaches is the host's
// light-and-sight truth. Both are REQUIRED at Setup and Load exactly when
// the field declares a concealment, refused at the door — and a field with
// none requires neither. The world itself — one journal, one graph — is
// built for every field since rpg-project#375 (world.go); what a plain
// dungeon skips is the two capabilities and the concealment sweep's work,
// which keeps its blob byte-identical to what it was before this file
// existed.

// CheckResolver resolves an authored check for one member: it applies the
// member's best listed approach — the choice is the resolver's, per the
// standing ruling that slice 1 pushes no approach choice to the player
// (rpg-project#350) — rolls it, and reports the verdict.
//
// This is the one seam dice and character sheets enter concealment through.
// The composition hands over the whole approach list and is told the
// verdict; "a total that meets the DC succeeds" is a 5e rule and lives on
// the far side of this interface, exactly as [Encounter.Unlock]'s verdict
// does.
//
// Implementations must be safe for concurrent use.
type CheckResolver interface {
	// ResolveCheck decides one attempt at one authored check. Returning an
	// error means the attempt could not be judged at all — an unknown
	// member, an approach the rulebook does not have — which is a wiring
	// fault, not a failed check.
	ResolveCheck(in *ResolveCheckInput) (*ResolveCheckOutput, error)
}

// ResolveCheckInput is one member against one authored check.
type ResolveCheckInput struct {
	// Member is who attempts the check.
	Member MemberID

	// Approaches are the check's authored routes, each with its own DC —
	// [CheckApproach]'s contract. The resolver applies the member's best
	// listed one.
	Approaches []CheckApproach
}

// ResolveCheckOutput is the resolver's verdict.
type ResolveCheckOutput struct {
	// Beaten is whether the applied route's DC was beaten. The resolver
	// decided; nothing here recomputes it.
	Beaten bool

	// Applied is the route the resolver applied — exactly one of the listed
	// approaches, carried so a beat can name the DC that was actually
	// faced.
	Applied CheckApproach

	// Total is what the check totalled, carried and never compared —
	// [UnlockInput.Total]'s law.
	Total int

	// Calculation is the resolver's full sourced arithmetic for the roll: the
	// d20 pool with every face it threw and the keep record naming any rule
	// that decided which one counted, then the modifier and any bonuses.
	//
	// CARRIED AND NEVER COMPARED, the same law as Total and Beaten. It exists
	// because this verdict used to be three numbers wide, so an untrained
	// character's second d20 face and the word "Untrained" died at the seam
	// and the table saw one number (rpg-project#462). Optional: a resolver
	// that records no arithmetic leaves it nil, and nil means exactly that.
	Calculation *RollCalculation
}

// Witness answers who currently perceives a door's geometry — the injected
// half of "opened in one's presence" and "walking up to it later".
//
// Position, light, line of sight and its reach are the host's truth, not
// this module's (the session's sight seam implements this for the live game
// — rpg-project#351's §22 rung); tests script it. Asked only about a hidden
// door standing OPEN, because perception of present state is what reveals: a
// shut hidden door is a wall to everyone who has not found it, and no amount
// of standing in front of a wall perceives the door in it.
//
// Implementations must be safe for concurrent use.
type Witness interface {
	// Perceivers reports which members currently perceive the given door.
	// IDs that are not members of this encounter are ignored. Returning an
	// error means the question could not be answered at all — a wiring
	// fault, the same as a [CheckResolver] that cannot judge.
	Perceivers(in *PerceiversInput) ([]MemberID, error)
}

// PerceiversInput names the door being perceived and where it stands.
//
// EXACTLY ONE OF THE TWO GEOMETRIES IS FILLED, because a door stands in
// exactly one ([DoorInput]): Edges for a door on a crossing, Cells for a
// door that stands as a rectangle. A witness that reads only Edges would be
// asked an unanswerable question about a footprint door and would have to
// answer "nobody" — a secret that could never be perceived, silently, which
// is the failure mode this module refuses everywhere else.
type PerceiversInput struct {
	// Door is the door's identifier.
	Door DoorID

	// Edges are the door's crossings, dungeon-absolute — the cells a
	// perceiver would have to see. Empty for a footprint door.
	Edges []DoorEdge

	// Cells are the cells a FOOTPRINT door's rectangle stands on,
	// dungeon-absolute ([field.placedCells]) — the same derivation reach
	// and the probe law ask of a rectangle. Empty for an edge door.
	Cells []spatial.Position
}

// concealmentEntityID mints the graph entity ID for a concealment. Prefixed
// rather than bare, so the id namespace cannot collide with a member's.
func concealmentEntityID(id ConcealmentID) journal.EntityID {
	return journal.EntityID("concealment:" + id)
}

// concealmentKnownKind mints the fact kind that pierces one concealment.
// One kind per entity, because a [graph.Pierce] fires on kind alone: a
// shared kind would give away every secret on any one of them being found.
func concealmentKnownKind(id ConcealmentID) journal.Kind {
	return journal.Kind("known:concealment:" + id)
}

// fieldHasConcealment reports whether authored inputs hide anything — the
// question that decides whether the two concealment capabilities are
// required. The world itself is built either way (rpg-project#375,
// world.go): a field with no secret still has sides and knowledge.
func fieldHasConcealment(concealments []ConcealmentInput) bool { return len(concealments) > 0 }

// sweepConcealment is concealment's trigger detection: it notices knowledge
// that present state forces — occupancy of hidden floor, and perception of a
// hidden door standing OPEN — writes the facts, and appends the reveal
// beats. It runs inside every sight refresh, for [Encounter.refreshSight]'s
// reason: a rule wired at the verbs is a rule some verb forgets, and
// perceiving present state is a rule about sight.
//
// A no-op for a field that hides nothing, and idempotent: knowledge already
// held is never re-written and never re-beat. Deterministic (C8):
// concealments in sorted-ID order, their doors in authored order, members in
// sorted-ID order, witness answers sorted before use.
func (e *Encounter) sweepConcealment() error {
	at := uint64(e.clock.ToData().HighWater)

	if err := e.sweepOccupancy(at); err != nil {
		return err
	}

	// Perceiving a hidden door OPEN reveals the whole concealment to every
	// perceiver — present state, so a member walking up later gets their
	// reveal here exactly as one present at the opening did.
	for _, id := range e.world.concealments {
		c := e.field.concealmentOf(id)
		if c == nil {
			continue
		}
		for _, doorID := range c.doors {
			d, ok := e.doorsByID[doorID]
			if !ok || d.state.Kind() != DoorOpen {
				continue
			}
			perceivers, err := e.witness.Perceivers(&PerceiversInput{
				Door:  d.id,
				Edges: append([]DoorEdge(nil), d.edges...),
				Cells: e.doorFootprintCells(d),
			})
			if err != nil {
				return fmt.Errorf("witness door %q: %w", d.id, err)
			}
			for _, p := range e.sortedPresentMembers(perceivers) {
				if e.world.knowsConcealment(p, c.id) {
					continue
				}
				if rerr := e.revealConcealmentTo(p, c, "perceived its door standing open", at); rerr != nil {
					return rerr
				}
			}
		}
	}

	return nil
}

// doorFootprintCells is the cells a FOOTPRINT door stands on, or nil for an
// edge door — the geometry half of what the witness is asked.
func (e *Encounter) doorFootprintCells(d *doorRecord) []spatial.Position {
	if d.placement == nil {
		return nil
	}

	return e.field.placedCells(*d.placement)
}

// sweepOccupancy is the PRESENCE half of the sweep, on its own so LOAD can
// run it too (rpg-project#351): a member standing on a concealment's cell
// perceives it, from the first frame — you cannot occupy a secret you do not
// know exists. LoadEncounter calls this directly for the one window the rule
// would otherwise miss: a blob whose occupant holds no occupancy fact, whose
// own atlas would otherwise withhold the floor under their feet until some
// verb happens to refresh sight (PR #1373 review, Minor 4). Idempotent —
// knowledge already held is never re-written.
//
// PRESENCE TRANSFER RIDES IT TOO (rpg-project#375, design §3.6 and R3): a
// member holding a record that reveals a fact, standing in the region of a
// faction's mind, teaches the mind — the second thing standing somewhere
// makes true, folded by the same sweep, for the same reason. See
// [Encounter.sweepPresence].
func (e *Encounter) sweepOccupancy(at uint64) error {
	for _, id := range e.rosterIDs() {
		cell, placed := e.canvas.GetEntityPosition(string(id))
		if !placed {
			continue
		}
		c := e.concealmentOnCell(cell)
		if c == nil || e.world.knowsConcealment(id, c.id) {
			continue
		}
		if err := e.revealConcealmentTo(id, c, "stands inside it", at); err != nil {
			return err
		}
	}
	return e.sweepPresence(at)
}

// concealmentOnCell is the concealment hiding a cell, or nil — the AUTHORED
// cells and the cells any footprint door of a concealment stands on, which
// is the one set [Encounter.hiddenCellsOf] names and every reader of hidden
// floor asks.
func (e *Encounter) concealmentOnCell(cell spatial.Position) *concealment {
	if id, hidden := e.field.concealmentOfCell[cell]; hidden {
		return e.field.concealmentOf(id)
	}
	for i := range e.field.concealments {
		c := &e.field.concealments[i]
		for _, doorID := range c.doors {
			d, ok := e.doorsByID[doorID]
			if !ok || d.placement == nil {
				continue
			}
			for _, at := range e.field.placedCells(*d.placement) {
				if at == cell {
					return c
				}
			}
		}
	}

	return nil
}

// sortedPresentMembers filters an answer from the witness down to current
// members, deduplicated, in sorted order — the witness's truth is the
// host's, but WHO can be a recipient is this composition's roster.
func (e *Encounter) sortedPresentMembers(ids []MemberID) []MemberID {
	seen := make(map[MemberID]bool, len(ids))
	out := make([]MemberID, 0, len(ids))
	for _, id := range ids {
		if _, ok := e.members[id]; !ok || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// learnCrossedConcealment is THE CROSSING CAUSE: a member who just walked
// through a hidden door, or onto a cell a concealment hides, perceived the
// secret as directly as perception gets — whatever the witness would have
// said about the light.
//
// ONE FUNCTION FOR BOTH HALVES, because they are one event. A door on a
// crossing is walked THROUGH and a concealment's floor is walked ONTO, and
// which of the two a given secret offers is a fact about how it was
// authored, not a rule about walking.
func (e *Encounter) learnCrossedConcealment(
	member MemberID, crossed []CrossedDoor, to spatial.Position, at uint64,
) error {
	for _, c := range crossed {
		id, hidden := e.field.concealmentOfDoor[c.ID]
		if !hidden || e.world.knowsConcealment(member, id) {
			continue
		}
		if err := e.revealConcealmentTo(member, e.field.concealmentOf(id), "crossed its door", at); err != nil {
			return err
		}
	}
	if c := e.concealmentOnCell(to); c != nil && !e.world.knowsConcealment(member, c.id) {
		if err := e.revealConcealmentTo(member, c, "stepped onto it", at); err != nil {
			return err
		}
	}

	return nil
}

// revealConcealmentTo is THE ONE REVEAL WRITER: it writes the knowledge fact
// and the recipient-scoped CONCEALMENT_REVEALED beat, in that order — the
// fact is the cause the beat narrates.
func (e *Encounter) revealConcealmentTo(member MemberID, c *concealment, cause string, at uint64) error {
	if c == nil {
		return nil
	}
	// THE MAP AS THEY HAD IT, read before the knowledge fact lands. A reveal
	// beat is a PATCH, and the only honest way to say which walls are new to
	// somebody is to have seen which walls they already had — the alternative
	// is working it out from the footprints a second time, beside the answer
	// rather than from it, which is how a patch and an atlas learn to
	// disagree (PR #1373 review, Minor 1).
	before, err := e.AtlasFor(member)
	if err != nil {
		return fmt.Errorf("concealment reveal %q: %w", c.id, err)
	}
	if err := e.world.learnConcealment(member, c.id, cause); err != nil {
		return fmt.Errorf("learn concealment %q: %w", c.id, err)
	}
	if _, err := e.appendConcealmentRevealedBeat(member, c, before, at); err != nil {
		return err
	}

	return nil
}
