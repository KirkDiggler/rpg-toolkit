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
// THE CHAIN IS SHORT AND THE ENGINE'S OWN: join → what bars it (a lock, a
// holding post, a secret, a passage's carrier) → what lifts that bar (a lock
// approach; a line that grants what the post's pass asks, or teaches a fact
// the holder's routine leaves its post on; the fight; a search; a record that
// reveals the secret). A need is named, never followed further: the studio's
// threads follow a need to who teaches it. Nothing here simulates play — who
// a line's words reach, which holder witnessed it — and every sentence says
// where play decides.
//
// COUNTABLE WITHOUT THE SENTENCE: every way connects to its join
// (`through`), to what carries it (`by`), to each need (`needs`), to what a
// miss grants or steps (`on-miss`), and to its method as
// {Kind: "method", ID: "skill:<name>" | "ability:<name>" | "tool:<ref>" |
// "knowledge" | "item" | "fight" | "free" | "conversation"}. A check with
// alternatives carries one method per alternative. A DC is said in the
// sentence only: [Connection] has no place for a number.

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

// wayStmt makes a way statement. A way is not a hub entry: the needs it
// names are the carrier's own requirements, already in the hubs, so it is
// built without [builder.fstmt]'s hub registration.
func wayStmt(kind Kind, join Statement, slug, path, sentence string, conns ...Connection) Statement {
	conns = append([]Connection{{Kind: "join", ID: join.About.ID, Role: "through"}}, conns...)
	return Statement{Kind: kind, About: Subject{Kind: "way", ID: "way:" + join.About.ID + ":" + slug},
		Sentence: sentence, Path: path, Connects: dedupe(conns)}
}

// wayKind is whether a statement says a way.
func wayKind(k Kind) bool { return strings.HasPrefix(string(k), "place.way.") }

