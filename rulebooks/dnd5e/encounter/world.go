// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
	"strings"

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
// every current member, and the concealments. Edges:
// `belongs-to` from each member to its faction, and the stance edges —
// `hostile-to` or `allied-with`, one per direction — for every pair of
// factions, declared or default (disposition.go). Reducers: a [graph.Raise]
// per fact id the field mentions, raising `knows:<fact>` on whoever a
// `known:fact:<fact>` fact is about, and one per turnable pair per stance,
// raising a settled flag on the pair. Projections: a [graph.Settle] per
// disposition with an `until: { fact }`, per mind of its pair, and two per
// turnable pair for the settled flags (design R11, Kirk: "the graph should
// tell the truth").
//
// # A pair turns two ways, on two grains
//
// A `fact` on an `until` is the MIND'S: the Settle names the mind as Of, so
// the camp knows what its chief knows and a scout reading the same letter
// changes nothing (R3). Every other way a pair turns is PUBLIC, and is a
// fact in this journal audienced to the two factions — a round the guards
// were waiting for, a scout falling, another pair turning, or somebody
// swinging across a neutral pair (rpg-project#493, R2 and R3). Public
// because none of those is anybody's private knowledge: they are the world's
// truth, and the pair is what witnessed them.
//
// The two settled Settles are declared NEUTRAL FIRST, HOSTILE SECOND, and
// that order is the betrayed truce: a pair its own `until` turned neutral is
// turned hostile again by an attack, because the later projection wins
// ([graph.Settle]'s own rule for overruling an earlier one).
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
// coming to know a fact — `known:fact:<id>`, beside `known:concealment:`.
// The fact's ACTOR and SUBJECT are both the learner, and its
// audience is the learner alone: the subject is what [graph.Raise] flags, the
// audience is whose fold carries it.
const factKnownPrefix = "known:fact:"

func factKnownKind(id FactID) journal.Kind { return journal.Kind(factKnownPrefix + id) }

// knowsFlag is the flag a `known:fact:<id>` fact raises on its learner.
func knowsFlag(id FactID) graph.Flag { return graph.Flag("knows:" + id) }

// settledPrefix and settledKind are the fact kind that records a pair's
// PUBLIC turn — `settled:<stance>:<a>|<b>`, beside `known:fact:` and
// `known:concealment:` in the one journal (rpg-project#493).
//
// THE KIND NAMES THE RESULT, NOT THE CAUSE. A round, a fall, another pair's
// stance and an attack all turn a pair the same way and are the same fact;
// which of them it was is [journal.Outcome.Detail], the transcript's own
// field, exactly as a pierce records why a member came to know a
// concealment. One kind per pair per stance, because a flag is raised by
// kind: a shared kind would settle every pair on any one of them turning.
const settledPrefix = "settled:"

func settledKind(to Stance, pair factionPair) journal.Kind {
	return journal.Kind(settledPrefix + string(to) + ":" + pair.a + "|" + pair.b)
}

// settledFlag is the flag a settled fact raises on the pair.
func settledFlag(to Stance, pair factionPair) graph.Flag {
	return graph.Flag(settledPrefix + string(to) + ":" + pair.a + "|" + pair.b)
}

// settledStances are the two stances a pair can be settled to publicly, IN
// PROJECTION ORDER — neutral first so hostile overrules it. Allied is not
// among them: nothing turns a pair allied, and nothing turns an allied pair.
var settledStances = []Stance{StanceNeutral, StanceHostile}

// settledRelation is the edge a pair holds while it is settled to a stance:
// the hostile edge, or none at all, which is what neutral means in a graph
// whose only stances are edges.
func settledRelation(to Stance) graph.Relation {
	if to == StanceHostile {
		return relHostileTo
	}
	return ""
}

// factionEntityID mints the graph entity for a faction. Members are their
// own ids, unprefixed — every knowledge fact is audienced to a member by its
// bare id, and the observer of a fold has to be the same word.
func factionEntityID(id FactionID) journal.EntityID { return journal.EntityID("faction:" + id) }

