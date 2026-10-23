// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The cause a table-driven walk carries, so an observer can tell a creature
// obeying its own orders from a creature being shoved, commanded or routed by
// a spell — and, since both directions leave as a [Routed], tell the two
// directions apart.
//
// THE TABLE IS THE CAUSE, AND THE WORD IS THE DIRECTION. [Routed]'s own doc
// says "a Move is the member's own decision; a Routed is somebody else's" —
// which was true when the only Routed was Command's. A table's `toward` and
// `away` are the creature's own decision carried out by the engine's router,
// so the cause names the word the author wrote rather than a spell.
//
// ONE REF FOR BOTH WAS A LIE THE STORY COULD NOT RECOVER FROM: the `moved` and
// `stayed` beats an observer reads carry this ref and nothing else about the
// intent, so bandits walking to the front room under `toward: {at}` read as
// bandits running away. A walk narrated with no cause at all is an observer
// being told a creature walked away for no reason, which is exactly what
// [DirectInput.Cause] is required to prevent; a walk narrated with the WRONG
// cause is worse, because it answers.
var (
	tableCauseToward = core.Ref{Module: "encounter", Type: "table", ID: "toward"}
	tableCauseAway   = core.Ref{Module: "encounter", Type: "table", ID: "away"}
	tableCauseSearch = core.Ref{Module: "encounter", Type: "table", ID: "search"}
)

// TableDriver is THE ONE DRIVER: it rolls the creature's own authored table
// under [AnswerTime] and turns the entry that fired into an intent
// (rpg-project#465, design §7).
//
// # It decides nothing
//
// Every choice this makes was written by an author and loaded by a
// temperament. What this type owns is the translation — a word and a selector
// into a [TurnIntent] — and the selector resolution, which is done against
// the view and nothing else (C2): the nearest opposed sighting, the actor of
// a deed this creature holds, an authored cell. It never reaches for the
// encounter, which is what makes it testable with a fixture view and no world
// at all.
//
// # The pick travels back
//
// [Decision.Pick] carries the whole arithmetic so the encounter can append the
// answer beat. The die is the world's shared dice, and the creature named on
// that beat is whose it is.
//
// # What it is NOT asked
//
// A compelled turn (Command, Dissonant Whispers) does not pass through here:
// those keep their own engine-driven [Routed] path and their own driver. A
// player's turn never reaches a driver at all.
type TableDriver struct {
	// Roller is the world's shared dice. Required: a driver that cannot
	// roll cannot pick, and a local rand would make every table unreplayable
	// (R7).
	Roller dice.Roller
}

// Act rolls this member's `time` table and turns the entry that fired into an
// intent.
//
// A CONTEXT OF ITS OWN, for [Encounter.executeTurnIntent]'s stated reason: no
// verb on this composition accepts one, and plumbing one through every verb
// between a host's request and here is a bigger change than this slice asks
// for.
//
// AN ENTRY THAT SELECTED NOBODY STILL ROLLED. `attack: attacker` on a creature
// nobody has attacked, `toward: enemy` with nothing opposed in sight or
// memory: the entry was on the table, the die chose it, and the answer carries
// the pick — so the beat shows what the creature tried to do. Silently
// re-rolling would hide an authoring mistake behind plausible behaviour. An
// `attack` that cannot land goes out as the [Attack] anyway, and the encounter
// says why it did nothing ([Attack]'s doc); a `toward` whose target is
// already in reach goes out as a [Pass] that names itself.
func (d TableDriver) Act(view MonsterView) (Decision, error) {
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: AnswerTime, Table: view.Table, Temper: view.Temper, Facts: factsFromView(view), Die: d.Roller,
	})
	if err != nil {
		return Decision{}, fmt.Errorf("table driver %q: %w", view.Self, err)
	}
	// pick never answers nil for AnswerTime — an empty table is a hold — but
	// a nil here would be a nil-pointer panic inside a driver, which is the
	// one failure mode this seam turns into an aborted verb rather than a
	// crash.
	if chosen == nil {
		return Decision{}, fmt.Errorf("table driver %q: %w", view.Self, ErrBadAnswer)
	}

	intent, err := d.intentFor(view, chosen.Answer)
	if err != nil {
		return Decision{}, err
	}

	return Decision{Intent: intent, Pick: chosen}, nil
}

