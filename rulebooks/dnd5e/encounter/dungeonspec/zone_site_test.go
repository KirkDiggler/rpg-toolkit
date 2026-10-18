// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// zone_site_test.go plays the studio-authored zone site (rpg-project#619,
// sneaking design R9, R15): a sneaking rogue sees the sentry pacing its own
// room and not the warden behind the wall, and is sent the sentry's red floor
// alone, cut to the floor the rogue knows, moving as the sentry walks.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const zoneSiteFixture = "testdata/zone-site.yaml"

const (
	zoneSiteRogue  = encounter.MemberID("rogue")
	zoneSiteSentry = encounter.MemberID("sentry")
	zoneSiteWarden = encounter.MemberID("warden")
)

// zoneSiteSentrySight is the sentry's sight in cells: three, so its zone
// never reaches the rogue's corner, but from the east of its loop it reaches
// through the doorway onto far-room floor the rogue has never seen.
const zoneSiteSentrySight = 3

// zoneSiteSight is the site's light: the rogue sees twelve cells, the sentry
// three, the warden six.
type zoneSiteSight struct{}

func (zoneSiteSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 12
		switch id {
		case zoneSiteSentry:
			out[id] = zoneSiteSentrySight
		case zoneSiteWarden:
			out[id] = 6
		}
	}

	return out, nil
}

// zoneSiteSheets answers thirty feet, halved while the member sneaks (the
// session's rule, R8), and a passive Perception of 10 for every knight.
type zoneSiteSheets struct{ enc **encounter.Encounter }

func (s zoneSiteSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		speed := 30
		if *s.enc != nil && (*s.enc).Sneaking(id) {
			speed /= 2
		}
		passive := 10
		if id == zoneSiteRogue {
			passive = 12
		}
		out[id] = encounter.SheetFacts{
			SpeedFeet:         speed,
			Actions:           []encounter.ActionView{{Ref: kingSpear, RangeFeet: 5}},
			PassivePerception: passive,
			WatchResetCells:   sneakSiteReset,
		}
	}

	return out, nil
}

// openTheZoneSite seats the rogue at the party start and the knights where
// the file posts them. No Stealth is scripted: the scene never enters a zone,
// so any roll fails it.
func openTheZoneSite(t *testing.T) (*encounter.Encounter, *sneakSiteStealth) {
	t.Helper()
	compiled := loadContent(t, zoneSiteFixture)
	members := []encounter.MemberInput{{ID: zoneSiteRogue, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Facing: m.FacingDegrees,
		})
	}
	stealth := &sneakSiteStealth{}
	var enc *encounter.Encounter
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     compiled.Field,
		Members:   members,
		Endings:   append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         zoneSiteSight{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zoneSiteSheets{enc: &enc},
			Participation: &castleStanding{down: map[encounter.MemberID]bool{}},
			Initiative:    orderAsGiven{},
			Driver:        encounter.TableDriver{Roller: rollsOne{}},
			Roller:        rollsOne{},
			CheckResolver: stealth,
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   cellsStriker{calls: &[]cellsStrike{}},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return enc, stealth
}

// zonesOf is a member's red floor.
func zonesOf(t *testing.T, enc *encounter.Encounter, who encounter.MemberID) []encounter.WatchZone {
	t.Helper()
	out, err := enc.WatchZonesFor(&encounter.WatchZonesInput{Member: who})
	require.NoError(t, err)
	require.NotNil(t, out.Zones, "the zones are never nil")

	return out.Zones
}

// cellSet is a list of cells as a set.
func cellSet(cells []spatial.Position) map[spatial.Position]bool {
	out := make(map[spatial.Position]bool, len(cells))
	for _, c := range cells {
		out[c] = true
	}

	return out
}

