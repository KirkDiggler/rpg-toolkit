// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// facing_site_test.go plays the studio-authored facing site (rpg-project#582,
// sneaking design R4 and R13): facing is member state the engine sets — seeded
// from the authored word, turned by every step, turned toward a target at the
// end of a walk that names one — and it is testimony, carried on what each
// observer saw and kept across a save.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const facingSiteFixture = "testdata/facing-site.yaml"

const (
	facingHero   = encounter.MemberID("hero")
	facingKnight = encounter.MemberID("knight")
	facingGuard  = encounter.MemberID("guard")
)

// facingSight is how far each member sees on the facing site: the guard two
// cells, so the hero can pace out of its sight; everybody else the whole room.
type facingSight struct{}

func (facingSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 20
		if id == facingGuard {
			out[id] = 2
		}
	}

	return out, nil
}

// facingCapabilities is the castle's host, with the facing site's sight.
func facingCapabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         facingSight{},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        castleSheets{},
		Participation: &castleStanding{down: map[encounter.MemberID]bool{}},
		Initiative:    orderAsGiven{},
		Driver:        encounter.TableDriver{Roller: rollsOne{}},
		Roller:        rollsOne{},
		CheckResolver: nothingIsEverFound{},
		Witness:       nobodyPerceivesAnything{},
		Actors: encounter.Actors{
			Striker:   cellsStriker{calls: &[]cellsStrike{}},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

// openTheFacingSite seats the hero at the party start, facing nowhere stated,
// and every authored creature at its placement with the facing the compile
// lowered from the author's word.
func openTheFacingSite(t *testing.T) *encounter.Encounter {
	t.Helper()
	compiled := loadContent(t, facingSiteFixture)
	start := compiled.PartyStart[0].At
	members := []encounter.MemberInput{{ID: facingHero, Kind: encounter.KindPlayer, Position: start}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID:       encounter.MemberID(m.MemberID),
			Kind:     encounter.KindMonster,
			Position: m.At,
			Faction:  encounter.FactionID(m.Faction),
			Table:    m.Table,
			Facing:   m.FacingDegrees,
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		// The site authors no ending; a run needs one, and the host's own
		// withdrawal is the one every host declares.
		Endings:      append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention:    encounter.RetentionUnbounded,
		Capabilities: facingCapabilities(),
	})
	require.NoError(t, err)

	return enc
}