// intentFor translates one fired entry into the intent that carries it out.
func (d TableDriver) intentFor(view MonsterView, entry Answer) (TurnIntent, error) {
	switch {
	case entry.Hold:
		return Pass{}, nil

	case entry.Attack != nil:
		return d.attackIntent(view, entry), nil

	case entry.Toward != nil:
		return d.towardIntent(view, entry), nil

	case entry.Away != nil:
		return d.awayIntent(view, entry), nil

	case entry.Search != nil:
		return d.searchIntent(view, *entry.Search), nil

	case entry.Fact != "", !entry.Flee && entry.Say != "":
		// THE CREATURE SPENDS ITS TIME SAYING SOMETHING (rpg-project#569 R6):
		// an alarm raised, or a line with nothing else in it. Saying is not a
		// walk or a swing, so the intent is a Pass — and the line and the
		// fact land where the pick's beat is written
		// ([Encounter.answerTime]), because what was said and who heard it
		// is the encounter's to land and not this driver's.
		return Pass{}, nil

	default:
		// `flee` is a social word, refused under `time` by [validateTable]
		// at every door a table comes in through, and a row with no word and
		// no line is refused by the dialect. Reaching here means a blob got
		// past that, and the honest answer is a named error rather than a
		// creature standing there for reasons nobody could find.
		return nil, fmt.Errorf("table driver %q: %q under `time`: %w",
			view.Self, answerWord(entry), ErrBadAnswer)
	}
}

// attackIntent strikes the selected member with the first action it is in
// reach of, in the author's own order.
//
// THE ORDER IS THE INSTRUCTION ([dungeonspec.MonsterPlacement.Actions]): the
// first action whose target is in reach is the one taken, so sorting the list
// would quietly rewrite what the author said the monster does.
//
// NOBODY IN REACH IS NOT A WALK. The table has `toward` for closing and this
// entry said `attack`; turning one word into the other would be this driver
// making a decision the author did not write.
//
// AND IT IS NOT A SILENT PASS EITHER (rpg-toolkit#2000). The Attack goes out
// whatever this view says — with the author's first action when none reaches,
// and with no Target when the selector named nobody — and the encounter, which
// is where every attack is judged, refuses it and tells why. Judging reach
// here as well used to mean two judges, both silent: this one returned a Pass
// and the encounter's own returned "turn over", and the story could not say
// which, or that either had happened.
func (d TableDriver) attackIntent(view MonsterView, entry Answer) TurnIntent {
	var first core.Ref
	if len(view.Actions) > 0 {
		first = view.Actions[0].Ref
	}
	target, ok := d.selectMember(view, entry, *entry.Attack)
	if !ok {
		return Attack{Action: first}
	}
	for _, s := range view.Seen {
		if s.ID != target {
			continue
		}
		for _, a := range view.Actions {
			if s.InReach[a.Ref] {
				return Attack{Target: target, Action: a.Ref}
			}
		}
	}

	return Attack{Target: target, Action: first}
}

