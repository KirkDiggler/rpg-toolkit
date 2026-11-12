// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
	"strings"
)

// folded.go is the folded rendering: the same statements, said for a person
// to read in a couple of minutes. Nothing here adds a fact or drops one; it
// only groups. Every [Item] carries the path of every statement it stands
// for, and a test holds every statement to some item.
//
// The folds, all mechanical:
//   - a line is said once, as a heading and short sub-items;
//   - things that differ only by state are said once with the states listed;
//   - creatures (or routines) whose lines, posts or rows are identical apart
//     from the creature's own name are said once, and the others name what
//     they share and say only what differs;
//   - a creature is named in full on its heading and by a short form after;
//   - topics and facts become short tables of places.

// Item is one entry of the folded read-back: its text, the paths of every
// statement it stands for, and the entries under it. Base is the path its
// entries' paths are shown relative to.
type Item struct {
	Text  string   `json:"text"`
	Paths []string `json:"paths"`
	Base  string   `json:"base,omitempty"`
	Items []Item   `json:"items,omitempty"`

	// terse shows only the paths of the lines a fold names, not of every
	// facet beneath them; Paths still carries them all.
	terse bool
}

// FoldedSection is one heading of the folded read-back.
type FoldedSection struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}

// self substitutes a short name for the {self} placeholders and starts the
// text with a capital.
func self(text, name string) string {
	text = strings.ReplaceAll(text, "{Self}", capitalize(name))
	return capitalize(strings.ReplaceAll(text, "{self}", name))
}

// Folded is the read-back folded for a person.
func (r *ReadBack) Folded() []FoldedSection {
	var out []FoldedSection
	for _, s := range r.Sections {
		var items []Item
		switch s.Title {
		case "Creatures":
			items = foldCreatures(s.Groups)
		case "Conversations":
			for _, g := range s.Groups {
				items = append(items, foldConversation(g))
			}
		case "Routines":
			items = foldRoutines(s.Groups)
		case "Topics", "Facts":
			for _, g := range s.Groups {
				items = append(items, foldHub(g))
			}
		default:
			for _, g := range s.Groups {
				items = append(items, foldPlain(g))
			}
		}
		if len(items) > 0 {
			out = append(out, FoldedSection{Title: s.Title, Items: items})
		}
	}
	return out
}

// foldPlain folds a group of the place or a side: each statement as itself,
// and each line as a block.
func foldPlain(g Group) Item {
	head := Item{Text: g.Title}
	short := "the creature spoken to"
	if g.About.Kind == "site" {
		short = "the prop"
	}
	var bundles Item
	for _, chunk := range chunkLines(g.Statements) {
		if chunk.line != nil {
			head.Items = append(head.Items, foldLine(chunk.facets, short))
			continue
		}
		st := chunk.facets[0]
		if st.Kind == KindEnding {
			n := len(head.Items)
			if n > 0 && strings.HasPrefix(head.Items[n-1].Text, "The run ends when "+st.fold.state+",") {
				head.Items[n-1].Text = strings.TrimSuffix(head.Items[n-1].Text, ".") + "; " + st.fold.text + "."
				head.Items[n-1].Paths = append(head.Items[n-1].Paths, st.Path)
				continue
			}
			head.Items = append(head.Items, Item{Text: "The run ends when " + st.fold.state +
				", on the first of these whose condition holds: " + st.fold.text + ".", Paths: []string{st.Path}})
			continue
		}
		if st.Kind == KindBundle {
			if bundles.Text == "" {
				bundles.Text = "Bundles, each a group of rows used as one row of a routine's table and said in full where used:"
			}
			bundles.Text += " " + quote(st.About.ID)
			bundles.Paths = append(bundles.Paths, st.Path)
			continue
		}
		head.Items = append(head.Items, Item{Text: st.Sentence, Paths: []string{st.Path}})
	}
	if bundles.Text != "" {
		head.Items = append(head.Items, bundles)
	}
	return head
}

