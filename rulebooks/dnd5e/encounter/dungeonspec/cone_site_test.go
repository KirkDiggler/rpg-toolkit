// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// cone_site_test.go plays the studio-authored cone site (rpg-project#621,
// sneaking design R2, R10, R14): a knight given an arc of 120 watches the
// third of the room in front of it, so a sneaking rogue walks up behind it
// unrolled and unseen and rolls the moment it steps into that third — while
// the knight's fog, and its sight of a rogue who is not sneaking, stay all
// round, and a cook that already sees the rogue keeps seeing it.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const coneSiteFixture = "testdata/cone-site.yaml"

const (
	coneSiteRogue  = encounter.MemberID("rogue")
	coneSiteWalker = encounter.MemberID("walker")
	coneSiteKnight = encounter.MemberID("knight")
	coneSiteCook   = encounter.MemberID("cook")
)

// coneSiteArc is the knight's and the cook's arc this host supplies: the
// session's seed (design O6), scripted here.
const coneSiteArc = 120

// coneSiteSight is the site's light: the players see twelve cells, the
// knight four, the cook two.
type coneSiteSight struct{}

func (coneSiteSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 12
		switch id {
		case coneSiteKnight:
			out[id] = 4
		case coneSiteCook:
			out[id] = 2
		}
	}

	return out, nil
}

// coneSiteSheets answers thirty feet (halved while sneaking, the session's
// rule), the knight's passive 10 and the cook's 9 unless deaf names them —
// then 0, not supplied — a re-arm distance of two, and each member's arc from
// arcs. The scene changes both to ask what an answer of zero means.
type coneSiteSheets struct {
	enc  **encounter.Encounter
	arcs map[encounter.MemberID]int
	deaf map[encounter.MemberID]bool
}

func (s coneSiteSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	passive := map[encounter.MemberID]int{coneSiteRogue: 12, coneSiteWalker: 12, coneSiteKnight: 10, coneSiteCook: 9}
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		speed := 30
		if *s.enc != nil && (*s.enc).Sneaking(id) {
			speed /= 2
		}
		out[id] = encounter.SheetFacts{
			SpeedFeet:         speed,
			Actions:           []encounter.ActionView{{Ref: kingSpear, RangeFeet: 5}},
			PassivePerception: map[bool]int{false: passive[id]}[s.deaf[id]],
			WatchResetCells:   2,
			WatchArc:          s.arcs[id],
		}
	}

	return out, nil
}

// coneSite is one run of the site.
type coneSite struct {
	*sneakSite
	arcs map[encounter.MemberID]int
	deaf map[encounter.MemberID]bool
}

// openTheConeSite seats the rogue at the party start, the walker — a second
// rogue who never sneaks — two cells behind the knight, and the cast where
// the file posts them, with the rogue's d20s scripted.
func openTheConeSite(t *testing.T, d20s ...int) *coneSite {
	t.Helper()
	compiled := loadContent(t, coneSiteFixture)
	members := []encounter.MemberInput{
		{ID: coneSiteRogue, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At},
		{ID: coneSiteWalker, Kind: encounter.KindPlayer, Position: castleCell(2, 2)},
	}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Facing: m.FacingDegrees,
		})
	}
	site := &coneSite{
		sneakSite: &sneakSite{t: t, stealth: &sneakSiteStealth{d20s: d20s}},
		arcs:      map[encounter.MemberID]int{coneSiteKnight: coneSiteArc, coneSiteCook: coneSiteArc},
		deaf:      map[encounter.MemberID]bool{},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     compiled.Field,
		Members:   members,
		Endings:   append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         coneSiteSight{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        coneSiteSheets{enc: &site.enc, arcs: site.arcs, deaf: site.deaf},
			Participation: &castleStanding{down: map[encounter.MemberID]bool{}},
			Initiative:    orderAsGiven{},
			Driver:        encounter.TableDriver{Roller: rollsOne{}},
			Roller:        rollsOne{},
			CheckResolver: site.stealth,
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   cellsStriker{calls: &[]cellsStrike{}},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)
	site.enc = enc

	return site
}