// methodsOf is a check's methods, one per alternative: a skill or an
// ability, and the tool it is made with.
func methodsOf(approaches []encounter.CheckApproach) []Connection {
	var out []Connection
	for _, a := range approaches {
		kind := "skill:"
		switch a.Ability {
		case "str", "dex", "con", "int", "wis", "cha":
			kind = "ability:"
		}
		out = append(out, Connection{Kind: "method", ID: kind + a.Ability, Role: "by"})
		if a.Tool != "" {
			out = append(out, Connection{Kind: "method", ID: "tool:" + a.Tool, Role: "by"})
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

// waysFor says every way through one join, in a fixed order: what lifts a
// secret, a lock, a holding post, a passage, else that nothing bars it.
func (b *builder) waysFor(join Statement, ctx *wayCtx, joinWords string) []Statement {
	var out []Statement
	carrier := func(kind string) string {
		for _, c := range join.Connects {
			if c.Kind == kind && c.Role == "by" {
				return c.ID
			}
		}
		return ""
	}
	switch join.Kind {
	case KindJoinDoor:
		door := carrier("door")
		out = append(out, b.secretWays(join, joinWords, "door", door)...)
		out = append(out, b.lockWays(join, joinWords, door)...)
		out = append(out, b.holdWays(join, joinWords, door, ctx)...)
		if len(out) == 0 {
			out = append(out, wayStmt(KindWayFree, join, "free", join.Path, fmt.Sprintf(
				"Nothing bars %s: the door %s is neither locked, held nor hidden, so a member opens it and walks through.",
				joinWords, quote(door)), Connection{Kind: "door", ID: door, Role: "by"}, method("free")))
		}
	case KindJoinOpen:
		out = append(out, wayStmt(KindWayFree, join, "free", join.Path, fmt.Sprintf(
			"Nothing bars %s: a member walks from one to the other.", joinWords), method("free")))
	case KindJoinPassage:
		if conv := carrier("conversation"); conv != "" {
			out = append(out, b.arrestWay(join, joinWords, conv))
			break
		}
		for _, kind := range []string{"prop", "creature", "faction"} {
			if id := carrier(kind); id != "" {
				out = append(out, b.secretWays(join, joinWords, kind, id)...)
				out = append(out, b.relocateWay(join, joinWords, kind, id, ctx))
			}
		}
	}
	if len(out) == 0 {
		out = append(out, wayStmt(KindWayNone, join, "none", join.Path, fmt.Sprintf(
			"No way through %s is said: nothing the file authors lifts what bars it.", joinWords)))
	}
	return out
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

// secretWays are the ways to KNOW a hidden door or prop: a search, and each
// record that reveals the secret. Until a member knows it, what it hides is
// not shown to them; seeing someone use it also reveals it, which is play,
// not a way the file offers.
func (b *builder) secretWays(join Statement, joinWords, kind, id string) []Statement {
	c := b.concealmentOf(kind, id)
	if c == nil {
		return nil
	}
	cid := b.unqualify(string(c.ID))
	what := fmt.Sprintf("the %s %s", kind, quote(id))
	out := []Statement{wayStmt(KindWaySearch, join, "search:"+cid, "concealments."+cid, fmt.Sprintf(
		"A search finds the secret %s, which hides %s on %s, a way to know %s: until a member knows the secret, %s is "+
			"not shown to them. A miss finds nothing.", quote(cid), what, joinWords, what, what),
		append([]Connection{{Kind: "concealment", ID: cid, Role: "by"}}, methodsOf(c.Checks)...)...)}
	out[0].Sentence = strings.Replace(out[0].Sentence, "A search finds", "A search with "+checkWords(c.Checks)+" finds", 1)
	for i, in := range b.c.Intel {
		if b.unqualify(string(in.Reveals.Concealment)) != cid {
			continue
		}
		rid := b.unqualify(string(in.ID))
		out = append(out, wayStmt(KindWayTold, join, "told:"+rid, fmt.Sprintf("intel[%d]", i), fmt.Sprintf(
			"Holding the record %s reveals the secret %s, a way to know %s on %s: whoever comes to hold the record — "+
				"told it, or taking what carries it — knows the secret. Who tells it or carries it is said with the record.",
			quote(rid), quote(cid), what, joinWords),
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

// lockWays is one way per approach of a locked door's lock.
func (b *builder) lockWays(join Statement, joinWords, door string) []Statement {
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
		out = append(out, wayStmt(KindWayLock, join, slug, "room.room.doorBindings."+door, fmt.Sprintf(
			"The door %s starts locked; beating its lock with %s opens it, a way through %s. A miss leaves the door locked.",
			quote(door), checkWords([]encounter.CheckApproach{a}), joinWords),
			append([]Connection{{Kind: "door", ID: door, Role: "by"}}, methodsOf(lock.Approaches[i:i+1])...)...))
	}
	return out
}

// holder is one creature posted on a door: the states its post holds the
// door in, the topics its pass asks, and its recipe.
type holder struct {
	id     string
	states []string
	topics []string
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
			for _, t := range p.Pass {
				if r, ok := t.(encounter.TriggerRecognized); ok && !contains(h.topics, string(r.Topic)) {
					h.topics = append(h.topics, string(r.Topic))
				}
			}
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
// door in when it comes to know the fact: a transition on the fact, guarded
// by that fact alone being true, to a state that holds nothing on the door.
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

// holdWays are the ways past a door's holding posts: every line whose win
// (or plain saying) grants a topic the posts' pass asks, or teaches a fact
// the holders' routines leave their posts on; and the fight.
func (b *builder) holdWays(join Statement, joinWords, door string, ctx *wayCtx) []Statement {
	holders := b.holdersOf(door)
	if len(holders) == 0 {
		return nil
	}
	var ids, topics []string
	for _, h := range holders {
		ids = append(ids, h.id)
		for _, t := range h.topics {
			if !contains(topics, t) {
				topics = append(topics, t)
			}
		}
	}
	var out []Statement
	for _, site := range b.lineSites() {
		o := site.option
		short := b.shortName(site.member)
		lift, conns := b.liftWords(o, topics, holders)
		if lift == "" {
			continue
		}
		f := ctx.facets(site.path, short)
		lead, tail := lineHow(o, f)
		s := selfIn(fmt.Sprintf("The line %s of %s is a way through %s held by %s: %s; %s.%s",
			quote(o.ID), b.creature(site.member), joinWords, b.creatures(ids), lead, lift, tail), short)
		cs := []Connection{{Kind: "line", ID: o.ID, Role: "by"}, {Kind: "creature", ID: site.member, Role: "by"}}
		if o.Checked() {
			cs = append(cs, methodsOf(o.Check)...)
		} else {
			cs = append(cs, method("free"))
		}
		cs = append(cs, b.needsOf(o.Requires)...)
		cs = append(cs, conns...)
		cs = append(cs, f.onMiss...)
		out = append(out, wayStmt(KindWayLine, join, "line:"+site.member+":"+o.ID, site.path, s, cs...))
	}
	var fc []Connection
	for _, id := range ids {
		fc = append(fc, Connection{Kind: "creature", ID: id, Role: "by"})
	}
	fc = append(fc, method("fight"))
	out = append(out, wayStmt(KindWayFight, join, "fight", "room.room.monsterBindings."+holders[0].id+".posts", fmt.Sprintf(
		"The fight is a way through %s: %s each hold the door only while posted, up and standing on a cell of the "+
			"post; one who is down, or drawn off the post, holds nothing. Failing it is losing the fight.",
		joinWords, b.creatures(ids)), fc...))
	return out
}

// liftWords is how a line's win (or plain saying) lifts the holders' bar,
// or empty: the topics its rows or its conversation's outcome grant that a
// pass asks, and the facts it teaches that routines leave their posts on.
func (b *builder) liftWords(o encounter.Option, topics []string, holders []holder) (string, []Connection) {
	var parts []string
	var conns []Connection
	for _, g := range o.Grants {
		if g.Failure || g.Consequence.Recognize == nil || !contains(topics, string(g.Consequence.Recognize.Topic)) {
			continue
		}
		t := string(g.Consequence.Recognize.Topic)
		parts = append(parts, fmt.Sprintf("its %s grants %s (the topic the posts let through), held by the creature it "+
			"is said to about %s, and by the conversation's carriers who witness it", winWord(o), quote(t),
			shortSubjects(g.Consequence.Recognize.About)))
		conns = append(conns, Connection{Kind: "topic", ID: t, Role: "grants"})
	}
	if o.Advance != nil && o.Advance.Success != "" {
		conv := string(o.Advance.Interaction)
		for _, in := range b.c.Field.Interactions {
			if string(in.ID) != conv {
				continue
			}
			for _, state := range targetsOf(in.Recipe, o.Advance.Success) {
				for _, c := range in.Outcomes[state] {
					if c.Recognize == nil || !contains(topics, string(c.Recognize.Topic)) {
						continue
					}
					t := string(c.Recognize.Topic)
					parts = append(parts, fmt.Sprintf("its %s steps %s to %s, whose outcome grants %s (the topic the posts "+
						"let through) to the creature it is said to and the carriers who witness it", winWord(o), quote(conv),
						quote(state), quote(t)))
					conns = append(conns, Connection{Kind: "topic", ID: t, Role: "grants"},
						Connection{Kind: "conversation", ID: conv, Role: "advances"})
				}
			}
		}
	}
	rows := o.Success
	total := 0
	for _, r := range rows {
		total += r.Weight
	}
	for _, r := range rows {
		if r.Fact == "" {
			continue
		}
		var off, stay []string
		for _, h := range holders {
			if h.leavesOn(r.Fact) {
				off = append(off, h.id)
			} else {
				stay = append(stay, h.id)
			}
		}
		if len(off) == 0 {
			continue
		}
		p := fmt.Sprintf("its %s teaches %s to whoever its words reach", winWord(o), quote(r.Fact))
		if len(rows) > 1 {
			p = fmt.Sprintf("its %s, on the row of weight %d of %d, teaches %s to whoever its words reach", winWord(o),
				r.Weight, total, quote(r.Fact))
		}
		if len(stay) == 0 {
			p += fmt.Sprintf(", and every holder (%s) leaves its post once it knows it — a holder the words do not "+
				"reach keeps holding", b.creatures(off))
		} else {
			p += fmt.Sprintf(", and %s leave their posts once they know it, but %s keep holding, so on its own it is "+
				"not a way through", b.creatures(off), b.creatures(stay))
		}
		parts = append(parts, p)
		conns = append(conns, Connection{Kind: "fact", ID: r.Fact, Role: "teaches"})
		for _, id := range off {
			conns = append(conns, Connection{Kind: "creature", ID: id, Role: "draws-off"})
		}
	}
	return strings.Join(parts, "; "), conns
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

// relocateWay is the line a passage is carried by, as a way.
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
	cs := []Connection{{Kind: "line", ID: o.ID, Role: "by"}, {Kind: kind, ID: id, Role: "by"}}
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
		hidden = fmt.Sprintf(" Only a member who knows the secret %s is shown the %s and can use the line.", quote(cid), kind)
	}
	cs = append(cs, f.onMiss...)
	s := fmt.Sprintf("The line %s of the %s %s carries %s: %s; its %s sets the speaker down at the far end.%s%s",
		quote(o.ID), kind, quote(id), joinWords, lead, winWord(o), hidden, tail)
	return wayStmt(KindWayPassage, join, "line:"+kind+"-"+id+":"+o.ID, linePath, s, cs...)
}

// arrestWay is a conversation's arrest as the way it is taken: the
// conversation enters the arresting state on the beats said, which the lines
// that step it with them send.
func (b *builder) arrestWay(join Statement, joinWords, conv string) Statement {
	path := join.Path
	state := strings.TrimPrefix(path, "interactions."+conv+".outcomes.")
	state = state[:strings.IndexByte(state, '[')]
	var into []string
	var beats []machine.Beat
	for _, in := range b.c.Field.Interactions {
		if string(in.ID) != conv {
			continue
		}
		for _, s := range sortedKeys(in.Recipe.States) {
			for _, tr := range in.Recipe.States[s].Transitions {
				if tr.To == state {
					into = append(into, fmt.Sprintf("from %s on %s", quote(s), quote(string(tr.On))))
					beats = append(beats, tr.On)
				}
			}
		}
	}
	var senders []string
	var cs []Connection
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
				senders = append(senders, fmt.Sprintf("%s of %s (%s)", verdict, quote(site.option.ID), site.member))
				cs = append(cs, Connection{Kind: "line", ID: site.option.ID, Role: "steps"})
				break
			}
		}
	}
	s := fmt.Sprintf("The conversation %s carries %s: the party is taken on entering %s", quote(conv), joinWords, quote(state))
	if len(into) > 0 {
		s += ", which the conversation enters " + joinOr(into)
	}
	if len(senders) > 0 {
		s += "; the lines that step it so are " + joinAnd(senders)
	}
	s += ". Nobody chooses it: it is what those lines cost."
	cs = append([]Connection{{Kind: "conversation", ID: conv, Role: "by"}}, cs...)
	cs = append(cs, method("conversation"))
	return wayStmt(KindWayPassage, join, "conversation:"+conv+":"+state, path, s, cs...)
}