// towardIntent walks toward the selected member's BELIEVED position, or the
// authored cell, for this turn's movement.
//
// TWO SHAPES, AND THE DIFFERENCE IS WHO KNOWS THE PATH. A member the view can
// see or remember arrives with a route already computed against this
// composition's own walls and doors ([SeenMember.Path],
// [RememberedMember.Path]), truncated here to what the budget pays for — a
// [Move], so the creature may still swing when it arrives. An authored cell
// has no such route on the view, so it goes out as a [Routed] the encounter
// paths itself, which is TERMINAL: a creature walking to a cell somebody
// wrote on the map has nothing else it was told to do there.
func (d TableDriver) towardIntent(view MonsterView, entry Answer) TurnIntent {
	cells := CellsFromFeet(view.Budget.MovementFeet)
	if cells <= 0 {
		return Pass{}
	}

	if at := entry.Toward.At; at != nil {
		cell := *at

		return Routed{Policy: MoveToward, AnchorAt: &cell, Cause: tableCauseToward}
	}

	if path := entry.Toward.Path; len(path) > 0 {
		// ON THE ROUTE, THE LEG'S OWN CELLS; OFF IT, BACK TO THE NEAREST
		// WAYPOINT ([patrolStep]). A Move rather than a Routed walk, because
		// the leg IS the route: the creature walks the same cells the next
		// round will find it on.
		//
		// THE WAYPOINT AHEAD MAY STATE A PAUSE (rpg-project#624) — a facing,
		// a wait, or both — and the walk carries it: the encounter pauses the
		// creature there only if the walk ends standing on it
		// ([WaypointPause]). Asked of the leg's own end, before the budget's
		// cut, because a cut walk simply does not end on it.
		//
		// STANDING OUT A WAIT, THE ROW HOLDS (dwell.go): the creature is on
		// its waypoint and its turns there are not spent, so the patrol
		// walks nowhere. Rows above this one still fire first.
		if view.Dwelling {
			return Pass{Cause: tableCauseToward, Why: towardDwelling}
		}
		ahead, back := patrolStep(view.Position, path)
		if len(ahead) == 0 {
			return Routed{Policy: MoveToward, AnchorAt: &back, Cause: tableCauseToward,
				Pause: pauseAt(entry.Toward, back)}
		}
		pause := pauseAt(entry.Toward, ahead[len(ahead)-1])
		if len(ahead) > cells {
			ahead = ahead[:cells]
		}

		return Move{Path: append([]spatial.Position(nil), ahead...), Pause: pause}
	}

	if entry.Toward.Word == SelectorSound {
		cell, heard := soundFor(view.Sounds, entry.When)
		if !heard {
			return Pass{}
		}

		// A SOUND IS A TARGET, so the walk ends facing where it rang
		// ([Routed.Heard]).
		return Routed{Policy: MoveToward, AnchorAt: &cell, Heard: true, Cause: tableCauseToward}
	}

	target, ok := d.selectMember(view, entry, *entry.Toward)
	if !ok {
		return Pass{}
	}
	if alreadyInReach(view, target) {
		return Pass{Cause: tableCauseToward, Why: towardAlreadyInReach}
	}
	path := d.pathTo(view, target)
	if len(path) == 0 {
		// A MEMORY WITH NOWHERE LEFT TO WALK IS LOST (memory.go,
		// rpg-project#668 R3): the creature stands where it last saw them,
		// or cannot get there. Sight first, as [pathTo] reads: a target in
		// view with no route is still in view, and nothing is lost.
		//
		// A SILENT PASS, as this arm always was: the creature's next pick
		// is `enemy: lost`, and that pick's own beat is where the story
		// says what it does about it.
		if !d.inSight(view, target) {
			return Pass{Loses: target}
		}

		return Pass{}
	}
	if len(path) > cells {
		path = path[:cells]
	}

	// THE WALK NAMES WHO IT CLOSES ON, so the creature ends it facing them
	// ([Move.Target]).
	return Move{Path: append([]spatial.Position(nil), path...), Target: target}
}

// pauseAt is the pause a route states for one of its waypoints
// ([Selector.Facings], [Selector.Waits]), or nil when the cell is not a
// waypoint of the route or the author stated no wait there. A route names
// each waypoint once ([RetracedCell] refuses one that comes back), so the
// cell is the lookup.
func pauseAt(route *Selector, cell spatial.Position) *WaypointPause {
	for i, waypoint := range route.Path {
		if waypoint != cell {
			continue
		}
		pause := WaypointPause{Cell: cell}
		if i < len(route.Facings) && route.Facings[i] != nil {
			degrees := *route.Facings[i]
			pause.Facing = &degrees
		}
		if i < len(route.Waits) {
			pause.Wait = route.Waits[i]
		}
		if pause.Wait == 0 {
			// No wait, no pause: a facing is taken while standing
			// ([WaypointPause]).
			return nil
		}

		return &pause
	}

	return nil
}

// towardDwelling is the sentence a patrol row holding at its waypoint tells
// on its stayed beat.
const towardDwelling = "standing at its waypoint"

// inSight reports whether target is one of the view's current sightings.
func (d TableDriver) inSight(view MonsterView, target MemberID) bool {
	for _, s := range view.Seen {
		if s.ID == target {
			return true
		}
	}

	return false
}

// towardAlreadyInReach is the sentence a `toward` whose target is already
// within range of one of the creature's actions tells on its stayed beat.
const towardAlreadyInReach = "already in reach"

// alreadyInReach reports whether target is in sight and within RANGE of any
// of this member's own actions — the place `toward` was walking it to.
//
// RANGE, NOT MELEE REACH. [SeenMember.InReach] is each action's own range, so
// a creature carrying a shortbow is "already in reach" of a target 80 feet
// away and does not close. That matches the seen route's goal (the longest
// range, [Encounter.buildMonsterView]), which is why the two agree; whether a
// bow-carrier should close to melee is a separate question.
//
// THERE IS NO FORWARD STEP FROM HERE (rpg-toolkit#2001). The seen route is
// already empty when the target is in reach ([Encounter.buildMonsterView]'s
// goal holds where the creature stands), so this returns nothing the route
// would not; what it adds is the reason, so the story says the creature
// stayed because it was already there instead of passing in silence.
func alreadyInReach(view MonsterView, target MemberID) bool {
	for _, s := range view.Seen {
		if s.ID != target {
			continue
		}
		for _, a := range view.Actions {
			if s.InReach[a.Ref] {
				return true
			}
		}
	}

	return false
}

