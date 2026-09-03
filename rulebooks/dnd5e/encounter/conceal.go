// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// conceal.go is THE RUN COMPOSES ITS WORLD (living-world slice 1, wave 1b —
// rpg-toolkit#1371; ruled on rpg-project#350 and #351).
//
// The dungeon carries concealed structure — doors behind a find check
// (DoorInput.Concealed, since v0.41.0) and regions authored as hidden space
// (RegionInput.Concealed) — and until this file, the composition carried it
// opaquely. Here it stops being opaque: at construction, the concealed
// structure is seeded into a world/graph declaration, and WHO KNOWS WHAT is a
// journal of audience-scoped facts folded per member (world v0.3.0's own
// concealment machinery, the kernel the tomb example proved).
//
// One knowledge model, three consequences, each in its own file:
//
//   - [Encounter.Search] rolls a region's find checks (search.go);
//   - [Encounter.AtlasFor] and [Encounter.DoorsFor] answer as one member,
//     under the never-authored yardstick and the masquerade wall
//     (projection.go);
//   - the probe law and the move law make an unfound door unnameable and
//     uncrossable-without-a-trace (doorverbs.go, step.go).
//
// # Knowledge arrives as facts, never as flags
//
// Every way a member comes to know a door or a region — their own search, a
// door opened in their presence, walking up to one standing open, crossing
// it, standing inside a hidden room from frame one — writes the SAME fact
// kind, audienced to the learner alone, and a [graph.Pierce] declared per
// concealed entity folds it into that member's view. The enumerated causes
// are examples of knowledge arriving, not a closed set (ruled on
// rpg-project#350); a new cause is a new writer of an existing fact, never a
// new mechanism. The fold is recomputed from the journal on every question,
// so there is no second copy of "who knows" anywhere to disagree with it.
//
// # Two knowledge moments, deliberately distinct (rpg-project#351)
//
// Finding a door reveals the DOOR alone — knowing where a door is is not
// seeing what is behind it. The region behind it arrives only on perceiving
// the door OPEN: present at the opening, or walking up later (present state,
// the truth grain). And PRESENCE PIERCES: a member standing inside a
// concealed region perceives it — you cannot occupy a secret you do not know
// exists — so a party start inside one is legal authoring and the occupants
// begin knowing.
//
// # The capabilities are supplied, never defaulted
//
// [CheckResolver] rolls a find check; [Witness] answers who currently
// perceives a door. Both are rules this module is not allowed to know
// (rpg-toolkit#1033's law, the same move as Standing and Sight): what a DC
// means is the rulebook's, and how far perception reaches is the host's
// light-and-sight truth. Both are REQUIRED at Setup and Load exactly when
// the field carries concealed structure, refused at the door — and a field
// with none builds no world machinery at all, which is what keeps a plain
// dungeon byte-identical to what it was before this file existed.

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
}

// Witness answers who currently perceives a door's crossings — the injected
// half of "opened in one's presence" and "walking up to it later".
//
// Position, light, line of sight and its reach are the host's truth, not
// this module's (the session's sight seam implements this for the live game
// — rpg-project#351's §22 rung); tests script it. Asked only about OPEN
// concealed doors, because perception of present state is what reveals: a
// shut concealed door is a wall to everyone who has not found it, and no
// amount of standing in front of a wall perceives the door in it.
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
type PerceiversInput struct {
	// Door is the door's identifier.
	Door DoorID

	// Edges are the door's crossings, dungeon-absolute — the cells a
	// perceiver would have to see.
	Edges []DoorEdge
}

// encounterWorld is the run's concealment knowledge: the seeded graph
// declaration (construction truth, rebuilt from the field at every Setup and
// Load) and the journal of knowledge facts (world state, persisted on
// [EncounterData.World] — load-act-save, like every other leaf).
type encounterWorld struct {
	structure *graph.World
	log       *journal.Journal

	// concealedDoors is every concealed door's ID, sorted (C8 — the sweep
	// walks it, and beat order is observable).
	concealedDoors []DoorID

	// concealedRegions is every region authored as hidden space.
	concealedRegions map[RegionID]bool

	// doorRegions maps each concealed door to the concealed regions its
	// edges touch, sorted — the regions perceiving it OPEN reveals.
	doorRegions map[DoorID][]RegionID
}

// worldMembership is the membership relation the seeded graph declares —
// required by the kernel ([graph.Config.Membership]), and deliberately never
// used in an edge: this composition audiences every knowledge fact to
// individual members, so there are no groups for the fold to follow.
const worldMembership graph.Relation = "member-of"

// doorEntityID mints the graph entity ID for a door. Doors and regions are
// both named by plain strings at this seam, so each gets its own prefix
// rather than trusting the two namespaces never to collide.
func doorEntityID(id DoorID) journal.EntityID { return journal.EntityID("door:" + id) }

// regionEntityID mints the graph entity ID for a region.
func regionEntityID(id RegionID) journal.EntityID { return journal.EntityID("region:" + id) }

// doorKnownKind mints the fact kind that pierces one door's concealment.
// One kind per entity, because a [graph.Pierce] fires on kind alone: a
// shared kind would reveal every door on any door's find.
func doorKnownKind(id DoorID) journal.Kind { return journal.Kind("known:door:" + id) }

