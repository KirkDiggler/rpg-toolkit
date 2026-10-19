// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// pause_facing_site_test.go plays the studio-authored pause-facing site
// (rpg-project#624): a guard pacing a loop whose east waypoint states
// `wait: 2, facing: s`. By Kirk's rulings of 2026-10-19 a pause is a dwell —
// the patrol walks to the waypoint, stands there two rounds, then goes on —
// and the facing applies only while he stands there: walking, and at a walk
// that ends mid-leg, he faces his step as before (sneaking design R17–R20).
// With an arc of 120 scripted, the dwell is what keeps a sneaking rogue south
// of the waypoint in his wedge, and walking on is what takes her out of it.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const pauseFacingSiteFixture = "testdata/pause-facing-site.yaml"

const (
	pauseFacingRogue = encounter.MemberID("rogue")
	pauseFacingGuard = encounter.MemberID("guard")
)

// The site's cells, axial as the file writes them.
var (
	pauseFacingPost   = spatial.Position{X: -6, Y: 0} // the first waypoint, where he is posted
	pauseFacingFaced  = spatial.Position{X: 2, Y: 0}  // the waypoint that states `wait: 2, facing: s`
	pauseFacingSouth  = spatial.Position{X: 1, Y: 2}  // due south of the faced waypoint
	pauseFacingSouth2 = spatial.Position{X: 1, Y: 3}  // the rogue's other pacing cell
)

// pauseFacingSight lets everyone see twelve cells.
type pauseFacingSight struct{}

func (pauseFacingSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 12
	}

	return out, nil
}

// pauseFacingSheets answers thirty feet (halved while sneaking), a passive
// of 10, and the guard's arc of 120 — the session's seed, scripted here.
type pauseFacingSheets struct{ enc **encounter.Encounter }

func (s pauseFacingSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		speed := 30
		if *s.enc != nil && (*s.enc).Sneaking(id) {
			speed /= 2
		}
		arc := 0
		if id == pauseFacingGuard {
			arc = 120
		}
		out[id] = encounter.SheetFacts{
			SpeedFeet:         speed,
			Actions:           []encounter.ActionView{{Ref: kingSpear, RangeFeet: 5}},
			PassivePerception: 10,
			WatchResetCells:   2,
			WatchArc:          arc,
		}
	}

	return out, nil
}

// authoredAt is an axial cell as the authored offset pair a member is seated
// with ([encounter.MemberInput.Position]).
func authoredAt(cell spatial.Position) spatial.Position {
	col, row := encounter.OffsetOf(encounter.HexesArePointyTop(), cell)

	return spatial.Position{X: float64(col), Y: float64(row)}
}

// openThePauseFacingSite seats the rogue at the cell due south of the faced
// waypoint and the guard where the file posts him, with the rogue's d20s
// scripted high: she is never seen, so nothing about the guard's walk
// changes.
func openThePauseFacingSite(t *testing.T) *sneakSite {
	t.Helper()
	compiled := loadContent(t, pauseFacingSiteFixture)
	members := []encounter.MemberInput{
		{ID: pauseFacingRogue, Kind: encounter.KindPlayer, Position: authoredAt(pauseFacingSouth)},
	}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Facing: m.FacingDegrees,
		})
	}
	site := &sneakSite{t: t, stealth: &sneakSiteStealth{d20s: []int{20, 20, 20, 20, 20, 20, 20, 20}}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        compiled.Field,
		Members:      members,
		Endings:      append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention:    encounter.RetentionUnbounded,
		Capabilities: pauseFacingCapabilities(site),
	})
	require.NoError(t, err)
	site.enc = enc

	return site
}

// seenFacing is the facing the rogue's own sighting of the guard carries.
func seenFacing(t *testing.T, s *sneakSite) float64 {
	t.Helper()
	seen := sightingOf(t, s.enc, pauseFacingRogue, pauseFacingGuard)
	require.NotNil(t, seen.Facing, "the rogue's sighting of the guard carries his facing")

	return *seen.Facing
}

// pauseFacingCapabilities is the site's host, for a fresh run and a loaded
// one alike.
func pauseFacingCapabilities(site *sneakSite) encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         pauseFacingSight{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        pauseFacingSheets{enc: &site.enc},
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
	}
}

// reload saves the run and loads it again with the same host.
func reloadPauseFacingSite(t *testing.T, site *sneakSite) {
	t.Helper()
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: site.enc.ToData(), Capabilities: pauseFacingCapabilities(site),
	})
	require.NoError(t, err)
	site.enc = loaded
}

