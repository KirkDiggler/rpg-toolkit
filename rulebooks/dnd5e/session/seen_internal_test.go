// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// seen_internal_test.go pins ADR-0041's projection: Seen is present exactly
// when the sighting was produced by sight, and a held memory keeps its last
// Seen (rpg-toolkit#1157). rpg-toolkit#1702 moved Standing onto the same
// footing as Position: a fact decoded out of the stored testimony, never a
// live consult — see standingTestimonyBytes and the cases built on it below.

// sightPayloadBytes encodes the legacy untagged {x,y} form: a known position
// and nothing else. It predates Down and Equipment entirely, which is exactly
// what makes it useful for the tests that are not about either.
func sightPayloadBytes(t *testing.T, x, y float64) []byte {
	t.Helper()
	b, err := json.Marshal(encounter.SightPayload{X: x, Y: y})
	require.NoError(t, err)
	return b
}

// standingTestimonyBytes encodes canonical tagged sight testimony carrying a
// known position and the given Down claim — nil for "not observed", a
// pointer for "observed to be exactly this".
func standingTestimonyBytes(t *testing.T, x, y float64, down *bool) []byte {
	t.Helper()
	b, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationKnown, Position: spatial.Position{X: x, Y: y}, Down: down,
	})
	require.NoError(t, err)
	return b
}

func boolPtr(b bool) *bool { return &b }

// TestProjectSightingsSightChannelGetsASeen is the ordinary case: a
// sight-channel holding decodes into Seen.Position.
func TestProjectSightingsSightChannelGetsASeen(t *testing.T) {
	holdings := []perception.Holding{{
		Subject:    "skeleton-1",
		Payload:    standingTestimonyBytes(t, 10, 3, boolPtr(false)),
		Channel:    perception.Sight,
		Confirmed:  5,
		CurrentVia: []perception.Channel{perception.Sight},
	}}

	out := projectSightings(holdings, nil, nil)
	require.Len(t, out, 1)
	require.NotNil(t, out[0].Seen, "a sight-channel holding must carry Seen")
	require.Equal(t, spatial.Position{X: 10, Y: 3}, out[0].Seen.Position)
	require.Equal(t, LocationKnown, out[0].LocationState)
	require.NotNil(t, out[0].Seen.Standing, "the testimony observed standing")
	require.Equal(t, StandingUp, *out[0].Seen.Standing)
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
	holdings := []perception.Holding{{
		Subject:    "goblin-1",
		Payload:    sightPayloadBytes(t, 4, 4),
		Channel:    perception.Channel("hearing"),
		Confirmed:  5,
		CurrentVia: []perception.Channel{perception.Channel("hearing")},
	}}

	out := projectSightings(holdings, nil, nil)
	require.Len(t, out, 1)
	require.Nil(t, out[0].Seen, "a non-sight channel must not carry Seen, however the payload happens to decode")
	require.Empty(t, out[0].LocationState, "a non-sight channel carries no location state either")
}

// TestProjectSightingsHeldMemoryKeepsItsLastSeen is the ADR's own case: a
// subject no longer currently sustained (Current false, a ghost) still
// carries the Channel and Payload of the last accepted testimony, so it still
// gets a Seen — the last-known cell a client draws a faded marker on.
func TestProjectSightingsHeldMemoryKeepsItsLastSeen(t *testing.T) {
	holdings := []perception.Holding{{
		Subject:   "goblin-1",
		Payload:   sightPayloadBytes(t, 6, 10),
		Channel:   perception.Sight,
		Confirmed: 3,
	}}

	out := projectSightings(holdings, nil, nil)
	require.Len(t, out, 1)
	require.NotNil(t, out[0].Seen, "a held memory must keep its last Seen")
	require.Equal(t, spatial.Position{X: 6, Y: 10}, out[0].Seen.Position)
	require.Nil(t, out[0].Seen.Standing,
		"the legacy untagged payload never observed standing at all — nil, not StandingUp")
}

