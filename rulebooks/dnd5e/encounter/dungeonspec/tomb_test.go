// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// tomb_test.go is THE FORCING CASE (rpg-toolkit#1127, rpg-project#227's §1):
// reference-tomb.yaml compiles into a runnable new-stack world, and a player
// walks entrance → hall → tomb.
//
// It is the case the whole world-model wave was aimed at, and it is a TEST
// rather than a demo script because the point was never that the compile can be
// made to happen once. Every slice from S0 to S4 was justified by this scene —
// one canvas (#1106), rooms as regions (#1108), sight as geometry (#1111), one
// installed world (#1114), a readable canvas (#1118), authored void (#1116),
// doors as state on the wall (#1123), props that say what they do (#1128) — and
// this is where they are asked to hold up together, in order, in one sitting.
//
// # It reads the map rather than hardcoding it
//
// Almost nothing here is a literal cell. The party's seat, the skeletons' cells,
// the door's ID and the boss's cell all come out of the COMPILED dungeon, and
// the walk is computed from them. That is deliberate: a test that hardcoded
// (5,3) would be pinning the compiler's arithmetic in the same breath as the
// scene, and would have to be rewritten by anyone who changed the grid family
// or the doorway rule. What this file asserts is that the SCENE happens — she
// walks, they see her, the door refuses and says what it would take, and beaten
// it reveals the chamber. Where exactly the cells fall is the compiler's own
// tests' business.
//
// The dungeon it walks is tombsource_test.go's — the reference tomb re-authored
// in this package's own dialect, per Kirk's ruling that the old one "has never
// run in a game… it is legacy and will have no home after". That file says what
// went and what it now insists on.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	delve = core.EntityID("delve")

	// The DC the tomb authors on its second connector, and the only number in
	// this file that is a literal on purpose: it is the authored fact the
	// scene is about, so reading it back out of the compiler would make the
	// assertion circular.
	tombLockDC = 12
)

type TombSuite struct {
	suite.Suite

	compiled dungeonspec.Compiled
	enc      *encounter.Encounter
}

func TestTombSuite(t *testing.T) {
	suite.Run(t, new(TombSuite))
}

func (s *TombSuite) SetupTest() {
	compiled, err := dungeonspec.Load([]byte(tombYAML(s.T())))
	s.Require().NoError(err, "the shipping tomb must compile")
	s.compiled = compiled

	members := []encounter.MemberInput{{
		ID: delve, Kind: encounter.KindPlayer,
		Position: compiled.PartyStart[0].At,
	}}
	for i, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: monsterID(i), Kind: encounter.KindMonster, Position: m.At,
		})
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{},
		Field:   compiled.Field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err, "and the compiled field must be a field this composition accepts")
	s.enc = enc
}

