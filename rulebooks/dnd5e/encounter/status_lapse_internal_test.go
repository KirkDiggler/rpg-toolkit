// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// status_lapse_internal_test.go is A STATUS GIVEN FOR A NUMBER OF ROUNDS
// (rpg-project#1080 S2): the encounter keeps one timer per application,
// saved with the run, and announces one StatusLapsed boundary to that one
// holder when the world clock reaches the round it is due.
//
// EVERY TEST THAT COUNTS ROUNDS: rounds pass only when someone acts or waits
// (Wait is rpg-project#1079). A raise below is one world action of a member.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// lapseAnnouncer records every boundary announced, in order.
type lapseAnnouncer struct{ seen []Boundary }

func (a *lapseAnnouncer) Announce(_ context.Context, _ *Encounter, crossed []Boundary) error {
	a.seen = append(a.seen, crossed...)
	return nil
}

// lapsesSeen is the StatusLapsed boundaries the announcer saw so far.
func (a *lapseAnnouncer) lapsesSeen() []Boundary {
	var out []Boundary
	for _, b := range a.seen {
		if b.Kind == StatusLapsed {
			out = append(out, b)
		}
	}
	return out
}

// lapseCapabilities is the initiative scene's capabilities with the
// recording announcer.
func lapseCapabilities(a Announcer) Capabilities {
	return Capabilities{
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     UnobservedEquipment{},
		Conditions:    UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: a},
	}
}

// lapseScene is the three-member scene (alice, bob, a goblin) with a
// recording announcer.
func lapseScene(t *testing.T) (*Encounter, *lapseAnnouncer) {
	t.Helper()
	enc := newInitiativeEnc(t)
	rec := &lapseAnnouncer{}
	enc.announcer = rec
	return enc, rec
}

// raise spends one world action of alice: one raise of the world clock.
func raise(t *testing.T, enc *Encounter, times int) {
	t.Helper()
	for i := 0; i < times; i++ {
		require.NoError(t, enc.spendWorldAction("alice"))
	}
}

func arm(t *testing.T, enc *Encounter, member MemberID, application string, rounds int) {
	t.Helper()
	require.NoError(t, enc.ArmStatusLapse(&ArmStatusLapseInput{
		Member: member, Status: "held", Application: application, Rounds: rounds,
	}))
}

func TestALapseFiresWhenTheWorldReachesItsRound(t *testing.T) {
	enc, rec := lapseScene(t)
	// Rounds pass only when someone acts or waits (Wait is rpg-project#1079).
	raise(t, enc, 4) // the world is at round 4 when it is given
	arm(t, enc, "bob", "status:s:held:bob", 3)

	raise(t, enc, 2) // round 6
	require.Empty(t, rec.lapsesSeen(), "two of three rounds: nothing yet")

	raise(t, enc, 1) // round 7
	got := rec.lapsesSeen()
	require.Len(t, got, 1, "exactly one message, to the one holder")
	require.Equal(t, Boundary{Kind: StatusLapsed, Subject: "bob", Application: "status:s:held:bob"}, got[0])
	require.Zero(t, got[0].Round, "a world round is not a fight round")

	raise(t, enc, 3)
	require.Len(t, rec.lapsesSeen(), 1, "fired once, never again")
}

func TestRearmingReplacesTheRound(t *testing.T) {
	enc, rec := lapseScene(t)
	// Rounds pass only when someone acts or waits (Wait is rpg-project#1079).
	arm(t, enc, "bob", "app", 2)
	raise(t, enc, 1)
	arm(t, enc, "bob", "app", 3) // replaces: due at round 4, not 2
	raise(t, enc, 2)             // round 3
	require.Empty(t, rec.lapsesSeen())
	raise(t, enc, 1) // round 4
	require.Len(t, rec.lapsesSeen(), 1)
}

func TestClearingALapseMeansItNeverFires(t *testing.T) {
	enc, rec := lapseScene(t)
	// Rounds pass only when someone acts or waits (Wait is rpg-project#1079).
	arm(t, enc, "bob", "app", 2)
	require.NoError(t, enc.ClearStatusLapse(&ClearStatusLapseInput{Application: "app"}))
	require.NoError(t, enc.ClearStatusLapse(&ClearStatusLapseInput{Application: "never-armed"}), "unknown is no error")
	raise(t, enc, 5)
	require.Empty(t, rec.lapsesSeen())
}