// chunk is either one statement or the facets of one line.
type chunk struct {
	line   *Statement
	facets []Statement
}

// chunkLines splits statements into single statements and line blocks; a
// line's facets follow its KindLine statement and share its path prefix.
func chunkLines(sts []Statement) []chunk {
	var out []chunk
	for _, st := range sts {
		if st.fold.line != "" && len(out) > 0 && out[len(out)-1].line != nil &&
			strings.HasPrefix(st.Path, out[len(out)-1].line.Path) && st.Kind != KindLine {
			out[len(out)-1].facets = append(out[len(out)-1].facets, st)
			continue
		}
		if st.Kind == KindLine {
			s := st
			out = append(out, chunk{line: &s, facets: []Statement{st}})
			continue
		}
		out = append(out, chunk{facets: []Statement{st}})
	}
	return out
}

// foldLine says one line as a block: the quote and id once, then who it is
// said to and what it costs and rolls, what it needs, and what a win and a
// miss each do. Its texts keep {self} until the caller substitutes.
func foldLine(facets []Statement, short string) Item {
	line := facets[0]
	head := Item{Text: line.fold.line + ", " + line.fold.text, Paths: []string{line.Path}, Base: line.Path}
	var reqs Item
	type alt struct {
		text  string
		paths []string
	}
	verdicts := map[string][]alt{}
	adv := map[string]Item{}
	for _, f := range facets[1:] {
		switch f.Kind {
		case KindLineCheck:
			head.Text += "; " + f.fold.text
			head.Paths = append(head.Paths, f.Path)
		case KindLineRequire:
			if reqs.Text == "" {
				reqs.Text = "Offered while: " + f.fold.text
			} else {
				reqs.Text += " and " + f.fold.text
			}
			reqs.Paths = append(reqs.Paths, f.Path)
		case KindLineAdvance:
			if f.fold.text != "" {
				adv["win"] = Item{Text: "the line " + f.fold.text, Paths: []string{f.Path}}
			}
			if f.fold.miss != "" {
				adv["miss"] = Item{Text: "the line " + f.fold.miss, Paths: []string{f.Path}}
			}
		case KindLineRow:
			verdicts[f.fold.verdict] = append(verdicts[f.fold.verdict], alt{text: f.fold.text, paths: []string{f.Path}})
		case KindLineGrant:
			alts := verdicts[f.fold.verdict]
			if len(alts) > 0 {
				last := &alts[len(alts)-1]
				last.text += "; " + f.fold.text
				last.paths = append(last.paths, f.Path)
			}
		}
	}
	if reqs.Text != "" {
		reqs.Text += "."
		head.Items = append(head.Items, reqs)
	}
	// A plain line's rows are what saying it does; a checked line has a win
	// and a miss. An advance's win half goes with the win (or the saying).
	for _, v := range []struct{ key, lead, adv string }{
		{"said", "When said: ", "win"}, {"win", "On a win: ", "win"}, {"miss", "On a miss: ", "miss"},
	} {
		if v.key == "said" && len(verdicts["said"]) == 0 {
			continue
		}
		if v.key == "win" && len(verdicts["said"]) > 0 {
			continue
		}
		it := Item{}
		var texts []string
		for _, x := range verdicts[v.key] {
			texts = append(texts, x.text)
			it.Paths = append(it.Paths, x.paths...)
		}
		if a, ok := adv[v.adv]; ok {
			texts = append(texts, a.Text)
			it.Paths = append(it.Paths, a.Paths...)
		}
		if len(texts) == 0 {
			continue
		}
		it.Text = v.lead + strings.Join(texts, "; or ")
		if _, ok := adv[v.adv]; ok && len(texts) > 1 {
			// The advance is not an alternative row: it follows whichever row.
			it.Text = v.lead + strings.Join(texts[:len(texts)-1], "; or ") + "; and " + texts[len(texts)-1]
		}
		head.Items = append(head.Items, it)
	}
	return withSelf(head, short)
}

