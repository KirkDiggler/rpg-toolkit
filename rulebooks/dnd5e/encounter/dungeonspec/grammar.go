// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// grammar.go is THE ONE GAMEPLAY GRAMMAR, UNDER EITHER GEOMETRY
// (rpg-project#484). The factions, dispositions, answer tables, tempers and
// actions an author writes are judged by the functions here whichever dialect
// drew the room they are in — the v2 document's offset cells and painted
// floor, or the single room's axial hexes.
//
// # Why it is a value and not a receiver on the validator
//
// Every check here used to be a method on [validation], whose receiver is
// built from a v2 [Spec], and the single-room dialect reached them by BUILDING
// A FAKE ONE: a Spec carrying the declared sides and one synthetic PlaceSpec
// per authored monster, each with its cell left at [0,0], plus a string field
// on the validator that stopped the cell rules from ever asking about it. A
// fake owner is the mechanism standing in for the ownership question, and the
// question has one answer: the DIALECT owns cells. The grammar owns none.
//
// So the grammar reads no document at all. It takes exactly two things from
// the dialect that called it:
//
//   - [members], the cast it may name — who exists, what each one is, and
//     which side it is on. A faction's `mind:`, a `{ down: … }` predicate, the
//     faction-of-one rule and an orders block naming nobody are all questions
//     about that index and nothing else.
//   - [cells], the frame a cell or route selector resolves in. The v2
//     dialect looks an offset pair up in the floor it validated; the single
//     room looks an axial `{ q, r }` up in its walkable hexes. Each refuses
//     the other's spelling in one sentence.
//
// A validator that needs a THIRD input from the dialect is a design change,
// not another parameter (rpg-project#484, R1).
//
// # Paths are the contract
//
// Every function takes the path prefix of the thing it is judging, and each
// dialect passes its own: `place[3]` and `factions[0]` in the v2 document,
// `room.room.monsters[3]` and `room.room.monsterBindings.goblin-1` in the
// single room. The World Builder draws each refusal on the field it names, so
// a path that moved would put a refusal on the wrong form row — which is why
// the path is an argument here rather than something rewritten afterwards.
//
// The faction half of the grammar lives in factions.go; this file holds the
// value, the two inputs, and the orders half.

// member is what the grammar reads off one thing a dialect placed: who it is,
// what it is, and which side it is on.
//
// NOTHING GEOMETRIC, deliberately. Where the thing stands, and whether it can
// stand there, is the dialect's own question, asked in the dialect's own
// frame and refused in the dialect's own coordinates.
type member struct {
	// id is the stable name the author gave it, or "" when they gave none.
	id string

	// ref is what it is, as authored — carried for the refusals that name it.
	ref string

	// faction is the side AS AUTHORED: "" means the reserved `monsters`, and
	// [member.side] is the resolved answer.
	faction string
}

// side is the faction this member is on: the one it named, or the reserved
// `monsters`.
func (m member) side() string { return sideOf(m.faction) }

// isMonster reports whether this member is a creature rather than a prop.
//
// A MALFORMED REF IS NOT ONE, and is not reported here: the dialect that read
// the ref has already refused it by name, and asking again would report a
// second defect for the one mistake.
func (m member) isMonster() bool {
	kind, _ := refKind(m.ref)

	return kind == typeMonsters
}

// sideOf is the faction a creature is on, as authored — empty means the
// reserved `monsters`, which a faction block may declare and give orders to
// like any other.
func sideOf(faction string) string {
	if faction == "" {
		return encounter.FactionMonsters
	}

	return faction
}

// members is the cast a dialect placed, in authored order, with every
// declared id indexed to its place in it.
//
// THE INDEX KEEPS THE FIRST of a repeated id, which is the rule the v2
// placement loop reports a duplicate against: the refusal names the line that
// keeps the name, and everything downstream resolves that id to that one
// thing.
type members struct {
	all  []member
	byID map[string]int
}

// newMembers indexes one dialect's cast.
func newMembers(all []member) members {
	byID := make(map[string]int, len(all))
	for i, m := range all {
		if m.id == "" {
			continue
		}
		if _, taken := byID[m.id]; taken {
			continue
		}
		byID[m.id] = i
	}

	return members{all: all, byID: byID}
}

// indexOf is the position an id names, and whether anything was declared
// under it.
func (m members) indexOf(id string) (int, bool) {
	i, ok := m.byID[id]

	return i, ok
}

// at is the member an id names, and whether anything was declared under it.
func (m members) at(id string) (member, bool) {
	i, ok := m.byID[id]
	if !ok {
		return member{}, false
	}

	return m.all[i], true
}

// authoredCell is one cell a selector names, in whichever spelling the author
// wrote: an offset pair (the v2 frame) or an axial `{ q, r }` (the single
// room's). Exactly one is set. Which spelling is LEGAL is the dialect's
// answer, never the grammar's ([cells]).
//
// facing and wait are what a route's waypoint states for its pause, or ""
// and 0 — only a waypoint carries them (rpg-project#624).
type authoredCell struct {
	offset *[2]int
	hex    *RoomCell
	facing string
	wait   int
}

// selectorCell is the one cell a selector names, and whether it names one.
func selectorCell(sel *SelectorSpec) (authoredCell, bool) {
	switch {
	case sel.AtHex != nil:
		return authoredCell{hex: sel.AtHex}, true
	case sel.At != nil:
		return authoredCell{offset: sel.At}, true
	default:
		return authoredCell{}, false
	}
}

// selectorRoute is the route a selector names, each waypoint in its authored
// spelling. A route that mixes the two spellings is refused whole by the
// dialect for the spelling that is not its own, so the order the two halves
// come back in here never reaches an author.
func selectorRoute(sel *SelectorSpec) []authoredCell {
	out := make([]authoredCell, 0, len(sel.Path)+len(sel.PathHex))
	for i := range sel.Path {
		out = append(out, authoredCell{offset: &sel.Path[i]})
	}
	for i := range sel.PathHex {
		waypoint := authoredCell{hex: &sel.PathHex[i]}
		if i < len(sel.PathHexFacing) {
			waypoint.facing = sel.PathHexFacing[i]
		}
		if i < len(sel.PathHexWait) {
			waypoint.wait = sel.PathHexWait[i]
		}
		out = append(out, waypoint)
	}

	return out
}

