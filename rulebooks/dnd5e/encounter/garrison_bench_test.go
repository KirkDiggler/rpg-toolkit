// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BenchmarkGarrisonMove is a four-cell Move on the garrison keep
// (rpg-project#766 S1, S3): the 10k keep with 99 creatures given orders and
// 20 of them on patrol routes, the same document
//
//	go run ./cmd/keepgen -hexes 10000 -seed 1 -garrison 99 -patrols 20 -key bench-keep-garrison
//
// writes. One player paces four cells, back and forth beside the party start,
// one walk per op; at thirty feet a round is six cells, so two ops in three
// raise the world clock and the garrison is asked what it does with time.
// Opening the keep is outside the timer, and the keep stays open across ops.
//
//	go test -run XXX -bench BenchmarkGarrisonMove -benchtime 20x -count 3 .
//
// Numbers on rpg-project#773 and #766: before is the census (every
// world-clock member walked on every raise); after is the due heap.
func BenchmarkGarrisonMove(b *testing.B) {
	b.StopTimer()
	compiled := compiledOnce(b, "keep-garrison-10000", func() ([]byte, error) {
		opts := keepgen.DefaultOptions()
		opts.Hexes, opts.Seed, opts.Key = 10000, 1, "bench-keep-garrison"
		opts.Garrison, opts.Patrols = 99, 20
		raw, _, err := keepgen.Generate(keepgen.DefaultPlan(), opts)
		return raw, err
	})
	members := []encounter.MemberInput{{ID: keepPlayers[0], Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Options: m.Options,
			Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees,
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
			Sheets:        walksThirty{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        encounter.TableDriver{Roller: rollsLowest{}},
			Roller:        rollsLowest{},
			CheckResolver: findsNothing{},
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	if err != nil {
		b.Fatalf("open the garrison keep: %v", err)
	}

	player := keepPlayers[0]
	start := whereIs(b, enc, player)
	beside := besideStart(b, enc, player, start)
	walk := []spatial.Position{beside, start, beside, start}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for j, cell := range walk {
			if _, err := enc.Step(&encounter.StepInput{Member: player, To: cell, EndWalk: j == len(walk)-1}); err != nil {
				b.Fatalf("op %d step %d to %v: %v", i, j, cell, err)
			}
		}
	}
}

// besideStart is the first hex neighbour of start the player can step onto
// and back from; the probe steps are taken outside the timer.
func besideStart(b *testing.B, enc *encounter.Encounter, player encounter.MemberID, start spatial.Position) spatial.Position {
	b.Helper()
	for _, d := range []spatial.Position{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}} {
		cell := spatial.Position{X: start.X + d.X, Y: start.Y + d.Y}
		if _, err := enc.Step(&encounter.StepInput{Member: player, To: cell}); err != nil {
			continue
		}
		if _, err := enc.Step(&encounter.StepInput{Member: player, To: start, EndWalk: true}); err != nil {
			b.Fatalf("step back to %v: %v", start, err)
		}
		return cell
	}
	b.Fatalf("no cell beside %v to pace", start)

	return spatial.Position{}
}