// encounterWorld is the run's world: the one journal (persisted) and the one
// graph (rebuilt), plus the indexes a sweep walks.
type encounterWorld struct {
	structure *graph.World
	log       *journal.Journal

	// concealments is every concealment's ID, sorted (C8 — the sweep walks
	// it, and beat order is observable).
	//
	// ONE LIST WHERE THERE WERE THREE. It replaced `concealedDoors`,
	// `concealedRegions` and the `doorRegions` index that tied them
	// together — the index existed only because finding a door and learning
	// the room behind it were two knowledge moments that had to be kept in
	// step. They are one moment now (rpg-project#490, R1), so the thing
	// that kept them in step has nothing left to do.
	concealments []ConcealmentID

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
func (w *encounterWorld) conceals() bool { return len(w.concealments) > 0 }

// knowledgeFacts is every fact the WORLD folds over, in append order — the
// half of the one journal [EncounterData.World] carries: what somebody came
// to know (`known:`) and what a pair publicly settled to (`settled:`).
//
// THE SETTLED FACTS PERSIST FOR THE SAME REASON THE KNOWN ONES DO. A pair's
// stance is derived, never stored, so the fact that turned it is the whole
// record; drop it from the blob and a camp the party provoked would reload
// civil, which is the fail-silent this slice exists to remove.
func (w *encounterWorld) knowledgeFacts() []journal.Fact {
	var out []journal.Fact
	for _, f := range w.log.All() {
		if strings.HasPrefix(string(f.Kind), "known:") || strings.HasPrefix(string(f.Kind), settledPrefix) {
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
	w.concealments = nil
	w.minds = make(map[FactionID]MemberID)
	w.observers = make(map[factionPair][]MemberID)

	cfg := graph.Config{Membership: worldMembership}

	// The concealments, pierced per entity by their own minted kind — one
	// kind per entity, because a pierce fires on kind alone: a shared kind
	// would give away every secret on any one of them being found.
	for i := range e.field.concealments {
		c := &e.field.concealments[i]
		w.concealments = append(w.concealments, c.id)
		cfg.Entities = append(cfg.Entities, graph.Entity{
			ID: concealmentEntityID(c.id), Kind: "concealment", Concealed: true,
		})
		cfg.Pierces = append(cfg.Pierces, graph.Pierce{
			On:       concealmentKnownKind(c.id),
			Entities: []journal.EntityID{concealmentEntityID(c.id)},
		})
	}
	sort.Strings(w.concealments)

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
	// that mind carries the flag, the pair holds the stance its declaration
	// TURNS TO (rpg-project#493, R1). Hostile-until-a-fact settles to no
	// edge at all, which is what neutral means; neutral-until-a-fact settles
	// to the hostile edge, which is the direction that did not exist before
	// this slice.
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
				To:        settledRelation(turnsTo(d.Stance)),
			})
			w.observers[pair] = append(w.observers[pair], mind)
		}
	}

	// The public turns: per turnable pair, per stance a pair can settle to,
	// a Raise flagging the PAIR and a Settle reading that flag. Declared
	// after every fact Settle and in [settledStances] order, so an attack
	// overrules a truce a fact already granted.
	//
	// DECLARED FOR PAIRS NOTHING HAS TURNED YET, which is what makes a turn
	// derivable at all: the graph is construction truth, rebuilt from the
	// field on every Setup, Load, Join and Exit, and only the journal says
	// whether a flag is up. A pair with no way to turn ([turnablePairsOf])
	// gets neither, so an allied pair carries no machinery it could never
	// use.
	for _, pair := range turnablePairsOf(e.field) {
		for _, to := range settledStances {
			cfg.Reducers = append(cfg.Reducers,
				graph.Raise{On: settledKind(to, pair), Flag: settledFlag(to, pair)})
			cfg.Projections = append(cfg.Projections, graph.Settle{
				OnFlag:    settledFlag(to, pair),
				Of:        factionEntityID(pair.a),
				Between:   [2]journal.EntityID{factionEntityID(pair.a), factionEntityID(pair.b)},
				Relations: []graph.Relation{relHostileTo, relAlliedWith},
				To:        settledRelation(to),
			})
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
// reveals, what a disposition waits for, what an ending waits for, and what a
// member teaches its witnesses when a threat against it lands — the
// `known:fact` kinds the trust boundary accepts at load.
//
// members is the roster and the reserve AS PERSISTED, because that is where a
// shenanigan's fact is authored (rpg-project#454): `on: { intimidated: { fact:
// … } }` on the sergeant's placement means this run can mint
// `known:fact:sergeant-cowed` even though no record reveals it and no
// disposition has to wait for it. Left out, a saved run in which the sergeant
// was cowed would refuse to load — its own fact would look like another
// dungeon's.
func mintedFactIDs(f *field, endings []Trigger, members []FactID) []FactID {
	seen := make(map[FactID]bool)
	for _, rec := range f.intel {
		if rec.Reveals.Fact != "" {
			seen[rec.Reveals.Fact] = true
		}
	}
	for _, id := range members {
		if id != "" {
			seen[id] = true
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

// knowsConcealment reports whether a member's own fold shows the
// concealment. A concealment this graph was never told about — every one, on
// a field that hides nothing — folds as visible, which is what
// [graph.State.Visible] answers for anything undeclared and is the honest
// answer here: there is no secret to not know.
func (w *encounterWorld) knowsConcealment(member MemberID, id ConcealmentID) bool {
	return w.knowledgeOf(member).Visible(concealmentEntityID(id))
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

// learnConcealment writes the fact that pierces one concealment for one
// member alone. cause is a human-readable trace
// ([journal.Outcome.Detail]) — the causes are exemplary, and the journal
// records which one it was.
func (w *encounterWorld) learnConcealment(member MemberID, id ConcealmentID, cause string) error {
	_, err := w.log.Append(journal.Fact{
		Kind:     concealmentKnownKind(id),
		Actor:    journal.EntityID(member),
		Subject:  concealmentEntityID(id),
		Audience: journal.Audience{journal.EntityID(member)},
		Outcome:  journal.Outcome{Detail: cause},
	})
	return err
}

// stanceBetween is THE STANCE READER (design §3.2): what the graph says two
// factions are to each other right now.
//
// # Two folds, and the first difference wins
//
// The PAIR'S OWN fold is the base — folded as the faction entity, which
// witnesses the declaration and the public turns audienced to it, and no
// member's private knowledge. Then each mind whose knowledge can turn the
// pair is folded in turn, and THE FIRST ONE THAT DISAGREES WITH THE BASE IS
// THE ANSWER: one mind learning is enough, and it turns the pair for
// everyone, because a pair is a pair (rpg-project#493, R1). A scout reading
// the letter is not a mind and changes nothing (R3).
//
// THIS REPLACED "HOSTILE WHILE EVERY MIND STILL HOLDS THE EDGE", which said
// the same thing in one direction and the opposite in the other. Under it a
// neutral-declared pair whose chief learned the fact would have folded
// neutral — the second mind's fold still shows no edge, so "every" is never
// met — and the camp the author wrote as turnable would never turn. The rule
// was never about the hostile edge; it was about a mind disagreeing with the
// declaration, which is what this asks directly.
//
// A pair no mind can turn folds as the pair alone. Same faction twice is the
// allied-with-itself edge.
func (e *Encounter) stanceBetween(pair factionPair) Stance {
	base := e.stanceAs(factionEntityID(pair.a), pair)
	for _, mind := range e.world.observers[pair] {
		if s := e.stanceAs(journal.EntityID(mind), pair); s != base {
			return s
		}
	}
	return base
}

// stanceAs folds one observer's present and reads the pair's stance out of
// it: the hostile edge, the allied edge, or neither, which is neutral.
func (e *Encounter) stanceAs(observer journal.EntityID, pair factionPair) Stance {
	a, b := factionEntityID(pair.a), factionEntityID(pair.b)
	state := e.world.structure.StateFor(observer, e.world.log)
	switch {
	case state.HasEdge(a, relHostileTo, b):
		return StanceHostile
	case state.HasEdge(a, relAlliedWith, b):
		return StanceAllied
	default:
		return StanceNeutral
	}
}

// settlePair writes the PUBLIC fact that a pair turned, audienced to the two
// factions — which is every fold that asks about the pair: its own, and its
// minds', who belong to it (design §6: a stance is truth, the same for
// everybody). cause is the transcript's own word for why, and it is what
// carries "attacked by <actor>" into the journal (R3).
//
// ACTOR AND SUBJECT ARE BOTH THE PAIR'S FIRST SIDE — the entity the Settle
// names as Of. A pair turning has no one actor: the attacker is a cause, not
// a doer of this, and two factions coming to blows have two. Idempotent by
// construction — a flag already up cannot go further up — but asked anyway
// by the callers, so a second turn writes no second fact.
func (w *encounterWorld) settlePair(pair factionPair, to Stance, cause string) error {
	side := factionEntityID(pair.a)
	_, err := w.log.Append(journal.Fact{
		Kind:     settledKind(to, pair),
		Actor:    side,
		Subject:  side,
		Audience: journal.Audience{factionEntityID(pair.a), factionEntityID(pair.b)},
		Outcome:  journal.Outcome{Detail: cause},
	})
	if err != nil {
		return fmt.Errorf("settle %s to %s: %w", pair, to, err)
	}
	return nil
}

// settled reports whether a pair has already been settled to a stance
// publicly — the one question a turn asks before writing, so an `until` that
// keeps holding round after round turns its pair once.
func (w *encounterWorld) settled(pair factionPair, to Stance) bool {
	_, ok := w.settledCause(pair, to)
	return ok
}

// settledCause is the reason a pair was settled to a stance publicly, and
// whether it was — the FIRST such fact, because the first one is the one that
// turned it and everything after is noise a later verb could not append
// anyway ([encounterWorld.settled] is asked before every write).
func (w *encounterWorld) settledCause(pair factionPair, to Stance) (string, bool) {
	kind := settledKind(to, pair)
	for _, f := range w.log.All() {
		if f.Kind == kind {
			return f.Outcome.Detail, true
		}
	}
	return "", false
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

// BelievedStance answers what one member BELIEVES about another's side —
// the fact a per-viewer sighting carries next to a creature's name and kind,
// and the one a ring under a token is coloured from (rpg-project#458, "The
// ring: what a player believes about a creature").
//
// # Today it is the truth, and that is the honest answer
//
// With no deception in play, every viewer believes what the creature shows,
// and what it shows is the derived stance between their factions — so this
// returns exactly what [Encounter.IsHostile] and [Encounter.IsAllied] fold,
// in one word instead of two booleans. Every viewer gets the same answer, and
// the existing faction ring does not change colour.
//
// # It exists per viewer BEFORE anything can lie, deliberately
//
// This is the perception law applied to stance. The proto's own doc on
// sightings says the server must not state a fact a viewer's stale view could
// be wrong about, "because the fact is exactly what an illusion has to be able
// to lie about" — and a stance is such a fact. A stance read live off the
// graph can only ever be true, and a game with no way to lie can never have
// illusion in it (rpg-project/CLAUDE.md's own worked example).
//
// So THIS IS THE ONE PLACE `pretend` WILL MAKE BELIEF AND TRUTH DIVERGE: a
// creature showing one stance and holding another changes what this function
// answers for a viewer whose Insight did not beat its Deception, and changes
// nothing else anywhere. Nothing today reads a per-viewer answer out of a
// shared one, which is what makes that a later slice's edit rather than a
// later slice's rewrite.
//
// # known
//
// False when there is no PAIR to have a stance about, which is two cases and
// they are the same case: an id that is not a member of this encounter, and a
// member in NO FACTION, which a world NPC is.
//
// THIS USED TO ANSWER NEUTRAL FOR A WORLD NPC, and that was wrong. "Nobody is
// against them" and "there is no side here to be on" are different statements,
// and reporting the second as [StanceNeutral] collapsed an ABSENCE into an
// ANSWER — the defect this repo's own zero-value rule exists to prevent. A
// client drawing a ring off this would have painted a vendor the same colour
// as a goblin the party had declared a truce with, and had no way to tell.
// [Encounter.IsAllied] reports (false, true) for the same pair, and that is a
// different question: "are they on my side" has a correct false answer, while
// "what is their stance" has none.
func (e *Encounter) BelievedStance(viewer, subject MemberID) (Stance, bool) {
	mv, ok := e.members[viewer]
	if !ok {
		return "", false
	}
	ms, ok := e.members[subject]
	if !ok {
		return "", false
	}
	fv, fs := factionOf(mv), factionOf(ms)
	if fv == "" || fs == "" {
		return "", false
	}

	return e.stanceBetween(pairOf(fv, fs)), true
}

// turnablePairs is every pair whose stance this run can change, sorted, so a
// stance table folds in one order.
func (e *Encounter) turnablePairs() []factionPair { return turnablePairsOf(e.field) }

// turnablePairsOf is the same list read off a field alone — [Encounter.buildWorld]
// needs it before there is an encounter to ask.
//
// TWO WAYS IN, and between them they are the whole set (rpg-project#493):
//
//   - a pair with an `until`, either direction — hostile to neutral, or
//     neutral to hostile;
//   - EVERY NEUTRAL PAIR, authored or default, because the aggression law
//     turns one hostile with nothing authored at all (R3).
//
// What is left out is exactly what cannot move: an allied pair, which takes
// no until and is not turned by an attack, and a hostile pair with no until,
// which has nothing to turn it — an attack across a pair already hostile
// changes nothing.
func turnablePairsOf(f *field) []factionPair {
	ids := f.factionIDs()
	out := make([]factionPair, 0, len(ids))
	for i, a := range ids {
		for _, b := range ids[i+1:] {
			pair := pairOf(a, b)
			switch declared, until := f.declaredStance(pair); {
			case declared == StanceAllied:
			case until != nil, declared == StanceNeutral:
				out = append(out, pair)
			}
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