// cellAnswer is what a dialect says about the cell a selector named.
type cellAnswer struct {
	// framed is whether the authored cell is spelt in this dialect's frame
	// at all. When it is false the refusal is the WHOLE defect and the word
	// carrying the cell is never judged: the spelling is the mistake, and
	// two messages for one mistake sends an author looking for a second
	// problem.
	framed bool

	// refusal is why the cell is not somewhere to walk to, or "" when it is.
	refusal string

	// routeRefusal is the dialect's sentence for a `path` with a waypoint
	// spelt in the other dialect's frame — said once for the route rather
	// than once per waypoint. Empty when framed.
	routeRefusal string

	// cell is the dungeon-absolute cell the authored one names, when framed:
	// what a route's legs are drawn between.
	cell spatial.Position
}

// cells is the dialect's frame: what an authored cell inside a selector
// means, whether a creature can be sent there, and how a cell the engine
// computed is written back in the author's own coordinates.
//
// THE GRAMMAR OWNS NO CELLS (rpg-project#484, R2). The v2 dialect resolves an
// offset pair through the orientation its document declares and looks it up
// in the floor its regions painted ([floorCells]). The single room declares
// no orientation; its frame is the axial `{ q, r }` its walkable hexes are
// written in, and a cell must be one of them ([singleRoomCells];
// rpg-project#605). Each refuses the other's spelling, so the same authored
// bytes never name two different cells in the two dialects.
//
// The route rules — a leg off the floor, two legs sharing a cell — are the
// grammar's ([grammar.routeLegs]), asked through [cells.onFloor] and spoken
// through [cells.spell], so both dialects keep them in one statement.
type cells interface {
	cellAt(at authoredCell) cellAnswer
	onFloor(cell spatial.Position) bool
	spell(cell spatial.Position) string
}

// floorCells is the v2 dialect's frame: the orientation the document declared,
// and the owned floor its regions painted.
type floorCells struct {
	orientation encounter.Orientation
	owner       map[spatial.Position]int
}

// The v2 dialect's refusals of the single room's spelling. A v2 file that
// writes `{ q, r }` used to fail at decode in the parser's words; it decodes
// now, because the decoder cannot tell the dialects apart, and this is the
// dialect saying which spelling is its own.
const (
	v2AxialCell = "at names a cell of this document in its orientation, written [col, row]: " +
		"{ q, r } is the single-room dialect's axial cell"
	v2AxialRoute = "path names a route of this document's cells, each written [col, row]: " +
		"{ q, r } is the single-room dialect's axial cell"
)

// cellAt resolves an authored pair against that floor, refusing a cell nobody
// could walk to in the author's own coordinates.
func (f floorCells) cellAt(at authoredCell) cellAnswer {
	if at.offset == nil {
		return cellAnswer{refusal: v2AxialCell, routeRefusal: v2AxialRoute}
	}
	cell := encounter.HexCellAt(f.orientation, at.offset[0], at.offset[1])
	if !f.onFloor(cell) {
		return cellAnswer{framed: true, cell: cell,
			refusal: fmt.Sprintf("this walks to %s, which is not floor", f.spell(cell))}
	}

	return cellAnswer{framed: true, cell: cell}
}

// onFloor reports whether a region painted the cell.
func (f floorCells) onFloor(cell spatial.Position) bool {
	_, ok := f.owner[cell]

	return ok
}

// forms is how a v2 author spells a cell and a route, for a refusal's hint.
func (floorCells) forms() cellForms {
	return cellForms{cell: "[col, row]", route: "[[col, row], …]"}
}

// spell writes a cell as the v2 author did: an offset pair.
func (f floorCells) spell(cell spatial.Position) string {
	col, row := encounter.OffsetOf(f.orientation, cell)

	return fmt.Sprintf("[%d,%d]", col, row)
}

// grammarInput is what a dialect hands the grammar: where defects go, the
// site scope it read, the cast it placed, and the frame it draws cells in.
type grammarInput struct {
	// Add is where a defect goes. Each dialect passes its own sink, so every
	// refusal lands in the list its author is shown, in the order it was
	// found.
	Add errSink

	// Factions and Dispositions are the site scope as the dialect read it —
	// the same two shapes in both, by construction ([FactionSpec],
	// [DispositionSpec]).
	Factions     []FactionSpec
	Dispositions []DispositionSpec

	// Tables is the site's root answer tables, by id (rpg-toolkit#1897). The
	// grammar needs them to judge a NAME — a faction's `table:` and a
	// binding's — and judging a name is all it does with them: the tables
	// themselves are judged by [siteGrammar]'s own loop, once each.
	Tables map[string]TableSpec

	// Members is the cast; Cells is the frame.
	Members members
	Cells   cells

	// Facts is every fact id this document can set: what its rows teach and
	// what its records reveal (rpg-project#670, #684 L3). A `fact` pattern
	// in a `when` that matches none of them is a condition that can never
	// hold, and is refused. [factsIn] builds it.
	Facts map[string]bool

	// Bundled says the dialect resolved `use:` rows against its `bundles:`
	// ([resolveBundles]) before this grammar runs, and reported every name
	// it could not. A dialect with no `bundles:` leaves it false, and a
	// `use:` row there is refused here.
	Bundled bool
}

