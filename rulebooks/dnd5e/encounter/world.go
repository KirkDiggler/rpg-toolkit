// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// world.go is THE ONE WORLD (rpg-project#375, the hold-out design §3, and
// the law it states: "The run's world is the only state. Content declares
// it, verbs append to it, readers fold over it, the projection presents it.
// No reader keeps a copy.")
//
// One journal and one graph exist from New and from Load, whether or not
// anything is concealed. Before this file the run kept two: a concealment
// world built only when the field hid something (conceal.go) and a holdings
// journal with no audiences (holdings.go). Both fold into this one — the
// same fact kinds, every fact audienced to whoever it happened to — and the
// two readers this slice moves, SIDES and KNOWLEDGE, ask it and nothing else.
//
// # What the graph declares
//
// Entities: every faction (the two reserved ones and the declared ones),
// every current member, and the concealed regions and doors. Edges:
// `belongs-to` from each member to its faction, and the stance edges —
// `hostile-to` or `allied-with`, one per direction — for every pair of
// factions, declared or default (disposition.go). Reducers: a [graph.Raise]
// per fact id the field mentions, raising `knows:<fact>` on whoever a
// `known:fact:<fact>` fact is about. Projections: a [graph.Settle] per
// disposition with an `until: { fact }`, per mind of its pair — while the
// mind carries the flag, the pair's stance edges are gone in both directions
// (design R11, Kirk: "the graph should tell the truth").
//
// # The graph is construction truth; the journal is the run
//
// The declaration is rebuilt from the field and the roster at every Setup,
// Load, Join and Exit — a member is an entity, so the roster changing is the
// declaration changing — and it is never stored. The journal persists, on
// [EncounterData.World] and [EncounterData.Holdings] exactly as before, split
// by kind at the storage boundary and replayed into the one log at load. A
// pair's stance is therefore derived on every question from the declaration
// plus the facts: nothing anywhere stores "the raiders turned".
//
// # Whose fold
//
// Knowledge is somebody's: `knowsFact(member, id)` folds as that member. A
// pair's stance folds as the pair's MIND — the observer whose flag the
// Settle reads — so a scout who reads the letter changes nothing (R3), and a
// pair no mind can turn folds as nobody: the declaration alone, never
// [graph.World.Truth], which would count the scout's fact too.

// Relations the graph declares. worldMembership is the kernel's required
// belonging relation ([graph.Config.Membership]); it is what makes a fact
// audienced to a faction reach its members, and what [graph.State.FactionOf]
// follows.
const (
	worldMembership graph.Relation = "belongs-to"
	relHostileTo    graph.Relation = "hostile-to"
	relAlliedWith   graph.Relation = "allied-with"
)

// factKnownPrefix and factKnownKind are the fact kind that records a member
// coming to know a fact — `known:fact:<id>`, beside `known:door:` and
// `known:region:`. The fact's ACTOR and SUBJECT are both the learner, and its
// audience is the learner alone: the subject is what [graph.Raise] flags, the
// audience is whose fold carries it.
const factKnownPrefix = "known:fact:"

func factKnownKind(id FactID) journal.Kind { return journal.Kind(factKnownPrefix + id) }

// knowsFlag is the flag a `known:fact:<id>` fact raises on its learner.
func knowsFlag(id FactID) graph.Flag { return graph.Flag("knows:" + id) }

// factionEntityID mints the graph entity for a faction. Members are their
// own ids, unprefixed — every knowledge fact is audienced to a member by its
// bare id, and the observer of a fold has to be the same word.
func factionEntityID(id FactionID) journal.EntityID { return journal.EntityID("faction:" + id) }

// encounterWorld is the run's world: the one journal (persisted) and the one
// graph (rebuilt), plus the indexes a sweep walks.
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

	// minds is each faction's mind AS THE GRAPH WAS DECLARED: the declared
	// one while it is a current member, else the faction's sole current
	// member, else nobody. What every Settle names as Of, and the observers a
	// stance folds as.
	minds map[FactionID]MemberID

	// observers is, per turnable pair, the minds its Settles were declared
	// for — the folds a stance question asks, sorted.
	observers map[factionPair][]MemberID
}

