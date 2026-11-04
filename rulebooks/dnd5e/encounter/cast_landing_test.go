// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cast_landing_test.go is rpg-project#831: a cast's shoves are part of the
// cast landing. On the castle in free roam a hostile Thunderwave turns the
// garrison and forms the fight inside the deed (#785 R3), so the shove has to
// land inside the deed too, after the cast's own beats and before the deed
// sets anything off.

var clThunderwave = encounter.SpellIdentity{Ref: "dnd5e:spells:thunderwave", Name: "Thunderwave"}

// clBard is the caster, a player standing beside the captain.
const clBard = encounter.MemberID("bard")

// bardLast rolls the garrison's initiative over the bard's, as the walk on
// rpg-project#825 did: the fight's first turns are played inside the cast.
type bardLast struct{}

func (bardLast) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	out := slices.DeleteFunc(slices.Clone(members), func(id encounter.MemberID) bool { return id == clBard })
	if len(out) < len(members) {
		out = append(out, clBard)
	}
	return out, nil
}

// walkingSheets gives every member thirty feet and a sword in reach, so a
// guard the shove threw out of reach walks back on its first turn.
type walkingSheets struct{}

func (walkingSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	sword := encounter.ActionView{Ref: core.Ref{Module: "dnd5e", Type: "weapons", ID: "longsword"}, Name: "Longsword", RangeFeet: 5}
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{SpeedFeet: 30, Actions: []encounter.ActionView{sword}}
	}
	return out, nil
}

// swingsAtNothing lets a guard's swing go by without a story of its own: the
// order under test is the walk, not the blow.
type swingsAtNothing struct{}

func (swingsAtNothing) Strike(context.Context, *encounter.Encounter, encounter.MemberID, encounter.MemberID, core.Ref) error {
	return nil
}