// grammar judges one document's gameplay vocabulary, reporting every defect
// at the path the dialect named it by.
type grammar struct {
	add errSink

	// tables is what a `table:` name may resolve to.
	tables map[string]TableSpec

	declaredFactions     []FactionSpec
	declaredDispositions []DispositionSpec

	members members
	cells   cells

	// factionIDs is every declared faction id to the index that declared it
	// — built by factions(), read by placeFaction(), dispositions() and
	// whatever else asks whether a side exists. factionMembers is every
	// faction, declared or reserved, to the members in it — built by
	// placeFaction(), read by minds() and dispositions() for the
	// faction-of-one rule. mindValid is every faction whose declared mind
	// passed minds(). dispositionAt is every normalized pair to the
	// disposition that speaks for it (rpg-project#375).
	factionIDs     map[string]int
	factionMembers map[string][]int
	mindValid      map[string]bool
	dispositionAt  map[[2]string]int

	// facts and bundled are [grammarInput.Facts] and [grammarInput.Bundled].
	facts   map[string]bool
	bundled bool
}

// newGrammar builds the grammar one dialect's document is judged by.
func newGrammar(in grammarInput) *grammar {
	return &grammar{
		add:                  in.Add,
		declaredFactions:     in.Factions,
		declaredDispositions: in.Dispositions,
		members:              in.Members,
		cells:                in.Cells,
		tables:               in.Tables,
		facts:                in.Facts,
		bundled:              in.Bundled,
	}
}

// fail reports one defect at its path, in the words the author reads.
func (g *grammar) fail(path, format string, args ...any) {
	g.add(path, fmt.Sprintf(format, args...))
}

// placeOn validates an answer table an author wrote — a creature's own
// ([PlaceSpec.On], [RoomMonsterBinding.On]) or the one its faction hands it
// ([FactionSpec.On], rpg-project#458).
//
// THE KEY IS THE TRIGGER, and only `time` is accepted. The four social keys
// that retired to options (rpg-project#569 R2) are refused naming where they
// went; a word the design NAMES and has not built is refused by name in the
// entry's own decoder ([laterWords]); a key nobody designed is refused here,
// listing what there is. Either way the author finds out on the form instead
// of at the table.
//
// Refusals per entry, each its own sentence:
//
//   - a weight below 1, which is a row that can never fire;
//   - two outcome words in one entry, so ordering never has to be guessed;
//   - an entry with no word and nothing to say, which is a row written for no
//     reason;
//   - an empty `fact:`, which says the world learns something and not what;
//   - a word under a key it is not legal on: `flee` answers a social verdict,
//     `hold`/`attack`/`toward`/`away` are what a creature does with time, and
//     a `when` is a time word because a social key IS the condition; `fact`
//     is legal under both (rpg-project#569 R6: on `time` it is an alarm,
//     taught to whoever hears the speaker);
//   - `at:` or `path:` on anything but `toward`, because walking away from a
//     fixed cell is a direction rather than a flight and nothing has paid for
//     one, and a route is walked;
//   - `sound` on `attack`, because a sound cannot be struck, and `sound` on a
//     row with no `heard` condition — mind/behavior's own refusals, asked of
//     the compiled row ([grammar.entryShape]);
//   - `actor` in an entry whose `when` names no deed, because there is no
//     actor otherwise;
//   - an `at:` cell the dialect cannot send anybody to: not floor in a dialect
//     that has a frame, and not nameable at all in one that has none;
//   - a trigger key with no entries at all, which is a table that cannot be
//     rolled.
//
// WHAT IS NOT CHECKED IS THE FACT'S MEMBERSHIP. The dungeon ALLOWS a fact
// nothing else mentions (R8, pre-release: show the cost) — a `fact` that no
// disposition waits for is a cost, not a defect, and it joins the run's
// mintable facts so a world blob may name it.
func (g *grammar) placeOn(path string, on map[string][]AnswerSpec) {
	for _, key := range sortedKeys(on) {
		at := fmt.Sprintf("%s.on.%s", path, key)
		if encounter.IsRetiredTableKey(encounter.AnswerKey(key)) {
			g.fail(at, "%s", retiredTableKey(key))
			continue
		}
		if !knownTableKey(key) {
			g.fail(at, "%q is not a trigger this build rolls: they are %s",
				key, strings.Join(tableKeyWords(), ", "))
			continue
		}
		entries := on[key]
		if len(entries) == 0 {
			g.fail(at, "this names a trigger and lists nothing that happens on it")
			continue
		}
		for j, entry := range entries {
			g.answerEntry(fmt.Sprintf("%s[%d]", at, j), key, entry)
		}
	}
}

// retiredTableKey is the sentence one of the four retired social keys is
// refused with, naming where it went (rpg-project#569 R2) — [knowsRefusal]'s
// courtesy, for a key this build once understood.
func retiredTableKey(key string) string {
	return fmt.Sprintf("`%s` is no longer a table key: author an option", key)
}

// The sentences an option is refused with (rpg-project#569). Constant, because
// the World Builder draws each on a form row and a second spelling of any is
// the drift one grammar exists to prevent.
const (
	errOptionNoID      = "the option has no id"
	errOptionNoSay     = "the option does not say what the character says"
	errOptionNoCheck   = "this option needs at least one way to make its check — an ability and a DC"
	errOptionPropFlees = "a prop cannot run: `flee` is a creature's answer"
)