// newEncounterWorld is a world with an empty journal and no declaration yet
// — [Encounter.buildWorld] declares the graph once the field, the roster and
// the endings are known.
func newEncounterWorld() *encounterWorld { return &encounterWorld{log: journal.New()} }

// conceals reports whether the field hid anything — the question the
// projection asks before withholding, and the storage boundary asks before
// writing a world key for a field nobody has learned anything in.
func (w *encounterWorld) conceals() bool {
	return len(w.concealedDoors) > 0 || len(w.concealedRegions) > 0
}

// knowledgeFacts is every fact of a knowledge kind, in append order — the
// half of the one journal [EncounterData.World] carries.
func (w *encounterWorld) knowledgeFacts() []journal.Fact {
	var out []journal.Fact
	for _, f := range w.log.All() {
		if strings.HasPrefix(string(f.Kind), "known:") {
			out = append(out, f)
		}
	}
	return out
}

// buildWorld (re)declares the graph from the field and the roster, keeping
// the journal. Called at Setup and Load once the endings are known, and at
// Join and Exit — a member is an entity, and the mind of a faction of one is
// whoever is in it right now.
func (e *Encounter) buildWorld() error {
	w := e.world
	if w == nil {
		w = &encounterWorld{log: journal.New()}
		e.world = w
	}
	w.concealedRegions = make(map[RegionID]bool)
	w.doorRegions = make(map[DoorID][]RegionID)
	w.concealedDoors = nil
	w.minds = make(map[FactionID]MemberID)
	w.observers = make(map[factionPair][]MemberID)

	cfg := graph.Config{Membership: worldMembership}

	// Concealed structure, pierced per entity by its own minted kind — one
	// kind per entity, because a pierce fires on kind alone: a shared kind
	// would reveal every door on any door's find.
	for _, r := range e.field.regions {
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
	for _, d := range e.doors {
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
		for _, edge := range d.edges {
			for _, cell := range []spatial.Position{edge.From, edge.To} {
				if r, owned := e.field.regionOf(cell); owned && w.concealedRegions[r] && !seen[r] {
					seen[r] = true
					w.doorRegions[d.id] = append(w.doorRegions[d.id], r)
				}
			}
		}
		sort.Strings(w.doorRegions[d.id])
	}
	sort.Strings(w.concealedDoors)

	// The sides: every faction, allied with itself, and the declared or
	// default stance between every pair, one edge per direction.
	factions := e.field.factionIDs()
	for _, id := range factions {
		cfg.Entities = append(cfg.Entities, graph.Entity{ID: factionEntityID(id), Kind: "faction"})
		cfg.Edges = append(cfg.Edges, graph.Edge{From: factionEntityID(id), Rel: relAlliedWith, To: factionEntityID(id)})
	}
	for i, a := range factions {
		for _, b := range factions[i+1:] {
			var rel graph.Relation
			switch stance, _ := e.field.declaredStance(pairOf(a, b)); stance {
			case StanceHostile:
				rel = relHostileTo
			case StanceAllied:
				rel = relAlliedWith
			default:
				continue
			}
			cfg.Edges = append(cfg.Edges,
				graph.Edge{From: factionEntityID(a), Rel: rel, To: factionEntityID(b)},
				graph.Edge{From: factionEntityID(b), Rel: rel, To: factionEntityID(a)})
		}
	}

	// The members, each belonging to its faction. A world NPC belongs to
	// nobody: it is an entity so its own facts have somewhere to land, and
	// it is on no side.
	members := e.rosterIDs()
	byFaction := make(map[FactionID][]MemberID)
	for _, id := range members {
		cfg.Entities = append(cfg.Entities, graph.Entity{ID: journal.EntityID(id), Kind: "member", Grain: graph.GrainIndividual})
		if faction := factionOf(e.members[id]); faction != "" {
			cfg.Edges = append(cfg.Edges, graph.Edge{
				From: journal.EntityID(id), Rel: worldMembership, To: factionEntityID(faction),
			})
			byFaction[faction] = append(byFaction[faction], id)
		}
	}
	for _, id := range factions {
		w.minds[id] = e.field.mindOf(id, byFaction[id])
	}

	// The flip: per fact a disposition waits on, a Raise flagging the
	// learner; per such disposition, per mind of its pair, a Settle — while
	// that mind carries the flag, the pair holds no stance edge at all,
	// which is what neutral means (R2).
	raised := make(map[FactID]bool)
	for _, d := range e.field.dispositions {
		fact, ok := d.Until.(TriggerFact)
		if !ok {
			continue
		}
		if !raised[fact.Fact] {
			raised[fact.Fact] = true
			cfg.Reducers = append(cfg.Reducers, graph.Raise{On: factKnownKind(fact.Fact), Flag: knowsFlag(fact.Fact)})
		}
		pair := pairOf(d.Between[0], d.Between[1])
		for _, id := range []FactionID{pair.a, pair.b} {
			mind := w.minds[id]
			if mind == "" {
				continue
			}
			cfg.Projections = append(cfg.Projections, graph.Settle{
				OnFlag:    knowsFlag(fact.Fact),
				Of:        journal.EntityID(mind),
				Between:   [2]journal.EntityID{factionEntityID(pair.a), factionEntityID(pair.b)},
				Relations: []graph.Relation{relHostileTo, relAlliedWith},
				To:        "",
			})
			w.observers[pair] = append(w.observers[pair], mind)
		}
	}

	structure, err := graph.New(cfg)
	if err != nil {
		return fmt.Errorf("seed world: %w", err)
	}
	w.structure = structure
	return nil
}

// mindOf is a faction's mind, given who is in it right now: the DECLARED
// mind while it is one of them, else nobody. A faction of one has its member
// declared as its mind by the authoring compiler, never inferred here from
// whoever happens to be standing in the faction — so a declared mind that
// fell or left leaves the faction unable to learn (design §3.9, R7:
// accidental succession is still succession, and succession is a shelf).
func (f *field) mindOf(id FactionID, members []MemberID) MemberID {
	i, ok := f.factionIndex[id]
	if !ok || f.factions[i].Mind == "" {
		return ""
	}
	for _, m := range members {
		if m == f.factions[i].Mind {
			return m
		}
	}
	return ""
}

// factionIDs is every faction this field has, reserved first, then declared
// in authored order — the entity list the graph is seeded from.
func (f *field) factionIDs() []FactionID {
	out := make([]FactionID, 0, 2+len(f.factions))
	out = append(out, FactionParty, FactionMonsters)
	for _, fa := range f.factions {
		if fa.ID != FactionMonsters {
			out = append(out, fa.ID)
		}
	}
	return out
}

// mintedFactIDs is every fact id this run can mention, sorted: what a record
// reveals, what a disposition waits for, what an ending waits for — the
// `known:fact` kinds the trust boundary accepts at load.
func mintedFactIDs(f *field, endings []Trigger) []FactID {
	seen := make(map[FactID]bool)
	for _, rec := range f.intel {
		if rec.Reveals.Fact != "" {
			seen[rec.Reveals.Fact] = true
		}
	}
	for _, d := range f.dispositions {
		if t, ok := d.Until.(TriggerFact); ok {
			seen[t.Fact] = true
		}
	}
	for _, t := range endings {
		if t, ok := t.(TriggerFact); ok {
			seen[t.Fact] = true
		}
	}
	out := make([]FactID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
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

// knowsFact is THE ONE FOLD FOR KNOWLEDGE OF A FACT (design §3.3; no reader
// keeps a copy): a member knows a fact when the journal holds a
// `known:fact:<id>` fact whose SUBJECT is that member — the same shape the
// graph's Raise flags for a Settle, read directly so knowledge does not
// depend on which facts a disposition happens to wait for.
func (w *encounterWorld) knowsFact(member MemberID, id FactID) bool {
	kind := factKnownKind(id)
	for _, f := range w.log.All() {
		if f.Kind == kind && f.Subject == journal.EntityID(member) {
			return true
		}
	}
	return false
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

// stanceBetween is THE STANCE READER (design §3.2): what the graph says two
// factions are to each other right now, folded as the pair's minds.
//
// Hostile while EVERY mind's fold still holds the hostile edge — any one of
// them coming to know turns the pair; allied when a fold shows the allied
// edge; neutral otherwise. A pair no mind can turn folds as nobody, which is
// the declaration alone. Same faction twice is the allied-with-itself edge.
func (e *Encounter) stanceBetween(pair factionPair) Stance {
	a, b := factionEntityID(pair.a), factionEntityID(pair.b)
	observers := e.world.observers[pair]
	if len(observers) == 0 {
		observers = []MemberID{""}
	}
	hostile, allied := true, false
	for _, o := range observers {
		state := e.world.structure.StateFor(journal.EntityID(o), e.world.log)
		if !state.HasEdge(a, relHostileTo, b) {
			hostile = false
		}
		if state.HasEdge(a, relAlliedWith, b) {
			allied = true
		}
	}
	switch {
	case hostile:
		return StanceHostile
	case allied:
		return StanceAllied
	default:
		return StanceNeutral
	}
}

// opposed reports whether two members are on opposed sides: a hostile-to
// edge stands between their factions (design §3.2). A member in no faction —
// a world NPC — is opposed to nobody.
func (e *Encounter) opposed(a, b MemberID) bool {
	ma, ok := e.members[a]
	if !ok {
		return false
	}
	mb, ok := e.members[b]
	if !ok {
		return false
	}
	fa, fb := factionOf(ma), factionOf(mb)
	if fa == "" || fb == "" {
		return false
	}
	return e.stanceBetween(pairOf(fa, fb)) == StanceHostile
}

// Stance reports the stance between two factions right now — the fold, for
// a host or a test that wants the pair rather than two members.
//
// Errors: ErrNoFaction when either id names no faction this field has.
func (e *Encounter) Stance(a, b FactionID) (Stance, error) {
	for _, id := range []FactionID{a, b} {
		if !e.field.isFaction(id) {
			return "", fmt.Errorf("stance: %q: %w", id, ErrNoFaction)
		}
	}
	return e.stanceBetween(pairOf(a, b)), nil
}

// IsHostile answers whether b is an enemy of a — the read resolution's cast
// asks for Sneak Attack and Pack Tactics (design §4): a hostile-to edge
// between their factions, folded from this run's own world. known is false
// when either is not a member of this encounter: an effect asking about
// somebody who is not here has to be able to tell that apart from an answer.
func (e *Encounter) IsHostile(a, b MemberID) (hostile, known bool) {
	if _, ok := e.members[a]; !ok {
		return false, false
	}
	if _, ok := e.members[b]; !ok {
		return false, false
	}
	return e.opposed(a, b), true
}

// IsAllied answers whether b is on a's side — an allied-with edge between
// their factions, which a faction has with itself and with any faction a
// disposition declares it allied to. Not the negation of [Encounter.IsHostile]:
// two neutral factions are neither. known is false when either is not a
// member.
func (e *Encounter) IsAllied(a, b MemberID) (allied, known bool) {
	ma, ok := e.members[a]
	if !ok {
		return false, false
	}
	mb, ok := e.members[b]
	if !ok {
		return false, false
	}
	fa, fb := factionOf(ma), factionOf(mb)
	if fa == "" || fb == "" {
		return false, true
	}
	return e.stanceBetween(pairOf(fa, fb)) == StanceAllied, true
}

// turnablePairs is every pair whose stance a fact can change — the
// dispositions with an until — sorted, so a stance table folds in one order.
func (e *Encounter) turnablePairs() []factionPair {
	out := make([]factionPair, 0, len(e.field.dispositions))
	for _, d := range e.field.dispositions {
		if d.Until != nil {
			out = append(out, pairOf(d.Between[0], d.Between[1]))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].a != out[j].a {
			return out[i].a < out[j].a
		}
		return out[i].b < out[j].b
	})
	return out
}

// stanceTable folds every turnable pair — the before-and-after a flip is
// noticed by. Two folds around one append, never a copy kept between verbs.
func (e *Encounter) stanceTable() map[factionPair]Stance {
	out := make(map[factionPair]Stance)
	for _, pair := range e.turnablePairs() {
		out[pair] = e.stanceBetween(pair)
	}
	return out
}