// TestTheTombIsWalkedEntranceToHallToBoss is the done-when, in one sitting.
func (s *TombSuite) TestTheTombIsWalkedEntranceToHallToBoss() {
	// ── she starts in the entrance ────────────────────────────────────────
	region, ok := s.enc.RegionAt(s.cellOf(delve))
	s.Require().True(ok)
	s.Require().Equal("entrance", region, "a player starts where the dungeon says the party enters")

	s.Require().False(s.anySkeletonSees(), "and the hall is dark to its garrison")

	// ── she walks for the hall, and the open doorway gives her away ───────
	//
	// The garrison catches her BEFORE she is through it, and that is the
	// dungeon rather than the fixture: an open connector is an opening, so
	// the sightline runs both ways through it. The locked door two beats
	// from now is what a closed one does instead, and the contrast is the
	// whole point of the pair.
	nearest := s.nearestMonsterCell("hall")
	out := s.walkTo(nearest.beside)

	s.True(s.anySkeletonSees(), "the garrison sees her")
	s.Require().NotNil(out.Formed, "and a fight forms on the sight of her, unasked")

	// ── she breaks off and gets in ────────────────────────────────────────
	//
	// A member in a bubble acts through the fight's own turn structure, and
	// there is no in-fight movement verb — so free roam is exactly what a
	// fight suspends, and crossing the hall means disengaging first.
	s.walkOn(nearest.beside)
	s.Require().Equal("hall", s.regionOf(s.cellOf(delve)), "she is in the hall")

	// ── she goes to the door, and it refuses ──────────────────────────────
	door := s.tombDoor()
	s.Require().Equal(encounter.DoorStateKind("locked"), door.State.Kind())

	near, far := s.doorSides(door)
	s.walkOn(near)

	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: door.ID})
	s.Require().ErrorIs(err, encounter.ErrLocked, "the tomb is shut")
	s.Require().Contains(err.Error(), "DC 12", "and the refusal says what it would take")

	// Standing in the doorway itself, she cannot see the far side of it —
	// which is what a door IS, and the difference between this connector and
	// the one the garrison saw her through.
	s.False(s.canSee(far), "the tomb is dark through a shut door")

	// ── beaten, it opens and reveals the chamber ──────────────────────────
	failed, err := s.enc.Unlock(&encounter.UnlockInput{Door: door.ID, Beaten: false})
	s.Require().NoError(err)
	s.False(failed.Beaten, "a missed check leaves it locked")
	s.Equal([]encounter.CheckApproach{{Ability: "dex", DC: tombLockDC}}, failed.Approaches,
		"and reports the approaches it carries")
	s.Require().Equal(encounter.DoorStateKind("locked"), s.tombDoor().State.Kind(),
		"and leaves it there to try again")

	beaten, err := s.enc.Unlock(&encounter.UnlockInput{Door: door.ID, Beaten: true})
	s.Require().NoError(err)
	s.True(beaten.Beaten)
	s.Equal([]encounter.CheckApproach{{Ability: "dex", DC: tombLockDC}}, beaten.Approaches)

	// Beaten, the door ends OPEN rather than merely unlocked — a party that
	// just picked a lock is going through, and a second OpenDoor call would be
	// ceremony with a window in the middle where the door is a state nobody
	// authored ([encounter.Encounter.Unlock]'s own doc).
	s.Require().Equal(encounter.DoorStateKind("open"), s.doorByID(door.ID).State.Kind())

	s.True(s.canSee(far), "and open, the chamber beyond is there")

	// ── and she reaches the captain ───────────────────────────────────────
	boss := s.bossCell()
	s.walkOn(s.cellBeside(boss))
	s.Require().Equal("tomb", s.regionOf(s.cellOf(delve)), "she stands in the tomb")
	s.True(s.sees(delve, boss), "with the captain in front of her")
}

// TestTheCompilerCarriesWhatItMayNotInterpret pins the roster half: the refs
// and the authored targeting word survive the compile untouched, because this
// package is not allowed to know what a skeleton is or what "lowest-health"
// means (rpg-toolkit#1127's ruled two-layer split).
func (s *TombSuite) TestTheCompilerCarriesWhatItMayNotInterpret() {
	refs := map[string]int{}
	for _, m := range s.compiled.Monsters {
		refs[m.Ref]++
	}
	s.Equal(2, refs["dnd5e:monsters:skeleton"], "both hall skeletons")
	s.Equal(1, refs["dnd5e:monsters:skeleton-captain"], "and the captain")

	for _, m := range s.compiled.Monsters {
		if m.Ref != "dnd5e:monsters:skeleton" {
			continue
		}
		s.Equal("lowest-health", m.Targeting,
			"carried as the author's word, uninterpreted — this package has no opinion on it")
	}

	var boss int
	for _, m := range s.compiled.Monsters {
		if m.Boss {
			boss++
			s.Equal("tomb", m.Region, "the boss is authored into the boss chamber")
		}
	}
	s.Equal(1, boss)
}

