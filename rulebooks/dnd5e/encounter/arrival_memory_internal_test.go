// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	propagationActive  MemberID = "active-player"
	propagationGoblin  MemberID = "driven-goblin"
	propagationSubject MemberID = "remembered-player"
	propagationCaller  MemberID = "world-player"
	propagationDoor    DoorID   = "propagation-door"
)

var propagationArrival = spatial.Position{X: 2, Y: 1}

type propagationDriver struct {
	intents []TurnIntent
	calls   int
}

func (d *propagationDriver) Act(MonsterView) (TurnIntent, error) {
	i := d.calls
	d.calls++
	if i < len(d.intents) {
		return d.intents[i], nil
	}
	return Pass{}, nil
}

type propagationSight struct{}

func (propagationSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}
	return out, nil
}

type propagationStanding struct {
	down []MemberID
}

func (s *propagationStanding) Standing(members []MemberID) ([]MemberID, error) {
	return s.reported(members), nil
}

func (s *propagationStanding) Assess(members []MemberID) (*ParticipationAssessment, error) {
	return testAssessmentFromDown(members, s.reported(members)), nil
}

func (s *propagationStanding) reported(members []MemberID) []MemberID {
	asked := make(map[MemberID]bool, len(members))
	for _, id := range members {
		asked[id] = true
	}

	var out []MemberID
	for _, id := range s.down {
		if asked[id] {
			out = append(out, id)
		}
	}
	return out
}

// newArrivalMemoryEncounter builds the one real driven-arrival state every
// enclosing caller below needs. Setup first creates lawful holdings without
// forming a fight by giving every seed member the same kind; the saved roster
// is then assigned its runtime kinds and the goblin's current sight testimony
// is changed into held testimony at the arrival cell before load — so the
// goblin walks onto a cell it remembers somebody standing on.
func newArrivalMemoryEncounter(
	t *testing.T,
	withBubble bool,
) (*Encounter, *propagationStanding) {
	t.Helper()

	base, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("propagation-field", 0, 0, 10, 10)},
			Doors: []DoorInput{{
				ID: propagationDoor,
				Edges: []DoorEdge{{
					From: spatial.Position{X: 4, Y: 4},
					To:   spatial.Position{X: 5, Y: 4},
				}},
				State: DoorIsClosed(),
			}},
		},
		Members: []MemberInput{
			{ID: propagationActive, Kind: KindMonster, Position: spatial.Position{X: 0, Y: 1}},
			{ID: propagationGoblin, Kind: KindMonster, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
			{ID: propagationSubject, Kind: KindMonster, Position: spatial.Position{X: 8, Y: 8}},
			{ID: propagationCaller, Kind: KindMonster, Position: spatial.Position{X: 0, Y: 3}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	data := base.ToData()
	for i := range data.Members {
		switch data.Members[i].ID {
		case propagationGoblin:
			data.Members[i].Kind = KindMonster
		default:
			data.Members[i].Kind = KindPlayer
		}
	}

	known, err := EncodeSightTestimony(SightTestimony{
		State: LocationKnown, Position: propagationArrival,
	})
	require.NoError(t, err)
	// Ranged rather than indexed: the subject key is play/intel's own type,
	// which this package no longer names (rpg-toolkit#1691), and perception's
	// charter is what makes the persisted map readable here at all.
	seeded := data.Perception.Intel.Holdings[propagationGoblin]
	var present bool
	for subject, holding := range seeded {
		if string(subject) != string(propagationSubject) {
			continue
		}
		holding.Payload = known
		holding.CurrentVia = nil
		seeded[subject] = holding
		present = true
	}
	require.True(t, present, "seed must give the driven goblin sight testimony")

	standing := &propagationStanding{}
	driver := &propagationDriver{intents: []TurnIntent{
		Move{Path: []spatial.Position{propagationArrival}},
		Pass{},
	}}
	enc, err := LoadEncounter(&LoadEncounterInput{
		Data: data, Sight: propagationSight{}, Equipment: noHandsAreObserved{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	require.NoError(t, err)

	if withBubble {
		_, err = enc.form(&FormInput{Order: []MemberID{
			propagationActive, propagationGoblin, propagationSubject,
		}})
		require.NoError(t, err)
		require.Zero(t, driver.calls, "the player starts the fixture bubble")
	}

	return enc, standing
}

// requireArrivalMemoryUntouched pins the ghost the driven goblin walked
// through: still held, still naming the cell it was last seen on.
//
// THE MEMORY IS HONEST AND STALE, AND THAT IS THE POINT (rpg-toolkit#1691).
// This used to assert the opposite — the goblin arriving on the cell rewrote
// its own memory of whoever it remembered standing there to
// [LocationUnknown], and every row below pinned that rewrite reaching the
// enclosing verb's output as an IntelDelta category of its own. Adopting
// mind/perception deleted it: the module exposes no way to write one
// observer's testimony behind a pass, and a system that edits a player's
// memory to patch staleness is the wrong system. How to draw a memory the
// world has moved past is the client's decision.
func requireArrivalMemoryUntouched(
	t *testing.T,
	enc *Encounter,
	_ map[MemberID]*IntelDelta,
) {
	t.Helper()

	holdings, err := enc.View(&ViewInput{Member: propagationGoblin})
	require.NoError(t, err)
	for _, holding := range holdings {
		if holding.Subject != propagationSubject {
			continue
		}
		require.False(t, holding.Current, "the remembered player is still a ghost")
		location, ok := DecodeSightTestimony(holding.Payload)
		require.True(t, ok)
		require.Equal(t, LocationKnown, location.State)
		require.Equal(t, propagationArrival, location.Position)
		return
	}
	require.Fail(t, "the remembered player's holding is gone entirely")
}

// TestDrivenArrivalLeavesTheMemoryAlone pins every distinct caller that can
// enclose a driven turn: none of them edits the mover's memory of the cell it
// arrived on.
func TestDrivenArrivalLeavesTheMemoryAlone(t *testing.T) {
	tests := []struct {
		name       string
		withBubble bool
		noticeDown bool
		act        func(*testing.T, *Encounter) map[MemberID]*IntelDelta
	}{
		{
			name: "direct form",
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.form(&FormInput{Order: []MemberID{
					propagationGoblin, propagationActive, propagationSubject,
				}})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "direct transfer",
			withBubble: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Transfer(&TransferInput{Member: propagationActive, To: ClockWorld})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "step refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Step(&StepInput{
					Member: propagationCaller,
					To:     spatial.Position{X: 0, Y: 4},
				})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "pump refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Pump(&PumpInput{})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "join refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Join(&JoinInput{
					Member: "joining-player", Kind: KindPlayer,
					Cell: spatial.Position{X: 3, Y: 3},
				})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "exit refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Exit(&ExitInput{Member: propagationCaller})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "door refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.OpenDoor(&OpenDoorInput{Door: propagationDoor, Actor: propagationCaller})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enc, standing := newArrivalMemoryEncounter(t, test.withBubble)
			if test.noticeDown {
				standing.down = []MemberID{propagationActive}
			}

			deltas := test.act(t, enc)
			requireArrivalMemoryUntouched(t, enc, deltas)
		})
	}
}