func TestHeldUnknownSightProjectsExplicitUnknownLocation(t *testing.T) {
	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationUnknown})
	require.NoError(t, err)
	out := projectSightings([]perception.Holding{{
		Subject: "billy", Payload: payload, Channel: perception.Sight,
	}}, nil, nil)
	require.Len(t, out, 1)
	require.Equal(t, LocationUnknown, out[0].LocationState)
	require.Nil(t, out[0].Seen)
}

// TestProjectSightingsCarriesKindFromTheRoster pins rpg-toolkit#1230: kind is
// a roster fact looked up the same way Name is, not a second perception
// question. Two holdings, two kinds, so a projection that swapped the map
// lookup for a constant would still fail.
func TestProjectSightingsCarriesKindFromTheRoster(t *testing.T) {
	holdings := []perception.Holding{
		{
			Subject:    "fighter",
			Payload:    sightPayloadBytes(t, 1, 1),
			Channel:    perception.Sight,
			Confirmed:  1,
			CurrentVia: []perception.Channel{perception.Sight},
		},
		{
			Subject:    "skeleton-1",
			Payload:    sightPayloadBytes(t, 2, 2),
			Channel:    perception.Sight,
			Confirmed:  1,
			CurrentVia: []perception.Channel{perception.Sight},
		},
	}
	kinds := map[string]MemberKind{"fighter": KindPlayer, "skeleton-1": KindMonster}

	out := projectSightings(holdings, nil, kinds)
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
// twin for Kind: a subject no longer currently sustained (Current false, a
// ghost) still carries the kind the roster reports for it, same as it keeps
// its name — a memory does not forget what it once classified at a glance.
func TestProjectSightingsHeldMemoryKeepsItsKind(t *testing.T) {
	holdings := []perception.Holding{{
		Subject:   "goblin-1",
		Payload:   sightPayloadBytes(t, 6, 10),
		Channel:   perception.Sight,
		Confirmed: 3,
	}}
	kinds := map[string]MemberKind{"goblin-1": KindMonster}

	out := projectSightings(holdings, nil, kinds)
	require.Len(t, out, 1)
	require.Equal(t, KindMonster, out[0].Kind, "a held memory must keep its kind")
}

// TestProjectSeenIsNilWhenASightPayloadFailsToDecode is the defensive arm: an
// impossible state today (the composition is the only writer of sight
// payloads), pinned so a future regression that corrupts a sight payload
// fails loudly as a missing Seen rather than a wrong Position.
func TestProjectSeenIsNilWhenASightPayloadFailsToDecode(t *testing.T) {
	got := projectSeen(perception.Sight, []byte("not json"))
	require.Nil(t, got)
}

// TestProjectSeenReportsWhatTheTestimonyClaimsAboutStanding is test case 2
// and 3 of rpg-toolkit#1702's own table: an observer CURRENTLY seeing a
// subject reports exactly the Down claim the testimony carries, standing or
// downed, never a live consult of anyone's sheet — projectSeen has no roster
// to ask and no down parameter to receive one through any more.
func TestProjectSeenReportsWhatTheTestimonyClaimsAboutStanding(t *testing.T) {
	t.Run("observed standing", func(t *testing.T) {
		got := projectSeen(perception.Sight, standingTestimonyBytes(t, 1, 1, boolPtr(false)))
		require.NotNil(t, got)
		require.NotNil(t, got.Standing)
		require.Equal(t, StandingUp, *got.Standing)
	})

	t.Run("observed downed", func(t *testing.T) {
		got := projectSeen(perception.Sight, standingTestimonyBytes(t, 1, 1, boolPtr(true)))
		require.NotNil(t, got)
		require.NotNil(t, got.Standing)
		require.Equal(t, StandingDowned, *got.Standing)
	})
}

// TestProjectSeenStandingIsNilWhenTestimonyNeverObservedIt is test case 4: a
// testimony whose Down is nil — written before rpg-toolkit#1697, or by a
// channel that does not report it — must project a nil Standing. Collapsing
// nil to StandingUp would assert something nobody observed, the exact class
// of defect rpg-toolkit#1702 removes; the failure message says so rather
// than just naming the mismatch.
func TestProjectSeenStandingIsNilWhenTestimonyNeverObservedIt(t *testing.T) {
	got := projectSeen(perception.Sight, standingTestimonyBytes(t, 1, 1, nil))
	require.NotNil(t, got)
	require.Nil(t, got.Standing,
		"Down was never observed in this testimony; reporting StandingUp would assert a fact nobody witnessed")
}

// TestProjectReportSeenCannotDistinguishSightFromALookalikePayload documents
// the one soft spot in this PR, named in projectReportSeen's own comment:
// a first-contact presence carries no Channel of its own, so projectReportSeen
// decodes and checks rather than gating on Channel the way projectSeen does.
// That is equivalent to a real channel check ONLY because sight is the only
// channel any composition in this codebase surveils with today
// (rebuildPercepts is the sole Observe call site, always perception.Sight).
//
// This test proves the gap rather than hiding it: a payload that merely
// LOOKS like sight testimony — decodable with a known position — gets a Seen
// from projectReportSeen with no way to ask "but was this really sight?",
// because nothing here has a channel to ask about. The day a second channel
// starts calling Observe, this stops being a hypothetical and starts being a
// wrong answer; closing it needs mind/perception's Delta (or the percept it
// is built from) to carry its own channel, which is outside this PR.
func TestProjectReportSeenCannotDistinguishSightFromALookalikePayload(t *testing.T) {
	lookalike := standingTestimonyBytes(t, 1, 2, boolPtr(true)) // could be any future channel's
	// bytes that happen to parse as sight testimony; today it can only actually be sight.

	got := projectReportSeen(lookalike)
	require.NotNil(t, got, "documents the gap: any sight-shaped payload decodes, whatever channel actually produced it")
	require.Equal(t, spatial.Position{X: 1, Y: 2}, got.Position)
	require.NotNil(t, got.Standing)
	require.Equal(t, StandingDowned, *got.Standing, "the testimony's own Down claim still projects even through the gap")
}

// TestSightingStatusReportsTheSustainingChannelNotTheProvenance is the defect
// mind/perception v0.2.0 closed, pinned here because this seam is where it
// would have reached a host.
//
// The shape is exactly what intel.Report produces: a deed lands about a
// subject the observer currently SEES, so Channel (provenance of the latest
// landing) moves to the reporting channel while CurrentVia (what is actually
// delivering it) stays sight. Under v0.1.0 this seam had only a Current bool
// and reconstructed the pair as {"current", [Channel]} — putting "deeds" on
// the wire as a sustaining channel when deeds sustain nothing, by design.
//
// Fails if sightingStatus ever derives the channel list from Channel again.
func TestSightingStatusReportsTheSustainingChannelNotTheProvenance(t *testing.T) {
	out := projectSightings([]perception.Holding{{
		Subject:    "goblin-1",
		Payload:    sightPayloadBytes(t, 4, 4),
		Channel:    perception.Channel("deeds"),
		Confirmed:  9,
		CurrentVia: []perception.Channel{perception.Sight},
	}}, nil, nil)

	require.Len(t, out, 1)
	require.Equal(t, "current", out[0].Status)
	require.Equal(t, []string{"sight"}, out[0].CurrentVia,
		"the wire must name what is delivering the subject, never what last landed")
	require.Equal(t, "deeds", out[0].Channel, "provenance is still reported, separately and honestly")
}

// A ghost carries no sustaining channel at all, and "held" is the wire word
// for it. Fails if an empty CurrentVia ever projects as an empty-but-current
// sighting.
func TestSightingStatusReportsAGhostAsHeldWithNoChannels(t *testing.T) {
	out := projectSightings([]perception.Holding{{
		Subject:   "goblin-1",
		Payload:   sightPayloadBytes(t, 4, 4),
		Channel:   perception.Sight,
		Confirmed: 9,
	}}, nil, nil)

	require.Len(t, out, 1)
	require.Equal(t, "held", out[0].Status)
	require.Empty(t, out[0].CurrentVia)
}
