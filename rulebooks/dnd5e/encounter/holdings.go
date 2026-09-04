// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// holdings.go is WHO HAS WHAT (rpg-project#368, design §5 and P5 — "one
// mechanism for who has what").
//
// One noun answers it: a HOLDING. The captain holds the way to a door; a
// member who took the artifact holds the artifact; a fallen holder's body
// holds it still, and [Encounter.Loot] takes it back. The parchment shelf
// (design §9) is this same noun with an item in it, which is why it is a
// noun rather than two special cases.
//
// # An append-only journal, folded on every question
//
// Three fact kinds, and present state is what they fold to — never a second
// copy of the answer stored beside them:
//
//   - holds:prop:<id>     — Actor now carries that prop.
//   - holds:intel:door:<id> — Actor now carries the way to that door.
//   - taken:<id>          — that prop has left the floor.
//   - dropped:<id>@<x>,<y> — that prop is back on the floor, at that cell.
//
// A fold in Seq order is the whole rule. Nothing is ever edited and nothing
// is ever removed, which is the [journal.Journal]'s own contract and the
// reason the answer cannot disagree with the record.
//
// # A prop MOVES; intel COPIES
//
// The two kinds of holding fold differently, because they are different
// kinds of thing:
//
//   - A PROP IS AN OBJECT. One of it exists, one person has it, and a later
//     holds:prop fact supersedes the earlier one — looting a body takes the
//     thing off the body.
//   - INTEL IS KNOWLEDGE. Two monsters may be authored knowing one door, two
//     players may loot the same body, and neither takes anything from
//     anyone. So intel folds to a SET of holders per door, and a transfer
//     ADDS one rather than replacing it.
//
// Design §9 is where this stops being true, and says so: the parchment shelf
// makes loot "yield an ITEM that carries the intel instead of the intel
// itself", at which point the intel becomes a prop and inherits the prop
// rule above with nothing here to change. Until then, taking the knowledge
// of a door off a corpse the way you take a coin off it would mean the
// SECOND player to loot the captain learns nothing — which is not a rule
// anybody wrote, and reads at the table as a bug.
//
// # Two prefixes, not one namespace
//
// A prop id and a door id are both plain strings, so each holding kind
// carries its own prefix rather than trusting the two vocabularies never to
// collide — the same call [doorEntityID] makes one file over, for the same
// reason. Design §5 writes the kind as `holds:<placement id>`; splitting it
// into `holds:prop:` and `holds:intel:door:` is that shape with the
// collision closed by construction, so no id needs a shape rule to make the
// fold safe.
//
// # A separate journal from the concealment world's, on purpose
//
// [encounterWorld] exists ONLY when the field carries concealed structure —
// "a field with none builds no world machinery at all, which is what keeps a
// plain dungeon byte-identical to what it was before that file existed"
// (conceal.go). Holdings are not about concealment: a dungeon with no secret
// anywhere can still have a takeable idol on a plinth. So this journal is
// always present and independently persisted, and an encounter where nobody
// holds anything writes NO holdings at all — `holdings` is omitempty, so a
// plain dungeon's blob is byte-identical to what it was before this file
// existed too.
//
// # Nothing here is ever projected
//
// Design P3. The affordance must not say which monster carries intel: Loot
// is offered on every downed member, and a body with nothing to give must be
// indistinguishable from the captain. So no atlas, no percept, no beat and
// no verb output reads a `holds:` fact — the ONLY observable consequence of
// one is what its transfer causes, which for intel is the ordinary
// DOOR_REVEALED beat a search would have produced, and for a prop is the
// prop's own presence on the floor. `taken` and `dropped` are different in
// kind and deliberately so: where a thing physically IS folds on the truth
// grain, the same for everybody (ruled 2026-09-01).

// PropID is the author's name for one placed prop — [PropInput.ID].
//
// An alias rather than a defined type, matching [DoorID] and [RegionID]: an
// id at this seam is a string the author wrote, and the composition carries
// it without interpreting it.
type PropID = string

// ExitID is the author's name for one way out — [FieldExit.ID].
type ExitID = string

// holdsPropKind mints the fact kind recording that somebody carries a prop.
func holdsPropKind(id PropID) journal.Kind { return journal.Kind("holds:prop:" + id) }

// holdsIntelDoorKind mints the fact kind recording that somebody carries the
// way to a door.
func holdsIntelDoorKind(id DoorID) journal.Kind { return journal.Kind("holds:intel:door:" + id) }