// requireTheSentrysZone pins what a zone sent to the rogue is: the sentry's,
// holding the sentry's own cell, every cell floor the rogue knows and a cell
// the sentry's sight reaches now — the one sight test the zone is, read
// through the sentry's own seen-now set. It returns the zone's cells, and
// whether the sentry sees floor the rogue does not know — floor the zone was
// cut to leave out.
func requireTheSentrysZone(t *testing.T, enc *encounter.Encounter) (map[spatial.Position]bool, bool) {
	t.Helper()
	zones := zonesOf(t, enc, zoneSiteRogue)
	require.Len(t, zones, 1, "one zone: the sentry's, and not the warden's behind the wall")
	require.Equal(t, zoneSiteSentry, zones[0].Watcher)

	atlas, err := enc.AtlasFor(zoneSiteRogue)
	require.NoError(t, err)
	known := cellSet(atlas.Cells)
	seen, err := enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: zoneSiteSentry})
	require.NoError(t, err)
	reached := cellSet(seen.Cells)
	sentry := positionOf(t, enc, zoneSiteSentry)
	for _, cell := range zones[0].Cells {
		require.True(t, known[cell], "zone cell %v is floor the rogue knows", cell)
		require.True(t, reached[cell], "zone cell %v is one the sentry sees now", cell)
		require.LessOrEqual(t, enc.Distance(sentry, cell), float64(zoneSiteSentrySight),
			"zone cell %v is within the sentry's sight", cell)
	}
	zone := cellSet(zones[0].Cells)
	require.True(t, zone[sentry], "the sentry's own cell is in its zone")
	cut := false
	for _, cell := range seen.Cells {
		cut = cut || !known[cell]
	}

	return zone, cut
}

// TestZoneSite is the red floor's scenario, on a site authored in the studio:
// a rogue who is not sneaking is sent no zones; sneaking, it is sent the
// zone of the one knight it sees and none for the knight behind the wall,
// every cell floor it knows; and as the sentry walks its loop the zone walks
// with it.
func TestZoneSite(t *testing.T) {
	enc, stealth := openTheZoneSite(t)

	_, err := enc.WatchZonesFor(nil)
	require.ErrorIs(t, err, encounter.ErrNilInput)
	_, err = enc.WatchZonesFor(&encounter.WatchZonesInput{Member: "stranger"})
	require.ErrorIs(t, err, encounter.ErrNotMember)

	require.Empty(t, zonesOf(t, enc, zoneSiteRogue), "not sneaking: no red floor (R15)")

	_, err = enc.SetSneaking(&encounter.SetSneakingInput{Member: zoneSiteRogue, Sneaking: true})
	require.NoError(t, err)
	sentrySeen := (&sneakSite{t: t, enc: enc}).holding(zoneSiteRogue, zoneSiteSentry)
	require.NotNil(t, sentrySeen)
	require.NotEmpty(t, sentrySeen.CurrentVia, "the rogue sees the sentry")
	require.Nil(t, (&sneakSite{t: t, enc: enc}).holding(zoneSiteRogue, zoneSiteWarden),
		"the warden stands behind the wall, unseen")
	first, cut := requireTheSentrysZone(t, enc)
	require.Empty(t, zonesOf(t, enc, zoneSiteSentry), "a monster never sneaks, so it is sent no zones")

	// THE SENTRY WALKS. The rogue paces its corner, out of the sentry's three
	// cells, while the world clock moves the sentry along its loop. Every
	// refresh's zone is the sentry's, cut to the rogue's floor; the walk goes
	// on until the zone has moved off a cell and onto another, and the sentry
	// has stood where it sees through the doorway onto floor the rogue never
	// saw — floor its zone leaves out.
	start := positionOf(t, enc, zoneSiteRogue)
	east := spatial.Position{X: start.X + 1, Y: start.Y}
	left, joined := false, false
	for i := 0; !left || !joined || !cut; i++ {
		require.Less(t, i, 36, "the sentry walks its loop within a few rounds")
		to := east
		if i%2 == 1 {
			to = start
		}
		_, err := enc.Step(&encounter.StepInput{Member: zoneSiteRogue, To: to})
		require.NoError(t, err, "the rogue steps to %v", to)
		require.Equal(t, encounter.ClockWorld, clockKind(t, enc, zoneSiteRogue), "no fight formed")

		zone, cutHere := requireTheSentrysZone(t, enc)
		cut = cut || cutHere
		for cell := range first {
			left = left || !zone[cell]
		}
		for cell := range zone {
			joined = joined || !first[cell]
		}
	}
	require.Empty(t, stealth.asked, "the rogue never entered a zone, so nothing rolled")

	_, err = enc.SetSneaking(&encounter.SetSneakingInput{Member: zoneSiteRogue, Sneaking: false})
	require.NoError(t, err)
	require.Empty(t, zonesOf(t, enc, zoneSiteRogue), "sneaking off: the red floor is gone")
}