// options validates the lines an author wrote on a creature or a prop
// ([OptionSpec], rpg-project#569), at `<path>.options[i]`.
//
// Refusals per option, each its own sentence: no id; an id already used on
// this binding; no line; a check with no way through, or an approach that
// rolls nothing or has nothing to beat ([grammar.approaches], the lock's own
// grammar); a `requires` that is neither `{ fact }` nor `{ holds }`; a `holds`
// naming a prop this dungeon does not place, or one nobody can pick up; a
// verdict written and left empty; and every row by the table's own grammar
// ([grammar.answerEntry]) under the verdict's key — so `fact`, `flee` and a
// line are legal, a time word and a `when` are not.
//
// props is the dialect's placed props, id to whether somebody can pick it up
// — the universe a `holds` may name. isProp says the options are a prop's,
// whose rows may not `flee`.
func (g *grammar) options(path string, options []OptionSpec, props map[string]bool, isProp bool) {
	seen := map[string]int{}
	for i, o := range options {
		p := fmt.Sprintf("%s.options[%d]", path, i)
		switch prev, dup := seen[o.ID]; {
		case o.ID == "":
			g.fail(p+".id", "%s", errOptionNoID)
		case dup:
			g.fail(p+".id", "option %q is already declared at options[%d]", o.ID, prev)
		default:
			seen[o.ID] = i
		}
		if o.Say == "" {
			g.fail(p+".say", "%s", errOptionNoSay)
		}
		g.approaches(p+".check", errOptionNoCheck, o.Check)
		g.optionRequires(p+".requires", o.Requires, props)
		for _, verdict := range []struct {
			key  string
			rows []AnswerSpec
		}{{string(encounter.AnswerSuccess), o.Success}, {string(encounter.AnswerFailure), o.Failure}} {
			if verdict.rows == nil {
				continue
			}
			at := p + "." + verdict.key
			if len(verdict.rows) == 0 {
				g.fail(at, "`%s` lists nothing — omit it, or write what is answered", verdict.key)
				continue
			}
			for j, row := range verdict.rows {
				rowAt := fmt.Sprintf("%s[%d]", at, j)
				g.answerEntry(rowAt, verdict.key, row)
				if isProp && row.Flee != nil {
					g.fail(rowAt+".flee", "%s", errOptionPropFlees)
				}
			}
		}
	}
}

// optionRequires judges one option's `requires`: absent, a fact, or a holding
// of a placed prop somebody can pick up — and nothing else, because an option
// is offered or not by what the SPEAKER knows and carries (R9).
func (g *grammar) optionRequires(path string, p *PredicateSpec, props map[string]bool) {
	switch form := p.Form(); form {
	case "", predicateFact:
	case predicateHolds:
		holdable, placed := props[p.Holds]
		switch {
		case !placed:
			g.fail(path+".holds", "%q is not a prop in this dungeon, so nobody can hold it", p.Holds)
		case !holdable:
			g.fail(path+".holds", "%q is a prop nobody can pick up, so nobody can hold it", p.Holds)
		}
	default:
		g.fail(path, "an option requires { fact: <id> } or { holds: <prop id> }, and `%s` is neither", form)
	}
}

// answerEntry validates one row of one trigger's table.
func (g *grammar) answerEntry(at string, key string, entry AnswerSpec) {
	if entry.Weight != nil && *entry.Weight < 1 {
		g.fail(at+".weight", "a weight of %d can never be rolled: omit it for 1, or give it a share",
			*entry.Weight)
	}

	// A GROUP ROW IS ITS OWN SHAPE (rpg-project#670): no word of its own,
	// and its rows judged one by one beneath it.
	if entry.Use != nil {
		g.groupEntry(at, key, entry)

		return
	}
	if entry.With != nil {
		g.fail(at+".with", "%s (line %d)", errWithWithoutUse, entry.Line)
	}
	if entry.As != nil {
		g.fail(at+".as", "%s (line %d)", errAsWithoutUse, entry.Line)
	}

	// An empty `fact:` is its own sentence rather than a missing word: the
	// author wrote the key, so they meant to teach something. Reported
	// INSTEAD of the no-word refusal below, not beside it — two defects for
	// one mistake sends an author looking for a second problem.
	if entry.Fact != nil && *entry.Fact == "" {
		g.fail(at+".fact", "this says the world learns something and does not say what")

		return
	}
	// A ROW TEACHES ONE FACT, BY NAME (rpg-project#684 L2): a pattern is a
	// value of a `when` kind, matching names already authored, and a fact
	// the world learns has to be one.
	if entry.Fact != nil {
		if _, pattern := patternOf(*entry.Fact); pattern {
			g.fail(at+".fact", "a row teaches one fact by its name, and %s is a pattern: a pattern only "+
				"matches facts, in a `when` (line %d)", *entry.Fact, entry.Line)

			return
		}
	}
	g.whenPatterns(at, entry.When)
	g.whenWho(at, entry.When)

	words := entryWords(entry)
	switch {
	case len(words) > 1:
		g.fail(at, "an entry does one thing: `%s` in the same entry is %d (line %d)",
			strings.Join(words, "` and `"), len(words), entry.Line)

		return
	case len(words) == 0 && entry.Say == "":
		g.fail(at, "this entry does nothing and says nothing (line %d)", entry.Line)

		return
	}

	g.wordLegality(at, key, words, entry)
	g.entrySelector(at, key, entry)
	g.entryShape(at, entry)
}

// groupEntry judges a `use:` row (rpg-project#670): it carries no word, no
// `search`, no `say` and no `fact` of its own; its `as` and its own `when`
// are judged as a group's; and every row the bundle gave it is judged under
// the same key, at `<at>.rows[j]`, as any row is.
//
// THE NAME AND THE PARAMETERS WERE JUDGED ALREADY ([resolveBundles]), at the
// same paths, so a group the resolver could not fill is not judged again
// here: one mistake, one defect.
func (g *grammar) groupEntry(at, key string, entry AnswerSpec) {
	if !g.bundled {
		g.fail(at+".use", "`use: %s` names a bundle, and only a version 4 document declares `bundles:` "+
			"(line %d)", *entry.Use, entry.Line)

		return
	}
	if len(entry.When) > 0 && key != string(encounter.AnswerTime) {
		g.fail(at+".when",
			"`%s` is already the condition — a `when` under it asks when a thing that just happened happened (line %d)",
			key, entry.When[0].Line)
	}
	g.whenPatterns(at, entry.When)
	g.whenWho(at, entry.When)
	if entry.Rows == nil {
		return
	}
	g.entryShape(at, entry)
	for j, row := range entry.Rows {
		g.answerEntry(fmt.Sprintf("%s.rows[%d]", at, j), key, row)
	}
}