// regionKnownKind mints the fact kind that pierces one region's concealment.
func regionKnownKind(id RegionID) journal.Kind { return journal.Kind("known:region:" + id) }

// fieldHasConcealment reports whether authored inputs carry any concealed
// structure — the question that decides whether the world machinery is
// built and whether the two concealment capabilities are required.
func fieldHasConcealment(regions []RegionInput, doors []DoorInput) bool {
	for _, r := range regions {
		if r.Concealed {
			return true
		}
	}
	for _, d := range doors {
		if d.Concealed != nil {
			return true
		}
	}
	return false
}

// newEncounterWorld seeds the world from the compiled field: every concealed
// door and region as a concealed graph declaration, pierced by its own
// minted fact kind. The journal starts empty; Load replays persisted facts
// into it afterwards.
func newEncounterWorld(f *field, doors []*doorRecord) (*encounterWorld, error) {
	w := &encounterWorld{
		log:              journal.New(),
		concealedRegions: make(map[RegionID]bool),
		doorRegions:      make(map[DoorID][]RegionID),
	}

	cfg := graph.Config{Membership: worldMembership}

	for _, r := range f.regions {
		if !r.Concealed {
			continue
		}
		w.concealedRegions[r.ID] = true
		cfg.Entities = append(cfg.Entities, graph.Entity{ID: regionEntityID(r.ID), Kind: "region", Concealed: true})
		cfg.Pierces = append(cfg.Pierces, graph.Pierce{
			On:       regionKnownKind(r.ID),
			Entities: []journal.EntityID{regionEntityID(r.ID)},
		})
	}

	for _, d := range doors {
		if d.concealed == nil {
			continue
		}
		w.concealedDoors = append(w.concealedDoors, d.id)
		cfg.Entities = append(cfg.Entities, graph.Entity{ID: doorEntityID(d.id), Kind: "door", Concealed: true})
		cfg.Pierces = append(cfg.Pierces, graph.Pierce{
			On:       doorKnownKind(d.id),
			Entities: []journal.EntityID{doorEntityID(d.id)},
		})

		// The concealed regions this door guards: the regions its edge
		// endpoints stand in, deduplicated and sorted. Perceiving the door
		// OPEN reveals exactly these.
		seen := make(map[RegionID]bool)
		for _, e := range d.edges {
			for _, cell := range []spatial.Position{e.From, e.To} {
				if r, owned := f.regionOf(cell); owned && w.concealedRegions[r] && !seen[r] {
					seen[r] = true
					w.doorRegions[d.id] = append(w.doorRegions[d.id], r)
				}
			}
		}
		sort.Strings(w.doorRegions[d.id])
	}
	sort.Strings(w.concealedDoors)

	structure, err := graph.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("seed world: %w", err)
	}
	w.structure = structure

	return w, nil
}

// knowledgeOf folds one member's present: what the journal's facts, filtered
// to what this member witnessed, let them see. Computed fresh per question —
// nothing present is stored (the kernel's own law).
func (w *encounterWorld) knowledgeOf(member MemberID) *graph.State {
	return w.structure.StateFor(journal.EntityID(member), w.log)
}

// knowsDoor reports whether a member's own fold shows the door.
func (w *encounterWorld) knowsDoor(member MemberID, id DoorID) bool {
	return w.knowledgeOf(member).Visible(doorEntityID(id))
}

// knowsRegion reports whether a member's own fold shows the region.
func (w *encounterWorld) knowsRegion(member MemberID, id RegionID) bool {
	return w.knowledgeOf(member).Visible(regionEntityID(id))
}

// learnDoor writes the fact that pierces one door for one member alone.
// cause is a human-readable trace ([journal.Outcome.Detail]) — the causes
// are exemplary, and the journal records which one it was.
func (w *encounterWorld) learnDoor(member MemberID, id DoorID, cause string) error {
	_, err := w.log.Append(journal.Fact{
		Kind:     doorKnownKind(id),
		Actor:    journal.EntityID(member),
		Subject:  doorEntityID(id),
		Audience: journal.Audience{journal.EntityID(member)},
		Outcome:  journal.Outcome{Detail: cause},
	})
	return err
}

// learnRegion writes the fact that pierces one region for one member alone.
func (w *encounterWorld) learnRegion(member MemberID, id RegionID, cause string) error {
	_, err := w.log.Append(journal.Fact{
		Kind:     regionKnownKind(id),
		Actor:    journal.EntityID(member),
		Subject:  regionEntityID(id),
		Audience: journal.Audience{journal.EntityID(member)},
		Outcome:  journal.Outcome{Detail: cause},
	})
	return err
}

