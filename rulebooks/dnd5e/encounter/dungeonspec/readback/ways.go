// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// ways.go says the WAYS through each join (rpg-project#981, design R30): one
// statement per means through, naming its method, what it needs and what
// failing does, each said right after its join.
//
// BARS (the lead's ruling on rpg-toolkit#2443): a join is barred by zero or
// more bars — `lock` (a door's lock), `hold` (posted holders), `hidden` (a
// secret on its door or on the prop that carries it) — and a walker gets
// through when EVERY bar is lifted. A way lifts exactly one bar and says so
// as {bar, <bar>, lifts}; a consumer groups a join's ways by bar and reads
// "any way past the lock AND any way past the holders". A join with no bar
// has one `place.way.free` with no bar; a bar nothing lifts is said as
// `place.way.none`. A passage's carrier (the line that relocates, the line
// that steps a conversation into its jail) is how the passage is taken once
// its bars are lifted, not a bar's alternative, and carries no bar.
//
// THE HOLD IS THE ENGINE'S PASS, PER HOLDER (post.go `passLets`): a holder
// lets a walker through only when EVERY requirement of its post's pass is
// met, and the door stays held while ANY posted holder bars the walker. So a
// line lifts the hold only when, for every holder, its grant reaches that
// holder (said to it, or heard as a carrier of the conversation when the
// grant spreads to carriers who witness it) and meets that holder's whole
// pass — the pass's facts and props named as needs — or it teaches a fact
// that holder's routine leaves its posts on. A line that lifts some holders
// and not all is `place.way.partial`, which a count of ways ignores by kind.
//
// THE CHAIN IS SHORT AND THE ENGINE'S OWN: join → its bars → what lifts each
// bar → that thing's own check and requirements. A need is named, never
// followed further. Nothing here simulates play — whose words reach whom,
// who witnesses — and every sentence says where play decides.
//
// COUNTABLE WITHOUT THE SENTENCE: every way connects to its join
// (`through`), to the bar it lifts (`lifts`), to what carries it (`by`), to
// each need (`needs`), to what a miss grants or steps (`on-miss`), to the
// holders a partial way leaves holding (`still-holds`), and to its method as
// {Kind: "method", ID: "skill:<name>" | "ability:<name>" | "tool:<ref>" |
// "knowledge" | "item" | "fight" | "free"}. A check with alternatives
// carries one `by` method per alternative, each with its own DC as the
// connection's Value; a part of the same approach (its tool) follows it
// with role `with` and the same DC;
// free, knowledge, item and fight carry none. A pass's `not` is a condition,
// connected `unless`.

// wayCtx is what the ways of one read-back read: every statement said so
// far, by path, for the facets of the lines a way is carried by.
type wayCtx struct {
	byPath map[string][]Statement
	all    []Statement
}

func newWayCtx(lines []Statement) *wayCtx {
	c := &wayCtx{byPath: map[string][]Statement{}, all: lines}
	for _, st := range lines {
		c.byPath[st.Path] = append(c.byPath[st.Path], st)
	}
	return c
}

// under is every statement at path or beneath it.
func (c *wayCtx) under(path string) []Statement {
	var out []Statement
	for _, st := range c.all {
		if st.Path == path || strings.HasPrefix(st.Path, path+".") {
			out = append(out, st)
		}
	}
	return out
}

// The bars a join can carry.
const (
	barLock   = "lock"
	barHold   = "hold"
	barHidden = "hidden"
)

// wayLineID is a line's id in a way's connections: its carrier's id and its
// own, so the same line on two creatures is two ids.
func wayLineID(carrier, option string) string { return carrier + "/" + option }

// wayStmt makes a way statement that lifts bar ("" for none). A way is not a hub entry: the needs it
// names are the carrier's own requirements, already in the hubs, so it is
// built without [builder.fstmt]'s hub registration.
func wayStmt(kind Kind, join Statement, bar, slug, path, sentence string, conns ...Connection) Statement {
	conns = append([]Connection{{Kind: "join", ID: join.About.ID, Role: "through"}}, conns...)
	if bar != "" {
		conns = append(conns[:1], append([]Connection{{Kind: "bar", ID: bar, Role: "lifts"}}, conns[1:]...)...)
	}
	return Statement{Kind: kind, About: Subject{Kind: "way", ID: "way:" + join.About.ID + ":" + slug},
		Sentence: sentence, Path: path, Connects: dedupeKeepingMethods(conns)}
}

// wayKind is whether a statement says a way.
func wayKind(k Kind) bool { return strings.HasPrefix(string(k), "place.way.") }

// methodsOf is a check's methods (rpg-toolkit#2444): each alternative's
// rolled ability or skill with role `by`, then each further part of that
// same approach (its tool) with role `with`, immediately after it. A
// consumer lists a way's alternatives as its `by` methods and counts each
// approach once; every connection of an approach carries the approach's DC.
func methodsOf(approaches []encounter.CheckApproach) []Connection {
	var out []Connection
	for _, a := range approaches {
		kind := "skill:"
		switch a.Ability {
		case "str", "dex", "con", "int", "wis", "cha":
			kind = "ability:"
		}
		out = append(out, Connection{Kind: "method", ID: kind + a.Ability, Role: "by", Value: a.DC})
		if a.Tool != "" {
			out = append(out, Connection{Kind: "method", ID: "tool:" + a.Tool, Role: "with", Value: a.DC})
		}
	}
	return out
}