// whenPatterns refuses a `fact` pattern, anywhere in a row's `when` (a
// `not` included), that matches no fact this document can set
// (rpg-project#684 L3). A `heard` pattern was judged at decode, against the
// sounds a file can make. Whether a pattern compiles is mind/behavior's
// sentence ([grammar.entryShape]); one that does not is not judged here.
func (g *grammar) whenPatterns(at string, list WhenListSpec) {
	for i := range list {
		for w := &list[i]; w != nil; w = w.Not {
			pattern, ok := patternOf(w.Fact)
			if !ok {
				continue
			}
			if pattern == "" {
				g.fail(at+".when", "`fact: //` is a pattern between empty slashes, and names no fact (line %d)",
					w.Line)
				continue
			}
			if _, err := regexp.Compile(pattern); err != nil {
				continue
			}
			if !g.factMatches(nameOf(w.Fact)) {
				g.fail(at+".when", "`fact` %s matches no fact this file sets (line %d)", w.Fact, w.Line)
			}
		}
	}
}

// whenWho refuses a `fell: { who }`, anywhere in a row's `when` (a `not`
// included), that does not name a monster placed in this file
// (rpg-project#716): `who` is the one whose fall the row waits on, and only a
// monster can be down. `who` on any other deed was refused at decode.
func (g *grammar) whenWho(at string, list WhenListSpec) {
	for i := range list {
		for w := &list[i]; w != nil; w = w.Not {
			if w.Who == "" {
				continue
			}
			m, ok := g.members.at(w.Who)
			if !ok {
				g.fail(at+".when", "`%s: { who: %s }`: %q is not a placement in this dungeon (line %d)",
					w.Deed, w.Who, w.Who, w.Line)
				continue
			}
			if !m.isMonster() {
				g.fail(at+".when", "`%s: { who: %s }`: %q is a prop, and only a monster can fall (line %d)",
					w.Deed, w.Who, w.Who, w.Line)
			}
		}
	}
}

// factMatches reports whether a pattern matches a fact this document sets.
func (g *grammar) factMatches(name behavior.Name) bool {
	for id := range g.facts {
		if name.Matches(id) {
			return true
		}
	}

	return false
}

// entryShape asks mind/behavior the questions it owns about this row — a
// `path` only on `toward`, `sound` never on `attack` and only on a row that
// listens with `heard`, one target per selector, one condition per `when`
// entry — and reports its sentence at the entry's path.
//
// ONE STATEMENT OF EACH RULE. The composition's own door asks the same
// [behavior.Answer.Validate] ([encounter] validateEntry), so the form and the
// table cannot come to disagree about what a row may say; this file adds only
// what a dialect knows that the evaluator cannot — key legality, the frame a
// cell is drawn in, and the sealed word lists. The sentinel's wording is
// trimmed off: it names a Go package, and the form shows an author a sentence.
func (g *grammar) entryShape(at string, entry AnswerSpec) {
	compiled := answerOf(entry, encounter.HexesArePointyTop())
	if entry.Use != nil {
		// A GROUP IS ASKED AS WRITTEN: any word the `use:` row carries is
		// compiled beside its rows, so mind/behavior refuses the group that
		// carries one in its own sentence. Its rows are judged one by one at
		// their own paths ([grammar.groupEntry]), so here each stands in as a
		// bare hold: an inner row's fault is reported once, at the row.
		written := entry
		written.Use = nil
		compiled = answerOf(written, encounter.HexesArePointyTop())
		compiled.Rows = make([]encounter.Answer, len(entry.Rows))
		for i := range compiled.Rows {
			compiled.Rows[i] = encounter.Answer{Weight: 1, Hold: true}
		}
		if entry.As != nil {
			compiled.As = *entry.As
		}
		compiled.From = entry.From
	}
	err := compiled.Validate()
	if err == nil {
		return
	}
	sentence := err.Error()
	for _, sentinel := range []error{
		behavior.ErrBadSelector, behavior.ErrBadWhen, behavior.ErrBadSearch, behavior.ErrBadGroup,
	} {
		sentence = strings.TrimSuffix(sentence, ": "+sentinel.Error())
	}
	g.fail(at, "%s (line %d)", sentence, entry.Line)
}

// wordLegality refuses a word, or a `when`, under a key it is not legal on.
// The key is `time` for a table's row and `success` or `failure` for an
// option's — a verdict, where `fact` and `flee` are legal and the four time
// words and a `when` are not. `fact` is legal under `time` too
// (rpg-project#569 R6).
func (g *grammar) wordLegality(at, key string, words []string, entry AnswerSpec) {
	time := key == string(encounter.AnswerTime)
	for _, word := range words {
		switch word {
		case "fact":
		case "flee":
			if time {
				g.fail(at+"."+word,
					"`%s` answers a social verdict, and `time` is not one (line %d)", word, entry.Line)
			}
		default:
			if !time {
				g.fail(at+"."+word,
					"`%s` is what a creature does with time, and `%s` is an outcome (line %d)",
					word, key, entry.Line)
			}
		}
	}
	if len(entry.When) > 0 && !time {
		g.fail(at+".when",
			"`%s` is already the condition — a `when` under it asks when a thing that just happened happened (line %d)",
			key, entry.When[0].Line)
	}
}