// facingOf is a member's facing on the roster.
func facingOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) float64 {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == id {
			return m.Facing
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return 0
}

// positionOf is where a member stands now.
func positionOf(t *testing.T, enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	require.FailNowf(t, "not in the run", "%s", id)

	return spatial.Position{}
}

// sightingOf is the facing one member's own sighting of another carries.
func sightingOf(t *testing.T, enc *encounter.Encounter, observer, subject encounter.MemberID) encounter.SightTestimony {
	t.Helper()
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	require.NoError(t, err)
	for _, h := range holdings {
		if h.Channel == perception.Sight && encounter.MemberID(h.Subject) == subject {
			testimony, ok := encounter.DecodeSightTestimony(h.Payload)
			require.True(t, ok, "%s's sighting of %s decodes", observer, subject)

			return testimony
		}
	}
	require.FailNowf(t, "no sighting", "%s holds no sighting of %s", observer, subject)

	return encounter.SightTestimony{}
}

// facingStep is one cell the hero steps to.
type facingStep spatial.Position

// take steps the hero there.
func (step facingStep) take(t *testing.T, enc *encounter.Encounter) {
	t.Helper()
	_, err := enc.Step(&encounter.StepInput{Member: facingHero, To: spatial.Position(step)})
	require.NoError(t, err, "the hero steps to %v", step)
}

// TestFacingSite is the facing slice's scenario, on a site authored in the
// studio: the knight's authored word seeds its facing, its patrol turns it,
// a guard that closes on the hero ends facing the hero, the hero's own
// sighting of the knight carries the knight's facing, and a save keeps all
// three.
func TestFacingSite(t *testing.T) {
	enc := openTheFacingSite(t)
	start := positionOf(t, enc, facingHero)
	require.Equal(t, spatial.Position{X: -4, Y: -1}, start, "the hero comes in at the authored party start")

	// THE SEED. `facing: e` is 90; the hero stated none, and an unstated
	// facing is a rules bearing of 180.
	require.Equal(t, 90.0, facingOf(t, enc, facingKnight), "the knight starts facing the author's e")
	require.Equal(t, 180.0, facingOf(t, enc, facingHero), "an unstated facing is south")

	// ONE WORLD ROUND. The hero paces six cells out of the guard's two-cell
	// sight, which is one round at thirty feet; the knight walks its first
	// leg, six cells down column q = 0 (axial +r), which on pointy-top hexes
	// is a bearing of 150.
	east := spatial.Position{X: start.X + 1, Y: start.Y}
	for i := range 6 {
		to := east
		if i%2 == 1 {
			to = start
		}
		facingStep(to).take(t, enc)
	}
	require.Equal(t, encounter.ClockWorld, clockKind(t, enc, facingHero), "the guard has not seen the hero yet")
	require.Equal(t, spatial.Position{X: 0, Y: 3}, positionOf(t, enc, facingKnight), "one round walked the first leg")
	require.InDelta(t, 150.0, facingOf(t, enc, facingKnight), 1e-9,
		"the knight faces along the leg it walked, not the way it was authored")
	require.InDelta(t, 270.0, facingOf(t, enc, facingHero), 1e-9, "the hero's own last step, back west, turned it too")

	// INTO THE GUARD'S SIGHT. A step east and one along +r bring the hero to
	// the guard's cell plus (1, -2): two cells off, and not along any of the
	// six step directions, so the guard's last step cannot point at the hero
	// by itself.
	facingStep(east).take(t, enc)
	facingStep{X: east.X, Y: east.Y + 1}.take(t, enc)
	hero := positionOf(t, enc, facingHero)
	require.Equal(t, spatial.Position{X: -3, Y: 0}, hero)
	if clockKind(t, enc, facingHero) == encounter.ClockTurn {
		if _, err := enc.EndTurn(&encounter.EndTurnInput{Member: facingHero}); err != nil {
			require.ErrorIs(t, err, encounter.ErrNotActive, "the hero's turn ends, or it is not the hero's turn")
		}
	}

	// The guard walked one step to stand beside the hero and turned to face
	// it: from (-4, 1) the hero is at (+1, -1), bearing 30, after a step of
	// 330; from (-3, 1) the hero is at (0, -1), bearing 330, after a step of
	// 30. Either way the end of the walk is what faced it.
	guard := positionOf(t, enc, facingGuard)
	toHero := map[spatial.Position]float64{{X: -4, Y: 1}: 30, {X: -3, Y: 1}: 330}
	want, adjacent := toHero[guard]
	require.True(t, adjacent, "the guard stopped beside the hero, at %v", guard)
	require.InDelta(t, want, facingOf(t, enc, facingGuard), 1e-9, "the guard ends its walk facing the hero")

	// TESTIMONY. The hero's own sighting of the knight carries the knight's
	// facing as the hero last saw it, beside where it saw it.
	knight := facingOf(t, enc, facingKnight)
	seen := sightingOf(t, enc, facingHero, facingKnight)
	require.NotNil(t, seen.Facing, "a sighting carries the facing it saw")
	require.InDelta(t, knight, *seen.Facing, 1e-9)

	// THE SAVE KEEPS ALL THREE.
	heroFacing, guardFacing := facingOf(t, enc, facingHero), facingOf(t, enc, facingGuard)
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Capabilities: facingCapabilities(),
	})
	require.NoError(t, err)
	require.InDelta(t, knight, facingOf(t, loaded, facingKnight), 1e-9)
	require.InDelta(t, guardFacing, facingOf(t, loaded, facingGuard), 1e-9)
	require.InDelta(t, heroFacing, facingOf(t, loaded, facingHero), 1e-9)
	reloaded := sightingOf(t, loaded, facingHero, facingKnight)
	require.NotNil(t, reloaded.Facing)
	require.InDelta(t, *seen.Facing, *reloaded.Facing, 1e-9)
}
