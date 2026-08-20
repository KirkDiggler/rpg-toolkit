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
// The dungeon it walks is tombsource_test.go's, which is rpg-api's shipping
// reference-tomb.yaml plus the two declarations this composition requires and
// cannot invent. That file carries the caveat about being a copy.

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
	compiled, err := dungeonspec.Load([]byte(tombYAML))
	s.Require().NoError(err, "the shipping tomb must compile")
	s.compiled = compiled

	members := []encounter.MemberInput{{
		ID: delve, Kind: encounter.KindPlayer,
		Room: compiled.PartyStart[0].Region, Position: compiled.PartyStart[0].At,
	}}
	for i, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: monsterID(i), Kind: encounter.KindMonster, Room: m.Region, Position: m.At,
		})
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
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

	// ── she walks to the hall, and the skeletons see her ──────────────────
	nearest := s.nearestMonsterCell("hall")
	out := s.walkTo(nearest.beside)
	s.Require().Equal("hall", s.regionOf(s.cellOf(delve)), "she is in the hall")

	s.True(s.anySkeletonSees(), "the garrison sees her")
	s.Require().NotNil(out.Formed, "and a fight forms on the sight of her, unasked")

	// ── the locked connector refuses, and says what it would take ─────────
	door := s.tombDoor()
	s.Require().Equal(encounter.DoorStateKind("locked"), door.State.Kind())

	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: door.ID})
	s.Require().ErrorIs(err, encounter.ErrLocked, "the tomb is shut")
	s.Require().Contains(err.Error(), "DC 12", "and the refusal says what it would take")

	beyond := s.bossCell()
	s.False(s.sees(delve, beyond), "the boss chamber is dark through a shut door")

	// ── beaten, it opens and reveals the chamber ──────────────────────────
	failed, err := s.enc.Unlock(&encounter.UnlockInput{Door: door.ID, Beaten: false})
	s.Require().NoError(err)
	s.False(failed.Beaten, "a missed check leaves it locked")
	s.Equal(tombLockDC, failed.DC, "and reports the DC it carries")

	beaten, err := s.enc.Unlock(&encounter.UnlockInput{Door: door.ID, Beaten: true})
	s.Require().NoError(err)
	s.True(beaten.Beaten)

	_, err = s.enc.OpenDoor(&encounter.OpenDoorInput{Door: door.ID})
	s.Require().NoError(err, "and now it opens")

	s.True(s.sees(delve, beyond), "the boss chamber is revealed")

	// ── and she reaches the captain ───────────────────────────────────────
	s.walkTo(beyond)
	s.Require().Equal("tomb", s.regionOf(s.cellOf(delve)), "she stands in the tomb")
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
	for _, region := range atlas.Regions {
		for _, p := range region.Props {
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

func (s *TombSuite) tombDoor() encounter.Door {
	for _, d := range s.enc.Doors() {
		if _, locked := d.State.Lock(); locked {
			return d
		}
	}
	s.Require().Fail("the tomb authors a locked connector and the compile lost it")
	return encounter.Door{}
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

// walkTo steps toward a cell one step at a time, the way a walk seam does —
// Step itself does not check adjacency, and a scene that jumped would not be
// walking the dungeon, it would be teleporting through it.
func (s *TombSuite) walkTo(target spatial.Position) *encounter.StepOutput {
	canvas, err := s.enc.Canvas()
	s.Require().NoError(err)
	grid := canvas.GetGrid()

	var last *encounter.StepOutput
	for i := 0; i < 200; i++ {
		here := s.cellOf(delve)
		if here == target {
			return last
		}

		best := here
		bestDist := grid.Distance(here, target)
		for _, n := range grid.GetNeighbors(here) {
			if _, floor := s.enc.RegionAt(n); !floor {
				continue
			}
			if d := grid.Distance(n, target); d < bestDist {
				best, bestDist = n, d
			}
		}
		s.Require().NotEqual(here, best, "stuck at %v walking to %v", here, target)

		out, serr := s.enc.Step(&encounter.StepInput{Member: delve, To: best})
		s.Require().NoError(serr, "walking %v -> %v", here, best)
		if out.Formed != nil {
			last = out
		} else if last == nil {
			last = out
		}
	}
	s.Require().Fail("walk did not terminate")
	return nil
}