// dedupeKeepingMethods is [dedupe] for a way: its method connections are
// kept as built, in order, because a part (`with`) belongs to the `by` just
// before it, and two approaches may share an identical part (the same tool
// at the same DC) that must stay under each.
func dedupeKeepingMethods(conns []Connection) []Connection {
	seen := map[Connection]bool{}
	out := make([]Connection, 0, len(conns))
	for _, c := range conns {
		if c.Kind == "method" || !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// method is one method connection.
func method(id string) Connection { return Connection{Kind: "method", ID: id, Role: "by"} }

// needsOf is what a line's requirements need, positively: a fact known, a
// topic the creature spoken to holds about the speaker, a prop carried, a
// conversation's state. A requirement that something NOT hold is a bar,
// said in the sentence, not a need.
func (b *builder) needsOf(requires []encounter.Trigger) []Connection {
	var out []Connection
	for _, t := range requires {
		switch v := t.(type) {
		case encounter.TriggerFact:
			out = append(out, Connection{Kind: "fact", ID: string(v.Fact), Role: "needs"})
		case encounter.TriggerRecognized:
			out = append(out, Connection{Kind: "topic", ID: string(v.Topic), Role: "needs"})
		case encounter.TriggerHolds:
			out = append(out, Connection{Kind: "prop", ID: b.unqualify(string(v.Prop)), Role: "needs"})
		case encounter.TriggerInteraction:
			out = append(out, Connection{Kind: "conversation", ID: string(v.Interaction), Role: "needs"})
		}
	}
	return out
}

// lineFacets is a line's check, offered-while and miss words, as its own
// statements say them in the fold, with the short name put in, and what a
// miss grants or steps.
type lineFacets struct {
	check, offered, miss string
	onMiss               []Connection
}

func (c *wayCtx) facets(path, short string) lineFacets {
	var f lineFacets
	var reqs, miss []string
	for _, st := range c.under(path) {
		switch {
		case st.Kind == KindLineCheck && st.Path == path+".check":
			f.check = selfIn(st.fold.text, short)
		case st.Kind == KindLineRequire:
			reqs = append(reqs, st.fold.text)
		case st.Kind == KindLineAdvance && st.fold.miss != "":
			miss = append(miss, "the line "+st.fold.miss)
			for _, cn := range st.Connects {
				if cn.Kind == "conversation" {
					f.onMiss = append(f.onMiss, Connection{Kind: cn.Kind, ID: cn.ID, Role: "on-miss"})
				}
			}
		case (st.Kind == KindLineRow || st.Kind == KindLineGrant) && st.fold.verdict == "miss":
			miss = append(miss, st.fold.text)
			for _, cn := range st.Connects {
				if (cn.Kind == "topic" && cn.Role == "grants") || (cn.Kind == "fact" && cn.Role == "teaches") {
					f.onMiss = append(f.onMiss, Connection{Kind: cn.Kind, ID: cn.ID, Role: "on-miss"})
				}
			}
		}
	}
	if len(reqs) > 0 {
		f.offered = selfIn("offered while "+strings.Join(reqs, " and "), short)
	}
	if len(miss) > 0 {
		f.miss = selfIn(strings.Join(miss, "; "), short)
	}
	return f
}

// selfIn puts a short name in for a fold text's placeholders, mid-sentence.
func selfIn(text, name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "{Self}", capitalize(name)), "{self}", name)
}

// lineHow is the opening of a line way's sentence after its subject: what
// it rolls (or that it rolls nothing), what it needs, and its miss.
func lineHow(o encounter.Option, f lineFacets) (lead, tail string) {
	lead = "it rolls nothing"
	if o.Checked() {
		lead = "it rolls " + checkWords(o.Check)
	}
	if f.offered != "" {
		lead += "; it is " + f.offered
	}
	switch {
	case f.miss != "":
		tail = " On a miss: " + f.miss + "."
	case o.Checked():
		tail = " A miss does nothing more than fail."
	}
	return lead, tail
}