// withSelf substitutes a short name through an item and its entries.
func withSelf(it Item, name string) Item {
	it.Text = self(it.Text, name)
	items := make([]Item, len(it.Items))
	for i := range it.Items {
		items[i] = withSelf(it.Items[i], name)
	}
	if it.Items != nil {
		it.Items = items
	}
	return it
}

// canonical is an item as text, its paths relative to base, for comparing two
// creatures' blocks: equal means equal apart from where in the file.
func canonical(it Item, base string) string {
	var sb strings.Builder
	sb.WriteString(it.Text)
	for _, p := range it.Paths {
		sb.WriteString("|" + strings.TrimPrefix(p, base))
	}
	for _, c := range it.Items {
		sb.WriteString("{" + canonical(c, base) + "}")
	}
	return sb.String()
}

// allPaths is every path an item and its entries stand for.
func allPaths(it Item) []string {
	out := append([]string{}, it.Paths...)
	for _, c := range it.Items {
		out = append(out, allPaths(c)...)
	}
	return out
}

// creatureBlock is one creature's folded block before folding across
// creatures: its lines and posts kept apart so they can be compared.
type creatureBlock struct {
	name, short string
	head        Item
	before      []Item // holds, arrival, routine, blocking
	posts       []Item // texts with {self}
	lines       []Item // texts with {self}, Base the line's own path
	lineIDs     []string
	after       []Item // borrowed lines
}

// foldCreatures folds every creature, then folds across creatures: a line
// or post identical, apart from the name, to one an earlier creature has is
// named, not said again.
func foldCreatures(groups []Group) []Item {
	var blocks []creatureBlock
	for _, g := range groups {
		blocks = append(blocks, foldCreature(g))
	}
	var out []Item
	for i, c := range blocks {
		it := c.head
		for _, x := range c.before {
			if x.Text == "" {
				continue
			}
			it.Items = append(it.Items, withSelf(x, c.short))
		}
		it.Items = append(it.Items, foldShared(c, blocks[:i], c.posts, nil, "post", "posts")...)
		it.Items = append(it.Items, foldShared(c, blocks[:i], c.lines, c.lineIDs, "line", "lines")...)
		for _, x := range c.after {
			it.Items = append(it.Items, withSelf(x, c.short))
		}
		out = append(out, it)
	}
	return out
}

// foldShared says a creature's posts or lines: those identical to an earlier
// creature's are named once per earlier creature, the rest said in full.
func foldShared(c creatureBlock, earlier []creatureBlock, items []Item, ids []string, one, many string) []Item {
	type shared struct {
		with  string
		ids   []string
		paths []string
	}
	var order []string
	byWith := map[string]*shared{}
	var own []Item
	// The earlier creature sharing the most of these is named first, so a
	// creature that repeats another whole is said as one fold.
	poolOf := func(e creatureBlock) []Item {
		if one == "post" {
			return e.posts
		}
		return e.lines
	}
	has := func(e creatureBlock, it Item) bool {
		for _, x := range poolOf(e) {
			if canonical(x, x.Base) == canonical(it, it.Base) {
				return true
			}
		}
		return false
	}
	ranked := append([]creatureBlock{}, earlier...)
	score := func(e creatureBlock) int {
		n := 0
		for _, it := range items {
			if has(e, it) {
				n++
			}
		}
		return n
	}
	sort.SliceStable(ranked, func(i, j int) bool { return score(ranked[i]) > score(ranked[j]) })
	for k, it := range items {
		var match string
		for _, e := range ranked {
			if has(e, it) {
				match = e.name
				break
			}
		}
		if match == "" {
			own = append(own, withSelf(it, c.short))
			continue
		}
		s, ok := byWith[match]
		if !ok {
			s = &shared{with: match}
			byWith[match] = s
			order = append(order, match)
		}
		if ids != nil {
			s.ids = append(s.ids, quote(ids[k]))
		}
		s.paths = append(s.paths, allPaths(it)...)
	}
	var out []Item
	for _, w := range order {
		s := byWith[w]
		n := len(s.paths)
		what := one
		if ids != nil {
			n = len(s.ids)
		}
		if n != 1 {
			what = many
		}
		text := fmt.Sprintf("%s has the same %s as %s", c.name, what, s.with)
		switch {
		case ids != nil && n == 1:
			text = fmt.Sprintf("%s has the same line %s as %s", c.name, s.ids[0], s.with)
		case ids != nil:
			text = fmt.Sprintf("%s has the same %d lines as %s: %s", c.name, n, s.with, joinAnd(s.ids))
		}
		out = append(out, Item{Text: text + ".", Paths: s.paths, terse: ids != nil})
	}
	return append(out, own...)
}

