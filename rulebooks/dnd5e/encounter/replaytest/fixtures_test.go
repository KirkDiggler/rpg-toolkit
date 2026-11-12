// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

// fixtures_test.go is the two documents the replay plays: the mad king's
// castle as its QA script opens it (castletest's castle_king_script_test.go)
// and the garrison keep (keepgen's `-garrison`, rpg-project#766 S1).

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	kingFixture = "../dungeonspec/testdata/world-builder-v4-castle.yaml"

	// hero is the castle's one player.
	hero = encounter.MemberID("hero")
)

// The castle's cast and its gate, by the ids the file gives them.
const (
	kingCaptain  = encounter.MemberID("captain")
	kingGate1    = encounter.MemberID("gate-1")
	kingGate2    = encounter.MemberID("gate-2")
	kingPedlar   = encounter.MemberID("pedlar")
	kingGateDoor = encounter.DoorID("the-mad-king/gate-wall-door")
)

// castleCell is the dungeon-absolute cell of an offset [col, row] on the
// castle's floor.
func castleCell(col, row int) spatial.Position {
	return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
}

// The script's cells (castletest's castle_king_script_test.go).
var (
	// gatehouseSeat is the gatehouse floor in front of the gate guards, as
	// an offset cell: where a step that acts in front of the gate opens.
	gatehouseSeat = spatial.Position{X: 0, Y: 11}

	// beneathTheGate is the cell under the gate door, where a party stands
	// to open it.
	beneathTheGate = castleCell(0, 7)

	// besideTheGate is beneathTheGate's west neighbour, for pacing.
	besideTheGate = castleCell(-1, 7)
)

// castleOpener is the castle's QA-script host with the hero at a cell:
// every creature watching a 120-degree arc, the gate's watchers at passive
// 11, 12 and 13, and nothing ever found.
func castleOpener(at spatial.Position) opener {
	return castleOpenerWith(at)
}

// castleOpenerWith is [castleOpener] with more of the party placed beside the
// hero, each where it is given.
func castleOpenerWith(at spatial.Position, party ...encounter.MemberInput) opener {
	return func(t testing.TB, h host, down *standing) *encounter.SetupInput {
		t.Helper()
		compiled := loadFile(t, kingFixture)
		arcs := map[encounter.MemberID]int{}
		members := append([]encounter.MemberInput{{ID: hero, Kind: encounter.KindPlayer, Position: at}}, party...)
		for _, m := range compiled.Monsters {
			id := encounter.MemberID(m.MemberID)
			arcs[id] = 120
			members = append(members, monsterInput(m))
		}

		return setupOn(compiled, members, h, down, plainSheets{
			passive: map[encounter.MemberID]int{kingGate1: 11, kingGate2: 12, kingCaptain: 13},
			arcs:    arcs,
		})
	}
}

// The garrison keep: rpg-project#766 S1's fixture, keepgen's 10k keep with
// 99 creatures given orders and 20 of them on patrol routes (keepgen's floor
// is 10k hexes, so there is no smaller keep to replay).
const (
	garrisonHexes   = 10000
	garrisonSize    = 99
	garrisonPatrols = 20
)

// garrisonWalker is the garrison keep's one player.
const garrisonWalker = encounter.MemberID("walker")

// garrisonOpener is the garrison keep with one walker at the party start.
func garrisonOpener(listen bool) opener {
	return func(t testing.TB, h host, down *standing) *encounter.SetupInput {
		t.Helper()
		key := "garrison"
		if listen {
			key += "-posts-listen"
		}
		compiled := compiledOnce(t, key, func() ([]byte, error) {
			opts := keepgen.DefaultOptions()
			opts.Hexes, opts.Seed, opts.Key = garrisonHexes, 1, "replay-keep-garrison"
			opts.Garrison, opts.Patrols, opts.PostsListen = garrisonSize, garrisonPatrols, listen
			raw, _, err := keepgen.Generate(keepgen.DefaultPlan(), opts)
			return raw, err
		})
		members := []encounter.MemberInput{{ID: garrisonWalker, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
		for _, m := range compiled.Monsters {
			members = append(members, monsterInput(m))
		}

		return setupOn(compiled, members, h, down, plainSheets{})
	}
}

// monsterInput is a compiled monster as a host seats it.
func monsterInput(m dungeonspec.MonsterPlacement) encounter.MemberInput {
	var holds []encounter.IntelID
	for _, held := range m.Holds {
		holds = append(holds, encounter.IntelID(held))
	}

	return encounter.MemberInput{
		ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
		Faction: encounter.FactionID(m.Faction), Table: m.Table, Options: m.Options, Arrives: m.Arrives,
		Behaviors: m.Behaviors, Machine: m.Machine, BlockingStates: m.BlockingStates, Posts: m.Posts, Temper: soldierProfiles(m.Temper), Holds: holds, Facing: m.FacingDegrees,
	}
}

// setupOn is a SetupInput for compiled content and seated members, with the
// host's collaborators that roll given: the world's die (the table
// driver's and the composition's), the driven swing, initiative and
// Stealth.
func setupOn(
	compiled dungeonspec.Compiled, members []encounter.MemberInput, h host, down *standing, sheets plainSheets,
) *encounter.SetupInput {
	return &encounter.SetupInput{
		Field:     compiled.Field,
		Members:   members,
		Endings:   append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         plainSight{feet: 60},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sheets,
			Participation: down,
			Initiative:    h.initiative,
			Driver:        encounter.BehaviorDriver{},
			Roller:        h.die,
			CheckResolver: h.check,
			Witness:       perceivesNothingAsked{},
			Actors: encounter.Actors{
				Striker:   h.striker,
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	}
}
