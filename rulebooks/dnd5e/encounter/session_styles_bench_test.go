package encounter_test

import (
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// A small simulation for rpg-project#879 R3: N games served by one process.
// "rebuild" is today's server: every verb unmarshals the record, rebuilds the
// encounter, steps, and marshals the whole record back. "live" keeps N built
// encounters resident and each verb steps and marshals the checkpoint.
// "live-nosave" shows the checkpoint's share. One verb = one Step; a game's
// turn is the garrison bench's four-cell pace; games are served round-robin.

func simCaps() encounter.Capabilities {
	return encounter.Capabilities{
		Sight: &sightList{fallback: 24}, Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{},
		Sheets: walksThirty{}, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: encounter.TableDriver{Roller: rollsLowest{}},
		Roller: rollsLowest{}, CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Actors: encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

func simKeep(b *testing.B) (*encounter.Encounter, []spatial.Position) {
	compiled := compiledOnce(b, "sim-garrison-10000", func() ([]byte, error) {
		opts := keepgen.DefaultOptions()
		opts.Hexes, opts.Seed, opts.Key = 10000, 1, "bench-keep-garrison"
		opts.Garrison, opts.Patrols, opts.PostsListen = 99, 20, false
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
		Field: compiled.Field, Members: members,
		Endings:      append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: simCaps(),
	})
	if err != nil {
		b.Fatal(err)
	}
	player := keepPlayers[0]
	start := whereIs(b, enc, player)
	beside := besideStart(b, enc, player, start)
	return enc, []spatial.Position{beside, start, beside, start}
}

func heapMB() float64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.HeapInuse) / 1e6
}

func BenchmarkSessionStyles(b *testing.B) {
	for _, games := range []int{1, 8, 32} {
		for _, style := range []string{"rebuild", "live", "live-nosave"} {
			b.Run(fmt.Sprintf("%s/games=%d", style, games), func(b *testing.B) { benchSessionStyle(b, style, games) })
		}
	}
}

func benchSessionStyle(b *testing.B, style string, games int) {
	b.StopTimer()
	base := heapMB()
	player := keepPlayers[0]
	var walk []spatial.Position
	var blobs [][]byte
	var live []*encounter.Encounter
	switch style {
	case "rebuild":
		enc, w := simKeep(b)
		walk = w
		raw, err := json.Marshal(enc.ToData())
		if err != nil {
			b.Fatal(err)
		}
		for i := 0; i < games; i++ {
			blobs = append(blobs, append([]byte(nil), raw...))
		}
	default:
		for i := 0; i < games; i++ {
			enc, w := simKeep(b)
			walk = w
			live = append(live, enc)
		}
	}
	resident := heapMB() - base
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var saved int
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		g := i % games
		for j, cell := range walk {
			in := &encounter.StepInput{Member: player, To: cell, EndWalk: j == len(walk)-1}
			switch style {
			case "rebuild":
				var data encounter.EncounterData
				if err := json.Unmarshal(blobs[g], &data); err != nil {
					b.Fatal(err)
				}
				enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: simCaps()})
				if err != nil {
					b.Fatal(err)
				}
				if _, err := enc.Step(in); err != nil {
					b.Fatalf("game %d step %d: %v", g, j, err)
				}
				raw, err := json.Marshal(enc.ToData())
				if err != nil {
					b.Fatal(err)
				}
				blobs[g] = raw
				saved += len(raw)
			case "live":
				if _, err := live[g].Step(in); err != nil {
					b.Fatalf("game %d step %d: %v", g, j, err)
				}
				raw, err := json.Marshal(live[g].ToData())
				if err != nil {
					b.Fatal(err)
				}
				saved += len(raw)
			case "live-nosave":
				if _, err := live[g].Step(in); err != nil {
					b.Fatalf("game %d step %d: %v", g, j, err)
				}
			}
		}
	}
	b.StopTimer()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	b.ReportMetric(resident, "resident-MB")
	b.ReportMetric(float64(after.NumGC-before.NumGC)/float64(b.N), "gc-cycles/op")
	b.ReportMetric(float64(after.PauseTotalNs-before.PauseTotalNs)/1e6/float64(b.N), "gc-pause-ms/op")
	_ = saved
}