// openTheGatehouseForTheBard opens the castle with the bard at the party
// start and every monster at its post, then walks the bard up to the captain
// on the world clock, as the session's Move would: step by step, stopping at
// the first cell the board refuses.
func openTheGatehouseForTheBard(t *testing.T) *encounter.Encounter {
	t.Helper()
	compiled := compiledSite(t, 0)
	start := compiled.PartyStart[0].At

	members := []encounter.MemberInput{{ID: clBard, Kind: encounter.KindPlayer, Position: start}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Options: m.Options,
			Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees, Machine: m.Machine, BlockingStates: m.BlockingStates, Posts: m.Posts,
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		Endings: append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: encounter.Capabilities{
			Sight:         &sightList{fallback: 24},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        walkingSheets{},
			Participation: everyoneStanding{},
			Initiative:    bardLast{},
			Driver:        tableDriver(),
			Roller:        rollsLowest{},
			CheckResolver: findsNothing{},
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   swingsAtNothing{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	route, err := enc.Route(encounter.RouteInput{
		Mover: clBard, Policy: encounter.MoveToward, Anchor: whereIs(t, enc, "captain"), Budget: 200,
	})
	require.NoError(t, err)
	require.NotEmpty(t, route.Path, "the bard can walk up to the captain")
	for i, cell := range route.Path {
		if _, err := enc.Step(&encounter.StepInput{Member: clBard, To: cell, EndWalk: i == len(route.Path)-1}); err != nil {
			require.Positive(t, i, "the bard takes at least one step: %v", err)
			break
		}
	}
	return enc
}

// thunderwaveOn is the failed save and the shove for one guard, routed two
// cells away from the bard the way the session routes it.
func thunderwaveOn(t *testing.T, enc *encounter.Encounter, guard encounter.MemberID) (encounter.CastTargetResult, encounter.CastPush) {
	t.Helper()
	route, err := enc.Route(encounter.RouteInput{
		Mover: guard, Policy: encounter.MoveAway, Anchor: whereIs(t, enc, clBard), Budget: 2,
	})
	require.NoError(t, err)
	require.NotEmpty(t, route.Path, "%s has somewhere to be thrown", guard)
	return encounter.CastTargetResult{
		Target: guard,
		Save: &encounter.CastSave{
			Saver: guard, Ability: "constitution", Roll: 1, Total: 3, DC: 13,
			Calculation: saveCalculation(clThunderwave, "constitution", 1, 3),
		},
		Results: []encounter.ActivationResult{{
			Kind: encounter.ResultMoved, Target: guard, Ref: clThunderwave.Ref, Name: clThunderwave.Name,
			Moved: len(route.Path), StoppedBy: route.StoppedBy,
		}},
	}, encounter.CastPush{
		Mover: guard, Route: route.Path,
	}
}

// TestACastsShovesLandInsideItsLandingOnTheWorldClock: the cast beat, then the
// shoves, then the stance, the fight and its first turns. The captain's first
// own step leaves from the cell the blast threw it to.
func TestACastsShovesLandInsideItsLandingOnTheWorldClock(t *testing.T) {
	enc := openTheGatehouseForTheBard(t)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: clBard})
	require.NoError(t, err)
	require.Equal(t, encounter.ClockWorld, clock.Kind, "precondition: the bard casts from free roam")
	before := len(storyBeats(t, enc, clBard))

	captain, captainPush := thunderwaveOn(t, enc, "captain")
	_, err = enc.RecordCast(&encounter.RecordCastInput{
		Actor: clBard, Spell: clThunderwave,
		Targets: []encounter.CastTargetResult{captain},
		Pushes:  []encounter.CastPush{captainPush},
	})
	require.NoError(t, err)

	thrownTo := captainPush.Route[len(captainPush.Route)-1]
	castAt, formedAt, lastPushAt, firstTurnEnded := -1, -1, -1, -1
	var ownStep *spatial.Position
	for i, beat := range storyBeats(t, enc, clBard)[before:] {
		switch beat["beat"] {
		case encounter.BeatCast:
			castAt = i
		case encounter.BeatFightStarted:
			if formedAt < 0 {
				formedAt = i
			}
		case encounter.BeatTurnEnded:
			if firstTurnEnded < 0 {
				firstTurnEnded = i
			}
		case encounter.BeatMoved:
			if beat["member"] != "captain" {
				continue
			}
			if beat["cause"] == clThunderwave.Ref {
				lastPushAt = i
				continue
			}
			if formedAt >= 0 && ownStep == nil {
				cell := beat["position"].(map[string]any)
				ownStep = &spatial.Position{X: cell["x"].(float64), Y: cell["y"].(float64)}
			}
		}
	}

	require.GreaterOrEqual(t, castAt, 0, "the cast is told")
	require.GreaterOrEqual(t, lastPushAt, 0, "the shove is walked")
	require.GreaterOrEqual(t, formedAt, 0, "the hostile cast forms the fight")
	require.GreaterOrEqual(t, firstTurnEnded, 0, "the garrison takes its first turns inside the cast")
	assert.Less(t, castAt, lastPushAt, "the thunder, then the slide")
	assert.Less(t, lastPushAt, formedAt, "the shove lands before the fight forms (#785 R3)")
	assert.Less(t, lastPushAt, firstTurnEnded, "the shove lands before any turn of the fight passes")

	require.NotNil(t, ownStep, "the captain walks back on its first turn")
	assert.LessOrEqual(t, math.Max(math.Abs(ownStep.X-thrownTo.X), math.Abs(ownStep.Y-thrownTo.Y)), 1.0,
		"the captain's first step %v leaves from the cell it was thrown to %v", *ownStep, thrownTo)
}

// TestAPushOnSomebodyTheCastDidNotReachIsRefusedBeforeAnyBeat: a push names a
// creature the cast caught. One that names anybody else is refused before the
// first append, so a refusal leaves no half-told cast in the story.
func TestAPushOnSomebodyTheCastDidNotReachIsRefusedBeforeAnyBeat(t *testing.T) {
	enc := openTheGatehouseForTheBard(t)
	before := len(storyBeats(t, enc, clBard))

	captain, _ := thunderwaveOn(t, enc, "captain")
	_, err := enc.RecordCast(&encounter.RecordCastInput{
		Actor: clBard, Spell: clThunderwave,
		Targets: []encounter.CastTargetResult{captain},
		Pushes:  []encounter.CastPush{{Mover: "gate-1", Route: []spatial.Position{whereIs(t, enc, "gate-1")}}},
	})
	require.ErrorIs(t, err, encounter.ErrInvalidData)
	assert.Len(t, storyBeats(t, enc, clBard), before, "nothing of the cast was told")
}