// takenKind mints the fact kind recording that a prop has left the floor.
func takenKind(id PropID) journal.Kind { return journal.Kind("taken:" + id) }

// droppedKind mints the fact kind recording that a prop is back on the floor
// at a cell. The cell is IN THE KIND rather than in the outcome, so a fold
// reads it from the one field [journal.Fact] guarantees is never empty, and
// a second drop of the same prop at a different cell is a different kind —
// which is exactly what supersession has to be able to tell apart.
func droppedKind(id PropID, at spatial.Position) journal.Kind {
	return journal.Kind(fmt.Sprintf("dropped:%s@%g,%g", id, at.X, at.Y))
}

// holdingsPrefix and friends are the parse side of the four minters above.
const (
	holdsPropPrefix      = "holds:prop:"
	holdsIntelDoorPrefix = "holds:intel:door:"
	takenPrefix          = "taken:"
	droppedPrefix        = "dropped:"
)

// holdings is the run's answer to who has what: the append-only journal and
// nothing else. Present state is folded from it on every question.
type holdings struct {
	log *journal.Journal
}

// newHoldings returns an empty holdings journal.
func newHoldings() *holdings { return &holdings{log: journal.New()} }

// seedIntel writes the author's knowledge links as the holdings they are
// (design P1): the monster carries the way to each door it was declared to
// know. Construction only — see [MemberInput.Knows] on why Load replays the
// journal instead of re-seeding.
//
// Audience is EMPTY, not the holder: a holding is not a thing that happened
// to anybody, and [journal.Journal] is incurious about who witnessed what.
// Nothing folds these by audience, because nothing but this file reads them.
func (h *holdings) seedIntel(member MemberID, doors []DoorID) error {
	for _, id := range doors {
		if _, err := h.log.Append(journal.Fact{
			Kind:    holdsIntelDoorKind(id),
			Actor:   journal.EntityID(member),
			Subject: journal.EntityID(doorEntityID(id)),
			Outcome: journal.Outcome{Detail: "authored"},
		}); err != nil {
			return fmt.Errorf("seed intel door %q for %q: %w", id, member, err)
		}
	}
	return nil
}

// holdProp records that member now carries the prop.
func (h *holdings) holdProp(member MemberID, id PropID, cause string) error {
	_, err := h.log.Append(journal.Fact{
		Kind:    holdsPropKind(id),
		Actor:   journal.EntityID(member),
		Subject: journal.EntityID("prop:" + id),
		Outcome: journal.Outcome{Detail: cause},
	})
	return err
}

// holdIntelDoor records that member now carries the way to the door.
func (h *holdings) holdIntelDoor(member MemberID, id DoorID, cause string) error {
	_, err := h.log.Append(journal.Fact{
		Kind:    holdsIntelDoorKind(id),
		Actor:   journal.EntityID(member),
		Subject: journal.EntityID(doorEntityID(id)),
		Outcome: journal.Outcome{Detail: cause},
	})
	return err
}

// takeProp records that the prop has left the floor.
func (h *holdings) takeProp(member MemberID, id PropID) error {
	_, err := h.log.Append(journal.Fact{
		Kind:    takenKind(id),
		Actor:   journal.EntityID(member),
		Subject: journal.EntityID("prop:" + id),
	})
	return err
}

// dropProp records that the prop is back on the floor at a cell.
func (h *holdings) dropProp(member MemberID, id PropID, at spatial.Position) error {
	_, err := h.log.Append(journal.Fact{
		Kind:    droppedKind(id, at),
		Actor:   journal.EntityID(member),
		Subject: journal.EntityID("prop:" + id),
	})
	return err
}

// heldItem is one thing a body carries, in the fold's own terms.
type heldItem struct {
	// prop is the prop id when this holding is a thing; empty otherwise.
	prop PropID

	// door is the door id when this holding is the way to one; empty
	// otherwise. Exactly one of prop and door is set.
	door DoorID
}

// heldBy folds one member's present holdings, in a deterministic order:
// intel first by door id, then props by prop id.
//
// SUPERSESSION IS BY SUBJECT, IN SEQ ORDER. The last member to be recorded
// holding a thing is the one holding it, and a prop dropped after that is
// held by nobody. Computed fresh on every question — there is no second copy
// of "who has what" anywhere to disagree with this one.
func (h *holdings) heldBy(member MemberID) []heldItem {
	props, doors := h.fold()

	out := make([]heldItem, 0, len(props)+len(doors))
	doorIDs := make([]DoorID, 0, len(doors))
	for id, holders := range doors {
		if holders[member] {
			doorIDs = append(doorIDs, id)
		}
	}
	sort.Strings(doorIDs)
	for _, id := range doorIDs {
		out = append(out, heldItem{door: id})
	}

	propIDs := make([]PropID, 0, len(props))
	for id, holder := range props {
		if holder == member {
			propIDs = append(propIDs, id)
		}
	}
	sort.Strings(propIDs)
	for _, id := range propIDs {
		out = append(out, heldItem{prop: id})
	}
	return out
}