func TestTwoLapsesOnOneHolderFireSeparately(t *testing.T) {
	enc, rec := lapseScene(t)
	// Rounds pass only when someone acts or waits (Wait is rpg-project#1079).
	arm(t, enc, "bob", "app-b", 2)
	arm(t, enc, "bob", "app-a", 2)
	arm(t, enc, "bob", "app-c", 3)
	raise(t, enc, 2)
	got := rec.lapsesSeen()
	require.Len(t, got, 2)
	require.Equal(t, "app-a", got[0].Application, "in application order")
	require.Equal(t, "app-b", got[1].Application)
	raise(t, enc, 1)
	require.Len(t, rec.lapsesSeen(), 3)
	require.Equal(t, "app-c", rec.lapsesSeen()[2].Application)
}

func TestAFightRoundCountsOnce(t *testing.T) {
	enc, rec := lapseScene(t)
	_, err := enc.form(&FormInput{Order: []MemberID{"alice", "bob", "goblin"}})
	require.NoError(t, err)
	arm(t, enc, "bob", "app", 2)

	// A full round of three is alice, bob and the driven goblin: two
	// EndTurn calls. Each wrap raises the world clock once, not once per
	// fighter. Rounds pass only when someone acts or waits (Wait is
	// rpg-project#1079); in a fight every round is acted.
	endRound := func() {
		for _, who := range []MemberID{"alice", "bob"} {
			_, eerr := enc.EndTurn(&EndTurnInput{Member: who})
			require.NoError(t, eerr)
		}
	}
	endRound() // round 2 starts: one raise
	require.Empty(t, rec.lapsesSeen(), "one fight round is one world round")
	endRound() // round 3 starts: the second raise
	require.Len(t, rec.lapsesSeen(), 1)
}

func TestATimeJumpElapsesALapse(t *testing.T) {
	enc, rec := lapseScene(t)
	arm(t, enc, "bob", "app", 3)
	require.NoError(t, enc.elapseWorld(600))
	require.Len(t, rec.lapsesSeen(), 1, "three rounds of a long rest are passed")
}

func TestALapseSurvivesSaveAndLoad(t *testing.T) {
	enc, _ := lapseScene(t)
	// Rounds pass only when someone acts or waits (Wait is rpg-project#1079).
	arm(t, enc, "bob", "app-b", 3)
	arm(t, enc, "bob", "app-a", 3)

	data := enc.ToData()
	require.Len(t, data.Lapses, 2)
	require.Equal(t, "app-a", data.Lapses[0].Application, "saved sorted by application")
	require.Equal(t, StatusLapseData{Member: "bob", Status: "held", Application: "app-a", At: 3}, data.Lapses[0])
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	var decoded EncounterData
	require.NoError(t, json.Unmarshal(raw, &decoded))

	rec := &lapseAnnouncer{}
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: decoded, Capabilities: lapseCapabilities(rec)})
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		require.NoError(t, loaded.spendWorldAction("alice"))
	}
	require.Empty(t, rec.lapsesSeen())
	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Len(t, rec.lapsesSeen(), 2, "each fires once")
}

func TestLoadRefusesABadLapse(t *testing.T) {
	enc, _ := lapseScene(t)
	good := StatusLapseData{Member: "bob", Status: "held", Application: "app", At: 3}
	cases := map[string][]StatusLapseData{
		"unknown member":         {{Member: "ghost", Status: "held", Application: "app", At: 3}},
		"blank application":      {{Member: "bob", Status: "held", Application: "", At: 3}},
		"application twice":      {good, good},
		"due at round zero":      {{Member: "bob", Status: "held", Application: "app", At: 0}},
		"a good one still loads": nil,
	}
	for name, lapses := range cases {
		t.Run(name, func(t *testing.T) {
			data := enc.ToData()
			data.Lapses = lapses
			_, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: lapseCapabilities(&lapseAnnouncer{})})
			if lapses == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrInvalidData)
		})
	}
}

