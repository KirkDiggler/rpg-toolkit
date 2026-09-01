// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// seen_internal_test.go pins ADR-0041's projection: Seen is present exactly
// when the sighting was produced by sight, and a held memory keeps its last
// Seen (rpg-toolkit#1157).

func sightPayloadBytes(t *testing.T, x, y float64) []byte {
	t.Helper()
	b, err := json.Marshal(encounter.SightPayload{X: x, Y: y})
	require.NoError(t, err)
	return b
}

// TestProjectSightingsSightChannelGetsASeen is the ordinary case: a
// sight-channel holding decodes into Seen.Position.
func TestProjectSightingsSightChannelGetsASeen(t *testing.T) {
	holdings := []intel.Holding{{
		Subject:    intel.Subject("skeleton-1"),
		Payload:    sightPayloadBytes(t, 10, 3),
		Channel:    intel.Sight,
		At:         5,
		CurrentVia: []intel.Channel{intel.Sight},
		Status:     intel.Current,
	}}

	out := projectSightings(holdings, nil, nil, map[string]bool{"skeleton-1": true})
	require.Len(t, out, 1)
	require.NotNil(t, out[0].Seen, "a sight-channel holding must carry Seen")
	require.Equal(t, spatial.Position{X: 10, Y: 3}, out[0].Seen.Position)
	require.Equal(t, LocationKnown, out[0].LocationState)
	require.Equal(t, StandingDowned, out[0].Seen.Standing,
		"the batched down-set this subject appears in projects onto Seen.Standing (rpg-toolkit#1137)")
}

func TestMonsterViewAdaptersCarryRememberedPathsByValue(t *testing.T) {
	path := []spatial.Position{{X: 1, Y: 2}, {X: 2, Y: 3}}
	in := encounter.MonsterView{
		Self: "goblin",
		Remembered: []encounter.RememberedMember{{
			ID: "billy", Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 2, Y: 3}, DistanceCells: 2,
			Path: path,
		}},
	}

	projected := projectMonsterView(in)
	require.Equal(t, []RememberedMember{{
		ID: "billy", Kind: KindPlayer,
		Position: spatial.Position{X: 2, Y: 3}, DistanceCells: 2,
		Path: path,
	}}, projected.Remembered)
	projected.Remembered[0].Path[0].X = 99
	require.Equal(t, float64(1), path[0].X, "projected path must not alias encounter view")

	roundTrip, err := unprojectMonsterView(projected)
	require.NoError(t, err)
	require.Equal(t, float64(99), roundTrip.Remembered[0].Path[0].X)
	roundTrip.Remembered[0].Path[1].Y = 88
	require.Equal(t, float64(3), projected.Remembered[0].Path[1].Y,
		"unprojected path must not alias session view")
}

// TestProjectSightingsNonSightChannelGetsNoSeen pins the other half: a
// holding whose provenance channel is not sight gets no Seen, even though the
// payload happens to be a valid SightPayload — Channel is what decides,
// consistent with the ADR's "present exactly when the sighting was produced
// by sight".
func TestProjectSightingsNonSightChannelGetsNoSeen(t *testing.T) {
	holdings := []intel.Holding{{
		Subject:    intel.Subject("goblin-1"),
		Payload:    sightPayloadBytes(t, 4, 4),
		Channel:    intel.Channel("hearing"),
		At:         5,
		CurrentVia: []intel.Channel{intel.Channel("hearing")},
		Status:     intel.Current,
	}}

	out := projectSightings(holdings, nil, nil, nil)
	require.Len(t, out, 1)
	require.Nil(t, out[0].Seen, "a non-sight channel must not carry Seen, however the payload happens to decode")
	require.Empty(t, out[0].LocationState, "a non-sight channel carries no location state either")
}

// TestProjectSightingsHeldMemoryKeepsItsLastSeen is the ADR's own case: a
// subject whose CurrentVia has gone empty (Status == Held, a ghost) still
// carries the Channel and Payload of the last accepted testimony, so it still
// gets a Seen — the last-known cell a client draws a faded marker on.
func TestProjectSightingsHeldMemoryKeepsItsLastSeen(t *testing.T) {
	holdings := []intel.Holding{{
		Subject:    intel.Subject("goblin-1"),
		Payload:    sightPayloadBytes(t, 6, 10),
		Channel:    intel.Sight,
		At:         3,
		CurrentVia: nil, // faded: no channel currently sustains it
		Status:     intel.Held,
	}}

	out := projectSightings(holdings, nil, nil, nil)
	require.Len(t, out, 1)
	require.NotNil(t, out[0].Seen, "a held memory must keep its last Seen")
	require.Equal(t, spatial.Position{X: 6, Y: 10}, out[0].Seen.Position)
	require.Equal(t, StandingUp, out[0].Seen.Standing, "not in the down set: reports up")
}

func TestHeldUnknownSightProjectsExplicitUnknownLocation(t *testing.T) {
	payload, err := encounter.EncodeLocationPayload(encounter.LocationKnowledge{State: encounter.LocationUnknown})
	require.NoError(t, err)
	out := projectSightings([]intel.Holding{{
		Subject: "billy", Payload: payload, Channel: intel.Sight,
		Status: intel.Held,
	}}, nil, nil, nil)
	require.Len(t, out, 1)
	require.Equal(t, LocationUnknown, out[0].LocationState)
	require.Nil(t, out[0].Seen)
}

