// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// facing.go is WHICH WAY A MEMBER FACES (rpg-project#582, sneaking design R4
// and R13).
//
// # Facing is member state the engine sets
//
// Every member carries a facing: a bearing in degrees, 0 is map north,
// clockwise, in [0, 360). It is seeded from the caller's stated facing
// ([MemberInput.Facing], [JoinInput.Facing]) or, when none was stated, from
// [UnstatedFacing]. After that the engine writes it and nobody else does:
//
//   - every step a creature takes turns it along that step;
//   - a walk whose intent names a target member ends facing that member's
//     cell as the walker holds it ([Move.Target], [Routed.Anchor] toward),
//     a walk toward a heard sound ends facing the cell the walker holds it
//     at ([Routed.Heard]), and a walk paused for a reaction window keeps
//     that target across the pause, so the resumed walk still ends facing it;
//   - an attack turns the attacker toward its target, whether a driven turn
//     declared it or the rulebook records it ([Encounter.RecordTrain]) — an
//     opportunity attack included: a [Mover] records the reaction through
//     the same train, with the mover still on the cell it is leaving.
//
// ONLY A CREATURE'S OWN STEPS AND INTENTS TURN IT (lead ruling on
// rpg-toolkit#2073). A forced move — a push, a pull, a shove, any walk an
// effect imposes through [Encounter.Direct] — does not: the mover keeps the
// facing it had, because being shoved backwards is not turning round. So the
// turn is written where a walk is the walker's own ([Encounter.Step], a
// driven or world-round walk, a resumed turn), never in the step every walk
// shares.
//
// A [Routed] WALK IS THE CREATURE'S OWN. Its doc calls it "somebody else's"
// because somebody else chose the route — a table's `toward` or `away`, a
// Command — but the creature's own legs walk it, on its own turn, so it turns
// along its steps and ends facing the anchor it closed on. Imposed means an
// effect moved the body, which is [Encounter.Direct] and nothing else.
//
// ONE SOURCE FOR WHERE A CREATURE TURNS TO (lead ruling on
// rpg-toolkit#2073): the cell it HOLDS for its target, through [faceHeld] —
// at a walk's end, before a driven attack, and when a recorded attack lands.
// Never a live read: a creature that holds no placed sighting of its target
// does not turn toward it.
//
// A TRAIN THAT CLOSES THE ENCOUNTER LEAVES THE LAST TESTIMONY AS IT STOOD
// (lead ruling on rpg-toolkit#2076): the striker's own facing still turns,
// but no sight refresh and no beat follows `ended`, so a watcher's sighting of
// it stays the last one taken before the end.
//
// It persists with the encounter, and it is TESTIMONY: every sighting
// snapshots the subject's facing as the observer saw it
// ([SightTestimony.Facing]), so a memory keeps the facing it last saw. No
// observer reads it live.
//
// # Why a bearing and not a compass word
//
// Six step directions and any bearing toward a target fit one type, and a
// watcher's cone needs an angle. The eight authored compass words convert at
// the dialect, once, through [FacingFromCompass].
//
// # The frame
//
// Map north is the top of the authored map: decreasing row, the -Z the studio
// draws north. [bearing] asks the field's own embedding
// ([spatial.HexEmbedding.Bearing], X east and Y south, zero at east) and turns
// its east-zero answer a quarter turn to put zero at north. So on pointy-top
// hexes the six steps land on 30, 90, 150, 210, 270 and 330, and on flat-top
// hexes on 0, 60, 120, 180, 240 and 300.

// UnstatedFacing is the facing of a member nobody stated one for: 180, map
// south. A RULES BEARING, not a presentation default — cones read it (design
// R13) — so a member seeded without a facing and one stated `s` are the same
// member.
const UnstatedFacing = 180.0

// compassBearings is the eight true-compass words an author writes, the same
// eight a prop's and a start's facing speak, and the bearing each one names.
var compassBearings = map[string]float64{
	"n": 0, "ne": 45, "e": 90, "se": 135, "s": 180, "sw": 225, "w": 270, "nw": 315,
}

// FacingFromCompass is the ONE conversion from an authored compass word to a
// facing: n=0, ne=45, e=90, se=135, s=180, sw=225, w=270, nw=315.
//
// EXPORTED BECAUSE CONTENT AND THE SESSION NEED IT. A compiler lowering an
// authored placement, and a host seating a party at an authored start, both
// hold a word and owe this composition a bearing; a second table anywhere is
// two answers to one question.
//
// Errors: [ErrBadFacing] for any other word, the empty word included — an
// author who stated no facing is a nil [MemberInput.Facing], not a word.
func FacingFromCompass(word string) (float64, error) {
	degrees, ok := compassBearings[word]
	if !ok {
		return 0, fmt.Errorf("%q is not a compass direction: a facing is one of n|ne|e|se|s|sw|w|nw: %w",
			word, ErrBadFacing)
	}

	return degrees, nil
}