// waysFor says every way through one join: for each bar, what lifts it;
// for a passage, the line that carries it; else that nothing bars it.
func (b *builder) waysFor(join Statement, ctx *wayCtx, joinWords string) []Statement {
	carrier := func(kind string) string {
		for _, c := range join.Connects {
			if c.Kind == kind && c.Role == "by" {
				return c.ID
			}
		}
		return ""
	}
	var out []Statement
	switch join.Kind {
	case KindJoinDoor:
		door := carrier("door")
		bars := b.doorBars(door)
		out = append(out, b.secretWays(join, joinWords, "door", door, bars)...)
		out = append(out, b.lockWays(join, joinWords, door, bars)...)
		out = append(out, b.holdWays(join, joinWords, door, bars, ctx)...)
		if len(bars) == 0 {
			out = append(out, wayStmt(KindWayFree, join, "", "free", join.Path, fmt.Sprintf(
				"Nothing bars %s: the door %s is neither locked, held nor hidden, so a member opens it and walks through.",
				joinWords, quote(door)), Connection{Kind: "door", ID: door, Role: "by"}, method("free")))
		}
	case KindJoinOpen:
		out = append(out, wayStmt(KindWayFree, join, "", "free", join.Path, fmt.Sprintf(
			"Nothing bars %s: a member walks from one to the other.", joinWords), method("free")))
	case KindJoinPassage:
		if conv := carrier("conversation"); conv != "" {
			out = append(out, b.jailWays(join, joinWords, conv, ctx)...)
			break
		}
		for _, kind := range []string{"prop", "creature", "faction"} {
			if id := carrier(kind); id != "" {
				var bars []string
				if b.concealmentOf(kind, id) != nil {
					bars = []string{barHidden}
				}
				out = append(out, b.secretWays(join, joinWords, kind, id, bars)...)
				out = append(out, b.relocateWay(join, joinWords, kind, id, ctx))
			}
		}
	}
	// A bar nothing lifts is said, never skipped.
	for _, bar := range []string{barHidden, barLock, barHold} {
		barred, lifted := false, false
		if join.Kind == KindJoinDoor {
			barred = contains(b.doorBars(carrier("door")), bar)
		}
		for _, w := range out {
			if w.Kind != KindWayPartial && connected0(w, "bar", bar, "lifts") {
				lifted = true
			}
		}
		if barred && !lifted {
			out = append(out, Statement{Kind: KindWayNone, About: Subject{Kind: "bar", ID: "bar:" + join.About.ID + ":" + bar},
				Sentence: fmt.Sprintf("Nothing the file authors lifts the %s bar of %s: no way past it is said.", bar, joinWords),
				Path:     join.Path, Connects: []Connection{{Kind: "join", ID: join.About.ID, Role: "through"},
					{Kind: "bar", ID: bar, Role: "unlifted"}}})
		}
	}
	return out
}

// connected0 is whether a statement carries the connection.
func connected0(st Statement, kind, id, role string) bool {
	for _, c := range st.Connects {
		if c.Kind == kind && c.ID == id && c.Role == role {
			return true
		}
	}
	return false
}

// doorBars is what bars a door's join, in the order said: hidden, locked,
// held.
func (b *builder) doorBars(door string) []string {
	var out []string
	if b.concealmentOf("door", door) != nil {
		out = append(out, barHidden)
	}
	if d := b.doorInput(door); d != nil && d.State != nil {
		if _, locked := d.State.Lock(); locked {
			out = append(out, barLock)
		}
	}
	if len(b.doorHolders[door]) > 0 {
		out = append(out, barHold)
	}
	return out
}

// barWords names a bar for a sentence.
func barWords(bar string) string {
	switch bar {
	case barLock:
		return "its lock"
	case barHold:
		return "its holders"
	}
	return "its secret"
}

// alsoBarred is the clause a way says when its join has other bars: lifting
// this one does not pass them.
func alsoBarred(bars []string, this string) string {
	var others []string
	for _, b := range bars {
		if b != this {
			others = append(others, barWords(b))
		}
	}
	if len(others) == 0 {
		return ""
	}
	return fmt.Sprintf(" The join is also barred by %s, each to be passed as well: lifting this bar alone does not get a "+
		"walker through.", joinAnd(others))
}

// concealmentOf is the secret hiding a door or a prop, or nil.
func (b *builder) concealmentOf(kind, id string) *encounter.ConcealmentInput {
	for i := range b.c.Concealments {
		c := &b.c.Concealments[i]
		if kind == "door" {
			for _, d := range c.Doors {
				if b.unqualify(string(d)) == id {
					return c
				}
			}
		}
		if kind == "prop" {
			for _, p := range c.Props {
				if b.unqualify(string(p)) == id {
					return c
				}
			}
		}
	}
	return nil
}

// secretWays are the ways to KNOW a hidden door or prop, lifting its
// `hidden` bar: a search, and each record that reveals the secret. Seeing
// someone use it also reveals it, which is play, not a way the file offers.
func (b *builder) secretWays(join Statement, joinWords, kind, id string, bars []string) []Statement {
	c := b.concealmentOf(kind, id)
	if c == nil {
		return nil
	}
	cid := b.unqualify(string(c.ID))
	what := fmt.Sprintf("the %s %s", kind, quote(id))
	also := alsoBarred(bars, barHidden)
	out := []Statement{wayStmt(KindWaySearch, join, barHidden, "search:"+cid, "concealments."+cid, fmt.Sprintf(
		"A search with %s finds the secret %s, which hides %s, lifting the hidden bar of %s: until a member knows the "+
			"secret, %s is not shown to them. A miss finds nothing.%s", checkWords(c.Checks), quote(cid), what, joinWords, what, also),
		append([]Connection{{Kind: "concealment", ID: cid, Role: "by"}}, methodsOf(c.Checks)...)...)}
	for i, in := range b.c.Intel {
		if b.unqualify(string(in.Reveals.Concealment)) != cid {
			continue
		}
		rid := b.unqualify(string(in.ID))
		out = append(out, wayStmt(KindWayTold, join, barHidden, "told:"+rid, fmt.Sprintf("intel[%d]", i), fmt.Sprintf(
			"Holding the record %s reveals the secret %s, lifting the hidden bar of %s: whoever comes to hold the record — "+
				"told it, or taking what carries it — knows %s. Who tells it or carries it is said with the record.%s",
			quote(rid), quote(cid), joinWords, what, also),
			Connection{Kind: "record", ID: rid, Role: "by"}, Connection{Kind: "concealment", ID: cid, Role: "by"}, method("knowledge")))
	}
	return out
}