// foldCreature folds one creature's statements.
func foldCreature(g Group) creatureBlock {
	c := creatureBlock{name: g.Title}
	id := g.About.ID
	base := "room.room.monsterBindings." + id
	type postKey struct{ post, pass string }
	var postOrder []postKey
	postStates := map[postKey][]string{}
	postPaths := map[postKey][]string{}
	passByState := map[string]Statement{}
	for _, st := range g.Statements {
		if st.Kind == KindPass {
			passByState[st.fold.state] = st
		}
	}
	for _, ch := range chunkLines(g.Statements) {
		if ch.line != nil {
			l := foldLine(ch.facets, "{self}")
			// foldLine substituted "{self}" with itself; capitals aside, the
			// texts still carry the placeholder for the comparison.
			c.lines = append(c.lines, l)
			c.lineIDs = append(c.lineIDs, lineID(ch.line.fold.line))
			continue
		}
		st := ch.facets[0]
		switch st.Kind {
		case KindCreature:
			c.head = Item{Text: g.Title + ": " + st.fold.text, Paths: []string{st.Path}, Base: base}
			c.short = shortOf(g.Title)
		case KindPost:
			pass := passByState[st.fold.state]
			k := postKey{post: st.fold.text, pass: pass.fold.text}
			if _, ok := postStates[k]; !ok {
				postOrder = append(postOrder, k)
			}
			postStates[k] = append(postStates[k], st.fold.state)
			postPaths[k] = append(postPaths[k], st.Path)
			if pass.Path != "" {
				postPaths[k] = append(postPaths[k], pass.Path)
			}
		case KindPass:
		case KindBorrowLines:
			c.after = append(c.after, Item{Text: st.fold.text, Paths: []string{st.Path}})
		default:
			c.before = append(c.before, Item{Text: st.fold.text, Paths: []string{st.Path}})
		}
	}
	var blocking *Item
	for i := range c.before {
		if strings.HasPrefix(c.before[i].Text, "While its routine is in ") {
			blocking = &c.before[i]
		}
	}
	for _, k := range postOrder {
		states := joinOr(quoteAll(postStates[k]))
		text := fmt.Sprintf("While its routine is in %s %s", states, k.post)
		if blocking != nil && strings.HasPrefix(blocking.Text, "While its routine is in "+states+",") {
			text = fmt.Sprintf("While its routine is in %s, its body blocks its cell; and when %s", states,
				strings.TrimPrefix(k.post, "and "))
			postPaths[k] = append(blocking.Paths, postPaths[k]...)
			blocking.Text = ""
		}
		if k.pass != "" {
			text += "; " + k.pass
		}
		c.posts = append(c.posts, Item{Text: text + ".", Paths: postPaths[k], Base: base})
	}
	return c
}

// lineID is the id out of a line heading “say” (`id`).
func lineID(heading string) string {
	i := strings.LastIndex(heading, "(`")
	if i < 0 {
		return heading
	}
	return strings.TrimSuffix(heading[i+2:], "`)")
}

// shortOf is the short name of a creature from its display name.
func shortOf(display string) string {
	if i := strings.LastIndex(display, " ("); i > 0 {
		display = display[:i]
	}
	return "the " + strings.ToLower(strings.TrimPrefix(display, "The "))
}