// step takes the rogue one cell, a walk of its own.
func (s *coneSite) step(to spatial.Position) {
	s.t.Helper()
	_, err := s.enc.Step(&encounter.StepInput{Member: coneSiteRogue, To: to, EndWalk: true})
	require.NoError(s.t, err, "the rogue steps to %v", to)
}

// declare sets the rogue's mode.
func (s *coneSite) declare(sneaking bool) {
	s.t.Helper()
	out, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: coneSiteRogue, Sneaking: sneaking})
	require.NoError(s.t, err)
	require.Equal(s.t, sneaking, out.Sneaking)
}

// seesLive reports whether observer holds subject live on sight now.
func (s *coneSite) seesLive(observer, subject encounter.MemberID) bool {
	s.t.Helper()
	h := s.holding(observer, subject)

	return h != nil && len(h.CurrentVia) > 0
}

// zoneOf is a watcher's zone as the rogue's red floor sends it, as a set.
func (s *coneSite) zoneOf(watcher encounter.MemberID) map[spatial.Position]bool {
	s.t.Helper()
	for _, zone := range zonesOf(s.t, s.enc, coneSiteRogue) {
		if zone.Watcher == watcher {
			return cellSet(zone.Cells)
		}
	}
	require.FailNowf(s.t, "no zone", "the rogue is sent no zone for %s", watcher)

	return nil
}

// seenNow is a member's fog-of-war seen-now cells, as a set.
func (s *coneSite) seenNow(member encounter.MemberID) map[spatial.Position]bool {
	s.t.Helper()
	out, err := s.enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: member})
	require.NoError(s.t, err)

	return cellSet(out.Cells)
}

// verdict is the rogue's stored pair against a watcher.
func (s *coneSite) verdict(watcher encounter.MemberID) (encounter.SneakVerdictData, bool) {
	v, ok := s.enc.ToData().SneakVerdicts[coneSiteRogue][watcher]

	return v, ok
}

// coneBearing is the bearing from one cell to another on the site's
// pointy-top floor, map north zero, clockwise — the test's own reading of the
// frame facing.go turns members in.
func coneBearing(from, to spatial.Position) float64 {
	plane := spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{
		Orientation: spatial.HexOrientationPointyTop, CellWidth: encounter.FeetPerCell,
	})
	east, _ := plane.Bearing(from, to)
	north := east + 90
	for north >= 360 {
		north -= 360
	}
	for north < 0 {
		north += 360
	}

	return north
}