// doorInput is the compiled door with id.
func (b *builder) doorInput(id string) *encounter.DoorInput {
	for i := range b.c.Field.Doors {
		if b.unqualify(string(b.c.Field.Doors[i].ID)) == id {
			return &b.c.Field.Doors[i]
		}
	}
	return nil
}

// lockWays is one way per approach of a locked door's lock, each lifting
// its `lock` bar.
func (b *builder) lockWays(join Statement, joinWords, door string, bars []string) []Statement {
	d := b.doorInput(door)
	if d == nil || d.State == nil {
		return nil
	}
	lock, locked := d.State.Lock()
	if !locked {
		return nil
	}
	var out []Statement
	for i, a := range lock.Approaches {
		slug := "lock:" + a.Ability
		if a.Tool != "" {
			slug += "+" + a.Tool
		}
		out = append(out, wayStmt(KindWayLock, join, barLock, slug, "room.room.doorBindings."+door, fmt.Sprintf(
			"The door %s starts locked; beating its lock with %s opens it, lifting the lock bar of %s. A miss leaves the "+
				"door locked.%s", quote(door), checkWords([]encounter.CheckApproach{a}), joinWords, alsoBarred(bars, barLock)),
			append([]Connection{{Kind: "door", ID: door, Role: "by"}}, methodsOf(lock.Approaches[i:i+1])...)...))
	}
	return out
}

// holder is one creature posted on a door: the states its post holds the
// door in, each such state's pass, and its recipe.
type holder struct {
	id     string
	states []string
	passes [][]encounter.Trigger
	recipe *machine.Recipe
}

// holdersOf is every creature posted on a door, in the order the file
// places them.
func (b *builder) holdersOf(door string) []holder {
	var out []holder
	for _, id := range b.doorHolders[door] {
		m := b.monsters[id]
		h := holder{id: id}
		for _, state := range sortedKeys(m.Posts) {
			p := m.Posts[state]
			if b.unqualify(string(p.Door)) != door {
				continue
			}
			h.states = append(h.states, state)
			h.passes = append(h.passes, p.Pass)
		}
		h.recipe = postRecipe(m.Machine, m.Behaviors, h.states)
		out = append(out, h)
	}
	return out
}

// postRecipe is the recipe whose states the posts are keyed by: the
// creature's machine, else the routine whose states hold them all.
func postRecipe(m *machine.Recipe, bh *encounter.BehaviorInput, states []string) *machine.Recipe {
	has := func(r *machine.Recipe) bool {
		for _, s := range states {
			if _, ok := r.States[s]; !ok {
				return false
			}
		}
		return true
	}
	if m != nil && has(m) {
		return m
	}
	if bh != nil {
		for i := range bh.Instances {
			if r := &bh.Instances[i].Instance.Recipe; has(r) {
				return r
			}
		}
	}
	return nil
}

// leavesOn is whether a holder's routine leaves every state it holds the
// door in when it comes to know the fact: a transition guarded by that fact
// alone being true, to a state that holds nothing on the door.
func (h holder) leavesOn(fact string) bool {
	if h.recipe == nil || len(h.states) == 0 {
		return false
	}
	for _, s := range h.states {
		left := false
		for _, tr := range h.recipe.States[s].Transitions {
			if tr.To == "" || contains(h.states, tr.To) || tr.When == nil || tr.When.Fact == nil {
				continue
			}
			leaf := tr.When.Fact
			if strings.TrimPrefix(string(leaf.Name), "fact:") == fact && leaf.Op == "eq" && leaf.Value == true {
				left = true
			}
		}
		if !left {
			return false
		}
	}
	return true
}

// grant is one recognition a line's win (or saying) makes: the topic, whom
// it is about, the conversation whose carriers may hear it, whether it
// spreads to them, and how it is made.
type grant struct {
	topic, about, conv, how string
	heard                   bool
	// aboutWord is whom it is about as the who-words say it
	// ([encounter.WhoWords]), for its note edges.
	aboutWord string
}

// grantsOf is every recognition a line's win (or saying) makes: its own
// rows' and its conversation's outcome on the state its advance steps to.
func (b *builder) grantsOf(o encounter.Option) []grant {
	var out []grant
	conv, _ := optionInteraction(o)
	for _, g := range o.Grants {
		if g.Failure || g.Consequence.Recognize == nil {
			continue
		}
		r := g.Consequence.Recognize
		out = append(out, grant{topic: string(r.Topic), about: shortSubjects(r.About), conv: conv, aboutWord: string(r.About),
			heard: r.HeardBy != "", how: fmt.Sprintf("its %s", winWord(o))})
	}
	if o.Advance != nil && o.Advance.Success != "" {
		c := string(o.Advance.Interaction)
		for _, in := range b.c.Field.Interactions {
			if string(in.ID) != c {
				continue
			}
			for _, state := range targetsOf(in.Recipe, o.Advance.Success) {
				for _, cq := range in.Outcomes[state] {
					if cq.Recognize == nil {
						continue
					}
					out = append(out, grant{topic: string(cq.Recognize.Topic), about: shortSubjects(cq.Recognize.About), conv: c,
						heard: cq.Recognize.HeardBy != "", aboutWord: string(cq.Recognize.About),
						how: fmt.Sprintf("its %s steps %s to %s, whose outcome", winWord(o), quote(c), quote(state))})
				}
			}
		}
	}
	return out
}