// foldConversation says a conversation: its carriers and hail, its moves
// grouped by what they do across the states they leave, its final states,
// what entering a state grants, and the lines that read it.
func foldConversation(g Group) Item {
	head := Item{Text: g.Title, Base: "interactions." + g.About.ID}
	var rules []Item
	var steps []Item
	stepAt := map[string]int{}
	var finals Item
	statePath := map[string]string{}
	stateOrder := []string{}
	// The lines that read it, grouped by the states they need and who
	// carries them: "`vouch`, `straggler` (gate-1, gate-2) in `admitted`".
	var read Item
	type readKey struct{ who, states string }
	var readOrder []readKey
	readLines := map[readKey][]string{}
	lineWho := map[string][]string{}
	lineStates := map[string]string{}
	var lineOrder []string
	for _, st := range g.Statements {
		switch st.Kind {
		case KindConversation:
			head.Text += " — " + st.fold.text
			head.Paths = []string{st.Path}
		case KindApproach, KindHailers, KindOutcome:
			head.Items = append(head.Items, Item{Text: st.fold.text, Paths: []string{st.Path}})
		case KindEngineRule:
			rules = append(rules, Item{Text: st.fold.text, Paths: []string{st.Path}})
		case KindConvState:
			if st.fold.text == "final" {
				finals.Paths = append(finals.Paths, st.Path)
				finals.Text += quote(st.fold.state) + " "
				continue
			}
			statePath[st.fold.state] = st.Path
			stateOrder = append(stateOrder, st.fold.state)
		case KindConvStep:
			i, ok := stepAt[st.fold.text]
			if !ok {
				i = len(steps)
				stepAt[st.fold.text] = i
				steps = append(steps, Item{Text: st.fold.text})
			}
			steps[i].Paths = append(steps[i].Paths, st.Path)
			steps[i].Base += st.fold.state + "\n"
		case KindConvAsked:
			k := st.fold.text + "|" + st.fold.state
			if _, ok := lineStates[k]; !ok {
				lineOrder = append(lineOrder, k)
				lineStates[k] = st.fold.state
			}
			if !contains(lineWho[k], st.fold.who) {
				lineWho[k] = append(lineWho[k], st.fold.who)
			}
			read.Paths = append(read.Paths, st.Path)
		}
	}
	placed := map[string]bool{}
	var moves []Item
	for _, s := range steps {
		states := strings.Fields(s.Base)
		var paths []string
		for _, st := range states {
			if p, ok := statePath[st]; ok && !placed[st] {
				placed[st] = true
				paths = append(paths, p)
			}
		}
		moves = append(moves, Item{
			Text:  fmt.Sprintf("From %s: %s.", joinOr(quoteAll(states)), s.Text),
			Paths: append(paths, s.Paths...),
		})
	}
	for _, st := range stateOrder {
		if !placed[st] {
			moves = append(moves, Item{Text: fmt.Sprintf("In %s it listens for nothing.", quote(st)), Paths: []string{statePath[st]}})
		}
	}
	// The outcome reads after the moves; the hail before them.
	var hail, outcomes []Item
	for _, it := range head.Items {
		if strings.HasPrefix(it.Text, "On entering") {
			outcomes = append(outcomes, it)
		} else {
			hail = append(hail, it)
		}
	}
	head.Items = append(hail, moves...)
	if finals.Text != "" {
		states := strings.Fields(finals.Text)
		verb := "is"
		if len(states) > 1 {
			verb = "are"
		}
		head.Items = append(head.Items, Item{Text: fmt.Sprintf("%s %s final: once entered, nothing moves it again.",
			joinAnd(states), verb), Paths: finals.Paths})
	}
	head.Items = append(head.Items, outcomes...)
	for _, k := range lineOrder {
		rk := readKey{who: strings.Join(lineWho[k], ", "), states: lineStates[k]}
		if _, ok := readLines[rk]; !ok {
			readOrder = append(readOrder, rk)
		}
		readLines[rk] = append(readLines[rk], strings.SplitN(k, "|", 2)[0])
	}
	if len(readOrder) > 0 {
		var parts []string
		for _, rk := range readOrder {
			parts = append(parts, fmt.Sprintf("%s (%s) in %s", joinAnd(readLines[rk]), rk.who, rk.states))
		}
		read.Text = "Lines offered only in a state of it: " + strings.Join(parts, "; ") + "."
		read.terse = true
		head.Items = append(head.Items, read)
	}
	head.Items = append(head.Items, rules...)
	return head
}

