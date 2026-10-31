// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// launch_scale_test.go is WHAT A LAUNCH AND A STEP COST ON THE KEEP
// (rpg-project#753, S3): the session's launch is an empty encounter and one
// [encounter.Encounter.Board] of the party and every monster, whose one look
// sweeps every observer cold; a step's look is the warm sweep. Re-run rather
// than quoted:
//
//	go test . -run '^$' -bench 'Launch_Keep|RefreshSightWarm|LoadEncounter_Keep10k' -benchtime 3x

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// launchBoard is the session's launch board: the party in seat order, then
// every monster in authored order. The compile's cells are authored offset
// pairs and a join's cell is axial, so each is converted as the session's
// launch converts it.
func launchBoard(compiled dungeonspec.Compiled) []encounter.JoinInput {
	o := compiled.Field.Canvas.Orientation
	cell := func(at spatial.Position) spatial.Position { return encounter.HexCellAt(o, int(at.X), int(at.Y)) }
	board := make([]encounter.JoinInput, 0, len(keepPlayers)+len(compiled.Monsters))
	for _, p := range seatedParty(compiled) {
		board = append(board, encounter.JoinInput{Member: p.ID, Kind: p.Kind, Cell: cell(p.Position)})
	}
	for _, m := range compiled.Monsters {
		board = append(board, encounter.JoinInput{
			Member: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Cell: cell(m.At),
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Options: m.Options,
			Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees,
		})
	}
	return board
}

// launchSite opens the site empty and boards the launch, as the session's
// launch does.
func launchSite(tb testing.TB, compiled dungeonspec.Compiled) *encounter.Encounter {
	tb.Helper()
	enc, err := launchSiteErr(compiled)
	require.NoError(tb, err)
	return enc
}

// launchSiteErr is [launchSite] for a caller with no test to fail.
func launchSiteErr(compiled dungeonspec.Compiled) (*encounter.Encounter, error) {
	enc, err := openCompiledErr(compiled, nil, false, findsNothing{})
	if err != nil {
		return nil, err
	}
	if _, err := enc.Board(&encounter.BoardInput{Members: launchBoard(compiled)}); err != nil {
		return nil, err
	}
	return enc, nil
}

func benchmarkLaunch(b *testing.B, hexes int) {
	compiled := compiledSite(b, hexes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		launchSite(b, compiled)
	}
}

// BenchmarkLaunch_Keep10k is the 10k keep's launch: compile excluded.
func BenchmarkLaunch_Keep10k(b *testing.B) { benchmarkLaunch(b, 10000) }

// BenchmarkLaunch_Keep30k is the 30k keep's launch: compile excluded.
func BenchmarkLaunch_Keep30k(b *testing.B) { benchmarkLaunch(b, 30000) }

// BenchmarkRefreshSightWarm_Keep10k is the whole roster's sight pass after the
// launch, with every observer's sweep key kept: what each step's look pays
// for the observers that did not move. Recheck runs the pass the verbs run.
func BenchmarkRefreshSightWarm_Keep10k(b *testing.B) {
	enc := launchSite(b, compiledSite(b, 10000))
	roster := enc.ExportedRosterIDs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.Recheck(&encounter.RecheckInput{Members: roster}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadEncounter_Keep10k is what every session verb pays to read the
// launched 10k keep back (rpg-project#753, S4): unmarshal the saved run and
// load it. The saved bytes are reported as a metric beside the time.
func BenchmarkLoadEncounter_Keep10k(b *testing.B) {
	raw, err := json.Marshal(launchedKeep10k(b).ToData())
	require.NoError(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var data encounter.EncounterData
		if err := json.Unmarshal(raw, &data); err != nil {
			b.Fatal(err)
		}
		if _, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: data, Capabilities: knowledgeCapabilities(&sightList{fallback: 24}),
		}); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(raw)), "blob-bytes")
}
