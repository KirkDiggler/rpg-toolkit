// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// sneakseen_internal_test.go is the refusal's second half (sneaking R10 as
// amended by the-world-runs R11, rpg-project#768): a declaration is refused
// when a watcher's ZONE holds the mover's cell, live holding or not. The two
// part only while a watcher's holdings lag its sight, so the test makes
// them lag: it empties the run's sight holdings, leaving the zone the only
// thing that sees the rogue.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// zoneOnlyYard is a yard with the rogue and one watcher who sees it, and
// the run's holdings emptied, so nobody holds anybody live.
func zoneOnlyYard(t *testing.T, passive int) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas:  CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions: []RegionInput{rectRegion("yard", 0, 0, 12, 4)},
		},
		Members: []MemberInput{
			{ID: "rogue", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: "sentry", Kind: KindWorld, Position: spatial.Position{X: 6, Y: 1}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  UnobservedEquipment{},
			Conditions: UnobservedEquipment{},
			Sheets: sheetFacts{
				"rogue":  {SpeedFeet: 30, PassivePerception: 12},
				"sentry": {SpeedFeet: 30, PassivePerception: passive},
			},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: RefusingCheckResolver{},
			Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)
	live, err := enc.holdsLive("sentry", "rogue")
	require.NoError(t, err)
	require.True(t, live, "precondition: the sentry sees the rogue")

	enc.intelLog, err = perception.New()
	require.NoError(t, err)
	live, err = enc.holdsLive("sentry", "rogue")
	require.NoError(t, err)
	require.False(t, live, "precondition: the sentry's holdings lag")

	return enc
}

// TestADeclarationInAZoneIsRefused: the sentry holds nothing, but its zone
// holds the rogue's cell, and that alone refuses the declaration.
func TestADeclarationInAZoneIsRefused(t *testing.T) {
	enc := zoneOnlyYard(t, 10)
	next := enc.ToData().Log.NextSeq

	_, err := enc.SetSneaking(&SetSneakingInput{Member: "rogue", Sneaking: true})
	require.ErrorIs(t, err, ErrSeen)
	require.ErrorContains(t, err, "sentry")
	require.False(t, enc.Sneaking("rogue"))
	require.Equal(t, next, enc.ToData().Log.NextSeq, "nothing written")
}

// TestAWatcherWithNoPassiveDoesNotRefuse: the same zone, held by a watcher
// that supplies no passive Perception and so judges no sneak, refuses
// nothing — the set refused is the set a sneak is judged against.
func TestAWatcherWithNoPassiveDoesNotRefuse(t *testing.T) {
	enc := zoneOnlyYard(t, 0)

	out, err := enc.SetSneaking(&SetSneakingInput{Member: "rogue", Sneaking: true})
	require.NoError(t, err)
	require.True(t, out.Sneaking)
}