// reaches is whether a grant made by a line said to member comes to be held
// by the holder: the holder is the one it is said to, or the grant spreads
// to the conversation's carriers who witness it and the holder carries it.
func (b *builder) reaches(g grant, member, h string) bool {
	return member == h || (g.heard && g.conv != "" && contains(b.carriers[g.conv], h))
}

// passJudge is the engine's pass judgment ([encounter.Encounter.passLets])
// for one holder, every state it holds the door in: an empty pass lets
// nobody but allies through; a recognition is met only by a topic the holder
// notes (held); a fact or a prop is met by what the walker knows or
// carries (known), and is otherwise a need; a `not` holds for a walker who
// lacks what it negates, so it is a condition said with the way. recognised
// is whether every pass asks a recognition: a grant lifts only such a pass.
func passJudge(h holder, held []string, known map[string]bool) (met, recognised bool, needs, nots []encounter.Trigger) {
	if len(h.passes) == 0 {
		return false, false, nil, nil
	}
	recognised = true
	for _, pass := range h.passes {
		if len(pass) == 0 {
			return false, false, nil, nil
		}
		has := false
		for _, t := range pass {
			switch v := t.(type) {
			case encounter.TriggerRecognized:
				if !contains(held, string(v.Topic)) {
					return false, false, nil, nil
				}
				has = true
			case encounter.TriggerFact:
				if !known["fact:"+string(v.Fact)] {
					needs = append(needs, t)
				}
			case encounter.TriggerHolds:
				if !known["prop:"+string(v.Prop)] {
					needs = append(needs, t)
				}
			case encounter.TriggerNot:
				nots = append(nots, v.Inner)
			}
		}
		recognised = recognised && has
	}
	return true, recognised, needs, nots
}