// entrySelector refuses a selector that names a cell where only a member can
// stand, an `actor` with no deed to have been the actor of, and a cell the
// dialect cannot send anybody to.
//
// THE FRAME COMES FIRST, and a cell spelt in the other dialect's frame is the
// whole answer: the authored cell is refused outright and the word carrying
// it is never judged, because the spelling is the mistake. A cell spelt in
// this dialect's frame judges the word next — a cell is somewhere to walk
// toward, never something to attack — and only then asks whether that cell
// is floor.
func (g *grammar) entrySelector(at, _ string, entry AnswerSpec) {
	word, sel := entrySelectorOf(entry)
	if sel == nil {
		return
	}
	if route := selectorRoute(sel); len(route) > 0 {
		g.entryRoute(at, word, route)

		return
	}
	if cell, ok := selectorCell(sel); ok {
		answer := g.cells.cellAt(cell)
		if !answer.framed {
			g.fail(at+"."+word+".at", "%s", answer.refusal)

			return
		}
		if !walksTo(word) {
			g.fail(at+"."+word,
				"a cell is somewhere to walk toward, and `%s` acts on a creature (line %d)", word, sel.Line)

			return
		}
		if answer.refusal != "" {
			g.fail(at+"."+word+".at", "%s", answer.refusal)
		}

		return
	}
	if sel.Word == string(encounter.SelectorActor) && entry.When.deed() == nil {
		g.fail(at+"."+word,
			"`actor` is the actor of the deed this entry's `when` names, and this entry names no deed (line %d)",
			sel.Line)
	}
}

// entryRoute refuses a `path` the dialect cannot walk: a waypoint spelt in
// the other dialect's frame, or one that is not floor — the frame and floor
// questions [grammar.entrySelector] asks of an `at`, asked of every waypoint.
// A route on a word that does not walk one is mind/behavior's refusal
// ([grammar.entryShape]), so its waypoints are not judged as well: one
// mistake, one defect.
func (g *grammar) entryRoute(at, word string, route []authoredCell) {
	path := at + "." + word + ".path"
	answers := make([]cellAnswer, 0, len(route))
	for _, waypoint := range route {
		answer := g.cells.cellAt(waypoint)
		if !answer.framed {
			g.fail(path, "%s", answer.routeRefusal)

			return
		}
		answers = append(answers, answer)
	}
	if !walksTo(word) {
		return
	}
	floorDefect := false
	cells := make([]spatial.Position, 0, len(answers))
	for i, answer := range answers {
		if answer.refusal != "" {
			g.fail(fmt.Sprintf("%s[%d]", path, i), "%s", answer.refusal)
			floorDefect = true
		}
		cells = append(cells, answer.cell)
		// A WAYPOINT'S FACING IS A PLACEMENT'S WORD (rpg-project#624), refused
		// in the placement's sentence, so an author meets one vocabulary.
		if word := route[i].facing; word != "" && !facings[word] {
			g.fail(fmt.Sprintf("%s[%d].facing", path, i),
				"%q is not a compass direction: a facing is one of n|ne|e|se|s|sw|w|nw", word)
		} else if word != "" && route[i].wait == 0 {
			// NO WAIT, NO PAUSE (lead ruling on rpg-toolkit#2087): the
			// facing is the stand's, so it is refused alone.
			g.fail(fmt.Sprintf("%s[%d].facing", path, i),
				"a facing is taken while standing; give the waypoint a wait")
		}
		if wait := route[i].wait; wait < 0 {
			g.fail(fmt.Sprintf("%s[%d].wait", path, i),
				"a wait is how many rounds the walker stands here, and %d is negative", wait)
		}
	}
	if floorDefect {
		return
	}
	g.routeLegs(path, cells)
}

// routeLegs asks the two questions a route's LEGS raise, once its waypoints
// are floor (rpg-project#569 R7): does a leg cross off the floor, and does the
// route retrace itself. A leg is the hex line the creature walks between two
// waypoints ([encounter.RetracedCell]'s legs), so a wall of void across that
// line is a patrol that would stop dead there, and two legs sharing a cell is
// a creature on both. Both are named in the author's own coordinates, through
// the dialect's [cells.spell], and asked of the dialect's own floor — the
// rules live here, below either dialect, so they are the same in both.
func (g *grammar) routeLegs(path string, cells []spatial.Position) {
	for i := range cells {
		for _, cell := range encounter.LegLine(cells[i], cells[(i+1)%len(cells)]) {
			if !g.cells.onFloor(cell) {
				g.fail(path, "leg %d of this route walks through %s, which is not floor: "+
					"move a waypoint so the line between them stays on the floor", i+1, g.cells.spell(cell))

				return
			}
		}
	}
	if cell, retraced := encounter.RetracedCell(cells); retraced {
		g.fail(path, "this route retraces itself at %s: a patrol walks a loop — "+
			"add a waypoint so no leg crosses another", g.cells.spell(cell))
	}
}

// walksTo reports whether a row word walks to a place — a cell or a route —
// rather than acting on a creature: `toward`, and `dash`, which is a run
// toward (rpg-project#671, R17).
func walksTo(word string) bool {
	return word == "toward" || word == "dash"
}

// entryWords is the outcome words this entry carries, in the order the design
// lists them.
func entryWords(entry AnswerSpec) []string {
	var out []string
	if entry.Fact != nil {
		out = append(out, "fact")
	}
	if entry.Flee != nil {
		out = append(out, "flee")
	}
	if entry.Hold != nil {
		out = append(out, "hold")
	}
	if entry.Attack != nil {
		out = append(out, "attack")
	}
	if entry.Toward != nil {
		out = append(out, "toward")
	}
	if entry.Dash != nil {
		out = append(out, "dash")
	}
	if entry.Away != nil {
		out = append(out, "away")
	}
	if entry.Search != nil {
		out = append(out, "search")
	}

	return out
}

// entrySelectorOf is the entry's selector and the word carrying it.
func entrySelectorOf(entry AnswerSpec) (string, *SelectorSpec) {
	switch {
	case entry.Attack != nil:
		return "attack", entry.Attack
	case entry.Toward != nil:
		return "toward", entry.Toward
	case entry.Dash != nil:
		return "dash", entry.Dash
	case entry.Away != nil:
		return "away", entry.Away
	default:
		return "", nil
	}
}