func TestAnOlderSaveWithNoLapsesLoads(t *testing.T) {
	enc, _ := lapseScene(t)
	data := enc.ToData()
	require.Empty(t, data.Lapses)
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"lapses"`, "omitted when nothing is armed")
	var decoded EncounterData
	require.NoError(t, json.Unmarshal(raw, &decoded))
	_, err = LoadEncounter(&LoadEncounterInput{Data: decoded, Capabilities: lapseCapabilities(&lapseAnnouncer{})})
	require.NoError(t, err)
}

func TestLeavingTheRunDropsTheHoldersLapses(t *testing.T) {
	enc, rec := lapseScene(t)
	// Rounds pass only when someone acts or waits (Wait is rpg-project#1079).
	arm(t, enc, "bob", "bobs", 2)
	arm(t, enc, "alice", "alices", 2)
	_, err := enc.Exit(&ExitInput{Member: "bob"})
	require.NoError(t, err)
	raise(t, enc, 2)
	got := rec.lapsesSeen()
	require.Len(t, got, 1, "nothing fires about a member who is gone")
	require.Equal(t, MemberID("alice"), got[0].Subject)
}

func TestArmRefusesBadInput(t *testing.T) {
	enc, _ := lapseScene(t)
	good := ArmStatusLapseInput{Member: "bob", Status: "held", Application: "app", Rounds: 3}
	require.ErrorIs(t, enc.ArmStatusLapse(nil), ErrNilInput)
	with := func(f func(*ArmStatusLapseInput)) *ArmStatusLapseInput {
		in := good
		f(&in)
		return &in
	}
	require.ErrorIs(t, enc.ArmStatusLapse(with(func(i *ArmStatusLapseInput) { i.Member = "stranger" })), ErrNotMember)
	require.ErrorIs(t, enc.ArmStatusLapse(with(func(i *ArmStatusLapseInput) { i.Status = "" })), ErrInvalidData)
	require.ErrorIs(t, enc.ArmStatusLapse(with(func(i *ArmStatusLapseInput) { i.Application = "" })), ErrInvalidData)
	require.ErrorIs(t, enc.ArmStatusLapse(with(func(i *ArmStatusLapseInput) { i.Rounds = 0 })), ErrInvalidData)
	require.ErrorIs(t, enc.ArmStatusLapse(with(func(i *ArmStatusLapseInput) { i.Rounds = 1001 })), ErrInvalidData)
	require.NoError(t, enc.ArmStatusLapse(&good))
}

func TestAGrantCarriesItsUntilToTheLandedRow(t *testing.T) {
	timed := statusGrantTo(string(RecognitionSpeaker))
	timed.ApplyStatus.Until = &StatusUntil{Rounds: 3}
	line := holdLine("hold-timed")
	line.Grants = []RowGrant{{Row: 0, Consequence: timed}}
	plain := holdLine("hold-plain", string(RecognitionSpeaker))
	enc := receiverHall(t, 2, []Option{line, plain})

	out := sayTo(t, enc, "hold-timed")
	require.Len(t, out.Statuses, 1)
	require.NotNil(t, out.Statuses[0].Grant.Until)
	require.Equal(t, 3, out.Statuses[0].Grant.Until.Rounds)

	out = sayTo(t, enc, "hold-plain")
	require.Len(t, out.Statuses, 1)
	require.Nil(t, out.Statuses[0].Grant.Until)
}

func TestAGrantWithBadRoundsIsRefusedWhenBuilt(t *testing.T) {
	for _, rounds := range []int{0, -2, 1001} {
		g := StatusGrant{ID: "held", Status: json.RawMessage(`{"name":"Held"}`), To: string(RecognitionSpeaker), Until: &StatusUntil{Rounds: rounds}}
		err := validateStatusGrant("a row", g)
		require.ErrorIs(t, err, ErrInvalidData, "rounds %d", rounds)
		require.Contains(t, err.Error(), StatusUntilRefusal(rounds))
	}
	g := StatusGrant{ID: "held", Status: json.RawMessage(`{"name":"Held"}`), To: string(RecognitionSpeaker), Until: &StatusUntil{Rounds: 1000}}
	require.NoError(t, validateStatusGrant("a row", g))
	require.Empty(t, StatusUntilRefusal(3))
	require.Equal(t, "rounds must be a whole number of one or more, at most 1000, such as 3", StatusUntilRefusal(0))
}