// foldRoutines folds every routine, then names a routine identical, apart
// from its runner's name, to an earlier one instead of saying it again.
func foldRoutines(groups []Group) []Item {
	var out []Item
	var seen []Item
	var seenDef []string
	for _, g := range groups {
		raw := foldRoutine(g)
		key := canonical(Item{Items: raw.Items}, raw.Base)
		match := ""
		for i, s := range seen {
			if canonical(Item{Items: s.Items}, s.Base) == key {
				match = seenDef[i]
				break
			}
		}
		if match != "" {
			out = append(out, Item{Text: fmt.Sprintf("%s Its states and rows are word for word those of %s.", raw.Text, match),
				Paths: allPaths(raw)})
			continue
		}
		seen = append(seen, raw)
		seenDef = append(seenDef, quote(g.About.ID))
		// A routine's texts already name its runner; this only starts each
		// entry with a capital.
		out = append(out, withSelfTail(raw, "the creature running it"))
	}
	return out
}

// withSelfTail substitutes a short name in an item's entries, not its
// heading.
func withSelfTail(it Item, name string) Item {
	items := make([]Item, len(it.Items))
	for i := range it.Items {
		items[i] = withSelf(it.Items[i], name)
	}
	if it.Items != nil {
		it.Items = items
	}
	return it
}

// foldRoutine says one routine: its wakes grouped by what they do across
// states, then its rows by table key. Texts keep their short names.
func foldRoutine(g Group) Item {
	base := "behaviorDefinitions." + g.About.ID
	head := Item{Text: g.Title, Base: base}
	type wake struct {
		states []string
		paths  []string
	}
	var order []string
	wakes := map[string]*wake{}
	statePath := map[string]string{}
	var finals []string
	var finalPaths []string
	rows := map[string][]Item{}
	var keys []string
	for _, st := range g.Statements {
		switch st.Kind {
		case KindRoutine:
			head.Text += " — " + st.fold.text
			head.Paths = []string{st.Path}
		case KindRoutineState:
			if st.fold.text == "final" {
				finals = append(finals, quote(st.fold.state))
				finalPaths = append(finalPaths, st.Path)
				continue
			}
			statePath[st.fold.state] = st.Path
		case KindRoutineWake:
			w, ok := wakes[st.fold.text]
			if !ok {
				w = &wake{}
				wakes[st.fold.text] = w
				order = append(order, st.fold.text)
			}
			if p, ok := statePath[st.fold.state]; ok {
				w.paths = append(w.paths, p)
				delete(statePath, st.fold.state)
			}
			w.states = append(w.states, st.fold.state)
			w.paths = append(w.paths, st.Path)
		case KindRoutineRow:
			if _, ok := rows[st.fold.state]; !ok {
				keys = append(keys, st.fold.state)
			}
			rows[st.fold.state] = append(rows[st.fold.state], Item{Text: st.fold.text, Paths: []string{st.Path}})
		}
	}
	states := Item{Text: "States (it is the runner):", Base: base + ".recipe.states."}
	for _, t := range order {
		w := wakes[t]
		states.Items = append(states.Items, Item{Text: fmt.Sprintf("In %s, %s", joinOr(quoteAll(w.states)), t), Paths: w.paths})
	}
	for _, st := range sortedKeys(statePath) {
		states.Items = append(states.Items, Item{Text: fmt.Sprintf("In %s it listens for nothing.", quote(st)), Paths: []string{statePath[st]}})
	}
	if len(finals) > 0 {
		states.Items = append(states.Items, Item{Text: joinAnd(finals) + " final.", Paths: finalPaths})
	}
	if len(states.Items) > 0 {
		head.Items = append(head.Items, states)
	}
	table := Item{Text: "Rows (one whose conditions hold is picked by weight):", Base: base + ".table."}
	for _, k := range keys {
		rs := rows[k]
		if len(rs) == 1 {
			table.Items = append(table.Items, Item{Text: fmt.Sprintf("%s: %s", quote(k), rs[0].Text), Paths: rs[0].Paths})
			continue
		}
		table.Items = append(table.Items, Item{Text: quote(k) + ":", Items: rs})
	}
	if len(table.Items) > 0 {
		head.Items = append(head.Items, table)
	}
	return head
}