// TestPauseFacingSite is the slice's scenario, on a site authored in the
// studio.
func TestPauseFacingSite(t *testing.T) {
	// THE WAYPOINT SPEAKS A PLACEMENT'S WORDS, and is refused in a
	// placement's sentence.
	raw, err := os.ReadFile(pauseFacingSiteFixture)
	require.NoError(t, err)
	_, err = dungeonspec.Load([]byte(strings.Replace(string(raw), "facing: s }", "facing: south }", 1)))
	require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
	require.ErrorContains(t, err, `tables.pacing.on.time[0].toward.path[1].facing: "south" is not a compass direction: `+
		"a facing is one of n|ne|e|se|s|sw|w|nw")
	_, err = dungeonspec.Load([]byte(strings.Replace(string(raw), "r: 0, wait: 2,", "r: 0, wait: -1,", 1)))
	require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
	require.ErrorContains(t, err, "tables.pacing.on.time[0].toward.path[1].wait: "+
		"a wait is how many rounds the walker stands here, and -1 is negative")

	_, err = dungeonspec.Load([]byte(strings.Replace(string(raw), "r: 0, wait: 2, facing: s }", "r: 0, facing: s }", 1)))
	require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
	require.ErrorContains(t, err, "tables.pacing.on.time[0].toward.path[1].facing: "+
		"a facing is taken while standing; give the waypoint a wait")

	s := openThePauseFacingSite(t)
	require.Equal(t, pauseFacingPost, positionOf(t, s.enc, pauseFacingGuard))
	require.Equal(t, pauseFacingSouth, positionOf(t, s.enc, pauseFacingRogue))
	require.InDelta(t, 90, coneBearing(pauseFacingPost, pauseFacingFaced), 1e-6, "the leg into the faced waypoint runs east")
	require.InDelta(t, 180, coneBearing(pauseFacingFaced, pauseFacingSouth), 1e-6, "the rogue stands due south of it")

	out, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: pauseFacingRogue, Sneaking: true})
	require.NoError(t, err)
	require.True(t, out.Sneaking)

	// THE RULINGS ARE ASKED OF EVERY ROUND OF HIS. The rogue paces between
	// her two cells, both south of the faced waypoint, and after each of her
	// steps the guard is judged by where he stands and the dwell his record
	// carries. How far a round carries him is the world's pacing, not this
	// slice's, so the scenario names his rounds by what they are rather than
	// by number.
	var (
		midEastLeg, walkedOn, reloaded bool
		dwellLeft                      = map[int]bool{}
		last                           = positionOf(t, s.enc, pauseFacingGuard)
		lastDwell                      *encounter.DwellData
	)
	for range 90 {
		to := pauseFacingSouth2
		if positionOf(t, s.enc, pauseFacingRogue) == pauseFacingSouth2 {
			to = pauseFacingSouth
		}
		_, err := s.enc.Step(&encounter.StepInput{Member: pauseFacingRogue, To: to, EndWalk: true})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, pauseFacingRogue), "a civil guard forms no fight")

		at := positionOf(t, s.enc, pauseFacingGuard)
		facing := facingOf(t, s.enc, pauseFacingGuard)
		dwell := dwellOf(t, s.enc, pauseFacingGuard)
		moved := at != last
		from := last
		last = at
		rogueAt := positionOf(t, s.enc, pauseFacingRogue)
		inWedge := (&coneSite{sneakSite: s}).zoneOf(pauseFacingGuard)[rogueAt]
		if moved || dwell != nil {
			require.InDelta(t, facing, seenFacing(t, s), 1e-9, "the rogue's sighting testifies to his facing at %v", at)
		}

		switch {
		case dwell != nil:
			// STANDING: on the waypoint, facing its south, the sneaking rogue
			// south of him in his wedge, round after round.
			require.Equal(t, pauseFacingFaced, at, "a dwell stands on its waypoint")
			require.Equal(t, pauseFacingFaced, spatial.Position{X: dwell.Cell.X, Y: dwell.Cell.Y})
			require.InDelta(t, 180, facing, 1e-9, "standing at the waypoint he faces the stated south")
			require.LessOrEqual(t, offArc(180, coneBearing(at, rogueAt)), 60.0, "the rogue is south of him")
			require.True(t, inWedge, "while he stands facing south, the sneaking rogue's cell is in his wedge")
			if lastDwell == nil {
				require.True(t, moved, "the dwell begins on the walk that ends on the waypoint")
				require.Equal(t, 2, dwell.Rounds, "the waypoint's two rounds are ahead of him")
				// SAVED AND LOADED MID-DWELL: the loaded run stands out the
				// rest of it, below, exactly as this one would have.
				reloadPauseFacingSite(t, s)
				reloaded = true
				require.Equal(t, dwell, dwellOf(t, s.enc, pauseFacingGuard), "the dwell survives the reload")
			}
			dwellLeft[dwell.Rounds] = true
		case lastDwell != nil:
			// THE DWELL SPENT, HE WALKS ON, facing his step, and the rogue is
			// out of the wedge.
			walkedOn = true
			require.Equal(t, pauseFacingFaced, from, "he walks on from the waypoint")
			require.NotEqual(t, pauseFacingFaced, at, "the round after his last standing round walks him on")
			require.NotEqual(t, 180.0, facing, "walking on, he faces his step and not the pause")
			require.False(t, inWedge, "walked on, he no longer watches the rogue's cell")
		case moved && at.Y == 0 && at.X > pauseFacingPost.X && at.X < pauseFacingFaced.X:
			// A WALK THAT ENDS MID-LEG FACES ITS LAST STEP: east, though the
			// waypoint it walks to states south.
			midEastLeg = true
			require.InDelta(t, 90, facing, 1e-9, "mid-leg on the way to the waypoint he faces his step, east")
		case moved:
			require.NotEqual(t, pauseFacingFaced, at, "a walk that ends on the waypoint begins a dwell")
			require.NotEqual(t, 180.0, facing, "away from the waypoint he never faces its south, at %v", at)
		}
		lastDwell = dwell
	}
	require.True(t, midEastLeg, "a round ended mid-leg on the way to the waypoint")
	require.Equal(t, map[int]bool{2: true, 1: true, 0: true}, dwellLeft,
		"he stood on the waypoint for the rest of the round he arrived in and two rounds more")
	require.True(t, walkedOn, "and then walked on")
	require.True(t, reloaded, "the run was reloaded mid-dwell")
}

// dwellOf is a member's dwell as the run persists it, or nil.
func dwellOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) *encounter.DwellData {
	t.Helper()
	for _, m := range enc.ToData().Members {
		if m.ID == id {
			return m.Dwell
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return nil
}