// unlessWords says the `not` conditions of a pass as a clause, with their
// connections: the walker is let through only while lacking each.
func (b *builder) unlessWords(nots []encounter.Trigger) (string, []Connection) {
	var parts []string
	var cs []Connection
	seen := map[string]bool{}
	for _, t := range nots {
		if k := fmt.Sprintf("%#v", t); seen[k] {
			continue
		} else {
			seen[k] = true
		}
		switch v := t.(type) {
		case encounter.TriggerFact:
			parts = append(parts, "knows the fact "+quote(string(v.Fact)))
			cs = append(cs, Connection{Kind: "fact", ID: string(v.Fact), Role: "unless"})
		case encounter.TriggerHolds:
			p := b.unqualify(string(v.Prop))
			parts = append(parts, "carries the prop "+quote(p))
			cs = append(cs, Connection{Kind: "prop", ID: p, Role: "unless"})
		case encounter.TriggerRecognized:
			parts = append(parts, "has "+quote(string(v.Topic))+" noted about them by the holder")
			cs = append(cs, Connection{Kind: "topic", ID: string(v.Topic), Role: "unless"})
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "; a walker who " + joinOr(parts) + " is still barred", cs
}

// holdWays are the ways past a door's holders, each lifting its `hold`
// bar: every line that lifts every holder (by meeting its pass, or drawing
// it off its post), every knowing or carrying that meets every holder's
// pass, and the fight; what lifts some holders and not all is partial.
func (b *builder) holdWays(join Statement, joinWords, door string, bars []string, ctx *wayCtx) []Statement {
	holders := b.holdersOf(door)
	if len(holders) == 0 {
		return nil
	}
	var ids []string
	for _, h := range holders {
		ids = append(ids, h.id)
	}
	also := alsoBarred(bars, barHold)
	var out []Statement
	for _, site := range b.lineSites() {
		if w, ok := b.lineHoldWay(join, joinWords, site, holders, ids, also, ctx); ok {
			out = append(out, w)
		}
	}
	out = append(out, b.knowWays(join, joinWords, holders, also)...)
	var fc []Connection
	for _, id := range ids {
		fc = append(fc, Connection{Kind: "creature", ID: id, Role: "by"})
	}
	fc = append(fc, method("fight"))
	out = append(out, wayStmt(KindWayFight, join, barHold, "fight", "room.room.monsterBindings."+holders[0].id+".posts", fmt.Sprintf(
		"The fight lifts the holders' bar of %s: %s each hold the door only while posted, up and standing on a cell of the "+
			"post; one who is down, or drawn off the post, holds nothing. Failing it is losing the fight.%s",
		joinWords, b.creatures(ids), also), fc...))
	return out
}

// lineHoldWay says one line as a way past the holders, or partial, or
// nothing when it lifts no holder.
func (b *builder) lineHoldWay(join Statement, joinWords string, site lineSite, holders []holder, ids []string,
	also string, ctx *wayCtx) (Statement, bool) {
	o := site.option
	grants := b.grantsOf(o)
	short := b.shortName(site.member)
	var lifted, still []string
	var cs []Connection
	var why []string
	var passNeeds, passNots []encounter.Trigger
	viaHeard := false
	teach := map[string]bool{}
	for _, h := range holders {
		var held []string
		for _, g := range grants {
			if b.reaches(g, site.member, h.id) {
				held = append(held, g.topic)
				if site.member != h.id {
					viaHeard = true
				}
			}
		}
		met, recognised, needs, nots := passJudge(h, held, nil)
		met = met && recognised
		drawn := ""
		for _, r := range o.Success {
			if r.Fact != "" && h.leavesOn(r.Fact) {
				drawn = r.Fact
			}
		}
		switch {
		case met:
			lifted = append(lifted, h.id)
			passNeeds = append(passNeeds, needs...)
			passNots = append(passNots, nots...)
		case drawn != "":
			lifted = append(lifted, h.id)
			teach[drawn] = true
			cs = append(cs, Connection{Kind: "creature", ID: h.id, Role: "draws-off"})
		default:
			still = append(still, h.id)
		}
	}
	if len(lifted) == 0 {
		return Statement{}, false
	}
	for _, g := range grants {
		s := fmt.Sprintf("%s makes the creature it is said to note %s about %s", g.how, quote(g.topic), g.about)
		if g.heard {
			s += fmt.Sprintf(", and the conversation %s's other carriers who witness it note the same, heard", quote(g.conv))
		}
		why = append(why, s)
		// THE ONE PLACE A GIVE'S NOTE EDGES ARE MADE (noteedges.go): the
		// one the line is said to judges, the conversation's other
		// carriers who catch it hear.
		give := noteGive{topic: g.topic, about: g.aboutWord, cause: Connection{Kind: "option", ID: o.ID, Role: "on", Is: verdictOf(o, "")},
			takers: []noteTaker{{word: string(encounter.RecognitionSpokenTo), creatures: []string{site.member}}}}
		if g.heard {
			var others []string
			for _, c := range b.carriers[g.conv] {
				if c != site.member {
					others = append(others, c)
				}
			}
			give.takers = append(give.takers, noteTaker{word: string(encounter.RecognitionCarriersWhoWitnessed), creatures: others})
		}
		cs = append(cs, noteGiveEdges(give)...)
		if strings.Contains(g.how, "steps") {
			cs = append(cs, Connection{Kind: "conversation", ID: g.conv, Role: "advances"})
		}
	}
	total := 0
	for _, r := range o.Success {
		total += r.Weight
	}
	for _, r := range o.Success {
		if !teach[r.Fact] {
			continue
		}
		s := fmt.Sprintf("its %s teaches %s to whoever its words reach, and a holder that knows it leaves its post; a holder "+
			"the words do not reach keeps holding", winWord(o), quote(r.Fact))
		if len(o.Success) > 1 {
			s = strings.Replace(s, "teaches", fmt.Sprintf("on the row of weight %d of %d, teaches", r.Weight, total), 1)
		}
		why = append(why, s)
		cs = append(cs, Connection{Kind: "fact", ID: r.Fact, Role: "teaches"})
	}
	if len(passNeeds) > 0 || len(grants) > 0 {
		pass := "each holder lets a walker through only when every requirement of its post's pass is met"
		if viaHeard {
			pass += ", so a holder who did not witness it keeps holding"
		}
		why = append(why, pass)
	}
	for _, t := range passNeeds {
		switch v := t.(type) {
		case encounter.TriggerFact:
			cs = append(cs, Connection{Kind: "fact", ID: string(v.Fact), Role: "needs"})
			why = append(why, fmt.Sprintf("the walker must also know the fact %s", quote(string(v.Fact))))
		case encounter.TriggerHolds:
			cs = append(cs, Connection{Kind: "prop", ID: b.unqualify(string(v.Prop)), Role: "needs"})
			why = append(why, fmt.Sprintf("the walker must also carry the prop %s", quote(b.unqualify(string(v.Prop)))))
		}
	}
	if unless, ucs := b.unlessWords(passNots); unless != "" {
		why[len(why)-1] += unless
		cs = append(cs, ucs...)
	}
	f := ctx.facets(site.path, short)
	lead, tail := lineHow(o, f)
	kind, verb := KindWayLine, "lifts the holders' bar of"
	if len(still) > 0 {
		kind, verb = KindWayPartial, "helps past the holders of"
		for _, id := range still {
			cs = append(cs, Connection{Kind: "creature", ID: id, Role: "still-holds"})
		}
		tail = fmt.Sprintf(" It is not enough on its own: %s still %s.", b.creatures(still), pluralVerb(len(still), "holds", "hold")) + tail
	}
	s := selfIn(fmt.Sprintf("The line %s of %s %s %s, held by %s: %s; %s.%s%s",
		quote(o.ID), b.creature(site.member), verb, joinWords, b.creatures(ids), lead, strings.Join(why, "; "), tail, also), short)
	conns := []Connection{{Kind: "line", ID: wayLineID(site.member, o.ID), Role: "by"}, {Kind: "creature", ID: site.member, Role: "by"}}
	if o.Checked() {
		conns = append(conns, methodsOf(o.Check)...)
	} else {
		conns = append(conns, method("free"))
	}
	conns = append(conns, b.needsOf(o.Requires)...)
	conns = append(conns, cs...)
	conns = append(conns, f.onMiss...)
	return wayStmt(kind, join, barHold, "line:"+site.member+":"+o.ID, site.path, s, conns...), true
}

// pluralVerb picks a verb by count.
func pluralVerb(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// knowWays are the ways past the holders by knowing a fact or carrying a
// prop: one per distinct pass made only of those, lifting every holder whose
// every pass it meets; partial when another holder still holds.
func (b *builder) knowWays(join Statement, joinWords string, holders []holder, also string) []Statement {
	var out []Statement
	seen := map[string]bool{}
	for _, h := range holders {
		for _, pass := range h.passes {
			var facts, props []string
			ok := len(pass) > 0
			for _, t := range pass {
				switch v := t.(type) {
				case encounter.TriggerFact:
					facts = append(facts, string(v.Fact))
				case encounter.TriggerHolds:
					props = append(props, b.unqualify(string(v.Prop)))
				case encounter.TriggerRecognized:
					ok = false
				}
			}
			if !ok || len(facts)+len(props) == 0 {
				continue
			}
			key := strings.Join(facts, ",") + "|" + strings.Join(props, ",")
			if seen[key] {
				continue
			}
			seen[key] = true
			known := map[string]bool{}
			for _, f := range facts {
				known["fact:"+f] = true
			}
			for _, p := range props {
				known["prop:"+b.c.Key+"/"+p] = true
				known["prop:"+p] = true
			}
			var lifted, still []string
			var allNots []encounter.Trigger
			for _, k := range holders {
				met, _, needs, nots := passJudge(k, nil, known)
				if met && len(needs) == 0 {
					lifted = append(lifted, k.id)
					allNots = append(allNots, nots...)
				} else {
					still = append(still, k.id)
				}
			}
			var what []string
			var cs []Connection
			for _, f := range facts {
				what = append(what, "knowing the fact "+quote(f))
				cs = append(cs, Connection{Kind: "fact", ID: f, Role: "needs"})
			}
			for _, p := range props {
				what = append(what, "carrying the prop "+quote(p))
				cs = append(cs, Connection{Kind: "prop", ID: p, Role: "needs"})
			}
			if len(facts) > 0 {
				cs = append(cs, method("knowledge"))
			}
			if len(props) > 0 {
				cs = append(cs, method("item"))
			}
			kind := KindWayKnown
			s := fmt.Sprintf("%s lifts the holders' bar of %s: %s %s a walker through on it.", capitalize(joinAnd(what)),
				joinWords, b.creatures(lifted), pluralVerb(len(lifted), "lets", "let"))
			if len(still) > 0 {
				kind = KindWayPartial
				s = fmt.Sprintf("%s helps past the holders of %s: %s %s a walker through on it, but it is not enough on its "+
					"own: %s still %s.", capitalize(joinAnd(what)), joinWords, b.creatures(lifted),
					pluralVerb(len(lifted), "lets", "let"), b.creatures(still), pluralVerb(len(still), "holds", "hold"))
				for _, id := range still {
					cs = append(cs, Connection{Kind: "creature", ID: id, Role: "still-holds"})
				}
			}
			if unless, ucs := b.unlessWords(allNots); unless != "" {
				s = strings.TrimSuffix(s, ".") + unless + "."
				cs = append(cs, ucs...)
			}
			for _, id := range lifted {
				cs = append([]Connection{{Kind: "creature", ID: id, Role: "by"}}, cs...)
			}
			out = append(out, wayStmt(kind, join, barHold, "know:"+key, "room.room.monsterBindings."+h.id+".posts", s+also, cs...))
		}
	}
	return out
}

// winWord is what lifts the bar: a win of a checked line, the saying of a
// plain one.
func winWord(o encounter.Option) string {
	if o.Checked() {
		return "win"
	}
	if o.Does() {
		return "doing"
	}
	return "saying"
}

// targetsOf is every state a beat moves a recipe to, from any state, in
// name order.
func targetsOf(r machine.Recipe, beat machine.Beat) []string {
	var out []string
	for _, s := range sortedKeys(r.States) {
		for _, tr := range r.States[s].Transitions {
			if tr.On == beat && tr.To != "" && !contains(out, tr.To) {
				out = append(out, tr.To)
			}
		}
	}
	sort.Strings(out)
	return out
}

// relocateWay is the line a passage is carried by: how the passage is
// taken once its bars are lifted, not an alternative to lifting them.
func (b *builder) relocateWay(join Statement, joinWords, kind, id string, ctx *wayCtx) Statement {
	path := join.Path
	i := strings.LastIndex(path, ".options[")
	j := strings.IndexByte(path[i:], ']')
	linePath := path[:i+j+1]
	var opts []encounter.Option
	short := "the creature spoken to"
	switch kind {
	case "prop":
		for _, p := range b.c.Field.Props {
			if b.unqualify(string(p.ID)) == id {
				opts = p.Options
			}
		}
		for _, p := range b.c.Field.Placed {
			if b.unqualify(string(p.ID)) == id {
				opts = p.Options
			}
		}
		short = "the prop"
	case "creature":
		if m := b.monsters[id]; m != nil {
			opts = m.Options
		}
		short = b.shortName(id)
	case "faction":
		opts = b.factionLines(id)
	}
	var k int
	_, _ = fmt.Sscanf(path[i+len(".options["):], "%d", &k)
	var o encounter.Option
	if k < len(opts) {
		o = opts[k]
	}
	f := ctx.facets(linePath, short)
	lead, tail := lineHow(o, f)
	cs := []Connection{{Kind: "line", ID: wayLineID(id, o.ID), Role: "by"}, {Kind: kind, ID: id, Role: "by"}}
	if o.Checked() {
		cs = append(cs, methodsOf(o.Check)...)
	} else {
		cs = append(cs, method("free"))
	}
	cs = append(cs, b.needsOf(o.Requires)...)
	hidden := ""
	if c := b.concealmentOf(kind, id); c != nil {
		cid := b.unqualify(string(c.ID))
		cs = append(cs, Connection{Kind: "concealment", ID: cid, Role: "needs"})
		hidden = fmt.Sprintf(" It is how the passage is taken once its hidden bar is lifted, not an alternative to it: only a "+
			"member who knows the secret %s is shown the %s and can use the line.", quote(cid), kind)
	}
	cs = append(cs, f.onMiss...)
	s := fmt.Sprintf("The line %s of the %s %s carries %s: %s; its %s sets the speaker down at the far end.%s%s",
		quote(o.ID), kind, quote(id), joinWords, lead, winWord(o), hidden, tail)
	return wayStmt(KindWayPassage, join, "", "line:"+kind+"-"+id+":"+o.ID, linePath, s, cs...)
}

// jailWays are the lines that carry an arrest's passage: those that step
// the conversation into a state whose jail sets the taken down. Entering the
// arresting state takes and holds the party and moves nobody; with no jail,
// nobody passes, and no way is said (the join says so).
func (b *builder) jailWays(join Statement, joinWords, conv string, ctx *wayCtx) []Statement {
	path := join.Path
	state := strings.TrimPrefix(path, "interactions."+conv+".outcomes.")
	state = state[:strings.IndexByte(state, '[')]
	var in *encounter.InteractionInput
	for i := range b.c.Field.Interactions {
		if string(b.c.Field.Interactions[i].ID) == conv {
			in = &b.c.Field.Interactions[i]
		}
	}
	if in == nil {
		return nil
	}
	var jails []string
	for _, s := range sortedKeys(in.Outcomes) {
		for _, c := range in.Outcomes[s] {
			if c.Jail != nil && !contains(jails, s) {
				jails = append(jails, s)
			}
		}
	}
	if len(jails) == 0 {
		return nil
	}
	// What takes them: the beats into the arresting state, and the lines
	// that send those beats.
	var into []string
	var beats []machine.Beat
	for _, s := range sortedKeys(in.Recipe.States) {
		for _, tr := range in.Recipe.States[s].Transitions {
			if tr.To == state {
				into = append(into, fmt.Sprintf("from %s on %s", quote(s), quote(string(tr.On))))
				beats = append(beats, tr.On)
			}
		}
	}
	var takers []string
	var arrests []Connection
	for _, site := range b.lineSites() {
		a := site.option.Advance
		if a == nil || string(a.Interaction) != conv {
			continue
		}
		for _, beat := range beats {
			if a.Failure == beat || a.Success == beat {
				verdict := "a miss"
				if a.Success == beat {
					verdict = "a win"
				}
				takers = append(takers, fmt.Sprintf("%s of %s (%s)", verdict, quote(site.option.ID), site.member))
				arrests = append(arrests, Connection{Kind: "line", ID: wayLineID(site.member, site.option.ID), Role: "arrests"})
				break
			}
		}
	}
	taken := fmt.Sprintf("The party is taken on entering %s", quote(state))
	if len(into) > 0 {
		taken += ", which the conversation enters " + joinOr(into)
	}
	if len(takers) > 0 {
		taken += "; the lines that step it so are " + joinAnd(takers)
	}
	taken += "."
	var out []Statement
	for _, site := range b.lineSites() {
		a := site.option.Advance
		if a == nil || string(a.Interaction) != conv || a.Success == "" {
			continue
		}
		var to []string
		for _, t := range targetsOf(in.Recipe, a.Success) {
			if contains(jails, t) {
				to = append(to, t)
			}
		}
		if len(to) == 0 {
			continue
		}
		o := site.option
		short := b.shortName(site.member)
		f := ctx.facets(site.path, short)
		lead, tail := lineHow(o, f)
		cs := []Connection{{Kind: "line", ID: wayLineID(site.member, o.ID), Role: "by"}, {Kind: "creature", ID: site.member, Role: "by"},
			{Kind: "conversation", ID: conv, Role: "by"}}
		if o.Checked() {
			cs = append(cs, methodsOf(o.Check)...)
		} else {
			cs = append(cs, method("free"))
		}
		cs = append(cs, b.needsOf(o.Requires)...)
		cs = append(cs, arrests...)
		cs = append(cs, f.onMiss...)
		s := selfIn(fmt.Sprintf("The line %s of %s carries %s: %s; its %s steps %s to %s, whose jail sets the taken down "+
			"there, without walking the way between. %s Until a taken member says it, the taken stay held where they were "+
			"taken.%s", quote(o.ID), b.creature(site.member), joinWords, lead, winWord(o), quote(conv), joinOr(quoteAll(to)),
			taken, tail), short)
		out = append(out, wayStmt(KindWayPassage, join, "", "line:"+site.member+":"+o.ID, site.path, s, cs...))
	}
	return out
}