// fold walks the journal once in Seq order and returns who holds what: each
// prop to its ONE holder, and each door to the SET of members who know the
// way to it — the move/copy split this file's doc comment states. A prop
// nobody holds is absent from the first map, because a dropped thing is held
// by nobody.
//
// ONE WALK, TWO ANSWERS, because every question here needs the same walk and
// two of them would be two chances to fold differently.
func (h *holdings) fold() (props map[PropID]MemberID, doors map[DoorID]map[MemberID]bool) {
	props = map[PropID]MemberID{}
	doors = map[DoorID]map[MemberID]bool{}
	for _, f := range h.log.All() {
		kind := string(f.Kind)
		switch {
		case strings.HasPrefix(kind, holdsIntelDoorPrefix):
			id := strings.TrimPrefix(kind, holdsIntelDoorPrefix)
			if doors[id] == nil {
				doors[id] = map[MemberID]bool{}
			}
			doors[id][MemberID(f.Actor)] = true
		case strings.HasPrefix(kind, holdsPropPrefix):
			props[strings.TrimPrefix(kind, holdsPropPrefix)] = MemberID(f.Actor)
		case strings.HasPrefix(kind, droppedPrefix):
			// A dropped prop is on the floor and held by nobody. The cell
			// lives in the kind and is read by propPlacements, not here.
			id, _, ok := parseDropped(kind)
			if ok {
				delete(props, id)
			}
		}
	}
	return props, doors
}

// propPlacement is where one prop physically is, folded on the truth grain —
// the same answer for every member (ruled 2026-09-01).
type propPlacement struct {
	// gone is whether the prop has been taken off the floor. When true, At
	// is meaningless.
	gone bool

	// at is where a dropped prop now lies. Meaningful only when moved.
	at spatial.Position

	// moved is whether the prop has been dropped somewhere other than where
	// the author placed it.
	moved bool
}

// propPlacements folds where every prop physically is: taken off the floor,
// dropped at a cell, or exactly where the author put it (absent from the
// map). The LAST fact about a prop wins, so a thing taken, dropped and taken
// again is gone.
func (h *holdings) propPlacements() map[PropID]propPlacement {
	out := map[PropID]propPlacement{}
	for _, f := range h.log.All() {
		kind := string(f.Kind)
		switch {
		case strings.HasPrefix(kind, takenPrefix):
			out[strings.TrimPrefix(kind, takenPrefix)] = propPlacement{gone: true}
		case strings.HasPrefix(kind, droppedPrefix):
			if id, at, ok := parseDropped(kind); ok {
				out[id] = propPlacement{at: at, moved: true}
			}
		}
	}
	return out
}

// parseDropped reads a dropped fact's kind back into the prop and the cell
// it names. Reports false for a kind this build did not mint — the trust
// boundary for persisted bytes is [LoadEncounter], and this stays total so a
// fold can never panic on one that slipped through.
func parseDropped(kind string) (PropID, spatial.Position, bool) {
	rest, ok := strings.CutPrefix(kind, droppedPrefix)
	if !ok {
		return "", spatial.Position{}, false
	}
	// The prop id is everything before the LAST '@': a cell never contains
	// one, and an id conceivably could.
	i := strings.LastIndex(rest, "@")
	if i < 0 {
		return "", spatial.Position{}, false
	}
	id, coords := rest[:i], rest[i+1:]
	x, y, ok := strings.Cut(coords, ",")
	if !ok {
		return "", spatial.Position{}, false
	}
	var at spatial.Position
	if _, err := fmt.Sscanf(x, "%g", &at.X); err != nil {
		return "", spatial.Position{}, false
	}
	if _, err := fmt.Sscanf(y, "%g", &at.Y); err != nil {
		return "", spatial.Position{}, false
	}
	return id, at, true
}

// exitAt is the authored exit standing on a cell, or empty when none does.
// Compiled cells compared by EQUALITY, which is [compileEndings]' own rule
// for a positional trigger: one conversion at construction, one comparison at
// play, and no second projection anywhere to disagree with the first.
func (f *field) exitAt(cell spatial.Position) ExitID {
	for _, ex := range f.exits {
		if f.exitCells[ex.ID] == cell {
			return ex.ID
		}
	}
	return ""
}