func TestProjectIntelCorrectionsSortsObserverThenSubject(t *testing.T) {
	got := projectIntelCorrections(map[encounter.MemberID]*encounter.IntelDelta{
		"zog": {Corrected: []intel.Subject{"billy", "alice"}},
		"abe": {Corrected: []intel.Subject{"david", "carol"}},
	})
	require.Equal(t, []IntelCorrection{
		{Observer: "abe", Subject: "carol"},
		{Observer: "abe", Subject: "david"},
		{Observer: "zog", Subject: "alice"},
		{Observer: "zog", Subject: "billy"},
	}, got)
}

// TestProjectSightingsCarriesKindFromTheRoster pins rpg-toolkit#1230: kind is
// a roster fact looked up the same way Name is, not a second perception
// question. Two holdings, two kinds, so a projection that swapped the map
// lookup for a constant would still fail.
func TestProjectSightingsCarriesKindFromTheRoster(t *testing.T) {
	holdings := []intel.Holding{
		{
			Subject:    intel.Subject("fighter"),
			Payload:    sightPayloadBytes(t, 1, 1),
			Channel:    intel.Sight,
			At:         1,
			CurrentVia: []intel.Channel{intel.Sight},
			Status:     intel.Current,
		},
		{
			Subject:    intel.Subject("skeleton-1"),
			Payload:    sightPayloadBytes(t, 2, 2),
			Channel:    intel.Sight,
			At:         1,
			CurrentVia: []intel.Channel{intel.Sight},
			Status:     intel.Current,
		},
	}
	kinds := map[string]MemberKind{"fighter": KindPlayer, "skeleton-1": KindMonster}

	out := projectSightings(holdings, nil, kinds, nil)
	require.Len(t, out, 2)
	for _, s := range out {
		switch s.Subject {
		case "fighter":
			require.Equal(t, KindPlayer, s.Kind)
		case "skeleton-1":
			require.Equal(t, KindMonster, s.Kind)
		default:
			t.Fatalf("unexpected subject %q", s.Subject)
		}
	}
}

// TestProjectSightingsHeldMemoryKeepsItsKind is TestProjectSightingsHeldMemoryKeepsItsLastSeen's
// twin for Kind: a subject whose CurrentVia has gone empty (Status == Held, a
// ghost) still carries the kind the roster reports for it, same as it keeps
// its name — a memory does not forget what it once classified at a glance.
func TestProjectSightingsHeldMemoryKeepsItsKind(t *testing.T) {
	holdings := []intel.Holding{{
		Subject:    intel.Subject("goblin-1"),
		Payload:    sightPayloadBytes(t, 6, 10),
		Channel:    intel.Sight,
		At:         3,
		CurrentVia: nil, // faded: no channel currently sustains it
		Status:     intel.Held,
	}}
	kinds := map[string]MemberKind{"goblin-1": KindMonster}

	out := projectSightings(holdings, nil, kinds, nil)
	require.Len(t, out, 1)
	require.Equal(t, KindMonster, out[0].Kind, "a held memory must keep its kind")
}

// TestProjectSeenIsNilWhenASightPayloadFailsToDecode is the defensive arm: an
// impossible state today (the composition is the only writer of sight
// payloads), pinned so a future regression that corrupts a sight payload
// fails loudly as a missing Seen rather than a wrong Position.
func TestProjectSeenIsNilWhenASightPayloadFailsToDecode(t *testing.T) {
	got := projectSeen(intel.Sight, []byte("not json"), false)
	require.Nil(t, got)
}

// TestProjectReportSeenCannotDistinguishSightFromALookalikePayload documents
// the one soft spot in this PR, named in projectReportSeen's own comment:
// intel.Report carries no Channel of its own, so projectReportSeen decodes
// and checks rather than gating on Channel the way projectSeen does. That is
// equivalent to a real channel check ONLY because sight is the only channel
// any composition in this codebase surveils with today (rebuildPercepts is
// the sole Surveil call site, always intel.Sight).
//
// This test proves the gap rather than hiding it: a payload that merely
// LOOKS like a SightPayload — decodable as {x,y} — gets a Seen from
// projectReportSeen with no way to ask "but was this really sight?", because
// nothing here has a channel to ask about. The day a second channel starts
// calling Surveil, this stops being a hypothetical and starts being a wrong
// answer; closing it needs SurveilOutput (or the percept it is built from) to
// carry its own channel, which is a play/intel change outside this PR.
func TestProjectReportSeenCannotDistinguishSightFromALookalikePayload(t *testing.T) {
	lookalike := sightPayloadBytes(t, 1, 2) // could be any future channel's bytes that
	// happen to parse as {x,y}; today it can only actually be sight.

	got := projectReportSeen(lookalike, true)
	require.NotNil(t, got, "documents the gap: any {x,y}-shaped payload decodes, whatever channel actually produced it")
	require.Equal(t, spatial.Position{X: 1, Y: 2}, got.Position)
	require.Equal(t, StandingDowned, got.Standing, "the down flag still projects even through the gap")
}