// The two sentences a door's authored checks are refused empty with. Constant
// and shared, because a lock nobody can pick means the same thing to an author
// whichever dialect drew the room — and a second spelling of either is exactly
// the drift one grammar exists to prevent (rpg-project#484).
const (
	errLockNoWayThrough     = "this locked door needs at least one way through it — an ability and a DC"
	errConcealedNoWayToFind = "this concealed door needs at least one way to find it — an ability and a DC"
)

// approaches validates one authored check: at least one approach, each naming
// the ability it rolls and a DC of at least 1.
//
// THE EMPTY-CHECK SENTENCE IS THE CALLER'S, worded for the form-filler at the
// door, since "the check has no approaches" means one thing on a lock and
// another on a concealment; every per-approach refusal names the field that is
// missing at the row that misses it.
//
// ONE CHECK GRAMMAR, UNDER EITHER GEOMETRY. A v2 door is a position on a wall
// and a single-room door is a footprint, and the lock on them is the same lock
// — so this moved here from [validation] when the second dialect grew one
// (rpg-project#485, R2), rather than being written a second time beside the
// second door.
func (g *grammar) approaches(path, none string, check CheckSpec) {
	if len(check) == 0 {
		g.fail(path, "%s", none)

		return
	}
	for j, a := range check {
		ap := fmt.Sprintf("%s[%d]", path, j)
		if a.Ability == "" {
			g.fail(ap+".ability", "the approach does not say which ability it rolls")
		}
		if a.DC < 1 {
			g.fail(ap+".dc", "an approach with dc %d has nothing to beat", a.DC)
		}
	}
}

// doorState validates the state half of an authored door — the keys a v2
// [DoorSpec] and a single-room [RoomDoorBinding] share, at whichever path the
// dialect names them by.
//
// WHAT IT DOES NOT ASK IS WHERE THE DOOR IS. A position on a wall and a
// footprint are the two geometries (encounter.DoorInput), and each dialect
// refuses its own in its own coordinates — the grammar owns no geometry for
// the reason it owns no cells.
func (g *grammar) doorState(path string, locked, concealed CheckSpec) {
	if locked != nil {
		g.approaches(path+".locked", errLockNoWayThrough, locked)
	}
	if concealed != nil {
		g.approaches(path+".concealed", errConcealedNoWayToFind, concealed)
	}
}

// doorOpenedNoFact is the refusal for an `opened` that does not say what the
// door teaches (rpg-project#569 R12) — the key missing, or written empty.
const doorOpenedNoFact = "opening this door teaches a fact, and this does not say which: " +
	"write the fact's id, like `fact: cells-opened`"

// doorOpened judges a door's `opened` (rpg-project#569 R12) in either
// dialect, at the door's own path: present means it names a fact. A fact id
// is declared by mention ([PredicateSpec.Fact]) — there is no id grammar
// beyond being written, the same rule `reveals: { fact }` and an answer row's
// `fact` keep — so the one refusal is a fact that is not there.
func (g *grammar) doorOpened(path string, opened *OpenedSpec) {
	if opened == nil {
		return
	}
	if opened.Fact == "" {
		g.fail(path+".opened.fact", "%s", doorOpenedNoFact)
	}
}

// placeTemper refuses a temperament this build does not ship. The decoder
// already refuses an unknown word inside a [TemperSpec]; a creature's own
// `temper` is a plain word in either dialect, so this is where it is checked.
func (g *grammar) placeTemper(path, temper string) {
	if temper == "" {
		return
	}
	if !encounter.ValidTemperWord(temper) {
		g.fail(path+".temper", "%q is not a temperament this build ships: they are %s",
			temper, strings.Join(encounter.TemperWords, ", "))
	}
}

// factionOrders validates every declared faction's inherited `on:` block — the
// same refusals a creature's own earns, at the layer above (design §1,
// layer 2), and at `factions[i]` in either dialect.
func (g *grammar) factionOrders() {
	for i, fa := range g.declaredFactions {
		g.placeOn(fmt.Sprintf("factions[%d]", i), fa.On)
	}
}

// factionOptions validates every declared faction's lines ([FactionSpec.Options],
// rpg-project#718) by the one option grammar a creature's own lines get, at
// `factions[i].options[j]` in either dialect: ids unique WITHIN the faction,
// a check with a way through, a `requires` of a fact or a holding of one of
// props, and both verdicts' rows by the table's own grammar.
//
// A creature's lines, never a prop's: a faction's members are monsters, so a
// `flee` row is legal here exactly as it is on a binding.
func (g *grammar) factionOptions(props map[string]bool) {
	for i, fa := range g.declaredFactions {
		g.options(fmt.Sprintf("factions[%d]", i), fa.Options, props, false)
	}
}

// knownTableKey reports whether a key is one the composition rolls.
func knownTableKey(key string) bool {
	for _, known := range encounter.TableKeys {
		if key == string(known) {
			return true
		}
	}

	return false
}

// tableKeyWords is every trigger key as a string, for a refusal that lists
// what there is.
func tableKeyWords() []string {
	out := make([]string, 0, len(encounter.TableKeys))
	for _, k := range encounter.TableKeys {
		out = append(out, string(k))
	}

	return out
}

// sortedKeys orders a map's keys so a file with two bad `on:` entries reports
// them in the same order every run — a validator whose defect list depends on
// Go's map iteration is one no transcript can compare (C8).
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)

	return out
}

// # What the room is for: the way out, the ending, and the binding
//
// The three root keys below are the last of the v2 root to reach the
// single-room dialect (rpg-project#488, slice 1), and they reach it as these
// functions rather than as a second spelling. `exits:`, `endings:` and
// `scenarios:` mean the same thing in both documents and are authored under
// the same paths, so every sentence an author reads about one is written
// here once. What a dialect keeps for itself is named at each function, and
// there is exactly one such thing: the frame an exit's cell is in.

// exitIDs is one document's ways out, indexed as they are read: every
// declared id to the position that declared it. It is also the universe a
// scenario binding may name an exit from.
type exitIDs map[string]int