// awayIntent routes away from the selected member for this turn's movement —
// the coward's run.
//
// [Routed] RATHER THAN A PATH, because running away is a policy and not a
// route: the board is what knows where the corners are, and a driver picking
// its own cells would flee into one. Terminal, which is what running is.
func (d TableDriver) awayIntent(view MonsterView, entry Answer) TurnIntent {
	if CellsFromFeet(view.Budget.MovementFeet) <= 0 {
		return Pass{}
	}
	if entry.Away.Word == SelectorSound {
		cell, heard := soundFor(view.Sounds, entry.When)
		if !heard {
			return Pass{}
		}

		return Routed{Policy: MoveAway, AnchorAt: &cell, Cause: tableCauseAway}
	}
	target, ok := d.selectMember(view, entry, *entry.Away)
	if !ok {
		return Pass{}
	}

	return Routed{Policy: MoveAway, Anchor: target, Cause: tableCauseAway}
}

// searchIntent walks on from the nearest lost enemy's cell along the trail
// its heading word names, to the trail's Nth cell or its last
// (rpg-project#668 R4, rpg-project#679 R10), and names whom it
// is looking for so the encounter can end the search when it gets there
// ([Routed.GivesUp]).
//
// A [Routed] TO A CELL, because the search cell is a place to BE — the
// exact-cell walk an authored `toward: { at }` takes — and the encounter
// paths it against the walls the trail was clipped by. Terminal, like every
// routed walk: a creature searching has nothing else it was told to do.
//
// AN EMPTY TRAIL SEARCHES WHERE IT STANDS: the lost cell itself is the search
// cell, so a creature whose very first step along the heading is a wall
// gives up there rather than walking off somewhere the author never said.
// No lost enemy at all is a struct built past the eligibility gate
// ([behavior.Facts]: `search` needs `enemy: lost`), and goes out as a plain
// Pass, the rule for any word that names nothing.
func (d TableDriver) searchIntent(view MonsterView, search Search) TurnIntent {
	var target *LostMember
	for i := range view.Lost {
		l := &view.Lost[i]
		if !l.Opposed {
			continue
		}
		if target == nil || l.DistanceCells < target.DistanceCells {
			target = l
		}
	}
	if target == nil {
		return Pass{}
	}
	// THE ROW'S HEADING PICKS THE TRAIL (rpg-project#679 R10): which way to
	// walk on is the author's word, read here, never the engine's guess.
	trail := target.Trails[search.Heading]
	cell := target.Position
	if n := min(search.Distance, len(trail)); n > 0 {
		cell = trail[n-1]
	}

	return Routed{Policy: MoveToward, AnchorAt: &cell, Cause: tableCauseSearch, GivesUp: target.ID}
}

// pathTo is the route the view already computed toward one member — current
// sight first, then memory.
//
// SIGHT BEFORE MEMORY, because a creature that can see where somebody IS does
// not walk to where they WERE. The view is rebuilt after every executed
// intent, so a remembered pursuit is interrupted by the first glimpse.
func (d TableDriver) pathTo(view MonsterView, target MemberID) []spatial.Position {
	for _, s := range view.Seen {
		if s.ID == target {
			return s.Path
		}
	}
	for _, r := range view.Remembered {
		if r.ID == target {
			return r.Path
		}
	}

	return nil
}