// TestConeSite is the cone slice's scenario, on a site authored in the
// studio, with an arc of 120 scripted for the knight and the cook.
func TestConeSite(t *testing.T) {
	s := openTheConeSite(t, 1)
	knightAt := positionOf(t, s.enc, coneSiteKnight)
	require.Equal(t, castleCell(3, 0), knightAt)
	require.InDelta(t, 0, facingOf(t, s.enc, coneSiteKnight), 1e-9, "the knight faces the authored north")

	behind := castleCell(3, 2)     // due south: 180 off the knight's facing
	sideways := castleCell(5, 0)   // due east: 90 off, outside the front third
	frontEdge := castleCell(4, -1) // 60 off: the edge of the front third
	inFront := castleCell(3, -2)   // due north
	require.InDelta(t, 180, coneBearing(knightAt, behind), 1e-6)
	require.InDelta(t, 90, coneBearing(knightAt, sideways), 1e-6)
	require.InDelta(t, 60, coneBearing(knightAt, frontEdge), 1e-6)
	require.InDelta(t, 0, coneBearing(knightAt, inFront), 1e-6)

	// A ROGUE WHO IS NOT SNEAKING IS SEEN AS TODAY (R2): the walker stands
	// behind the knight, inside its sight, and the knight holds it live.
	walkerAt := positionOf(t, s.enc, coneSiteWalker)
	require.Greater(t, coneBearing(knightAt, walkerAt), 150.0, "the walker stands behind the knight")
	require.True(t, s.seesLive(coneSiteKnight, coneSiteWalker), "the knight sees the walker behind it")

	// BEHIND THE KNIGHT: NO ROLL, NOT SEEN. Declared out of every watcher's
	// sight, the rogue walks up to the cell behind the knight, inside its
	// ordinary sight the whole last stretch.
	require.False(t, s.seesLive(coneSiteKnight, coneSiteRogue), "the rogue starts out of the knight's sight")
	s.declare(true)
	s.walkTo(behind)
	require.Equal(t, behind, positionOf(t, s.enc, coneSiteRogue))
	require.Empty(t, s.stealth.asked, "behind the knight nothing rolls")
	require.False(t, s.seesLive(coneSiteKnight, coneSiteRogue), "the knight does not see the sneaker behind it")
	require.Nil(t, s.holding(coneSiteKnight, coneSiteRogue), "the knight's holdings never gained the rogue")
	armed, judged := s.verdict(coneSiteKnight)
	require.True(t, judged, "the knight's sight reaching the rogue outside its zone records the pair")
	require.Equal(t, encounter.SneakVerdictData{Armed: true}, armed, "armed and un-rolled: unseen, and the next entry rolls (R28)")
	require.True(t, s.seesLive(coneSiteKnight, coneSiteWalker), "the walker beside it is still seen")

	// The knight's zone is its front third: the cell ahead and the edge cell
	// are in it, the cells behind and beside are not; its own cell is.
	zone := s.zoneOf(coneSiteKnight)
	require.True(t, zone[inFront], "the cell ahead is in the knight's zone")
	require.True(t, zone[frontEdge], "a cell exactly half the arc off is in it: the edge is inclusive")
	require.True(t, zone[knightAt], "the knight's own cell is never in question")
	require.False(t, zone[behind], "the cell behind is not")
	require.False(t, zone[sideways], "the cell beside is not")

	// FOG IS ALL ROUND (R14). The knight's own seen-now cells hold the cell
	// behind it and are the same set with its arc answered as zero; so are
	// the rogue's.
	knightFog, rogueFog := s.seenNow(coneSiteKnight), s.seenNow(coneSiteRogue)
	require.True(t, knightFog[behind], "the knight's fog sees the floor behind it")
	require.True(t, knightFog[walkerAt], "and the walker's cell")

	// A KNIGHT ANSWERED WITH ZERO WATCHES ALL ROUND.
	s.arcs[coneSiteKnight] = 0
	require.Equal(t, knightFog, s.seenNow(coneSiteKnight), "the arc never touches the knight's fog")
	require.Equal(t, rogueFog, s.seenNow(coneSiteRogue), "nor the rogue's")
	whole := s.zoneOf(coneSiteKnight)
	require.True(t, whole[behind], "with no arc the cell behind is in the knight's zone")
	require.True(t, whole[sideways], "and the cell beside")
	s.arcs[coneSiteKnight] = coneSiteArc

	// INTO THE FRONT THIRD: THE ROLL. Round the knight's east side, each cell
	// in its ordinary sight; the cell 90 off rolls nothing, the cell 60 off
	// rolls. A 1 is 8 against the knight's passive 10: lost, and seen.
	s.step(castleCell(3, 1))
	s.step(castleCell(4, 1))
	s.step(sideways)
	require.Empty(t, s.stealth.asked, "beside the knight nothing rolls")
	require.False(t, s.seesLive(coneSiteKnight, coneSiteRogue), "nor does it see the rogue beside it")
	s.step(frontEdge)
	require.NotEmpty(t, s.stealth.asked, "stepping into the front third rolls")
	require.Equal(t, []encounter.CheckApproach{{Ability: "stealth", DC: 10}}, s.stealth.asked[0].Approaches,
		"against the knight's passive")
	require.True(t, s.seesLive(coneSiteKnight, coneSiteRogue), "the lost roll: the knight sees the rogue")
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, coneSiteRogue), "a civil knight forms no fight")
	rolled := append([]encounter.ResolveCheckInput(nil), s.stealth.asked...)

	// DECLARING BEHIND A WATCHER THAT SEES YOU KEEPS YOU SEEN (R10). Not
	// sneaking, the rogue walks up behind the cook, inside her ordinary sight
	// and outside her front third, and declares.
	s.declare(false)
	cookAt := positionOf(t, s.enc, coneSiteCook)
	behindCook := castleCell(-5, 2)
	require.InDelta(t, 180, coneBearing(cookAt, behindCook), 1e-6)
	s.walkTo(behindCook)
	require.Equal(t, behindCook, positionOf(t, s.enc, coneSiteRogue))
	require.True(t, s.seesLive(coneSiteCook, coneSiteRogue), "the cook sees the rogue behind her")
	s.declare(true)
	v, ok := s.verdict(coneSiteCook)
	require.True(t, ok, "declaring in the cook's sight writes a verdict")
	require.Equal(t, encounter.SneakVerdictData{Beaten: false, Armed: false, InZone: false}, v,
		"lost with no roll, outside her zone")
	require.Equal(t, rolled, s.stealth.asked, "declaring in plain view rolls nothing")
	require.True(t, s.seesLive(coneSiteCook, coneSiteRogue), "the cook keeps holding the rogue")

	// Still in her sight, still behind her: still seen, not re-armed.
	s.step(castleCell(-4, 2))
	require.True(t, s.seesLive(coneSiteCook, coneSiteRogue), "inside her sight the lost pair holds")
	v, _ = s.verdict(coneSiteCook)
	require.False(t, v.Armed, "a lost pair inside ordinary sight does not re-arm")

	// Out of her sight and the re-arm distance clear of her zone: re-armed.
	s.walkTo(castleCell(-5, 5))
	require.False(t, s.seesLive(coneSiteCook, coneSiteRogue), "out of her sight")
	v, _ = s.verdict(coneSiteCook)
	require.True(t, v.Armed, "broken sight and the distance clear: the pair re-arms")

	// Back behind her, armed: unseen, and nothing rolls.
	s.walkTo(behindCook)
	require.Equal(t, behindCook, positionOf(t, s.enc, coneSiteRogue))
	require.False(t, s.seesLive(coneSiteCook, coneSiteRogue), "re-armed, the sneaker behind her is unseen")
	require.Equal(t, rolled, s.stealth.asked, "behind her nothing rolls")

	// A WATCHER THAT HOLDS THE SNEAKER IS NEVER ANSWERED BY THE ARC (R10,
	// lead ruling R28). With no passive supplied the knight judges no sneak
	// and sees the rogue by ordinary sight, so the rogue walks up behind it
	// held live. Supplied again, the knight still holds the rogue: the arc
	// does not make a creature that sees you lose you.
	s.deaf[coneSiteKnight] = true
	s.walkTo(behind)
	require.Equal(t, behind, positionOf(t, s.enc, coneSiteRogue))
	require.True(t, s.seesLive(coneSiteKnight, coneSiteRogue), "a knight with no passive sees the sneaker behind it")
	delete(s.deaf, coneSiteKnight)
	stillBehind := castleCell(4, 2)
	require.Greater(t, offArc(0, coneBearing(knightAt, stillBehind)), 60.0, "the next cell is outside the front third")
	s.step(stillBehind)
	require.True(t, s.seesLive(coneSiteKnight, coneSiteRogue), "held live, the sneaker behind it stays seen")
	_, judged = s.verdict(coneSiteKnight)
	require.False(t, judged, "no armed pair is written against a watcher that holds the mover")
	require.Equal(t, rolled, s.stealth.asked, "and nothing rolls")
}
