// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// pause_facing_site_test.go plays the studio-authored pause-facing site
// (rpg-project#624): a guard pacing a loop whose east waypoint states
// `facing: s`. By Kirk's ruling of 2026-10-19 the facing applies only while
// he is paused on that waypoint: a walk that ends there turns him south; a
// walk that ends mid-leg, or on a waypoint that states nothing, faces its
// last step as before (sneaking design R17–R20). With an arc of 120 scripted,
// the turn is what puts a sneaking rogue south of the waypoint in his wedge,
// and walking on is what takes her out of it.

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
	pauseFacingPost    = spatial.Position{X: -6, Y: 0} // the first waypoint, where he is posted
	pauseFacingFaced   = spatial.Position{X: 2, Y: 0}  // the waypoint that states `facing: s`
	pauseFacingUnfaced = spatial.Position{X: -2, Y: 4} // the waypoint that states none
	pauseFacingSouth   = spatial.Position{X: 1, Y: 2}  // due south of the faced waypoint
	pauseFacingSouth2  = spatial.Position{X: 1, Y: 3}  // the rogue's other pacing cell
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
		Field:     compiled.Field,
		Members:   members,
		Endings:   append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
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
		},
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

	s := openThePauseFacingSite(t)
	require.Equal(t, pauseFacingPost, positionOf(t, s.enc, pauseFacingGuard))
	require.Equal(t, pauseFacingSouth, positionOf(t, s.enc, pauseFacingRogue))
	require.InDelta(t, 90, coneBearing(pauseFacingPost, pauseFacingFaced), 1e-6, "the leg into the faced waypoint runs east")
	require.InDelta(t, 180, coneBearing(pauseFacingFaced, pauseFacingSouth), 1e-6, "the rogue stands due south of it")

	out, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: pauseFacingRogue, Sneaking: true})
	require.NoError(t, err)
	require.True(t, out.Sneaking)

	// THE RULING IS ASKED OF EVERY STOP HE MAKES. The rogue paces between
	// her two cells, both south of the faced waypoint, and each time the
	// guard's world round moves him the stop is judged. How far a round
	// carries him is the world's pacing, not this slice's, so the scenario
	// names the stops by where they are rather than by which round.
	var (
		midEastLeg, onFaced, onUnfaced, walkedOnFromFaced, passedFaced bool
		lastStop                                                       = positionOf(t, s.enc, pauseFacingGuard)
	)
	for range 80 {
		to := pauseFacingSouth2
		if positionOf(t, s.enc, pauseFacingRogue) == pauseFacingSouth2 {
			to = pauseFacingSouth
		}
		_, err := s.enc.Step(&encounter.StepInput{Member: pauseFacingRogue, To: to, EndWalk: true})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, pauseFacingRogue), "a civil guard forms no fight")
		stop := positionOf(t, s.enc, pauseFacingGuard)
		if stop == lastStop {
			continue
		}
		from := lastStop
		lastStop = stop
		facing := facingOf(t, s.enc, pauseFacingGuard)
		require.InDelta(t, facing, seenFacing(t, s), 1e-9, "the rogue's sighting testifies to his facing at %v", stop)
		rogueAt := positionOf(t, s.enc, pauseFacingRogue)
		inWedge := (&coneSite{sneakSite: s}).zoneOf(pauseFacingGuard)[rogueAt]

		switch {
		case stop == pauseFacingFaced:
			// A WALK THAT ENDS ON THE FACED WAYPOINT, ITS MOVEMENT SPENT,
			// TURNS TO ITS FACING: the leg in runs east, and he pauses
			// facing south, with the sneaking rogue south of him in his
			// wedge.
			onFaced = true
			require.InDelta(t, 180, facing, 1e-9, "paused on the faced waypoint he faces the stated south")
			require.LessOrEqual(t, offArc(180, coneBearing(stop, rogueAt)), 60.0, "the rogue is south of him")
			require.True(t, inWedge, "while he pauses facing south, the sneaking rogue's cell is in his wedge")
		case from == pauseFacingFaced:
			// WALKING ON, HE FACES HIS STEP, and the rogue is out of the
			// wedge.
			walkedOnFromFaced = true
			require.NotEqual(t, 180.0, facing, "walking on, he faces his step and not the pause")
			require.False(t, inWedge, "walked on, he no longer watches the rogue's cell")
		case stop == pauseFacingUnfaced:
			// A WAYPOINT WITH NO FACING CHANGES NOTHING: his last step into
			// it ran south-west.
			onUnfaced = true
			require.InDelta(t, 210, facing, 1e-9, "on the unfaced waypoint he faces his step")
		case stop.Y == 0 && stop.X > pauseFacingPost.X && stop.X < pauseFacingFaced.X:
			// A WALK THAT ENDS MID-LEG FACES ITS LAST STEP: east, though the
			// waypoint it walks to states south.
			midEastLeg = true
			require.InDelta(t, 90, facing, 1e-9, "mid-leg on the way to the faced waypoint he faces his step, east")
		default:
			require.NotEqual(t, 180.0, facing, "away from the faced waypoint he never faces its south, at %v", stop)
		}
		// PASSING THROUGH IS NOT PAUSING: a round that carried him over the
		// faced waypoint and on is judged by the stop it ended on, above.
		if from.Y == 0 && from.X < pauseFacingFaced.X && stop != pauseFacingFaced && !(stop.Y == 0 && stop.X < pauseFacingFaced.X) {
			passedFaced = true
		}
	}
	require.True(t, midEastLeg, "a round ended mid-leg on the way to the faced waypoint")
	require.True(t, onFaced, "a round ended on the faced waypoint")
	require.True(t, walkedOnFromFaced, "and the next round walked him on")
	require.True(t, onUnfaced, "a round ended on the unfaced waypoint")
	require.True(t, passedFaced, "a round carried him over the faced waypoint without pausing")
}