// sweepConcealment is concealment's trigger detection: it notices knowledge
// that present state forces — occupancy of hidden space, and perception of a
// concealed door standing OPEN — writes the facts, and appends the reveal
// beats. It runs inside every sight refresh, for [Encounter.refreshSight]'s
// reason: a rule wired at the verbs is a rule some verb forgets, and
// perceiving present state is a rule about sight.
//
// A no-op for a field with no concealment (there is no world), and
// idempotent: knowledge already held is never re-written and never re-beat.
// Deterministic (C8): members in sorted-ID order, doors in sorted-ID order,
// witness answers sorted before use.
func (e *Encounter) sweepConcealment() error {
	if e.world == nil {
		return nil
	}
	at := uint64(e.clock.ToData().HighWater)

	if err := e.sweepOccupancy(at); err != nil {
		return err
	}

	// Perceiving a concealed door OPEN reveals the door to a non-knower and
	// the regions behind it to every perceiver — present state, so a member
	// walking up later gets their reveal here exactly as one present at the
	// opening did.
	for _, doorID := range e.world.concealedDoors {
		d := e.doorsByID[doorID]
		if d.state.Kind() != DoorOpen {
			continue
		}
		perceivers, err := e.witness.Perceivers(&PerceiversInput{
			Door:  d.id,
			Edges: append([]DoorEdge(nil), d.edges...),
		})
		if err != nil {
			return fmt.Errorf("witness door %q: %w", d.id, err)
		}
		for _, p := range e.sortedPresentMembers(perceivers) {
			if !e.world.knowsDoor(p, d.id) {
				if rerr := e.revealDoorTo(p, d, "perceived it standing open", at); rerr != nil {
					return rerr
				}
			}
			for _, r := range e.world.doorRegions[d.id] {
				if e.world.knowsRegion(p, r) {
					continue
				}
				if rerr := e.revealRegionTo(p, r, "perceived its door open", at); rerr != nil {
					return rerr
				}
			}
		}
	}

	return nil
}

// sweepOccupancy is the presence-pierce half of the sweep, on its own so
// LOAD can run it too (rpg-project#351): a member standing in a concealed
// region perceives it, from the first frame — you cannot occupy a secret
// you do not know exists. LoadEncounter calls this directly for the one
// window the rule would otherwise miss: a blob saved between v0.41.0's
// carried concealment and the world existing holds an occupant with no
// occupancy fact, and their own atlas may not withhold the floor under
// their feet until some verb happens to refresh sight (PR #1373 review,
// Minor 4). Idempotent — knowledge already held is never re-written.
func (e *Encounter) sweepOccupancy(at uint64) error {
	if e.world == nil {
		return nil
	}
	for _, id := range e.rosterIDs() {
		cell, placed := e.canvas.GetEntityPosition(string(id))
		if !placed {
			continue
		}
		region, owned := e.field.regionOf(cell)
		if !owned || !e.world.concealedRegions[region] || e.world.knowsRegion(id, region) {
			continue
		}
		if err := e.revealRegionTo(id, region, "stands inside it", at); err != nil {
			return err
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

// learnCrossedDoors is the crossing cause: a member who just walked through
// a door perceived it as directly as perception gets, whatever the witness
// would have said, and an open concealed door crossed is an open concealed
// door perceived — so the regions it guards arrive with it.
func (e *Encounter) learnCrossedDoors(member MemberID, crossed []CrossedDoor, at uint64) error {
	if e.world == nil {
		return nil
	}
	for _, c := range crossed {
		d, ok := e.doorsByID[c.ID]
		if !ok || d.concealed == nil {
			continue
		}
		if !e.world.knowsDoor(member, d.id) {
			if err := e.revealDoorTo(member, d, "crossed it", at); err != nil {
				return err
			}
		}
		for _, r := range e.world.doorRegions[d.id] {
			if e.world.knowsRegion(member, r) {
				continue
			}
			if err := e.revealRegionTo(member, r, "crossed its door", at); err != nil {
				return err
			}
		}
	}
	return nil
}

// revealDoorTo writes the knowledge fact and the recipient-scoped
// DOOR_REVEALED beat, in that order — the fact is the cause the beat
// narrates.
func (e *Encounter) revealDoorTo(member MemberID, d *doorRecord, cause string, at uint64) error {
	if err := e.world.learnDoor(member, d.id, cause); err != nil {
		return fmt.Errorf("learn door %q: %w", d.id, err)
	}
	if _, err := e.appendDoorRevealedBeat(member, d, at); err != nil {
		return err
	}
	return nil
}

// revealRegionTo writes the knowledge fact and the recipient-scoped
// REGION_REVEALED beat.
func (e *Encounter) revealRegionTo(member MemberID, region RegionID, cause string, at uint64) error {
	// THE MAP AS THEY HAD IT, read before the knowledge fact lands. A reveal
	// beat is a PATCH, and the only honest way to say which walls are new to
	// somebody is to have seen which walls they already had — the alternative
	// is working it out from the footprints a second time, beside the answer
	// rather than from it, which is how a patch and an atlas learn to
	// disagree (PR #1373 review, Minor 1).
	before, err := e.AtlasFor(member)
	if err != nil {
		return fmt.Errorf("region reveal %q: %w", region, err)
	}
	if err := e.world.learnRegion(member, region, cause); err != nil {
		return fmt.Errorf("learn region %q: %w", region, err)
	}
	if _, err := e.appendRegionRevealedBeat(member, region, before, at); err != nil {
		return err
	}
	return nil
}