// foldHub says a topic or a fact as a short table of places: what grants or
// teaches it, what asks for it, what wakes on it, each place once with the
// creatures it belongs to.
func foldHub(g Group) Item {
	id := g.About.ID
	head := Item{Text: fmt.Sprintf("%s — its meaning is not declared in the file.", quote(id))}
	type table struct {
		order []string
		who   map[string][]string
		paths []string
	}
	tables := map[string]*table{}
	var heads []string
	for _, st := range g.Statements {
		if st.Kind == KindTopic || st.Kind == KindFact {
			head.Paths = []string{st.Path}
			continue
		}
		role := "Asked by"
		for _, c := range st.Connects {
			if c.ID != id || c.Kind != g.About.Kind {
				continue
			}
			switch c.Role {
			case "grants":
				role = "Granted by"
			case "teaches":
				role = "Taught by"
			case "wakes-on":
				role = "Wakes"
			}
		}
		t, ok := tables[role]
		if !ok {
			t = &table{who: map[string][]string{}}
			tables[role] = t
			heads = append(heads, role)
		}
		site := st.fold.site
		if site == "" {
			site = string(st.Kind)
		}
		if _, ok := t.who[site]; !ok {
			t.order = append(t.order, site)
			t.who[site] = nil
		}
		if st.fold.who != "" && !contains(t.who[site], st.fold.who) {
			t.who[site] = append(t.who[site], st.fold.who)
		}
		t.paths = append(t.paths, st.Path)
	}
	for _, role := range []string{"Granted by", "Taught by", "Asked by", "Wakes"} {
		t, ok := tables[role]
		if !ok {
			continue
		}
		var parts []string
		for _, site := range t.order {
			if who := t.who[site]; len(who) > 0 {
				site += " (" + strings.Join(who, ", ") + ")"
			}
			parts = append(parts, site)
		}
		head.Items = append(head.Items, Item{Text: role + ": " + strings.Join(parts, "; ") + ".", Paths: t.paths})
	}
	if len(heads) == 0 {
		head.Text += " Nothing in the file grants it or asks for it."
	}
	return head
}

// FoldedText renders the folded read-back, wrapped for a person: each entry
// followed by its paths in brackets, relative to the entry above it where
// they share its start.
func (r *ReadBack) FoldedText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "READ-BACK: %s (%s)\n", r.Name, r.Key)
	for _, s := range r.Folded() {
		fmt.Fprintf(&sb, "\n%s\n", strings.ToUpper(s.Title))
		for _, it := range s.Items {
			sb.WriteString(ItemText(it))
		}
	}
	sb.WriteString("\nNOT YET SAID\n")
	if len(r.NotYetSaid) == 0 {
		sb.WriteString("  Nothing: every authored thing above is said or summarised.\n")
	}
	for _, u := range r.NotYetSaid {
		sb.WriteString(wrap(fmt.Sprintf("%s (%s): %s", u.Path, u.Kind, u.Why), "  - ", "    "))
	}
	return sb.String()
}

// ItemText renders one top-level folded entry and everything under it.
func ItemText(it Item) string {
	var sb strings.Builder
	sb.WriteString("\n")
	writeItem(&sb, it, "", 1)
	return sb.String()
}

// wrapAt is the folded text's width.
const wrapAt = 110

