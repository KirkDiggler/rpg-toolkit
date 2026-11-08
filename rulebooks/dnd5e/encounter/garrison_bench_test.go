// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BenchmarkGarrisonMove is a four-cell Move on the garrison keep
// (rpg-project#766 S1, S3): the 10k keep with 99 creatures given orders and
// 20 of them on patrol routes, in two keeps (below), the documents
//
//	go run ./cmd/keepgen -hexes 10000 -seed 1 -garrison 99 -patrols 20 -key bench-keep-garrison -posts-listen=false   # default
//	go run ./cmd/keepgen -hexes 10000 -seed 1 -garrison 99 -patrols 20 -key bench-keep-garrison                       # posts-asleep
//
// write. One player paces four cells, back and forth beside the party start,
// one walk per op; at thirty feet a round is six cells, so two ops in three
// raise the world clock and the garrison is asked what it does with time.
// Opening the keep is outside the timer, and the keep stays open across ops.
//
//	go test -run XXX -bench BenchmarkGarrisonMove -benchtime 20x -count 3 .
//
// Numbers on rpg-project#773 and #766: before is the census (every
// world-clock member walked on every raise); after is the due heap. S3's
// ~193 ms/op was taken before the garrison turned neutral to the party
// (97098c41), when every Move ran inside a fight and nobody was asked the
// time; ~4.2 s/op on `default` is the first number in which it is.
//
// TWO KEEPS (rpg-project#766 S4): `default` is the garrison as S1 wrote it,
// every creature on the default recipe and due every round; `posts-asleep`
// is the same keep with keepgen's `-posts-listen`, the 79 posts posted and
// asking the world clock nothing until a sound lands near them, so a raise
// costs the 20 patrols.
func BenchmarkGarrisonMove(b *testing.B) {
	for _, tc := range []struct {
		name   string
		listen bool
	}{{"default", false}, {"posts-asleep", true}} {
		b.Run(tc.name, func(b *testing.B) { benchGarrisonMove(b, tc.listen) })
	}
}

func benchGarrisonMove(b *testing.B, listen bool) {
	b.StopTimer()
	key := "keep-garrison-10000"
	if listen {
		key += "-posts-listen"
	}
	compiled := compiledOnce(b, key, func() ([]byte, error) {
		opts := keepgen.DefaultOptions()
		opts.Hexes, opts.Seed, opts.Key = 10000, 1, "bench-keep-garrison"
		opts.Garrison, opts.Patrols, opts.PostsListen = 99, 20, listen
		raw, _, err := keepgen.Generate(keepgen.DefaultPlan(), opts)
		return raw, err
	})
	members := []encounter.MemberInput{{ID: keepPlayers[0], Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
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

	// WHAT THE VERB SPENDS, APART (rpg-project#879 S0): a step that raises the
	// world clock pays for the garrison's turn as well as its own; one that
	// does not pays only the player's walk and the player's own sight. The
	// two are told apart by the clock's high-water mark, read with the timer
	// stopped, and reported beside ns/op so a change to either shows on its
	// own line. blob-bytes is the whole record as the keep opens; the growth
	// is the net change one op makes to it after retention trims the log, so
	// it can read negative on a quiet keep.
	opening := blobBytes(b, enc)
	var walkNs, raiseNs time.Duration
	var walkSteps, raiseSteps int
	before := worldRound(enc)
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for j, cell := range walk {
			started := time.Now()
			if _, err := enc.Step(&encounter.StepInput{Member: player, To: cell, EndWalk: j == len(walk)-1}); err != nil {
				b.Fatalf("op %d step %d to %v: %v", i, j, cell, err)
			}
			took := time.Since(started)
			b.StopTimer()
			after := worldRound(enc)
			if after != before {
				raiseNs += took
				raiseSteps++
			} else {
				walkNs += took
				walkSteps++
			}
			before = after
			b.StartTimer()
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(opening), "blob-bytes")
	b.ReportMetric(float64(blobBytes(b, enc)-opening)/float64(b.N), "blob-growth-bytes/op")
	b.ReportMetric(float64(raiseSteps)/float64(b.N), "raises/op")
	if walkSteps > 0 {
		b.ReportMetric(float64(walkNs)/float64(walkSteps)/float64(time.Millisecond), "walk-step-ms")
	}
	if raiseSteps > 0 {
		b.ReportMetric(float64(raiseNs)/float64(raiseSteps)/float64(time.Millisecond), "raise-step-ms")
	}
}

// worldRound is the world clock's high-water mark: the round the keep has
// reached, read without the record so the harness allocates nothing it is
// not measuring.
func worldRound(enc *encounter.Encounter) int { return enc.WorldRound() }

// blobBytes is the size of the whole saved record, the shape the host
// writes on every verb today (rpg-project#770).
func blobBytes(b *testing.B, enc *encounter.Encounter) int {
	b.Helper()
	raw, err := json.Marshal(enc.ToData())
	if err != nil {
		b.Fatalf("marshal the record: %v", err)
	}
	return len(raw)
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