// TestTheCoffinIsSolidAndSeenOver is the prop half, and the reason #1128 landed
// first: `blocks_los: false` is the tomb's own authored exception, and until
// props carried their answers it was the one placement in the file that could
// not compile honestly.
func (s *TombSuite) TestTheCoffinIsSolidAndSeenOver() {
	atlas, err := s.enc.Atlas()
	s.Require().NoError(err)

	var found int
	{
		for _, p := range atlas.Props {
			switch p.Ref {
			case "dnd5e:props:coffin":
				found++
				s.False(p.BlocksLineOfSight, "authored blocks_los: false — you see over a coffin")
				s.True(p.BlocksMovement, "and you do not walk through one")
			case "dnd5e:props:pillar", "dnd5e:props:statue-reaper":
				s.True(p.BlocksLineOfSight, "an unflagged prop blocks both")
				s.True(p.BlocksMovement)
			}
		}
	}
	s.Equal(1, found, "the tomb authors exactly one coffin")
}

// ─────────────────────────────────────────────────────────────────────────
// helpers: everything below reads the compiled map rather than assuming it.

func monsterID(i int) core.EntityID {
	return core.EntityID(string(rune('a'+i)) + "-monster")
}

func (s *TombSuite) cellOf(id core.EntityID) spatial.Position {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Fail("member not on the roster", string(id))
	return spatial.Position{}
}

func (s *TombSuite) regionOf(cell spatial.Position) string {
	region, ok := s.enc.RegionAt(cell)
	s.Require().True(ok, "cell %v is not floor", cell)
	return region
}

func (s *TombSuite) sees(observer core.EntityID, cell spatial.Position) bool {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	var subject core.EntityID
	for _, m := range members {
		if m.Position == cell {
			subject = m.ID
		}
	}
	if subject == "" {
		return false
	}

	view, err := s.enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject != intel.Subject(subject) {
			continue
		}
		for _, via := range h.CurrentVia {
			if via == intel.Sight {
				return true
			}
		}
	}
	return false
}

func (s *TombSuite) anySkeletonSees() bool {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.Kind != encounter.KindMonster || m.Region != "hall" {
			continue
		}
		if s.sees(m.ID, s.cellOf(delve)) {
			return true
		}
	}
	return false
}

func (s *TombSuite) doorByID(id encounter.DoorID) encounter.Door {
	for _, d := range s.enc.Doors() {
		if d.ID == id {
			return d
		}
	}
	s.Require().Fail("no such door", string(id))
	return encounter.Door{}
}

func (s *TombSuite) tombDoor() encounter.Door {
	for _, d := range s.enc.Doors() {
		if _, locked := d.State.Lock(); locked {
			return d
		}
	}
	s.Require().Fail("the tomb authors a locked connector and the compile lost it")
	return encounter.Door{}
}

// doorSides is a door's two cells, near side first — the one a member walks to
// in order to be AT the door, and the one on the other side of it.
//
// Read off the door rather than computed, and then sorted by which region holds
// each: a door's edge is undirected (spatial normalizes the pair on
// registration, so From and To describe the same crossing either way round), so
// "which end is the hall's" is a question about the map, not about the order
// the compiler happened to write them in.
func (s *TombSuite) doorSides(door encounter.Door) (near, far spatial.Position) {
	s.Require().Len(door.Edges, 1, "the tomb's locked connector is one crossing wide")
	edge := door.Edges[0]

	if s.regionOf(edge.From) == "hall" {
		return edge.From, edge.To
	}

	return edge.To, edge.From
}

// canSee reports whether the line from where she stands to a cell is clear,
// asked of the LIVE canvas.
//
// The canvas is the right place to ask because this question is about a CELL
// rather than about a member: "is the chamber beyond the door dark" has nobody
// standing in it to be seen. And it must be the live canvas rather than the
// Atlas, because opening a door is precisely the thing that changes the answer
// — which is the split [encounter.Encounter.Doors] documents.
func (s *TombSuite) canSee(cell spatial.Position) bool {
	canvas, err := s.enc.Canvas()
	s.Require().NoError(err)

	return !canvas.IsLineOfSightBlocked(s.cellOf(delve), cell)
}