// validateFacing refuses a stated facing that is not a bearing in [0, 360).
// Nil is not a refusal: it is the unstated facing, [UnstatedFacing].
//
// REFUSED, NEVER WRAPPED. 360 and -90 name real directions, but a caller who
// wrote one computed it wrong somewhere, and folding it into range would hide
// that; the wire's own range is [0, 360) and a stored value outside it is a
// blob some other build wrote.
func validateFacing(facing *float64) error {
	if facing == nil {
		return nil
	}
	if f := *facing; math.IsNaN(f) || f < 0 || f >= 360 {
		return fmt.Errorf("facing %v is not a bearing in [0, 360): %w", f, ErrBadFacing)
	}

	return nil
}

// seedFacing is the facing a member starts with: the stated one, or
// [UnstatedFacing] when none was stated. Validated first ([validateFacing]).
func seedFacing(facing *float64) float64 {
	if facing == nil {
		return UnstatedFacing
	}

	return *facing
}

// bearing is the direction from one cell's centre to another's on a field of
// the given orientation, in degrees from map north, clockwise, in [0, 360).
//
// ok is false when the two cells are the same: a cell has no direction to
// itself, and a caller asked to face its own cell keeps the facing it has.
//
// ROUNDED TO A MILLIONTH OF A DEGREE, so a step due east is 90 and not
// 90.00000000000001: a facing persists and travels on the wire, and the
// floating-point dust of a square root is not a fact anybody observed.
func bearing(o Orientation, from, to spatial.Position) (float64, bool) {
	plane := spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{Orientation: o.spatial(), CellWidth: FeetPerCell})
	east, ok := plane.Bearing(from, to)
	if !ok {
		return 0, false
	}
	north := math.Round((east+90)*1e6) / 1e6
	north = math.Mod(north, 360)
	if north < 0 {
		north += 360
	}

	return north, true
}

// faceToward turns a member to face a cell from the cell it stands on. A
// member asked to face its own cell, or one not on the map, keeps the facing
// it has.
func (e *Encounter) faceToward(m *memberRecord, cell spatial.Position) {
	here, placed := e.canvas.GetEntityPosition(string(m.ID))
	if !placed {
		return
	}
	if degrees, ok := bearing(e.field.orientation, here, cell); ok {
		m.Facing = degrees
	}
}

// turnAlong turns a creature along a step it took itself: the bearing from
// the cell it left to the cell it took. A zero-distance step has no direction
// and keeps the facing it had. Callers whose walk is imposed do not ask.
func (e *Encounter) turnAlong(action executedAction) {
	if degrees, ok := bearing(e.field.orientation, action.from, action.to); ok {
		action.member.Facing = degrees
	}
}

// faceHeld turns a member toward another as the member HOLDS them: the cell
// its own sighting of that member places them at, current or remembered —
// the cell a walk toward them was routed to, never a live read of where they
// are. A member it holds no placed sighting of leaves the facing as it was.
//
// Errors: the member's own holdings could not be read.
func (e *Encounter) faceHeld(m *memberRecord, target MemberID) error {
	if target == "" {
		return nil
	}
	held, err := e.memberIntel(m.ID)
	if err != nil {
		return fmt.Errorf("facing %q: %w", target, err)
	}
	for _, h := range held {
		if h.Channel != perception.Sight || MemberID(h.Subject) != target {
			continue
		}
		if seen, ok := DecodeSightTestimony(h.Payload); ok && seen.State == LocationKnown {
			e.faceToward(m, seen.Position)
		}
		return nil
	}

	return nil
}

// walkFace is what a walk ends facing: a member as the walker holds them, or
// the cell of a sound the walker holds ([Routed.Heard]). The zero value faces
// nothing, and the walk keeps the facing of its last step.
//
// TWO TARGETS, ONE SOURCE. Both are the walker's own holdings: the member's
// cell is read off its sighting at the walk's end ([Encounter.faceHeld]), and
// the sound's cell was read off its hearing when the walk was decided
// ([MonsterView.Sounds]) — a sound cannot move, so the cell it rang at is the
// cell it is held at.
type walkFace struct {
	member MemberID
	heard  *spatial.Position
}

// faceWalkEnd turns a walker toward what its walk closed on ([walkFace]).
//
// Errors: the walker's own holdings could not be read.
func (e *Encounter) faceWalkEnd(m *memberRecord, f walkFace) error {
	if f.heard != nil {
		e.faceToward(m, *f.heard)

		return nil
	}

	return e.faceHeld(m, f.member)
}

// routedFace is what a [Routed] walk ends facing: its anchor member when the
// route closes on one, the sound's cell when it walks toward a sound
// ([Routed.Heard]), and nothing for a flight or an authored cell — a creature
// running from something does not end its run turned toward it, and a cell
// names a place, not a target.
func routedFace(r Routed) walkFace {
	if r.Policy != MoveToward {
		return walkFace{}
	}
	if r.Heard && r.AnchorAt != nil {
		cell := *r.AnchorAt

		return walkFace{heard: &cell}
	}

	return walkFace{member: r.Anchor}
}