// heldPropsOf is the prop ids a member is carrying, sorted — the half of
// their holdings that is a PHYSICAL thing, and therefore the half a
// departure can leave on the floor.
//
// INTEL IS NOT HERE, DELIBERATELY. Knowing where a door is is not an object;
// it goes with the person, and a member who leaves takes what they learned
// with them ([ExitOutput.Carry] already carries their percepts out for the
// campaign). Dropping knowledge on a floor tile would be a thing this game
// does not have.
func (e *Encounter) heldPropsOf(member MemberID) []PropID {
	out := []PropID{}
	for _, item := range e.holdings.heldBy(member) {
		if item.prop != "" {
			out = append(out, item.prop)
		}
	}
	return out
}

// firedExitedHolding evaluates every declared [TriggerExitedHolding] against
// one departure: the member stood on the ending's exit cell, and was holding
// the ending's item. Declaration order decides when two could fire at once —
// the same order every ending scan walks.
//
// audience is the roster captured BEFORE the departing member was removed, so
// the beat announcing the run's end reaches the person who ended it.
//
// Returns nil when nothing fired, and short-circuits on an already-closed
// encounter exactly as [Encounter.firedReachedPosition] does: a close is
// written once and narrated once.
func (e *Encounter) firedExitedHolding(
	member MemberID, through ExitID, carried []PropID, at uint64, audience []MemberID,
) (*Outcome, error) {
	// NO GUARDS BUT THE LOOP. Three were written here first and a mutation
	// pass removed all three, because none of them decided anything:
	//
	//   - "they left from no exit" and "they carried nothing" both fall out
	//     of the match below, since an ending names a real exit and a real
	//     item and neither can match an empty answer;
	//   - "the encounter is already closed" is unreachable — [Encounter.Exit]
	//     refuses ErrClosed long before this, and nothing between the two can
	//     close the scene (the sight refresh that can runs afterwards).
	//
	// [Encounter.firedReachedPosition] keeps that last guard and needs it:
	// the sight refresh preceding ITS scan can close the encounter. This one
	// has no such caller, and a branch nothing can reach is a branch nobody
	// can be sure of. A second caller adds it back, with a scene.
	holding := make(map[PropID]bool, len(carried))
	for _, id := range carried {
		holding[id] = true
	}

	for _, de := range e.endings {
		trigger, ok := de.trigger.(TriggerExitedHolding)
		if !ok {
			continue
		}
		if trigger.Exit != through || !holding[trigger.Item] {
			continue
		}
		return e.closeWith(de.key, at, audience...)
	}
	return nil, nil
}

// dropCarried puts everything a departing member was carrying back on the
// floor at the cell they stood on, each with a `dropped` beat to everyone
// present (design R9).
//
// # Why this is not conditioned on which exit
//
// The rule is about the RUN, not about the door: a departure that did not end
// the run drops what the member carried, wherever they left from. An exit no
// ending names is not a way to walk the artifact out of a run that is still
// going — that is the same hole leaving through the lobby would be, and R9's
// stated reason ("otherwise a carrier who leaves... takes the only win out of
// the run with them") is about the hole rather than about the cell. A
// departure that DID end the run drops nothing, because there is no longer a
// run for anybody to be denied.
//
// The dropped prop reappears exactly as the takeable prop it was: same id,
// same ref, same flags, at the drop cell — a fold over the atlas, not a new
// kind of thing (holdings.go).
func (e *Encounter) dropCarried(
	member MemberID, carried []PropID, at spatial.Position, clock uint64, audience []MemberID,
) error {
	for _, id := range carried {
		if err := e.holdings.dropProp(member, id, at); err != nil {
			return fmt.Errorf("drop %q: %w", id, err)
		}
		payload, err := json.Marshal(map[string]interface{}{
			"beat":     "dropped",
			"member":   string(member),
			"prop":     string(id),
			"position": at,
		})
		if err != nil {
			return fmt.Errorf("drop %q: marshal beat: %w", id, err)
		}
		if _, err := e.appendBeat(&record.AppendInput{
			At:       clock,
			Audience: audience,
			Tags:     map[string]string{"tag": "drop"},
			Payload:  payload,
		}); err != nil {
			return fmt.Errorf("drop %q: append beat: %w", id, err)
		}
	}
	return nil
}