// declare judges one authored way out's id and indexes it — the two things an
// exit must get right WHATEVER FRAME ITS CELL IS IN ([ExitSpec.ID],
// [RoomExit.ID]): it has an id, and no exit before it had that id. A binding
// that named an ambiguous exit would have no answer.
//
// THE CELL IS THE DIALECT'S OWN, and is deliberately not asked about here.
// v2 resolves an absolute [col,row] against the floor its regions painted;
// the single room puts axial {q, r} through the same standability its
// `partyStart` goes through (rpg-project#488 R2). One answer per frame, at
// the exit's own path, in words about what that frame has under the cell.
//
// Called from inside each dialect's own walk rather than over the whole list,
// so an author reads one exit's defects together instead of every id followed
// by every cell.
func (x exitIDs) declare(g *grammar, index int, id string) {
	p := fmt.Sprintf("exits[%d].id", index)
	switch prev, dup := x[id]; {
	case id == "":
		g.fail(p, "the exit has no id")
	case dup:
		g.fail(p, "exit %q is already declared at exits[%d]", id, prev)
	default:
		x[id] = index
	}
}

// endings judges the authored ways a run can end (rpg-project#375, R10): an
// id, no two alike, and a predicate that is one and can hold — "an ending
// nobody can reach" refused in the file's own path exactly as the run refuses
// it at construction ([encounter.ErrNoEnding]).
//
// NOTHING HERE IS A DIALECT'S OWN. An ending names no cell and needs no
// frame, so `endings:` is [EndingSpec] verbatim in either document and these
// are its refusals in both.
//
// RUN AFTER THE DISPOSITIONS, in both: a `{ stance }` is judged against the
// whole stance table, and a `{ down }` names a placement.
func (g *grammar) endings(endings []EndingSpec) {
	at := map[string]int{}
	for i, e := range endings {
		p := fmt.Sprintf("endings[%d]", i)
		switch prev, dup := at[e.ID]; {
		case e.ID == "":
			g.fail(p+".id", "the ending has no id")
		case dup:
			g.fail(p+".id", "ending %q is already declared at endings[%d]", e.ID, prev)
		default:
			at[e.ID] = i
		}
		if e.When == nil {
			g.fail(p+".when", "the ending does not say when it fires — %s", predicateForms)
			continue
		}
		g.predicate(p+".when", e.When, nil)
	}
}

// scenarios judges the scenario bindings, and judges EXACTLY ONE THING about
// them: that every binding's value names something this document declares
// (design law C1, ruled 2026-09-01 — "the dungeon spec stores
// {scenario_id, bindings} as pure references").
//
// WHAT IS DELIBERATELY NOT CHECKED HERE, and where it is checked instead:
// whether the scenario id is one that exists, which keys it wants, which are
// required, and whether the thing a key names is the right KIND of thing — a
// prop where a prop is wanted, an exit where an exit is wanted, and holdable
// when the scenario is about carrying something out. Every one of those is a
// fact about a SCENARIO, and a scenario is content this package may not
// resolve. They are the scenario package's own refusals, made at its
// `New(cfg, compiled)` in form-filler words, where the author is looking at
// the form that asked the question.
//
// So the refusal here is about the FILE: a binding that names nothing is a
// dangling reference whatever scenario reads it, and this package is the one
// layer that can see the whole file at once.
//
// `names` IS THE DIALECT'S UNIVERSE, and it is the only thing that differs
// between the two. v2 places props and creatures in one `place:` list, so a
// placement id covers both; the single room places creatures under `monsters:`
// and declares its props under `propDeclarations`, so it offers the two
// separately. Exits and factions are named the same way in each.
//
// Enumeration is sorted by scenario id and then by field key, because a Go
// map range is not, and a refusal list whose order changes between runs is
// one nobody can diff (C8).
func (g *grammar) scenarios(scenarios map[string]map[string]string, names func(string) bool) {
	for _, id := range sortedKeys(scenarios) {
		bindings := scenarios[id]
		for _, k := range sortedKeys(bindings) {
			named := bindings[k]
			p := fmt.Sprintf("scenarios.%s.%s", id, k)
			switch {
			case named == "":
				g.fail(p, "scenario %q binds %s to nothing", id, k)
			case !names(named):
				g.fail(p, "scenario %q binds %s to %q, and nothing in this dungeon has that id", id, k, named)
			}
		}
	}
}

// placeActions validates the weapons an author armed a monster with
// (rpg-project#448, [PlaceSpec.Actions]).
//
// SHAPE AND VOCABULARY, NOT MEMBERSHIP. Each entry must be a well-formed
// `dnd5e:weapons:<id>` — which is as far as this module can see. The weapons
// catalog lives in the rulebook root, which this module does not import and
// is not about to start importing for a string check; the session resolves
// the id at spawn and refuses the file's boot when the catalog does not have
// it (design §5, "fail closed at author time, not turn time" — boot IS author
// time for a shipped file).
//
// A DUPLICATE IS ALLOWED AND MEANS SOMETHING. `[scimitar, scimitar]` lists
// the same weapon twice, which is a pointless loadout rather than a malformed
// one, and refusing it would be this compiler having an opinion about play.
func (g *grammar) placeActions(path string, actions []string) {
	g.placeActionAugment(path, "actions", actions)
}

// placeActionAugment applies the shared weapon-reference grammar to one
// authored action list, retaining its exact field path for a refusal.
func (g *grammar) placeActionAugment(path, key string, actions []string) {
	for j, ref := range actions {
		at := fmt.Sprintf("%s.%s[%d]", path, key, j)

		parsed, err := core.ParseString(ref)
		if err != nil {
			g.fail(at, "%q is not a ref: %v", ref, err)
			continue
		}
		if parsed.Module != moduleDND5e || parsed.Type != typeWeapons {
			g.fail(at, "%q is not a weapon: an action names a weapon as %s:%s:<id>",
				ref, moduleDND5e, typeWeapons)
		}
	}
}