// cellBeside is a floor cell next to the given one that nothing solid stands
// in — where you get to when you walk up to somebody.
func (s *TombSuite) cellBeside(cell spatial.Position) spatial.Position {
	canvas, err := s.enc.Canvas()
	s.Require().NoError(err)
	solid := s.solidCells()

	for _, n := range canvas.GetGrid().GetNeighbors(cell) {
		if _, floor := s.enc.RegionAt(n); floor && !solid[n] {
			return n
		}
	}
	s.Require().Fail("nowhere to stand beside", "%v", cell)
	return spatial.Position{}
}

func (s *TombSuite) bossCell() spatial.Position {
	for i, m := range s.compiled.Monsters {
		if m.Boss {
			return s.cellOf(monsterID(i))
		}
	}
	s.Require().Fail("no boss compiled")
	return spatial.Position{}
}

// nearestMonsterCell returns a monster of the named region and a cell beside
// it — where she has to get to for the garrison to have anything to see.
func (s *TombSuite) nearestMonsterCell(region string) struct {
	on, beside spatial.Position
} {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.Kind != encounter.KindMonster || m.Region != region {
			continue
		}
		canvas, cerr := s.enc.Canvas()
		s.Require().NoError(cerr)
		for _, n := range canvas.GetGrid().GetNeighbors(m.Position) {
			if r, ok := s.enc.RegionAt(n); ok && r == region {
				return struct{ on, beside spatial.Position }{m.Position, n}
			}
		}
	}
	s.Require().Fail("no monster in region", region)
	return struct{ on, beside spatial.Position }{}
}

// walkTo walks to a cell one step at a time, THE WAY A HOST WOULD.
//
// The route is found here rather than asked for, because there is nothing to
// ask: [encounter.Encounter.Step] is one step and says so — "walking is the
// seam's job, because anything that fires because a member entered a PARTICULAR
// CELL can only be noticed by something that visits each of them in turn". This
// test is standing in for that seam, and a scene that jumped the wall would not
// be walking the dungeon, it would be teleporting through it.
//
// It is a breadth-first search rather than a greedy descent, and that is the
// whole point of it: a greedy walk toward the tomb marches east along the row
// it starts on and stops dead against the seam wall. Finding the doorway is the
// scene.
//
// IT STOPS WHEN A FIGHT FORMS, because at that moment she stops being able to
// walk: free roam is a world-clock verb and a member in a bubble acts through
// the fight's own turn structure ([encounter.Encounter.Step] refuses with
// ErrInBubble). A host's walk seam has the same interruption to handle, and a
// scene that stepped through it would be asserting something no player can do.
func (s *TombSuite) walkTo(target spatial.Position) *encounter.StepOutput {
	var last *encounter.StepOutput
	for _, step := range s.routeTo(target) {
		out, err := s.enc.Step(&encounter.StepInput{Member: delve, To: step})
		s.Require().NoError(err, "walking to %v", step)
		last = out
		if out.Formed != nil {
			return out
		}
	}

	return last
}

// walkOn is walkTo pressed through: whatever catches her on the way, she
// disengages and keeps going.
//
// A FIGHTING WITHDRAWAL, which is a thing a player does and a thing this
// composition models — free roam and a fight are two clocks, and Dissolve is
// how you get off the second one back onto the first (rpg-toolkit#964). It is a
// separate helper from [TombSuite.walkTo] rather than a flag on it because the
// two say opposite things: walkTo is used where being interrupted IS the
// assertion, and this is used where getting there is.
func (s *TombSuite) walkOn(target spatial.Position) {
	for i := 0; i < 8 && s.cellOf(delve) != target; i++ {
		s.breakOff()
		s.walkTo(target)
	}
	s.Require().Equal(target, s.cellOf(delve), "she never got there")
}

