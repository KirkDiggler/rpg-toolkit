// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
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
//   - [cells], the frame a `{ at: [col, row] }` selector resolves in. The v2
//     dialect looks the pair up in the floor it validated; the single room
//     names no cell at all, and says so in one sentence.
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

// cellAnswer is what a dialect says about the cell a selector named.
type cellAnswer struct {
	// framed is whether this dialect has a frame to resolve an authored pair
	// in at all. When it is false the refusal is the WHOLE defect and the
	// word carrying the cell is never judged: the frame is the mistake, and
	// two messages for one mistake sends an author looking for a second
	// problem.
	framed bool

	// refusal is why the pair is not somewhere to walk to, or "" when it is.
	refusal string
}

// cells is the dialect's frame: what an authored `{ at: [col, row] }` inside
// a selector means, and whether a creature can be sent there.
//
// THE GRAMMAR OWNS NO CELLS (rpg-project#484, R2). The v2 dialect resolves the
// pair through the orientation its document declares and looks it up in the
// floor its regions painted ([floorCells]). The single room declares no
// orientation and its own cells are axial, so the same authored bytes would
// name two different cells in the two dialects, and it names none
// ([singleRoomCells]).
type cells interface {
	cellAt(at [2]int) cellAnswer
}

// floorCells is the v2 dialect's frame: the orientation the document declared,
// and the owned floor its regions painted.
type floorCells struct {
	orientation encounter.Orientation
	owner       map[spatial.Position]int
}

// cellAt resolves an authored pair against that floor, refusing a cell nobody
// could walk to in the author's own coordinates.
func (f floorCells) cellAt(at [2]int) cellAnswer {
	if _, onFloor := f.owner[encounter.HexCellAt(f.orientation, at[0], at[1])]; !onFloor {
		return cellAnswer{framed: true, refusal: fmt.Sprintf("this walks to [%d,%d], which is not floor", at[0], at[1])}
	}

	return cellAnswer{framed: true}
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

	// Members is the cast; Cells is the frame.
	Members members
	Cells   cells
}

// grammar judges one document's gameplay vocabulary, reporting every defect
// at the path the dialect named it by.
type grammar struct {
	add errSink

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
}

// newGrammar builds the grammar one dialect's document is judged by.
func newGrammar(in grammarInput) *grammar {
	return &grammar{
		add:                  in.Add,
		declaredFactions:     in.Factions,
		declaredDispositions: in.Dispositions,
		members:              in.Members,
		cells:                in.Cells,
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
// THE KEY IS THE TRIGGER, and only the five this build rolls are accepted:
// the four social outcomes, and `time`. A word the design NAMES and has not
// built is refused by name in the entry's own decoder ([laterWords]); a key
// nobody designed is refused here, listing what there is. Either way the
// author finds out on the form instead of at the table.
//
// Refusals per entry, each its own sentence:
//
//   - a weight below 1, which is a row that can never fire;
//   - two outcome words in one entry, so ordering never has to be guessed;
//   - an entry with no word and nothing to say, which is a row written for no
//     reason;
//   - an empty `fact:`, which says the world learns something and not what;
//   - a word under a key it is not legal on: `fact` and `flee` answer a social
//     verdict, `hold`/`attack`/`toward`/`away` are what a creature does with
//     time, and a `when` is a time word because a social key IS the condition;
//   - `at:` on anything but `toward`, because walking away from a fixed cell
//     is a direction rather than a flight and nothing has paid for one;
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

// answerEntry validates one row of one trigger's table.
func (g *grammar) answerEntry(at string, key string, entry AnswerSpec) {
	if entry.Weight != nil && *entry.Weight < 1 {
		g.fail(at+".weight", "a weight of %d can never be rolled: omit it for 1, or give it a share",
			*entry.Weight)
	}

	// An empty `fact:` is its own sentence rather than a missing word: the
	// author wrote the key, so they meant to teach something. Reported
	// INSTEAD of the no-word refusal below, not beside it — two defects for
	// one mistake sends an author looking for a second problem.
	if entry.Fact != nil && *entry.Fact == "" {
		g.fail(at+".fact", "this says the world learns something and does not say what")

		return
	}

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
}

// wordLegality refuses a word, or a `when`, under a key it is not legal on.
func (g *grammar) wordLegality(at, key string, words []string, entry AnswerSpec) {
	time := key == string(encounter.AnswerTime)
	for _, word := range words {
		switch word {
		case "fact", "flee":
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
	if entry.When != nil && !time {
		g.fail(at+".when",
			"`%s` is already the condition — a `when` under it asks when a thing that just happened happened (line %d)",
			key, entry.When.Line)
	}
}

// entrySelector refuses a selector that names a cell where only a member can
// stand, an `actor` with no deed to have been the actor of, and a cell the
// dialect cannot send anybody to.
//
// THE FRAME COMES FIRST, and when the dialect has none it is the whole
// answer: a single room refuses the authored pair outright and the word
// carrying it is never judged, because the frame is the mistake. A dialect
// that HAS a frame judges the word next — a cell is somewhere to walk toward,
// never something to attack — and only then asks whether that cell is floor.
func (g *grammar) entrySelector(at, _ string, entry AnswerSpec) {
	word, sel := entrySelectorOf(entry)
	if sel == nil {
		return
	}
	if sel.At != nil {
		answer := g.cells.cellAt(*sel.At)
		if !answer.framed {
			g.fail(at+"."+word+".at", "%s", answer.refusal)

			return
		}
		if word != "toward" {
			g.fail(at+"."+word,
				"a cell is somewhere to walk toward, and `%s` acts on a creature (line %d)", word, sel.Line)

			return
		}
		if answer.refusal != "" {
			g.fail(at+"."+word+".at", "%s", answer.refusal)
		}

		return
	}
	if sel.Word == string(encounter.SelectorActor) && (entry.When == nil || entry.When.Deed == "") {
		g.fail(at+"."+word,
			"`actor` is the actor of the deed this entry's `when` names, and this entry names no deed (line %d)",
			sel.Line)
	}
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
	if entry.Away != nil {
		out = append(out, "away")
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
func sortedKeys(m map[string][]AnswerSpec) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)

	return out
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
	for j, ref := range actions {
		at := fmt.Sprintf("%s.actions[%d]", path, j)

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