func writeItem(sb *strings.Builder, it Item, base string, depth int) {
	var shown []string
	seen := map[string]bool{}
	for _, p := range it.Paths {
		if seen[p] {
			continue
		}
		if it.terse && strings.Count(strings.TrimPrefix(p, base), ".") > 1 {
			continue
		}
		seen[p] = true
		switch {
		case base != "" && p == base:
		case base != "" && strings.HasPrefix(p, base):
			shown = append(shown, strings.TrimPrefix(p, base))
		default:
			shown = append(shown, p)
		}
	}
	shown = withoutImplied(shown)
	text := it.Text
	if it.terse && len(shown) == 0 && len(it.Paths) > 0 {
		text += "  [at each line's requires, above]"
	}
	if len(shown) > 0 {
		tail := ""
		if it.terse {
			tail = ", each with all beneath it"
		}
		text += "  [" + compactPaths(shown) + tail + "]"
	}
	indent := strings.Repeat("  ", depth)
	lead := indent + "- "
	if depth == 1 {
		lead = indent
	}
	sb.WriteString(wrap(text, lead, indent+"    "))
	childBase := base
	if it.Base != "" {
		childBase = it.Base
	}
	for _, c := range it.Items {
		writeItem(sb, c, childBase, depth+1)
	}
}

// wrap breaks text into lines of at most wrapAt columns at spaces, the first
// led by lead and the rest by hang. A single word longer than the width
// stands on its own line.
func wrap(text, lead, hang string) string {
	var sb strings.Builder
	line := lead
	fresh := true
	for _, w := range strings.Fields(text) {
		if !fresh && len([]rune(line))+1+len([]rune(w)) > wrapAt {
			sb.WriteString(line + "\n")
			line, fresh = hang, true
		}
		if fresh {
			line += w
			fresh = false
		} else {
			line += " " + w
		}
	}
	sb.WriteString(line + "\n")
	return sb.String()
}

// withoutImplied drops a shown path that another shown path extends: the
// state `ready` is implied by `ready.transitions[0]`. The item's Paths still
// carry both.
func withoutImplied(paths []string) []string {
	var out []string
	for _, p := range paths {
		implied := false
		for _, q := range paths {
			if q != p && (strings.HasPrefix(q, p+".") || strings.HasPrefix(q, p+"[")) {
				implied = true
				break
			}
		}
		if !implied {
			out = append(out, p)
		}
	}
	return out
}

// compactPaths shows paths with their owner said once: full paths that
// share their first four segments (`room.room.monsterBindings.gate-1`) are
// written as that owner followed by each remainder, so a list of a dozen
// places stays a line or two. Relative paths are shown as they are.
func compactPaths(paths []string) string {
	var order []string
	rest := map[string][]string{}
	for _, p := range paths {
		key, tail := p, ""
		if !strings.HasPrefix(p, ".") {
			if parts := strings.SplitN(p, ".", 5); len(parts) == 5 && parts[0] == "room" {
				key, tail = strings.Join(parts[:4], "."), "."+parts[4]
			} else if parts := strings.SplitN(p, ".", 3); len(parts) == 3 && parts[0] != "room" {
				key, tail = strings.Join(parts[:2], "."), "."+parts[2]
			}
		}
		if _, ok := rest[key]; !ok {
			order = append(order, key)
		}
		if tail != "" {
			rest[key] = append(rest[key], tail)
		} else if _, ok := rest[key]; !ok {
			rest[key] = nil
		}
	}
	relative := true
	for _, p := range paths {
		relative = relative && strings.HasPrefix(p, ".")
	}
	if relative {
		return strings.Join(paths, ", ")
	}
	tails := false
	for _, k := range order {
		tails = tails || len(rest[k]) > 0
	}
	if !tails {
		return strings.Join(order, ", ")
	}
	var groups []string
	for _, k := range order {
		if len(rest[k]) == 0 {
			groups = append(groups, k)
			continue
		}
		groups = append(groups, k+" "+strings.Join(rest[k], " "))
	}
	return strings.Join(groups, "; ")
}