// breakOff disengages if she is fighting, and does nothing if she is not.
//
// Dissolve is also the only way to ASK. Nothing on the read surface reports a
// bubble — a fight is turn structure rather than map data — and the verb
// answers ErrNoBubble for a member who is not in one, which is exactly the
// question. Asserting on that error rather than swallowing it is what keeps
// this from hiding a real refusal.
func (s *TombSuite) breakOff() {
	if _, err := s.enc.Dissolve(&encounter.DissolveInput{Member: delve}); err != nil {
		s.Require().ErrorIs(err, encounter.ErrNoBubble, "she is either fighting or free")
	}
}

// routeTo is the cells to step onto, in order, to get from where she is to the
// target: floor only, never onto a cell something solid stands in, and never
// through a crossing something shut.
func (s *TombSuite) routeTo(target spatial.Position) []spatial.Position {
	canvas, err := s.enc.Canvas()
	s.Require().NoError(err)
	grid := canvas.GetGrid()
	shut := s.shutCrossings()
	solid := s.solidCells()

	from := s.cellOf(delve)
	prev := map[spatial.Position]spatial.Position{from: from}
	queue := []spatial.Position{from}

	for len(queue) > 0 {
		here := queue[0]
		queue = queue[1:]
		if here == target {
			var route []spatial.Position
			for at := target; at != from; at = prev[at] {
				route = append([]spatial.Position{at}, route...)
			}

			return route
		}

		for _, n := range grid.GetNeighbors(here) {
			if _, seen := prev[n]; seen {
				continue
			}
			if _, floor := s.enc.RegionAt(n); !floor {
				continue
			}
			if solid[n] && n != target {
				continue
			}
			if shut[normalizedCrossing(here, n)] {
				continue
			}
			prev[n] = here
			queue = append(queue, n)
		}
	}

	s.Require().Fail("no route", "from %v to %v", from, target)
	return nil
}

// solidCells is every cell a step cannot END on: the props that say they block
// movement.
//
// A PROP'S CELL IS STILL FLOOR — [encounter.Encounter.RegionAt] names it like
// any other, because what a prop blocks is not ownership — so "is this floor"
// and "can I stand here" are two questions, and a walk seam has to ask both.
// The tomb's four pillars are what makes that concrete: they sit in the middle
// of the hall, RegionAt calls all four of them "hall", and walking into one is
// ErrBadPlacement.
func (s *TombSuite) solidCells() map[spatial.Position]bool {
	atlas, err := s.enc.Atlas()
	s.Require().NoError(err)

	out := map[spatial.Position]bool{}
	for _, p := range atlas.Props {
		if p.BlocksMovement {
			out[p.At] = true
		}
	}

	return out
}

// shutCrossings is every crossing a step cannot cross right now: the walls the
// chambers drew, plus the doors that are shut at this moment.
//
// Read off the two surfaces a HOST reads, deliberately. The Atlas carries the
// authored walls, which never change; [encounter.Encounter.Doors] carries the
// live state of each door, which is exactly the thing this scene changes
// mid-way — and reading walls from a construction snapshot while reading doors
// live is the split the door API documents rather than a workaround for it.
func (s *TombSuite) shutCrossings() map[[2]spatial.Position]bool {
	atlas, err := s.enc.Atlas()
	s.Require().NoError(err)

	out := map[[2]spatial.Position]bool{}
	for _, b := range atlas.Boundaries {
		if b.BlocksMovement {
			out[normalizedCrossing(b.From, b.To)] = true
		}
	}
	for _, d := range s.enc.Doors() {
		if d.State.Kind() == encounter.DoorOpen {
			continue
		}
		for _, e := range d.Edges {
			out[normalizedCrossing(e.From, e.To)] = true
		}
	}

	return out
}

// normalizedCrossing orders an edge's two endpoints so either direction hashes
// to the same key — a wall has no side.
func normalizedCrossing(a, b spatial.Position) [2]spatial.Position {
	if a.X < b.X || (a.X == b.X && a.Y < b.Y) {
		return [2]spatial.Position{a, b}
	}

	return [2]spatial.Position{b, a}
}