// soundFor is WHERE the freshest sound of the kinds this row LISTENS FOR was
// made (rpg-project#569 R10, refined on the behavior review): the kinds its
// own `heard` conditions name, the list's entries only — a `heard` inside a
// `not` is a sound the row is waiting NOT to have heard. A guard that heard a
// voice after the clang it is answering still walks to the clang. Ties on
// the stamp break on the kind's name so the same holdings always pick the
// same place (C8).
//
// ok IS ALWAYS TRUE FOR A ROW THAT FIRED. mind/behavior refuses a `sound`
// row with no `heard` condition, its eligibility gates a `sound` selector on
// a held sound, and the row's own `heard` holding means a sound of that kind
// is held — so the false answer is a struct built in Go past every door, and
// it goes out as a plain Pass, the rule for any selector that names nothing.
func soundFor(sounds []Sound, when []When) (spatial.Position, bool) {
	kinds := map[string]bool{}
	for _, w := range when {
		if w.Heard != nil {
			kinds[w.Heard.Kind] = true
		}
	}
	var best Sound
	found := false
	for _, s := range sounds {
		if !kinds[s.Kind] {
			continue
		}
		if !found || s.At > best.At || (s.At == best.At && s.Kind < best.Kind) {
			best, found = s, true
		}
	}

	return best.Where, found
}

// scopeOf is the reading the entry's deed condition asks for, or the self
// reading when no condition in its list names a deed — a `time` row with no
// deed condition reads the creature's own deeds.
func scopeOf(entry Answer) DeedScope {
	when := deedWhen(entry.When)
	if when == nil {
		return ScopeSelf
	}

	return when.Scope
}

// deedsForScope is the deeds list ONE SCOPE reads, off the view: the mirror of
// the evaluator's own dispatch (behavior's When.deedsOf), so a condition and
// the selector it pairs with can never read different deeds.
//
// FOUND IN REVIEW (rpg-toolkit#1890 thread 2): selectMember read view.Deeds
// unconditionally, so the row this whole slice exists for —
// `when: { attacked: { within: 3, on: ally } }, attack: attacker` — fired on
// the ALLY reading and then resolved `attacker` off the SELF reading. With
// nobody having struck the creature itself that is a Pass (the row won the roll
// and nothing happened, silently); with somebody having struck it, it struck
// the WRONG member.
//
// AN UNKNOWN SCOPE MATCHES NOTHING, the same policy the evaluator keeps, so a
// scope this build does not read names nobody rather than silently reading as
// the self one.
func deedsForScope(view MonsterView, scope DeedScope) []HeldDeed {
	switch scope {
	case ScopeSelf:
		return view.Deeds
	case ScopeAlly:
		return view.AllyDeeds
	case ScopeActor:
		return view.OwnDeeds
	default:
		return nil
	}
}

// selectMember resolves a selector word against the view.
//
//   - `enemy`: the nearest opposed member in sight, else the nearest opposed
//     member remembered. Opposition is the stance graph's answer, projected
//     onto the view — "a mind is never handed a target it is not opposed to",
//     which is what keeps a neutral creature from advancing on the party.
//   - `attacker`: who last landed an attack on this creature, off its own
//     held deeds.
//   - `actor`: the actor of the deed this entry's `when` named, so `fled`
//     pairs with `away: actor`.
//
// `sound` is not here: it names a PLACE, never a member (R4), and is resolved
// by [soundFor] where `toward` and `away` read their cells.
//
// ok=false is a selector that named nobody, which the caller turns into a
// [Pass] with the pick still attached.
func (d TableDriver) selectMember(view MonsterView, entry Answer, sel Selector) (MemberID, bool) {
	switch sel.Word {
	case SelectorEnemy:
		return nearestOpposed(view)

	case SelectorAttacker:
		if held, ok := freshestDeed(deedsForScope(view, scopeOf(entry)), DeedAttack); ok && held.Actor != "" {
			return held.Actor, true
		}

		return "", false

	case SelectorActor:
		when := deedWhen(entry.When)
		if when == nil {
			return "", false
		}
		if held, ok := freshestDeed(deedsForScope(view, when.Scope), DeedVerbFor(when.Deed)); ok && held.Actor != "" {
			return held.Actor, true
		}

		return "", false

	default:
		return "", false
	}
}

// nearestOpposed is the nearest opposed member the view holds: sighted first,
// then remembered. Ties break on the member's id so the same view always
// selects the same target (C8).
func nearestOpposed(view MonsterView) (MemberID, bool) {
	best := MemberID("")
	bestDist := 0.0
	for _, s := range view.Seen {
		if !s.Opposed || !s.Standing {
			continue
		}
		if best == "" || s.DistanceCells < bestDist {
			best, bestDist = s.ID, s.DistanceCells
		}
	}
	if best != "" {
		return best, true
	}
	for _, r := range view.Remembered {
		if !r.Opposed {
			continue
		}
		if best == "" || r.DistanceCells < bestDist {
			best, bestDist = r.ID, r.DistanceCells
		}
	}

	return best, best != ""
}
